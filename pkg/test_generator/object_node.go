package testgenerator

import (
	"bytes"
	"encoding/json"
	"sort"
)

var _ Caseable = new(ObjectNode)

type ObjectNode struct {
	BaseNode             `yaml:",inline"`
	Required             []string                 `yaml:"required"`
	AdditionalProperties AdditionalPropertiesNode `yaml:"additionalProperties"`
	Properties           map[string]SchemaNode    `yaml:"properties"`
}

func (o *ObjectNode) ValidCases() []Case {
	cases := append([]Case{}, o.BaseNode.ValidCases()...)
	cases = append(cases, o.objectCase(o.requiredFields()...))

	for _, name := range o.requiredPropertyNames() {
		schema, ok := o.Properties[name]
		if !ok {
			continue
		}

		for index := 1; index < len(schema.ValidCases()); index++ {
			cases = append(cases, o.objectCase(
				replaceField(o.requiredFields(), objectFieldCase{
					name:      name,
					schema:    schema,
					valid:     true,
					caseIndex: index,
				})...,
			))
		}
	}

	for _, name := range o.optionalPropertyNames() {
		schema := o.Properties[name]
		for index := range schema.ValidCases() {
			cases = append(cases, o.objectCase(
				append(o.requiredFields(), objectFieldCase{
					name:      name,
					schema:    schema,
					valid:     true,
					caseIndex: index,
				})...,
			))
		}
	}

	return append(cases, o.additionalPropertyValidCases()...)
}

func (o *ObjectNode) InvalidCases() []Case {
	cases := append([]Case{}, o.BaseNode.InvalidCases()...)
	cases = append(cases,
		rawCase(`"not-object"`),
		rawCase(`123`),
		rawCase(`true`),
		rawCase(`[]`),
	)

	if len(o.requiredPropertyNames()) > 0 {
		cases = append(cases, o.objectCase())
	}

	if len(o.requiredPropertyNames()) > 1 {
		for _, missingName := range o.requiredPropertyNames() {
			var fields []objectFieldCase
			for _, field := range o.requiredFields() {
				if field.name != missingName {
					fields = append(fields, field)
				}
			}
			cases = append(cases, o.objectCase(fields...))
		}
	}

	for _, name := range o.propertyNames() {
		schema := o.Properties[name]
		for index := range schema.InvalidCases() {
			fields := append([]objectFieldCase{}, o.requiredFields()...)
			fields = replaceField(fields, objectFieldCase{
				name:      name,
				schema:    schema,
				valid:     false,
				caseIndex: index,
			})

			cases = append(cases, o.objectCase(fields...))
		}
	}

	return append(cases, o.additionalPropertyInvalidCases()...)
}

type AdditionalPropertiesNode struct {
	Allowed *bool
	Schema  *SchemaNode
}

type objectFieldCase struct {
	name      string
	schema    SchemaNode
	valid     bool
	caseIndex int
	raw       json.RawMessage
}

func (o *ObjectNode) objectCase(fields ...objectFieldCase) Case {
	return Case{
		GenerateValid: func(valid, invalid map[string]SchemaNode) json.RawMessage {
			return objectRawMessage(fields)
		},
		RequiredValid:   requiredSchemas(fields, true),
		RequiredInvalid: requiredSchemas(fields, false),
	}
}

func (o *ObjectNode) additionalPropertyValidCases() []Case {
	switch {
	case o.AdditionalProperties.Schema != nil:
		var cases []Case
		schema := *o.AdditionalProperties.Schema
		for index := range schema.ValidCases() {
			cases = append(cases, o.objectCase(
				append(o.requiredFields(), objectFieldCase{
					name:      "extra",
					schema:    schema,
					valid:     true,
					caseIndex: index,
				})...,
			))
		}
		return cases
	case o.AdditionalProperties.Allowed != nil && !*o.AdditionalProperties.Allowed:
		return nil
	default:
		return []Case{o.objectCase(
			append(o.requiredFields(), objectFieldCase{
				name:  "extra",
				valid: true,
				raw:   json.RawMessage(`"additional-property"`),
			})...,
		)}
	}
}

func (o *ObjectNode) additionalPropertyInvalidCases() []Case {
	switch {
	case o.AdditionalProperties.Schema != nil:
		var cases []Case
		schema := *o.AdditionalProperties.Schema
		for index := range schema.InvalidCases() {
			cases = append(cases, o.objectCase(
				append(o.requiredFields(), objectFieldCase{
					name:      "extra",
					schema:    schema,
					valid:     false,
					caseIndex: index,
				})...,
			))
		}
		return cases
	case o.AdditionalProperties.Allowed != nil && !*o.AdditionalProperties.Allowed:
		return []Case{o.objectCase(
			append(o.requiredFields(), objectFieldCase{
				name:  "extra",
				valid: false,
				raw:   json.RawMessage(`"not-allowed"`),
			})...,
		)}
	default:
		return nil
	}
}

func (o *ObjectNode) requiredFields() []objectFieldCase {
	var fields []objectFieldCase
	for _, name := range o.requiredPropertyNames() {
		schema, ok := o.Properties[name]
		if !ok {
			continue
		}

		fields = append(fields, objectFieldCase{
			name:   name,
			schema: schema,
			valid:  true,
		})
	}

	return fields
}

func (o *ObjectNode) requiredPropertyNames() []string {
	seen := map[string]struct{}{}
	var names []string
	for _, name := range o.Required {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	return names
}

func (o *ObjectNode) optionalPropertyNames() []string {
	required := map[string]struct{}{}
	for _, name := range o.requiredPropertyNames() {
		required[name] = struct{}{}
	}

	var names []string
	for name := range o.Properties {
		if _, ok := required[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	return names
}

func (o *ObjectNode) propertyNames() []string {
	var names []string
	seen := map[string]struct{}{}
	for _, name := range o.requiredPropertyNames() {
		if _, ok := o.Properties[name]; ok {
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}

	for _, name := range o.optionalPropertyNames() {
		if _, ok := seen[name]; ok {
			continue
		}
		names = append(names, name)
	}

	return names
}

func objectRawMessage(fields []objectFieldCase) json.RawMessage {
	var buffer bytes.Buffer
	buffer.WriteByte('{')

	for index, field := range fields {
		if index > 0 {
			buffer.WriteByte(',')
		}

		name, _ := json.Marshal(field.name)
		buffer.Write(name)
		buffer.WriteByte(':')
		buffer.Write(field.rawMessage())
	}

	buffer.WriteByte('}')
	return buffer.Bytes()
}

func (f objectFieldCase) rawMessage() json.RawMessage {
	if f.raw != nil {
		return f.raw
	}

	var cases []Case
	if f.valid {
		cases = f.schema.ValidCases()
	} else {
		cases = f.schema.InvalidCases()
	}

	if f.caseIndex >= len(cases) {
		return nil
	}

	return cases[f.caseIndex].GenerateValid(nil, nil)
}

func replaceField(fields []objectFieldCase, replacement objectFieldCase) []objectFieldCase {
	for index, field := range fields {
		if field.name == replacement.name {
			fields[index] = replacement
			return fields
		}
	}

	return append(fields, replacement)
}

func requiredSchemas(fields []objectFieldCase, valid bool) map[string]SchemaNode {
	schemas := map[string]SchemaNode{}
	for _, field := range fields {
		if field.raw != nil || field.valid != valid {
			continue
		}

		schemas[field.name] = field.schema
	}

	if len(schemas) == 0 {
		return nil
	}

	return schemas
}

func rawCase(raw string) Case {
	return Case{
		GenerateValid: func(valid, invalid map[string]SchemaNode) json.RawMessage {
			return json.RawMessage(raw)
		},
	}
}
