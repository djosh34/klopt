package validation

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/djosh34/klopt/pkg/internal/oas"
)

// rejectAuthoredUniqueItems walks every OpenAPI 3.0 Schema Object slot after
// ordinary request acquisition and compilation have succeeded.
func rejectAuthoredUniqueItems(document json.RawMessage) error {
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
	raw, pointer, ok := walker.resolve("path item", raw, pointer)
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
	raw, pointer, ok := walker.resolve("parameter", raw, pointer)
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
	raw, pointer, ok := walker.resolve("header", raw, pointer)
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
	raw, pointer, ok := walker.resolve("request body", raw, pointer)
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
	raw, pointer, ok := walker.resolve("response", raw, pointer)
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
	raw, pointer, ok := walker.resolve("callback", raw, pointer)
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

// schema inspects one authored Schema Object and then traverses its nested schemas.
func (walker *authoredSchemaWalker) schema(raw json.RawMessage, pointer string) error {
	if walker.seen("schema", pointer) {
		return nil
	}

	if _, ok := rawObject(raw); !ok {
		return nil
	}

	resolved, err := walker.resolveSchema(raw, pointer)
	if err != nil {
		return err
	}

	members, ok := rawObject(resolved.Raw)
	if !ok {
		return nil
	}

	return walker.nestedSchemas(members, resolved.Pointer)
}

// resolveSchema inspects every raw schema in a reference chain before resolution discards siblings.
func (walker *authoredSchemaWalker) resolveSchema(
	raw json.RawMessage,
	pointer string,
) (oas.LocatedSchema, error) {
	resolved, err := walker.source.ResolveAndInspect(
		oas.LocatedSchema{Raw: raw, Pointer: pointer},
		func(authored oas.LocatedSchema) error {
			walker.markSeen("schema", authored.Pointer)

			return rejectAuthoredSchemaUniqueItems(authored)
		},
	)
	if err != nil {
		return oas.LocatedSchema{}, err
	}

	return resolved, nil
}

// nestedSchemas traverses nested Schema Object keywords in one fixed order.
func (walker *authoredSchemaWalker) nestedSchemas(
	members map[string]json.RawMessage,
	pointer string,
) error {
	// Arrays retain source order and the properties map is lexical.
	if rawItems, present := members["items"]; present {
		if err := walker.schema(rawItems, appendSchemaPointer(pointer, "items")); err != nil {
			return err
		}
	}

	properties, _ := rawObject(members["properties"])
	for _, name := range slices.Sorted(maps.Keys(properties)) {
		if err := walker.schema(properties[name], appendSchemaPointer(pointer, "properties", name)); err != nil {
			return err
		}
	}

	if additional, present := members["additionalProperties"]; present {
		if err := walker.schema(additional, appendSchemaPointer(pointer, "additionalProperties")); err != nil {
			return err
		}
	}

	if err := walker.schemaArrays(members, pointer); err != nil {
		return err
	}

	if rawNot, present := members["not"]; present {
		return walker.schema(rawNot, appendSchemaPointer(pointer, "not"))
	}

	return nil
}

// schemaArrays traverses array-valued composition keywords in fixed order.
func (walker *authoredSchemaWalker) schemaArrays(
	members map[string]json.RawMessage,
	pointer string,
) error {
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		children, ok := rawArray(members[keyword])
		if !ok {
			continue
		}

		for index, child := range children {
			if err := walker.schema(child, appendSchemaPointer(pointer, keyword, strconv.Itoa(index))); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolve follows a non-schema Reference Object while breaking traversal cycles.
func (walker *authoredSchemaWalker) resolve(
	kind string,
	raw json.RawMessage,
	pointer string,
) (json.RawMessage, string, bool) {
	if walker.seen(kind, pointer) {
		return nil, "", false
	}

	resolved, err := walker.source.Resolve(oas.LocatedSchema{Raw: raw, Pointer: pointer})
	if err != nil {
		return nil, "", false
	}

	return resolved.Raw, resolved.Pointer, true
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

// rejectAuthoredSchemaUniqueItems rejects key presence without decoding its value.
func rejectAuthoredSchemaUniqueItems(schema oas.LocatedSchema) error {
	members, ok := rawObject(schema.Raw)
	if ok {
		if _, present := members["uniqueItems"]; present {
			return unsupportedUniqueItems(schema.Pointer)
		}
	}

	return nil
}

// unsupportedUniqueItems reports the exact authored keyword pointer.
func unsupportedUniqueItems(pointer string) error {
	return fmt.Errorf("compile schema at %s/uniqueItems: unsupported keyword", pointer)
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
