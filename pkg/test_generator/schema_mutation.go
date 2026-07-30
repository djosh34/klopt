//nolint:cyclop,godoclint,mnd // Private mutation IDs form a fixed test matrix.
package testgenerator

import "fmt"

const generatedMutationCount = 12

func mutateGeneratedDocument(document map[string]any, mutationID int) error {
	schema, err := generatedRequestSchema(document)
	if err != nil {
		return err
	}

	switch mutationID {
	case 0:
		schema["oneOf"] = []any{generatedSchemaObject{}}
	case 1:
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{
			"$ref": "#/components/schemas/Missing",
		})
	case 2:
		schemas, schemasErr := generatedComponentSchemas(document)
		if schemasErr != nil {
			return schemasErr
		}

		schemas["Cycle"] = generatedSchemaObject{"$ref": "#/components/schemas/Cycle"}
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{
			"$ref": "#/components/schemas/Cycle",
		})
	case 3:
		schemas, schemasErr := generatedComponentSchemas(document)
		if schemasErr != nil {
			return schemasErr
		}

		schemas["CycleA"] = generatedSchemaObject{"$ref": "#/components/schemas/CycleB"}
		schemas["CycleB"] = generatedSchemaObject{"$ref": "#/components/schemas/CycleA"}
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{
			"$ref": "#/components/schemas/CycleA",
		})
	case 4:
		schema["minLength"] = "zero"
	case 5:
		schema["nullable"] = nil
	case 6:
		schema["minItems"] = -1
	case 7:
		schema["allOf"] = []any{}
	case 8:
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{"$ref": "#not-a-pointer"})
	case 9:
		schema["required"] = "property"
	case 10:
		schema["allOf"] = appendGeneratedAllOf(schema, generatedSchemaObject{"$ref": "#/info"})
	case 11:
		schema["maxLength"] = nil
	default:
		return fmt.Errorf("unknown mutation %d", mutationID)
	}

	return nil
}

func appendGeneratedAllOf(schema generatedSchemaObject, child generatedSchemaObject) []any {
	children, ok := schema["allOf"].([]generatedSchemaObject)
	if ok {
		result := make([]any, 0, len(children)+1)
		for _, existing := range children {
			result = append(result, existing)
		}

		return append(result, child)
	}

	if children, ok := schema["allOf"].([]any); ok {
		return append(append([]any(nil), children...), child)
	}

	return []any{child}
}

func generatedRequestSchema(document map[string]any) (generatedSchemaObject, error) {
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths")
	}

	path, ok := paths["/things"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things")
	}

	post, ok := path["post"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post")
	}

	requestBody, ok := post["requestBody"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post.requestBody")
	}

	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post.requestBody.content")
	}

	mediaType, ok := content["application/json"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post.requestBody.content.application/json")
	}

	schema, ok := mediaType["schema"].(generatedSchemaObject)
	if !ok {
		return nil, errorsForGeneratedPath("paths./things.post.requestBody.content.application/json.schema")
	}

	return schema, nil
}

func generatedComponentSchemas(document map[string]any) (map[string]any, error) {
	components, ok := document["components"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("components")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		return nil, errorsForGeneratedPath("components.schemas")
	}

	return schemas, nil
}

func errorsForGeneratedPath(path string) error {
	return fmt.Errorf("generated document path %s has the wrong shape", path)
}
