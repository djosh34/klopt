// Package testgenerator generates JSON request bodies from OpenAPI schemas and checks a validator.
package testgenerator

import (
	"testing"

	"decode_and_validate_generator/pkg/internal/oas"
	"decode_and_validate_generator/pkg/test_generator/internal/suite"
	"pgregory.net/rapid"
)

// Option configures request-body case generation.
type Option func(*suite.Compiler)

// MustHaveAllXValidCases requires every oracle-backed allOf merge to retain a shared valid case.
func MustHaveAllXValidCases(compiler *suite.Compiler) {
	suite.MustHaveAllXValidCases(compiler)
}

// DefaultOption applies every default case-generation requirement.
func DefaultOption(compiler *suite.Compiler) {
	MustHaveAllXValidCases(compiler)
}

// CheckJSONRequestBody checks validate with generated application/json request bodies for operationID.
// Every CasePlan runs as its own Rapid property, and validate is the only source of verdicts.
func CheckJSONRequestBody(
	t *testing.T,
	openAPIYAML []byte,
	operationID string,
	validate func([]byte) error,
	options ...Option,
) {
	t.Helper()

	if validate == nil {
		t.Fatal("validator is nil")
	}

	source, err := oas.Parse(openAPIYAML, operationID)
	if err != nil {
		t.Fatal(err)
	}

	compileOptions := make([]suite.CompileOption, len(options))
	for index, option := range options {
		compileOptions[index] = suite.CompileOption(option)
	}

	compiled, err := suite.NewCompiler(source).CompileSuite(compileOptions...)
	if err != nil {
		t.Fatal(err)
	}

	for _, plannedCase := range compiled.Cases {
		t.Run(plannedCase.Name, rapid.MakeCheck(func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "json value")

			body, marshalErr := value.MarshalJSON()
			if marshalErr != nil {
				rt.Fatalf("encode generated JSON: %v", marshalErr)
			}

			checkValidationResult(rt, plannedCase.Expect, body, validate(body))
		}))
	}
}

// checkValidationResult checks one validator verdict against its planned expectation.
func checkValidationResult(rt *rapid.T, expectation suite.ExpectedResult, body []byte, err error) {
	if expectation == suite.ExpectAccepted && err != nil {
		rt.Fatalf("valid JSON rejected: %v\n%s", err, body)
	}

	if expectation == suite.ExpectRejected && err == nil {
		rt.Fatalf("invalid JSON accepted:\n%s", body)
	}
}
