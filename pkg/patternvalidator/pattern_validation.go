// Package patternvalidator validates strings against OpenAPI 3.0.3 patterns.
//
// By default Parse accepts the documented ECMAScript 5.1 regular subset.
// UseRE2 selects raw Go regexp syntax. RejectNonASCII makes the guaranteed
// ASCII-only contract explicit for both patterns and subjects.
//
//nolint:godoclint // Private compilation helpers are documented by their small call sites.
package patternvalidator

import (
	"errors"
	"regexp"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
)

const (
	maximumGeneratedRegexpBytes = 1024 * 1024
	maximumASCII                = 0x7f
)

// Option configures a newly allocated PatternValidation before parsing starts.
type Option func(*PatternValidation)

// PatternValidation is one immutable compiled pattern validation.
type PatternValidation struct {
	checks         []check
	rejectNonASCII bool
	useRE2         bool
	sealed         bool
}

type check struct {
	regexp    *regexp.Regexp
	wantMatch bool
}

// RejectNonASCII requires an ASCII pattern and rejects non-ASCII subjects.
func RejectNonASCII(validation *PatternValidation) {
	validation.mustAcceptOptions()
	validation.rejectNonASCII = true
}

// UseRE2 selects raw Go regexp syntax and semantics.
func UseRE2(validation *PatternValidation) {
	validation.mustAcceptOptions()
	validation.useRE2 = true
}

// Parse compiles source into an immutable pattern validation.
//
//nolint:cyclop,nestif // Construction stages remain explicit so no partially compiled value escapes.
func Parse(source string, options ...Option) (*PatternValidation, error) {
	validation := new(PatternValidation)

	for _, option := range options {
		if option == nil {
			return nil, errors.New("patternvalidator: nil option")
		}

		option(validation)
	}

	if len(source) > patternsyntax.MaximumSourceBytes {
		return nil, &ComplexityError{
			Phase: "input", Limit: "source bytes",
			Maximum: patternsyntax.MaximumSourceBytes, Observed: uint64(len(source)),
		}
	}

	if !utf8.ValidString(source) {
		return nil, &ParseError{
			Kind: ParseErrorInvalidSyntax, Offset: firstInvalidUTF8(source),
			Cause: errors.New("source is not valid UTF-8"),
		}
	}

	if validation.rejectNonASCII {
		if offset := firstNonASCII(source); offset >= 0 {
			return nil, &ParseError{
				Kind: ParseErrorPolicy, Offset: offset,
				Cause: errors.New("non-ASCII pattern is rejected by policy"),
			}
		}
	}

	var checks []check

	if validation.useRE2 {
		compiled, err := regexp.Compile(source)
		if err != nil {
			return nil, &ParseError{Kind: ParseErrorRawGoSyntax, Cause: err}
		}

		checks = []check{{regexp: compiled, wantMatch: true}}
	} else {
		tree, err := patternsyntax.Parse(source)
		if err != nil {
			return nil, publicSyntaxError(err)
		}

		if validation.rejectNonASCII {
			if span, ok := firstNonASCIIExpression(tree); ok {
				return nil, &ParseError{
					Kind: ParseErrorPolicy, Offset: span.Start,
					Cause: errors.New("non-ASCII pattern value is rejected by policy"),
				}
			}
		}

		specifications, err := translate(tree)
		if err != nil {
			return nil, err
		}

		checks = make([]check, 0, len(specifications))
		for _, specification := range specifications {
			compiled, err := regexp.Compile(specification.source)
			if err != nil {
				return nil, &ParseError{
					Kind:   ParseErrorInternalTranslation,
					Offset: specification.span.Start,
					Cause:  err,
				}
			}

			checks = append(checks, check{regexp: compiled, wantMatch: specification.wantMatch})
		}
	}

	validation.checks = checks
	validation.sealed = true

	return validation, nil
}

// MustParse is like Parse but panics if source cannot be compiled.
func MustParse(source string, options ...Option) *PatternValidation {
	validation, err := Parse(source, options...)
	if err != nil {
		panic(err)
	}

	return validation
}

// Validate reports whether value satisfies every compiled check.
func (validation *PatternValidation) Validate(value string) bool {
	if validation == nil || len(validation.checks) == 0 {
		return false
	}

	if validation.rejectNonASCII && firstNonASCII(value) >= 0 {
		return false
	}

	for _, compiled := range validation.checks {
		if compiled.regexp.MatchString(value) != compiled.wantMatch {
			return false
		}
	}

	return true
}

// RejectsNonASCII reports the effective strict-ASCII setting.
func (validation *PatternValidation) RejectsNonASCII() bool {
	return validation != nil && validation.rejectNonASCII
}

// UsesRE2 reports the effective raw-Go-regexp setting.
func (validation *PatternValidation) UsesRE2() bool {
	return validation != nil && validation.useRE2
}

func (validation *PatternValidation) mustAcceptOptions() {
	if validation.sealed {
		panic("patternvalidator: option applied after Parse")
	}
}

func publicSyntaxError(err error) error {
	var syntaxError *patternsyntax.Error
	if !errors.As(err, &syntaxError) {
		return &ParseError{Kind: ParseErrorInternalTranslation, Cause: err}
	}

	if syntaxError.Kind == patternsyntax.ErrorTooComplex {
		return &ComplexityError{
			Phase: "parse", Limit: syntaxError.Limit,
			Maximum: syntaxError.Maximum, Observed: syntaxError.Observed,
		}
	}

	kind := ParseErrorInvalidSyntax

	switch syntaxError.Kind {
	case patternsyntax.ErrorUnsupported:
		kind = ParseErrorUnsupported
	case patternsyntax.ErrorForeignSyntax:
		kind = ParseErrorForeignSyntax
	case patternsyntax.ErrorInvalidSyntax, patternsyntax.ErrorTooComplex:
	}

	return &ParseError{Kind: kind, Offset: syntaxError.Offset, Cause: syntaxError}
}

func firstNonASCII(value string) int {
	for index := range len(value) {
		if value[index] >= utf8.RuneSelf {
			return index
		}
	}

	return -1
}

func firstInvalidUTF8(value string) int {
	for index := 0; index < len(value); {
		_, size := utf8.DecodeRuneInString(value[index:])
		if size == 1 && value[index] >= utf8.RuneSelf {
			return index
		}

		index += size
	}

	return 0
}

//nolint:cyclop // Both literal and range payloads need source-ordered policy checks.
func firstNonASCIIExpression(tree *patternsyntax.Tree) (patternsyntax.Span, bool) {
	first := patternsyntax.Span{}
	found := false

	for _, node := range tree.Nodes {
		nonASCII := node.Kind == patternsyntax.KindLiteral && node.Value > maximumASCII
		if node.Kind == patternsyntax.KindClass {
			for _, item := range node.ClassItems {
				if item.Kind == patternsyntax.ClassItemRange && (item.Low > 0x7f || item.High > 0x7f) {
					nonASCII = true

					break
				}
			}
		}

		if nonASCII && (!found || node.Span.Start < first.Start) {
			first = node.Span
			found = true
		}
	}

	return first, found
}
