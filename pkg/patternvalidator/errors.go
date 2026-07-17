package patternvalidator

import (
	"errors"
	"fmt"
)

// ErrTooComplex identifies a fixed resource-limit failure.
var ErrTooComplex = errors.New("pattern validation is too complex")

// ParseErrorKind classifies a pattern that could not be compiled.
type ParseErrorKind uint8

// Pattern parse error classes.
const (
	ParseErrorInvalidSyntax ParseErrorKind = iota
	ParseErrorUnsupported
	ParseErrorForeignSyntax
	ParseErrorPolicy
	ParseErrorRawGoSyntax
	ParseErrorInternalTranslation
)

// ParseError reports a rejected source at its zero-based UTF-8 byte offset.
type ParseError struct {
	Kind   ParseErrorKind
	Offset int
	Cause  error
}

// Error formats the classified parse failure.
func (parseError *ParseError) Error() string {
	return fmt.Sprintf("pattern at byte %d: %s: %v", parseError.Offset, parseError.Kind, parseError.Cause)
}

// Unwrap exposes the underlying parser or regexp compiler error.
func (parseError *ParseError) Unwrap() error {
	return parseError.Cause
}

// String names one parse error class.
func (kind ParseErrorKind) String() string {
	switch kind {
	case ParseErrorInvalidSyntax:
		return "invalid syntax"
	case ParseErrorUnsupported:
		return "valid ECMAScript 5.1 syntax is unsupported"
	case ParseErrorForeignSyntax:
		return "foreign syntax"
	case ParseErrorPolicy:
		return "policy rejection"
	case ParseErrorRawGoSyntax:
		return "invalid raw Go regexp syntax"
	case ParseErrorInternalTranslation:
		return "internal translation failure"
	default:
		return "unknown parse failure"
	}
}

// ComplexityError reports one fixed limit and the value that exceeded it.
type ComplexityError struct {
	Phase    string
	Limit    string
	Maximum  uint64
	Observed uint64
}

// Error formats the resource-limit failure.
func (complexityError *ComplexityError) Error() string {
	return fmt.Sprintf(
		"pattern %s exceeds %s limit: maximum %d, observed %d",
		complexityError.Phase,
		complexityError.Limit,
		complexityError.Maximum,
		complexityError.Observed,
	)
}

// Is supports errors.Is(err, ErrTooComplex).
func (complexityError *ComplexityError) Is(target error) bool {
	return target == ErrTooComplex
}
