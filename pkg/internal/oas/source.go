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
const semanticVersionPattern = `^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
	`(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)` +
	`(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?` +
	`(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$`

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

// DocumentValidator validates the normalized document before semantic compilation.
type DocumentValidator func(json.RawMessage) error

// ParameterValidator validates one independently resolved raw Parameter Object.
type ParameterValidator func(Source, LocatedSchema) error

// ReferenceError describes a failed local reference chain.
type ReferenceError struct {
	Referrer        string
	AuthoredKeyword string
	Reference       string
	Chain           []string
	Cause           error
}

// Reference exclusion causes distinguish profile exclusions from malformed references.
var (
	ErrExternalReference = errors.New("external reference")
	ErrReferenceCycle    = errors.New("reference cycle")
)

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
	sources, _, err := parse(spec, nil)

	return sources, err
}

// ParseWithProfileValidation validates the normalized document profile before request
// acquisition, then validates every acquired raw Parameter Object.
func ParseWithProfileValidation(
	spec []byte,
	validateDocument DocumentValidator,
	validateParameter ParameterValidator,
) (map[string]Source, json.RawMessage, error) {
	if validateDocument == nil {
		return nil, nil, errors.New("document validator must not be nil")
	}

	if validateParameter == nil {
		return nil, nil, errors.New("parameter validator must not be nil")
	}

	return parseWithSemanticVersionPattern(spec, validateDocument, validateParameter, semanticVersionPattern)
}

// ParseWithParameterValidation validates every raw Parameter Object before merge or filtering.
func ParseWithParameterValidation(
	spec []byte,
	validateParameter ParameterValidator,
) (map[string]Source, json.RawMessage, error) {
	if validateParameter == nil {
		return nil, nil, errors.New("parameter validator must not be nil")
	}

	return parse(spec, validateParameter)
}

// parse ingests and acquires one OpenAPI document with optional raw Parameter Object validation.
func parse(spec []byte, validateParameter ParameterValidator) (map[string]Source, json.RawMessage, error) {
	return parseWithSemanticVersionPattern(spec, nil, validateParameter, semanticVersionPattern)
}

// parseWithSemanticVersionPattern parses using the supplied version grammar.
//
//nolint:cyclop // Document decoding, version admission, and request collection form one ordered parse.
func parseWithSemanticVersionPattern(
	spec []byte,
	validateDocument DocumentValidator,
	validateParameter ParameterValidator,
	versionPattern string,
) (map[string]Source, json.RawMessage, error) {
	document := spec
	if json.Valid(spec) {
		if err := rejectDuplicateJSONNames(spec); err != nil {
			return nil, nil, fmt.Errorf("parse OpenAPI document JSON: %w", err)
		}
	} else {
		var err error

		document, err = yaml.YAMLToJSON(spec)
		if err != nil {
			return nil, nil, fmt.Errorf("parse OpenAPI YAML: %w", err)
		}
	}

	var root map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(document, &root); unmarshalErr != nil {
		return nil, nil, fmt.Errorf("parse OpenAPI document JSON: %w", unmarshalErr)
	}

	if root == nil {
		return nil, nil, errors.New("OpenAPI document must be an object")
	}

	const versionPointer = "#/openapi"

	var version string
	if err := json.Unmarshal(root["openapi"], &version); err != nil {
		return nil, nil, fmt.Errorf(
			"%s: OpenAPI document version must be a Semantic Versioning 2.0.0 version: %w",
			versionPointer,
			err,
		)
	}

	compiledVersionPattern, err := regexp.Compile(versionPattern)
	if err != nil {
		return nil, nil, fmt.Errorf("compile Semantic Versioning pattern: %w", err)
	}

	versionParts := compiledVersionPattern.FindStringSubmatch(version)
	if len(versionParts) == 0 {
		return nil, nil, fmt.Errorf(
			"%s: OpenAPI document version must be a Semantic Versioning 2.0.0 version",
			versionPointer,
		)
	}

	if versionParts[1] != "3" || versionParts[2] != "0" {
		return nil, nil, fmt.Errorf(
			"%s: OpenAPI document feature set %s.%s is outside the Klopt 3.0 profile",
			versionPointer,
			versionParts[1],
			versionParts[2],
		)
	}

	if structuralErr := preflightDocumentStructure(root); structuralErr != nil {
		return nil, nil, structuralErr
	}

	normalized := append(json.RawMessage(nil), document...)
	if validateDocument != nil {
		if profileErr := validateDocument(normalized); profileErr != nil {
			return nil, nil, profileErr
		}
	}

	source := Source{Document: normalized}

	type pendingParameter struct {
		source Source
		schema LocatedSchema
	}

	pending := make([]pendingParameter, 0)

	queueParameter := validateParameter
	if validateParameter != nil {
		queueParameter = func(source Source, schema LocatedSchema) error {
			pending = append(pending, pendingParameter{source: source, schema: schema})

			return nil
		}
	}

	sources, err := source.collectRequests(root["paths"], queueParameter)
	if err != nil {
		return nil, nil, err
	}

	for _, parameter := range pending {
		if err := validateParameter(parameter.source, parameter.schema); err != nil {
			return nil, nil, err
		}
	}

	return sources, normalized, nil
}

// preflightDocumentStructure validates document structure without following references.
func preflightDocumentStructure(root map[string]json.RawMessage) error {
	pathsRaw, exists := root["paths"]
	if !exists {
		return errors.New("#/paths: required field is missing")
	}

	var paths map[string]json.RawMessage
	if err := json.Unmarshal(pathsRaw, &paths); err != nil || paths == nil {
		return errors.New("#/paths: must be an object")
	}

	seenPaths := make(map[string]string)

	for _, path := range slices.Sorted(maps.Keys(paths)) {
		if strings.HasPrefix(path, "x-") {
			continue
		}

		pointer := appendPointer("#/paths", path)

		identity, _, err := ParsePathTemplate(path)
		if err != nil {
			return fmt.Errorf("%s: %w", pointer, err)
		}

		if first, duplicate := seenPaths[identity]; duplicate {
			return fmt.Errorf("%s: templated path is identical to %s", pointer, first)
		}

		seenPaths[identity] = pointer
		if err := preflightPathItemStructure(paths[path], pointer); err != nil {
			return err
		}
	}

	return preflightComponentCallbacks(root["components"], "#/components")
}

// preflightPathItemStructure checks one authored Path Item without following its reference.
//
//nolint:cyclop // Fixed fields, operation shapes, and callback descent form one structural check.
func preflightPathItemStructure(raw json.RawMessage, pointer string) error {
	var pathItem map[string]json.RawMessage
	if err := json.Unmarshal(raw, &pathItem); err != nil || pathItem == nil {
		return fmt.Errorf("%s: must be an object", pointer)
	}

	if _, referenced := pathItem["$ref"]; referenced {
		for name := range pathItem {
			if name != "$ref" && !strings.HasPrefix(name, "x-") {
				return fmt.Errorf(
					"%s/$ref: Path Item Object fields beside $ref have undefined OAS 3.0 behavior",
					pointer,
				)
			}
		}
	}

	allowed := map[string]struct{}{
		"$ref": {}, "summary": {}, "description": {}, "get": {}, "put": {}, "post": {}, "delete": {},
		"options": {}, "head": {}, "patch": {}, "trace": {}, "servers": {}, "parameters": {},
	}
	for name := range pathItem {
		if _, ok := allowed[name]; ok || strings.HasPrefix(name, "x-") {
			continue
		}

		return fmt.Errorf("%s: unknown Path Item Object field", appendPointer(pointer, name))
	}

	for _, method := range []string{"delete", "get", "head", "options", "patch", "post", "put", "trace"} {
		operation, exists := pathItem[method]
		if !exists {
			continue
		}

		var object map[string]json.RawMessage
		if err := json.Unmarshal(operation, &object); err != nil || object == nil {
			return fmt.Errorf("%s/%s: must be an object", pointer, method)
		}

		if err := preflightCallbacksStructure(object["callbacks"], appendPointer(pointer, method, "callbacks")); err != nil {
			return err
		}
	}

	return nil
}

// preflightCallbacksStructure checks inline callback Path Items without following references.
//
//nolint:cyclop // Container, callback, reference, and expression checks are one structural pass.
func preflightCallbacksStructure(raw json.RawMessage, pointer string) error {
	if raw == nil {
		return nil
	}

	var callbacks map[string]json.RawMessage
	if err := json.Unmarshal(raw, &callbacks); err != nil || callbacks == nil {
		return fmt.Errorf("%s: must be an object", pointer)
	}

	for _, name := range slices.Sorted(maps.Keys(callbacks)) {
		var callback map[string]json.RawMessage
		if err := json.Unmarshal(callbacks[name], &callback); err != nil || callback == nil {
			return fmt.Errorf("%s: must be an object", appendPointer(pointer, name))
		}

		if _, referenced := callback["$ref"]; referenced {
			continue
		}

		callbackPointer := appendPointer(pointer, name)

		for _, expression := range slices.Sorted(maps.Keys(callback)) {
			if strings.HasPrefix(expression, "x-") {
				continue
			}

			if err := preflightPathItemStructure(
				callback[expression],
				appendPointer(callbackPointer, expression),
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// preflightComponentCallbacks checks the reusable inline Callback Objects.
func preflightComponentCallbacks(raw json.RawMessage, pointer string) error {
	if raw == nil {
		return nil
	}

	var components map[string]json.RawMessage
	if err := json.Unmarshal(raw, &components); err != nil || components == nil {
		return fmt.Errorf("%s: must be an object", pointer)
	}

	return preflightCallbacksStructure(components["callbacks"], appendPointer(pointer, "callbacks"))
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

// jsonTokenReader is the token-stream seam used for duplicate-name admission.
type jsonTokenReader interface {
	Token() (json.Token, error)
	More() bool
}

// rejectDuplicateJSONValue walks one complete value from the decoder.
//
//nolint:cyclop // JSON objects and arrays require separate recursive token handling.
func rejectDuplicateJSONValue(decoder jsonTokenReader, pointer string) error {
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
	}

	_, closingErr := decoder.Token()

	return closingErr
}

// Resolve follows a local Reference Object chain and ignores all Reference Object siblings.
func (source Source) Resolve(schema LocatedSchema) (LocatedSchema, error) {
	return source.resolve(schema, nil)
}

// ResolveAndInspect inspects each raw occurrence before following its Reference Object.
func (source Source) ResolveAndInspect(
	schema LocatedSchema,
	inspect func(LocatedSchema) error,
) (LocatedSchema, error) {
	if inspect == nil {
		return LocatedSchema{}, errors.New("reference inspector must not be nil")
	}

	return source.resolve(schema, inspect)
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

// resolve implements reference traversal with an optional pre-resolution inspector.
func (source Source) resolve(
	schema LocatedSchema,
	inspect func(LocatedSchema) error,
) (LocatedSchema, error) {
	current := LocatedSchema{Raw: append(json.RawMessage(nil), schema.Raw...), Pointer: schema.Pointer}
	seen := make(map[string]struct{})
	chain := make([]string, 0)

	for {
		if inspect != nil {
			if err := inspect(current); err != nil {
				return LocatedSchema{}, err
			}
		}

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
				ErrReferenceCycle,
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

		pathItemMembers, validateErr := decodePathItem(resolved)
		if validateErr != nil {
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

		for _, operation := range operationChildren(pathItemMembers, resolved.Pointer) {
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

// decodePathItem rejects unknown non-extension fixed fields.
func decodePathItem(pathItem LocatedSchema) (map[string]json.RawMessage, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(pathItem.Raw, &members); err != nil {
		return nil, err
	}

	if members == nil {
		return nil, errors.New("path item must be an object")
	}

	allowed := map[string]struct{}{
		"$ref": {}, "summary": {}, "description": {}, "get": {}, "put": {}, "post": {}, "delete": {},
		"options": {}, "head": {}, "patch": {}, "trace": {}, "servers": {}, "parameters": {},
	}
	for name := range members {
		if _, ok := allowed[name]; ok || strings.HasPrefix(name, "x-") {
			continue
		}

		return nil, fmt.Errorf("unknown Path Item field %q at %s", name, pathItem.Pointer)
	}

	return members, nil
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
func operationChildren(members map[string]json.RawMessage, pointer string) []operationChild {
	methods := []string{"delete", "get", "head", "options", "patch", "post", "put", "trace"}

	operations := make([]operationChild, 0, len(methods))
	for _, method := range methods {
		if raw, ok := members[method]; ok {
			operations = append(operations, operationChild{
				Schema: LocatedSchema{
					Raw:     raw,
					Pointer: appendPointer(pointer, method),
				},
				Method: method,
			})
		}
	}

	return operations
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
		return fmt.Errorf("%w %q is unsupported", ErrExternalReference, reference)
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
		Referrer:        referrer,
		AuthoredKeyword: appendPointer(referrer, "$ref"),
		Reference:       reference,
		Chain:           append([]string(nil), chain...),
		Cause:           cause,
	}
}
