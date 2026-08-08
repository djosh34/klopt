package validation

import (
	"bytes"
	"encoding/json"
	"errors"
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
		source:             oas.Source{Document: document},
		inspected:          make(map[string]struct{}),
		active:             make(map[string]struct{}),
		resolvedReferences: make(map[string]oas.LocatedSchema),
	}

	// Top-level schema-bearing fields have a fixed paths-before-components order.
	if err := walker.paths(root["paths"], "#/paths"); err != nil {
		return err
	}

	return walker.components(root["components"], "#/components")
}

// authoredSchemaWalker traverses every OpenAPI object that can own a Schema Object.
type authoredSchemaWalker struct {
	source             oas.Source
	inspected          map[string]struct{}
	active             map[string]struct{}
	resolvedReferences map[string]oas.LocatedSchema
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
		{name: "examples", walk: walker.example},
		{name: "securitySchemes", walk: walker.securityScheme},
		{name: "links", walk: walker.link},
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
func (walker *authoredSchemaWalker) pathItem(raw json.RawMessage, pointer string) (result error) {
	raw, pointer, ok, err := walker.resolve("path item", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	defer func() {
		if result == nil {
			walker.finish("path item", pointer)
		}
	}()

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	if err := walker.parameters(members["parameters"], appendSchemaPointer(pointer, "parameters")); err != nil {
		return err
	}

	for _, method := range []string{"delete", "get", "head", "options", "patch", "post", "put", "trace"} {
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
func (walker *authoredSchemaWalker) parameter(raw json.RawMessage, pointer string) (result error) {
	raw, pointer, ok, err := walker.resolve("parameter", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	defer func() {
		if result == nil {
			walker.finish("parameter", pointer)
		}
	}()

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	if err := walker.examples(members["examples"], appendSchemaPointer(pointer, "examples")); err != nil {
		return err
	}

	return walker.schemaOrContent(members, pointer)
}

// header traverses one resolved Header Object.
func (walker *authoredSchemaWalker) header(raw json.RawMessage, pointer string) (result error) {
	raw, pointer, ok, err := walker.resolve("header", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	defer func() {
		if result == nil {
			walker.finish("header", pointer)
		}
	}()

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	if err := walker.examples(members["examples"], appendSchemaPointer(pointer, "examples")); err != nil {
		return err
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
func (walker *authoredSchemaWalker) requestBody(raw json.RawMessage, pointer string) (result error) {
	raw, pointer, ok, err := walker.resolve("request body", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	defer func() {
		if result == nil {
			walker.finish("request body", pointer)
		}
	}()

	members, ok := rawObject(raw)
	if !ok {
		return nil
	}

	return walker.content(members["content"], appendSchemaPointer(pointer, "content"))
}

// examples traverses Example Object maps without inspecting example payloads.
func (walker *authoredSchemaWalker) examples(raw json.RawMessage, pointer string) error {
	examples, ok := rawObject(raw)
	if !ok {
		return nil
	}

	for _, name := range slices.Sorted(maps.Keys(examples)) {
		if err := walker.example(examples[name], appendSchemaPointer(pointer, name)); err != nil {
			return err
		}
	}

	return nil
}

// example follows one Example Object reference chain without inspecting its payload.
func (walker *authoredSchemaWalker) example(raw json.RawMessage, pointer string) error {
	return walker.referenceLeaf("example", raw, pointer)
}

// securityScheme follows one Security Scheme Object reference chain.
func (walker *authoredSchemaWalker) securityScheme(raw json.RawMessage, pointer string) error {
	return walker.referenceLeaf("security scheme", raw, pointer)
}

// link follows one Link Object reference chain.
func (walker *authoredSchemaWalker) link(raw json.RawMessage, pointer string) error {
	return walker.referenceLeaf("link", raw, pointer)
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
func (walker *authoredSchemaWalker) response(raw json.RawMessage, pointer string) (result error) {
	raw, pointer, ok, err := walker.resolve("response", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	defer func() {
		if result == nil {
			walker.finish("response", pointer)
		}
	}()

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

	if err := walker.content(members["content"], appendSchemaPointer(pointer, "content")); err != nil {
		return err
	}

	links, _ := rawObject(members["links"])
	for _, name := range slices.Sorted(maps.Keys(links)) {
		if err := walker.link(links[name], appendSchemaPointer(pointer, "links", name)); err != nil {
			return err
		}
	}

	return nil
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

	if err := walker.examples(members["examples"], appendSchemaPointer(pointer, "examples")); err != nil {
		return err
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
func (walker *authoredSchemaWalker) callback(raw json.RawMessage, pointer string) (result error) {
	raw, pointer, ok, err := walker.resolve("callback", raw, pointer)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	defer func() {
		if result == nil {
			walker.finish("callback", pointer)
		}
	}()

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

// schema resolves references and decodes each complete inline tree once.
func (walker *authoredSchemaWalker) schema(raw json.RawMessage, pointer string) error {
	value, err := rawValue(raw)
	if err != nil {
		return fmt.Errorf("decode schema at %s: %w", pointer, err)
	}

	members, object := value.(map[string]any)
	if object {
		if _, referenced := members["$ref"]; !referenced {
			return walker.schemaValue(value, pointer, pointer)
		}
	}

	resolved, err := walker.resolveReference(raw, pointer, "schema")
	if err != nil {
		return err
	}

	value, err = rawValue(resolved.Raw)
	if err != nil {
		return fmt.Errorf("decode schema at %s: %w", resolved.Pointer, err)
	}

	return walker.schemaValue(value, resolved.Pointer, pointer)
}

// schemaValue fully inspects one decoded Schema Object before marking it complete.
func (walker *authoredSchemaWalker) schemaValue(value any, pointer, usePointer string) error {
	members, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	if _, referenced := members["$ref"]; referenced {
		raw, err := json.Marshal(members)
		if err != nil {
			return fmt.Errorf("encode schema reference at %s: %w", pointer, err)
		}

		return walker.schema(raw, pointer)
	}

	key := "schema\x00" + pointer
	if _, active := walker.active[key]; active {
		return fmt.Errorf(
			"compile schema at %s/$ref: recursive schema graph reaching %s is outside the Klopt profile",
			usePointer,
			pointer,
		)
	}

	if _, inspected := walker.inspected[key]; inspected {
		return nil
	}

	walker.active[key] = struct{}{}
	defer delete(walker.active, key)

	if _, present := members["uniqueItems"]; present {
		return unsupportedUniqueItems(pointer)
	}

	if _, present := members["discriminator"]; present {
		if _, adjacentOneOf := members["oneOf"]; adjacentOneOf {
			return unsupportedOneOf(pointer)
		}

		return unsupportedAuthoredDiscriminator(pointer)
	}

	if err := walker.nestedSchemaValues(members, pointer); err != nil {
		return err
	}

	walker.inspected[key] = struct{}{}

	return nil
}

// nestedSchemaValues traverses nested Schema Object slots in canonical order.
//
//nolint:cyclop // Nested Schema Object slots have one fixed canonical traversal order.
func (walker *authoredSchemaWalker) nestedSchemaValues(members map[string]any, pointer string) error {
	if items, present := members["items"]; present {
		if err := walker.schemaValue(items, appendSchemaPointer(pointer, "items"), pointer); err != nil {
			return err
		}
	}

	properties, propertiesObject := members["properties"].(map[string]any)
	if !propertiesObject {
		properties = nil
	}

	for _, name := range slices.Sorted(maps.Keys(properties)) {
		childPointer := appendSchemaPointer(pointer, "properties", name)
		if err := walker.schemaValue(properties[name], childPointer, childPointer); err != nil {
			return err
		}
	}

	if additional, present := members["additionalProperties"]; present {
		childPointer := appendSchemaPointer(pointer, "additionalProperties")
		if err := walker.schemaValue(additional, childPointer, childPointer); err != nil {
			return err
		}
	}

	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		children, ok := members[keyword].([]any)
		if !ok {
			continue
		}

		for index, child := range children {
			childPointer := appendSchemaPointer(pointer, keyword, strconv.Itoa(index))
			if err := walker.schemaValue(child, childPointer, childPointer); err != nil {
				return err
			}
		}
	}

	if not, present := members["not"]; present {
		childPointer := appendSchemaPointer(pointer, "not")

		return walker.schemaValue(not, childPointer, childPointer)
	}

	return nil
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

// resolve begins traversal of one non-schema referenceable object.
func (walker *authoredSchemaWalker) resolve(
	kind string,
	raw json.RawMessage,
	pointer string,
) (json.RawMessage, string, bool, error) {
	resolved, err := walker.resolveReference(raw, pointer, kind)
	if err != nil {
		return nil, "", false, err
	}

	key := kind + "\x00" + resolved.Pointer
	if _, active := walker.active[key]; active {
		return nil, "", false, fmt.Errorf(
			"compile schema at %s/$ref: recursive %s reference reaching %s is outside the Klopt profile",
			pointer,
			kind,
			resolved.Pointer,
		)
	}

	if _, inspected := walker.inspected[key]; inspected {
		return nil, "", false, nil
	}

	walker.active[key] = struct{}{}

	return resolved.Raw, resolved.Pointer, true, nil
}

// finish marks one fully traversed non-schema object complete.
func (walker *authoredSchemaWalker) finish(kind string, pointer string) {
	key := kind + "\x00" + pointer
	delete(walker.active, key)
	walker.inspected[key] = struct{}{}
}

// resolveReference follows one reference chain and classifies profile exclusions.
//
//nolint:cyclop // Cache, resolution, and the two profile exclusion causes are one ordered operation.
func (walker *authoredSchemaWalker) resolveReference(
	raw json.RawMessage,
	pointer string,
	kind string,
) (oas.LocatedSchema, error) {
	reference := ""
	if members, ok := rawObject(raw); ok {
		if err := json.Unmarshal(members["$ref"], &reference); err != nil {
			reference = ""
		}
	}

	if resolved, ok := walker.resolvedReferences[reference]; ok && reference != "" {
		return resolved, nil
	}

	resolved, err := walker.source.Resolve(oas.LocatedSchema{Raw: raw, Pointer: pointer})
	if err == nil {
		if reference != "" {
			if walker.resolvedReferences == nil {
				walker.resolvedReferences = make(map[string]oas.LocatedSchema)
			}

			walker.resolvedReferences[reference] = resolved
		}

		return resolved, nil
	}

	var referenceError *oas.ReferenceError
	if errors.As(err, &referenceError) {
		switch {
		case errors.Is(referenceError.Cause, oas.ErrExternalReference):
			return oas.LocatedSchema{}, fmt.Errorf(
				"compile schema at %s: external reference %q is outside the Klopt profile",
				referenceError.AuthoredKeyword,
				referenceError.Reference,
			)
		case errors.Is(referenceError.Cause, oas.ErrReferenceCycle):
			return oas.LocatedSchema{}, fmt.Errorf(
				"compile schema at %s: recursive %s reference reaching %s is outside the Klopt profile",
				referenceError.AuthoredKeyword,
				kind,
				referenceError.Reference,
			)
		}
	}

	return oas.LocatedSchema{}, fmt.Errorf("resolve %s at %s: %w", kind, pointer, err)
}

// referenceLeaf inspects a referenceable object with no nested Reference Object slots.
func (walker *authoredSchemaWalker) referenceLeaf(kind string, raw json.RawMessage, pointer string) error {
	_, resolvedPointer, ok, err := walker.resolve(kind, raw, pointer)
	if err != nil || !ok {
		return err
	}

	walker.finish(kind, resolvedPointer)

	return nil
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
