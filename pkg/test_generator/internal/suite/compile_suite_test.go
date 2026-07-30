//nolint:godoclint // Test names state the public seam behavior.
package suite_test

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // Public-seam tests decode final CasePlans.
	"github.com/djosh34/klopt/pkg/test_generator/internal/suite"
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestCompileSuitePublishesOnlyExecutableCasesWhoseSeedsMatchTheirVerdict(t *testing.T) {
	t.Parallel()

	spec := []byte(`
openapi: 3.0.4
info: {title: test, version: 1.0.0}
paths:
  /demo:
    post:
      operationId: demo
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [x]
              additionalProperties: false
              properties:
                x:
                  enum: [1, 2, 3, ok, no]
                  anyOf:
                    - {type: number, enum: [1, 2]}
                    - {type: string, enum: [ok]}
      responses: {'204': {description: ok}}
`)

	sources, err := oas.Parse(spec)
	require.NoError(t, err)
	compiled, err := suite.CompileSuite(sources["demo"], suite.CompilerOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Cases)

	replayed, err := suite.CompileSuite(sources["demo"], suite.CompilerOptions{})
	require.NoError(t, err)
	require.Len(t, replayed.Cases, len(compiled.Cases))

	for index := range compiled.Cases {
		require.Equal(
			t,
			compiled.Cases[index].Program.Fingerprint(),
			replayed.Cases[index].Program.Fingerprint(),
		)
	}

	validations, err := validation.Parse(spec)
	require.NoError(t, err)

	limits := program.Limits{MaxSteps: 1000, MaxOutputBytes: 1000, MaxDepth: 32}

	for _, plan := range compiled.Cases {
		require.NotEmpty(t, plan.Name)
		require.NotEmpty(t, plan.Source.Pointer)
		require.NotEmpty(t, plan.Seeds)

		for _, seed := range plan.Seeds {
			value, decodeErr := plan.Program.Decode(seed, limits)
			require.NoError(t, decodeErr, plan.Name)

			body, marshalErr := value.MarshalJSON()
			require.NoError(t, marshalErr)

			accepted := len(validations["demo"].Body.Validate(body)) == 0
			require.Equal(t, plan.Expect == suite.ExpectAccepted, accepted, "%s: %s", plan.Name, body)
		}
	}
}

func TestCompileSuiteDecodesHugeNumbersPatternsAndNestedContainers(t *testing.T) {
	t.Parallel()

	spec := []byte(`{
  "openapi":"3.0.4",
  "info":{"title":"test","version":"1.0.0"},
  "paths":{"/demo":{"post":{
    "operationId":"demo",
    "requestBody":{"content":{"application/json":{"schema":{
      "type":"object",
      "required":["rows"],
      "additionalProperties":false,
      "properties":{"rows":{
        "type":"array",
        "minItems":1,
        "items":{
          "type":"object",
          "required":["id","label"],
          "additionalProperties":false,
          "properties":{
            "id":{"type":"integer","minimum":100000000000000000000},
            "label":{"type":"string","pattern":"^a+$"}
          }
        }
      }}
    }}}},
    "responses":{"204":{"description":"ok"}}
  }}}
}`)

	sources, err := oas.Parse(spec)
	require.NoError(t, err)
	compiled, err := suite.CompileSuite(sources["demo"], suite.CompilerOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Cases)

	value, err := compiled.Cases[0].Program.Decode(nil, program.Limits{
		MaxSteps: 1000, MaxOutputBytes: 1000, MaxDepth: 32,
	})
	require.NoError(t, err)
	body, err := value.MarshalJSON()
	require.NoError(t, err)

	validations, err := validation.Parse(spec)
	require.NoError(t, err)
	require.Empty(t, validations["demo"].Body.Validate(body), string(body))
}

func TestCompileSuiteLowersBoundedArraysAndIntegersWithoutEnumeratingDocuments(t *testing.T) {
	t.Parallel()

	spec := []byte(`
openapi: 3.0.4
info: {title: test, version: 1.0.0}
paths:
  /demo:
    post:
      operationId: demo
      requestBody:
        content:
          application/json:
            schema:
              type: array
              minItems: 1
              maxItems: 3
              items: {type: integer, minimum: 1}
      responses: {'204': {description: ok}}
`)

	sources, err := oas.Parse(spec)
	require.NoError(t, err)
	compiled, err := suite.CompileSuite(sources["demo"], suite.CompilerOptions{})
	require.NoError(t, err)

	var validRoot *suite.CasePlan

	for index := range compiled.Cases {
		if compiled.Cases[index].Name == "valid root" {
			validRoot = &compiled.Cases[index]

			break
		}
	}

	require.NotNil(t, validRoot)

	tape := make([]byte, 24)
	binary.LittleEndian.PutUint64(tape[0:8], 1)
	binary.LittleEndian.PutUint64(tape[8:16], 5)
	binary.LittleEndian.PutUint64(tape[16:24], 7)

	value, err := validRoot.Program.Decode(tape, program.Limits{
		MaxSteps: 100, MaxOutputBytes: 100, MaxDepth: 4,
	})
	require.NoError(t, err)
	replayedValue, err := validRoot.Program.Decode(tape, program.Limits{
		MaxSteps: 100, MaxOutputBytes: 100, MaxDepth: 4,
	})
	require.NoError(t, err)
	require.True(t, value.Equal(replayedValue))
	body, err := value.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `[6,8]`, string(body))
}

func TestCompileSuiteChargesStringProgramWorkBeforePublication(t *testing.T) {
	t.Parallel()

	spec := []byte(`
openapi: 3.0.4
info: {title: test, version: 1.0.0}
paths:
  /demo:
    post:
      operationId: demo
      requestBody:
        content:
          application/json:
            schema: {type: string}
      responses: {'204': {description: ok}}
`)

	sources, err := oas.Parse(spec)
	require.NoError(t, err)
	compiled, err := suite.CompileSuite(sources["demo"], suite.CompilerOptions{
		WorkLimits: suite.WorkLimits{UnicodeClasses: 1},
	})
	require.Nil(t, compiled)

	var resourceError *suite.ResourceError
	require.True(t, errors.As(err, &resourceError))
	require.Equal(t, "Unicode classes", resourceError.Resource)
}

func TestCompileSuiteKeepsAllOfAdditionalPropertyRulesActiveForSiblingProperties(t *testing.T) {
	t.Parallel()

	spec := []byte(`
openapi: 3.0.4
info: {title: test, version: 1.0.0}
paths:
  /demo:
    post:
      operationId: demo
      requestBody:
        content:
          application/json:
            schema:
              allOf:
                - {type: object, additionalProperties: {type: string}}
                - {type: object, required: [label], properties: {label: {type: string}}}
      responses: {'204': {description: ok}}
`)

	sources, err := oas.Parse(spec)
	require.NoError(t, err)
	compiled, err := suite.CompileSuite(sources["demo"], suite.CompilerOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Cases)

	validations, err := validation.Parse(spec)
	require.NoError(t, err)

	for _, planned := range compiled.Cases {
		for _, seed := range planned.Seeds {
			value, decodeErr := planned.Program.Decode(seed, program.Limits{
				MaxSteps: 1000, MaxOutputBytes: 1000, MaxDepth: 32,
			})
			require.NoError(t, decodeErr, planned.Name)

			body, marshalErr := value.MarshalJSON()
			require.NoError(t, marshalErr)

			accepted := len(validations["demo"].Body.Validate(body)) == 0
			require.Equal(t, planned.Expect == suite.ExpectAccepted, accepted, "%s: %s", planned.Name, body)
		}
	}
}
