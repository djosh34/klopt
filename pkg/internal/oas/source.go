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
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// semanticVersionPattern implements the Semantic Versioning 2.0.0 grammar.
var semanticVersionPattern = regexp.MustCompile(
	`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
		`(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)` +
		`(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?` +
		`(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$`,
)

// Source retains one parsed document and one acquired operation's request inputs.
type Source struct {
	Document            json.RawMessage
	PathTemplate        string
	RequestSchema       LocatedSchema
	RequestBodyRequired bool
	QueryParameters     []LocatedSchema
	PathParameters      []LocatedSchema
}

// LocatedSchema is raw JSON together with its canonical document pointer.
type LocatedSchema struct {
	Raw     json.RawMessage
	Pointer string
}

// ParameterValidator validates one independently resolved raw Parameter Object.
type ParameterValidator func(Source, LocatedSchema) error

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
	return parse(spec, nil)
}

// ParseWithParameterValidation validates every raw Parameter Object before merge or filtering.
func ParseWithParameterValidation(
	spec []byte,
	validateParameter ParameterValidator,
) (map[string]Source, error) {
	if validateParameter == nil {
		return nil, errors.New("parameter validator must not be nil")
	}

	return parse(spec, validateParameter)
}

// parse ingests and acquires one OpenAPI document with optional raw Parameter Object validation.
func parse(spec []byte, validateParameter ParameterValidator) (map[string]Source, error) {
	document := spec
	if json.Valid(spec) {
		if err := rejectDuplicateJSONNames(spec); err != nil {
			return nil, fmt.Errorf("parse OpenAPI document JSON: %w", err)
		}
	} else {
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
	if err := json.Unmarshal(root["openapi"], &version); err != nil {
		return nil, fmt.Errorf(
			"OpenAPI document version must be a Semantic Versioning 2.0.0 version: %w",
			err,
		)
	}

	versionParts := semanticVersionPattern.FindStringSubmatch(version)
	if len(versionParts) == 0 {
		return nil, errors.New("OpenAPI document version must be a Semantic Versioning 2.0.0 version")
	}

	if versionParts[1] != "3" || versionParts[2] != "0" {
		return nil, errors.New("OpenAPI document feature set must be 3.0")
	}

	source := Source{Document: append(json.RawMessage(nil), document...)}

	return source.collectRequests(root["paths"], validateParameter)
}

// rejectDuplicateJSONNames preflights decoded member-name uniqueness across one JSON document.
func rejectDuplicateJSONNames(document []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()

	if err := rejectDuplicateJSONValue(decoder, "#"); err != nil {
		return err
	}

	return nil
}

// rejectDuplicateJSONValue walks one complete value from the decoder.
//
//nolint:cyclop // JSON objects and arrays require separate recursive token handling.
func rejectDuplicateJSONValue(decoder *json.Decoder, pointer string) error {
	token, tokenErr := decoder.Token()
	if tokenErr != nil {
		return tokenErr
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})

		for decoder.More() {
			nameToken, nameTokenErr := decoder.Token()
			if nameTokenErr != nil {
				return nameTokenErr
			}

			name, ok := nameToken.(string)
			if !ok {
				return fmt.Errorf("object name at %s must be a string", pointer)
			}

			if _, duplicate := seen[name]; duplicate {
				return fmt.Errorf("duplicate object name %q at %s", name, pointer)
			}

			seen[name] = struct{}{}
			if childErr := rejectDuplicateJSONValue(decoder, appendPointer(pointer, name)); childErr != nil {
				return childErr
			}
		}
	case '[':
		for index := 0; decoder.More(); index++ {
			if childErr := rejectDuplicateJSONValue(
				decoder,
				appendPointer(pointer, strconv.Itoa(index)),
			); childErr != nil {
				return childErr
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q at %s", delimiter, pointer)
	}

	closing, closingErr := decoder.Token()
	if closingErr != nil {
		return closingErr
	}

	if closing != delimiter+2 {
		return fmt.Errorf("unexpected JSON delimiter %q at %s", closing, pointer)
	}

	return nil
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
func (source Source) collectRequests(
	pathsRaw json.RawMessage,
	validateParameter ParameterValidator,
) (map[string]Source, error) {
	var paths map[string]json.RawMessage
	if err := json.Unmarshal(pathsRaw, &paths); err != nil {
		return nil, fmt.Errorf("parse OpenAPI paths: %w", err)
	}

	if paths == nil {
		return nil, errors.New("parse OpenAPI paths: paths must be an object")
	}

	result := make(map[string]Source)
	locations := make(map[string]string)
	pathTemplates := make([]string, 0, len(paths))
	hierarchies := make(map[string]string, len(paths))
	expressionsByPath := make(map[string][]string, len(paths))

	for _, path := range slices.Sorted(maps.Keys(paths)) {
		if strings.HasPrefix(path, "x-") {
			continue
		}

		canonical, expressions, err := ParsePathTemplate(path)
		if err != nil {
			return nil, fmt.Errorf("parse OpenAPI path %q: %w", path, err)
		}

		if first, duplicate := hierarchies[canonical]; duplicate {
			return nil, fmt.Errorf(
				"OpenAPI paths %q and %q have an identical templated hierarchy",
				first,
				path,
			)
		}

		hierarchies[canonical] = path
		expressionsByPath[path] = expressions
		pathTemplates = append(pathTemplates, path)
	}

	for _, path := range pathTemplates {
		pathItem := LocatedSchema{Raw: paths[path], Pointer: appendPointer("#/paths", path)}

		resolved, err := source.Resolve(pathItem)
		if err != nil {
			return nil, fmt.Errorf("resolve OpenAPI path item %q: %w", path, err)
		}

		if validateErr := validatePathItem(resolved); validateErr != nil {
			return nil, fmt.Errorf("parse OpenAPI path item %q: %w", path, validateErr)
		}

		pathParameters, err := source.parameterList(resolved)
		if err != nil {
			return nil, fmt.Errorf("parse OpenAPI path item parameters for %q: %w", path, err)
		}

		if validationErr := source.validateParameters(pathParameters, validateParameter); validationErr != nil {
			return nil, fmt.Errorf("parse OpenAPI path item parameters for %q: %w", path, validationErr)
		}

		if correspondenceErr := validateDeclaredPathParameters(
			path,
			expressionsByPath[path],
			pathParameters,
		); correspondenceErr != nil {
			return nil, fmt.Errorf("parse OpenAPI path item parameters for %q: %w", path, correspondenceErr)
		}

		operations, err := operationChildren(resolved)
		if err != nil {
			return nil, fmt.Errorf("parse OpenAPI path item %q: %w", path, err)
		}

		for _, operation := range operations {
			operationSource, operationID, err := source.requestSource(
				path,
				expressionsByPath[path],
				pathParameters,
				operation.Schema,
				validateParameter,
			)
			if err != nil {
				return nil, err
			}

			if first, duplicate := locations[operationID]; duplicate {
				return nil, fmt.Errorf(
					"%w: operationId %q is duplicated at %s and %s",
					ErrInvalidOperationID,
					operationID,
					first,
					appendPointer(pathItem.Pointer, operation.Method),
				)
			}

			locations[operationID] = appendPointer(pathItem.Pointer, operation.Method)
			result[operationID] = operationSource
		}
	}

	return result, nil
}

// ParsePathTemplate returns a canonical hierarchy and expression order.
//
//nolint:cyclop // Each malformed brace/expression form needs a distinct Parse diagnostic.
func ParsePathTemplate(pathTemplate string) (string, []string, error) {
	if !strings.HasPrefix(pathTemplate, "/") {
		return "", nil, fmt.Errorf("path template %q must begin with /", pathTemplate)
	}

	canonical := make([]byte, 0, len(pathTemplate))
	seen := make(map[string]struct{})
	expressions := make([]string, 0)
	previousExpression := false

	for index := 0; index < len(pathTemplate); {
		switch pathTemplate[index] {
		case '}':
			return "", nil, fmt.Errorf("path template %q has an unmatched }", pathTemplate)
		case '{':
			if previousExpression {
				return "", nil, fmt.Errorf("path template %q has adjacent expressions", pathTemplate)
			}

			closing := strings.IndexByte(pathTemplate[index+1:], '}')
			if closing == -1 {
				return "", nil, fmt.Errorf("path template %q has an unmatched {", pathTemplate)
			}

			closing += index + 1

			name := pathTemplate[index+1 : closing]
			if name == "" {
				return "", nil, fmt.Errorf("path template %q has an empty expression", pathTemplate)
			}

			if strings.ContainsRune(name, '{') {
				return "", nil, fmt.Errorf("path template %q has a nested expression", pathTemplate)
			}

			if _, duplicate := seen[name]; duplicate {
				return "", nil, fmt.Errorf("path template %q repeats expression %q", pathTemplate, name)
			}

			seen[name] = struct{}{}
			expressions = append(expressions, name)

			canonical = append(canonical, '{', '}')

			index = closing + 1
			previousExpression = true
		default:
			canonical = append(canonical, pathTemplate[index])
			index++
			previousExpression = false
		}
	}

	return string(canonical), expressions, nil
}

// requestSource returns one acquired operation's request source.
//
//nolint:cyclop,nestif // Each request-body field needs its own malformed-input diagnostic or skip decision.
func (source Source) requestSource(
	pathTemplate string,
	pathExpressions []string,
	pathParameters []locatedParameter,
	operation LocatedSchema,
	validateParameter ParameterValidator,
) (Source, string, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(operation.Raw, &members); err != nil {
		return Source{}, "", fmt.Errorf("parse operation at %s: %w", operation.Pointer, err)
	}

	if members == nil {
		return Source{}, "", fmt.Errorf("parse operation at %s: operation must be an object", operation.Pointer)
	}

	operationParameters, parametersErr := source.parameterList(operation)
	if parametersErr != nil {
		return Source{}, "", fmt.Errorf(
			"operation at %s parameters: operation parameters: %w",
			operation.Pointer,
			parametersErr,
		)
	}

	if validationErr := source.validateParameters(operationParameters, validateParameter); validationErr != nil {
		return Source{}, "", fmt.Errorf("operation at %s parameters: %w", operation.Pointer, validationErr)
	}

	var operationID string
	if err := json.Unmarshal(members["operationId"], &operationID); err != nil || operationID == "" {
		return Source{}, "", fmt.Errorf(
			"operation at %s: operationId must be a non-empty string: %w",
			operation.Pointer,
			ErrInvalidOperationID,
		)
	}

	if _, err := RequestValidationName(operationID); err != nil {
		return Source{}, "", fmt.Errorf("operation at %s: %w", operation.Pointer, err)
	}

	queryParameters, mergedPathParameters, err := source.mergedParameters(
		pathTemplate,
		pathExpressions,
		pathParameters,
		operationParameters,
	)
	if err != nil {
		return Source{}, "", fmt.Errorf("operation at %s parameters: %w", operation.Pointer, err)
	}

	result := Source{
		Document:        source.Document,
		PathTemplate:    pathTemplate,
		QueryParameters: queryParameters,
		PathParameters:  mergedPathParameters,
	}

	requestBodyRaw, hasRequestBody := members["requestBody"]
	if hasRequestBody {
		requestBody := LocatedSchema{Raw: requestBodyRaw, Pointer: appendPointer(operation.Pointer, "requestBody")}

		requestBody, resolveErr := source.Resolve(requestBody)
		if resolveErr != nil {
			return Source{}, "", fmt.Errorf("operation at %s request body: %w", operation.Pointer, resolveErr)
		}

		var body map[string]json.RawMessage
		if unmarshalErr := json.Unmarshal(requestBody.Raw, &body); unmarshalErr != nil {
			return Source{}, "", fmt.Errorf(
				"parse operation at %s request body: %w",
				operation.Pointer,
				unmarshalErr,
			)
		}

		if body == nil {
			return Source{}, "", fmt.Errorf("parse operation at %s request body: must be an object", operation.Pointer)
		}

		required, requiredErr := optionalBoolean(body["required"], "required")
		if requiredErr != nil {
			return Source{}, "", fmt.Errorf("parse operation at %s request body: %w", operation.Pointer, requiredErr)
		}

		contentRaw, hasContent := body["content"]
		if !hasContent {
			return Source{}, "", fmt.Errorf(
				"parse operation at %s request body: content does not exist",
				operation.Pointer,
			)
		}

		var content map[string]json.RawMessage
		if unmarshalErr := json.Unmarshal(contentRaw, &content); unmarshalErr != nil {
			return Source{}, "", fmt.Errorf(
				"parse operation at %s request body content: %w",
				operation.Pointer,
				unmarshalErr,
			)
		}

		if content == nil {
			return Source{}, "", fmt.Errorf(
				"parse operation at %s request body: content must be an object",
				operation.Pointer,
			)
		}

		mediaTypeName, mediaTypeRaw, ok := applicationJSONMediaType(content)
		if ok {
			mediaType := LocatedSchema{
				Raw:     append(json.RawMessage(nil), mediaTypeRaw...),
				Pointer: appendPointer(requestBody.Pointer, "content", mediaTypeName),
			}

			schema, schemaErr := mediaTypeSchema(mediaType)
			if schemaErr != nil {
				return Source{}, "", fmt.Errorf(
					"parse operation at %s request body: %w",
					operation.Pointer,
					schemaErr,
				)
			}

			result.RequestSchema = schema
			result.RequestBodyRequired = required
		}
	}

	return result, operationID, nil
}

// validateParameters invokes the caller's validator for every independently resolved declaration.
func (source Source) validateParameters(
	parameters []locatedParameter,
	validateParameter ParameterValidator,
) error {
	if validateParameter == nil {
		return nil
	}

	for _, parameter := range parameters {
		if err := validateParameter(source, parameter.schema); err != nil {
			return fmt.Errorf("parameter at %s: %w", parameter.schema.Pointer, err)
		}
	}

	return nil
}

// validatePathParameterCorrespondence requires exact template/declaration ownership.
func validatePathParameterCorrespondence(
	pathTemplate string,
	expressions []string,
	parameters []locatedParameter,
) error {
	declared := make(map[string]struct{}, len(parameters))
	for _, parameter := range parameters {
		if parameter.identity.location != "path" {
			continue
		}

		if strings.ContainsRune(parameter.identity.name, '/') {
			return fmt.Errorf("path parameter %q must not contain /", parameter.identity.name)
		}

		declared[parameter.identity.name] = struct{}{}
	}

	for _, expression := range expressions {
		if _, ok := declared[expression]; !ok {
			return fmt.Errorf(
				"path template %q expression %q has no parameter declaration",
				pathTemplate,
				expression,
			)
		}

		delete(declared, expression)
	}

	if len(declared) != 0 {
		unused := slices.Sorted(maps.Keys(declared))[0]

		return fmt.Errorf(
			"path parameter %q has no expression in path template %q",
			unused,
			pathTemplate,
		)
	}

	return nil
}

// validateDeclaredPathParameters requires every Path Item path declaration to name one template expression.
func validateDeclaredPathParameters(
	pathTemplate string,
	expressions []string,
	parameters []locatedParameter,
) error {
	expressionSet := make(map[string]struct{}, len(expressions))
	for _, expression := range expressions {
		expressionSet[expression] = struct{}{}
	}

	for _, parameter := range parameters {
		if parameter.identity.location != "path" {
			continue
		}

		if strings.ContainsRune(parameter.identity.name, '/') {
			return fmt.Errorf("path parameter %q must not contain /", parameter.identity.name)
		}

		if _, ok := expressionSet[parameter.identity.name]; !ok {
			return fmt.Errorf(
				"path parameter %q has no expression in path template %q",
				parameter.identity.name,
				pathTemplate,
			)
		}
	}

	return nil
}

// validatePathItem rejects unknown non-extension fixed fields.
func validatePathItem(pathItem LocatedSchema) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(pathItem.Raw, &members); err != nil {
		return err
	}

	if members == nil {
		return errors.New("path item must be an object")
	}

	allowed := map[string]struct{}{
		"$ref": {}, "summary": {}, "description": {}, "get": {}, "put": {}, "post": {}, "delete": {},
		"options": {}, "head": {}, "patch": {}, "trace": {}, "servers": {}, "parameters": {},
	}
	for name := range members {
		if _, ok := allowed[name]; ok || strings.HasPrefix(name, "x-") {
			continue
		}

		return fmt.Errorf("unknown Path Item field %q at %s", name, pathItem.Pointer)
	}

	return nil
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

// mediaTypeSchema returns a selected Media Type Object's explicit or synthetic empty schema.
func mediaTypeSchema(mediaType LocatedSchema) (LocatedSchema, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(mediaType.Raw, &members); err != nil {
		return LocatedSchema{}, fmt.Errorf(
			"parse Media Type Object at %s: must be an object: %w",
			mediaType.Pointer,
			err,
		)
	}

	if members == nil {
		return LocatedSchema{}, fmt.Errorf(
			"parse Media Type Object at %s: must be an object",
			mediaType.Pointer,
		)
	}

	raw, ok := members["schema"]
	if !ok {
		raw = json.RawMessage(`{}`)
	}

	return LocatedSchema{
		Raw:     append(json.RawMessage(nil), raw...),
		Pointer: appendPointer(mediaType.Pointer, "schema"),
	}, nil
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
