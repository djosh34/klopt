package testgenerator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/config"
	validatorerrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGeneratedCasesValidateAgainstOpenAPISchema(t *testing.T) {
	openapiPath := testdataOpenAPIPath("nullable_object_with_nullable_string.yaml")
	content, err := os.ReadFile(openapiPath)
	require.NoError(t, err)

	document, err := libopenapi.NewDocument(content)
	require.NoError(t, err)

	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)

	openAPIRoot := unmarshalYAMLDocument(t, content)
	validator := schema_validation.NewSchemaValidator(config.WithOpenAPIMode())
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

				var generatorSchema SchemaNode
				schemaNode := findSchemaNode(t, openAPIRoot, requestPath, method, mediaType)
				require.NoError(t, schemaNode.Decode(&generatorSchema))

				testGeneratedCases(t, validator, openAPISchema.Schema(), true, generatorSchema.ValidCases())
				testGeneratedCases(t, validator, openAPISchema.Schema(), false, generatorSchema.InvalidCases())
				testedSchemas++
			}
		}
	}

	require.NotZero(t, testedSchemas)
}

func testdataOpenAPIPath(name string) string {
	return filepath.Join("testdata", name)
}

func testGeneratedCases(
	t *testing.T,
	validator schema_validation.SchemaValidator,
	schema *base.Schema,
	wantValid bool,
	cases []Case,
) {
	t.Helper()

	for _, testCase := range cases {
		validity := "invalid"
		if wantValid {
			validity = "valid"
		}

		t.Run(fmt.Sprintf("%s %s", validity, testCase.Name), func(t *testing.T) {
			valid, validationErrors := validator.ValidateSchemaStringWithVersion(schema, string(testCase.Value), 3.0)
			if wantValid {
				require.Truef(t, valid, "expected generated case to validate, got: %s", validationErrorMessages(validationErrors))
			} else {
				require.False(t, valid, "expected generated case to be invalid")
			}
		})
	}
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

func validationErrorMessages(validationErrors []*validatorerrors.ValidationError) string {
	messages := make([]string, 0, len(validationErrors))
	for _, validationError := range validationErrors {
		messages = append(messages, validationError.Message)
		for _, schemaError := range validationError.SchemaValidationErrors {
			messages = append(messages, schemaError.Reason)
		}
	}

	return strings.Join(messages, "; ")
}
