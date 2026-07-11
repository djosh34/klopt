// Package testgenerator generates JSON request bodies from OpenAPI schemas and checks a validator.
package testgenerator

import (
	"testing"

	//nolint:depguard // Public checking is implemented by the internal OpenAPI compiler and suite planner.
	"decode_and_validate_generator/pkg/test_generator/internal/oas"
	//nolint:depguard // Public checking executes internally compiled semantic CasePlans.
	"decode_and_validate_generator/pkg/test_generator/internal/suite"
	"pgregory.net/rapid"
)

// CheckJSONRequestBody checks validate with generated application/json request bodies for operationID.
// Every CasePlan runs as its own Rapid property, and validate is the only source of verdicts.
func CheckJSONRequestBody(
	t *testing.T,
	openAPIYAML []byte,
	operationID string,
	validate func([]byte) error,
) {
	t.Helper()

	if validate == nil {
		t.Fatal("validator is nil")
	}

	source, err := oas.Parse(openAPIYAML, operationID)
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := suite.NewCompiler(source).CompileSuite()
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

			err := validate(body)
			if plannedCase.Expect == suite.ExpectAccepted && err != nil {
				rt.Fatalf("valid JSON rejected: %v\n%s", err, body)
			}

			if plannedCase.Expect == suite.ExpectRejected && err == nil {
				rt.Fatalf("invalid JSON accepted:\n%s", body)
			}
		}))
	}
}
