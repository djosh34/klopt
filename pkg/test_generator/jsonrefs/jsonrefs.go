// Package jsonrefs provides utilities for resolving JSON references.
package jsonrefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Node is a JSON value that can be marshaled and traversed by JSON Pointer.
type Node interface {
	json.Marshaler

	// GetPathPart returns the child selected by one JSON Pointer token.
	GetPathPart(p string) (Node, error)
	resolve(root Node, stack []string) (Node, error)
}

// noPath rejects JSON Pointer traversal for scalar nodes.
type noPath struct{}

// ObjectNode is a JSON object node.
type ObjectNode struct {
	// Map contains the object's members.
	Map map[string]Node
}

// ArrayNode is a JSON array node.
type ArrayNode struct {
	// Items contains the array's elements.
	Items []Node
}

// LeafNode is a scalar JSON node.
type LeafNode struct {
	noPath `json:"-"`
	json.RawMessage
}

// RefNode is a JSON Reference Object.
type RefNode struct {
	noPath `json:"-"`

	// Ref is the reference URI.
	Ref string
}

// Replace resolves every in-document JSON reference in raw.
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

// ResolveReference resolves reference against root without resolving references nested in its target.
// An inline value is returned unchanged.
func ResolveReference(root *json.RawMessage, reference *json.RawMessage) (*json.RawMessage, error) {
	if root == nil {
		return nil, errors.New("json root message is nil")
	}

	if reference == nil {
		return nil, errors.New("json reference message is nil")
	}

	ref, isReference, err := rawReference(*reference)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json reference: %w", err)
	}

	if !isReference {
		inline := append(json.RawMessage(nil), (*reference)...)

		return &inline, nil
	}

	var validatedRoot json.RawMessage

	err = json.Unmarshal(*root, &validatedRoot)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json root: %w", err)
	}

	resolved, err := resolveRawReference(*root, ref)
	if err != nil {
		return nil, fmt.Errorf("resolve json reference: %w", err)
	}

	return &resolved, nil
}

// rawReference returns the $ref from a top-level Reference Object.
func rawReference(raw json.RawMessage) (string, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", false, errors.New("empty json")
	}

	if trimmed[0] != '{' {
		var validated json.RawMessage
		if err := json.Unmarshal(raw, &validated); err != nil {
			return "", false, err
		}

		return "", false, nil
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return "", false, fmt.Errorf("unmarshal object: %w", err)
	}

	rawRef, ok := rawMap["$ref"]
	if !ok {
		return "", false, nil
	}

	var ref string
	if err := json.Unmarshal(rawRef, &ref); err != nil {
		return "", false, fmt.Errorf("unmarshal $ref string: %w", err)
	}

	return ref, true, nil
}

// resolveRawReference follows a Reference Object chain without decoding nested values.
func resolveRawReference(root json.RawMessage, ref string) (json.RawMessage, error) {
	var stack []string

	for {
		for _, seen := range stack {
			if seen == ref {
				return nil, fmt.Errorf("reference cycle for %q", ref)
			}
		}

		target, err := rawReferenceTarget(root, ref)
		if err != nil {
			return nil, err
		}

		stack = append(stack, ref)

		nextRef, isReference, err := rawReference(target)
		if err != nil {
			return nil, fmt.Errorf("unmarshal referenced value: %w", err)
		}

		if !isReference {
			return append(json.RawMessage(nil), target...), nil
		}

		ref = nextRef
	}
}

// rawReferenceTarget traverses root using the JSON Pointer in ref.
func rawReferenceTarget(root json.RawMessage, ref string) (json.RawMessage, error) {
	parts, err := referencePathParts(ref)
	if err != nil {
		return nil, err
	}

	node := root

	for _, part := range parts {
		node, err = rawPathPart(node, part)
		if err != nil {
			return nil, fmt.Errorf("get $ref %q path part %q: %w", ref, part, err)
		}
	}

	return node, nil
}

// rawPathPart returns one child without decoding any other child values.
func rawPathPart(raw json.RawMessage, part string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("empty json")
	}

	switch trimmed[0] {
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, fmt.Errorf("unmarshal object: %w", err)
		}

		child, ok := object[part]
		if !ok {
			return nil, fmt.Errorf("path part %q not found", part)
		}

		return child, nil
	case '[':
		var array []json.RawMessage
		if err := json.Unmarshal(raw, &array); err != nil {
			return nil, fmt.Errorf("unmarshal array: %w", err)
		}

		index, err := parseArrayIndex(part, len(array))
		if err != nil {
			return nil, err
		}

		return array[index], nil
	default:
		return nil, fmt.Errorf("cannot get path part %q from non-object", part)
	}
}

// unmarshalNode decodes one JSON value into its matching node type.
func unmarshalNode(data []byte) (Node, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty json")
	}

	switch trimmed[0] {
	case '{':
		return unmarshalObjectNode(data)
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

// unmarshalObjectNode distinguishes Reference Objects from regular objects.
func unmarshalObjectNode(data []byte) (Node, error) {
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
}

// UnmarshalJSON decodes a JSON object into child nodes.
func (n *ObjectNode) UnmarshalJSON(data []byte) error {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return fmt.Errorf("unmarshal object: %w", err)
	}

	n.Map = make(map[string]Node, len(rawMap))
	for _, key := range sortedRawKeys(rawMap) {
		rawValue := rawMap[key]

		child, err := unmarshalNode(rawValue)
		if err != nil {
			return fmt.Errorf("unmarshal object key %q: %w", key, err)
		}

		n.Map[key] = child
	}

	return nil
}

// MarshalJSON encodes an object node.
func (n *ObjectNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.Map)
}

// GetPathPart returns an object member by name.
func (n *ObjectNode) GetPathPart(p string) (Node, error) {
	child, ok := n.Map[p]
	if !ok {
		return nil, fmt.Errorf("path part %q not found", p)
	}

	return child, nil
}

// resolve recursively replaces references in an object node.
func (n *ObjectNode) resolve(root Node, stack []string) (Node, error) {
	keys := make([]string, 0, len(n.Map))
	for key := range n.Map {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		child := n.Map[key]

		resolved, err := child.resolve(root, stack)
		if err != nil {
			return nil, fmt.Errorf("resolve object key %q: %w", key, err)
		}

		n.Map[key] = resolved
	}

	return n, nil
}

// UnmarshalJSON decodes a JSON array into child nodes.
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

// MarshalJSON encodes an array node.
func (n *ArrayNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.Items)
}

// GetPathPart returns an array element selected by a canonical decimal index.
func (n *ArrayNode) GetPathPart(p string) (Node, error) {
	index, err := parseArrayIndex(p, len(n.Items))
	if err != nil {
		return nil, err
	}

	return n.Items[index], nil
}

// parseArrayIndex validates a JSON Pointer array index.
func parseArrayIndex(p string, length int) (int, error) {
	if p == "" {
		return 0, errors.New("array index is empty")
	}

	if len(p) > 1 && p[0] == '0' {
		return 0, fmt.Errorf("array index %q has a leading zero", p)
	}

	for i := range p {
		if p[i] < '0' || p[i] > '9' {
			return 0, fmt.Errorf("array index %q is not a non-negative integer", p)
		}
	}

	index, err := strconv.ParseUint(p, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse array index %q: %w", p, err)
	}

	if index >= uint64(length) {
		return 0, fmt.Errorf("array index %q is out of bounds", p)
	}

	return int(index), nil
}

// resolve recursively replaces references in an array node.
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

// resolve returns a scalar node unchanged.
func (n *LeafNode) resolve(_ Node, _ []string) (Node, error) {
	return n, nil
}

// UnmarshalJSON decodes a JSON Reference Object.
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

// MarshalJSON encodes a JSON Reference Object.
func (n *RefNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"$ref": n.Ref})
}

// resolve recursively replaces a reference and references nested in its target.
func (n *RefNode) resolve(root Node, stack []string) (Node, error) {
	node, nextStack, err := n.referenceTarget(root, stack)
	if err != nil {
		return nil, err
	}

	return node.resolve(root, nextStack)
}

// referenceTarget returns the referenced node and updated cycle-detection stack.
func (n *RefNode) referenceTarget(root Node, stack []string) (Node, []string, error) {
	for _, seen := range stack {
		if seen == n.Ref {
			return nil, nil, fmt.Errorf("reference cycle for %q", n.Ref)
		}
	}

	parts, err := referencePathParts(n.Ref)
	if err != nil {
		return nil, nil, err
	}

	node := root

	for _, part := range parts {
		node, err = node.GetPathPart(part)
		if err != nil {
			return nil, nil, fmt.Errorf("get $ref %q path part %q: %w", n.Ref, part, err)
		}

		if node == nil {
			return nil, nil, fmt.Errorf("get $ref %q path part %q: node is nil", n.Ref, part)
		}
	}

	return node, append(stack, n.Ref), nil
}

// referencePathParts parses the JSON Pointer tokens from an in-document reference.
func referencePathParts(ref string) ([]string, error) {
	parsed, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("parse $ref %q: %w", ref, err)
	}

	if validationErr := validateReferenceURL(ref, parsed); validationErr != nil {
		return nil, validationErr
	}

	if parsed.Fragment == "" {
		return nil, nil
	}

	rawParts := strings.Split(parsed.Fragment[1:], "/")
	parts := make([]string, len(rawParts))

	for i, rawPart := range rawParts {
		parts[i], err = unescapePathPart(rawPart)
		if err != nil {
			return nil, fmt.Errorf("unescape $ref %q path part %q: %w", ref, rawPart, err)
		}
	}

	return parts, nil
}

// validateReferenceURL restricts references to in-document JSON Pointers.
func validateReferenceURL(ref string, parsed *url.URL) error {
	if parsed.Scheme != "" || parsed.Host != "" || parsed.Path != "" || parsed.RawQuery != "" ||
		parsed.Fragment == "" && ref != "#" ||
		parsed.Fragment != "" && !strings.HasPrefix(parsed.Fragment, "/") {
		return fmt.Errorf("$ref %q is invalid: must be an in-document JSON Pointer", ref)
	}

	return nil
}

// sortedRawKeys returns raw object keys in lexical order.
func sortedRawKeys(rawMap map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(rawMap))
	for key := range rawMap {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

// GetPathPart rejects JSON Pointer traversal on scalar nodes.
func (noPath) GetPathPart(p string) (Node, error) {
	return nil, fmt.Errorf("cannot get path part %q from non-object", p)
}

// unescapePathPart decodes one JSON Pointer token.
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
