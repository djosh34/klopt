//nolint:cyclop,godoclint,lll // Private selection code keeps complete pointer diagnostics beside each check.
package schematest

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var operationMethods = []string{"delete", "get", "head", "options", "patch", "post", "put", "trace"}

// parseInput selects and compiles one application/json request-body schema.
func parseInput(input Input) (*schemaModel, error) {
	document, err := decodeOpenAPIDocument(input.OpenAPI)
	if err != nil {
		return nil, fmt.Errorf("parse OpenAPI document: %w", err)
	}

	root, err := requireJSONObject(document, "#")
	if err != nil {
		return nil, err
	}

	if versionErr := validateOpenAPIVersion(root); versionErr != nil {
		return nil, versionErr
	}

	schema, pointer, err := selectRequestSchema(document, root, input.OperationID)
	if err != nil {
		return nil, err
	}

	parser := oasParser{document: document, resolving: make(map[string]bool)}

	node, err := parser.parseSchemaNode(schema, pointer, "#")
	if err != nil {
		return nil, err
	}

	return &schemaModel{root: node}, nil
}

func validateOpenAPIVersion(root map[string]*jsonValue) error {
	value, exists := root["openapi"]
	if !exists {
		return errors.New("#/openapi: required field is missing")
	}

	if value.kind != jsonString {
		return errors.New("#/openapi: must be a string")
	}

	version, err := parseSemanticVersion(value.text)
	if err != nil {
		return fmt.Errorf("#/openapi: must be a valid Semantic Version: %w", err)
	}

	if version.major != "3" || version.minor != "0" {
		return fmt.Errorf("#/openapi: feature set %s.%s is outside the supported 3.0 profile", version.major, version.minor)
	}

	return nil
}

func selectRequestSchema(document *jsonValue, root map[string]*jsonValue, operationID string) (*jsonValue, string, error) {
	pathsValue, exists := root["paths"]
	if !exists {
		return nil, "", errors.New("#/paths: required field is missing")
	}

	paths, err := requireJSONObject(pathsValue, "#/paths")
	if err != nil {
		return nil, "", err
	}

	seenOperationIDs := make(map[string]string)

	var selected map[string]*jsonValue

	selectedPointer := ""

	pathNames := sortedObjectNames(paths)
	for _, pathName := range pathNames {
		if strings.HasPrefix(pathName, "x-") {
			continue
		}

		pathPointer := "#/paths/" + escapePointerToken(pathName)

		pathItem, objectErr := requireJSONObject(paths[pathName], pathPointer)
		if objectErr != nil {
			return nil, "", objectErr
		}

		for _, method := range operationMethods {
			operation, exists := pathItem[method]
			if !exists {
				continue
			}

			operationPointer := pathPointer + "/" + method

			operationObject, objectErr := requireJSONObject(operation, operationPointer)
			if objectErr != nil {
				return nil, "", objectErr
			}

			identifier, exists := operationObject["operationId"]
			if !exists {
				continue
			}

			if identifier.kind != jsonString {
				return nil, "", fmt.Errorf("%s/operationId: must be a string", operationPointer)
			}

			if firstPointer, duplicate := seenOperationIDs[identifier.text]; duplicate {
				return nil, "", fmt.Errorf(
					"%s/operationId: operationId %q must be unique; first authored at %s/operationId",
					operationPointer,
					identifier.text,
					firstPointer,
				)
			}

			seenOperationIDs[identifier.text] = operationPointer
			if identifier.text == operationID {
				selected = operationObject
				selectedPointer = operationPointer
			}
		}
	}

	if selected == nil {
		return nil, "", fmt.Errorf("operationId %q was not found", operationID)
	}

	return requestSchema(document, selected, selectedPointer)
}

func requestSchema(document *jsonValue, operation map[string]*jsonValue, operationPointer string) (*jsonValue, string, error) {
	requestBody, exists := operation["requestBody"]
	if !exists {
		return nil, "", fmt.Errorf("%s/requestBody: selected operation has no request body", operationPointer)
	}

	bodyPointer := operationPointer + "/requestBody"

	bodyValue, resolvedBodyPointer, err := resolveReferenceChain(document, requestBody, bodyPointer, "request body")
	if err != nil {
		return nil, "", err
	}

	bodyPointer = resolvedBodyPointer

	body, err := requireJSONObject(bodyValue, bodyPointer)
	if err != nil {
		return nil, "", err
	}

	contentValue, exists := body["content"]
	if !exists {
		return nil, "", fmt.Errorf("%s/content: required field is missing", bodyPointer)
	}

	content, err := requireJSONObject(contentValue, bodyPointer+"/content")
	if err != nil {
		return nil, "", err
	}

	mediaValue, exists := content["application/json"]
	if !exists {
		return nil, "", fmt.Errorf("%s/content/application~1json: selected request body has no application/json media type", bodyPointer)
	}

	mediaPointer := bodyPointer + "/content/application~1json"

	media, err := requireJSONObject(mediaValue, mediaPointer)
	if err != nil {
		return nil, "", err
	}

	schema, exists := media["schema"]
	if !exists {
		return nil, "", fmt.Errorf("%s/schema: selected media type has no schema", mediaPointer)
	}

	return schema, mediaPointer + "/schema", nil
}

type oasParser struct {
	document  *jsonValue
	resolving map[string]bool
}

func (parser *oasParser) parseSchemaNode(value *jsonValue, usePointer, instanceTemplate string) (*schemaNode, error) {
	return parser.parseSchemaOccurrence(value, usePointer, usePointer, instanceTemplate)
}

func (parser *oasParser) parseSchemaOccurrence(
	value *jsonValue,
	usePointer string,
	authoredPointer string,
	instanceTemplate string,
) (*schemaNode, error) {
	object, err := requireJSONObject(value, authoredPointer)
	if err != nil {
		return nil, err
	}

	if referenceValue, referenced := object["$ref"]; referenced {
		if referenceValue.kind != jsonString {
			return nil, fmt.Errorf("%s/$ref: must be a string", authoredPointer)
		}

		target, targetPointer, resolveErr := resolveLocalReference(
			parser.document,
			referenceValue.text,
			authoredPointer+"/$ref",
		)
		if resolveErr != nil {
			return nil, resolveErr
		}

		if parser.resolving[targetPointer] {
			return nil, fmt.Errorf(
				"%s/$ref: recursive schema graph reaching %s is outside the schematest profile",
				authoredPointer,
				targetPointer,
			)
		}

		parser.resolving[targetPointer] = true
		defer delete(parser.resolving, targetPointer)

		return parser.parseSchemaOccurrence(target, usePointer, targetPointer, instanceTemplate)
	}

	node := &schemaNode{
		occurrence: schemaOccurrence{
			usePointer:       usePointer,
			targetPointer:    authoredPointer,
			instanceTemplate: instanceTemplate,
		},
		allowAdditionalProperties: true,
	}

	if err := parser.parseSchemaObject(node, object, authoredPointer); err != nil {
		return nil, err
	}

	return node, nil
}

func requireJSONObject(value *jsonValue, pointer string) (map[string]*jsonValue, error) {
	if value == nil || value.kind != jsonObject {
		return nil, fmt.Errorf("%s: must be an object", pointer)
	}

	return value.object, nil
}

func sortedObjectNames(object map[string]*jsonValue) []string {
	names := make([]string, 0, len(object))
	for name := range object {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func escapePointerToken(token string) string {
	return strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
}
