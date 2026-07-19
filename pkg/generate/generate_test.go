package generate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"

	"github.com/stretchr/testify/require"
)

// TestGenerateWritesCompiledValidation covers every exported validation field and generated compilation.
func TestGenerateWritesCompiledValidation(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-fixture-")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(output))
	})

	spec := []byte(strings.Join([]string{
		"openapi: 3.0.3",
		"info: {title: generated, version: \"1\"}",
		"paths:",
		"  /zeta:",
		"    post:",
		"      operationId: zetaRequest",
		"      requestBody:",
		"        content:",
		"          application/json:",
		"            schema: {type: boolean}",
		"      responses:",
		"        '204': {description: empty}",
		"  /alpha:",
		"    post:",
		"      operationId: alphaRequest",
		"      requestBody:",
		"        required: true",
		"        content:",
		"          application/json:",
		"            schema:",
		"              type: object",
		"              nullable: true",
		"              minProperties: 4",
		"              maxProperties: 6",
		"              required: [array, enum, number, text]",
		"              additionalProperties: {type: string, minLength: 1}",
		"              properties:",
		"                array:",
		"                  type: array",
		"                  nullable: true",
		"                  minItems: 1",
		"                  maxItems: 3",
		"                  uniqueItems: true",
		"                  items: {type: integer, minimum: 1, maximum: 5, multipleOf: 1}",
		"                enum:",
		"                  enum:",
		"                    - null",
		"                    - false",
		"                    - true",
		"                    - 0",
		"                    - ''",
		"                    - []",
		"                    - {}",
		"                    - [{nested: [false, 2, x]}]",
		"                    - {nested: [null, {x: []}]}",
		"                number:",
		"                  type: number",
		"                  minimum: 1",
		"                  exclusiveMinimum: true",
		"                  maximum: 10",
		"                  exclusiveMaximum: true",
		"                  multipleOf: 0.5",
		"                text:",
		"                  type: string",
		"                  minLength: 3",
		"                  maxLength: 30",
		"                  pattern: '^[^@]+@[^@]+$'",
		"                  format: email",
		"                closed:",
		"                  type: object",
		"                  additionalProperties: false",
		"                  properties:",
		"                    child: {type: string}",
		"              allOf:",
		"                - {minProperties: 4}",
		"                - properties:",
		"                    flag: {type: boolean}",
		"      responses:",
		"        '204': {description: empty}",
	}, "\n"))

	err = Generate(output, "generatefixture", spec, validation.PatternOptions())
	require.NoError(t, err)

	generated, err := os.ReadFile(filepath.Join(output, "validate.go"))
	require.NoError(t, err)

	generatedSource := string(generated)

	for _, field := range []string{
		"SchemaPointer:", "BodyRequired:", "KindValidation:", "Type:", "Nullable:",
		"EnumValidation:", "Values:", "ExactValues:", "ExactValue:", "NumberValidation:", "Minimum:",
		"Maximum:", "Exclusive:", "MultipleOf:", "ExactMultipleOf:", "StringValidation:",
		"MinLength:", "MaxLength:", "Pattern:", "Format:", "CompiledPattern:",
		"ArrayValidation:", "MinItems:", "MaxItems:", "Items:", "UniqueItems:",
		"ObjectValidation:", "MinProperties:", "MaxProperties:", "Required:", "Properties:", "Name:", "Validation:",
		"AdditionalPropertiesAllowed:", "AdditionalPropertiesValidation:", "AllOfValidations:",
	} {
		require.Contains(t, generatedSource, field)
	}

	for _, nestedLocation := range []string{
		"/properties/array/items",
		"/properties/closed/properties/child",
		"/additionalProperties",
		"/allOf/0",
	} {
		require.Contains(t, generatedSource, nestedLocation)
	}

	require.NotContains(t, generatedSource, "func")
	require.NotContains(t, generatedSource, "nodes :=")
	require.NotContains(t, generatedSource, ".Validation =")
	require.Equal(t, 3, strings.Count(generatedSource, "\nvar "))
	require.Less(
		t,
		strings.Index(generatedSource, "var alphaRequest"),
		strings.Index(generatedSource, "var zetaRequest"),
	)
	require.Less(
		t,
		strings.Index(generatedSource, "var zetaRequest"),
		strings.Index(generatedSource, "var validations"),
	)

	probe := []byte(`package generatefixture

import "testing"

func TestGeneratedValidation(t *testing.T) {
	enumValues := []string{
		"null",
		"false",
		"true",
		"0",
		"\"\"",
		"[]",
		"{}",
		"[{\"nested\":[false,2,\"x\"]}]",
		"{\"nested\":[null,{\"x\":[]}]}",
	}
	for _, enumValue := range enumValues {
		body := []byte(
			"{\"array\":[2],\"enum\":" + enumValue +
				",\"number\":1.5,\"text\":\"a@b.co\",\"extra\":\"ok\"}",
		)
		if errs := validations["alphaRequest"].Validate(body); len(errs) != 0 {
			t.Fatalf("valid enum %s: %v", enumValue, errs)
		}
	}

	invalid := []byte(
		"{\"array\":[2,2],\"enum\":\"missing\",\"number\":1,\"text\":\"bad\",\"extra\":1}",
	)
	if errs := validations["alphaRequest"].Validate(invalid); len(errs) == 0 {
		t.Fatal("invalid body passed")
	}
	if errs := validations["alphaRequest"].Validate([]byte("null")); len(errs) != 0 {
		t.Fatalf("nullable body: %v", errs)
	}
	if errs := validations["zetaRequest"].Validate([]byte("true")); len(errs) != 0 {
		t.Fatalf("zeta body: %v", errs)
	}
}
`)
	require.NoError(t, os.WriteFile(filepath.Join(output, "probe_test.go"), probe, 0o644))

	command := exec.CommandContext(
		t.Context(), "go", "test", "./pkg/"+filepath.Base(output), "-run", "TestGeneratedValidation",
	)
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGenerateWritesCompiledQueryDecoder verifies generated metadata avoids runtime spec compilation.
func TestGenerateWritesCompiledQueryDecoder(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-query-fixture-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(output)) })

	spec := []byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - {name: tags, in: query, schema: {type: array, items: {type: string}}}
        - {name: limit, in: query, schema: {type: integer, default: 25}}
`)
	require.NoError(t, Generate(output, "generatequeryfixture", spec, validation.PatternOptions()))

	generated, err := os.ReadFile(filepath.Join(output, "validate.go"))
	require.NoError(t, err)
	require.Contains(t, string(generated), "QueryDecoderDefinition")
	require.Contains(t, string(generated), "NewQueryDecoderFromGenerated")
	require.NotContains(t, string(generated), "validation.Parse")

	probe := []byte(`package generatequeryfixture

import (
	"net/url"
	"testing"
)

func TestGeneratedQueryDecoder(t *testing.T) {
	got, err := queryDecoders["listItems"].Decode(&url.URL{RawQuery: "tags=go&tags=api"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"tags\":[\"go\",\"api\"],\"limit\":25}" {
		t.Fatalf("got %s", got)
	}
}
`)
	require.NoError(t, os.WriteFile(filepath.Join(output, "probe_test.go"), probe, 0o644))

	command := exec.CommandContext(
		t.Context(), "go", "test", "./pkg/"+filepath.Base(output), "-run", "TestGeneratedQueryDecoder",
	)
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGeneratedQueryDecoderMatchesRuntimeForEveryWireKind checks generated Decode parity for the full style matrix.
//
//nolint:funlen // The embedded OpenAPI document keeps every wire case visible beside its generated-package probe.
func TestGeneratedQueryDecoderMatchesRuntimeForEveryWireKind(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-query-parity-fixture-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(output)) })

	spec := []byte(`openapi: 3.0.3
paths:
  /primitive:
    get:
      operationId: primitive
      parameters:
        - {name: q, in: query, schema: {type: string}}
  /form-array-repeated:
    get:
      operationId: formArrayRepeated
      parameters:
        - {name: q, in: query, schema: {type: array, items: {type: string}}}
  /form-array-delimited:
    get:
      operationId: formArrayDelimited
      parameters:
        - {name: q, in: query, explode: false, schema: {type: array, items: {type: string}}}
  /space-array:
    get:
      operationId: spaceArray
      parameters:
        - {name: q, in: query, style: spaceDelimited, explode: false, schema: {type: array, items: {type: string}}}
  /pipe-array:
    get:
      operationId: pipeArray
      parameters:
        - {name: q, in: query, style: pipeDelimited, explode: false, schema: {type: array, items: {type: string}}}
  /form-object-named:
    get:
      operationId: formObjectNamed
      parameters:
        - name: q
          in: query
          explode: false
          schema: {type: object, additionalProperties: false, properties: {x: {type: string}}}
  /form-object-exploded:
    get:
      operationId: formObjectExploded
      parameters:
        - {name: q, in: query, schema: {type: object, additionalProperties: false, properties: {x: {type: string}}}}
  /space-object:
    get:
      operationId: spaceObject
      parameters:
        - name: q
          in: query
          style: spaceDelimited
          explode: false
          schema: {type: object, additionalProperties: false, properties: {x: {type: string}}}
  /pipe-object:
    get:
      operationId: pipeObject
      parameters:
        - name: q
          in: query
          style: pipeDelimited
          explode: false
          schema: {type: object, additionalProperties: false, properties: {x: {type: string}}}
  /deep-object:
    get:
      operationId: deepObject
      parameters:
        - name: filter
          in: query
          style: deepObject
          explode: true
          schema: {type: object, additionalProperties: false, properties: {x: {type: string}}}
  /deep-array:
    get:
      operationId: deepArray
      parameters:
        - name: filter
          in: query
          style: deepObject
          explode: true
          schema:
            type: object
            additionalProperties: false
            properties: {x: {type: array, items: {type: string}}}
  /dynamic-form:
    get:
      operationId: dynamicForm
      parameters:
        - {name: filter, in: query, schema: {type: object}}
  /dynamic-form-named:
    get:
      operationId: dynamicFormNamed
      parameters:
        - {name: filter, in: query, explode: false, schema: {type: object, additionalProperties: true}}
  /dynamic-space:
    get:
      operationId: dynamicSpace
      parameters:
        - name: filter
          in: query
          style: spaceDelimited
          explode: false
          schema: {type: object, additionalProperties: {}}
  /dynamic-pipe:
    get:
      operationId: dynamicPipe
      parameters:
        - {name: filter, in: query, style: pipeDelimited, explode: false, schema: {type: object}}
  /dynamic-deep:
    get:
      operationId: dynamicDeep
      parameters:
        - name: filter
          in: query
          style: deepObject
          explode: true
          schema:
            type: object
            additionalProperties: {allOf: [{type: number}, {type: integer, minimum: 2}]}
  /dynamic-empty:
    get:
      operationId: dynamicEmpty
      parameters:
        - name: filter
          in: query
          schema:
            type: object
            additionalProperties: {type: string, allOf: [{type: integer}]}
  /json-content:
    get:
      operationId: jsonContent
      parameters:
        - {name: q, in: query, content: {'Application/JSON; charset=utf-8': {}}}
  /json-content-explicit:
    get:
      operationId: jsonContentExplicit
      parameters:
        - {name: q, in: query, content: {application/json: {schema: {}}}}
  /json-content-required:
    get:
      operationId: jsonContentRequired
      parameters:
        - {name: q, in: query, required: true, content: {application/json: {}}}
  /json-content-constrained:
    get:
      operationId: jsonContentConstrained
      parameters:
        - {name: q, in: query, content: {application/json: {schema: {type: integer, minimum: 2}}}}
`)
	require.NoError(t, Generate(output, "generatequeryparityfixture", spec, validation.PatternOptions()))

	probe := []byte(`package generatequeryparityfixture

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/validation"
)

func TestGeneratedRuntimeParity(t *testing.T) {
	_, runtimeDecoders, err := validation.Parse(openAPI)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		operationID   string
		rawQuery      string
		expected      string
		errorContains string
	}{
		{operationID: "primitive", rawQuery: "q=value", expected: "{\"q\":\"value\"}"},
		{operationID: "formArrayRepeated", rawQuery: "q=a&q=b", expected: "{\"q\":[\"a\",\"b\"]}"},
		{operationID: "formArrayDelimited", rawQuery: "q=a%2Cb,c", expected: "{\"q\":[\"a,b\",\"c\"]}"},
		{operationID: "spaceArray", rawQuery: "q=a+b", expected: "{\"q\":[\"a\",\"b\"]}"},
		{operationID: "pipeArray", rawQuery: "q=a%7Cb", expected: "{\"q\":[\"a\",\"b\"]}"},
		{operationID: "pipeArray", rawQuery: "q=a%257Cb%7Cc", expected: "{\"q\":[\"a%7Cb\",\"c\"]}"},
		{operationID: "pipeArray", rawQuery: "q=a|b", errorContains: "pipeDelimited separator"},
		{operationID: "formObjectNamed", rawQuery: "q=x,a", expected: "{\"q\":{\"x\":\"a\"}}"},
		{operationID: "formObjectExploded", rawQuery: "x=a", expected: "{\"q\":{\"x\":\"a\"}}"},
		{operationID: "spaceObject", rawQuery: "q=x+a", expected: "{\"q\":{\"x\":\"a\"}}"},
		{operationID: "pipeObject", rawQuery: "q=x%7Ca", expected: "{\"q\":{\"x\":\"a\"}}"},
		{operationID: "pipeObject", rawQuery: "q=x|a", errorContains: "pipeDelimited separator"},
		{operationID: "pipeObject", rawQuery: "q=x%7Ca%7Cx", errorContains: "name/value pairs"},
		{operationID: "deepObject", rawQuery: "filter%5Bx%5D=a", expected: "{\"filter\":{\"x\":\"a\"}}"},
		{
			operationID: "deepArray", rawQuery: "filter%5Bx%5D=a&filter%5Bx%5D=b",
			expected: "{\"filter\":{\"x\":[\"a\",\"b\"]}}",
		},
		{operationID: "dynamicForm", rawQuery: "a=1&b=true", expected: "{\"filter\":{\"a\":\"1\",\"b\":\"true\"}}"},
		{operationID: "dynamicForm", rawQuery: "a%5Bb%5D=1", expected: "{\"filter\":{\"a[b]\":\"1\"}}"},
		{
			operationID: "dynamicFormNamed", rawQuery: "filter=a,1,b,true",
			expected: "{\"filter\":{\"a\":\"1\",\"b\":\"true\"}}",
		},
		{operationID: "dynamicSpace", rawQuery: "filter=a+1+b+true", expected: "{\"filter\":{\"a\":\"1\",\"b\":\"true\"}}"},
		{
			operationID: "dynamicPipe", rawQuery: "filter=a%7C1%7Cb%7Ctrue",
			expected: "{\"filter\":{\"a\":\"1\",\"b\":\"true\"}}",
		},
		{operationID: "dynamicDeep", rawQuery: "filter%5Bvalue%5D=2.0", expected: "{\"filter\":{\"value\":2}}"},
		{operationID: "dynamicDeep", rawQuery: "filter%5Bvalue%5D=1", errorContains: "minimum"},
		{operationID: "dynamicDeep", rawQuery: "filter%5Bvalue%5D=2&filter%5Bvalue%5D=3", errorContains: "duplicate"},
		{operationID: "dynamicDeep", rawQuery: "filter[value]=2", errorContains: "canonical"},
		{operationID: "dynamicDeep", rawQuery: "unrelated[raw]=2", errorContains: "canonical"},
		{operationID: "dynamicDeep", rawQuery: "filter%5D=2", errorContains: "malformed"},
		{operationID: "dynamicEmpty", rawQuery: "", expected: "{}"},
		{operationID: "dynamicEmpty", rawQuery: "value=x", errorContains: "validate query"},
		{operationID: "jsonContent", rawQuery: "q=null", expected: "{\"q\":null}"},
		{operationID: "jsonContent", rawQuery: "q=true", expected: "{\"q\":true}"},
		{operationID: "jsonContent", rawQuery: "q=1.25", expected: "{\"q\":1.25}"},
		{operationID: "jsonContent", rawQuery: "q=%22value%22", expected: "{\"q\":\"value\"}"},
		{operationID: "jsonContent", rawQuery: "q=%5B1%2Ctrue%5D", expected: "{\"q\":[1,true]}"},
		{operationID: "jsonContent", rawQuery: "q=%7B%22x%22%3A1%7D", expected: "{\"q\":{\"x\":1}}"},
		{operationID: "jsonContent", rawQuery: "", expected: "{}"},
		{operationID: "jsonContent", rawQuery: "q=true&q=true", errorContains: "duplicate JSON content"},
		{operationID: "jsonContent", rawQuery: "q=true%20false", errorContains: "invalid character"},
		{operationID: "jsonContentExplicit", rawQuery: "q=null", expected: "{\"q\":null}"},
		{operationID: "jsonContentExplicit", rawQuery: "q=true", expected: "{\"q\":true}"},
		{operationID: "jsonContentExplicit", rawQuery: "q=1.25", expected: "{\"q\":1.25}"},
		{operationID: "jsonContentExplicit", rawQuery: "q=%22value%22", expected: "{\"q\":\"value\"}"},
		{operationID: "jsonContentExplicit", rawQuery: "q=%5B1%2Ctrue%5D", expected: "{\"q\":[1,true]}"},
		{operationID: "jsonContentExplicit", rawQuery: "q=%7B%22x%22%3A1%7D", expected: "{\"q\":{\"x\":1}}"},
		{operationID: "jsonContentRequired", rawQuery: "", errorContains: "required parameter is absent"},
		{operationID: "jsonContentConstrained", rawQuery: "q=2", expected: "{\"q\":2}"},
		{operationID: "jsonContentConstrained", rawQuery: "q=1", errorContains: "minimum"},
	}

	for _, test := range tests {
		t.Run(test.operationID, func(t *testing.T) {
			input := &url.URL{RawQuery: test.rawQuery}
			generated, generatedErr := queryDecoders[test.operationID].Decode(input)
			runtime, runtimeErr := runtimeDecoders[test.operationID].Decode(input)
			if fmt.Sprint(generatedErr) != fmt.Sprint(runtimeErr) {
				t.Fatalf("error mismatch: generated %v runtime %v", generatedErr, runtimeErr)
			}
			if test.errorContains != "" {
				if generatedErr == nil || !strings.Contains(generatedErr.Error(), test.errorContains) {
					t.Fatalf("generated error %v does not contain %q", generatedErr, test.errorContains)
				}

				return
			}
			if generatedErr != nil {
				t.Fatalf("unexpected matching errors: generated %v runtime %v", generatedErr, runtimeErr)
			}
			if string(generated) != string(runtime) || string(generated) != test.expected {
				t.Fatalf("result mismatch: generated %s runtime %s expected %s", generated, runtime, test.expected)
			}
		})
	}
}
`)
	require.NoError(t, os.WriteFile(filepath.Join(output, "probe_test.go"), probe, 0o644))

	command := exec.CommandContext(
		t.Context(), "go", "test", "./pkg/"+filepath.Base(output), "-run", "TestGeneratedRuntimeParity",
	)
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGenerateSchemaLessJSONRequestBodySuite verifies generated all-JSON suite parity.
func TestGenerateSchemaLessJSONRequestBodySuite(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-schema-less-body-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(output)) })

	spec := []byte(`openapi: 3.0.3
paths:
  /absent:
    post:
      operationId: absentSchema
      requestBody:
        content:
          application/json: {}
  /explicit:
    post:
      operationId: explicitEmptySchema
      requestBody:
        content:
          application/json:
            schema: {}
  /required:
    post:
      operationId: requiredAbsentSchema
      requestBody:
        required: true
        content:
          application/json: {}`)
	require.NoError(t, Generate(output, "generateschemalessbody", spec, validation.PatternOptions()))

	command := exec.CommandContext(
		t.Context(),
		"go",
		"test",
		"./pkg/"+filepath.Base(output),
		"-run",
		"^TestValidations$",
	)
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGenerateRejectsUnsafeOperationIdentifiers checks generated package-scope name conflicts.
func TestGenerateRejectsUnsafeOperationIdentifiers(t *testing.T) {
	t.Parallel()

	for _, operationID := range []string{
		"validations",
		"init",
		"_",
		"validation",
		"openAPI",
		"queryDecoders",
		"mustQueryDecoder",
		"TestValidations",
		"request/path",
		"type",
		"json",
		"patternvalidator",
		"jsonvalue",
		"errors",
		"testing",
		"testgenerator",
		"string",
		"byte",
		"error",
		"true",
	} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			output := filepath.Join(t.TempDir(), "output")
			spec := fmt.Appendf(nil, `
openapi: 3.0.3
info: {title: generated, version: "1"}
paths:
  /request:
    post:
      operationId: %q
      requestBody:
        content:
          application/json:
            schema: {type: string}
`, operationID)
			err := Generate(output, "example", spec, validation.PatternOptions())
			require.ErrorContains(t, err, fmt.Sprintf("operation ID %q", operationID))

			_, statErr := os.Stat(output)
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

// TestGeneratedPatternValidationMatchesRuntimeOptions covers all built-in setting combinations.
func TestGeneratedPatternValidationMatchesRuntimeOptions(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	specForPattern := func(pattern string) []byte {
		return fmt.Appendf(nil, `openapi: 3.0.3
info: {title: pattern parity, version: "1"}
paths:
  /request:
    post:
      operationId: patternRequest
      requestBody:
        content:
          application/json:
            schema: {type: string, pattern: %q}
      responses:
        '204': {description: empty}
`, pattern)
	}

	tests := []struct {
		name    string
		options []patternvalidator.Option
		reject  bool
		useRE2  bool
	}{
		{name: "default"},
		{name: "strict", options: []patternvalidator.Option{patternvalidator.RejectNonASCII}, reject: true},
		{name: "raw", options: []patternvalidator.Option{patternvalidator.UseRE2}, useRE2: true},
		{
			name: "strict raw",
			options: []patternvalidator.Option{
				patternvalidator.RejectNonASCII,
				patternvalidator.UseRE2,
			},
			reject: true,
			useRE2: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pattern := "^.$"
			probeBody := `"é"`

			probeAccepted := !test.reject
			if test.useRE2 {
				pattern = `(?m)^a$`
				probeBody = `"a"`
				probeAccepted = true
			}

			spec := specForPattern(pattern)
			composite := validation.PatternOptions(test.options...)
			runtime, _, err := validation.Parse(spec, composite)
			require.NoError(t, err)

			runtimePattern := runtime["patternRequest"].StringValidation.CompiledPattern
			require.Equal(t, test.reject, runtimePattern.RejectsNonASCII())
			require.Equal(t, test.useRE2, runtimePattern.UsesRE2())

			output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-pattern-parity-")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(output)) })

			require.NoError(t, Generate(output, "generatepatternparity", spec, composite))

			generated, err := os.ReadFile(filepath.Join(output, "validate.go"))
			require.NoError(t, err)
			generatedTest, err := os.ReadFile(filepath.Join(output, "validate_test.go"))
			require.NoError(t, err)

			source := string(generated)
			require.Contains(t, source, "patternvalidator.MustParse")
			require.Equal(t, test.reject, strings.Contains(source, "patternvalidator.RejectNonASCII"))
			require.Equal(t, test.useRE2, strings.Contains(source, "patternvalidator.UseRE2"))

			testSource := string(generatedTest)
			require.Equal(t, test.reject, strings.Contains(testSource, "patternvalidator.RejectNonASCII"))
			require.Equal(t, test.useRE2, strings.Contains(testSource, "patternvalidator.UseRE2"))

			probe := fmt.Appendf(nil, `package generatepatternparity

import "testing"

func TestPatternSettings(t *testing.T) {
	compiled := patternRequest.StringValidation.CompiledPattern
	if compiled.RejectsNonASCII() != %t {
		t.Fatalf("RejectsNonASCII = %%t", compiled.RejectsNonASCII())
	}
	if compiled.UsesRE2() != %t {
		t.Fatalf("UsesRE2 = %%t", compiled.UsesRE2())
	}
	accepted := len(patternRequest.Validate([]byte(%q))) == 0
	if accepted != %t {
		t.Fatalf("non-ASCII acceptance = %%t", accepted)
	}
}
`, test.reject, test.useRE2, probeBody, probeAccepted)
			require.NoError(t, os.WriteFile(filepath.Join(output, "pattern_probe_test.go"), probe, 0o644))

			command := exec.CommandContext(
				t.Context(), "go", "test", "./pkg/"+filepath.Base(output),
				"-run", "^(TestPatternSettings|TestValidations)$",
			)
			command.Dir = repo
			result, err := command.CombinedOutput()
			require.NoError(t, err, string(result))
		})
	}
}

// TestGenerateRejectsNilPatternOptionBeforeWriting checks programmer misuse is safe.
func TestGenerateRejectsNilPatternOptionBeforeWriting(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{"{type: string, pattern: a}", "{type: string}"} {
		output := filepath.Join(t.TempDir(), "output")
		err := Generate(output, "example", []byte(`openapi: 3.0.3
info: {title: nil option, version: "1"}
paths:
  /request:
    post:
      operationId: request
      requestBody:
        content:
          application/json:
            schema: `+schema+`
`), nil)
		require.ErrorContains(t, err, "nil pattern option")

		_, statErr := os.Stat(output)
		require.ErrorIs(t, statErr, os.ErrNotExist)
	}
}

// TestGenerateWritesEmptyValidationMap verifies documents without JSON request bodies still generate valid tests.
func TestGenerateWritesEmptyValidationMap(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-empty-fixture-")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(output))
	})

	err = Generate(output, "generateemptyfixture", []byte(`
openapi: 3.0.3
info: {title: generated, version: "1"}
paths:
  /bodyless:
    get: {}
  /plain:
    post:
      requestBody:
        content:
          text/plain:
            schema: {type: string}
`), validation.PatternOptions())
	require.NoError(t, err)

	command := exec.CommandContext(t.Context(), "go", "test", "./pkg/"+filepath.Base(output))
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGenerateStopsBeforeWritingOnParseError checks the required failure ordering.
func TestGenerateStopsBeforeWritingOnParseError(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "output")
	err := Generate(output, "example", []byte("not openapi"), validation.PatternOptions())
	require.Error(t, err)

	_, statErr := os.Stat(output)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestGenerateExampleMatchesFixture checks the checked-in generated example.
func TestGenerateExampleMatchesFixture(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	openAPI, err := os.ReadFile(filepath.Join(repo, "resources", "openapi.yaml"))
	require.NoError(t, err)

	output := t.TempDir()
	require.NoError(t, Generate(output, "example", openAPI, validation.PatternOptions()))

	for _, name := range []string{"validate.go", "validate_test.go"} {
		actual, readErr := os.ReadFile(filepath.Join(output, name))
		require.NoError(t, readErr)

		expected, readErr := os.ReadFile(filepath.Join(repo, "pkg", "decode", "example", name))
		require.NoError(t, readErr)
		require.True(t, bytes.Equal(expected, actual), "%s is stale; run make regen", name)
	}
}

// TestRegenerateExample rewrites the example only through the explicit regen target.
func TestRegenerateExample(t *testing.T) { //nolint:paralleltest // This test explicitly rewrites a shared fixture.
	if os.Getenv("REGENERATE") != "1" {
		t.Skip("set REGENERATE=1")
	}

	repo := repoRoot(t)
	openAPI, err := os.ReadFile(filepath.Join(repo, "resources", "openapi.yaml"))
	require.NoError(t, err)

	require.NoError(t, Generate(
		filepath.Join(repo, "pkg", "decode", "example"),
		"example",
		openAPI,
		validation.PatternOptions(),
	))
}

// repoRoot returns the repository root for generator tests.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	return root
}
