package testgenerator

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	//nolint:depguard // OpenAPI lookup requires the test generator's internal JSON Reference resolver.
	"decode_and_validate_generator/pkg/test_generator/jsonrefs"
)

// openAPIDocument contains the path items needed for operation lookup.
type openAPIDocument struct {
	Paths map[string]json.RawMessage `json:"paths"`
}

// openAPIPathItem contains every OpenAPI 3.0.3 operation field.
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

// openAPIOperation contains the fields needed for request-body lookup.
type openAPIOperation struct {
	OperationID string           `json:"operationId"`
	RequestBody *json.RawMessage `json:"requestBody"`
}

// openAPIRequestBody contains media types keyed by content type.
type openAPIRequestBody struct {
	Content map[string]openAPIMediaType `json:"content"`
}

// openAPIMediaType contains the request schema.
type openAPIMediaType struct {
	Schema *json.RawMessage `json:"schema"`
}

// OpenAPIRequestBodySchemaNode returns the application/json request schema for operationID.
func OpenAPIRequestBodySchemaNode(openAPIJSONSpec *json.RawMessage, operationID string) (*json.RawMessage, error) {
	if openAPIJSONSpec == nil {
		return nil, errors.New("openapi json spec is nil")
	}

	if operationID == "" {
		return nil, errors.New("operationId must not be empty")
	}

	var document openAPIDocument
	if err := json.Unmarshal(*openAPIJSONSpec, &document); err != nil {
		return nil, fmt.Errorf("parse openapi json spec: %w", err)
	}

	matches, err := findOpenAPIOperations(openAPIJSONSpec, document.Paths, operationID)
	if err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("operationId %q not found", operationID)
	case 1:
	default:
		return nil, fmt.Errorf("operationId %q found multiple times", operationID)
	}

	return openAPIRequestBodySchema(openAPIJSONSpec, matches[0], operationID)
}

// findOpenAPIOperations resolves path item references and finds operationID.
func findOpenAPIOperations(
	openAPIJSONSpec *json.RawMessage,
	paths map[string]json.RawMessage,
	operationID string,
) ([]*openAPIOperation, error) {
	var matches []*openAPIOperation

	pathNames := make([]string, 0, len(paths))
	for path := range paths {
		pathNames = append(pathNames, path)
	}

	sort.Strings(pathNames)

	for _, path := range pathNames {
		if strings.HasPrefix(path, "x-") {
			continue
		}

		rawPathItem := paths[path]

		resolvedPathItem, err := jsonrefs.ResolveReference(openAPIJSONSpec, &rawPathItem)
		if err != nil {
			return nil, fmt.Errorf("resolve openapi path item %q: %w", path, err)
		}

		var pathItem openAPIPathItem
		if err := json.Unmarshal(*resolvedPathItem, &pathItem); err != nil {
			return nil, fmt.Errorf("parse openapi path item %q: %w", path, err)
		}

		for _, operation := range pathItem.operations() {
			if operation != nil && operation.OperationID == operationID {
				matches = append(matches, operation)
			}
		}
	}

	return matches, nil
}

// operations returns every HTTP operation on the path item.
func (p openAPIPathItem) operations() []*openAPIOperation {
	return []*openAPIOperation{
		p.Get,
		p.Put,
		p.Post,
		p.Delete,
		p.Options,
		p.Head,
		p.Patch,
		p.Trace,
	}
}

// openAPIRequestBodySchema resolves a request body reference and returns its JSON schema.
func openAPIRequestBodySchema(
	openAPIJSONSpec *json.RawMessage,
	operation *openAPIOperation,
	operationID string,
) (*json.RawMessage, error) {
	if operation.RequestBody == nil {
		return nil, fmt.Errorf("operationId %q request body content type does not exist", operationID)
	}

	resolvedRequestBody, err := jsonrefs.ResolveReference(openAPIJSONSpec, operation.RequestBody)
	if err != nil {
		return nil, fmt.Errorf("resolve operationId %q request body: %w", operationID, err)
	}

	var requestBody openAPIRequestBody
	if err := json.Unmarshal(*resolvedRequestBody, &requestBody); err != nil {
		return nil, fmt.Errorf("parse operationId %q request body: %w", operationID, err)
	}

	if len(requestBody.Content) == 0 {
		return nil, fmt.Errorf("operationId %q request body content type does not exist", operationID)
	}

	mediaType, ok := requestBody.Content["application/json"]
	if !ok {
		return nil, fmt.Errorf("operationId %q request body content type is not json", operationID)
	}

	if mediaType.Schema == nil || len(*mediaType.Schema) == 0 || string(*mediaType.Schema) == "null" {
		return nil, fmt.Errorf("operationId %q application/json schema does not exist", operationID)
	}

	return mediaType.Schema, nil
}
