package jsonrefs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUnmarshalNodeBuildsNodeTypesAndDropsRefSiblings verifies node decoding.
func TestUnmarshalNodeBuildsNodeTypesAndDropsRefSiblings(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"object": {
			"number": 1,
			"string": "value",
			"bool": true,
			"null": null
		},
		"array": [1, {"nested": false}],
		"ref": {"$ref": "#/object", "description": "ignored"}
	}`)

	node, err := unmarshalNode(input)
	require.NoError(t, err)

	root, ok := node.(*ObjectNode)
	require.True(t, ok)
	require.Len(t, root.Map, 3)

	object, ok := root.Map["object"].(*ObjectNode)
	require.True(t, ok)
	require.Len(t, object.Map, 4)

	number, ok := object.Map["number"].(*LeafNode)
	require.True(t, ok)
	require.JSONEq(t, `1`, string(number.RawMessage))

	stringNode, ok := object.Map["string"].(*LeafNode)
	require.True(t, ok)
	require.JSONEq(t, `"value"`, string(stringNode.RawMessage))

	boolNode, ok := object.Map["bool"].(*LeafNode)
	require.True(t, ok)
	require.JSONEq(t, `true`, string(boolNode.RawMessage))

	nullNode, ok := object.Map["null"].(*LeafNode)
	require.True(t, ok)
	require.JSONEq(t, `null`, string(nullNode.RawMessage))

	array, ok := root.Map["array"].(*ArrayNode)
	require.True(t, ok)
	require.Len(t, array.Items, 2)
	_, ok = array.Items[0].(*LeafNode)
	require.True(t, ok)
	_, ok = array.Items[1].(*ObjectNode)
	require.True(t, ok)

	ref, ok := root.Map["ref"].(*RefNode)
	require.True(t, ok)
	require.Equal(t, "#/object", ref.Ref)

	refBytes, err := json.Marshal(ref)
	require.NoError(t, err)
	require.JSONEq(t, `{"$ref":"#/object"}`, string(refBytes))
}

// TestMarshalNodeIsReverseOfUnmarshalNode verifies node encoding.
func TestMarshalNodeIsReverseOfUnmarshalNode(t *testing.T) {
	t.Parallel()

	node := &ObjectNode{Map: map[string]Node{
		"object": &ObjectNode{Map: map[string]Node{
			"bool": &LeafNode{noPath{}, json.RawMessage(`true`)},
			"null": &LeafNode{noPath{}, json.RawMessage(`null`)},
		}},
		"array": &ArrayNode{Items: []Node{
			&LeafNode{noPath{}, json.RawMessage(`1`)},
			&ObjectNode{Map: map[string]Node{"string": &LeafNode{noPath{}, json.RawMessage(`"value"`)}}},
		}},
	}}

	bytes, err := json.Marshal(node)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"object": {"bool": true, "null": null},
		"array": [1, {"string": "value"}]
	}`, string(bytes))
}

// TestObjectNodeGetPathPart verifies object member traversal.
func TestObjectNodeGetPathPart(t *testing.T) {
	t.Parallel()

	child := &LeafNode{noPath{}, json.RawMessage(`true`)}
	node := ObjectNode{Map: map[string]Node{"child": child}}

	got, err := node.GetPathPart("child")
	require.NoError(t, err)
	require.Same(t, child, got)

	_, err = node.GetPathPart("missing")
	require.Error(t, err)
	require.ErrorContains(t, err, `path part "missing" not found`)
}

// TestArrayNodeGetPathPart verifies array index traversal and validation.
func TestArrayNodeGetPathPart(t *testing.T) {
	t.Parallel()

	first := &LeafNode{noPath{}, json.RawMessage(`true`)}
	second := &LeafNode{noPath{}, json.RawMessage(`false`)}
	node := &ArrayNode{Items: []Node{first, second}}

	got, err := node.GetPathPart("0")
	require.NoError(t, err)
	require.Same(t, first, got)

	got, err = node.GetPathPart("1")
	require.NoError(t, err)
	require.Same(t, second, got)

	tests := map[string]string{
		"empty":         "",
		"leading zero":  "01",
		"negative":      "-1",
		"not a number":  "one",
		"out of bounds": "2",
		"overflow":      "18446744073709551616",
	}

	for name, index := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := node.GetPathPart(index)
			require.Error(t, err)
		})
	}
}

// TestLeafAndRefNodesGetPathPartErrors verifies scalar traversal errors.
func TestLeafAndRefNodesGetPathPartErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]Node{
		"leaf": &LeafNode{noPath{}, json.RawMessage(`true`)},
		"ref":  &RefNode{Ref: "#/anything"},
	}

	for name, node := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := node.GetPathPart("part")
			require.Error(t, err)
		})
	}
}

// TestReplaceResolvesRefsEverywhere verifies nested object resolution.
func TestReplaceResolvesRefsEverywhere(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"components": {
			"schemas": {
				"ID": {"type": "integer"},
				"User": {
					"type": "object",
					"properties": {
						"id": {"$ref": "#/components/schemas/ID"}
					}
				}
			}
		},
		"responses": {
			"200": {
				"schema": {"$ref": "#/components/schemas/User"}
			}
		}
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"components": {
			"schemas": {
				"ID": {"type": "integer"},
				"User": {
					"type": "object",
					"properties": {
						"id": {"type": "integer"}
					}
				}
			}
		},
		"responses": {
			"200": {
				"schema": {
					"type": "object",
					"properties": {
						"id": {"type": "integer"}
					}
				}
			}
		}
	}`, string(*got))
}

// TestReplaceResolvesRefsInArrays verifies references stored in arrays.
func TestReplaceResolvesRefsInArrays(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"defs": {
			"string": {"type": "string"},
			"integer": {"type": "integer"}
		},
		"allOf": [
			{"$ref": "#/defs/string"},
			{"$ref": "#/defs/integer"}
		]
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"defs": {
			"string": {"type": "string"},
			"integer": {"type": "integer"}
		},
		"allOf": [
			{"type": "string"},
			{"type": "integer"}
		]
	}`, string(*got))
}

// TestReplaceResolvesRefsThroughArrayIndexes verifies array JSON Pointer traversal.
func TestReplaceResolvesRefsThroughArrayIndexes(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"defs": [
			{"type": "string"},
			{"type": "integer"}
		],
		"schema": {"$ref": "#/defs/1"}
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"defs": [
			{"type": "string"},
			{"type": "integer"}
		],
		"schema": {"type": "integer"}
	}`, string(*got))
}

// TestReplaceResolvesRefChains verifies chained reference resolution.
func TestReplaceResolvesRefChains(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"defs": {
			"Actual": {"type": "string"},
			"AliasB": {"$ref": "#/defs/Actual"},
			"AliasA": {"$ref": "#/defs/AliasB"}
		},
		"schema": {"$ref": "#/defs/AliasA"}
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"defs": {
			"Actual": {"type": "string"},
			"AliasB": {"type": "string"},
			"AliasA": {"type": "string"}
		},
		"schema": {"type": "string"}
	}`, string(*got))
}

// TestReplaceDropsRefSiblings verifies Reference Object sibling handling.
func TestReplaceDropsRefSiblings(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"defs": {
			"Date": {"type": "string", "format": "date"},
			"DateAlias": {
				"$ref": "#/defs/Date",
				"description": "ignored",
				"default": "2000-01-01"
			}
		},
		"schema": {
			"$ref": "#/defs/DateAlias",
			"description": "also ignored",
			"default": "1999-01-01"
		}
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"defs": {
			"Date": {"type": "string", "format": "date"},
			"DateAlias": {"type": "string", "format": "date"}
		},
		"schema": {"type": "string", "format": "date"}
	}`, string(*got))
}

// TestReplaceUnescapesRefPathParts verifies URI and JSON Pointer unescaping.
func TestReplaceUnescapesRefPathParts(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"paths": {
			"/blogs/{blog_id}/new~posts": {"path": true},
			"a b": {"space": true},
			"": {"empty": true},
			"~1": {"tildeOne": true}
		},
		"pathRef": {"$ref": "#/paths/~1blogs~1{blog_id}~1new~0posts"},
		"spaceRef": {"$ref": "#/paths/a%20b"},
		"encodedSlashRef": {"$ref": "#%2Fpaths%2Fa%20b"},
		"emptyRef": {"$ref": "#/paths/"},
		"tildeOneRef": {"$ref": "#/paths/~01"}
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"paths": {
			"/blogs/{blog_id}/new~posts": {"path": true},
			"a b": {"space": true},
			"": {"empty": true},
			"~1": {"tildeOne": true}
		},
		"pathRef": {"path": true},
		"spaceRef": {"space": true},
		"encodedSlashRef": {"space": true},
		"emptyRef": {"empty": true},
		"tildeOneRef": {"tildeOne": true}
	}`, string(*got))
}

// TestReplaceCanResolveToLeafNodes verifies scalar reference targets.
func TestReplaceCanResolveToLeafNodes(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"defs": {
			"number": 123,
			"string": "abc",
			"bool": false,
			"null": null
		},
		"number": {"$ref": "#/defs/number"},
		"string": {"$ref": "#/defs/string"},
		"bool": {"$ref": "#/defs/bool"},
		"null": {"$ref": "#/defs/null"}
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"defs": {
			"number": 123,
			"string": "abc",
			"bool": false,
			"null": null
		},
		"number": 123,
		"string": "abc",
		"bool": false,
		"null": null
	}`, string(*got))
}

// TestReplaceLeavesJSONWithoutRefsAlone verifies reference-free documents.
func TestReplaceLeavesJSONWithoutRefsAlone(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"object": {"type": "object"},
		"array": [1, true, null],
		"string": "value"
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, string(input), string(*got))
}

// TestReplaceErrors verifies invalid reference documents are rejected.
func TestReplaceErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input     string
		wantError string
	}{
		"invalid json": {
			input:     `{`,
			wantError: "unmarshal json as node",
		},
		"non string ref": {
			input:     `{"schema":{"$ref":123}}`,
			wantError: "unmarshal $ref string",
		},
		"empty ref": {
			input:     `{"schema":{"$ref":""}}`,
			wantError: "in-document JSON Pointer",
		},
		"remote ref": {
			input:     `{"schema":{"$ref":"document.json#/defs/User"}}`,
			wantError: "in-document JSON Pointer",
		},
		"url ref": {
			input:     `{"schema":{"$ref":"http://example.com/document.json#/defs/User"}}`,
			wantError: "in-document JSON Pointer",
		},
		"invalid uri escape": {
			input:     `{"schema":{"$ref":"#/%zz"}}`,
			wantError: "parse $ref",
		},
		"missing object key": {
			input:     `{"defs":{},"schema":{"$ref":"#/defs/User"}}`,
			wantError: `path part "User" not found`,
		},
		"path tries to traverse leaf": {
			input:     `{"defs":{"User":true},"schema":{"$ref":"#/defs/User/type"}}`,
			wantError: "non-object",
		},
		"invalid array index": {
			input:     `{"defs":[{"type":"string"}],"schema":{"$ref":"#/defs/-"}}`,
			wantError: "not a non-negative integer",
		},
		"array index out of bounds": {
			input:     `{"defs":[{"type":"string"}],"schema":{"$ref":"#/defs/1"}}`,
			wantError: "out of bounds",
		},
		"array index with leading zero": {
			input:     `{"defs":[{"type":"string"}],"schema":{"$ref":"#/defs/00"}}`,
			wantError: "leading zero",
		},
		"bad pointer escape at end": {
			input:     `{"schema":{"$ref":"#/bad~"}}`,
			wantError: "~ must be followed by 0 or 1",
		},
		"bad pointer escape character": {
			input:     `{"schema":{"$ref":"#/bad~2"}}`,
			wantError: "~2 is invalid",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input := json.RawMessage(tt.input)
			_, err := Replace(&input)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

// TestReplaceReportsTheFirstObjectErrorByKey verifies deterministic reference failures.
func TestReplaceReportsTheFirstObjectErrorByKey(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"z": {"$ref": "#/missing-z"},
		"a": {"$ref": "#/missing-a"}
	}`)

	resolved, err := Replace(&input)
	require.Error(t, err)
	require.ErrorContains(t, err, `resolve object key "a"`)
	require.Nil(t, resolved)
}

// TestReplaceErrorsForNilRawMessage verifies nil input handling.
func TestReplaceErrorsForNilRawMessage(t *testing.T) {
	t.Parallel()

	_, err := Replace(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "json raw message is nil")
}

// TestReplaceErrorsForReferenceCycles verifies cycle detection.
func TestReplaceErrorsForReferenceCycles(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"ref chain cycle": `{
			"defs": {
				"A": {"$ref": "#/defs/B"},
				"B": {"$ref": "#/defs/A"}
			},
			"schema": {"$ref": "#/defs/A"}
		}`,
		"object cycle": `{
			"defs": {
				"Node": {
					"type": "object",
					"properties": {
						"child": {"$ref": "#/defs/Node"}
					}
				}
			}
		}`,
	}

	for name, inputString := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input := json.RawMessage(inputString)
			_, err := Replace(&input)
			require.Error(t, err)
			require.ErrorContains(t, err, "reference cycle")
		})
	}
}

// TestResolveReferenceFollowsOnlyTheSelectedReferenceChain verifies lazy resolution.
func TestResolveReferenceFollowsOnlyTheSelectedReferenceChain(t *testing.T) {
	t.Parallel()

	root := json.RawMessage(`{
		"defs": {
			"Schema": {"type": "object"},
			"Body": {
				"content": {
					"application/json": {
						"schema": {"$ref": "#/defs/Schema"}
					}
				}
			},
			"Alias": {"$ref": "#/defs/Body"}
		}
	}`)
	reference := json.RawMessage(`{"$ref":"#/defs/Alias","description":"ignored"}`)

	got, err := ResolveReference(&root, &reference)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"content": {
			"application/json": {
				"schema": {"$ref": "#/defs/Schema"}
			}
		}
	}`, string(*got))
}

// TestResolveReferenceSupportsTheEmptyJSONPointer verifies whole-document references.
func TestResolveReferenceSupportsTheEmptyJSONPointer(t *testing.T) {
	t.Parallel()

	root := json.RawMessage(`{"openapi":"3.0.3","paths":{}}`)
	reference := json.RawMessage(`{"$ref":"#"}`)

	got, err := ResolveReference(&root, &reference)
	require.NoError(t, err)
	require.JSONEq(t, string(root), string(*got))
}

// TestResolveReferencePreservesNestedRefLikeValues verifies free-form JSON is not treated as a reference.
func TestResolveReferencePreservesNestedRefLikeValues(t *testing.T) {
	t.Parallel()

	root := json.RawMessage(`{
		"defs": {
			"Body": {
				"schema": {
					"type": "object",
					"example": {"$ref": "literal value", "sibling": true}
				}
			}
		},
		"x-unrelated": {"$ref": 123, "sibling": true}
	}`)
	reference := json.RawMessage(`{"$ref":"#/defs/Body"}`)

	got, err := ResolveReference(&root, &reference)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"schema": {
			"type": "object",
			"example": {"$ref": "literal value", "sibling": true}
		}
	}`, string(*got))
}

// TestResolveReferenceReturnsInlineValuesUnchanged verifies the inline fast path.
func TestResolveReferenceReturnsInlineValuesUnchanged(t *testing.T) {
	t.Parallel()

	root := json.RawMessage(`{"unused":true}`)
	inline := json.RawMessage("  {\"value\": true, \"example\": {\"$ref\": 123}}  ")

	got, err := ResolveReference(&root, &inline)
	require.NoError(t, err)
	require.Equal(t, inline, *got)

	(*got)[2] = '['

	require.Equal(t, byte('{'), inline[2])
}

// TestResolveReferenceErrors verifies selected-reference failures.
func TestResolveReferenceErrors(t *testing.T) {
	t.Parallel()

	validRoot := json.RawMessage(`{"defs":{}}`)
	validReference := json.RawMessage(`{"$ref":"#/defs/Missing"}`)
	cycleRoot := json.RawMessage(`{"defs":{"A":{"$ref":"#/defs/B"},"B":{"$ref":"#/defs/A"}}}`)
	cycleReference := json.RawMessage(`{"$ref":"#/defs/A"}`)
	invalidRoot := json.RawMessage(`{`)
	invalidReference := json.RawMessage(`{"$ref":123}`)

	tests := map[string]struct {
		root      *json.RawMessage
		reference *json.RawMessage
		wantError string
	}{
		"nil root":          {root: nil, reference: &validReference, wantError: "json root message is nil"},
		"nil reference":     {root: &validRoot, reference: nil, wantError: "json reference message is nil"},
		"invalid reference": {root: &validRoot, reference: &invalidReference, wantError: "unmarshal $ref string"},
		"invalid root":      {root: &invalidRoot, reference: &validReference, wantError: "unmarshal json root"},
		"missing target":    {root: &validRoot, reference: &validReference, wantError: "not found"},
		"reference cycle":   {root: &cycleRoot, reference: &cycleReference, wantError: "reference cycle"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := ResolveReference(tt.root, tt.reference)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}
