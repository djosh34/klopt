package testgenerator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// TestCheckJSONRequestBodyRunsCompiledPartitionsAsValidJSON verifies that compiled partitions contain valid JSON.
func TestCheckJSONRequestBodyRunsCompiledPartitionsAsValidJSON(t *testing.T) {
	t.Parallel()

	spec := requestBodySpec(`
      enum: [null, true, 1, "λ", [], {}]
`)
	calls := 0

	CheckJSONRequestBody(t, spec, "checkThing", func(body []byte) error {
		require.True(t, json.Valid(body))

		calls++

		compact, err := compactJSON(body)
		require.NoError(t, err)

		for _, accepted := range []string{`null`, `true`, `1`, `"λ"`, `[]`, `{}`} {
			if compact == accepted {
				return nil
			}
		}

		return errors.New("not an enum member")
	}, DefaultOption)
	require.Greater(t, calls, 6)
}

// TestCheckJSONRequestBodyFindsBuggyValidatorsByKeywordFamily verifies that each keyword family detects a bug.
func TestCheckJSONRequestBodyFindsBuggyValidatorsByKeywordFamily(t *testing.T) {
	t.Parallel()

	families := []string{
		"unicode-length", "maximum", "exclusive-minimum", "multiple-of",
		"pattern", "format", "array-items", "recursive-array-items", "required",
		"additional-properties", "recursive-object-property", "enum",
		"dual-multiple", "context-array-enum", "context-object-enum",
		"contradictory-items", "contradictory-additional", "contradictory-optional",
		"integer-number-allof", "exact-invalid-evidence", "exact-valid-evidence",
		"exact-invalid-property", "exact-invalid-item", "exact-invalid-ref",
	}
	fixtures := validatorBugFixtures()

	for _, family := range families {
		t.Run(family, func(t *testing.T) {
			t.Parallel()

			command := exec.Command(
				os.Args[0],
				"-test.run=^TestCheckJSONRequestBodyBuggyValidatorHelper$",
				"-test.v",
				"-rapid.checks=5",
				"-rapid.nofailfile",
			)

			command.Env = append(os.Environ(), "TEST_GENERATOR_BUG_FAMILY="+family)

			output, err := command.CombinedOutput()
			require.Error(t, err)
			require.Contains(t, string(output), fixtures[family].failure)

			if fixtures[family].caseName != "" {
				require.Contains(t, string(output), fixtures[family].caseName)
			}
		})
	}
}

// TestCheckJSONRequestBodyBuggyValidatorHelper runs a deliberately buggy validator in a subprocess.
func TestCheckJSONRequestBodyBuggyValidatorHelper(t *testing.T) {
	t.Parallel()

	family := os.Getenv("TEST_GENERATOR_BUG_FAMILY")
	if family == "" {
		t.Skip("subprocess helper")
	}

	fixture, ok := validatorBugFixtures()[family]
	require.True(t, ok)

	spec := requestBodySpec(fixture.schema)
	spec = append(spec, fixture.components...)
	CheckJSONRequestBody(
		t,
		spec,
		"checkThing",
		fixture.validate,
		DefaultOption,
	)
}

// validatorBugFixture describes a schema and a validator bug that the generated checks must find.
type validatorBugFixture struct {
	schema     string
	components []byte
	failure    string
	caseName   string
	validate   func([]byte) error
}

// validatorBugFixtures returns all deliberately buggy validator fixtures by keyword family.
func validatorBugFixtures() map[string]validatorBugFixture {
	fixtures := scalarValidatorBugFixtures()

	for family, fixture := range arrayValidatorBugFixtures() {
		fixtures[family] = fixture
	}

	for family, fixture := range objectValidatorBugFixtures() {
		fixtures[family] = fixture
	}

	return fixtures
}

// scalarValidatorBugFixtures returns fixtures for string, number, and boolean schemas.
func scalarValidatorBugFixtures() map[string]validatorBugFixture {
	return map[string]validatorBugFixture{
		"unicode-length": {
			schema: `
      type: string
      maxLength: 1
      enum: ["λ"]
`,
			failure: "valid JSON rejected",
			validate: stringValidator(func(value string) bool {
				return len(value) <= 1
			}),
		},
		"maximum": {
			schema: `
      type: number
      maximum: 5
`,
			failure:  "invalid JSON accepted",
			validate: numberValidator(func(_ json.Number) bool { return true }),
		},
		"exclusive-minimum": {
			schema: `
      type: number
      minimum: 5
      exclusiveMinimum: true
`,
			failure: "invalid JSON accepted",
			validate: numberValidator(func(value json.Number) bool {
				return value.String() == "5" || value.String() == "5.5" || value.String() == "6"
			}),
		},
		"multiple-of": {
			schema: `
      type: integer
      minimum: 0
      maximum: 20
      multipleOf: 3
`,
			failure: "invalid JSON accepted",
			validate: numberValidator(func(value json.Number) bool {
				var integer int

				return json.Unmarshal([]byte(value.String()), &integer) == nil && integer >= 0 && integer <= 20
			}),
		},
		"pattern": {
			schema: `
      type: string
      pattern: '^OK$'
      x-valid-examples: [OK]
      x-invalid-examples: [bad]
`,
			failure:  "invalid JSON accepted",
			validate: stringValidator(func(_ string) bool { return true }),
		},
		"format": {
			schema: `
      type: string
      format: email
      x-valid-examples: [a@example.com]
      x-invalid-examples: [not-an-email]
`,
			failure:  "invalid JSON accepted",
			validate: stringValidator(func(_ string) bool { return true }),
		},
		"enum": {
			schema: `
      type: boolean
      enum: [true]
`,
			failure:  "invalid JSON accepted",
			validate: booleanValidator,
		},
		"dual-multiple": {
			schema: `
      type: number
      minimum: 0
      maximum: 6
      allOf:
        - {multipleOf: 2}
        - {multipleOf: 3}
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_multipleOf_1",
			validate: numberValidator(func(value json.Number) bool {
				number, err := value.Int64()

				return err == nil && number >= 0 && number <= 6 && number%3 == 0
			}),
		},
		"integer-number-allof": {
			schema: `
      allOf:
        - {type: integer}
        - {type: number, minimum: 0, maximum: 0.1}
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_type_6",
			validate: numberValidator(integerNumberAllOfMutant),
		},
		"exact-invalid-evidence": {
			schema: `
      pattern: '^x$'
      x-valid-examples: [x]
      x-invalid-examples: [null]
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_exact_evidence_1",
			validate: func([]byte) error { return nil },
		},
		"exact-valid-evidence": {
			schema: `
      pattern: '^x$'
      x-valid-examples: [[1]]
`,
			failure:  "valid JSON rejected",
			caseName: "valid_exact_evidence_1",
			validate: func(body []byte) error {
				var value any
				if err := json.Unmarshal(body, &value); err != nil {
					return errors.New("rejected")
				}

				if _, array := value.([]any); array {
					return errors.New("rejected")
				}

				return nil
			},
		},
	}
}

// arrayValidatorBugFixtures returns fixtures for array schemas.
func arrayValidatorBugFixtures() map[string]validatorBugFixture {
	return map[string]validatorBugFixture{
		"array-items": {
			schema: `
      type: array
      minItems: 1
      maxItems: 3
      items: {type: string, minLength: 1}
`,
			failure: "invalid JSON accepted",
			validate: func(body []byte) error {
				var decoded any
				if err := json.Unmarshal(body, &decoded); err != nil {
					return errors.New("rejected")
				}

				values, ok := decoded.([]any)
				if !ok || len(values) > 3 {
					return errors.New("rejected")
				}

				for _, value := range values {
					text, ok := value.(string)
					if !ok || utf8.RuneCountInString(text) < 1 {
						return errors.New("rejected")
					}
				}

				return nil
			},
		},
		"recursive-array-items": {
			schema: `
      minItems: 1
      maxItems: 2
      allOf:
        - type: array
          items: {type: integer, minimum: 0}
        - type: array
          items: {type: integer, maximum: 0}
`,
			failure:  "invalid JSON accepted",
			validate: recursiveArrayItemsMutant,
		},
		"context-array-enum": {
			schema: `
      type: array
      minItems: 1
      maxItems: 1
      items: {type: integer, minimum: 0, maximum: 1}
      enum: [[0]]
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_enum_1",
			validate: func(body []byte) error {
				var values []int

				return json.Unmarshal(body, &values)
			},
		},
		"contradictory-items": {
			schema: `
      type: array
      items:
        allOf:
          - {type: string}
          - {type: boolean}
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_contradictory_array_items",
			validate: func(body []byte) error {
				var values []any

				return json.Unmarshal(body, &values)
			},
		},
		"exact-invalid-item": {
			schema: `
      type: array
      minItems: 1
      maxItems: 1
      items: {type: string, pattern: '^x$', x-valid-examples: [x], x-invalid-examples: [1]}
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_exact_evidence_1",
			validate: func(body []byte) error {
				var value []any

				return json.Unmarshal(body, &value)
			},
		},
	}
}

// objectValidatorBugFixtures returns fixtures for object schemas.
func objectValidatorBugFixtures() map[string]validatorBugFixture {
	return map[string]validatorBugFixture{
		"required": {
			schema: `
      type: object
      required: [name]
      properties:
        name: {type: string}
      additionalProperties: false
`,
			failure: "invalid JSON accepted",
			validate: func(body []byte) error {
				var object map[string]any
				if err := json.Unmarshal(body, &object); err != nil || object == nil {
					return errors.New("rejected")
				}

				for name, value := range object {
					if name != "name" {
						return errors.New("rejected")
					}

					if _, ok := value.(string); !ok {
						return errors.New("rejected")
					}
				}

				return nil
			},
		},
		"additional-properties": {
			schema: `
      type: object
      properties:
        name: {type: string}
      additionalProperties: false
`,
			failure: "invalid JSON accepted",
			validate: func(body []byte) error {
				var object map[string]any
				if err := json.Unmarshal(body, &object); err != nil || object == nil {
					return errors.New("rejected")
				}

				if value, ok := object["name"]; ok {
					if _, stringValue := value.(string); !stringValue {
						return errors.New("rejected")
					}
				}

				return nil
			},
		},
		"recursive-object-property": {
			schema: `
      allOf:
        - type: object
          required: [value]
          properties:
            value: {type: integer, minimum: 0}
          additionalProperties: false
        - type: object
          properties:
            value: {type: integer, maximum: 0}
          additionalProperties: false
`,
			failure:  "invalid JSON accepted",
			validate: recursiveObjectPropertyMutant,
		},
		"context-object-enum": {
			schema: `
      type: object
      required: [value]
      properties:
        value: {type: integer, minimum: 0, maximum: 1}
      additionalProperties: false
      enum: [{value: 0}]
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_enum_1",
			validate: func(body []byte) error {
				var value map[string]int

				return json.Unmarshal(body, &value)
			},
		},
		"contradictory-additional": {
			schema: `
      type: object
      additionalProperties:
        allOf:
          - {type: string}
          - {type: boolean}
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_contradictory_additional_property",
			validate: func(body []byte) error {
				var value map[string]any

				return json.Unmarshal(body, &value)
			},
		},
		"contradictory-optional": {
			schema: `
      type: object
      properties:
        value:
          allOf:
            - {type: string}
            - {type: boolean}
      additionalProperties: false
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_contradictory_optional_property_value",
			validate: func(body []byte) error {
				var value map[string]any

				return json.Unmarshal(body, &value)
			},
		},
		"exact-invalid-property": {
			schema: `
      type: object
      required: [value]
      maxProperties: 1
      properties:
        value: {type: string, pattern: '^x$', x-valid-examples: [x], x-invalid-examples: [1]}
      additionalProperties: false
`,
			failure:  "invalid JSON accepted",
			caseName: "invalid_exact_evidence_1",
			validate: func(body []byte) error {
				var value map[string]any

				return json.Unmarshal(body, &value)
			},
		},
		"exact-invalid-ref": {
			schema: `
      type: object
      required: [value]
      maxProperties: 1
      properties:
        value: {$ref: '#/components/schemas/Evidence'}
      additionalProperties: false
`,
			components: []byte(`
components:
  schemas:
    Evidence: {type: string, pattern: '^x$', x-valid-examples: [x], x-invalid-examples: [1]}
`),
			failure:  "invalid JSON accepted",
			caseName: "invalid_exact_evidence_1",
			validate: func(body []byte) error {
				var value map[string]any

				return json.Unmarshal(body, &value)
			},
		},
	}
}

// integerNumberAllOfMutant accepts bounded non-integers by ignoring the integer branch.
func integerNumberAllOfMutant(value json.Number) bool {
	number, err := value.Float64()

	return err == nil && number >= 0 && number <= 0.1
}

// recursiveArrayItemsMutant deliberately omits the second conjoined item maximum.
func recursiveArrayItemsMutant(body []byte) error {
	var values []any
	if err := json.Unmarshal(body, &values); err != nil || len(values) < 1 || len(values) > 2 {
		return errors.New("rejected")
	}

	for _, value := range values {
		number, ok := value.(float64)
		if !ok || number < 0 || number != float64(int64(number)) {
			return errors.New("rejected")
		}
	}

	return nil
}

// recursiveObjectPropertyMutant deliberately omits the second conjoined property maximum.
func recursiveObjectPropertyMutant(body []byte) error {
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil || len(object) != 1 {
		return errors.New("rejected")
	}

	value, ok := object["value"].(float64)
	if !ok || value < 0 || value != float64(int64(value)) {
		return errors.New("rejected")
	}

	return nil
}

// stringValidator returns a JSON validator with the supplied string acceptance rule.
func stringValidator(valid func(string) bool) func([]byte) error {
	return func(body []byte) error {
		var decoded any
		if err := json.Unmarshal(body, &decoded); err != nil {
			return errors.New("rejected")
		}

		value, ok := decoded.(string)
		if !ok || !valid(value) {
			return errors.New("rejected")
		}

		return nil
	}
}

// numberValidator returns a JSON validator with the supplied number acceptance rule.
func numberValidator(valid func(json.Number) bool) func([]byte) error {
	return func(body []byte) error {
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		var value any
		if err := decoder.Decode(&value); err != nil {
			return errors.New("rejected")
		}

		number, ok := value.(json.Number)
		if !ok || !valid(number) {
			return errors.New("rejected")
		}

		return nil
	}
}

// booleanValidator accepts any JSON boolean and rejects other values.
func booleanValidator(body []byte) error {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return errors.New("rejected")
	}

	if _, ok := value.(bool); !ok {
		return errors.New("rejected")
	}

	return nil
}

// compactJSON removes insignificant whitespace from a JSON value.
func compactJSON(body []byte) (string, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		return "", err
	}

	return compact.String(), nil
}

// requestBodySpec embeds a schema in a minimal OpenAPI request body document.
func requestBodySpec(schema string) []byte {
	lines := strings.Split(strings.Trim(schema, "\n"), "\n")

	indent := len(lines[0]) - len(strings.TrimLeft(lines[0], " "))

	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent = min(indent, len(line)-len(strings.TrimLeft(line, " ")))
	}

	for index := range lines {
		lines[index] = "              " + lines[index][min(indent, len(lines[index])):]
	}

	return fmt.Appendf(nil, `
openapi: 3.0.3
info:
  title: contract test
  version: 1.0.0
paths:
  /things:
    post:
      operationId: checkThing
      requestBody:
        content:
          application/json:
            schema:
%s
      responses:
        '204':
          description: accepted
`, strings.Join(lines, "\n"))
}
