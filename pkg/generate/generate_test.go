package generate

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

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

func TestGenerateExample(t *testing.T) {

	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	generateOutputDir := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example_gen")

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse")
	require.NoError(t, err)

	err = GenerateWithPathError(t, generateContext, generateOutputDir)
	require.NoError(t, err)

}

func SharedGenerateExampleMatchesFixture(t *testing.T, regen bool) {
	t.Helper()

	repoRoot := GetRepoRoot(t)
	exampleDir := filepath.Join(repoRoot, "pkg", "decode", "example")
	openapiExamplePath := filepath.Join(exampleDir, "openapi.yaml")
	generateOutputDir := filepath.Join(repoRoot, "pkg", "decode", "example_gen")

	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse")
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
		"openapi.yaml",
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
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse", "stringNoFormatNullable")
	require.NoError(t, err)

	operations, err := generateContext.JSONRequestBodySchemaObjects()
	require.NoError(t, err)

	require.ElementsMatch(t, []SchemaObject{
		{
			TypeName: "ObjectKeysAdditionalPropertiesFalse",
			Generatable: &ObjectContext{
				AdditionalProperties: false,
				Properties: []ObjectFieldContext{
					{
						PropertyName: "optionalNotNullableString",
						Schema: SchemaObject{
							Generatable: &StringContext{},
						},
					},
					{
						PropertyName: "optionalNullableString",
						Schema: SchemaObject{
							Generatable: &StringContext{},
							Nullable:    true,
						},
					},
					{
						PropertyName: "requiredNotNullableString",
						Schema: SchemaObject{
							Generatable: &StringContext{},
						},
						Required: true,
					},
					{
						PropertyName: "requiredNullableString",
						Schema: SchemaObject{
							Generatable: &StringContext{},
							Nullable:    true,
						},
						Required: true,
					},
				},
			},
		},
		{
			TypeName:    "StringNoFormatNullable",
			Generatable: &StringContext{},
			Nullable:    true,
		},
	}, operations)
}

func TestStringContextGenerateRequiredNotNullableString(t *testing.T) {
	schemaContext := SchemaObject{
		Generatable: &StringContext{},
		TypeName:    "RequiredNotNullableString",
	}

	generated, err := schemaContext.Generate()
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

func TestObjectContextGenerateObjectKeysAdditionalPropertiesFalse(t *testing.T) {
	schemaContext := SchemaObject{
		TypeName: "ObjectKeysAdditionalPropertiesFalse",
		Generatable: &ObjectContext{
			AdditionalProperties: false,
			Properties: []ObjectFieldContext{
				{
					PropertyName: "optionalNotNullableString",
					Schema: SchemaObject{
						Generatable: &StringContext{},
						TypeName:    "OptionalNotNullableString",
					},
				},
				{
					PropertyName: "optionalNullableString",
					Schema: SchemaObject{
						Generatable: &StringContext{},
						TypeName:    "OptionalNullableString",
						Nullable:    true,
					},
				},
				{
					PropertyName: "requiredNotNullableString",
					Required:     true,
					Schema: SchemaObject{
						Generatable: &StringContext{},
						TypeName:    "RequiredNotNullableString",
					},
				},
				{
					PropertyName: "requiredNullableString",
					Required:     true,
					Schema: SchemaObject{
						Generatable: &StringContext{},
						TypeName:    "RequiredNullableString",
						Nullable:    true,
					},
				},
			},
		},
	}

	generated, err := schemaContext.Generate()
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

		name, ok := nameTok.(string)
		if !ok {
			return NotAnObjectError
		}

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
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
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
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
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
