//nolint:cyclop,godoclint,lll // Private selection code keeps complete pointer diagnostics beside each check.
package schematest

import (
	"errors"
	"fmt"
	"mime"
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

	parser := oasParser{
		document:  document,
		parsed:    make(map[string]*schemaNode),
		resolving: make(map[string]bool),
		shapes:    make(map[*jsonValue]*schemaShape),
	}

	node, err := parser.parseSchemaNode(schema, pointer, "#")
	if err != nil {
		return nil, err
	}

	return &schemaModel{
		root:          node,
		schemaPointer: pointer,
		schemaValue:   schema,
	}, nil
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

//nolint:gocognit // One deterministic pass must validate uniqueness while selecting the exact operation.
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
	seenPaths := make(map[string]string)

	var selected map[string]*jsonValue

	selectedPointer := ""

	pathNames := sortedObjectNames(paths)
	for _, pathName := range pathNames {
		if strings.HasPrefix(pathName, "x-") {
			continue
		}

		pathPointer := "#/paths/" + escapePointerToken(pathName)
		if !strings.HasPrefix(pathName, "/") {
			return nil, "", fmt.Errorf("%s: path field must begin with '/'", pathPointer)
		}

		pathIdentity, identityErr := templatedPathIdentity(pathName)
		if identityErr != nil {
			return nil, "", fmt.Errorf("%s: %w", pathPointer, identityErr)
		}

		if firstPointer, duplicate := seenPaths[pathIdentity]; duplicate {
			return nil, "", fmt.Errorf("%s: templated path is identical to %s", pathPointer, firstPointer)
		}

		seenPaths[pathIdentity] = pathPointer

		pathItemValue, resolvedPathPointer, referenceErr := resolvePathItemReference(
			document,
			paths[pathName],
			pathPointer,
		)
		if referenceErr != nil {
			return nil, "", referenceErr
		}

		pathPointer = resolvedPathPointer

		pathItem, objectErr := requireJSONObject(pathItemValue, pathPointer)
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

func templatedPathIdentity(path string) (string, error) {
	identity := make([]byte, 0, len(path))

	for position := 0; position < len(path); {
		switch path[position] {
		case '}':
			return "", errors.New("path template has an unmatched '}'")
		case '{':
			relativeEnd := strings.IndexByte(path[position+1:], '}')
			if relativeEnd < 0 {
				return "", errors.New("path template has an unmatched '{'")
			}

			end := position + 1 + relativeEnd

			name := path[position+1 : end]
			if name == "" || strings.ContainsRune(name, '{') {
				return "", errors.New("path template has an invalid expression")
			}

			identity = append(identity, '{', '}')
			position = end + 1
		default:
			identity = append(identity, path[position])
			position++
		}
	}

	return string(identity), nil
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

	if bodyErr := validateRequestBodyFields(body, bodyPointer); bodyErr != nil {
		return nil, "", bodyErr
	}

	contentValue, exists := body["content"]
	if !exists {
		return nil, "", fmt.Errorf("%s/content: required field is missing", bodyPointer)
	}

	content, err := requireJSONObject(contentValue, bodyPointer+"/content")
	if err != nil {
		return nil, "", err
	}

	mediaValue, mediaPointer, err := selectJSONMediaType(content, bodyPointer+"/content")
	if err != nil {
		return nil, "", err
	}

	media, err := requireJSONObject(mediaValue, mediaPointer)
	if err != nil {
		return nil, "", err
	}

	if mediaErr := validateMediaTypeFields(media, mediaPointer); mediaErr != nil {
		return nil, "", mediaErr
	}

	if examples, hasExamples := media["examples"]; hasExamples {
		if examplesErr := validateMediaTypeExamples(examples, mediaPointer+"/examples"); examplesErr != nil {
			return nil, "", examplesErr
		}
	}

	if _, hasExample := media["example"]; hasExample {
		if _, hasExamples := media["examples"]; hasExamples {
			return nil, "", fmt.Errorf("%s/examples: example and examples are mutually exclusive", mediaPointer)
		}
	}

	schema, exists := media["schema"]
	if !exists {
		return nil, "", fmt.Errorf("%s/schema: selected media type has no schema", mediaPointer)
	}

	return schema, mediaPointer + "/schema", nil
}

func selectJSONMediaType(content map[string]*jsonValue, pointer string) (*jsonValue, string, error) {
	var selected *jsonValue

	selectedPointer := ""
	selectedSpecificity := -1

	for _, name := range sortedObjectNames(content) {
		mediaType, _, err := mime.ParseMediaType(name)
		if err != nil || !validMediaTypeOrRange(mediaType) {
			return nil, "", fmt.Errorf("%s/%s: must be a valid media type or media range", pointer, escapePointerToken(name))
		}

		specificity := -1

		switch mediaType {
		case "application/json":
			specificity = 2
		case "application/*":
			specificity = 1
		case "*/*":
			specificity = 0
		}

		if specificity < 0 || specificity < selectedSpecificity {
			continue
		}

		mediaPointer := pointer + "/" + escapePointerToken(name)
		if specificity == selectedSpecificity {
			return nil, "", fmt.Errorf("%s: multiple equally specific media types match application/json", mediaPointer)
		}

		selected = content[name]
		selectedPointer = mediaPointer
		selectedSpecificity = specificity
	}

	if selected == nil {
		return nil, "", fmt.Errorf("%s/application~1json: selected request body has no application/json media type", pointer)
	}

	return selected, selectedPointer, nil
}

func validMediaTypeOrRange(mediaType string) bool {
	typeName, subtype, found := strings.Cut(mediaType, "/")
	if !found || typeName == "" || subtype == "" {
		return false
	}

	if strings.ContainsRune(typeName, '*') {
		return typeName == "*" && subtype == "*"
	}

	return !strings.ContainsRune(subtype, '*') || subtype == "*"
}

func validateMediaTypeFields(media map[string]*jsonValue, pointer string) error {
	for _, name := range sortedObjectNames(media) {
		switch name {
		case "schema", "example", "examples":
		case "encoding":
			return fmt.Errorf("%s/encoding: encoding does not apply to application/json", pointer)
		default:
			if !strings.HasPrefix(name, "x-") {
				return fmt.Errorf("%s/%s: unknown Media Type Object field", pointer, escapePointerToken(name))
			}
		}
	}

	return nil
}

func validateRequestBodyFields(body map[string]*jsonValue, pointer string) error {
	for _, name := range sortedObjectNames(body) {
		value := body[name]

		switch name {
		case "content":
		case "description":
			if value.kind != jsonString {
				return fmt.Errorf("%s/description: must be a string", pointer)
			}
		case "required":
			if value.kind != jsonBoolean {
				return fmt.Errorf("%s/required: must be a boolean", pointer)
			}
		default:
			if !strings.HasPrefix(name, "x-") {
				return fmt.Errorf("%s/%s: unknown Request Body Object field", pointer, escapePointerToken(name))
			}
		}
	}

	return nil
}

type oasParser struct {
	document  *jsonValue
	parsed    map[string]*schemaNode
	resolving map[string]bool
	shapes    map[*jsonValue]*schemaShape
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
		return parser.parseSchemaReference(referenceValue, usePointer, authoredPointer, instanceTemplate)
	}

	if shape, parsed := parser.shapes[value]; parsed {
		return &schemaNode{
			schemaShape: shape,
			occurrence: schemaOccurrence{
				usePointer:       usePointer,
				targetPointer:    authoredPointer,
				instanceTemplate: instanceTemplate,
			},
		}, nil
	}

	node := &schemaNode{
		occurrence: schemaOccurrence{
			usePointer:       usePointer,
			targetPointer:    authoredPointer,
			instanceTemplate: instanceTemplate,
		},
		schemaShape: &schemaShape{allowAdditionalProperties: true},
	}

	if err := parser.parseSchemaObject(node, object, authoredPointer); err != nil {
		return nil, err
	}

	parser.shapes[value] = node.schemaShape

	return node, nil
}

func (parser *oasParser) parseSchemaReference(
	referenceValue *jsonValue,
	usePointer string,
	authoredPointer string,
	instanceTemplate string,
) (*schemaNode, error) {
	if referenceValue.kind != jsonString {
		return nil, fmt.Errorf("%s/$ref: must be a string", authoredPointer)
	}

	target, targetPointer, err := resolveLocalReference(parser.document, referenceValue.text, authoredPointer+"/$ref")
	if err != nil {
		return nil, err
	}

	if parsed, exists := parser.parsed[targetPointer]; exists {
		return newSchemaReferenceOccurrence(parsed, usePointer, instanceTemplate), nil
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

	parsed, err := parser.parseSchemaOccurrence(target, targetPointer, targetPointer, "#")
	if err != nil {
		return nil, err
	}

	parser.parsed[targetPointer] = parsed

	return newSchemaReferenceOccurrence(parsed, usePointer, instanceTemplate), nil
}

func newSchemaReferenceOccurrence(parsed *schemaNode, usePointer, instanceTemplate string) *schemaNode {
	return &schemaNode{
		occurrence: schemaOccurrence{
			usePointer:       usePointer,
			targetPointer:    parsed.occurrence.targetPointer,
			instanceTemplate: instanceTemplate,
			reference:        true,
		},
		schemaShape: parsed.schemaShape,
	}
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
