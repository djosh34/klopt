package testgenerator

import (
	"encoding/json"
	"sort"
	"strconv"
)

const additionalPropertyCaseKey = "__decode_and_validate_generator_additional_property_61f80f12f8b14e2f__"

var _ Caseable = new(ObjectNode)

type ObjectNode struct {
	BaseNode             `yaml:",inline"`
	Required             []string                 `yaml:"required"`
	AdditionalProperties AdditionalPropertiesNode `yaml:"additionalProperties"`
	Properties           map[string]SchemaNode    `yaml:"properties"`
}

func (o *ObjectNode) ValidCases() []Case {
	requiredNames, optionalNames, requiredObject := o.objectCaseData()

	cases := append([]Case{}, o.BaseNode.ValidCases()...)
	cases = append(cases, objectCase("required properties", requiredObject))

	for _, name := range requiredNames {
		schema, ok := o.Properties[name]
		if !ok {
			continue
		}

		validCases := schema.ValidCases()
		for index := 1; index < len(validCases); index++ {
			object := cloneObject(requiredObject)
			object[name] = validCases[index].Value

			cases = append(cases, objectCase(
				"required property "+name+" "+validCases[index].Name,
				object,
			))
		}
	}

	for _, name := range optionalNames {
		schema := o.Properties[name]
		for _, validCase := range schema.ValidCases() {
			object := cloneObject(requiredObject)
			object[name] = validCase.Value

			cases = append(cases, objectCase(
				"optional property "+name+" "+validCase.Name,
				object,
			))
		}
	}

	return append(cases, o.additionalPropertyValidCases(requiredObject)...)
}

func (o *ObjectNode) InvalidCases() []Case {
	requiredNames, optionalNames, requiredObject := o.objectCaseData()

	cases := append([]Case{}, o.BaseNode.InvalidCases()...)
	cases = append(cases,
		Case{Name: "string", Value: json.RawMessage(`"not-object"`)},
		Case{Name: "number", Value: json.RawMessage(`123`)},
		Case{Name: "boolean", Value: json.RawMessage(`true`)},
		Case{Name: "array", Value: json.RawMessage(`[]`)},
	)

	if len(requiredNames) > 0 {
		cases = append(cases, objectCase("missing required properties", nil))
	}

	if len(requiredNames) > 1 {
		for _, missingName := range requiredNames {
			object := cloneObject(requiredObject)
			delete(object, missingName)
			cases = append(cases, objectCase("missing required property "+missingName, object))
		}
	}

	for _, name := range append(requiredNames, optionalNames...) {
		schema := o.Properties[name]
		for _, invalidCase := range schema.InvalidCases() {
			object := cloneObject(requiredObject)
			object[name] = invalidCase.Value

			cases = append(cases, objectCase(
				"invalid property "+name+" "+invalidCase.Name,
				object,
			))
		}
	}

	return append(cases, o.additionalPropertyInvalidCases(requiredObject)...)
}

type AdditionalPropertiesNode struct {
	Allowed *bool
	Schema  *SchemaNode
}

func objectCase(name string, object map[string]json.RawMessage) Case {
	if object == nil {
		object = map[string]json.RawMessage{}
	}

	value, err := json.Marshal(object)
	if err != nil {
		panic(err)
	}

	return Case{
		Name:  name,
		Value: value,
	}
}

func (o *ObjectNode) additionalPropertyValidCases(requiredObject map[string]json.RawMessage) []Case {
	additionalPropertyKey := o.additionalPropertyKey()

	switch {
	case o.AdditionalProperties.Schema != nil:
		var cases []Case
		schema := *o.AdditionalProperties.Schema
		for _, validCase := range schema.ValidCases() {
			object := cloneObject(requiredObject)
			object[additionalPropertyKey] = validCase.Value

			cases = append(cases, objectCase(
				"additional property "+validCase.Name,
				object,
			))
		}
		return cases
	case o.AdditionalProperties.Allowed != nil && !*o.AdditionalProperties.Allowed:
		return nil
	default:
		object := cloneObject(requiredObject)
		object[additionalPropertyKey] = json.RawMessage(`"additional-property"`)

		return []Case{objectCase("additional property", object)}
	}
}

func (o *ObjectNode) additionalPropertyInvalidCases(requiredObject map[string]json.RawMessage) []Case {
	additionalPropertyKey := o.additionalPropertyKey()

	switch {
	case o.AdditionalProperties.Schema != nil:
		var cases []Case
		schema := *o.AdditionalProperties.Schema
		for _, invalidCase := range schema.InvalidCases() {
			object := cloneObject(requiredObject)
			object[additionalPropertyKey] = invalidCase.Value

			cases = append(cases, objectCase(
				"invalid additional property "+invalidCase.Name,
				object,
			))
		}
		return cases
	case o.AdditionalProperties.Allowed != nil && !*o.AdditionalProperties.Allowed:
		object := cloneObject(requiredObject)
		object[additionalPropertyKey] = json.RawMessage(`"not-allowed"`)

		return []Case{objectCase("additional property not allowed", object)}
	default:
		return nil
	}
}

func (o *ObjectNode) objectCaseData() ([]string, []string, map[string]json.RawMessage) {
	requiredNames := make([]string, 0, len(o.Required))
	requiredSet := map[string]struct{}{}
	for _, name := range o.Required {
		if _, ok := requiredSet[name]; ok {
			continue
		}

		requiredSet[name] = struct{}{}
		requiredNames = append(requiredNames, name)
	}

	requiredObject := map[string]json.RawMessage{}
	for _, name := range requiredNames {
		schema, ok := o.Properties[name]
		if !ok {
			continue
		}

		validCases := schema.ValidCases()
		if len(validCases) > 0 {
			requiredObject[name] = validCases[0].Value
		}
	}

	optionalNames := make([]string, 0, len(o.Properties))
	for name := range o.Properties {
		if _, ok := requiredSet[name]; !ok {
			optionalNames = append(optionalNames, name)
		}
	}
	sort.Strings(optionalNames)

	return requiredNames, optionalNames, requiredObject
}

func (o *ObjectNode) additionalPropertyKey() string {
	if _, ok := o.Properties[additionalPropertyCaseKey]; !ok {
		return additionalPropertyCaseKey
	}

	for suffix := 2; ; suffix++ {
		key := additionalPropertyCaseKey + "_" + strconv.Itoa(suffix)
		if _, ok := o.Properties[key]; !ok {
			return key
		}
	}
}

func cloneObject(object map[string]json.RawMessage) map[string]json.RawMessage {
	clone := make(map[string]json.RawMessage, len(object))
	for name, value := range object {
		clone[name] = value
	}

	return clone
}
