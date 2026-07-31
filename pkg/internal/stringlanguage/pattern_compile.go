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
) (*dfa, error) {
	if !utf8.ValidString(source) {
		return nil, errors.New("source is not valid UTF-8")
	}

	if settings.RejectsNonASCII() && firstNonASCII(source) >= 0 {
		return nil, errors.New("non-ASCII pattern is rejected by policy")
	}

	if settings.UsesRE2() {
		if _, err := regexp.Compile(source); err != nil {
			return nil, fmt.Errorf("raw Go regexp syntax: %w", err)
		}

		expression, err := regexpsyntax.Parse(source, regexpsyntax.Perl)
		if err != nil {
			return nil, fmt.Errorf("parse accepted raw Go regexp: %w", err)
		}

		if err := validateRawCapabilities(expression); err != nil {
			return nil, err
		}

		return compileRawPattern(expression)
	}

	tree, err := patternsyntax.Parse(source)
	if err != nil {
		return nil, err
	}

	if settings.RejectsNonASCII() && hasNonASCIIExpression(tree) {
		return nil, errors.New("non-ASCII pattern value is rejected by policy")
	}

	return compileESPattern(tree)
}

func validateRawCapabilities(expression *regexpsyntax.Regexp) error {
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
