package oas

import (
	"errors"
	"fmt"
)

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
	if !validOperationID(operationID) {
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

// validOperationID reports whether operationID has the supported generated-name grammar.
func validOperationID(operationID string) bool {
	if operationID == "" || !isASCIILetter(operationID[0]) {
		return false
	}

	for index := 1; index < len(operationID); index++ {
		character := operationID[index]
		if isASCIIAlphaNumeric(character) {
			continue
		}

		if (character == '_' || character == '/' || character == '-') &&
			index+1 < len(operationID) && isASCIIAlphaNumeric(operationID[index+1]) {
			continue
		}

		return false
	}

	return true
}

// isASCIILetter reports whether character is an ASCII letter.
func isASCIILetter(character byte) bool {
	return character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

// isASCIIAlphaNumeric reports whether character is an ASCII letter or digit.
func isASCIIAlphaNumeric(character byte) bool {
	return isASCIILetter(character) || character >= '0' && character <= '9'
}
