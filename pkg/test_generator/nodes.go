package testgenerator

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type OpenAPINode struct {
	Paths map[string]struct {
		Post *struct {
			OperationID string `yaml:"operationId"`
			RequestBody struct {
				Required bool `yaml:"required"`
				Content  map[string]struct {
					Schema SchemaNode `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"requestBody"`
		} `yaml:"post"`
	} `yaml:"paths"`
}

type SchemaNode struct {
	Object *ObjectNode
	String *StringNode
}

type BaseNode struct {
	Type     string `yaml:"type"`
	Nullable bool   `yaml:"nullable"`
}

type ObjectNode struct {
	BaseNode             `yaml:",inline"`
	Required             []string                 `yaml:"required"`
	AdditionalProperties AdditionalPropertiesNode `yaml:"additionalProperties"`
	Properties           map[string]SchemaNode    `yaml:"properties"`
}

type StringNode struct {
	BaseNode `yaml:",inline"`
}

type AdditionalPropertiesNode struct {
	Allowed *bool
	Schema  *SchemaNode
}

func (s *SchemaNode) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind == 0 {
		return fmt.Errorf("missing schema")
	}

	var base BaseNode
	err := value.Decode(&base)
	if err != nil {
		return err
	}

	switch base.Type {
	case "object":
		var objectNode ObjectNode
		err = value.Decode(&objectNode)
		if err != nil {
			return err
		}
		s.Object = &objectNode
		s.String = nil
		return nil
	case "string":
		s.Object = nil
		s.String = &StringNode{BaseNode: base}
		return nil
	default:
		return fmt.Errorf("unsupported schema type %q", base.Type)
	}
}

func (a *AdditionalPropertiesNode) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind == 0 {
		return nil
	}

	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag != "!!bool" {
			return fmt.Errorf("unsupported scalar %s", value.Tag)
		}

		var allowed bool
		err := value.Decode(&allowed)
		if err != nil {
			return err
		}

		a.Allowed = &allowed
		a.Schema = nil
		return nil
	case yaml.MappingNode:
		var schema SchemaNode
		err := value.Decode(&schema)
		if err != nil {
			return err
		}

		a.Allowed = nil
		a.Schema = &schema
		return nil
	default:
		return fmt.Errorf("unsupported yaml node kind %d", value.Kind)
	}
}
