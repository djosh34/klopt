package generate

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

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

func exampleOpenAPIPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(GetRepoRoot(t), "resources", "openapi.yaml")
}

func TestGenerateExample(t *testing.T) {
	openapiExamplePath := exampleOpenAPIPath(t)
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	generateOutputDir := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example_gen")

	err = GenerateWithPathError(t, generateContext, generateOutputDir)
	require.NoError(t, err)
}

func SharedGenerateExampleMatchesFixture(t *testing.T, regen bool) {
	t.Helper()

	repoRoot := GetRepoRoot(t)
	exampleDir := filepath.Join(repoRoot, "pkg", "decode", "example")
	openapiExamplePath := exampleOpenAPIPath(t)
	generateOutputDir := filepath.Join(repoRoot, "pkg", "decode", "example_gen")

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

func TestGenerateExampleMatchesFixture_NoRegen(t *testing.T) {
	SharedGenerateExampleMatchesFixture(t, false)
}

func TestGenerateExampleMatchesFixture_Regen(t *testing.T) {
	name := t.Name()
	fmt.Println(name)

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

func TestGeneratePopulatesOperationsMap(t *testing.T) {
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

func TestStringSchemaGenerateRequiredNotNullableString(t *testing.T) {
	schema := &StringSchema{
		BaseSchema: BaseSchema{Name: "RequiredNotNullableString"},
	}

	generated, err := schema.Generate()
	require.NoError(t, err)

	require.Equal(t, `type RequiredNotNullableString string

var _ json.Unmarshaler = new(RequiredNotNullableString)

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
`, generated)
}

func TestAllOfSchemaGenerate(t *testing.T) {
	schema := &AllOfSchema{
		BaseSchema: BaseSchema{Name: "AllOfObject"},
		Schemas: []Schema{
			&ObjectSchema{BaseSchema: BaseSchema{Name: "AllOfObjectFirst"}},
			&ObjectSchema{BaseSchema: BaseSchema{Name: "AllOfObjectSecond"}},
		},
	}

	generated, err := schema.Generate()
	require.NoError(t, err)

	require.Equal(t, `type AllOfObject struct {
	AllOfObjectFirst
	AllOfObjectSecond
}

var _ json.Unmarshaler = (*AllOfObject)(nil)

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
`, generated)
}

func TestSchemaFromOpenAPISchemaAllOf(t *testing.T) {
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

func TestObjectSchemaGenerateObjectKeysAdditionalPropertiesFalse(t *testing.T) {
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

	require.Equal(t, `type ObjectKeysAdditionalPropertiesFalse struct {
	OptionalNotNullableString *OptionalNotNullableString `+"`"+`json:"optionalNotNullableString,omitzero"`+"`"+`
	OptionalNullableString *OptionalNullableString `+"`"+`json:"optionalNullableString,omitzero"`+"`"+`
	RequiredNotNullableString RequiredNotNullableString `+"`"+`json:"requiredNotNullableString"`+"`"+`
	RequiredNullableString RequiredNullableString `+"`"+`json:"requiredNullableString"`+"`"+`
}

var _ json.Unmarshaler = (*ObjectKeysAdditionalPropertiesFalse)(nil)

func (o *ObjectKeysAdditionalPropertiesFalse) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()

	tok, err := d.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return NotAnObjectError
	}
	var hasRequiredNotNullableString bool
	var hasRequiredNullableString bool

	for d.More() {
		nameTok, nameErr := d.Token()
		if nameErr != nil {
			return nameErr
		}

		name := nameTok.(string)

		var value json.RawMessage
		err = d.Decode(&value)
		if err != nil {
			return err
		}

		switch name {
		case "optionalNotNullableString":

			var optionalNotNullableString OptionalNotNullableString
			err = json.Unmarshal(value, &optionalNotNullableString)
			if err != nil {
				return err
			}
			o.OptionalNotNullableString = &optionalNotNullableString
		case "optionalNullableString":

			var optionalNullableString OptionalNullableString
			err = json.Unmarshal(value, &optionalNullableString)
			if err != nil {
				return err
			}
			o.OptionalNullableString = &optionalNullableString
		case "requiredNotNullableString":
			hasRequiredNotNullableString = true

			var requiredNotNullableString RequiredNotNullableString
			err = json.Unmarshal(value, &requiredNotNullableString)
			if err != nil {
				return err
			}
			o.RequiredNotNullableString = requiredNotNullableString
		case "requiredNullableString":
			hasRequiredNullableString = true

			var requiredNullableString RequiredNullableString
			err = json.Unmarshal(value, &requiredNullableString)
			if err != nil {
				return err
			}
			o.RequiredNullableString = requiredNullableString
		default:
			return fmt.Errorf("%w: %s", AdditionalPropertyError, name)
		}
	}
	if _, err := d.Token(); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) != 0 {
		return NotAnObjectError
	}
	if !hasRequiredNotNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNotNullableString")
	}
	if !hasRequiredNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNullableString")
	}

	return nil
}
`, generated)
}

func TestFilterOperationsKeepsOnlyRequestedOperation(t *testing.T) {
	openapiExamplePath := exampleOpenAPIPath(t)
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse")
	require.NoError(t, err)

	require.Equal(t, []string{"/object-keys-additional-properties-false"}, generateContext.Document.Paths.InMatchingOrder())
	operation := generateContext.Document.Paths.Value("/object-keys-additional-properties-false").Post
	require.NotNil(t, operation)
	require.Equal(t, "objectKeysAdditionalPropertiesFalse", operation.OperationID)
}

func TestFilterOperationsReturnsErrorWhenOperationMissing(t *testing.T) {
	openapiExamplePath := exampleOpenAPIPath(t)
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("notAnOperation")
	require.ErrorContains(t, err, "operation not found: [notAnOperation]")
}

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
