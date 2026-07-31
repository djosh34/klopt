//nolint:godoclint // Package-private semantic tests document the expression model.
package testgenerator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestExpressionConstructorsCopyChildren(t *testing.T) {
	t.Parallel()

	first := newAtomExpression(atom{kind: atomKinds})
	second := newAtomExpression(atom{kind: atomEnum})
	children := []*expression{first, second}

	all := newAllExpression(children)
	anyExpression := newAnyExpression(children)

	children[0] = nil
	extended := append(children, newAtomExpression(atom{kind: atomNumberMinimum}))
	require.Len(t, extended, 3)

	require.Equal(t, []*expression{first, second}, all.children)
	require.Equal(t, []*expression{first, second}, anyExpression.children)
}

func TestLowererKeepsLocalRulesBesideAllOfAndAnyOf(t *testing.T) {
	t.Parallel()

	parsed, err := validation.Parse([]byte(`{
		"openapi":"3.0.4",
		"info":{"title":"test","version":"1"},
		"paths":{"/things":{"post":{
			"operationId":"request",
			"requestBody":{"content":{"application/json":{"schema":{
				"type":"string",
				"minLength":1,
				"allOf":[{"maxLength":3}],
				"anyOf":[{"enum":["a"]},{"enum":["b"]}]
			}}}},
			"responses":{"204":{"description":"ok"}}
		}}}
	}`), nil)
	require.NoError(t, err)

	lowerer := expressionLowerer{byValidation: make(map[*validation.Validation]*expression)}
	root, err := lowerer.lower(parsed["request"].Body)
	require.NoError(t, err)
	require.Equal(t, expressionAll, root.kind)
	require.Len(t, root.children, 4)
	require.Equal(t, []atomKind{
		atomKinds,
		atomStringMinLength,
		atomStringMaxLength,
		atomEnum,
	}, []atomKind{
		root.children[0].atom.kind,
		root.children[1].atom.kind,
		root.children[2].children[1].atom.kind,
		root.children[3].children[0].children[1].atom.kind,
	})
	require.Equal(t, expressionAny, root.children[3].kind)
	require.Len(t, root.children[3].children, 2)
	require.Equal(t, json.RawMessage(`"a"`), parsed["request"].Body.AnyOfValidations[0].EnumValidation.Values[0])
}

func TestLowererSharesOneValidationPointer(t *testing.T) {
	t.Parallel()

	parsed, err := validation.Parse([]byte(`{
		"openapi":"3.0.4",
		"info":{"title":"test","version":"1"},
		"paths":{"/things":{"post":{
			"operationId":"request",
			"requestBody":{"content":{"application/json":{"schema":{
				"allOf":[
					{"$ref":"#/components/schemas/shared"},
					{"$ref":"#/components/schemas/shared"}
				]
			}}}},
			"responses":{"204":{"description":"ok"}}
		}}},
		"components":{"schemas":{"shared":{"type":"string"}}}
	}`), nil)
	require.NoError(t, err)

	lowerer := expressionLowerer{byValidation: make(map[*validation.Validation]*expression)}
	root, err := lowerer.lower(parsed["request"].Body)
	require.NoError(t, err)
	require.Len(t, root.children, 3)
	require.Same(t, root.children[1], root.children[2])
}

func TestCompileSortsOperationIDsAndSkipsNilBodies(t *testing.T) {
	t.Parallel()

	generator, err := Compile([]byte(`{
		"openapi":"3.0.4",
		"info":{"title":"test","version":"1"},
		"paths":{
			"/z":{"post":{"operationId":"zulu","responses":{"204":{"description":"ok"}}}},
			"/a":{"post":{"operationId":"alpha","requestBody":{
				"content":{"application/json":{"schema":{"enum":[null]}}}
			},"responses":{"204":{"description":"ok"}}}},
			"/m":{"post":{"operationId":"middle","requestBody":{
				"content":{"application/json":{"schema":{"type":"boolean"}}}
			},"responses":{"204":{"description":"ok"}}}}
		}
	}`))
	require.NoError(t, err)
	require.Equal(t, []string{"alpha", "middle"}, []string{generator.operations[0].id, generator.operations[1].id})
	require.Equal(t, 0, generator.byID["alpha"])
	require.Equal(t, 1, generator.byID["middle"])
}

func TestCompileDoesNotExpandTwentyFourBinaryChoices(t *testing.T) {
	t.Parallel()

	choices := make([]string, 24)
	for index := range choices {
		choices[index] = `{"anyOf":[{"enum":[0]},{"enum":[1]}]}`
	}

	document := []byte(`{
		"openapi":"3.0.4",
		"info":{"title":"test","version":"1"},
		"paths":{"/things":{"post":{
			"operationId":"request",
			"requestBody":{"content":{"application/json":{"schema":{"allOf":[` + strings.Join(choices, ",") + `]}}}},
			"responses":{"204":{"description":"ok"}}
		}}}
	}`)

	generator, err := Compile(document)
	require.NoError(t, err)

	anyCount, childCount := countAnyExpressions(generator.operations[0].root)
	require.Equal(t, 24, anyCount)
	require.Equal(t, 48, childCount)
}

func countAnyExpressions(root *expression) (int, int) {
	if root == nil {
		return 0, 0
	}

	anyCount := 0
	childCount := 0

	if root.kind == expressionAny {
		anyCount++
		childCount += len(root.children)
	}

	for _, child := range root.children {
		childAnyCount, childCountInAny := countAnyExpressions(child)
		anyCount += childAnyCount
		childCount += childCountInAny
	}

	return anyCount, childCount
}
