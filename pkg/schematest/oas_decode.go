//nolint:cyclop,gocognit,godoclint // Generic YAML AST conversion keeps admitted JSON-shaped cases explicit.
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
		anchors: make(map[string]ast.Node),
		active:  make(map[ast.Node]bool),
		decoded: make(map[ast.Node]*jsonValue),
	}

	return decoder.decode(file.Docs[0].Body)
}

type yamlJSONDecoder struct {
	anchors map[string]ast.Node
	active  map[ast.Node]bool
	decoded map[ast.Node]*jsonValue
	result  *jsonValue
}

type yamlDecodeDestination struct {
	parent *jsonValue
	index  int
	name   string
	root   bool
}

type yamlDecodeFrame struct {
	node        ast.Node
	destination yamlDecodeDestination
	context     string
	exit        bool
}

func (decoder *yamlJSONDecoder) decode(node ast.Node) (*jsonValue, error) {
	stack := []yamlDecodeFrame{{node: node, destination: yamlDecodeDestination{root: true}}}

	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if frame.exit {
			delete(decoder.active, frame.node)

			continue
		}

		switch typed := frame.node.(type) {
		case *ast.NullNode:
			if typed.GetToken().Value != "null" || typed.String() != "null" {
				return nil, yamlContextError(frame.context, fmt.Errorf(
					"YAML null %q is outside the YAML JSON schema", typed.GetToken().Value,
				))
			}

			decoder.assignDecoded(frame.node, frame.destination, &jsonValue{kind: jsonNull})
		case *ast.BoolNode:
			value, err := decodeYAMLBoolean(typed)
			if err != nil {
				return nil, yamlContextError(frame.context, err)
			}

			decoder.assignDecoded(frame.node, frame.destination, value)
		case *ast.IntegerNode, *ast.FloatNode:
			value, err := decodeYAMLNumber(typed.GetToken().Value)
			if err != nil {
				return nil, yamlContextError(frame.context, err)
			}

			decoder.assignDecoded(frame.node, frame.destination, value)
		case *ast.StringNode:
			value, err := decodeYAMLString(typed)
			if err != nil {
				return nil, yamlContextError(frame.context, err)
			}

			decoder.assignDecoded(frame.node, frame.destination, value)
		case *ast.LiteralNode:
			decoder.assignDecoded(
				frame.node,
				frame.destination,
				&jsonValue{kind: jsonString, text: typed.Value.Value},
			)
		case *ast.SequenceNode:
			value := &jsonValue{kind: jsonArray, array: make([]*jsonValue, len(typed.Values))}
			decoder.assignDecoded(frame.node, frame.destination, value)

			for index := len(typed.Values) - 1; index >= 0; index-- {
				stack = append(stack, yamlDecodeFrame{
					node:        typed.Values[index],
					destination: yamlDecodeDestination{parent: value, index: index},
					context:     fmt.Sprintf("YAML sequence element %d", index),
				})
			}
		case *ast.MappingNode:
			value := &jsonValue{kind: jsonObject, object: make(map[string]*jsonValue, len(typed.Values))}

			names := make([]string, len(typed.Values))
			for index, entry := range typed.Values {
				name, err := yamlMappingKey(entry.Key)
				if err != nil {
					return nil, yamlContextError(frame.context, fmt.Errorf(
						"YAML mapping key at entry %d: %w", index, err,
					))
				}

				if _, duplicate := value.object[name]; duplicate {
					return nil, yamlContextError(frame.context, fmt.Errorf("duplicate YAML mapping key %q", name))
				}

				value.object[name] = nil
				names[index] = name
			}

			decoder.assignDecoded(frame.node, frame.destination, value)

			for index := len(typed.Values) - 1; index >= 0; index-- {
				name := names[index]
				stack = append(stack, yamlDecodeFrame{
					node:        typed.Values[index].Value,
					destination: yamlDecodeDestination{parent: value, name: name},
					context:     fmt.Sprintf("YAML mapping member %q", name),
				})
			}
		case *ast.AnchorNode:
			name, err := yamlString(typed.Name)
			if err != nil {
				return nil, yamlContextError(frame.context, fmt.Errorf("YAML anchor name: %w", err))
			}

			decoder.anchors[name] = typed.Value
			if err := decoder.pushAliasTarget(&stack, name, typed.Value, frame.destination, frame.context); err != nil {
				return nil, err
			}
		case *ast.AliasNode:
			name, err := yamlString(typed.Value)
			if err != nil {
				return nil, yamlContextError(frame.context, fmt.Errorf("YAML alias name: %w", err))
			}

			target, exists := decoder.anchors[name]
			if !exists {
				return nil, yamlContextError(frame.context, fmt.Errorf("YAML alias %q has no preceding anchor", name))
			}

			if err := decoder.pushAliasTarget(&stack, name, target, frame.destination, frame.context); err != nil {
				return nil, err
			}
		case *ast.TagNode:
			value, nested, err := decodeYAMLTag(typed)
			if err != nil {
				return nil, yamlContextError(frame.context, err)
			}

			if nested != nil {
				stack = append(stack, yamlDecodeFrame{
					node: nested, destination: frame.destination, context: frame.context,
				})
			} else {
				decoder.assignDecoded(frame.node, frame.destination, value)
			}
		default:
			return nil, yamlContextError(frame.context, fmt.Errorf(
				"YAML node type %T is outside the YAML JSON schema", frame.node,
			))
		}
	}

	return decoder.result, nil
}

func (decoder *yamlJSONDecoder) pushAliasTarget(
	stack *[]yamlDecodeFrame,
	name string,
	target ast.Node,
	destination yamlDecodeDestination,
	context string,
) error {
	if decoder.active[target] {
		return yamlContextError(context, fmt.Errorf("YAML alias %q creates a cycle", name))
	}

	if value, decoded := decoder.decoded[target]; decoded {
		decoder.assign(destination, value)

		return nil
	}

	decoder.active[target] = true
	*stack = append(
		*stack,
		yamlDecodeFrame{node: target, exit: true},
		yamlDecodeFrame{node: target, destination: destination, context: context},
	)

	return nil
}

func (decoder *yamlJSONDecoder) assignDecoded(
	node ast.Node,
	destination yamlDecodeDestination,
	value *jsonValue,
) {
	decoder.decoded[node] = value
	decoder.assign(destination, value)
}

func (decoder *yamlJSONDecoder) assign(destination yamlDecodeDestination, value *jsonValue) {
	switch {
	case destination.root:
		decoder.result = value
	case destination.parent.kind == jsonArray:
		destination.parent.array[destination.index] = value
	case destination.parent.kind == jsonObject:
		destination.parent.object[destination.name] = value
	}
}

func yamlContextError(context string, err error) error {
	if context == "" {
		return err
	}

	return fmt.Errorf("%s: %w", context, err)
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

func decodeYAMLTag(node *ast.TagNode) (*jsonValue, ast.Node, error) {
	name := node.GetToken().Value
	switch name {
	case "!!map", "!<tag:yaml.org,2002:map>":
		if _, ok := node.Value.(*ast.MappingNode); !ok {
			return nil, nil, fmt.Errorf("YAML tag %q requires a mapping", name)
		}

		return nil, node.Value, nil
	case "!!seq", "!<tag:yaml.org,2002:seq>":
		if _, ok := node.Value.(*ast.SequenceNode); !ok {
			return nil, nil, fmt.Errorf("YAML tag %q requires a sequence", name)
		}

		return nil, node.Value, nil
	}

	value, scalar := node.Value.(ast.ScalarNode)
	if !scalar {
		return nil, nil, fmt.Errorf("YAML tag %q on a non-scalar is outside the YAML JSON schema", name)
	}

	source := value.GetToken().Value

	switch name {
	case "!!str", "!<tag:yaml.org,2002:str>":
		return &jsonValue{kind: jsonString, text: source}, nil, nil
	case "!!null", "!<tag:yaml.org,2002:null>":
		if source != "null" {
			return nil, nil, fmt.Errorf("YAML null %q is outside the YAML JSON schema", source)
		}

		return &jsonValue{kind: jsonNull}, nil, nil
	case "!!bool", "!<tag:yaml.org,2002:bool>":
		boolean, ok := node.Value.(*ast.BoolNode)
		if !ok {
			return nil, nil, fmt.Errorf("YAML boolean %q is outside the YAML JSON schema", source)
		}

		result, err := decodeYAMLBoolean(boolean)

		return result, nil, err
	case "!!int", "!!float", "!<tag:yaml.org,2002:int>", "!<tag:yaml.org,2002:float>":
		result, err := decodeYAMLNumber(source)

		return result, nil, err
	default:
		return nil, nil, fmt.Errorf("YAML tag %q is outside the YAML JSON schema", name)
	}
}

func yamlMappingKey(node ast.Node) (string, error) {
	if tagged, ok := node.(*ast.TagNode); ok {
		if tagged.GetToken().Value != "!!str" && tagged.GetToken().Value != "!<tag:yaml.org,2002:str>" {
			return "", errors.New("must be a scalar string")
		}

		scalar, ok := tagged.Value.(ast.ScalarNode)
		if !ok {
			return "", errors.New("must be a scalar string")
		}

		return scalar.GetToken().Value, nil
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
