//nolint:godoclint // Private admission traversal keeps OAS object slots and authored pointers explicit.
package schematest

import (
	"fmt"
	"strconv"
	"strings"
)

// validateDocumentProfile rejects exclusions that apply to the complete OpenAPI document.
func validateDocumentProfile(document *jsonValue, root map[string]*jsonValue) error {
	validator := documentProfileValidator{
		document:        document,
		inspected:       make(map[string]bool),
		active:          make(map[string]bool),
		schemaInspected: make(map[*jsonValue]bool),
		schemaActive:    make(map[*jsonValue]bool),
	}

	if err := validator.paths(root["paths"], "#/paths"); err != nil {
		return err
	}

	return validator.components(root["components"], "#/components")
}

type documentProfileValidator struct {
	document        *jsonValue
	inspected       map[string]bool
	active          map[string]bool
	schemaInspected map[*jsonValue]bool
	schemaActive    map[*jsonValue]bool
}

func (validator *documentProfileValidator) paths(value *jsonValue, pointer string) error {
	if value == nil || value.kind != jsonObject {
		return nil
	}

	for _, name := range sortedObjectNames(value.object) {
		if strings.HasPrefix(name, "x-") {
			continue
		}

		if err := validator.pathItem(value.object[name], pointer+"/"+escapePointerToken(name)); err != nil {
			return err
		}
	}

	return nil
}

func (validator *documentProfileValidator) components(value *jsonValue, pointer string) error {
	if value == nil || value.kind != jsonObject {
		return nil
	}

	entries := []struct {
		name string
		walk func(*jsonValue, string) error
	}{
		{name: "schemas", walk: validator.schema},
		{name: "parameters", walk: validator.parameter},
		{name: "requestBodies", walk: validator.requestBody},
		{name: "responses", walk: validator.response},
		{name: "headers", walk: validator.header},
		{name: "callbacks", walk: validator.callback},
		{name: "examples", walk: validator.example},
		{name: "securitySchemes", walk: validator.securityScheme},
		{name: "links", walk: validator.link},
	}

	for _, entry := range entries {
		values := value.object[entry.name]
		if values == nil || values.kind != jsonObject {
			continue
		}

		for _, name := range sortedObjectNames(values.object) {
			if err := entry.walk(
				values.object[name],
				pointer+"/"+entry.name+"/"+escapePointerToken(name),
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func (validator *documentProfileValidator) pathItem(value *jsonValue, pointer string) error {
	if err := rejectPathItemReferenceSiblings(value, pointer); err != nil {
		return err
	}

	return validator.resolvedObject(
		value,
		pointer,
		"path item",
		func(object map[string]*jsonValue, resolvedPointer string) error {
			if err := validator.parameters(object["parameters"], resolvedPointer+"/parameters"); err != nil {
				return err
			}

			for _, method := range operationMethods {
				if operation, exists := object[method]; exists {
					if err := validator.operation(operation, resolvedPointer+"/"+method); err != nil {
						return err
					}
				}
			}

			return nil
		},
	)
}

func rejectPathItemReferenceSiblings(value *jsonValue, pointer string) error {
	if value == nil || value.kind != jsonObject {
		return nil
	}

	if _, referenced := value.object["$ref"]; !referenced {
		return nil
	}

	for _, name := range sortedObjectNames(value.object) {
		if name != "$ref" && !strings.HasPrefix(name, "x-") {
			return fmt.Errorf(
				"%s/$ref: Path Item Object fields beside $ref have undefined OAS 3.0 behavior",
				pointer,
			)
		}
	}

	return nil
}

//nolint:cyclop // Operation Object schema slots have one fixed traversal order.
func (validator *documentProfileValidator) operation(value *jsonValue, pointer string) error {
	if value == nil || value.kind != jsonObject {
		return nil
	}

	object := value.object
	if err := validator.parameters(object["parameters"], pointer+"/parameters"); err != nil {
		return err
	}

	if requestBody, exists := object["requestBody"]; exists {
		if err := validator.requestBody(requestBody, pointer+"/requestBody"); err != nil {
			return err
		}
	}

	if err := validator.responses(object["responses"], pointer+"/responses"); err != nil {
		return err
	}

	callbacks := object["callbacks"]
	if callbacks == nil || callbacks.kind != jsonObject {
		return nil
	}

	for _, name := range sortedObjectNames(callbacks.object) {
		if err := validator.callback(callbacks.object[name], pointer+"/callbacks/"+escapePointerToken(name)); err != nil {
			return err
		}
	}

	return nil
}

func (validator *documentProfileValidator) parameters(value *jsonValue, pointer string) error {
	if value == nil || value.kind != jsonArray {
		return nil
	}

	for index, parameter := range value.array {
		if err := validator.parameter(parameter, pointer+"/"+strconv.Itoa(index)); err != nil {
			return err
		}
	}

	return nil
}

func (validator *documentProfileValidator) parameter(value *jsonValue, pointer string) error {
	return validator.resolvedObject(value, pointer, "parameter", validator.schemaExamplesOrContent)
}

func (validator *documentProfileValidator) header(value *jsonValue, pointer string) error {
	return validator.resolvedObject(value, pointer, "header", validator.schemaExamplesOrContent)
}

func (validator *documentProfileValidator) schemaExamplesOrContent(
	object map[string]*jsonValue,
	pointer string,
) error {
	if err := validator.examples(object["examples"], pointer+"/examples"); err != nil {
		return err
	}

	return validator.schemaOrContent(object, pointer)
}

func (validator *documentProfileValidator) schemaOrContent(object map[string]*jsonValue, pointer string) error {
	if schema, exists := object["schema"]; exists {
		if err := validator.schema(schema, pointer+"/schema"); err != nil {
			return err
		}
	}

	return validator.content(object["content"], pointer+"/content")
}

func (validator *documentProfileValidator) examples(value *jsonValue, pointer string) error {
	if value == nil || value.kind != jsonObject {
		return nil
	}

	for _, name := range sortedObjectNames(value.object) {
		if err := validator.example(value.object[name], pointer+"/"+escapePointerToken(name)); err != nil {
			return err
		}
	}

	return nil
}

func (validator *documentProfileValidator) example(value *jsonValue, pointer string) error {
	return validator.referenceLeaf(value, pointer, "example")
}

func (validator *documentProfileValidator) securityScheme(value *jsonValue, pointer string) error {
	return validator.referenceLeaf(value, pointer, "security scheme")
}

func (validator *documentProfileValidator) link(value *jsonValue, pointer string) error {
	return validator.referenceLeaf(value, pointer, "link")
}

func (validator *documentProfileValidator) referenceLeaf(value *jsonValue, pointer, kind string) error {
	return validator.resolvedObject(
		value,
		pointer,
		kind,
		func(map[string]*jsonValue, string) error { return nil },
	)
}

func (validator *documentProfileValidator) requestBody(value *jsonValue, pointer string) error {
	return validator.resolvedObject(
		value,
		pointer,
		"request body",
		func(object map[string]*jsonValue, resolvedPointer string) error {
			return validator.content(object["content"], resolvedPointer+"/content")
		},
	)
}

func (validator *documentProfileValidator) responses(value *jsonValue, pointer string) error {
	if value == nil || value.kind != jsonObject {
		return nil
	}

	for _, status := range sortedObjectNames(value.object) {
		if strings.HasPrefix(status, "x-") {
			continue
		}

		if err := validator.response(value.object[status], pointer+"/"+escapePointerToken(status)); err != nil {
			return err
		}
	}

	return nil
}

func (validator *documentProfileValidator) response(value *jsonValue, pointer string) error {
	return validator.resolvedObject(
		value,
		pointer,
		"response",
		func(object map[string]*jsonValue, resolvedPointer string) error {
			headers := object["headers"]
			if headers != nil && headers.kind == jsonObject {
				for _, name := range sortedObjectNames(headers.object) {
					if err := validator.header(
						headers.object[name],
						resolvedPointer+"/headers/"+escapePointerToken(name),
					); err != nil {
						return err
					}
				}
			}

			if err := validator.content(object["content"], resolvedPointer+"/content"); err != nil {
				return err
			}

			links := object["links"]
			if links != nil && links.kind == jsonObject {
				for _, name := range sortedObjectNames(links.object) {
					if err := validator.link(
						links.object[name],
						resolvedPointer+"/links/"+escapePointerToken(name),
					); err != nil {
						return err
					}
				}
			}

			return nil
		},
	)
}

//nolint:cyclop,gocognit // Media Type encoding and header slots require explicit nested container checks.
func (validator *documentProfileValidator) content(value *jsonValue, pointer string) error {
	if value == nil || value.kind != jsonObject {
		return nil
	}

	for _, mediaType := range sortedObjectNames(value.object) {
		media := value.object[mediaType]
		if media == nil || media.kind != jsonObject {
			continue
		}

		mediaPointer := pointer + "/" + escapePointerToken(mediaType)
		if schema, exists := media.object["schema"]; exists {
			if err := validator.schema(schema, mediaPointer+"/schema"); err != nil {
				return err
			}
		}

		if err := validator.examples(media.object["examples"], mediaPointer+"/examples"); err != nil {
			return err
		}

		encodings := media.object["encoding"]
		if encodings == nil || encodings.kind != jsonObject {
			continue
		}

		for _, property := range sortedObjectNames(encodings.object) {
			encoding := encodings.object[property]
			if encoding == nil || encoding.kind != jsonObject {
				continue
			}

			headers := encoding.object["headers"]
			if headers == nil || headers.kind != jsonObject {
				continue
			}

			for _, name := range sortedObjectNames(headers.object) {
				if err := validator.header(
					headers.object[name],
					mediaPointer+"/encoding/"+escapePointerToken(property)+"/headers/"+escapePointerToken(name),
				); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (validator *documentProfileValidator) callback(value *jsonValue, pointer string) error {
	return validator.resolvedObject(
		value,
		pointer,
		"callback",
		func(object map[string]*jsonValue, resolvedPointer string) error {
			for _, expression := range sortedObjectNames(object) {
				if strings.HasPrefix(expression, "x-") {
					continue
				}

				if err := validator.pathItem(
					object[expression],
					resolvedPointer+"/"+escapePointerToken(expression),
				); err != nil {
					return err
				}
			}

			return nil
		},
	)
}

func (validator *documentProfileValidator) schema(value *jsonValue, pointer string) error {
	resolved, resolvedPointer, err := resolveReferenceChain(validator.document, value, pointer, "schema")
	if err != nil {
		return err
	}

	if validator.schemaActive[resolved] {
		return fmt.Errorf(
			"%s/$ref: recursive schema graph reaching %s is outside the Klopt profile",
			pointer,
			resolvedPointer,
		)
	}

	if validator.schemaInspected[resolved] {
		return nil
	}

	validator.schemaActive[resolved] = true
	defer delete(validator.schemaActive, resolved)

	object, isObject := asJSONObject(resolved)
	if !isObject {
		return nil
	}

	if _, present := object["uniqueItems"]; present {
		return fmt.Errorf("%s/uniqueItems: authored uniqueItems is outside the Klopt profile", resolvedPointer)
	}

	if _, present := object["discriminator"]; present {
		if _, adjacentOneOf := object["oneOf"]; adjacentOneOf {
			return fmt.Errorf("%s/oneOf: authored oneOf is outside the Klopt profile", resolvedPointer)
		}

		return fmt.Errorf("%s/discriminator: authored discriminator is outside the Klopt profile", resolvedPointer)
	}

	if err := validator.nestedSchemas(object, resolvedPointer); err != nil {
		return err
	}

	validator.schemaInspected[resolved] = true

	return nil
}

//nolint:cyclop // Nested Schema Object slots have one fixed canonical traversal order.
func (validator *documentProfileValidator) nestedSchemas(object map[string]*jsonValue, pointer string) error {
	if items, exists := object["items"]; exists {
		if err := validator.schema(items, pointer+"/items"); err != nil {
			return err
		}
	}

	properties := object["properties"]
	if properties != nil && properties.kind == jsonObject {
		for _, name := range sortedObjectNames(properties.object) {
			if err := validator.schema(
				properties.object[name],
				pointer+"/properties/"+escapePointerToken(name),
			); err != nil {
				return err
			}
		}
	}

	if additional, exists := object["additionalProperties"]; exists && additional.kind != jsonBoolean {
		if err := validator.schema(additional, pointer+"/additionalProperties"); err != nil {
			return err
		}
	}

	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		children := object[keyword]
		if children == nil || children.kind != jsonArray {
			continue
		}

		for index, child := range children.array {
			if err := validator.schema(child, pointer+"/"+keyword+"/"+strconv.Itoa(index)); err != nil {
				return err
			}
		}
	}

	if child, exists := object["not"]; exists {
		return validator.schema(child, pointer+"/not")
	}

	return nil
}

func (validator *documentProfileValidator) resolvedObject(
	value *jsonValue,
	pointer string,
	kind string,
	walk func(map[string]*jsonValue, string) error,
) error {
	resolved, resolvedPointer, err := resolveReferenceChain(validator.document, value, pointer, kind)
	if err != nil {
		return err
	}

	key := kind + "\x00" + resolvedPointer
	if validator.active[key] {
		return fmt.Errorf(
			"%s/$ref: recursive %s reference reaching %s is outside the Klopt profile",
			pointer,
			kind,
			resolvedPointer,
		)
	}

	if validator.inspected[key] {
		return nil
	}

	validator.active[key] = true
	defer delete(validator.active, key)

	object, isObject := asJSONObject(resolved)
	if !isObject {
		return nil
	}

	if err := walk(object, resolvedPointer); err != nil {
		return err
	}

	validator.inspected[key] = true

	return nil
}

func asJSONObject(value *jsonValue) (map[string]*jsonValue, bool) {
	if value == nil || value.kind != jsonObject {
		return nil, false
	}

	return value.object, true
}
