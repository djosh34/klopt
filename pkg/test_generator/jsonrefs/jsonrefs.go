package jsonrefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type Node interface {
	json.Marshaler
	GetPathPart(p string) (Node, error)
	resolve(root Node, stack []string) (Node, error)
}

type noPath struct{}

type ObjectNode struct {
	Map map[string]Node
}

type ArrayNode struct {
	noPath `json:"-"`
	Items  []Node
}

type LeafNode struct {
	noPath `json:"-"`
	json.RawMessage
}

type RefNode struct {
	noPath `json:"-"`
	Ref    string
}

func Replace(raw *json.RawMessage) (*json.RawMessage, error) {
	if raw == nil {
		return nil, errors.New("json raw message is nil")
	}

	root, err := unmarshalNode(*raw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json as node: %w", err)
	}

	resolved, err := root.resolve(root, nil)
	if err != nil {
		return nil, fmt.Errorf("replace json refs: %w", err)
	}

	resolvedBytes, err := json.Marshal(resolved)
	if err != nil {
		return nil, fmt.Errorf("marshal resolved node: %w", err)
	}

	resolvedRaw := json.RawMessage(resolvedBytes)
	return &resolvedRaw, nil
}

func unmarshalNode(data []byte) (Node, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty json")
	}

	switch trimmed[0] {
	case '{':
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(data, &rawMap); err != nil {
			return nil, fmt.Errorf("unmarshal object: %w", err)
		}

		if _, ok := rawMap["$ref"]; ok {
			node := new(RefNode)
			if err := json.Unmarshal(data, node); err != nil {
				return nil, err
			}
			return node, nil
		}

		node := new(ObjectNode)
		if err := json.Unmarshal(data, node); err != nil {
			return nil, err
		}
		return node, nil
	case '[':
		node := new(ArrayNode)
		if err := json.Unmarshal(data, node); err != nil {
			return nil, err
		}
		return node, nil
	default:
		node := new(LeafNode)
		if err := json.Unmarshal(data, node); err != nil {
			return nil, err
		}
		return node, nil
	}
}

func (n *ObjectNode) UnmarshalJSON(data []byte) error {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return fmt.Errorf("unmarshal object: %w", err)
	}

	n.Map = make(map[string]Node, len(rawMap))
	for key, rawValue := range rawMap {
		child, err := unmarshalNode(rawValue)
		if err != nil {
			return fmt.Errorf("unmarshal object key %q: %w", key, err)
		}
		n.Map[key] = child
	}

	return nil
}

func (n *ObjectNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.Map)
}

func (n *ObjectNode) GetPathPart(p string) (Node, error) {
	child, ok := n.Map[p]
	if !ok {
		return nil, fmt.Errorf("path part %q not found", p)
	}
	return child, nil
}

func (n *ObjectNode) resolve(root Node, stack []string) (Node, error) {
	for key, child := range n.Map {
		resolved, err := child.resolve(root, stack)
		if err != nil {
			return nil, fmt.Errorf("resolve object key %q: %w", key, err)
		}
		n.Map[key] = resolved
	}

	return n, nil
}

func (n *ArrayNode) UnmarshalJSON(data []byte) error {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return fmt.Errorf("unmarshal array: %w", err)
	}

	n.Items = make([]Node, len(rawItems))
	for i, rawItem := range rawItems {
		child, err := unmarshalNode(rawItem)
		if err != nil {
			return fmt.Errorf("unmarshal array index %d: %w", i, err)
		}
		n.Items[i] = child
	}

	return nil
}

func (n *ArrayNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.Items)
}

func (n *ArrayNode) resolve(root Node, stack []string) (Node, error) {
	for i, child := range n.Items {
		resolved, err := child.resolve(root, stack)
		if err != nil {
			return nil, fmt.Errorf("resolve array index %d: %w", i, err)
		}
		n.Items[i] = resolved
	}

	return n, nil
}

func (n *LeafNode) resolve(root Node, stack []string) (Node, error) {
	return n, nil
}

func (n *RefNode) UnmarshalJSON(data []byte) error {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return fmt.Errorf("unmarshal ref object: %w", err)
	}

	rawRef, ok := rawMap["$ref"]
	if !ok {
		return errors.New("ref object does not contain $ref")
	}

	if err := json.Unmarshal(rawRef, &n.Ref); err != nil {
		return fmt.Errorf("unmarshal $ref string: %w", err)
	}

	return nil
}

func (n *RefNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"$ref": n.Ref})
}

func (n *RefNode) resolve(root Node, stack []string) (Node, error) {
	for _, seen := range stack {
		if seen == n.Ref {
			return nil, fmt.Errorf("reference cycle for %q", n.Ref)
		}
	}

	if !strings.HasPrefix(n.Ref, "#/") {
		return nil, fmt.Errorf("$ref %q is invalid: must start with #/", n.Ref)
	}

	parsed, err := url.Parse(n.Ref)
	if err != nil {
		return nil, fmt.Errorf("parse $ref %q: %w", n.Ref, err)
	}

	node := root
	for _, rawPart := range strings.Split(parsed.Fragment[1:], "/") {
		part, err := unescapePathPart(rawPart)
		if err != nil {
			return nil, fmt.Errorf("unescape $ref %q path part %q: %w", n.Ref, rawPart, err)
		}

		node, err = node.GetPathPart(part)
		if err != nil {
			return nil, fmt.Errorf("get $ref %q path part %q: %w", n.Ref, part, err)
		}
		if node == nil {
			return nil, fmt.Errorf("get $ref %q path part %q: node is nil", n.Ref, part)
		}
	}

	return node.resolve(root, append(stack, n.Ref))
}

func (noPath) GetPathPart(p string) (Node, error) {
	return nil, fmt.Errorf("cannot get path part %q from non-object", p)
}

func unescapePathPart(part string) (string, error) {
	unescaped := make([]byte, 0, len(part))

	for i := 0; i < len(part); i++ {
		if part[i] != '~' {
			unescaped = append(unescaped, part[i])
			continue
		}

		if i+1 >= len(part) {
			return "", errors.New("~ must be followed by 0 or 1")
		}

		switch part[i+1] {
		case '0':
			unescaped = append(unescaped, '~')
		case '1':
			unescaped = append(unescaped, '/')
		default:
			return "", fmt.Errorf("~%c is invalid", part[i+1])
		}
		i++
	}

	return string(unescaped), nil
}
