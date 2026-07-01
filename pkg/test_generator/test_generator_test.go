package testgenerator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGeneratedCasesValidateAgainstOpenAPISchema(t *testing.T) {
	openapiPath := filepath.Join("..", "..", "resources", "openapi.yaml")
	content, err := os.ReadFile(openapiPath)
	require.NoError(t, err)

	document, err := libopenapi.NewDocument(content)
	require.NoError(t, err)

	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)

	openAPIRoot := unmarshalYAMLDocument(t, content)
	validationOptions := config.NewValidationOptions(config.WithOpenAPIMode(), config.WithFormatAssertions())
	testedSchemas := 0

	for pathPair := model.Model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
		requestPath := pathPair.Key()
		pathItem := pathPair.Value()

		for operationPair := pathItem.GetOperations().First(); operationPair != nil; operationPair = operationPair.Next() {
			method := operationPair.Key()
			operation := operationPair.Value()

			if operation.RequestBody == nil || operation.RequestBody.Content == nil {
				continue
			}

			for mediaTypePair := operation.RequestBody.Content.First(); mediaTypePair != nil; mediaTypePair = mediaTypePair.Next() {
				mediaType := mediaTypePair.Key()
				openAPISchema := mediaTypePair.Value().Schema
				require.NotNil(t, openAPISchema)

				compiledSchema, err := schema_validation.CompileSchemaForValidation(
					openAPISchema.Schema(),
					schema_validation.SchemaValidationPurposeGeneric,
					validationOptions,
					3.0,
				)
				require.NoError(t, err)
				require.NotNil(t, compiledSchema)
				require.NotNil(t, compiledSchema.CompiledSchema)

				schemaNode := findSchemaNode(t, openAPIRoot, requestPath, method, mediaType)
				generatorSchema, err := decodeSchemaNode(openAPIRoot, schemaNode)
				require.NoError(t, err)

				schemaName := fmt.Sprintf("%s %s %s", strings.ToUpper(method), requestPath, mediaType)

				t.Run(fmt.Sprintf("valid %s", schemaName), func(t *testing.T) {
					testCases := generatorSchema.ValidCases()
					t.Logf("Testcases: %v", len(testCases))
					for _, testCase := range generatorSchema.ValidCases() {
						valid, validationError := validateCompiledSchemaString(t, compiledSchema, string(testCase.Value))
						require.Truef(t, valid, "expected generated case to validate, got: %s", validationError)
					}
				})
				t.Run(fmt.Sprintf("invalid %s", schemaName), func(t *testing.T) {
					testCases := generatorSchema.InvalidCases()
					t.Logf("Testcases: %v", len(testCases))
					for _, testCase := range testCases {
						valid, _ := validateCompiledSchemaString(t, compiledSchema, string(testCase.Value))
						require.False(t, valid, "expected generated case to be invalid")
					}
				})
				testedSchemas++
			}
		}
	}

	require.NotZero(t, testedSchemas)
}

func testdataOpenAPIPath(name string) string {
	return filepath.Join("testdata", name)
}

func validateCompiledSchemaString(t *testing.T, compiledSchema *schema_validation.CompiledValidationSchema, payload string) (bool, string) {
	t.Helper()

	var decoded any
	require.NoError(t, json.Unmarshal([]byte(payload), &decoded))

	// The high-level string validator skips validation when a root JSON null decodes to nil.
	err := compiledSchema.CompiledSchema.Validate(decoded)
	if err == nil {
		return true, ""
	}

	return false, err.Error()
}

func unmarshalYAMLDocument(t *testing.T, content []byte) *yaml.Node {
	t.Helper()

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal(content, &root))
	require.Len(t, root.Content, 1)

	return root.Content[0]
}

func findSchemaNode(t *testing.T, root *yaml.Node, requestPath string, method string, mediaType string) *yaml.Node {
	t.Helper()

	schemaNode := mappingValue(
		mappingValue(
			mappingValue(
				mappingValue(
					mappingValue(
						mappingValue(root, "paths"),
						requestPath,
					),
					strings.ToLower(method),
				),
				"requestBody",
			),
			"content",
		),
		mediaType,
	)
	require.NotNil(t, schemaNode)

	schemaNode = mappingValue(schemaNode, "schema")
	require.NotNil(t, schemaNode)

	return schemaNode
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}

	return nil
}
