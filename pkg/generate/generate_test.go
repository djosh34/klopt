package generate

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// GetRepoRoot supports generator tests.
func GetRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}

		parent := filepath.Dir(wd)
		require.NotEqual(t, wd, parent)

		wd = parent
	}
}

// exampleOpenAPIPath supports generator tests.
func exampleOpenAPIPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(GetRepoRoot(t), "resources", "openapi.yaml")
}

// TestGenerateExample exercises the named generator behavior.
func TestGenerateExample(t *testing.T) {
	t.Parallel()

	openapiExamplePath := exampleOpenAPIPath(t)
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	generateOutputDir := t.TempDir()

	err = GenerateWithPathError(t, generateContext, generateOutputDir)
	require.NoError(t, err)
}

// SharedGenerateExampleMatchesFixture supports generator tests.
func SharedGenerateExampleMatchesFixture(t *testing.T, regen bool) {
	t.Helper()

	repoRoot := GetRepoRoot(t)
	exampleDir := filepath.Join(repoRoot, "pkg", "decode", "example")
	openapiExamplePath := exampleOpenAPIPath(t)

	generateOutputDir := filepath.Join(t.TempDir(), "example_gen")
	if regen {
		generateOutputDir = filepath.Join(repoRoot, "pkg", "decode", "example_gen")
	}

	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = os.MkdirAll(generateOutputDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(generateOutputDir, "stale.go"), []byte("package stale\n"), 0o644)
	require.NoError(t, err)

	err = GenerateWithPathError(t, generateContext, generateOutputDir)
	require.NoError(t, err)

	requireSameFiles(t, exampleDir, generateOutputDir, []string{
		"decode.go",
		"decode_tests",
	}, regen)
}

// TestGenerateExampleMatchesFixture_NoRegen exercises the named generator behavior.
func TestGenerateExampleMatchesFixture_NoRegen(t *testing.T) {
	t.Parallel()

	SharedGenerateExampleMatchesFixture(t, false)
}

// TestGenerateExampleMatchesFixture_Regen exercises the named generator behavior.
//
//nolint:paralleltest // Direct regeneration intentionally updates the checked-in fixture.
func TestGenerateExampleMatchesFixture_Regen(t *testing.T) {
	name := t.Name()
	t.Log(name)

	if len(os.Args) == 0 {
		return
	}

	lastArg := os.Args[len(os.Args)-1]

	var allowedLastArgs []string

	allowedLastArgs = append(allowedLastArgs, fmt.Sprintf("^\\Q%s\\E$", name))
	allowedLastArgs = append(allowedLastArgs, fmt.Sprintf("-test.run=^\\Q%s\\E$", name))

	for _, arg := range allowedLastArgs {
		if arg == lastArg {
			t.Log("Running Regen.....")
			SharedGenerateExampleMatchesFixture(t, true)

			return
		}
	}

	t.Log("Last Arg: ", lastArg)
	t.Skip("Intentionally no regen, when not ran directly")
}

// TestGeneratePopulatesOperationsMap exercises the named generator behavior.
func TestGeneratePopulatesOperationsMap(t *testing.T) {
	t.Parallel()

	openapiExamplePath := exampleOpenAPIPath(t)
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse", "refObject", "stringNoFormatNullable")
	require.NoError(t, err)

	operations, err := generateContext.JSONRequestBodyModelSchemas()
	require.NoError(t, err)

	optionalNotNullableString := &StringSchema{
		BaseSchema: BaseSchema{Name: "ObjectKeysAdditionalPropertiesFalseOptionalNotNullableString"},
	}
	optionalNullableString := &StringSchema{
		BaseSchema: BaseSchema{Name: "ObjectKeysAdditionalPropertiesFalseOptionalNullableString", Nullable: true},
	}
	requiredNotNullableString := &StringSchema{
		BaseSchema: BaseSchema{Name: "ObjectKeysAdditionalPropertiesFalseRequiredNotNullableString"},
	}
	requiredNullableString := &StringSchema{
		BaseSchema: BaseSchema{Name: "ObjectKeysAdditionalPropertiesFalseRequiredNullableString", Nullable: true},
	}
	refOptionalBool := &BoolSchema{
		BaseSchema: BaseSchema{Name: "RefObjectRefOptionalBool", Nullable: true},
	}
	refRequiredString := &StringSchema{
		BaseSchema: BaseSchema{Name: "RefObjectRefRequiredString"},
	}

	require.ElementsMatch(t, []Schema{
		&ObjectSchema{
			BaseSchema:           BaseSchema{Name: "ObjectKeysAdditionalPropertiesFalse"},
			AdditionalProperties: false,
			Properties: []ObjectFieldContext{
				{
					PropertyName: "optionalNotNullableString",
					Schema:       optionalNotNullableString,
				},
				{
					PropertyName: "optionalNullableString",
					Schema:       optionalNullableString,
				},
				{
					PropertyName: "requiredNotNullableString",
					Schema:       requiredNotNullableString,
					Required:     true,
				},
				{
					PropertyName: "requiredNullableString",
					Schema:       requiredNullableString,
					Required:     true,
				},
			},
		},
		optionalNotNullableString,
		optionalNullableString,
		requiredNotNullableString,
		requiredNullableString,
		&ObjectSchema{
			BaseSchema:           BaseSchema{Name: "RefObject"},
			AdditionalProperties: false,
			Properties: []ObjectFieldContext{
				{
					PropertyName: "refOptionalBool",
					Schema:       refOptionalBool,
				},
				{
					PropertyName: "refRequiredString",
					Schema:       refRequiredString,
					Required:     true,
				},
			},
		},
		refOptionalBool,
		refRequiredString,
		&StringSchema{
			BaseSchema: BaseSchema{Name: "StringNoFormatNullable", Nullable: true},
		},
	}, operations)
}

// TestStringSchemaGenerateRequiredNotNullableString exercises the named generator behavior.
func TestStringSchemaGenerateRequiredNotNullableString(t *testing.T) {
	t.Parallel()

	schema := &StringSchema{
		BaseSchema: BaseSchema{Name: "RequiredNotNullableString"},
	}

	generated, err := schema.Generate()
	require.NoError(t, err)

	require.Equal(t, `// RequiredNotNullableString is generated.
type RequiredNotNullableString string

var _ json.Unmarshaler = new(RequiredNotNullableString)

// UnmarshalJSON decodes JSON into the model.
func (s *RequiredNotNullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return NullForNotNullableStringError
	}

	var value string

	err := json.Unmarshal(data, &value)
	if err != nil {
		return NonStringForStringSchemaError
	}



	*s = RequiredNotNullableString(value)

	return nil
}

// jsonFields returns fields promoted when the model is embedded in allOf.
func (s RequiredNotNullableString) jsonFields() []jsonField {
	return []jsonField{requiredJSONField("RequiredNotNullableString", s)}
}
`, generated)
}

// TestAllOfSchemaGenerate exercises the named generator behavior.
func TestAllOfSchemaGenerate(t *testing.T) {
	t.Parallel()

	schema := &AllOfSchema{
		BaseSchema: BaseSchema{Name: "AllOfObject"},
		Schemas: []Schema{
			&ObjectSchema{BaseSchema: BaseSchema{Name: "AllOfObjectFirst"}},
			&ObjectSchema{BaseSchema: BaseSchema{Name: "AllOfObjectSecond"}},
		},
	}

	generated, err := schema.Generate()
	require.NoError(t, err)

	require.Equal(t, `// AllOfObject is generated.
type AllOfObject struct {
	AllOfObjectFirst
	AllOfObjectSecond
}

var (
	_ json.Unmarshaler = (*AllOfObject)(nil)
	_ json.Marshaler = AllOfObject{}
)

// UnmarshalJSON decodes JSON into every allOf member.
func (a *AllOfObject) UnmarshalJSON(data []byte) error {
	var errs []error
	if err := a.AllOfObjectFirst.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}


	if err := a.AllOfObjectSecond.UnmarshalJSON(data); err != nil {
		errs = append(errs, err)
	}


	return errors.Join(errs...)
}

// MarshalJSON encodes every member as one JSON object.
func (a AllOfObject) MarshalJSON() ([]byte, error) {
	return marshalJSONFields(a.jsonFields())
}

// jsonFields returns the JSON fields promoted by the allOf model.
func (a AllOfObject) jsonFields() []jsonField {
	var fields []jsonField

fields = appendEmbeddedJSONFields(fields, a.AllOfObjectFirst)
fields = appendEmbeddedJSONFields(fields, a.AllOfObjectSecond)


	return fields
}
`, generated)
}

// TestSchemaFromOpenAPISchemaAllOf exercises the named generator behavior.
func TestSchemaFromOpenAPISchemaAllOf(t *testing.T) {
	t.Parallel()

	schema, err := SchemaFromOpenAPISchema(&openapi3.Schema{
		AllOf: openapi3.SchemaRefs{
			{
				Value: &openapi3.Schema{
					Type:     &openapi3.Types{openapi3.TypeObject},
					Required: []string{"first"},
					Properties: openapi3.Schemas{
						"first": &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &openapi3.Types{openapi3.TypeString}},
						},
					},
				},
			},
			{
				Value: &openapi3.Schema{
					Type:     &openapi3.Types{openapi3.TypeObject},
					Required: []string{"second"},
					Properties: openapi3.Schemas{
						"second": &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &openapi3.Types{openapi3.TypeString}},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	schema.SetTypeName("AllOfObject")
	definitions, err := namedSchemaDefinitions(schema)
	require.NoError(t, err)

	first := &StringSchema{
		BaseSchema: BaseSchema{Name: "AllOfObjectAllOf1First"},
	}
	allOf1 := &ObjectSchema{
		BaseSchema:           BaseSchema{Name: "AllOfObjectAllOf1"},
		AdditionalProperties: true,
		Properties: []ObjectFieldContext{
			{
				PropertyName: "first",
				Schema:       first,
				Required:     true,
			},
		},
	}
	second := &StringSchema{
		BaseSchema: BaseSchema{Name: "AllOfObjectAllOf2Second"},
	}
	allOf2 := &ObjectSchema{
		BaseSchema:           BaseSchema{Name: "AllOfObjectAllOf2"},
		AdditionalProperties: true,
		Properties: []ObjectFieldContext{
			{
				PropertyName: "second",
				Schema:       second,
				Required:     true,
			},
		},
	}

	require.Equal(t, []Schema{
		&AllOfSchema{
			BaseSchema: BaseSchema{Name: "AllOfObject"},
			Schemas:    []Schema{allOf1, allOf2},
		},
		allOf1,
		first,
		allOf2,
		second,
	}, definitions)
}

// TestObjectSchemaGenerateObjectKeysAdditionalPropertiesFalse exercises the named generator behavior.
func TestObjectSchemaGenerateObjectKeysAdditionalPropertiesFalse(t *testing.T) {
	t.Parallel()

	schema := &ObjectSchema{
		BaseSchema:           BaseSchema{Name: "ObjectKeysAdditionalPropertiesFalse"},
		AdditionalProperties: false,
		Properties: []ObjectFieldContext{
			{
				PropertyName: "optionalNotNullableString",
				Schema: &StringSchema{
					BaseSchema: BaseSchema{Name: "OptionalNotNullableString"},
				},
			},
			{
				PropertyName: "optionalNullableString",
				Schema: &StringSchema{
					BaseSchema: BaseSchema{Name: "OptionalNullableString", Nullable: true},
				},
			},
			{
				PropertyName: "requiredNotNullableString",
				Required:     true,
				Schema: &StringSchema{
					BaseSchema: BaseSchema{Name: "RequiredNotNullableString"},
				},
			},
			{
				PropertyName: "requiredNullableString",
				Required:     true,
				Schema: &StringSchema{
					BaseSchema: BaseSchema{Name: "RequiredNullableString", Nullable: true},
				},
			},
		},
	}

	generated, err := schema.Generate()
	require.NoError(t, err)

	require.Contains(t, generated, "type ObjectKeysAdditionalPropertiesFalse struct {")
	require.Contains(t, generated, `requiredObjectProperty("requiredNullableString", &o.RequiredNullableString)`)
	require.Contains(t, generated, "return marshalJSONFields(o.jsonFields())")
	require.NotContains(t, generated, "switch name")
	require.NotContains(t, generated, "`json:")
}

// TestGeneratedAllOfMarshalPrefersAnonymousScalarOverPromotedObjectField preserves encoding/json field depth.
func TestGeneratedAllOfMarshalPrefersAnonymousScalarOverPromotedObjectField(t *testing.T) {
	t.Parallel()

	operation := operationWithContent(
		"marshalDominantScalarAllOf",
		openapi3.NewContentWithJSONSchema(&openapi3.Schema{
			AllOf: openapi3.SchemaRefs{
				{Value: openapi3.NewStringSchema()},
				{Value: &openapi3.Schema{
					Type:     &openapi3.Types{openapi3.TypeObject},
					Required: []string{"MarshalDominantScalarAllOfAllOf1"},
					Properties: openapi3.Schemas{
						"MarshalDominantScalarAllOfAllOf1": {
							Value: openapi3.NewStringSchema(),
						},
					},
				}},
			},
		}),
	)
	generateContext := &GenerateContext{
		Document: &openapi3.T{
			Paths: openapi3.NewPaths(openapi3.WithPath(
				"/marshal-dominant-scalar-all-of",
				&openapi3.PathItem{Post: operation},
			)),
		},
		OpenAPISource: []byte("{}"),
	}

	files, err := generateContext.GenerateInMemory()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "models.go"), files["models.go"], fileMode))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "models_test.go"),
		[]byte(`package example

import (
	"encoding/json"
	"testing"
)

func TestMarshalDominantField(t *testing.T) {
	model := MarshalDominantScalarAllOf{
		MarshalDominantScalarAllOfAllOf1: "scalar",
		MarshalDominantScalarAllOfAllOf2: MarshalDominantScalarAllOfAllOf2{
			MarshalDominantScalarAllOfAllOf1: "object",
		},
	}

	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"MarshalDominantScalarAllOfAllOf1\":\"scalar\"}" {
		t.Fatalf("unexpected JSON: %s", data)
	}
}
`),
		fileMode,
	))

	command := exec.CommandContext(t.Context(), "go", "test", "-count=1", ".")
	command.Dir = dir

	command.Env = append(os.Environ(), "GO111MODULE=off")
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

// TestGeneratedAllOfMarshalPrefersTaggedObjectFieldAtSameDepth preserves encoding/json tag dominance.
func TestGeneratedAllOfMarshalPrefersTaggedObjectFieldAtSameDepth(t *testing.T) {
	t.Parallel()

	operation := operationWithContent(
		"marshalTaggedFieldAllOf",
		openapi3.NewContentWithJSONSchema(&openapi3.Schema{
			AllOf: openapi3.SchemaRefs{
				{Value: &openapi3.Schema{
					AllOf: openapi3.SchemaRefs{{Value: openapi3.NewStringSchema()}},
				}},
				{Value: &openapi3.Schema{
					Type:     &openapi3.Types{openapi3.TypeObject},
					Required: []string{"MarshalTaggedFieldAllOfAllOf1AllOf1"},
					Properties: openapi3.Schemas{
						"MarshalTaggedFieldAllOfAllOf1AllOf1": {
							Value: openapi3.NewStringSchema(),
						},
					},
				}},
			},
		}),
	)
	generateContext := &GenerateContext{
		Document: &openapi3.T{
			Paths: openapi3.NewPaths(openapi3.WithPath(
				"/marshal-tagged-field-all-of",
				&openapi3.PathItem{Post: operation},
			)),
		},
		OpenAPISource: []byte("{}"),
	}

	files, err := generateContext.GenerateInMemory()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "models.go"), files["models.go"], fileMode))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "models_test.go"),
		[]byte(`package example

import (
	"encoding/json"
	"testing"
)

type BaselineField string

type BaselineNested struct {
	BaselineField
}

type BaselineObject struct {
	Value string `+"`json:\"BaselineField\"`"+`
}

type BaselineRoot struct {
	BaselineNested
	BaselineObject
}

func TestMarshalTaggedField(t *testing.T) {
	baseline, err := json.Marshal(BaselineRoot{
		BaselineNested: BaselineNested{BaselineField: "scalar"},
		BaselineObject: BaselineObject{Value: "object"},
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := "{\"BaselineField\":\"object\"}"
	if string(baseline) != expected {
		t.Fatalf("unexpected encoding/json baseline: %s", baseline)
	}

	model := MarshalTaggedFieldAllOf{
		MarshalTaggedFieldAllOfAllOf1: MarshalTaggedFieldAllOfAllOf1{
			MarshalTaggedFieldAllOfAllOf1AllOf1: "scalar",
		},
		MarshalTaggedFieldAllOfAllOf2: MarshalTaggedFieldAllOfAllOf2{
			MarshalTaggedFieldAllOfAllOf1AllOf1: "object",
		},
	}

	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	expected = "{\"MarshalTaggedFieldAllOfAllOf1AllOf1\":\"object\"}"
	if string(data) != expected {
		t.Fatalf("unexpected generated JSON: %s", data)
	}
}
`),
		fileMode,
	))

	command := exec.CommandContext(t.Context(), "go", "test", "-count=1", ".")
	command.Dir = dir

	command.Env = append(os.Environ(), "GO111MODULE=off")
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

// TestFilterOperationsKeepsOnlyRequestedOperation exercises the named generator behavior.
func TestFilterOperationsKeepsOnlyRequestedOperation(t *testing.T) {
	t.Parallel()

	openapiExamplePath := exampleOpenAPIPath(t)
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse")
	require.NoError(t, err)

	require.Equal(
		t,
		[]string{"/object-keys-additional-properties-false"},
		generateContext.Document.Paths.InMatchingOrder(),
	)
	operation := generateContext.Document.Paths.Value("/object-keys-additional-properties-false").Post
	require.NotNil(t, operation)
	require.Equal(t, "objectKeysAdditionalPropertiesFalse", operation.OperationID)
}

// TestFilterOperationsReturnsErrorWhenOperationMissing exercises the named generator behavior.
func TestFilterOperationsReturnsErrorWhenOperationMissing(t *testing.T) {
	t.Parallel()

	openapiExamplePath := exampleOpenAPIPath(t)
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("notAnOperation")
	require.ErrorContains(t, err, "operation not found: [notAnOperation]")
}

// requireSameFiles supports generator tests.
func requireSameFiles(t *testing.T, expectedDir string, actualDir string, exceptions []string, regen bool) {
	t.Helper()

	exceptionSet := make(map[string]struct{}, len(exceptions))
	for _, exception := range exceptions {
		exceptionSet[filepath.ToSlash(filepath.Clean(exception))] = struct{}{}
	}

	expectedFiles := comparableFiles(t, expectedDir, exceptionSet)
	actualFiles := comparableFiles(t, actualDir, exceptionSet)

	require.Equal(t, expectedFiles, actualFiles)

	for _, rel := range slices.Sorted(maps.Keys(expectedFiles)) {
		expectedFilePath := filepath.Join(expectedDir, filepath.FromSlash(rel))
		actualFilePath := filepath.Join(actualDir, filepath.FromSlash(rel))

		expected, err := os.ReadFile(expectedFilePath)
		require.NoError(t, err)

		actual, err := os.ReadFile(actualFilePath)
		require.NoError(t, err)

		if !bytes.Equal(expected, actual) {
			if !regen {
				PrettyDiff(t, string(expected), string(actual))

				t.Fail()
			} else {
				// Write expected the bytes from actual
				originalFile, originalFileErr := os.Open(expectedFilePath)
				require.NoError(t, originalFileErr)

				stat, statErr := originalFile.Stat()
				require.NoError(t, statErr)

				writeErr := os.WriteFile(expectedFilePath, actual, stat.Mode())
				require.NoError(t, writeErr)
			}
		}
	}
}
