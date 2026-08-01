//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package validation

import (
	"encoding/json"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/stretchr/testify/require"
)

func TestAuthoredSchemaWalkerMalformedSlotsAreSkipped(t *testing.T) {
	t.Parallel()

	walker := authoredSchemaWalker{
		source:  oas.Source{Document: json.RawMessage(`{}`)},
		visited: make(map[string]struct{}),
	}

	for _, walk := range []func() error{
		func() error { return walker.paths(json.RawMessage(`null`), "#/paths") },
		func() error { return walker.components(json.RawMessage(`null`), "#/components") },
		func() error { return walker.pathItem(json.RawMessage(`1`), "#/path") },
		func() error { return walker.operation(json.RawMessage(`1`), "#/operation") },
		func() error { return walker.parameters(json.RawMessage(`{}`), "#/parameters") },
		func() error { return walker.parameter(json.RawMessage(`1`), "#/parameter") },
		func() error { return walker.header(json.RawMessage(`1`), "#/header") },
		func() error { return walker.requestBody(json.RawMessage(`1`), "#/body") },
		func() error { return walker.responses(json.RawMessage(`[]`), "#/responses") },
		func() error { return walker.response(json.RawMessage(`1`), "#/response") },
		func() error { return walker.content(json.RawMessage(`[]`), "#/content") },
		func() error { return walker.mediaType(json.RawMessage(`1`), "#/media") },
		func() error { return walker.callback(json.RawMessage(`1`), "#/callback") },
		func() error { return walker.schema(json.RawMessage(`1`), "#/schema") },
	} {
		require.NoError(t, walk())
	}

	require.NoError(t, rejectAuthoredUniqueItems(json.RawMessage(`null`)))
	require.NoError(t, walker.paths(json.RawMessage(`{"x-skip":{"uniqueItems":false}}`), "#/paths"))
	require.NoError(t, walker.operation(json.RawMessage(`{}`), "#/operation-empty"))
	require.NoError(t, walker.operation(json.RawMessage(`{"callbacks":1}`), "#/operation-callbacks"))
	require.NoError(t, walker.operation(json.RawMessage(`{"callbacks":{"named":{"expression":{"get":{}}}}}`), "#/operation-valid-callback"))
	require.NoError(t, walker.responses(json.RawMessage(`{"x-skip":{"uniqueItems":false}}`), "#/responses"))
	require.NoError(t, walker.mediaType(json.RawMessage(`{"encoding":{"p":1}}`), "#/encoding"))
	require.NoError(t, walker.callback(json.RawMessage(`{"x-skip":{"uniqueItems":false}}`), "#/callback-extension"))
}

func TestAuthoredSchemaWalkerPropagatesEverySlot(t *testing.T) {
	t.Parallel()

	newWalker := func() *authoredSchemaWalker {
		return &authoredSchemaWalker{
			source:  oas.Source{Document: json.RawMessage(`{}`)},
			visited: make(map[string]struct{}),
		}
	}

	walks := []func(*authoredSchemaWalker) error{
		func(walker *authoredSchemaWalker) error {
			return walker.paths(json.RawMessage(`{"/x":{"get":{"parameters":[{"schema":{"uniqueItems":false}}]}}}`), "#/paths")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.components(json.RawMessage(`{"headers":{"H":{"schema":{"uniqueItems":false}}}}`), "#/components")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.pathItem(json.RawMessage(`{"parameters":[{"schema":{"uniqueItems":false}}]}`), "#/path")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.operation(json.RawMessage(`{"requestBody":{"content":{"application/json":{"schema":{"uniqueItems":false}}}}}`), "#/operation")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.parameters(json.RawMessage(`[{"schema":{"uniqueItems":false}}]`), "#/parameters")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.parameter(json.RawMessage(`{"schema":{"uniqueItems":false}}`), "#/parameter")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.header(json.RawMessage(`{"content":{"application/json":{"schema":{"uniqueItems":false}}}}`), "#/header")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.schemaOrContent(map[string]json.RawMessage{"schema": json.RawMessage(`{"uniqueItems":false}`)}, "#/owner")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.requestBody(json.RawMessage(`{"content":{"application/json":{"schema":{"uniqueItems":false}}}}`), "#/body")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.responses(json.RawMessage(`{"200":{"headers":{"H":{"schema":{"uniqueItems":false}}}}}`), "#/responses")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.response(json.RawMessage(`{"headers":{"H":{"schema":{"uniqueItems":false}}}}`), "#/response")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.content(json.RawMessage(`{"application/json":{"schema":{"uniqueItems":false}}}`), "#/content")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.mediaType(json.RawMessage(`{"encoding":{"p":{"headers":{"H":{"schema":{"uniqueItems":false}}}}}}`), "#/media")
		},
		func(walker *authoredSchemaWalker) error {
			return walker.callback(json.RawMessage(`{"expression":{"get":{"responses":{"200":{"content":{"application/json":{"schema":{"uniqueItems":false}}}}}}}}`), "#/callback")
		},
	}

	for _, walk := range walks {
		require.Error(t, walk(newWalker()))
	}
}

func TestAuthoredSchemaWalkerReferenceAndSeenBranches(t *testing.T) {
	t.Parallel()

	document := json.RawMessage(`{"schema":{"uniqueItems":false},"alias":{"$ref":"#/schema"}}`)
	walker := authoredSchemaWalker{
		source:  oas.Source{Document: document},
		visited: make(map[string]struct{}),
	}
	require.Error(t, walker.schema(json.RawMessage(`{"$ref":"#/alias"}`), "#/start"))
	require.NoError(t, walker.schema(json.RawMessage(`{}`), "#/seen"))
	require.NoError(t, walker.schema(json.RawMessage(`{"uniqueItems":false}`), "#/seen"))

	unavailable := authoredSchemaWalker{
		source:  oas.Source{Document: json.RawMessage(`{}`)},
		visited: make(map[string]struct{}),
	}
	require.NoError(t, unavailable.schema(json.RawMessage(`{"$ref":"#/missing"}`), "#/start"))

	for _, walk := range []func() error{
		func() error { return unavailable.pathItem(json.RawMessage(`{"$ref":"#/missing"}`), "#/path") },
		func() error { return unavailable.parameter(json.RawMessage(`{"$ref":"#/missing"}`), "#/parameter") },
		func() error { return unavailable.header(json.RawMessage(`{"$ref":"#/missing"}`), "#/header") },
		func() error { return unavailable.requestBody(json.RawMessage(`{"$ref":"#/missing"}`), "#/body") },
		func() error { return unavailable.response(json.RawMessage(`{"$ref":"#/missing"}`), "#/response") },
		func() error { return unavailable.callback(json.RawMessage(`{"$ref":"#/missing"}`), "#/callback") },
	} {
		require.NoError(t, walk())
	}

	_, _, ok := unavailable.resolve("owner", json.RawMessage(`{}`), "#/owner")
	require.True(t, ok)
	_, _, ok = unavailable.resolve("owner", json.RawMessage(`{}`), "#/owner")
	require.False(t, ok)
}
