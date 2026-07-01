package testgenerator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-openapi/jsonpointer"
	"gopkg.in/yaml.v3"
)

const maxSchemaRefDepth = 1000

func decodeSchemaNode(root *yaml.Node, schemaNode *yaml.Node) (SchemaNode, error) {
	resolved, err := resolveSchemaRefs(root, schemaNode, 0)
	if err != nil {
		return SchemaNode{}, err
	}

	var schema SchemaNode
	err = resolved.Decode(&schema)
	if err != nil {
		return SchemaNode{}, err
	}

	return schema, nil
}

func resolveSchemaRefs(root *yaml.Node, node *yaml.Node, depth int) (*yaml.Node, error) {
	if depth > maxSchemaRefDepth {
		return nil, fmt.Errorf("schema ref depth exceeds %d", maxSchemaRefDepth)
	}

	ref, ok, err := schemaRef(node)
	if err != nil {
		return nil, err
	}
	if ok {
		target, err := schemaRefTarget(root, ref)
		if err != nil {
			return nil, fmt.Errorf("resolve schema ref %q: %w", ref, err)
		}

		return resolveSchemaRefs(root, target, depth+1)
	}

	resolved := cloneYAMLNode(node)
	if resolved == nil || resolved.Kind != yaml.MappingNode {
		return resolved, nil
	}

	for i := 0; i < len(resolved.Content)-1; i += 2 {
		key := resolved.Content[i].Value
		value := resolved.Content[i+1]

		switch key {
		case "allOf":
			err := resolveSchemaSequenceRefs(root, value, depth)
			if err != nil {
				return nil, err
			}
		case "additionalProperties":
			if value.Kind != yaml.MappingNode {
				continue
			}

			child, err := resolveSchemaRefs(root, value, depth)
			if err != nil {
				return nil, fmt.Errorf("additionalProperties: %w", err)
			}
			resolved.Content[i+1] = child
		case "items":
			child, err := resolveSchemaRefs(root, value, depth)
			if err != nil {
				return nil, fmt.Errorf("items: %w", err)
			}
			resolved.Content[i+1] = child
		case "properties":
			err := resolvePropertySchemaRefs(root, value, depth)
			if err != nil {
				return nil, err
			}
		}
	}

	return resolved, nil
}

func schemaRef(node *yaml.Node) (string, bool, error) {
	if node == nil || node.Kind != yaml.MappingNode {
		return "", false, nil
	}

	var refNode *yaml.Node
	var sibling string
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		if key == "$ref" {
			if refNode != nil {
				return "", true, fmt.Errorf("schema ref has duplicate $ref")
			}

			refNode = node.Content[i+1]
			continue
		}

		if sibling == "" {
			sibling = key
		}
	}

	if refNode == nil {
		return "", false, nil
	}

	var ref string
	err := refNode.Decode(&ref)
	if err != nil {
		return "", true, fmt.Errorf("decode schema ref: %w", err)
	}

	if sibling != "" {
		return "", true, fmt.Errorf("schema ref %q has unsupported sibling %q", ref, sibling)
	}

	return ref, true, nil
}

func schemaRefTarget(root *yaml.Node, ref string) (*yaml.Node, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("unsupported non-local schema ref")
	}

	pointer, err := jsonpointer.New(strings.TrimPrefix(ref, "#"))
	if err != nil {
		return nil, fmt.Errorf("parse json pointer: %w", err)
	}

	node := root
	if node != nil && node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return nil, fmt.Errorf("openapi yaml must contain one document")
		}
		node = node.Content[0]
	}

	for _, token := range pointer.DecodedTokens() {
		if node == nil {
			return nil, fmt.Errorf("nil node before token %q", token)
		}

		switch node.Kind {
		case yaml.MappingNode:
			node = schemaMappingValue(node, token)
			if node == nil {
				return nil, fmt.Errorf("mapping has no key %q", token)
			}
		case yaml.SequenceNode:
			index, err := strconv.Atoi(token)
			if err != nil {
				return nil, fmt.Errorf("sequence token %q is not an index: %w", token, err)
			}
			if index < 0 || index >= len(node.Content) {
				return nil, fmt.Errorf("sequence index %d out of range", index)
			}
			node = node.Content[index]
		default:
			return nil, fmt.Errorf("node kind %d has no child %q", node.Kind, token)
		}
	}

	return node, nil
}

func resolveSchemaSequenceRefs(root *yaml.Node, node *yaml.Node, depth int) error {
	if node.Kind != yaml.SequenceNode {
		return nil
	}

	for i, schemaNode := range node.Content {
		resolved, err := resolveSchemaRefs(root, schemaNode, depth)
		if err != nil {
			return fmt.Errorf("allOf schema %d: %w", i+1, err)
		}
		node.Content[i] = resolved
	}

	return nil
}

func resolvePropertySchemaRefs(root *yaml.Node, node *yaml.Node, depth int) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		name := node.Content[i].Value
		resolved, err := resolveSchemaRefs(root, node.Content[i+1], depth)
		if err != nil {
			return fmt.Errorf("property %q: %w", name, err)
		}
		node.Content[i+1] = resolved
	}

	return nil
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	clone := *node
	clone.Content = make([]*yaml.Node, 0, len(node.Content))
	for _, child := range node.Content {
		clone.Content = append(clone.Content, cloneYAMLNode(child))
	}

	return &clone
}
