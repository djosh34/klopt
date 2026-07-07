package testgenerator

import (
	"encoding/json"
	"fmt"
)

type openAPIDocument struct {
	Paths map[string]openAPIPathItem `json:"paths"`
}

type openAPIPathItem struct {
	Get     *openAPIOperation `json:"get"`
	Put     *openAPIOperation `json:"put"`
	Post    *openAPIOperation `json:"post"`
	Delete  *openAPIOperation `json:"delete"`
	Options *openAPIOperation `json:"options"`
	Head    *openAPIOperation `json:"head"`
	Patch   *openAPIOperation `json:"patch"`
	Trace   *openAPIOperation `json:"trace"`
}

type openAPIOperation struct {
	OperationID string              `json:"operationId"`
	RequestBody *openAPIRequestBody `json:"requestBody"`
}

type openAPIRequestBody struct {
	Content map[string]openAPIMediaType `json:"content"`
}

type openAPIMediaType struct {
	Schema *json.RawMessage `json:"schema"`
}

func OpenAPIRequestBodySchemaNode(openAPIJSONSpec *json.RawMessage, operationID string) (*json.RawMessage, error) {
	var document openAPIDocument
	err := json.Unmarshal(*openAPIJSONSpec, &document)
	if err != nil {
		return nil, fmt.Errorf("parse openapi json spec: %w", err)
	}

	var matches []*openAPIOperation
	for _, pathItem := range document.Paths {
		for _, operation := range []*openAPIOperation{
			pathItem.Get,
			pathItem.Put,
			pathItem.Post,
			pathItem.Delete,
			pathItem.Options,
			pathItem.Head,
			pathItem.Patch,
			pathItem.Trace,
		} {
			if operation == nil {
				continue
			}
			if operation.OperationID == operationID {
				matches = append(matches, operation)
			}
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("operationId %q not found", operationID)
	case 1:
	default:
		return nil, fmt.Errorf("operationId %q found multiple times", operationID)
	}

	operation := matches[0]
	if operation.RequestBody == nil || len(operation.RequestBody.Content) == 0 {
		return nil, fmt.Errorf("operationId %q request body content type does not exist", operationID)
	}

	mediaType, ok := operation.RequestBody.Content["application/json"]
	if !ok {
		return nil, fmt.Errorf("operationId %q request body content type is not json", operationID)
	}
	if mediaType.Schema == nil || len(*mediaType.Schema) == 0 || string(*mediaType.Schema) == "null" {
		return nil, fmt.Errorf("operationId %q application/json schema does not exist", operationID)
	}

	return mediaType.Schema, nil
}
