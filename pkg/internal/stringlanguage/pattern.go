// Package stringlanguage compiles, intersects, matches, and lowers exact Unicode string languages.
//
//nolint:godoclint // Private construction vocabulary is documented at the public seam.
package stringlanguage

import (
	"errors"
	"fmt"
	regexpsyntax "regexp/syntax"
	"sync"

	"github.com/djosh34/klopt/pkg/patternvalidator"
)

// Frozen construction limits, chosen from the adversarial pattern benchmarks.
const (
	maximumRequirements          = 16
	maximumCumulativeSourceBytes = 128 * 1024
	maximumCumulativeASTNodes    = 20_000
	maximumNFAStates             = 32_768
	maximumNFAEdges              = 65_536
	maximumDFAStates             = 8_192
	maximumDFATransitions        = 1_048_576
	maximumProductStates         = 131_072
	maximumProductTransitions    = 16_777_216
	maximumGraphBytes            = 32 * 1024 * 1024
	maximumCertificationWork     = 8 * 1024 * 1024
)

const maximumASCII = 0x7f

// Language is an immutable exact string language backed by a private DFA.
type Language struct {
	dfa dfa
}

// Requirement says whether the result must match or not match Language.
type Requirement struct {
	Language  Language
	WantMatch bool
}

// Length is measured in Unicode code points. A nil Max means unbounded.
type Length struct {
	Min int
	Max *int
}

// Set is a compiled, proven non-empty signed product.
type Set struct {
	product product
}

var (
	uuidFormatLanguage     = sync.OnceValues(uuidLanguage)
	ipv4FormatLanguage     = sync.OnceValues(ipv4Language)
	cidrFormatLanguage     = sync.OnceValues(cidrLanguage)
	byteFormatLanguage     = sync.OnceValues(byteLanguage)
	dateFormatLanguage     = sync.OnceValues(dateLanguage)
	dateTimeFormatLanguage = sync.OnceValues(dateTimeLanguage)
	emailFormatLanguage    = sync.OnceValues(emailLanguage)
)

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
}

func defaultLimits() constructionLimits {
	return constructionLimits{
		requirements:          maximumRequirements,
		cumulativeSourceBytes: maximumCumulativeSourceBytes,
		cumulativeASTNodes:    maximumCumulativeASTNodes,
		nfaStates:             maximumNFAStates,
		nfaEdges:              maximumNFAEdges,
		dfaStates:             maximumDFAStates,
		dfaTransitions:        maximumDFATransitions,
		productStates:         maximumProductStates,
		productTransitions:    maximumProductTransitions,
		graphBytes:            maximumGraphBytes,
		certificationWork:     maximumCertificationWork,
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

// Pattern compiles one OpenAPI pattern into an exact language.
func Pattern(source string, options ...patternvalidator.Option) (Language, error) {
	settings := new(patternvalidator.PatternValidation)

	for _, option := range options {
		if option == nil {
			return Language{}, &CompileError{
				Operation: "compile pattern",
				Err:       errors.New("nil pattern option"),
			}
		}

		option(settings)
	}

	machine, err := compilePattern(source, settings, &budget{limits: defaultLimits()})
	if err != nil {
		return Language{}, &CompileError{Operation: "compile pattern", Err: err}
	}

	return Language{dfa: *machine}, nil
}

// Format compiles a supported string format into an exact language.
func Format(name string) (Language, error) {
	switch name {
	case "uuid", "uuidv4", "uuid-v4":
		return uuidFormatLanguage()
	case "ipv4":
		return ipv4FormatLanguage()
	case "cidr", "ipv4-cidr":
		return cidrFormatLanguage()
	case "byte":
		return byteFormatLanguage()
	case "date":
		return dateFormatLanguage()
	case "date-time":
		return dateTimeFormatLanguage()
	case "email":
		return emailFormatLanguage()
	}

	return Language{}, &CompileError{
		Operation: "compile format",
		Err:       fmt.Errorf("unsupported string format %q", name),
	}
}

func formatPattern(source string) (Language, error) {
	machine, err := formatDFA(source)
	if err != nil {
		return Language{}, err
	}

	return Language{dfa: *machine}, nil
}

func formatDFA(source string) (*dfa, error) {
	expression, err := regexpsyntax.Parse(source, regexpsyntax.Perl)
	if err != nil {
		return nil, &CompileError{Operation: "compile format", Err: err}
	}

	machine, err := compileRawPattern(expression, &budget{limits: defaultLimits()})
	if err != nil {
		return nil, &CompileError{Operation: "compile format", Err: err}
	}

	return machine, nil
}

// Compile constructs and proves one signed language product non-empty.
func Compile(requirements []Requirement, length Length) (*Set, error) {
	limits := defaultLimits()

	if err := validateLength(length); err != nil {
		return nil, err
	}

	if uint64(len(requirements)) > limits.requirements {
		return nil, limitError("input", "requirements", limits.requirements, uint64(len(requirements)))
	}

	machines := make([]*dfa, len(requirements))
	for index := range requirements {
		if len(requirements[index].Language.dfa.states) == 0 {
			return nil, &CompileError{
				Operation: "compile signed product",
				Err:       fmt.Errorf("requirement %d has an invalid language", index),
			}
		}

		machines[index] = &requirements[index].Language.dfa
	}

	compiled, err := buildProduct(machines, requirements, length, limits)
	if err != nil {
		return nil, err
	}

	return &Set{product: *compiled}, nil
}

// Matches reports whether value belongs to the compiled signed product.
func (set *Set) Matches(value string) bool {
	return set != nil && set.product.matches(value)
}

func validateLength(length Length) error {
	if length.Min < 0 {
		return &CompileError{
			Operation: "compile length",
			Err:       fmt.Errorf("negative minimum length %d", length.Min),
		}
	}

	if length.Max != nil && *length.Max < 0 {
		return &CompileError{
			Operation: "compile length",
			Err:       fmt.Errorf("negative maximum length %d", *length.Max),
		}
	}

	if length.Max != nil && *length.Max < length.Min {
		return &EmptyError{}
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
	return addLimited(counter, amount, maximum, phase, limit)
}

func limitError(phase string, limit string, maximum uint64, observed uint64) *ComplexityError {
	return &ComplexityError{Phase: phase, Resource: limit, Limit: maximum, Observed: observed}
}
