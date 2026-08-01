//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package oas

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDocumentAdmissionFailures(t *testing.T) {
	t.Parallel()

	for _, spec := range [][]byte{
		[]byte("openapi: ["),
		[]byte(`null`),
		[]byte(`[]`),
		[]byte(`{"openapi":3,"paths":{}}`),
		[]byte(`{"openapi":"invalid","paths":{}}`),
		[]byte(`{"openapi":"2.0.0","paths":{}}`),
		[]byte(`{"openapi":"3.0.3","paths":1}`),
		[]byte(`{"openapi":"3.0.3","paths":null}`),
	} {
		sources, err := Parse(spec)
		require.Error(t, err, string(spec))
		require.Nil(t, sources)
	}

	_, _, err := ParseWithParameterValidation([]byte(`{"openapi":"3.0.3","paths":{}}`), nil)
	require.Error(t, err)

	calls := 0
	sources, document, err := ParseWithParameterValidation([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: items
      parameters: [{name: q, in: query, schema: {}}]
`), func(_ Source, _ LocatedSchema) error {
		calls++

		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, document)
	require.Contains(t, sources, "items")
	require.Equal(t, 1, calls)

	_, _, err = ParseWithParameterValidation([]byte(`openapi: 3.0.3
paths:
  /items:
    get:
      operationId: items
      parameters: [{name: q, in: query, schema: {}}]
`), func(_ Source, _ LocatedSchema) error { return errors.New("rejected") })
	require.Error(t, err)
}

func TestParameterShapeBranches(t *testing.T) {
	t.Parallel()

	base := map[string]json.RawMessage{
		"name":   json.RawMessage(`"q"`),
		"in":     json.RawMessage(`"query"`),
		"schema": json.RawMessage(`{}`),
	}
	clone := func() map[string]json.RawMessage {
		cloned := make(map[string]json.RawMessage, len(base))
		for key, value := range base {
			cloned[key] = value
		}

		return cloned
	}

	for _, members := range []map[string]json.RawMessage{
		{"name": json.RawMessage(`"q"`), "in": json.RawMessage(`"query"`)},
		{"name": json.RawMessage(`"q"`), "in": json.RawMessage(`"query"`), "schema": json.RawMessage(`{}`), "content": json.RawMessage(`{}`)},
	} {
		raw, err := json.Marshal(members)
		require.NoError(t, err)
		_, err = parameterObjectIdentity(LocatedSchema{Raw: raw, Pointer: "#/parameter"})
		require.Error(t, err)
	}

	path := clone()
	path["in"] = json.RawMessage(`"path"`)
	path["required"] = json.RawMessage(`"bad"`)
	raw, err := json.Marshal(path)
	require.NoError(t, err)
	_, err = parameterObjectIdentity(LocatedSchema{Raw: raw, Pointer: "#/parameter"})
	require.Error(t, err)

	for index, change := range []func(map[string]json.RawMessage){
		func(fields map[string]json.RawMessage) { fields["description"] = json.RawMessage(`null`) },
		func(fields map[string]json.RawMessage) { fields["required"] = json.RawMessage(`"bad"`) },
		func(fields map[string]json.RawMessage) { fields["schema"] = json.RawMessage(`null`) },
		func(fields map[string]json.RawMessage) {
			delete(fields, "schema")
			fields["content"] = json.RawMessage(`{}`)
		},
		func(fields map[string]json.RawMessage) {
			delete(fields, "schema")
			fields["content"] = json.RawMessage(`null`)
		},
		func(fields map[string]json.RawMessage) {
			delete(fields, "schema")
			fields["content"] = json.RawMessage(`{"a/b":{},"c/d":{}}`)
		},
		func(fields map[string]json.RawMessage) {
			delete(fields, "schema")
			fields["content"] = json.RawMessage(`{"bad":{}}`)
		},
		func(fields map[string]json.RawMessage) {
			delete(fields, "schema")
			fields["content"] = json.RawMessage(`{"application/json":null}`)
		},
		func(fields map[string]json.RawMessage) {
			delete(fields, "schema")
			fields["content"] = json.RawMessage(`{"application/json":{"schema":null}}`)
		},
		func(fields map[string]json.RawMessage) {
			delete(fields, "schema")
			fields["content"] = json.RawMessage(`{"application/json":{}}`)
		},
		func(fields map[string]json.RawMessage) {
			delete(fields, "schema")
			fields["content"] = json.RawMessage(`{"application/json":{}}`)
			fields["style"] = json.RawMessage(`"form"`)
		},
	} {
		members := clone()
		change(members)

		err = validateParameterFields(
			members,
			parameterIdentity{name: "q", location: "query"},
			"#/parameter",
		)
		if index == 9 {
			require.NoError(t, err)
		} else {
			require.Error(t, err, index)
		}
	}
}

func TestReferenceAndPointerFailures(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	plain := &ReferenceError{Referrer: "#", Reference: "#/x", Cause: cause}
	require.Contains(t, plain.Error(), "#/x")
	require.ErrorIs(t, plain, cause)

	source := Source{Document: json.RawMessage(`{"target":{"type":"string"},"alias":{"$ref":"#/target"}}`)}
	_, err := source.ResolveAndInspect(LocatedSchema{}, nil)
	require.Error(t, err)

	seen := 0
	resolved, err := source.ResolveAndInspect(
		LocatedSchema{Raw: json.RawMessage(`{"$ref":"#/alias"}`), Pointer: "#/start"},
		func(_ LocatedSchema) error {
			seen++

			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, "#/target", resolved.Pointer)
	require.Equal(t, 3, seen)

	_, err = source.ResolveAndInspect(
		LocatedSchema{Raw: json.RawMessage(`{}`), Pointer: "#/start"},
		func(_ LocatedSchema) error { return cause },
	)
	require.ErrorIs(t, err, cause)
	_, err = source.Resolve(LocatedSchema{Raw: json.RawMessage(`{"$ref":1}`), Pointer: "#/start"})
	require.Error(t, err)
	_, err = source.Child(LocatedSchema{Raw: json.RawMessage(`{}`), Pointer: "#/start"}, "missing")
	require.Error(t, err)

	for _, reference := range []string{"%", "other.yaml#/x", "#fragment", "#/bad~", "#/bad~2"} {
		_, err = pointerTokens(reference)
		require.Error(t, err, reference)
	}

	tokens, err := pointerTokens("#")
	require.NoError(t, err)
	require.Empty(t, tokens)

	parsed, err := url.Parse("#/ok")
	require.NoError(t, err)
	require.NoError(t, validateLocalReference("#/ok", parsed))
}

func TestRawTraversalFailures(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", `{"a":`, `{"a":1,true:2}`, `[1`} {
		decoder := json.NewDecoder(strings.NewReader(input))
		err := rejectDuplicateJSONValue(decoder, "#")
		require.Error(t, err, input)
	}

	err := rejectDuplicateJSONValue(&tokenSequence{tokens: []json.Token{json.Delim('{'), 1}}, "#")
	require.Error(t, err)

	for _, raw := range []json.RawMessage{nil, json.RawMessage(`{`), json.RawMessage(`{"$ref":1}`)} {
		_, _, err = referenceFrom(raw)
		require.Error(t, err)
	}

	_, isReference, err := referenceFrom(json.RawMessage(`1`))
	require.NoError(t, err)
	require.False(t, isReference)
	_, isReference, err = referenceFrom(json.RawMessage(`{}`))
	require.NoError(t, err)
	require.False(t, isReference)

	for _, test := range []struct {
		parent json.RawMessage
		token  string
	}{
		{parent: nil, token: "x"},
		{parent: json.RawMessage(`{`), token: "x"},
		{parent: json.RawMessage(`{}`), token: "x"},
		{parent: json.RawMessage(`[`), token: "0"},
		{parent: json.RawMessage(`[]`), token: "0"},
		{parent: json.RawMessage(`[1]`), token: ""},
		{parent: json.RawMessage(`[1]`), token: "01"},
		{parent: json.RawMessage(`[1]`), token: "x"},
		{parent: json.RawMessage(`1`), token: "x"},
	} {
		_, err := childRaw(test.parent, test.token)
		require.Error(t, err)
	}
}

func TestPathAndOperationHelpers(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/x}", "/x/{", "/x/{}", "/x/{a{b}"} {
		_, _, err := ParsePathTemplate(path)
		require.Error(t, err)
	}

	for _, raw := range []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`null`)} {
		_, err := decodePathItem(LocatedSchema{Raw: raw, Pointer: "#/path"})
		require.Error(t, err)
	}

	parameters := []locatedParameter{{
		schema:   LocatedSchema{Pointer: "#/parameter"},
		identity: parameterIdentity{name: "extra", location: "path"},
	}}
	err := validatePathParameterCorrespondence("/items", nil, parameters)
	require.Error(t, err)

	parameters[0].identity.name = "a/b"
	err = validatePathParameterCorrespondence("/{a/b}", []string{"a/b"}, parameters)
	require.Error(t, err)

	err = (Source{}).validateParameters(parameters, func(_ Source, _ LocatedSchema) error {
		return errors.New("invalid")
	})
	require.Error(t, err)
	require.NoError(t, (Source{}).validateParameters(parameters, nil))

	_, _, err = ParseWithParameterValidation([]byte(`openapi: 3.0.3
paths:
  /{id}:
    parameters: [{name: id, in: path, required: true, schema: {}}]
`), func(_ Source, _ LocatedSchema) error { return errors.New("invalid") })
	require.Error(t, err)
}

type tokenSequence struct {
	tokens []json.Token
	index  int
}

func (sequence *tokenSequence) Token() (json.Token, error) {
	if sequence.index >= len(sequence.tokens) {
		return nil, errors.New("end")
	}

	token := sequence.tokens[sequence.index]
	sequence.index++

	return token, nil
}

func (sequence *tokenSequence) More() bool {
	return sequence.index < len(sequence.tokens)
}
