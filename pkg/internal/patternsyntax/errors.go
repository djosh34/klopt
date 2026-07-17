package patternsyntax

import (
	"errors"
	"fmt"
)

// ErrorKind classifies a rejected pattern.
type ErrorKind uint8

// Parser error classes.
const (
	ErrorInvalidSyntax ErrorKind = iota
	ErrorUnsupported
	ErrorForeignSyntax
	ErrorTooComplex
)

// ErrTooComplex identifies a parser resource-limit failure.
var ErrTooComplex = errors.New("pattern syntax is too complex")

// Error reports one syntax rejection at an original UTF-8 byte offset.
type Error struct {
	Kind     ErrorKind
	Offset   int
	Message  string
	Limit    string
	Maximum  uint64
	Observed uint64
}

// Error formats the rejection without losing its byte offset.
func (parseError *Error) Error() string {
	if parseError.Kind == ErrorTooComplex {
		return fmt.Sprintf(
			"pattern syntax at byte %d exceeds %s limit: maximum %d, observed %d",
			parseError.Offset,
			parseError.Limit,
			parseError.Maximum,
			parseError.Observed,
		)
	}

	return fmt.Sprintf("pattern syntax at byte %d: %s", parseError.Offset, parseError.Message)
}

// Is supports errors.Is(err, ErrTooComplex).
func (parseError *Error) Is(target error) bool {
	return target == ErrTooComplex && parseError.Kind == ErrorTooComplex
}
