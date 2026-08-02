//nolint:cyclop,godoclint // Generic YAML AST conversion keeps the admitted JSON-shaped node cases explicit.
package schematest

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

// decodeOpenAPIDocument decodes one JSON or YAML document without OAS semantics.
func decodeOpenAPIDocument(source []byte) (*jsonValue, error) {
	if !utf8.Valid(source) {
		return nil, errors.New("OpenAPI document is not valid UTF-8")
	}

	trimmed := bytes.TrimSpace(source)

	var jsonErr error

	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		value, parseErr := parseStrictJSON(source)
		if parseErr == nil {
			return value, nil
		}

		jsonErr = parseErr
	}

	file, err := parser.ParseBytes(source, 0)
	if err != nil {
		if jsonErr != nil {
			return nil, fmt.Errorf("parse strict JSON: %w; parse YAML: %w", jsonErr, err)
		}

		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	if len(file.Docs) != 1 {
		return nil, fmt.Errorf("expected one YAML document, got %d", len(file.Docs))
	}

	if file.Docs[0].Body == nil {
		return nil, errors.New("expected one YAML document value")
	}

	decoder := yamlJSONDecoder{
		anchors:   make(map[string]ast.Node),
		decoded:   make(map[ast.Node]*jsonValue),
		resolving: make(map[ast.Node]bool),
	}

	return decoder.decodeNode(file.Docs[0].Body)
}

type yamlJSONDecoder struct {
	anchors   map[string]ast.Node
	decoded   map[ast.Node]*jsonValue
	resolving map[ast.Node]bool
}

func (decoder *yamlJSONDecoder) decodeNode(node ast.Node) (*jsonValue, error) {
	switch typed := node.(type) {
	case *ast.NullNode:
		if typed.GetToken().Value != "null" || typed.String() != "null" {
			return nil, fmt.Errorf("YAML null %q is outside the YAML JSON schema", typed.GetToken().Value)
		}

		return &jsonValue{kind: jsonNull}, nil
	case *ast.BoolNode:
		return decodeYAMLBoolean(typed)
	case *ast.IntegerNode, *ast.FloatNode:
		return decodeYAMLNumber(typed.GetToken().Value)
	case *ast.StringNode:
		return decodeYAMLString(typed)
	case *ast.LiteralNode:
		return &jsonValue{kind: jsonString, text: typed.Value.Value}, nil
	case *ast.SequenceNode:
		return decoder.decodeSequence(typed)
	case *ast.MappingNode:
		return decoder.decodeMapping(typed)
	case *ast.AnchorNode:
		return decoder.decodeAnchor(typed)
	case *ast.AliasNode:
		return decoder.decodeAlias(typed)
	case *ast.TagNode:
		return decoder.decodeTag(typed)
	default:
		return nil, fmt.Errorf("YAML node type %T is outside the YAML JSON schema", node)
	}
}

func decodeYAMLBoolean(node *ast.BoolNode) (*jsonValue, error) {
	switch node.GetToken().Value {
	case "false":
		return &jsonValue{kind: jsonBoolean}, nil
	case "true":
		return &jsonValue{kind: jsonBoolean, boolean: true}, nil
	default:
		return nil, fmt.Errorf("YAML boolean %q is outside the YAML JSON schema", node.GetToken().Value)
	}
}

func decodeYAMLString(node *ast.StringNode) (*jsonValue, error) {
	if node.GetToken().Type == token.StringType {
		number, err := parseExactNumber(node.Value)
		if err == nil {
			return &jsonValue{kind: jsonNumber, number: number}, nil
		}
	}

	return &jsonValue{kind: jsonString, text: node.Value}, nil
}

func decodeYAMLNumber(source string) (*jsonValue, error) {
	number, err := parseExactNumber(source)
	if err != nil {
		return nil, fmt.Errorf("YAML number %q is outside the YAML JSON schema: %w", source, err)
	}

	return &jsonValue{kind: jsonNumber, number: number}, nil
}

func (decoder *yamlJSONDecoder) decodeSequence(node *ast.SequenceNode) (*jsonValue, error) {
	elements := make([]*jsonValue, 0, len(node.Values))
	for index, child := range node.Values {
		value, err := decoder.decodeNode(child)
		if err != nil {
			return nil, fmt.Errorf("YAML sequence element %d: %w", index, err)
		}

		elements = append(elements, value)
	}

	return &jsonValue{kind: jsonArray, array: elements}, nil
}

func (decoder *yamlJSONDecoder) decodeMapping(node *ast.MappingNode) (*jsonValue, error) {
	members := make(map[string]*jsonValue, len(node.Values))
	for index, entry := range node.Values {
		key, err := yamlMappingKey(entry.Key)
		if err != nil {
			return nil, fmt.Errorf("YAML mapping key at entry %d: %w", index, err)
		}

		if _, duplicate := members[key]; duplicate {
			return nil, fmt.Errorf("duplicate YAML mapping key %q", key)
		}

		value, err := decoder.decodeNode(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("YAML mapping member %q: %w", key, err)
		}

		members[key] = value
	}

	return &jsonValue{kind: jsonObject, object: members}, nil
}

func (decoder *yamlJSONDecoder) decodeAnchor(node *ast.AnchorNode) (*jsonValue, error) {
	name, err := yamlString(node.Name)
	if err != nil {
		return nil, fmt.Errorf("YAML anchor name: %w", err)
	}

	decoder.anchors[name] = node.Value

	return decoder.decodeAnchorValue(name, node.Value)
}

func (decoder *yamlJSONDecoder) decodeAlias(node *ast.AliasNode) (*jsonValue, error) {
	name, err := yamlString(node.Value)
	if err != nil {
		return nil, fmt.Errorf("YAML alias name: %w", err)
	}

	target, exists := decoder.anchors[name]
	if !exists {
		return nil, fmt.Errorf("YAML alias %q has no preceding anchor", name)
	}

	return decoder.decodeAnchorValue(name, target)
}

func (decoder *yamlJSONDecoder) decodeAnchorValue(name string, target ast.Node) (*jsonValue, error) {
	if value, decoded := decoder.decoded[target]; decoded {
		return value, nil
	}

	if decoder.resolving[target] {
		return nil, fmt.Errorf("YAML alias %q creates a cycle", name)
	}

	decoder.resolving[target] = true
	defer delete(decoder.resolving, target)

	value, err := decoder.decodeNode(target)
	if err != nil {
		return nil, err
	}

	decoder.decoded[target] = value

	return value, nil
}

func (decoder *yamlJSONDecoder) decodeTag(node *ast.TagNode) (*jsonValue, error) {
	name := node.GetToken().Value

	switch name {
	case "!!map", "!<tag:yaml.org,2002:map>":
		mapping, ok := node.Value.(*ast.MappingNode)
		if !ok {
			return nil, fmt.Errorf("YAML tag %q requires a mapping", name)
		}

		return decoder.decodeMapping(mapping)
	case "!!seq", "!<tag:yaml.org,2002:seq>":
		sequence, ok := node.Value.(*ast.SequenceNode)
		if !ok {
			return nil, fmt.Errorf("YAML tag %q requires a sequence", name)
		}

		return decoder.decodeSequence(sequence)
	}

	value, scalar := node.Value.(ast.ScalarNode)
	if !scalar {
		return nil, fmt.Errorf("YAML tag %q on a non-scalar is outside the YAML JSON schema", name)
	}

	source := value.GetToken().Value

	switch name {
	case "!!str", "!<tag:yaml.org,2002:str>":
		return &jsonValue{kind: jsonString, text: source}, nil
	case "!!null", "!<tag:yaml.org,2002:null>":
		if source != "null" {
			return nil, fmt.Errorf("YAML null %q is outside the YAML JSON schema", source)
		}

		return &jsonValue{kind: jsonNull}, nil
	case "!!bool", "!<tag:yaml.org,2002:bool>":
		boolean, ok := node.Value.(*ast.BoolNode)
		if !ok {
			return nil, fmt.Errorf("YAML boolean %q is outside the YAML JSON schema", source)
		}

		return decodeYAMLBoolean(boolean)
	case "!!int", "!!float", "!<tag:yaml.org,2002:int>", "!<tag:yaml.org,2002:float>":
		return decodeYAMLNumber(source)
	default:
		return nil, fmt.Errorf("YAML tag %q is outside the YAML JSON schema", name)
	}
}

func yamlMappingKey(node ast.Node) (string, error) {
	if tagged, ok := node.(*ast.TagNode); ok {
		if tagged.GetToken().Value != "!!str" && tagged.GetToken().Value != "!<tag:yaml.org,2002:str>" {
			return "", errors.New("must be a scalar string")
		}

		return yamlString(tagged.Value)
	}

	text, err := yamlString(node)
	if err != nil {
		return "", err
	}

	if plain, ok := node.(*ast.StringNode); ok && plain.GetToken().Type == token.StringType {
		if _, numberErr := parseExactNumber(text); numberErr == nil {
			return "", errors.New("must be a scalar string")
		}
	}

	return text, nil
}

func yamlString(node ast.Node) (string, error) {
	switch typed := node.(type) {
	case *ast.StringNode:
		return typed.Value, nil
	case *ast.LiteralNode:
		return typed.Value.Value, nil
	default:
		return "", errors.New("must be a scalar string")
	}
}
