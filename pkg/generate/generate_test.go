package generate

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

	err = generateContext.Generate(generateOutputDir)
	require.NoError(t, err)

}

func TestGenerateExampleMatchesFixture(t *testing.T) {
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

	err = generateContext.Generate(generateOutputDir)
	require.NoError(t, err)

	requireSameFiles(t, exampleDir, generateOutputDir, []string{
		"decode.go",
		"decode_tests",
		"openapi.yaml",
	})
}

func TestGeneratePopulatesOperationsMap(t *testing.T) {
	openapiExamplePath := filepath.Join(GetRepoRoot(t), "pkg", "decode", "example", "openapi.yaml")
	generateContext, err := LoadOpenapi(t.Context(), openapiExamplePath)
	require.NoError(t, err)

	err = generateContext.FilterOperations("objectKeysAdditionalPropertiesFalse", "stringNoFormatNullable")
	require.NoError(t, err)

	err = generateContext.Generate(t.TempDir())
	require.NoError(t, err)

	require.Equal(t, map[string]SchemaObject{
		"objectKeysAdditionalPropertiesFalse": ObjectContext{
			AdditionalProperties: false,
			Required: []string{
				"requiredNullableString",
				"requiredNotNullableString",
			},
			Properties: map[string]SchemaObject{
				"requiredNullableString":    StringContext{Nullable: true},
				"requiredNotNullableString": StringContext{},
				"optionalNullableString":    StringContext{Nullable: true},
				"optionalNotNullableString": StringContext{},
			},
		},
		"stringNoFormatNullable": StringContext{Nullable: true},
	}, generateContext.Operations)
}

func TestStringContextGenerateRequiredNotNullableString(t *testing.T) {
	schemaContext := StringContext{
		ContextName: "RequiredNotNullableString",
	}

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
`, schemaContext.Generate())
}

func TestObjectContextGenerateObjectKeysAdditionalPropertiesFalse(t *testing.T) {
	schemaContext := ObjectContext{
		ContextName:          "ObjectKeysAdditionalPropertiesFalse",
		AdditionalProperties: false,
		Required: []string{
			"requiredNullableString",
			"requiredNotNullableString",
		},
		Properties: map[string]SchemaObject{
			"requiredNullableString": StringContext{
				ContextName: "RequiredNullableString",
				Nullable:    true,
			},
			"requiredNotNullableString": StringContext{
				ContextName: "RequiredNotNullableString",
			},
			"optionalNullableString": StringContext{
				ContextName: "OptionalNullableString",
				Nullable:    true,
			},
			"optionalNotNullableString": StringContext{
				ContextName: "OptionalNotNullableString",
			},
		},
	}

	require.Equal(t, `type ObjectKeysAdditionalPropertiesFalse struct {
	RequiredNullableString    RequiredNullableString     `+"`"+`json:"requiredNullableString"`+"`"+`
	RequiredNotNullableString RequiredNotNullableString  `+"`"+`json:"requiredNotNullableString"`+"`"+`
	OptionalNullableString    *OptionalNullableString    `+"`"+`json:"optionalNullableString,omitzero"`+"`"+`
	OptionalNotNullableString *OptionalNotNullableString `+"`"+`json:"optionalNotNullableString,omitzero"`+"`"+`
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

	var hasRequiredNullableString bool
	var hasRequiredNotNullableString bool

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
		case "requiredNullableString":
			hasRequiredNullableString = true

			err = json.Unmarshal(value, &o.RequiredNullableString)
			if err != nil {
				return err
			}
		case "requiredNotNullableString":
			hasRequiredNotNullableString = true

			err = json.Unmarshal(value, &o.RequiredNotNullableString)
			if err != nil {
				return err
			}
		case "optionalNullableString":
			var optionalNullableString OptionalNullableString
			err = json.Unmarshal(value, &optionalNullableString)
			if err != nil {
				return err
			}
			o.OptionalNullableString = &optionalNullableString

		case "optionalNotNullableString":

			var optionalNotNullableString OptionalNotNullableString
			err = json.Unmarshal(value, &optionalNotNullableString)
			if err != nil {
				return err
			}
			o.OptionalNotNullableString = &optionalNotNullableString
		default:
			return fmt.Errorf("%w: %s", AdditionalPropertyError, name)
		}
	}

	if !hasRequiredNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNullableString")
	}
	if !hasRequiredNotNullableString {
		return fmt.Errorf("%w: %s", MissingRequiredPropertyError, "requiredNotNullableString")
	}

	return nil
}
`, schemaContext.Generate())
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

func requireSameFiles(t *testing.T, expectedDir string, actualDir string, exceptions []string) {
	t.Helper()

	exceptionSet := make(map[string]struct{}, len(exceptions))
	for _, exception := range exceptions {
		exceptionSet[filepath.ToSlash(filepath.Clean(exception))] = struct{}{}
	}

	expectedFiles := comparableFiles(t, expectedDir, exceptionSet)
	actualFiles := comparableFiles(t, actualDir, exceptionSet)

	require.Equal(t, expectedFiles, actualFiles)

	for _, rel := range slices.Sorted(maps.Keys(expectedFiles)) {
		expected, err := os.ReadFile(filepath.Join(expectedDir, filepath.FromSlash(rel)))
		require.NoError(t, err)

		actual, err := os.ReadFile(filepath.Join(actualDir, filepath.FromSlash(rel)))
		require.NoError(t, err)

		require.Equal(t, expected, actual, "file content differs: %s", rel)
	}
}

func comparableFiles(t *testing.T, root string, exceptions map[string]struct{}) map[string]struct{} {
	t.Helper()

	files := map[string]struct{}{}

	err := filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		rel = filepath.ToSlash(rel)
		if exceptedPath(rel, exceptions) {
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if dirEntry.IsDir() {
			return nil
		}

		files[rel] = struct{}{}
		return nil
	})
	require.NoError(t, err)

	return files
}

func exceptedPath(rel string, exceptions map[string]struct{}) bool {
	if _, ok := exceptions[rel]; ok {
		return true
	}

	for exception := range exceptions {
		if strings.HasPrefix(rel, exception+"/") {
			return true
		}
	}

	return false
}
