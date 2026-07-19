// Package patternvalidator validates strings against OpenAPI 3.0.x patterns.
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
	regexpsyntax "regexp/syntax"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/patternsyntax"
)

const (
	maximumGeneratedRegexpBytes = 1024 * 1024
	maximumASCII                = 0x7f
	hexadecimalBase             = 16
	hexadecimalAlphaOffset      = 10
	octalBase                   = 8
	rawHexadecimalDigits        = 2
	rawOctalDigits              = 3
	rawQuoteEnd                 = `\E`
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
			return nil, &ParseError{
				Kind: ParseErrorRawGoSyntax, Offset: rawGoErrorOffset(source, err), Cause: err,
			}
		}

		if validation.rejectNonASCII {
			if offset := firstNonASCIIRawEscape(source); offset >= 0 {
				return nil, &ParseError{
					Kind: ParseErrorPolicy, Offset: offset,
					Cause: errors.New("non-ASCII pattern value is rejected by policy"),
				}
			}
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

	if !validation.useRE2 {
		value = encodeUTF16CodeUnits(value)
	}

	for _, compiled := range validation.checks {
		if compiled.regexp.MatchString(value) != compiled.wantMatch {
			return false
		}
	}

	return true
}

// encodeUTF16CodeUnits maps every ECMAScript 16-bit string element to one Go
// regexp rune. Surrogates use a disjoint valid-rune block because Go regexp
// cannot consume surrogate code points directly.
func encodeUTF16CodeUnits(value string) string {
	const (
		firstSurrogate = 0xd800
		lastSurrogate  = 0xdfff
		mappedBase     = 0x10000
	)

	units := utf16.Encode([]rune(value))

	encoded := make([]rune, len(units))
	for index, unit := range units {
		if unit >= firstSurrogate && unit <= lastSurrogate {
			encoded[index] = mappedBase + rune(unit-firstSurrogate)
		} else {
			encoded[index] = rune(unit)
		}
	}

	return string(encoded)
}

func rawGoErrorOffset(source string, err error) int {
	var syntaxError *regexpsyntax.Error
	if !errors.As(err, &syntaxError) {
		return 0
	}

	if syntaxError.Code == regexpsyntax.ErrTrailingBackslash && len(source) > 0 {
		return len(source) - 1
	}

	if syntaxError.Expr != "" {
		if offset := strings.Index(source, syntaxError.Expr); offset >= 0 {
			return offset
		}
	}

	return 0
}

func firstNonASCIIRawEscape(source string) int {
	for index := 0; index < len(source); index++ {
		if source[index] != '\\' || index+1 >= len(source) {
			continue
		}

		offset := index
		next, value, ok := rawEscapeValue(source, offset)
		index = next

		if ok && value > maximumASCII {
			return offset
		}
	}

	return -1
}

func rawEscapeValue(source string, offset int) (int, rune, bool) {
	marker := offset + 1
	if source[marker] == 'Q' {
		quoted := source[marker+1:]
		if quoteEnd := strings.Index(quoted, rawQuoteEnd); quoteEnd >= 0 {
			return marker + quoteEnd + len(rawQuoteEnd), 0, false
		}

		return len(source) - 1, 0, false
	}

	if source[marker] == 'x' {
		digits := marker + 1
		if source[digits] == '{' {
			digits++
			closing := digits + strings.IndexByte(source[digits:], '}')

			return closing, hexadecimalValue(source[digits:closing]), true
		}

		end := digits + rawHexadecimalDigits

		return end - 1, hexadecimalValue(source[digits:end]), true
	}

	if source[marker] >= '0' && source[marker] <= '7' {
		end := min(marker+rawOctalDigits, len(source))

		return end - 1, octalValue(source[marker:end]), true
	}

	return marker, 0, false
}

func hexadecimalValue(source string) rune {
	value := rune(0)
	for index := range len(source) {
		value *= hexadecimalBase
		if source[index] >= '0' && source[index] <= '9' {
			value += rune(source[index] - '0')
		} else if source[index] >= 'a' && source[index] <= 'f' {
			value += rune(source[index]-'a') + hexadecimalAlphaOffset
		} else {
			value += rune(source[index]-'A') + hexadecimalAlphaOffset
		}
	}

	return value
}

func octalValue(source string) rune {
	value := rune(0)
	for index := range len(source) {
		value = value*octalBase + rune(source[index]-'0')
	}

	return value
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
