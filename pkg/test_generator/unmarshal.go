package testgenerator

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

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
