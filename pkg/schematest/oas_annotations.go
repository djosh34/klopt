//nolint:cyclop,godoclint // Private inert annotation shape checks keep the fixed OAS fields explicit.
package schematest

import (
	"fmt"
	"net/url"
	"strings"
)

func validateInertAnnotations(object map[string]*jsonValue, pointer string) error {
	for _, name := range []string{"title", "description"} {
		if value, exists := object[name]; exists && value.kind != jsonString {
			return fmt.Errorf("%s/%s: must be a string", pointer, name)
		}
	}

	if value, exists := object["deprecated"]; exists && value.kind != jsonBoolean {
		return fmt.Errorf("%s/deprecated: must be a boolean", pointer)
	}

	if value, exists := object["externalDocs"]; exists {
		if err := validateExternalDocs(value, pointer+"/externalDocs"); err != nil {
			return err
		}
	}

	if value, exists := object["xml"]; exists {
		if err := validateXMLMetadata(value, pointer+"/xml"); err != nil {
			return err
		}
	}

	return nil
}

func validateExternalDocs(value *jsonValue, pointer string) error {
	object, err := requireJSONObject(value, pointer)
	if err != nil {
		return err
	}

	for _, name := range sortedObjectNames(object) {
		if name == "description" || name == "url" || strings.HasPrefix(name, "x-") {
			continue
		}

		return fmt.Errorf("%s/%s: unknown External Documentation Object field", pointer, escapePointerToken(name))
	}

	if description, exists := object["description"]; exists && description.kind != jsonString {
		return fmt.Errorf("%s/description: must be a string", pointer)
	}

	address, exists := object["url"]
	if !exists {
		return fmt.Errorf("%s/url: required field is missing", pointer)
	}

	if address.kind != jsonString {
		return fmt.Errorf("%s/url: must be a string", pointer)
	}

	if address.text == "" {
		return fmt.Errorf("%s/url: must be a non-empty URL", pointer)
	}

	if _, err := url.Parse(address.text); err != nil {
		return fmt.Errorf("%s/url: must be a URL", pointer)
	}

	return nil
}

func validateMediaTypeExamples(value *jsonValue, pointer string) error {
	examples, err := requireJSONObject(value, pointer)
	if err != nil {
		return err
	}

	for _, name := range sortedObjectNames(examples) {
		if err := validateMediaTypeExample(examples[name], pointer+"/"+escapePointerToken(name)); err != nil {
			return err
		}
	}

	return nil
}

func validateMediaTypeExample(value *jsonValue, pointer string) error {
	example, err := requireJSONObject(value, pointer)
	if err != nil {
		return err
	}

	if reference, referenced := example["$ref"]; referenced {
		if reference.kind != jsonString {
			return fmt.Errorf("%s/$ref: must be a string", pointer)
		}

		if _, err := parseLocalReferenceFragment(reference.text, pointer+"/$ref"); err != nil {
			return err
		}

		return nil
	}

	return validateExampleObject(example, pointer)
}

func validateExampleObject(example map[string]*jsonValue, pointer string) error {
	for _, name := range sortedObjectNames(example) {
		field := example[name]

		switch name {
		case "summary", "description", "externalValue":
			if field.kind != jsonString {
				return fmt.Errorf("%s/%s: must be a string", pointer, name)
			}
		case "value":
		default:
			if !strings.HasPrefix(name, "x-") {
				return fmt.Errorf("%s/%s: unknown Example Object field", pointer, escapePointerToken(name))
			}
		}
	}

	if _, hasValue := example["value"]; hasValue {
		if _, hasExternalValue := example["externalValue"]; hasExternalValue {
			return fmt.Errorf("%s/externalValue: value and externalValue are mutually exclusive", pointer)
		}
	}

	if externalValue, exists := example["externalValue"]; exists {
		if externalValue.text == "" {
			return fmt.Errorf("%s/externalValue: must be a non-empty URL", pointer)
		}

		if _, err := url.Parse(externalValue.text); err != nil {
			return fmt.Errorf("%s/externalValue: must be a URL", pointer)
		}
	}

	return nil
}

func validateXMLMetadata(value *jsonValue, pointer string) error {
	object, err := requireJSONObject(value, pointer)
	if err != nil {
		return err
	}

	for _, name := range sortedObjectNames(object) {
		switch name {
		case "name", "namespace", "prefix", "attribute", "wrapped":
			continue
		default:
			if strings.HasPrefix(name, "x-") {
				continue
			}

			return fmt.Errorf("%s/%s: unknown XML Object field", pointer, escapePointerToken(name))
		}
	}

	for _, name := range []string{"name", "namespace", "prefix"} {
		if field, exists := object[name]; exists && field.kind != jsonString {
			return fmt.Errorf("%s/%s: must be a string", pointer, name)
		}
	}

	for _, name := range []string{"attribute", "wrapped"} {
		if field, exists := object[name]; exists && field.kind != jsonBoolean {
			return fmt.Errorf("%s/%s: must be a boolean", pointer, name)
		}
	}

	if namespace, exists := object["namespace"]; exists {
		parsed, parseErr := url.Parse(namespace.text)
		if parseErr != nil || !parsed.IsAbs() {
			return fmt.Errorf("%s/namespace: must be a non-relative URI", pointer)
		}
	}

	return nil
}
