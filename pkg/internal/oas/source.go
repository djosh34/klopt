// Package oas locates request schemas and resolves local OpenAPI references.
package oas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"mime"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// Source retains one parsed document and one request Schema Object.
type Source struct {
	Document            json.RawMessage
	RequestSchema       LocatedSchema
	RequestBodyRequired bool
}

// LocatedSchema is raw JSON together with its canonical document pointer.
type LocatedSchema struct {
	Raw     json.RawMessage
	Pointer string
}

// ReferenceError describes a failed local reference chain.
type ReferenceError struct {
	Referrer  string
	Reference string
	Chain     []string
	Cause     error
}

// Error formats reference location and chain context.
func (referenceError *ReferenceError) Error() string {
	if len(referenceError.Chain) == 0 {
		return fmt.Sprintf(
			"resolve reference %q from %s: %v",
			referenceError.Reference,
			referenceError.Referrer,
			referenceError.Cause,
		)
	}

	return fmt.Sprintf(
		"resolve reference %q from %s through %s: %v",
		referenceError.Reference,
		referenceError.Referrer,
		strings.Join(referenceError.Chain, " -> "),
		referenceError.Cause,
	)
}

// Unwrap returns the underlying reference failure.
func (referenceError *ReferenceError) Unwrap() error {
	return referenceError.Cause
}

// Parse parses YAML once and collects every application/json request Schema Object.
func Parse(spec []byte) (map[string]Source, error) {
	document := spec
	if !json.Valid(spec) {
		var err error

		document, err = yaml.YAMLToJSON(spec)
		if err != nil {
			return nil, fmt.Errorf("parse OpenAPI YAML: %w", err)
		}
	}

	var root map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(document, &root); unmarshalErr != nil {
		return nil, fmt.Errorf("parse OpenAPI document JSON: %w", unmarshalErr)
	}

	if root == nil {
		return nil, errors.New("OpenAPI document must be an object")
	}

	var version string
	if err := json.Unmarshal(root["openapi"], &version); err != nil || version != "3.0.3" {
		return nil, errors.New(`OpenAPI document version must be "3.0.3"`)
	}

	source := Source{Document: append(json.RawMessage(nil), document...)}

	return source.collectRequests(root["paths"])
}

// Resolve follows a local Reference Object chain and ignores all Reference Object siblings.
func (source Source) Resolve(schema LocatedSchema) (LocatedSchema, error) {
	current := LocatedSchema{Raw: append(json.RawMessage(nil), schema.Raw...), Pointer: schema.Pointer}
	seen := make(map[string]struct{})
	chain := make([]string, 0)

	for {
		reference, isReference, err := referenceFrom(current.Raw)
		if err != nil {
			return LocatedSchema{}, newReferenceError(current.Pointer, reference, chain, err)
		}

		if !isReference {
			return current, nil
		}

		if _, cycle := seen[reference]; cycle {
			return LocatedSchema{}, newReferenceError(
				current.Pointer,
				reference,
				append(chain, reference),
				errors.New("reference cycle"),
			)
		}

		seen[reference] = struct{}{}
		chain = append(chain, reference)

		target, targetErr := source.At(reference)
		if targetErr != nil {
			return LocatedSchema{}, newReferenceError(current.Pointer, reference, chain, targetErr)
		}

		current = target
	}
}

// At returns the value selected by a local JSON Pointer.
func (source Source) At(pointer string) (LocatedSchema, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return LocatedSchema{}, err
	}

	raw := source.Document
	canonical := "#"

	for _, token := range tokens {
		raw, err = childRaw(raw, token)
		if err != nil {
			return LocatedSchema{}, fmt.Errorf("pointer %s token %q: %w", canonical, token, err)
		}

		canonical = appendPointer(canonical, token)
	}

	return LocatedSchema{Raw: append(json.RawMessage(nil), raw...), Pointer: canonical}, nil
}

// Child returns a directly nested value with its canonical pointer.
func (source Source) Child(parent LocatedSchema, tokens ...string) (LocatedSchema, error) {
	current := LocatedSchema{Raw: parent.Raw, Pointer: parent.Pointer}

	for _, token := range tokens {
		var err error

		current.Raw, err = childRaw(current.Raw, token)
		if err != nil {
			return LocatedSchema{}, fmt.Errorf("pointer %s child %q: %w", current.Pointer, token, err)
		}

		current.Pointer = appendPointer(current.Pointer, token)
	}

	current.Raw = append(json.RawMessage(nil), current.Raw...)

	return current, nil
}

// collectRequests walks all path operations in deterministic path and method order.
//
//nolint:cyclop // Path, method, inclusion, and duplicate handling are one deterministic collection pass.
func (source Source) collectRequests(pathsRaw json.RawMessage) (map[string]Source, error) {
	var paths map[string]json.RawMessage
	if err := json.Unmarshal(pathsRaw, &paths); err != nil {
		return nil, fmt.Errorf("parse OpenAPI paths: %w", err)
	}

	if paths == nil {
		return nil, errors.New("parse OpenAPI paths: paths must be an object")
	}

	result := make(map[string]Source)
	locations := make(map[string]string)

	for _, path := range slices.Sorted(maps.Keys(paths)) {
		if strings.HasPrefix(path, "x-") {
			continue
		}

		pathItem := LocatedSchema{Raw: paths[path], Pointer: appendPointer("#/paths", path)}

		resolved, err := source.Resolve(pathItem)
		if err != nil {
			return nil, fmt.Errorf("resolve OpenAPI path item %q: %w", path, err)
		}

		operations, err := operationChildren(resolved)
		if err != nil {
			return nil, fmt.Errorf("parse OpenAPI path item %q: %w", path, err)
		}

		for _, operation := range operations {
			operationSource, operationID, included, err := source.requestSource(operation.Schema)
			if err != nil {
				return nil, err
			}

			if operationID != "" {
				if first, duplicate := locations[operationID]; duplicate {
					return nil, fmt.Errorf(
						"operationId %q is duplicated at %s and %s",
						operationID,
						first,
						appendPointer(pathItem.Pointer, operation.Method),
					)
				}

				locations[operationID] = appendPointer(pathItem.Pointer, operation.Method)
			}

			if !included {
				continue
			}

			result[operationID] = operationSource
		}
	}

	return result, nil
}

// requestSource returns one operation's JSON request-body source when it qualifies.
//
//nolint:cyclop // Each request-body field needs its own malformed-input diagnostic or skip decision.
func (source Source) requestSource(operation LocatedSchema) (Source, string, bool, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(operation.Raw, &members); err != nil {
		return Source{}, "", false, fmt.Errorf("parse operation at %s: %w", operation.Pointer, err)
	}

	if members == nil {
		return Source{}, "", false, fmt.Errorf("parse operation at %s: operation must be an object", operation.Pointer)
	}

	operationIDRaw, hasOperationID := members["operationId"]

	var (
		operationID    string
		operationIDErr error
	)
	if hasOperationID {
		operationIDErr = json.Unmarshal(operationIDRaw, &operationID)
	}

	if operationIDErr != nil {
		operationID = ""
	}

	requestBodyRaw, hasRequestBody := members["requestBody"]
	if !hasRequestBody {
		return Source{}, operationID, false, nil
	}

	requestBody := LocatedSchema{Raw: requestBodyRaw, Pointer: appendPointer(operation.Pointer, "requestBody")}

	requestBody, resolveErr := source.Resolve(requestBody)
	if resolveErr != nil {
		return Source{}, "", false, fmt.Errorf("operation at %s request body: %w", operation.Pointer, resolveErr)
	}

	var body map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(requestBody.Raw, &body); unmarshalErr != nil {
		return Source{}, "", false, fmt.Errorf(
			"parse operation at %s request body: %w",
			operation.Pointer,
			unmarshalErr,
		)
	}

	if body == nil {
		return Source{}, "", false, fmt.Errorf("parse operation at %s request body: must be an object", operation.Pointer)
	}

	required, err := optionalBoolean(body["required"], "required")
	if err != nil {
		return Source{}, "", false, fmt.Errorf("parse operation at %s request body: %w", operation.Pointer, err)
	}

	contentRaw, hasContent := body["content"]
	if !hasContent {
		return Source{}, "", false, fmt.Errorf(
			"parse operation at %s request body: content does not exist",
			operation.Pointer,
		)
	}

	var content map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(contentRaw, &content); unmarshalErr != nil {
		return Source{}, "", false, fmt.Errorf(
			"parse operation at %s request body content: %w",
			operation.Pointer,
			unmarshalErr,
		)
	}

	if content == nil {
		return Source{}, "", false, fmt.Errorf(
			"parse operation at %s request body: content must be an object",
			operation.Pointer,
		)
	}

	mediaTypeName, mediaTypeRaw, ok := applicationJSONMediaType(content)
	if !ok {
		return Source{}, operationID, false, nil
	}

	if !hasOperationID {
		return Source{}, "", false, fmt.Errorf("operation at %s: operationId must be a non-empty string", operation.Pointer)
	}

	if operationIDErr != nil {
		return Source{}, "", false, fmt.Errorf(
			"operation at %s: operationId must be a non-empty string: %w",
			operation.Pointer,
			operationIDErr,
		)
	}

	if operationID == "" {
		return Source{}, "", false, fmt.Errorf("operation at %s: operationId must be a non-empty string", operation.Pointer)
	}

	mediaType := LocatedSchema{
		Raw:     append(json.RawMessage(nil), mediaTypeRaw...),
		Pointer: appendPointer(requestBody.Pointer, "content", mediaTypeName),
	}

	schema, err := source.requiredChild(mediaType, "schema")
	if err != nil {
		return Source{}, "", false, fmt.Errorf("operationId %q application/json schema: %w", operationID, err)
	}

	return Source{
		Document:            source.Document,
		RequestSchema:       schema,
		RequestBodyRequired: required,
	}, operationID, true, nil
}

// optionalBoolean decodes an absent-or-boolean field without accepting null.
func optionalBoolean(raw json.RawMessage, name string) (bool, error) {
	if raw == nil {
		return false, nil
	}

	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, fmt.Errorf("%s must be a boolean", name)
	}

	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}

	return value, nil
}

// applicationJSONMediaType selects the most specific content entry matching application/json.
func applicationJSONMediaType(content map[string]json.RawMessage) (string, json.RawMessage, bool) {
	names := slices.Sorted(maps.Keys(content))

	for _, match := range []string{"application/json", "application/*", "*/*"} {
		if raw, ok := content[match]; ok {
			return match, raw, true
		}

		for _, name := range names {
			mediaType, _, err := mime.ParseMediaType(name)
			if err == nil && mediaType == match {
				return name, content[name], true
			}
		}
	}

	return "", nil, false
}

// requiredChild returns a present, non-null object member.
func (source Source) requiredChild(parent LocatedSchema, name string) (LocatedSchema, error) {
	child, err := source.Child(parent, name)
	if err != nil {
		return LocatedSchema{}, fmt.Errorf("%s does not exist: %w", name, err)
	}

	trimmed := bytes.TrimSpace(child.Raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return LocatedSchema{}, fmt.Errorf("%s does not exist", name)
	}

	return child, nil
}

// operationChild retains the resolved operation and its HTTP method.
type operationChild struct {
	Schema LocatedSchema
	Method string
}

// operationChildren returns operation members in deterministic method order.
func operationChildren(pathItem LocatedSchema) ([]operationChild, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(pathItem.Raw, &members); err != nil {
		return nil, err
	}

	if members == nil {
		return nil, errors.New("path item must be an object")
	}

	methods := []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

	operations := make([]operationChild, 0, len(methods))
	for _, method := range methods {
		if raw, ok := members[method]; ok {
			operations = append(operations, operationChild{
				Schema: LocatedSchema{
					Raw:     raw,
					Pointer: appendPointer(pathItem.Pointer, method),
				},
				Method: method,
			})
		}
	}

	return operations, nil
}

// referenceFrom recognizes an OpenAPI Reference Object.
func referenceFrom(raw json.RawMessage) (string, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", false, errors.New("empty JSON value")
	}

	if trimmed[0] != '{' {
		return "", false, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", false, err
	}

	referenceRaw, ok := object["$ref"]
	if !ok {
		return "", false, nil
	}

	var reference string
	if err := json.Unmarshal(referenceRaw, &reference); err != nil {
		return "", true, fmt.Errorf("$ref must be a string: %w", err)
	}

	return reference, true, nil
}

// pointerTokens parses one local URI fragment JSON Pointer.
func pointerTokens(reference string) ([]string, error) {
	parsed, err := url.Parse(reference)
	if err != nil {
		return nil, fmt.Errorf("parse reference %q: %w", reference, err)
	}

	if err := validateLocalReference(reference, parsed); err != nil {
		return nil, err
	}

	if reference == "#" {
		return nil, nil
	}

	rawTokens := strings.Split(parsed.Fragment[1:], "/")

	tokens := make([]string, len(rawTokens))
	for index, rawToken := range rawTokens {
		token, err := unescapeToken(rawToken)
		if err != nil {
			return nil, fmt.Errorf("reference %q token %q: %w", reference, rawToken, err)
		}

		tokens[index] = token
	}

	return tokens, nil
}

// validateLocalReference rejects external and non-pointer references.
func validateLocalReference(reference string, parsed *url.URL) error {
	if parsed.Scheme != "" || parsed.Host != "" || parsed.Path != "" || parsed.RawQuery != "" {
		return fmt.Errorf("external reference %q is unsupported", reference)
	}

	if reference != "#" && (parsed.Fragment == "" || !strings.HasPrefix(parsed.Fragment, "/")) {
		return fmt.Errorf("reference %q must be a local JSON Pointer", reference)
	}

	return nil
}

// unescapeToken decodes the two JSON Pointer escape sequences.
func unescapeToken(token string) (string, error) {
	decoded := make([]byte, 0, len(token))

	for index := 0; index < len(token); index++ {
		if token[index] != '~' {
			decoded = append(decoded, token[index])

			continue
		}

		if index+1 >= len(token) {
			return "", errors.New("~ must be followed by 0 or 1")
		}

		switch token[index+1] {
		case '0':
			decoded = append(decoded, '~')
		case '1':
			decoded = append(decoded, '/')
		default:
			return "", fmt.Errorf("~%c is invalid", token[index+1])
		}

		index++
	}

	return string(decoded), nil
}

// childRaw selects one object member or array element.
func childRaw(parent json.RawMessage, token string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(parent)
	if len(trimmed) == 0 {
		return nil, errors.New("empty JSON value")
	}

	switch trimmed[0] {
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(parent, &object); err != nil {
			return nil, err
		}

		child, ok := object[token]
		if !ok {
			return nil, fmt.Errorf("member %q does not exist", token)
		}

		return child, nil
	case '[':
		var array []json.RawMessage
		if err := json.Unmarshal(parent, &array); err != nil {
			return nil, err
		}

		index, err := arrayIndex(token, len(array))
		if err != nil {
			return nil, err
		}

		return array[index], nil
	default:
		return nil, fmt.Errorf("cannot select %q from a scalar", token)
	}
}

// arrayIndex parses a canonical JSON Pointer array index.
func arrayIndex(token string, length int) (int, error) {
	if token == "" || len(token) > 1 && token[0] == '0' {
		return 0, fmt.Errorf("invalid array index %q", token)
	}

	index, err := strconv.ParseUint(token, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q: %w", token, err)
	}

	if index >= uint64(length) {
		return 0, fmt.Errorf("array index %q is out of bounds", token)
	}

	return int(index), nil
}

// appendPointer appends escaped tokens to a canonical pointer.
func appendPointer(pointer string, tokens ...string) string {
	for _, token := range tokens {
		escaped := strings.ReplaceAll(token, "~", "~0")
		escaped = strings.ReplaceAll(escaped, "/", "~1")
		pointer += "/" + escaped
	}

	return pointer
}

// newReferenceError copies mutable chain data into a ReferenceError.
func newReferenceError(referrer string, reference string, chain []string, cause error) *ReferenceError {
	return &ReferenceError{
		Referrer:  referrer,
		Reference: reference,
		Chain:     append([]string(nil), chain...),
		Cause:     cause,
	}
}
