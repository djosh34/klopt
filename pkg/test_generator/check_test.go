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
	})
	require.Greater(t, calls, 6)
}

// TestCheckJSONRequestBodyFindsBuggyValidatorsByKeywordFamily verifies that each keyword family detects a bug.
func TestCheckJSONRequestBodyFindsBuggyValidatorsByKeywordFamily(t *testing.T) {
	t.Parallel()

	families := []string{
		"unicode-length", "maximum", "exclusive-minimum", "multiple-of",
		"pattern", "format", "array-items", "required", "additional-properties", "enum",
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
	CheckJSONRequestBody(t, requestBodySpec(fixture.schema), "checkThing", fixture.validate)
}

// validatorBugFixture describes a schema and a validator bug that the generated checks must find.
type validatorBugFixture struct {
	schema   string
	failure  string
	validate func([]byte) error
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
	}
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
`, strings.Join(lines, "\n"))
}
