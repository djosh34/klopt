package stringlanguage

import "fmt"

// EmptyError means exhaustive reachability proved the requested language empty.
type EmptyError struct{}

// Error reports an empty signed language product.
func (*EmptyError) Error() string {
	return "string language requirements have no values"
}

// ComplexityError reports one fixed resource limit and the value that exceeded it.
type ComplexityError struct {
	Phase    string
	Resource string
	Limit    uint64
	Observed uint64
}

// Error formats the resource-limit failure.
func (complexityError *ComplexityError) Error() string {
	return fmt.Sprintf(
		"string language %s exceeds %s limit: maximum %d, observed %d",
		complexityError.Phase,
		complexityError.Resource,
		complexityError.Limit,
		complexityError.Observed,
	)
}

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
