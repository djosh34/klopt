// Package patterngenerator constructively generates ASCII strings for signed pattern conjunctions.
//
//nolint:godoclint // Private construction vocabulary is documented at the public Strings seam.
package patterngenerator

import (
	"errors"
	"fmt"
	"regexp"
	regexpsyntax "regexp/syntax"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"pgregory.net/rapid"
)

// Frozen construction limits, chosen from the adversarial benchmarks in generator_test.go.
const (
	MaximumRequirements          = 16
	MaximumCumulativeSourceBytes = 128 * 1024
	MaximumCumulativeASTNodes    = 20_000
	MaximumNFAStates             = 32_768
	MaximumNFAEdges              = 65_536
	MaximumDFAStates             = 8_192
	MaximumDFATransitions        = MaximumDFAStates * asciiAlphabetSize
	MaximumProductStates         = 32_768
	MaximumProductTransitions    = MaximumProductStates * asciiAlphabetSize
	MaximumGraphBytes            = 32 * 1024 * 1024
	MaximumCertificationWork     = 8 * 1024 * 1024
	MaximumGeneratedBytes        = 256
	MaximumExtraLength           = 64
)

const asciiAlphabetSize = 128

// Requirement requests membership or non-membership in one original pattern language.
type Requirement struct {
	Source    string
	WantMatch bool
}

// Set stores component machines compiled once for multiple signed requests.
type Set struct {
	sources  []string
	machines []*dfa
	limits   constructionLimits
}

type constructionLimits struct {
	requirements          uint64
	cumulativeSourceBytes uint64
	cumulativeASTNodes    uint64
	nfaStates             uint64
	nfaEdges              uint64
	dfaStates             uint64
	dfaTransitions        uint64
	productStates         uint64
	productTransitions    uint64
	graphBytes            uint64
	certificationWork     uint64
	generatedBytes        uint64
	extraLength           uint64
}

func defaultLimits() constructionLimits {
	return constructionLimits{
		requirements:          MaximumRequirements,
		cumulativeSourceBytes: MaximumCumulativeSourceBytes,
		cumulativeASTNodes:    MaximumCumulativeASTNodes,
		nfaStates:             MaximumNFAStates,
		nfaEdges:              MaximumNFAEdges,
		dfaStates:             MaximumDFAStates,
		dfaTransitions:        MaximumDFATransitions,
		productStates:         MaximumProductStates,
		productTransitions:    MaximumProductTransitions,
		graphBytes:            MaximumGraphBytes,
		certificationWork:     MaximumCertificationWork,
		generatedBytes:        MaximumGeneratedBytes,
		extraLength:           MaximumExtraLength,
	}
}

type budget struct {
	limits constructionLimits

	cumulativeSourceBytes uint64
	cumulativeASTNodes    uint64
	nfaStates             uint64
	nfaEdges              uint64
	dfaStates             uint64
	dfaTransitions        uint64
}

// Strings constructs a Rapid generator whose every draw satisfies the signed requirements and lengths.
func Strings(
	requirements []Requirement,
	minLength int,
	maxLength *int,
	patternOption patternvalidator.Option,
) (*rapid.Generator[string], error) {
	return stringsWithLimits(requirements, minLength, maxLength, patternOption, defaultLimits())
}

// Compile constructs one reusable component machine per original pattern occurrence.
func Compile(sources []string, patternOption patternvalidator.Option) (*Set, error) {
	return compileWithLimits(sources, patternOption, defaultLimits())
}

// Strings constructs a generator for one signed request over the compiled component machines.
func (set *Set) Strings(
	wantMatches []bool,
	minLength int,
	maxLength *int,
) (*rapid.Generator[string], error) {
	if set == nil {
		return nil, errors.New("patterngenerator: nil compiled pattern set")
	}

	if len(wantMatches) != len(set.sources) {
		return nil, fmt.Errorf(
			"patterngenerator: got %d signed requirements for %d compiled patterns",
			len(wantMatches),
			len(set.sources),
		)
	}

	if err := validateLengthBounds(minLength, maxLength); err != nil {
		return nil, err
	}

	requirements := make([]Requirement, len(set.sources))
	for index, source := range set.sources {
		requirements[index] = Requirement{Source: source, WantMatch: wantMatches[index]}
	}

	graph, err := buildProductGraph(set.machines, requirements, minLength, maxLength, set.limits)
	if err != nil {
		return nil, err
	}

	return graph.generator(set.limits)
}

func stringsWithLimits(
	requirements []Requirement,
	minLength int,
	maxLength *int,
	patternOption patternvalidator.Option,
	limits constructionLimits,
) (*rapid.Generator[string], error) {
	if err := validateLengthBounds(minLength, maxLength); err != nil {
		return nil, err
	}

	sources := make([]string, len(requirements))

	wantMatches := make([]bool, len(requirements))
	for index, requirement := range requirements {
		sources[index] = requirement.Source
		wantMatches[index] = requirement.WantMatch
	}

	set, err := compileWithLimits(sources, patternOption, limits)
	if err != nil {
		return nil, err
	}

	return set.Strings(wantMatches, minLength, maxLength)
}

func compileWithLimits(
	sources []string,
	patternOption patternvalidator.Option,
	limits constructionLimits,
) (*Set, error) {
	if patternOption == nil {
		return nil, errors.New("patterngenerator: nil pattern option")
	}

	if uint64(len(sources)) > limits.requirements {
		return nil, limitError("input", "requirements", limits.requirements, uint64(len(sources)))
	}

	settings := new(patternvalidator.PatternValidation)
	patternOption(settings)

	work := &budget{limits: limits}
	machines := make([]*dfa, 0, len(sources))

	for index, source := range sources {
		machine, err := compileRequirement(source, settings, work)
		if err != nil {
			return nil, &RequirementError{Index: index, Source: source, Cause: err}
		}

		machines = append(machines, machine)
	}

	return &Set{
		sources:  append([]string(nil), sources...),
		machines: machines,
		limits:   limits,
	}, nil
}

func validateLengthBounds(minLength int, maxLength *int) error {
	if minLength < 0 {
		return fmt.Errorf("patterngenerator: negative minimum length %d", minLength)
	}

	if maxLength != nil && *maxLength < 0 {
		return fmt.Errorf("patterngenerator: negative maximum length %d", *maxLength)
	}

	if maxLength != nil && *maxLength < minLength {
		return ErrNoValues
	}

	return nil
}

//nolint:cyclop // Common policy and the two deliberately separate dialect paths stay explicit.
func compileRequirement(
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
		return &CapabilityError{Feature: "case-folding flags"}
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
		return &CapabilityError{Feature: expression.Op.String()}
	}

	for _, child := range expression.Sub {
		if err := validateRawCapabilities(child); err != nil {
			return err
		}
	}

	return nil
}

func (work *budget) add(
	counter *uint64,
	amount uint64,
	maximum uint64,
	phase string,
	limit string,
) error {
	if amount > ^uint64(0)-*counter {
		return limitError(phase, limit, maximum, ^uint64(0))
	}

	observed := *counter + amount
	if observed > maximum {
		return limitError(phase, limit, maximum, observed)
	}

	*counter = observed

	return nil
}

func limitError(phase string, limit string, maximum uint64, observed uint64) *ComplexityError {
	return &ComplexityError{Phase: phase, Limit: limit, Maximum: maximum, Observed: observed}
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
		if node.Kind == patternsyntax.KindLiteral && node.Value >= asciiAlphabetSize {
			return true
		}

		if node.Kind != patternsyntax.KindClass {
			continue
		}

		for _, item := range node.ClassItems {
			if item.Kind == patternsyntax.ClassItemRange &&
				(item.Low >= asciiAlphabetSize || item.High >= asciiAlphabetSize) {
				return true
			}
		}
	}

	return false
}
