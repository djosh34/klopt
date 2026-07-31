// Package stringlanguage compiles and matches exact Unicode string languages.
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

const maximumASCII = 0x7f

// Language is an immutable exact string language backed by a private DFA.
type Language struct {
	dfa dfa
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

	machine, err := compilePattern(source, settings)
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

	machine, err := compileRawPattern(expression)
	if err != nil {
		return nil, &CompileError{Operation: "compile format", Err: err}
	}

	return machine, nil
}
