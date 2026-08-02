package generate

import (
	"errors"
	"fmt"
	"go/scanner"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"

	generatedexample "github.com/djosh34/klopt/pkg/decode/example"
	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"

	"github.com/stretchr/testify/require"
)

// TestExecuteTemplatePreservesUnrelatedBlankLines keeps formatting ownership inside each template.
func TestExecuteTemplatePreservesUnrelatedBlankLines(t *testing.T) {
	t.Parallel()

	templates := template.Must(template.New("source.go").Parse(`package generated

func unrelated() {

	println("keep the deliberate blank line")
}
`))

	generated, err := executeTemplate(templates, "source.go", nil)
	require.NoError(t, err)
	require.Contains(t, string(generated), "func unrelated() {\n\n\tprintln")
}

// TestRenderReturnsTemplateParseErrors verifies malformed templates return errors.
func TestRenderReturnsTemplateParseErrors(t *testing.T) {
	t.Parallel()

	files, err := renderWithTemplates(
		fstest.MapFS{"templates/validate.go.tmpl": {Data: []byte(`{{`)}},
		"generated",
		nil,
	)
	require.Error(t, err)
	require.Nil(t, files)
}

// TestRenderReturnsConstructionErrors verifies malformed decoder definitions stop rendering.
func TestRenderReturnsConstructionErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]validation.RequestValidation{
		"operation name": {"": {}},
		"query definition": {
			"query": {Query: &validation.QueryDecoder{}},
		},
		"path definition": {
			"path": {Path: &validation.PathDecoder{}},
		},
	}
	for name, requests := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			files, err := render("generated", requests)
			require.Error(t, err)
			require.Nil(t, files)
		})
	}
}

// TestExecuteTemplateReturnsExecutionError verifies template execution errors are returned.
func TestExecuteTemplateReturnsExecutionError(t *testing.T) {
	t.Parallel()

	templates := template.Must(template.New("source.go").Parse(`{{template "missing.go" .}}`))
	generated, err := executeTemplate(templates, "source.go", nil)
	require.Error(t, err)
	require.Nil(t, generated)
}

// TestGenerateInMemoryReturnsOnlyValidationSource verifies generated artifact ownership.
func TestGenerateInMemoryReturnsOnlyValidationSource(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		spec []byte
	}{
		{
			name: "request body",
			spec: []byte(`openapi: 3.0.3
paths:
  /request:
    post:
      operationId: request
      requestBody:
        content:
          application/json:
            schema: {type: string, pattern: '(?i)a'}
`),
		},
		{
			name: "no validations",
			spec: []byte(`openapi: 3.0.3
paths:
  /bodyless:
    get: {operationId: bodyless}
`),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			files, err := GenerateInMemory(
				"generated",
				test.spec,
				validation.PatternOptions(patternvalidator.UseRE2),
			)
			require.NoError(t, err)
			require.Len(t, files, 1)
			require.NotEmpty(t, files["validate.go"])
		})
	}
}

// TestGenerateInMemoryPreservesParseAndRenderErrors verifies failures remain inspectable at their source.
func TestGenerateInMemoryPreservesParseAndRenderErrors(t *testing.T) {
	t.Parallel()

	t.Run("parse", func(t *testing.T) {
		t.Parallel()

		files, err := GenerateInMemory("generated", []byte(`openapi: 3.0.3
paths:
  /request:
    post:
      operationId: request
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Missing'}
`), validation.PatternOptions())
		require.Nil(t, files)

		var referenceError *oas.ReferenceError
		require.True(t, errors.As(err, &referenceError))
	})

	t.Run("render", func(t *testing.T) {
		t.Parallel()

		files, err := GenerateInMemory("not valid", []byte(`openapi: 3.0.3
paths: {}
`), validation.PatternOptions())
		require.Nil(t, files)

		var syntaxErrors scanner.ErrorList
		require.True(t, errors.As(err, &syntaxErrors))
	})
}

// TestGenerateInMemoryIsStable verifies repeated generation returns identical source.
func TestGenerateInMemoryIsStable(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.3
paths:
  /request:
    post:
      operationId: request
      requestBody:
        content:
          application/json:
            schema: {type: string, pattern: '(?i)a'}
`)
	first, err := GenerateInMemory(
		"generatedconstruction",
		spec,
		validation.PatternOptions(patternvalidator.UseRE2),
	)
	require.NoError(t, err)
	second, err := GenerateInMemory(
		"generatedconstruction",
		spec,
		validation.PatternOptions(patternvalidator.UseRE2),
	)
	require.NoError(t, err)
	require.Equal(t, first, second)
}

// TestGeneratePreservesSharedValidationNodesWithoutSourceExplosion covers shared-reference rendering.
func TestGeneratePreservesSharedValidationNodesWithoutSourceExplosion(t *testing.T) {
	t.Parallel()

	spec := `openapi: 3.0.3
paths:
  /shared:
    post:
      operationId: shared
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Level24'}
components:
  schemas:
    Level0: {type: string}
`

	for level := 1; level <= 24; level++ {
		spec += fmt.Sprintf(
			"    Level%d: {anyOf: [{$ref: '#/components/schemas/Level%d'}, {$ref: '#/components/schemas/Level%d'}]}\n",
			level,
			level-1,
			level-1,
		)
	}

	files, err := GenerateInMemory("generatedshared", []byte(spec), validation.PatternOptions())
	require.NoError(t, err)
	require.Less(t, len(files["validate.go"]), 100_000)

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-shared-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(output)) })
	require.NoError(t, os.WriteFile(filepath.Join(output, "validate.go"), files["validate.go"], 0o644))

	probe := []byte(`package generatedshared

import "testing"

func TestSharedNodes(t *testing.T) {
	compiled := RequestValidations["shared"].Body
	for level := 24; level > 0; level-- {
		if len(compiled.AnyOfValidations) != 2 || compiled.AnyOfValidations[0] != compiled.AnyOfValidations[1] {
			t.Fatalf("level %d does not preserve its shared child", level)
		}
		compiled = compiled.AnyOfValidations[0]
	}
	if errs := RequestValidations["shared"].Body.Validate([]byte("false")); len(errs) != 1 {
		t.Fatalf("shared failing graph returned %d errors: %v", len(errs), errs)
	}
}
`)
	require.NoError(t, os.WriteFile(filepath.Join(output, "shared_test.go"), probe, 0o644))

	command := exec.CommandContext(t.Context(), "go", "test", "./pkg/"+filepath.Base(output), "-run", "TestSharedNodes")
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

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
		"            schema: {type: boolean, anyOf: [{enum: [true]}, {enum: [false]}]}",
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
		if errs := RequestValidations["alphaRequest"].Body.Validate(body); len(errs) != 0 {
			t.Fatalf("valid enum %s: %v", enumValue, errs)
		}
	}

	invalid := []byte(
		"{\"array\":[2,2],\"enum\":\"missing\",\"number\":1,\"text\":\"bad\",\"extra\":1}",
	)
	if errs := RequestValidations["alphaRequest"].Body.Validate(invalid); len(errs) == 0 {
		t.Fatal("invalid body passed")
	}
	if errs := RequestValidations["alphaRequest"].Body.Validate([]byte("null")); len(errs) != 0 {
		t.Fatalf("nullable body: %v", errs)
	}
	if errs := RequestValidations["zetaRequest"].Body.Validate([]byte("true")); len(errs) != 0 {
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
//
//nolint:dupl // Generated-package compilation probes intentionally share setup and execution.
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

	probe := []byte(`package generatequeryfixture

import (
	"net/url"
	"testing"
)

func TestGeneratedQueryDecoder(t *testing.T) {
	got, err := RequestValidations["listItems"].Query.Decode(&url.URL{RawQuery: "tags=go&tags=api"})
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

// TestGenerateWritesAtomicRequestValidation verifies generated Body, Query, and Path integration.
//
//nolint:dupl // Generated-package compilation probes intentionally share setup and execution.
func TestGenerateWritesAtomicRequestValidation(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-request-fixture-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(output)) })

	spec := []byte(`openapi: 3.0.3
paths:
  /items/{id}:
    post:
      operationId: get-item/by_id
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer}}
        - {name: q, in: query, schema: {type: string}}
      requestBody:
        content:
          application/json:
            schema: {type: boolean}
  /enum-array/{value}:
    get:
      operationId: enumArray
      parameters:
        - {name: value, in: path, required: true, schema: {enum: [[1, 2]]}}
  /anyof/{id}:
    get:
      operationId: anyOfParameters
      parameters:
        - name: id
          in: path
          required: true
          schema: {anyOf: [{type: integer, minimum: 10}, {type: string, pattern: '^7$'}]}
        - name: q
          in: query
          schema: {anyOf: [{type: integer, minimum: 10}, {type: string, pattern: '^7$'}]}
  /enum-object/{value}:
    get:
      operationId: enumObject
      parameters:
        - {name: value, in: path, required: true, explode: true, schema: {enum: [{count: 2, enabled: true}]}}
`)
	require.NoError(t, Generate(output, "generaterequestfixture", spec, validation.PatternOptions()))

	probe := []byte(`package generaterequestfixture

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/djosh34/klopt/pkg/validation"
)

func TestGeneratedRequestValidation(t *testing.T) {
	request, ok := RequestValidations["get-item/by_id"]
	if !ok || request.Body == nil || request.Query == nil || request.Path == nil {
		t.Fatalf("incomplete request validation: %#v", request)
	}
	if errs := request.Body.Validate([]byte("true")); len(errs) != 0 {
		t.Fatalf("body: %v", errs)
	}
	query, err := request.Query.Decode(&url.URL{RawQuery: "q=value"})
	if err != nil || string(query) != "{\"q\":\"value\"}" {
		t.Fatalf("query %s: %v", query, err)
	}
	path, err := request.Path.DecodePathParams(&url.URL{Path: "/items/42"})
	if err != nil || string(path) != "{\"id\":42}" {
		t.Fatalf("path %s: %v", path, err)
	}

	runtimeRequests, err := validation.Parse(openAPI)
	if err != nil {
		t.Fatal(err)
	}
	runtimePath, err := runtimeRequests["get-item/by_id"].Path.DecodePathParams(&url.URL{Path: "/items/42"})
	if err != nil || string(runtimePath) != string(path) {
		t.Fatalf("runtime path %s generated path %s: %v", runtimePath, path, err)
	}
	for _, input := range []*url.URL{{Path: "/items/not-an-integer"}, {Path: "/wrong/42"}} {
		generatedJSON, generatedErr := request.Path.DecodePathParams(input)
		runtimeJSON, runtimeErr := runtimeRequests["get-item/by_id"].Path.DecodePathParams(input)
		if generatedJSON != nil || runtimeJSON != nil || fmt.Sprint(generatedErr) != fmt.Sprint(runtimeErr) {
			t.Fatalf(
				"path error mismatch: generated (%s, %v), runtime (%s, %v)",
				generatedJSON,
				generatedErr,
				runtimeJSON,
				runtimeErr,
			)
		}
	}

	generatedAnyOf := RequestValidations["anyOfParameters"]
	runtimeAnyOf := runtimeRequests["anyOfParameters"]
	generatedPath, generatedPathErr := generatedAnyOf.Path.DecodePathParams(&url.URL{Path: "/anyof/7"})
	runtimePath, runtimePathErr := runtimeAnyOf.Path.DecodePathParams(&url.URL{Path: "/anyof/7"})
	generatedQuery, generatedQueryErr := generatedAnyOf.Query.Decode(&url.URL{RawQuery: "q=7"})
	runtimeQuery, runtimeQueryErr := runtimeAnyOf.Query.Decode(&url.URL{RawQuery: "q=7"})
	if generatedPathErr != nil || runtimePathErr != nil || generatedQueryErr != nil || runtimeQueryErr != nil ||
		string(generatedPath) != "{\"id\":\"7\"}" || string(runtimePath) != string(generatedPath) ||
		string(generatedQuery) != "{\"q\":\"7\"}" || string(runtimeQuery) != string(generatedQuery) {
		t.Fatalf(
			"anyOf parity: generated path (%s, %v), runtime path (%s, %v), generated query (%s, %v), runtime query (%s, %v)",
			generatedPath,
			generatedPathErr,
			runtimePath,
			runtimePathErr,
			generatedQuery,
			generatedQueryErr,
			runtimeQuery,
			runtimeQueryErr,
		)
	}

	for operationID, test := range map[string]struct {
		path     string
		expected string
	}{
		"enumArray":  {path: "/enum-array/1,2", expected: "{\"value\":[1,2]}"},
		"enumObject": {path: "/enum-object/count=2,enabled=true", expected: "{\"value\":{\"count\":2,\"enabled\":true}}"},
	} {
		generated, generatedErr := RequestValidations[operationID].Path.DecodePathParams(&url.URL{Path: test.path})
		runtime, runtimeErr := runtimeRequests[operationID].Path.DecodePathParams(&url.URL{Path: test.path})
		if generatedErr != nil || runtimeErr != nil ||
			string(generated) != test.expected || string(runtime) != test.expected {
			t.Fatalf(
				"%s generated (%s, %v), runtime (%s, %v), expected %s",
				operationID,
				generated,
				generatedErr,
				runtime,
				runtimeErr,
				test.expected,
			)
		}
	}
}
`)
	require.NoError(t, os.WriteFile(filepath.Join(output, "probe_test.go"), probe, 0o644))
	writeRuntimeSpec(t, output, "generaterequestfixture", spec)

	command := exec.CommandContext(
		t.Context(), "go", "test", "./pkg/"+filepath.Base(output), "-run", "^TestGeneratedRequestValidation$",
	)
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGeneratedQueryDecoderMatchesRuntimeForEveryWireKind checks generated Decode parity for the full style matrix.
//
//nolint:dupl,funlen // The embedded OpenAPI document keeps every wire case visible beside its generated-package probe.
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
  /all-of-declared:
    get:
      operationId: allOfDeclared
      parameters:
        - name: filter
          in: query
          style: deepObject
          explode: true
          schema: {allOf: [{$ref: '#/components/schemas/Declared'}]}
  /all-of-dynamic:
    get:
      operationId: allOfDynamic
      parameters:
        - name: filter
          in: query
          style: deepObject
          explode: true
          schema: {allOf: [{$ref: '#/components/schemas/Dynamic'}]}
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
components:
  schemas:
    Declared:
      allOf:
        - type: object
          additionalProperties: false
          properties: {count: {type: integer}}
    Dynamic:
      allOf:
        - type: object
          additionalProperties: {type: integer}
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
	runtimeRequests, err := validation.Parse(openAPI)
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
		{
			operationID: "allOfDeclared", rawQuery: "filter%5Bcount%5D=2",
			expected: "{\"filter\":{\"count\":2}}",
		},
		{
			operationID: "allOfDynamic", rawQuery: "filter%5Bcount%5D=2",
			expected: "{\"filter\":{\"count\":2}}",
		},
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
			generated, generatedErr := RequestValidations[test.operationID].Query.Decode(input)
			runtime, runtimeErr := runtimeRequests[test.operationID].Query.Decode(input)
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
	writeRuntimeSpec(t, output, "generatequeryparityfixture", spec)

	command := exec.CommandContext(
		t.Context(), "go", "test", "./pkg/"+filepath.Base(output), "-run", "TestGeneratedRuntimeParity",
	)
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGenerateSchemaLessJSONRequestBodySuite verifies generated all-JSON suite parity.
//
//nolint:dupl // Generated-package compilation probes intentionally share setup and execution.
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

	probe := []byte(`package generateschemalessbody

import "testing"

func TestSchemaLessBodies(t *testing.T) {
	tests := []struct {
		operationID string
		body []byte
		valid bool
	}{
		{operationID: "absentSchema", body: []byte("null"), valid: true},
		{operationID: "explicitEmptySchema", body: []byte("{\"value\":1}"), valid: true},
		{operationID: "requiredAbsentSchema", valid: false},
	}
	for _, test := range tests {
		errs := RequestValidations[test.operationID].Body.Validate(test.body)
		if (len(errs) == 0) != test.valid {
			t.Fatalf("%s validity = %t, errors = %v", test.operationID, len(errs) == 0, errs)
		}
	}
}
`)
	require.NoError(t, os.WriteFile(filepath.Join(output, "probe_test.go"), probe, 0o644))

	command := exec.CommandContext(
		t.Context(),
		"go",
		"test",
		"./pkg/"+filepath.Base(output),
		"-run",
		"^TestSchemaLessBodies$",
	)
	command.Dir = repo
	result, err := command.CombinedOutput()
	require.NoError(t, err, string(result))
}

// TestGenerateUsesSharedOperationNames verifies valid IDs are converted and malformed IDs preserve the sentinel.
func TestGenerateUsesSharedOperationNames(t *testing.T) {
	t.Parallel()

	for operationID, backingName := range map[string]string{
		"init":                 "_xinit",
		"validation":           "_x" + "validation",
		"generatedValidations": "_xgeneratedValidations",
		"make":                 "_xmake",
		"new":                  "_xnew",
		"request/path":         "request_1path",
		"get-pet":              "get_0pet",
		"get_pet":              "get__pet",
		"errors":               "errors",
	} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

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
			files, err := GenerateInMemory("example", spec, validation.PatternOptions())
			require.NoError(t, err)
			require.Contains(t, string(files["validate.go"]), "var "+backingName+" = validation.RequestValidation{")
			require.Contains(t, string(files["validate.go"]), fmt.Sprintf("%q: %s", operationID, backingName))
		})
	}

	for _, operationID := range []string{"not valid", "1request"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			output := filepath.Join(t.TempDir(), "output")
			spec := fmt.Appendf(nil, `openapi: 3.0.3
paths:
  /request:
    post: {operationId: %q}
`, operationID)
			files, err := GenerateInMemory("example", spec, validation.PatternOptions())
			require.Nil(t, files)
			require.ErrorIs(t, err, oas.ErrInvalidOperationID)

			err = Generate(output, "example", spec, validation.PatternOptions())
			require.ErrorIs(t, err, oas.ErrInvalidOperationID)

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
			runtime, err := validation.Parse(spec, composite)
			require.NoError(t, err)

			runtimePattern := runtime["patternRequest"].Body.StringValidation.CompiledPattern
			require.Equal(t, test.reject, runtimePattern.RejectsNonASCII())
			require.Equal(t, test.useRE2, runtimePattern.UsesRE2())

			output, err := os.MkdirTemp(filepath.Join(repo, "pkg"), "generate-pattern-parity-")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(output)) })

			require.NoError(t, Generate(output, "generatepatternparity", spec, composite))

			probe := fmt.Appendf(nil, `package generatepatternparity

import "testing"

func TestPatternSettings(t *testing.T) {
	compiled := patternRequest.Body.StringValidation.CompiledPattern
	if compiled.RejectsNonASCII() != %t {
		t.Fatalf("RejectsNonASCII = %%t", compiled.RejectsNonASCII())
	}
	if compiled.UsesRE2() != %t {
		t.Fatalf("UsesRE2 = %%t", compiled.UsesRE2())
	}
	accepted := len(patternRequest.Body.Validate([]byte(%q))) == 0
	if accepted != %t {
		t.Fatalf("non-ASCII acceptance = %%t", accepted)
	}
}
`, test.reject, test.useRE2, probeBody, probeAccepted)
			require.NoError(t, os.WriteFile(filepath.Join(output, "pattern_probe_test.go"), probe, 0o644))

			command := exec.CommandContext(
				t.Context(), "go", "test", "./pkg/"+filepath.Base(output),
				"-run", "^TestPatternSettings$",
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
		spec := []byte(`openapi: 3.0.3
info: {title: nil option, version: "1"}
paths:
  /request:
    post:
      operationId: request
      requestBody:
        content:
          application/json:
            schema: ` + schema + `
`)

		files, err := GenerateInMemory("example", spec, nil)
		require.Nil(t, files)
		require.ErrorIs(t, err, ErrNilPatternOption)

		err = Generate(output, "example", spec, nil)
		require.ErrorIs(t, err, ErrNilPatternOption)

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
    get: {operationId: bodyless}
  /plain:
    post:
      operationId: plain
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

// TestGenerateReturnsFilesystemErrors verifies direct directory and file publication failures remain inspectable.
func TestGenerateReturnsFilesystemErrors(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.3
paths: {}
`)

	t.Run("create directory", func(t *testing.T) {
		t.Parallel()

		blocker := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(blocker, []byte("not a directory"), 0o644))

		err := Generate(filepath.Join(blocker, "output"), "generated", spec, validation.PatternOptions())

		var pathError *os.PathError
		require.True(t, errors.As(err, &pathError))
		require.Equal(t, "mkdir", pathError.Op)
	})

	t.Run("write file", func(t *testing.T) {
		t.Parallel()

		output := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(output, "validate.go"), 0o755))

		err := Generate(output, "generated", spec, validation.PatternOptions())

		var pathError *os.PathError
		require.True(t, errors.As(err, &pathError))
		require.Equal(t, "open", pathError.Op)
	})
}

// TestGeneratedExampleAnyOfMatchesRuntime proves body, path, and query parity for the committed fixture.
func TestGeneratedExampleAnyOfMatchesRuntime(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	openAPI, err := os.ReadFile(filepath.Join(repo, "resources", "openapi.yaml"))
	require.NoError(t, err)

	runtimeRequests, err := validation.Parse(openAPI)
	require.NoError(t, err)

	generated := generatedexample.RequestValidations["anyOfBodyAndParameters"]
	runtime := runtimeRequests["anyOfBodyAndParameters"]

	for _, body := range [][]byte{[]byte(`"ab"`), []byte(`"zz"`), []byte(`"x"`), []byte(`"xz"`), []byte(`7`)} {
		generatedErrors := generated.Body.Validate(body)
		runtimeErrors := runtime.Body.Validate(body)
		require.Equal(t, fmt.Sprint(runtimeErrors), fmt.Sprint(generatedErrors), string(body))
	}

	for _, value := range []string{"7", "12", "8"} {
		generatedPath, generatedPathErr := generated.Path.DecodePathParams(&url.URL{Path: "/any-of/" + value})
		runtimePath, runtimePathErr := runtime.Path.DecodePathParams(&url.URL{Path: "/any-of/" + value})
		require.Equal(t, string(runtimePath), string(generatedPath), "path %s", value)
		require.Equal(t, fmt.Sprint(runtimePathErr), fmt.Sprint(generatedPathErr), "path %s", value)

		generatedQuery, generatedQueryErr := generated.Query.Decode(&url.URL{RawQuery: "q=" + value})
		runtimeQuery, runtimeQueryErr := runtime.Query.Decode(&url.URL{RawQuery: "q=" + value})
		require.Equal(t, string(runtimeQuery), string(generatedQuery), "query %s", value)
		require.Equal(t, fmt.Sprint(runtimeQueryErr), fmt.Sprint(generatedQueryErr), "query %s", value)
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

	exampleDir := filepath.Join(repo, "pkg", "decode", "example")
	handOwnedTest, err := os.ReadFile(filepath.Join(exampleDir, "validate_test.go"))
	require.NoError(t, err)

	require.NoError(t, Generate(
		exampleDir,
		"example",
		openAPI,
		validation.PatternOptions(),
	))

	after, err := os.ReadFile(filepath.Join(exampleDir, "validate_test.go"))
	require.NoError(t, err)
	require.Equal(t, handOwnedTest, after)
}

// writeRuntimeSpec supplies runtime Parse input to one generated-package parity probe.
func writeRuntimeSpec(t *testing.T, output string, packageName string, spec []byte) {
	t.Helper()

	source := fmt.Appendf(nil, "package %s\n\nvar openAPI = []byte(%q)\n", packageName, spec)
	require.NoError(t, os.WriteFile(filepath.Join(output, "runtime_spec_test.go"), source, 0o644))
}

// repoRoot returns the repository root for generator tests.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	return root
}
