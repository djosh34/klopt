package stringlanguage

import "fmt"

// CompileError retains the failed construction operation and its cause.
type CompileError struct {
	Operation string
	Err       error
}

// Error formats one language construction failure.
func (compileError *CompileError) Error() string {
	return fmt.Sprintf("string language %s: %v", compileError.Operation, compileError.Err)
}

// Unwrap exposes the parser, regexp compiler, or policy error.
func (compileError *CompileError) Unwrap() error {
	return compileError.Err
}
