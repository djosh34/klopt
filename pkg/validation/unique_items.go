package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/oas"
)

// rejectAuthoredSchemaExclusions walks every OpenAPI 3.0 Schema Object slot before
// ordinary request acquisition and compilation begin.
func rejectAuthoredSchemaExclusions(document json.RawMessage) error {
	root, ok := rawObject(document)
	if !ok {
		return nil
	}

	walker := authoredSchemaWalker{
		source:  oas.Source{Document: document},
		visited: make(map[string]struct{}),
	}

	// Top-level schema-bearing fields have a fixed paths-before-components order.
	if err := walker.paths(root["paths"], "#/paths"); err != nil {
		return err
	}

	return walker.components(root["components"], "#/components")
}

// authoredSchemaWalker traverses every OpenAPI object that can own a Schema Object.
type authoredSchemaWalker struct {
	source  oas.Source
	visited map[string]struct{}
}

// paths traverses Path Item Objects in lexical path order.
func (walker *authoredSchemaWalker) paths(raw json.RawMessage, pointer string) error {
	paths, ok := rawObject(raw)
	if !ok {
		return nil
	}

	for _, name := range slices.Sorted(maps.Keys(paths)) {
		if strings.HasPrefix(name, "x-") {
			continue
		}

		if err := walker.pathItem(paths[name], appendSchemaPointer(pointer, name)); err != nil {
			return err
		}
	}

	return nil
}

// components traverses every schema-bearing component kind in fixed order.
func (walker *authoredSchemaWalker) components(raw json.RawMessage, pointer string) error {
	components, ok := rawObject(raw)
	if !ok {
		return nil
	}

	// Component kinds use this fixed order; each component map is lexical.
	for _, entry := range []struct {
		name string
		walk func(json.RawMessage, string) error
	}{
		{name: "schemas", walk: walker.schema},
		{name: "parameters", walk: walker.parameter},
		{name: "requestBodies", walk: walker.requestBody},
		{name: "responses", walk: walker.response},
		{name: "headers", walk: walker.header},
		{name: "callbacks", walk: walker.callback},
	} {
		values, ok := rawObject(components[entry.name])
		if !ok {
			continue
		}

		for _, name := range slices.Sorted(maps.Keys(values)) {
			if err := entry.walk(values[name], appendSchemaPointer(pointer, entry.name, name)); err != nil {
				return err
			}
		}
	}

	return nil
}

// pathItem traverses path-level parameters and operations.
func (walker *authoredSchemaWalker) pathItem(raw json.RawMessage, pointer string) error {
	raw, pointer, ok, err := walker.resolve("path item", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	if err := walker.parameters(members["parameters"], appendSchemaPointer(pointer, "parameters")); err != nil {
		return err
	}

	for _, method := range []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"} {
		if rawOperation, present := members[method]; present {
			if err := walker.operation(rawOperation, appendSchemaPointer(pointer, method)); err != nil {
				return err
			}
		}
	}

	return nil
}

// operation traverses operation parameters, bodies, responses, and callbacks.
func (walker *authoredSchemaWalker) operation(raw json.RawMessage, pointer string) error {
	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	if err := walker.parameters(members["parameters"], appendSchemaPointer(pointer, "parameters")); err != nil {
		return err
	}

	if rawRequestBody, present := members["requestBody"]; present {
		if err := walker.requestBody(rawRequestBody, appendSchemaPointer(pointer, "requestBody")); err != nil {
			return err
		}
	}

	if err := walker.responses(members["responses"], appendSchemaPointer(pointer, "responses")); err != nil {
		return err
	}

	callbacks, ok := rawObject(members["callbacks"])
	if !ok {
		return nil
	}

	for _, name := range slices.Sorted(maps.Keys(callbacks)) {
		if err := walker.callback(callbacks[name], appendSchemaPointer(pointer, "callbacks", name)); err != nil {
			return err
		}
	}

	return nil
}

// parameters traverses a Parameter Object array in source order.
func (walker *authoredSchemaWalker) parameters(raw json.RawMessage, pointer string) error {
	parameters, ok := rawArray(raw)
	if !ok {
		return nil
	}

	for index, parameter := range parameters {
		if err := walker.parameter(parameter, appendSchemaPointer(pointer, strconv.Itoa(index))); err != nil {
			return err
		}
	}

	return nil
}

// parameter traverses one resolved Parameter Object.
func (walker *authoredSchemaWalker) parameter(raw json.RawMessage, pointer string) error {
	raw, pointer, ok, err := walker.resolve("parameter", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	return walker.schemaOrContent(members, pointer)
}

// header traverses one resolved Header Object.
func (walker *authoredSchemaWalker) header(raw json.RawMessage, pointer string) error {
	raw, pointer, ok, err := walker.resolve("header", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	return walker.schemaOrContent(members, pointer)
}

// schemaOrContent traverses the mutually exclusive schema and content fields.
func (walker *authoredSchemaWalker) schemaOrContent(
	members map[string]json.RawMessage,
	pointer string,
) error {
	if rawSchema, present := members["schema"]; present {
		if err := walker.schema(rawSchema, appendSchemaPointer(pointer, "schema")); err != nil {
			return err
		}
	}

	return walker.content(members["content"], appendSchemaPointer(pointer, "content"))
}

// requestBody traverses one resolved Request Body Object.
func (walker *authoredSchemaWalker) requestBody(raw json.RawMessage, pointer string) error {
	raw, pointer, ok, err := walker.resolve("request body", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	return walker.content(members["content"], appendSchemaPointer(pointer, "content"))
}

// responses traverses Response Objects in lexical status order.
func (walker *authoredSchemaWalker) responses(raw json.RawMessage, pointer string) error {
	responses, ok := rawObject(raw)
	if !ok {
		return nil
	}

	for _, status := range slices.Sorted(maps.Keys(responses)) {
		if strings.HasPrefix(status, "x-") {
			continue
		}

		if err := walker.response(responses[status], appendSchemaPointer(pointer, status)); err != nil {
			return err
		}
	}

	return nil
}

// response traverses one resolved Response Object.
func (walker *authoredSchemaWalker) response(raw json.RawMessage, pointer string) error {
	raw, pointer, ok, err := walker.resolve("response", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	headers, _ := rawObject(members["headers"])
	for _, name := range slices.Sorted(maps.Keys(headers)) {
		if err := walker.header(headers[name], appendSchemaPointer(pointer, "headers", name)); err != nil {
			return err
		}
	}

	return walker.content(members["content"], appendSchemaPointer(pointer, "content"))
}

// content traverses Media Type Objects in lexical media-type order.
func (walker *authoredSchemaWalker) content(raw json.RawMessage, pointer string) error {
	content, ok := rawObject(raw)
	if !ok {
		return nil
	}

	for _, mediaType := range slices.Sorted(maps.Keys(content)) {
		if err := walker.mediaType(content[mediaType], appendSchemaPointer(pointer, mediaType)); err != nil {
			return err
		}
	}

	return nil
}

// mediaType traverses a media schema and encoding headers.
func (walker *authoredSchemaWalker) mediaType(raw json.RawMessage, pointer string) error {
	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	if rawSchema, present := members["schema"]; present {
		if err := walker.schema(rawSchema, appendSchemaPointer(pointer, "schema")); err != nil {
			return err
		}
	}

	encodings, _ := rawObject(members["encoding"])
	for _, property := range slices.Sorted(maps.Keys(encodings)) {
		encoding, ok := rawObject(encodings[property])
		if !ok {
			continue
		}

		headers, _ := rawObject(encoding["headers"])
		for _, name := range slices.Sorted(maps.Keys(headers)) {
			if err := walker.header(
				headers[name],
				appendSchemaPointer(pointer, "encoding", property, "headers", name),
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// callback traverses callback Path Item Objects in lexical expression order.
func (walker *authoredSchemaWalker) callback(raw json.RawMessage, pointer string) error {
	raw, pointer, ok, err := walker.resolve("callback", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	callback, ok := rawObject(raw)
	if !ok {
		return nil
	}

	for _, expression := range slices.Sorted(maps.Keys(callback)) {
		if strings.HasPrefix(expression, "x-") {
			continue
		}

		if err := walker.pathItem(callback[expression], appendSchemaPointer(pointer, expression)); err != nil {
			return err
		}
	}

	return nil
}

// authoredSchemaPointer retains path tokens without copying every ancestor path.
type authoredSchemaPointer struct {
	base   string
	parent *authoredSchemaPointer
	token  string
}

// schema decodes one complete inline schema tree before traversing it.
func (walker *authoredSchemaWalker) schema(raw json.RawMessage, pointer string) error {
	if _, seen := walker.visited["schema\x00"+pointer]; seen {
		return nil
	}

	value, err := rawValue(raw)
	if err != nil {
		return fmt.Errorf("decode schema at %s: %w", pointer, err)
	}

	members, object := value.(map[string]any)
	if object {
		if _, reference := members["$ref"]; !reference {
			walker.markSeen("schema", pointer)
		}
	}

	return walker.schemaValue(value, &authoredSchemaPointer{base: pointer})
}

// schemaValue inspects one decoded Schema Object and traverses its decoded children.
func (walker *authoredSchemaWalker) schemaValue(value any, pointer *authoredSchemaPointer) error {
	members, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	if reference, present := members["$ref"]; present {
		return walker.referencedSchema(reference, pointer)
	}

	if _, present := members["uniqueItems"]; present {
		return unsupportedUniqueItems(pointer.String())
	}

	if _, present := members["discriminator"]; present {
		if _, adjacentOneOf := members["oneOf"]; adjacentOneOf {
			return unsupportedOneOf(pointer.String())
		}

		return unsupportedAuthoredDiscriminator(pointer.String())
	}

	return walker.nestedSchemaValues(members, pointer)
}

// referencedSchema preserves reference diagnostics while decoding only the resolved target tree.
func (walker *authoredSchemaWalker) referencedSchema(
	reference any,
	pointer *authoredSchemaPointer,
) error {
	pointerString := pointer.String()
	if walker.seen("schema", pointerString) {
		return nil
	}

	if referenceString, ok := reference.(string); ok && walker.seen("schema reference", referenceString) {
		return nil
	}

	raw, err := json.Marshal(map[string]any{"$ref": reference})
	if err != nil {
		return fmt.Errorf("encode schema reference at %s: %w", pointerString, err)
	}

	resolved, err := walker.source.ResolveAndInspect(
		oas.LocatedSchema{Raw: raw, Pointer: pointerString},
		func(authored oas.LocatedSchema) error {
			walker.markSeen("schema", authored.Pointer)

			return rejectAuthoredSchemaKeywords(authored)
		},
	)
	if err != nil {
		return fmt.Errorf("resolve schema at %s: %w", pointerString, err)
	}

	if walker.seen("resolved schema", resolved.Pointer) {
		return nil
	}

	value, err := rawValue(resolved.Raw)
	if err != nil {
		return fmt.Errorf("decode schema at %s: %w", resolved.Pointer, err)
	}

	return walker.schemaValue(value, &authoredSchemaPointer{base: resolved.Pointer})
}

// nestedSchemaValues traverses nested Schema Object keywords in one fixed order.
func (walker *authoredSchemaWalker) nestedSchemaValues(
	members map[string]any,
	pointer *authoredSchemaPointer,
) error {
	// Arrays retain source order and the properties map is lexical.
	if items, present := members["items"]; present {
		if err := walker.schemaValue(items, pointer.Append("items")); err != nil {
			return err
		}
	}

	properties, propertiesObject := members["properties"].(map[string]any)
	if !propertiesObject {
		properties = nil
	}

	for _, name := range slices.Sorted(maps.Keys(properties)) {
		if err := walker.schemaValue(properties[name], pointer.Append("properties", name)); err != nil {
			return err
		}
	}

	if additional, present := members["additionalProperties"]; present {
		if err := walker.schemaValue(additional, pointer.Append("additionalProperties")); err != nil {
			return err
		}
	}

	if err := walker.schemaValueArrays(members, pointer); err != nil {
		return err
	}

	if not, present := members["not"]; present {
		return walker.schemaValue(not, pointer.Append("not"))
	}

	return nil
}

// schemaValueArrays traverses array-valued composition keywords in fixed order.
func (walker *authoredSchemaWalker) schemaValueArrays(
	members map[string]any,
	pointer *authoredSchemaPointer,
) error {
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		children, ok := members[keyword].([]any)
		if !ok {
			continue
		}

		for index, child := range children {
			if err := walker.schemaValue(child, pointer.Append(keyword, strconv.Itoa(index))); err != nil {
				return err
			}
		}
	}

	return nil
}

// Append returns a linked child pointer without copying ancestor bytes.
func (pointer *authoredSchemaPointer) Append(tokens ...string) *authoredSchemaPointer {
	for _, token := range tokens {
		pointer = &authoredSchemaPointer{parent: pointer, token: token}
	}

	return pointer
}

// String materializes one RFC 6901 pointer only when diagnostics or resolution need it.
func (pointer *authoredSchemaPointer) String() string {
	tokens := make([]string, 0)
	for current := pointer; current != nil && current.parent != nil; current = current.parent {
		tokens = append(tokens, current.token)
	}

	root := pointer
	for root.parent != nil {
		root = root.parent
	}

	result := append([]byte(nil), root.base...)
	for index := len(tokens) - 1; index >= 0; index-- {
		result = append(result, '/')
		result = appendPointerToken(result, tokens[index])
	}

	return string(result)
}

// appendPointerToken appends one RFC 6901-escaped token.
func appendPointerToken(result []byte, token string) []byte {
	for index := range len(token) {
		switch token[index] {
		case '~':
			result = append(result, "~0"...)
		case '/':
			result = append(result, "~1"...)
		default:
			result = append(result, token[index])
		}
	}

	return result
}

// rawValue decodes one complete raw subtree without converting exact numbers to float64.
func rawValue(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	return value, nil
}

// resolve follows a non-schema Reference Object while breaking traversal cycles.
func (walker *authoredSchemaWalker) resolve(
	kind string,
	raw json.RawMessage,
	pointer string,
) (json.RawMessage, string, bool, error) {
	if walker.seen(kind, pointer) {
		return nil, "", false, nil
	}

	resolved, err := walker.source.ResolveAndInspect(
		oas.LocatedSchema{Raw: raw, Pointer: pointer},
		func(authored oas.LocatedSchema) error {
			walker.markSeen(kind, authored.Pointer)

			return nil
		},
	)
	if err != nil {
		return nil, "", false, err
	}

	return resolved.Raw, resolved.Pointer, true, nil
}

// seen records one typed object pointer and reports whether it was already traversed.
func (walker *authoredSchemaWalker) seen(kind string, pointer string) bool {
	key := kind + "\x00" + pointer
	if _, ok := walker.visited[key]; ok {
		return true
	}

	walker.markSeen(kind, pointer)

	return false
}

// markSeen records one typed object pointer without changing traversal flow.
func (walker *authoredSchemaWalker) markSeen(kind string, pointer string) {
	walker.visited[kind+"\x00"+pointer] = struct{}{}
}

// rejectAuthoredSchemaUniqueItems rejects the uniqueItems exclusion without decoding its value.
func rejectAuthoredSchemaUniqueItems(schema oas.LocatedSchema) error {
	members, ok := rawObject(schema.Raw)
	if !ok {
		return nil
	}

	if _, reference := members["$ref"]; reference {
		return nil
	}

	if _, present := members["uniqueItems"]; present {
		return unsupportedUniqueItems(schema.Pointer)
	}

	return nil
}

// rejectAuthoredSchemaKeywords rejects excluded authored keywords without decoding their values.
func rejectAuthoredSchemaKeywords(schema oas.LocatedSchema) error {
	if err := rejectAuthoredSchemaUniqueItems(schema); err != nil {
		return err
	}

	members, ok := rawObject(schema.Raw)
	if !ok {
		return nil
	}

	if _, reference := members["$ref"]; reference {
		return nil
	}

	if _, present := members["discriminator"]; present {
		if _, adjacentOneOf := members["oneOf"]; adjacentOneOf {
			return unsupportedOneOf(schema.Pointer)
		}

		return unsupportedAuthoredDiscriminator(schema.Pointer)
	}

	return nil
}

// unsupportedUniqueItems reports the exact authored keyword pointer.
func unsupportedUniqueItems(pointer string) error {
	return fmt.Errorf(
		"compile schema at %s/uniqueItems: authored uniqueItems is outside the Klopt profile",
		pointer,
	)
}

// unsupportedOneOf preserves oneOf's distinct attribution ahead of an adjacent discriminator.
func unsupportedOneOf(pointer string) error {
	return fmt.Errorf(
		"compile schema at %s/oneOf: authored oneOf is outside the Klopt profile",
		pointer,
	)
}

// unsupportedAuthoredDiscriminator reports the deliberate profile exclusion at its authored pointer.
func unsupportedAuthoredDiscriminator(pointer string) error {
	return fmt.Errorf(
		"compile schema at %s/discriminator: authored discriminator is outside the Klopt profile",
		pointer,
	)
}

// rawObject decodes a non-null JSON object.
func rawObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, false
	}

	return object, true
}

// rawArray decodes a non-null JSON array.
func rawArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err != nil || array == nil {
		return nil, false
	}

	return array, true
}

// appendSchemaPointer appends RFC 6901 escaped tokens to a document fragment pointer.
func appendSchemaPointer(pointer string, tokens ...string) string {
	for _, token := range tokens {
		token = strings.ReplaceAll(token, "~", "~0")
		token = strings.ReplaceAll(token, "/", "~1")
		pointer += "/" + token
	}

	return pointer
}
