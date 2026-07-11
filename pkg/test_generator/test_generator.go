// Package testgenerator generates JSON request bodies from OpenAPI schemas and checks a validator.
package testgenerator

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	//nolint:depguard // OpenAPI source lookup is internal to test_generator.
	"decode_and_validate_generator/pkg/test_generator/internal/oas"
	"pgregory.net/rapid"
)

// expectedResult is the validator verdict required by a CasePlan.
type expectedResult uint8

const (
	// expectAccepted requires the validator to accept a generated body.
	expectAccepted expectedResult = iota
	// expectRejected requires the validator to reject a generated body.
	expectRejected
)

// casePlan is one named Rapid test partition.
type casePlan struct {
	name   string
	expect expectedResult
	values *rapid.Generator[json.RawMessage]
}

// CheckJSONRequestBody checks validate with generated request bodies for operationID.
//
// Each x-valid-examples and x-invalid-examples entry on the request Schema Object is
// a separate Rapid check. The examples are the deliberately small generation seam
// supported by this first implementation; later schema compilation can replace it
// without changing this testing interface. Every generated body is checked for valid
// JSON before validate is called. The validator callback is the only runtime verdict.
func CheckJSONRequestBody(
	t *testing.T,
	openAPIYAML []byte,
	operationID string,
	validate func([]byte) error,
) {
	t.Helper()

	plans, err := compileCasePlans(openAPIYAML, operationID)
	if err != nil {
		t.Fatal(err)
	}

	if validate == nil {
		t.Fatal("validator is nil")
	}

	runCasePlans(t, plans, validate)
}

// compileCasePlans compiles trusted generation examples into separate test partitions.
func compileCasePlans(openAPIYAML []byte, operationID string) ([]casePlan, error) {
	schemaNode, err := parseOpenAPIRequestBodySchemaNode(openAPIYAML, operationID)
	if err != nil {
		return nil, err
	}

	var schema map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(*schemaNode, &schema); unmarshalErr != nil {
		return nil, fmt.Errorf("decode request schema: %w", unmarshalErr)
	}

	if schema == nil {
		return nil, errors.New("request schema must be an object")
	}

	valid, err := exampleCasePlans(schema, "x-valid-examples", expectAccepted)
	if err != nil {
		return nil, err
	}

	invalid, err := exampleCasePlans(schema, "x-invalid-examples", expectRejected)
	if err != nil {
		return nil, err
	}

	return append(valid, invalid...), nil
}

// exampleCasePlans creates one constant-value CasePlan for each trusted example.
func exampleCasePlans(
	schema map[string]json.RawMessage,
	keyword string,
	expect expectedResult,
) ([]casePlan, error) {
	raw, ok := schema[keyword]
	if !ok {
		return nil, fmt.Errorf("request schema %s is required", keyword)
	}

	var examples []json.RawMessage
	if err := json.Unmarshal(raw, &examples); err != nil {
		return nil, fmt.Errorf("request schema %s must be an array: %w", keyword, err)
	}

	if len(examples) == 0 {
		return nil, fmt.Errorf("request schema %s must not be empty", keyword)
	}

	plans := make([]casePlan, 0, len(examples))
	for index, example := range examples {
		body := append(json.RawMessage(nil), example...)
		plans = append(plans, casePlan{
			name:   fmt.Sprintf("%s / %d", keyword, index),
			expect: expect,
			values: rapid.Just(body),
		})
	}

	return plans, nil
}

// runCasePlans runs every CasePlan in its own Rapid subtest.
func runCasePlans(t *testing.T, plans []casePlan, validate func([]byte) error) {
	t.Helper()

	for _, plan := range plans {
		t.Run(plan.name, rapid.MakeCheck(func(rt *rapid.T) {
			body := plan.values.Draw(rt, "json")
			if !json.Valid(body) {
				rt.Fatalf("generator produced invalid JSON:\n%s", body)
			}

			err := validate(body)
			if plan.expect == expectAccepted && err != nil {
				rt.Fatalf("valid JSON rejected: %v\n%s", err, body)
			}

			if plan.expect == expectRejected && err == nil {
				rt.Fatalf("invalid JSON accepted:\n%s", body)
			}
		}))
	}
}

// parseOpenAPIRequestBodySchemaNode parses the document once and resolves its request Schema Object.
func parseOpenAPIRequestBodySchemaNode(openAPIYAMLSpec []byte, operationID string) (*json.RawMessage, error) {
	source, err := oas.Parse(openAPIYAMLSpec, operationID)
	if err != nil {
		return nil, fmt.Errorf("openapi request body schema lookup failed: %w", err)
	}

	schema, err := source.Resolve(source.RequestSchema)
	if err != nil {
		return nil, fmt.Errorf("openapi request body schema reference failed: %w", err)
	}

	return new(schema.Raw), nil
}
