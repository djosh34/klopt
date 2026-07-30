//nolint:godoclint // Private dialect admission stays behind Pattern.
package stringlanguage

import (
	"errors"
	"fmt"
	"regexp"
	regexpsyntax "regexp/syntax"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
	"github.com/djosh34/klopt/pkg/patternvalidator"
)

//nolint:cyclop // Common policy and the two deliberately separate dialect paths stay explicit.
func compilePattern(
	source string,
	settings *patternvalidator.PatternValidation,
	work *budget,
) (*dfa, error) {
	if len(source) > patternsyntax.MaximumSourceBytes {
		return nil, limitError(
			"input", "source bytes", patternsyntax.MaximumSourceBytes, uint64(len(source)),
		)
	}

	if !utf8.ValidString(source) {
		return nil, errors.New("source is not valid UTF-8")
	}

	if settings.RejectsNonASCII() && firstNonASCII(source) >= 0 {
		return nil, errors.New("non-ASCII pattern is rejected by policy")
	}

	if err := work.add(
		&work.cumulativeSourceBytes,
		uint64(len(source)),
		work.limits.cumulativeSourceBytes,
		"input",
		"cumulative source bytes",
	); err != nil {
		return nil, err
	}

	if settings.UsesRE2() {
		if _, err := regexp.Compile(source); err != nil {
			return nil, fmt.Errorf("raw Go regexp syntax: %w", err)
		}

		expression, err := regexpsyntax.Parse(source, regexpsyntax.Perl)
		if err != nil {
			return nil, fmt.Errorf("parse accepted raw Go regexp: %w", err)
		}

		if err := work.add(
			&work.cumulativeASTNodes,
			rawNodeCount(expression),
			work.limits.cumulativeASTNodes,
			"parse",
			"cumulative AST nodes",
		); err != nil {
			return nil, err
		}

		if err := validateRawCapabilities(expression); err != nil {
			return nil, err
		}

		return compileRawPattern(expression, work)
	}

	tree, err := patternsyntax.Parse(source)
	if err != nil {
		return nil, err
	}

	if settings.RejectsNonASCII() && hasNonASCIIExpression(tree) {
		return nil, errors.New("non-ASCII pattern value is rejected by policy")
	}

	if err := work.add(
		&work.cumulativeASTNodes,
		uint64(len(tree.Nodes)),
		work.limits.cumulativeASTNodes,
		"parse",
		"cumulative AST nodes",
	); err != nil {
		return nil, err
	}

	return compileESPattern(tree, work)
}

func rawNodeCount(expression *regexpsyntax.Regexp) uint64 {
	count := uint64(0)

	stack := []*regexpsyntax.Regexp{expression}
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]

		if count == ^uint64(0) {
			return count
		}

		count++

		stack = append(stack, node.Sub...)
	}

	return count
}

func validateRawCapabilities(expression *regexpsyntax.Regexp) error {
	if expression.Flags&regexpsyntax.FoldCase != 0 {
		return errors.New("raw Go regexp generator does not support case-folding flags")
	}

	switch expression.Op {
	case regexpsyntax.OpNoMatch,
		regexpsyntax.OpEmptyMatch,
		regexpsyntax.OpLiteral,
		regexpsyntax.OpCharClass,
		regexpsyntax.OpAnyCharNotNL,
		regexpsyntax.OpAnyChar,
		regexpsyntax.OpBeginLine,
		regexpsyntax.OpEndLine,
		regexpsyntax.OpBeginText,
		regexpsyntax.OpEndText,
		regexpsyntax.OpWordBoundary,
		regexpsyntax.OpNoWordBoundary,
		regexpsyntax.OpCapture,
		regexpsyntax.OpStar,
		regexpsyntax.OpPlus,
		regexpsyntax.OpQuest,
		regexpsyntax.OpRepeat,
		regexpsyntax.OpConcat,
		regexpsyntax.OpAlternate:
	default:
		return fmt.Errorf("raw Go regexp generator does not support %s", expression.Op)
	}

	for _, child := range expression.Sub {
		if err := validateRawCapabilities(child); err != nil {
			return err
		}
	}

	return nil
}

func firstNonASCII(value string) int {
	for index := range len(value) {
		if value[index] >= utf8.RuneSelf {
			return index
		}
	}

	return -1
}

func hasNonASCIIExpression(tree *patternsyntax.Tree) bool {
	for _, node := range tree.Nodes {
		if node.Kind == patternsyntax.KindLiteral && node.Value > maximumASCII {
			return true
		}

		if node.Kind != patternsyntax.KindClass {
			continue
		}

		for _, item := range node.ClassItems {
			if item.Kind == patternsyntax.ClassItemRange &&
				(item.Low > maximumASCII || item.High > maximumASCII) {
				return true
			}
		}
	}

	return false
}
