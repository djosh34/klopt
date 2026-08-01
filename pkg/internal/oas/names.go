package oas

import (
	"errors"
	"fmt"
	"regexp"
)

// operationIDPattern is the exact supported operation-ID grammar.
var operationIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(?:[_/-][A-Za-z0-9]+)*$`)

// requestValidationConflicts contains every generated compilation conflict.
var requestValidationConflicts = map[string]struct{}{
	"break": {}, "case": {}, "chan": {}, "const": {}, "continue": {}, "default": {}, "defer": {},
	"else": {}, "fallthrough": {}, "for": {}, "func": {}, "go": {}, "goto": {}, "if": {}, "import": {},
	"interface": {}, "map": {}, "package": {}, "range": {}, "return": {}, "select": {}, "struct": {},
	"switch": {}, "type": {}, "var": {}, "init": {}, "RequestValidations": {}, "mustQueryDecoder": {},
	"mustPathDecoder": {}, "json": {}, "jsonvalue": {},
	"patternvalidator": {}, "validation": {}, "string": {}, "error": {}, "byte": {}, "int": {}, "nil": {},
	"true": {}, "panic": {},
}

// ErrInvalidOperationID reports an operation ID outside the supported grammar.
var ErrInvalidOperationID = errors.New("invalid operation ID")

// RequestValidationName converts an exact operation ID to its generated backing identifier.
func RequestValidationName(operationID string) (string, error) {
	if !operationIDPattern.MatchString(operationID) {
		return "", fmt.Errorf("%w: %q", ErrInvalidOperationID, operationID)
	}

	converted := make([]byte, 0, len(operationID))
	for index := range len(operationID) {
		character := operationID[index]
		switch character {
		case '_':
			converted = append(converted, '_', '_')
		case '-':
			converted = append(converted, '_', '0')
		case '/':
			converted = append(converted, '_', '1')
		default:
			converted = append(converted, character)
		}
	}

	name := string(converted)
	if _, conflict := requestValidationConflicts[name]; conflict {
		name = "_x" + name
	}

	return name, nil
}
