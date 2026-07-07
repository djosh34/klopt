package jsonrefs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalNodeBuildsNodeTypesAndDropsRefSiblings(t *testing.T) {
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

func TestMarshalNodeIsReverseOfUnmarshalNode(t *testing.T) {
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

func TestObjectNodeGetPathPart(t *testing.T) {
	child := &LeafNode{noPath{}, json.RawMessage(`true`)}
	node := ObjectNode{Map: map[string]Node{"child": child}}

	got, err := node.GetPathPart("child")
	require.NoError(t, err)
	require.Same(t, child, got)

	_, err = node.GetPathPart("missing")
	require.Error(t, err)
	require.ErrorContains(t, err, `path part "missing" not found`)
}

func TestArrayLeafAndRefNodesGetPathPartErrors(t *testing.T) {
	tests := map[string]Node{
		"array": &ArrayNode{},
		"leaf":  &LeafNode{noPath{}, json.RawMessage(`true`)},
		"ref":   &RefNode{Ref: "#/anything"},
	}

	for name, node := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := node.GetPathPart("part")
			require.Error(t, err)
		})
	}
}

func TestReplaceResolvesRefsEverywhere(t *testing.T) {
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

func TestReplaceResolvesRefsInArrays(t *testing.T) {
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

func TestReplaceResolvesRefChains(t *testing.T) {
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

func TestReplaceDropsRefSiblings(t *testing.T) {
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

func TestReplaceUnescapesRefPathParts(t *testing.T) {
	input := json.RawMessage(`{
		"paths": {
			"/blogs/{blog_id}/new~posts": {"path": true},
			"a b": {"space": true},
			"": {"empty": true},
			"~1": {"tildeOne": true}
		},
		"pathRef": {"$ref": "#/paths/~1blogs~1{blog_id}~1new~0posts"},
		"spaceRef": {"$ref": "#/paths/a%20b"},
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
		"emptyRef": {"empty": true},
		"tildeOneRef": {"tildeOne": true}
	}`, string(*got))
}

func TestReplaceCanResolveToLeafNodes(t *testing.T) {
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

func TestReplaceLeavesJSONWithoutRefsAlone(t *testing.T) {
	input := json.RawMessage(`{
		"object": {"type": "object"},
		"array": [1, true, null],
		"string": "value"
	}`)

	got, err := Replace(&input)
	require.NoError(t, err)
	require.JSONEq(t, string(input), string(*got))
}

func TestReplaceErrors(t *testing.T) {
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
			wantError: "must start with #/",
		},
		"fragment without slash": {
			input:     `{"schema":{"$ref":"#"}}`,
			wantError: "must start with #/",
		},
		"remote ref": {
			input:     `{"schema":{"$ref":"document.json#/defs/User"}}`,
			wantError: "must start with #/",
		},
		"url ref": {
			input:     `{"schema":{"$ref":"http://example.com/document.json#/defs/User"}}`,
			wantError: "must start with #/",
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
		"path tries to traverse array": {
			input:     `{"defs":[{"type":"string"}],"schema":{"$ref":"#/defs/0"}}`,
			wantError: "non-object",
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
			input := json.RawMessage(tt.input)
			_, err := Replace(&input)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

func TestReplaceErrorsForNilRawMessage(t *testing.T) {
	_, err := Replace(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "json raw message is nil")
}

func TestReplaceErrorsForReferenceCycles(t *testing.T) {
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
			input := json.RawMessage(inputString)
			_, err := Replace(&input)
			require.Error(t, err)
			require.ErrorContains(t, err, "reference cycle")
		})
	}
}
