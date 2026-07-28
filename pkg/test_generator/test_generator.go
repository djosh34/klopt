// Package testgenerator generates JSON request bodies from OpenAPI schemas and checks a validator.
package testgenerator

import (
	"maps"
	"slices"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/test_generator/internal/suite"
	"pgregory.net/rapid"
)

// CheckJSONRequestBodies checks validate with generated application/json request bodies for every operation.
// Every CasePlan runs as its own Rapid property, and validate is the only source of verdicts.
func CheckJSONRequestBodies(
	t *testing.T,
	openAPIYAML []byte,
	validate func(operationID string, body []byte) error,
	patternOption patternvalidator.Option,
) {
	t.Helper()

	if validate == nil {
		t.Fatal("validator is nil")
	}

	if patternOption == nil {
		t.Fatal("pattern option is nil")
	}

	sources, err := oas.Parse(openAPIYAML)
	if err != nil {
		t.Fatal(err)
	}

	for _, operationID := range slices.Sorted(maps.Keys(sources)) {
		if len(sources[operationID].RequestSchema.Raw) == 0 {
			continue
		}

		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			compiled, err := suite.NewCompiler(sources[operationID], patternOption).CompileSuite()
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

					checkValidationResult(rt, plannedCase.Expect, body, validate(operationID, body))
				}))
			}
		})
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
