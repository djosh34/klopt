//nolint:godoclint // Focused private record-sequence tests use behavior names.
package schematest

import (
	"fmt"
	"iter"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluationRecordsPreserveCanonicalFactOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		value  string
		want   []string
	}{
		{
			name:   "primitive",
			schema: `{"type":"string","minLength":1}`,
			value:  `"x"`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|string",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|minLength",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|minLength|valid",
			},
		},
		{
			name:   "array",
			schema: `{"type":"array","minItems":1,"items":{"type":"number"}}`,
			value:  `[1]`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|array",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|minItems",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|minItems|valid",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/items|#/0|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/items|#/0|type|number",
			},
		},
		{
			name:   "object",
			schema: `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`,
			value:  `{"name":"x"}`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|object",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#/name|required",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#/name|required|present",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/properties/name|#/name|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/properties/name|#/name|type|string",
			},
		},
		{
			name:   "allOf",
			schema: `{"allOf":[{"type":"string"},{"minLength":1}]}`,
			value:  `"x"`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|string",
				"allOf|#/paths/~1items/post/requestBody/content/application~1json/schema|#|allOf|true,true",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/0|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/0|#|type|string",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/1|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/1|#|type|string",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/1|#|minLength",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema/allOf/1|#|minLength|valid",
			},
		},
		{
			name:   "anyOf",
			schema: `{"anyOf":[{"type":"string"},{"type":"number"}]}`,
			value:  `true`,
			want: []string{
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type",
				"observed|#/paths/~1items/post/requestBody/content/application~1json/schema|#|type|boolean",
				"anyOf|#/paths/~1items/post/requestBody/content/application~1json/schema|#|anyOf|false,false",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/anyOf/0|#|type",
				"failure|#/paths/~1items/post/requestBody/content/application~1json/schema/anyOf/0|#|type",
				"applicable|#/paths/~1items/post/requestBody/content/application~1json/schema/anyOf/1|#|type",
				"failure|#/paths/~1items/post/requestBody/content/application~1json/schema/anyOf/1|#|type",
				"failure|#/paths/~1items/post/requestBody/content/application~1json/schema|#|anyOf",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := evaluateSchemaValue(t, test.schema, test.value)
			require.NoError(t, result.err)

			for index := range test.want {
				test.want[index] = strings.Replace(test.want[index], "#/paths/~1items/", "#/paths/~1/", 1)
			}

			require.Equal(t, test.want, evaluationRecordStrings(result.records))
		})
	}
}

func TestEvaluationRecordsCacheHitKeepsEveryFact(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{"Shared":{"type":"string"}}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"allOf":[
				{"$ref":"#/components/schemas/Shared"},
				{"$ref":"#/components/schemas/Shared"}
			]
		}}}}}}}
	}`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	value, err := parseStrictJSON([]byte(`"x"`))
	require.NoError(t, err)

	context := evaluationContext{cache: make(map[evaluationCacheKey]evaluationCacheEntry)}
	firstNode := model.root.allOf[0]
	secondNode := model.root.allOf[1]
	first := context.evaluateNode(firstNode, value, firstNode.occurrence)
	second := context.evaluateNode(secondNode, value, secondNode.occurrence)

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, []string{"type"}, applicableRules(first.applicableRecords()))
	require.Equal(t, []string{"type"}, applicableRules(second.applicableRecords()))
	require.Equal(t, []string{"string"}, observedLevels(first.observedRecords()))
	require.Equal(t, []string{"string"}, observedLevels(second.observedRecords()))
	require.Equal(t, 2, first.records.count)
	require.Equal(t, first.records.count, second.records.count)

	cached := context.cache[evaluationCacheKey{shape: firstNode.schemaShape, value: value}]
	require.Same(t, cached.result.records, second.records.parts[0].nested)
}

func TestEvaluationCacheStructurallyRebasesCompleteReferenceIdentities(t *testing.T) {
	t.Parallel()

	document := `{
		"openapi":"3.0.4",
		"components":{"schemas":{
			"Leaf":{"type":"string"},
			"Shared":{"type":"object","properties":{"name":{"$ref":"#/components/schemas/Leaf"}}}
		}},
		"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
			"allOf":[
				{"$ref":"#/components/schemas/Shared"},
				{"$ref":"#/components/schemas/Shared"}
			]
		}}}}}}}
	}`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	value, err := parseStrictJSON([]byte(`{"name":1}`))
	require.NoError(t, err)

	result := evaluate(model, value)
	require.NoError(t, result.err)

	rootUse := model.root.occurrence.usePointer
	sharedTarget := "#/components/schemas/Shared"
	leafTarget := "#/components/schemas/Leaf"
	require.Equal(t, []ruleIdentity{
		makeRuleIdentity(schemaOccurrence{
			usePointer:       rootUse + "/allOf/0/properties/name",
			targetPointer:    leafTarget,
			instanceTemplate: "#/name",
			reference:        true,
		}, oracleRuleType),
		makeRuleIdentity(schemaOccurrence{
			usePointer:       rootUse + "/allOf/1/properties/name",
			targetPointer:    leafTarget,
			instanceTemplate: "#/name",
			reference:        true,
		}, oracleRuleType),
	}, evaluationRecordValues(result.failureRecords()))

	var referenced []schemaOccurrence

	for identity := range result.applicableRecords() {
		if identity.rule == oracleRuleType && identity.occurrence.reference {
			referenced = append(referenced, identity.occurrence)
		}
	}

	require.Equal(t, []schemaOccurrence{
		{
			usePointer:       rootUse + "/allOf/0",
			targetPointer:    sharedTarget,
			instanceTemplate: "#",
			reference:        true,
		},
		{
			usePointer:       rootUse + "/allOf/0/properties/name",
			targetPointer:    leafTarget,
			instanceTemplate: "#/name",
			reference:        true,
		},
		{
			usePointer:       rootUse + "/allOf/1",
			targetPointer:    sharedTarget,
			instanceTemplate: "#",
			reference:        true,
		},
		{
			usePointer:       rootUse + "/allOf/1/properties/name",
			targetPointer:    leafTarget,
			instanceTemplate: "#/name",
			reference:        true,
		},
	}, referenced)
}

func TestEvaluationFactsReuseOneStructuredOccurrence(t *testing.T) {
	t.Parallel()

	valid := evaluateSchemaValue(t, `{"type":"string"}`, `"x"`)
	require.NoError(t, valid.err)
	validApplicable := valid.records.parts[0].records[0]
	validObserved := valid.records.parts[1].records[0]

	require.NotEmpty(t, validApplicable.identity.occurrence.use.tokens)
	require.Same(
		t,
		&validApplicable.identity.occurrence.use.tokens[0],
		&validObserved.identity.occurrence.use.tokens[0],
	)

	invalid := evaluateSchemaValue(t, `{"type":"string"}`, `1`)
	require.NoError(t, invalid.err)
	invalidApplicable := invalid.records.parts[0].records[0]
	invalidFailure := invalid.records.parts[1].records[0]
	require.Same(
		t,
		&invalidApplicable.identity.occurrence.use.tokens[0],
		&invalidFailure.identity.occurrence.use.tokens[0],
	)
}

func TestEvaluationRecordVisitorsCannotMutateStructuredIdentities(t *testing.T) {
	t.Parallel()

	document := `openapi: 3.0.4
x-shared: &shared
  type: object
  properties:
    name: {type: string}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                a: *shared
                b: *shared
`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	value, err := parseStrictJSON([]byte(`{"a":{"name":"x"},"b":{"name":"x"}}`))
	require.NoError(t, err)

	result := evaluate(model, value)
	require.NoError(t, result.err)
	before := evaluationRecordStrings(result.records)
	result.records.forEach(func(record evaluationRecord) bool {
		for _, pointer := range []*evaluationPointer{
			&record.identity.occurrence.use,
			&record.identity.occurrence.target,
			&record.identity.occurrence.instance,
			&record.identity.occurrence.targetRoot,
		} {
			if len(pointer.tokens) > 0 {
				pointer.tokens[0] = "mutated"
			}
		}

		return true
	})

	require.Equal(t, before, evaluationRecordStrings(result.records))

	again := evaluate(model, value)
	require.NoError(t, again.err)
	require.Equal(t, before, evaluationRecordStrings(again.records))
	require.Equal(t, evaluationRecordValues(result.applicableRecords()), evaluationRecordValues(again.applicableRecords()))
}

func TestEvaluationCachePreservesNestedReferenceTargetBelowInlineAliasTarget(t *testing.T) {
	t.Parallel()

	document := `openapi: 3.0.4
x-shared: &outer
  type: object
  properties:
    definition:
      type: object
      properties:
        leaf: {type: string}
    child:
      $ref: '#/x-shared/properties/definition'
    local: {type: string}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              allOf: [*outer, *outer]
`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Same(t, model.root.allOf[0].schemaShape, model.root.allOf[1].schemaShape)

	value, err := parseStrictJSON([]byte(`{"child":{"leaf":1},"local":"x"}`))
	require.NoError(t, err)

	context := evaluationContext{cache: make(map[evaluationCacheKey]evaluationCacheEntry)}
	first := context.evaluateNode(model.root.allOf[0], value, model.root.allOf[0].occurrence)
	cachedSecond := context.evaluateNode(model.root.allOf[1], value, model.root.allOf[1].occurrence)
	freshSecond := evaluateNode(model.root.allOf[1], value, model.root.allOf[1].occurrence)

	require.NoError(t, first.err)
	require.NoError(t, cachedSecond.err)
	require.NoError(t, freshSecond.err)
	require.Equal(t, evaluationRecordStrings(freshSecond.records), evaluationRecordStrings(cachedSecond.records))
	require.Equal(
		t,
		evaluationRecordValues(freshSecond.applicableRecords()),
		evaluationRecordValues(cachedSecond.applicableRecords()),
	)
	require.Equal(
		t,
		evaluationRecordValues(freshSecond.failureRecords()),
		evaluationRecordValues(cachedSecond.failureRecords()),
	)

	rootUse := model.root.occurrence.usePointer
	secondUse := rootUse + "/allOf/1"
	identities := evaluationRecordValues(cachedSecond.applicableRecords())
	require.Equal(t, secondUse, identities[0].occurrence.targetPointer)
	require.Equal(t, "#/x-shared/properties/definition", identities[1].occurrence.targetPointer)
	require.Equal(t, "#/x-shared/properties/definition/properties/leaf", identities[2].occurrence.targetPointer)
	require.Equal(t, secondUse+"/properties/local", identities[3].occurrence.targetPointer)
}

func TestEvaluationCacheStructurallyRebasesInlineAliasTargets(t *testing.T) {
	t.Parallel()

	document := `openapi: 3.0.4
x-shared: &shared
  type: object
  properties:
    name: {type: string}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              allOf: [*shared, *shared]
`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	require.Same(t, model.root.allOf[0].schemaShape, model.root.allOf[1].schemaShape)

	value, err := parseStrictJSON([]byte(`{"name":"value"}`))
	require.NoError(t, err)

	context := evaluationContext{cache: make(map[evaluationCacheKey]evaluationCacheEntry)}
	firstNode := model.root.allOf[0]
	secondNode := model.root.allOf[1]
	first := context.evaluateNode(firstNode, value, firstNode.occurrence)
	second := context.evaluateNode(secondNode, value, secondNode.occurrence)

	require.NoError(t, first.err)
	require.NoError(t, second.err)

	cached := context.cache[evaluationCacheKey{shape: firstNode.schemaShape, value: value}]
	require.Same(t, cached.result.records, second.records.parts[0].nested)

	firstType := evaluationRecordValues(first.applicableRecords())
	secondType := evaluationRecordValues(second.applicableRecords())

	require.Equal(t, []ruleIdentity{
		makeRuleIdentity(schemaOccurrence{
			usePointer:       firstNode.occurrence.usePointer,
			targetPointer:    firstNode.occurrence.targetPointer,
			instanceTemplate: "#",
		}, oracleRuleType),
		makeRuleIdentity(schemaOccurrence{
			usePointer:       firstNode.occurrence.usePointer + "/properties/name",
			targetPointer:    firstNode.occurrence.targetPointer + "/properties/name",
			instanceTemplate: "#/name",
		}, oracleRuleType),
	}, firstType)
	require.Equal(t, []ruleIdentity{
		makeRuleIdentity(schemaOccurrence{
			usePointer:       secondNode.occurrence.usePointer,
			targetPointer:    secondNode.occurrence.targetPointer,
			instanceTemplate: "#",
		}, oracleRuleType),
		makeRuleIdentity(schemaOccurrence{
			usePointer:       secondNode.occurrence.usePointer + "/properties/name",
			targetPointer:    secondNode.occurrence.targetPointer + "/properties/name",
			instanceTemplate: "#/name",
		}, oracleRuleType),
	}, secondType)
}

func TestEvaluationCacheRebasesBetweenAliasAndReferenceProvenance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		branches string
	}{
		{name: "alias then reference", branches: "[*shared, {$ref: '#/components/schemas/Shared'}]"},
		{name: "reference then alias", branches: "[{$ref: '#/components/schemas/Shared'}, *shared]"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document := `openapi: 3.0.4
x-shared: &shared
  type: object
  properties:
    child: {type: string}
components:
  schemas:
    Shared: *shared
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              allOf: ` + test.branches + "\n"
			model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.NoError(t, err)
			require.Same(t, model.root.allOf[0].schemaShape, model.root.allOf[1].schemaShape)

			value, err := parseStrictJSON([]byte(`{"child":"value"}`))
			require.NoError(t, err)

			result := evaluate(model, value)
			require.NoError(t, result.err)

			rootUse := model.root.occurrence.usePointer
			for index, branch := range model.root.allOf {
				wantRoot := branch.occurrence
				wantRoot.usePointer = rootUse + "/allOf/" + fmt.Sprint(index)
				wantRoot.instanceTemplate = "#"
				wantChild := schemaOccurrence{
					usePointer:       wantRoot.usePointer + "/properties/child",
					targetPointer:    wantRoot.targetPointer + "/properties/child",
					instanceTemplate: "#/child",
				}

				require.Contains(t, evaluationRecordValues(result.applicableRecords()),
					makeRuleIdentity(wantRoot, oracleRuleType))
				require.Contains(t, evaluationRecordValues(result.applicableRecords()),
					makeRuleIdentity(wantChild, oracleRuleType))
			}
		})
	}
}

func TestEvaluationStructurallyRebasesUncachedInlineAliasDescendants(t *testing.T) {
	t.Parallel()

	document := `openapi: 3.0.4
x-shared: &shared
  type: object
  properties:
    child: {type: string}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                a: *shared
                b: *shared
`
	model, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
	require.NoError(t, err)
	value, err := parseStrictJSON([]byte(`{"a":{"child":"first"},"b":{"child":"second"}}`))
	require.NoError(t, err)

	result := evaluate(model, value)
	require.NoError(t, result.err)

	var identities []ruleIdentity

	for identity := range result.applicableRecords() {
		if strings.HasSuffix(identity.occurrence.usePointer, "/properties/child") &&
			identity.rule == oracleRuleType {
			identities = append(identities, identity)
		}
	}

	rootUse := model.root.occurrence.usePointer
	require.Equal(t, []ruleIdentity{
		makeRuleIdentity(schemaOccurrence{
			usePointer:       rootUse + "/properties/a/properties/child",
			targetPointer:    rootUse + "/properties/a/properties/child",
			instanceTemplate: "#/a/child",
		}, oracleRuleType),
		makeRuleIdentity(schemaOccurrence{
			usePointer:       rootUse + "/properties/b/properties/child",
			targetPointer:    rootUse + "/properties/b/properties/child",
			instanceTemplate: "#/b/child",
		}, oracleRuleType),
	}, identities)
}

func TestEvaluationRecordsShareAppendAndRebaseWithoutMaterializing(t *testing.T) {
	t.Parallel()

	child := newEvaluationRecords()
	child.append(makeEvaluationRecord(evaluationRecordApplicable, makeRuleIdentity(schemaOccurrence{
		usePointer:       "#/target",
		targetPointer:    "#/target",
		instanceTemplate: "#",
	}, oracleRuleType)))

	parent := newEvaluationRecords()
	parent.appendRecords(child)
	parent.append(makeEvaluationRecord(evaluationRecordFailure, makeRuleIdentity(schemaOccurrence{
		usePointer:       "#/parent",
		targetPointer:    "#/parent",
		instanceTemplate: "#",
	}, oracleRuleAnyOf)))

	rebased := parent.rebased(
		schemaOccurrence{usePointer: "#/target", instanceTemplate: "#"},
		schemaOccurrence{usePointer: "#/use", instanceTemplate: "#/member"},
	)

	require.Equal(t, 2, rebased.count)
	require.Equal(t, []string{
		"applicable|#/use|#/member|type",
		"failure|#/parent|#/member|anyOf",
	}, evaluationRecordStrings(rebased))
	require.Same(t, parent, rebased.parts[0].nested)
}

func TestEvaluationRecordRebaseUsesExactPointerTokenPrefixes(t *testing.T) {
	t.Parallel()

	records := newEvaluationRecords()
	childOccurrence := schemaOccurrence{
		usePointer:       "#/schema/child",
		targetPointer:    "#/target/child",
		instanceTemplate: "#/member",
	}
	childPaths := mustParseEvaluationOccurrence(childOccurrence, "#/target")
	childOccurrence.structured = &childPaths
	records.append(makeEvaluationRecord(
		evaluationRecordApplicable,
		makeRuleIdentity(childOccurrence, oracleRuleType),
	))
	records.append(makeEvaluationRecord(evaluationRecordApplicable, makeRuleIdentity(schemaOccurrence{
		usePointer:       "#/schema~1sibling/child",
		targetPointer:    "#/target~1sibling/child",
		instanceTemplate: "#/member~1sibling",
	}, oracleRuleType)))

	rebased := records.rebased(
		schemaOccurrence{
			usePointer:       "#/schema",
			targetPointer:    "#/target",
			instanceTemplate: "#",
		},
		schemaOccurrence{
			usePointer:       "#/use",
			targetPointer:    "#/destination",
			instanceTemplate: "#",
		},
	)

	require.Equal(t, []ruleIdentity{
		makeRuleIdentity(schemaOccurrence{
			usePointer:       "#/use/child",
			targetPointer:    "#/destination/child",
			instanceTemplate: "#/member",
		}, oracleRuleType),
		makeRuleIdentity(schemaOccurrence{
			usePointer:       "#/schema~1sibling/child",
			targetPointer:    "#/target~1sibling/child",
			instanceTemplate: "#/member~1sibling",
		}, oracleRuleType),
	}, evaluationRecordValues(selectEvaluationRecords(rebased, func(record evaluationRecord) (ruleIdentity, bool) {
		return record.identity.project(), true
	})))
}

func evaluationRecordValues[T any](sequence iter.Seq[T]) []T {
	var values []T
	for value := range sequence {
		values = append(values, value)
	}

	return values
}

func evaluationRecordStrings(records *evaluationRecords) []string {
	if records == nil || records.count == 0 {
		return nil
	}

	values := make([]string, 0, records.count)
	records.forEach(func(record evaluationRecord) bool {
		name := "unknown"
		suffix := ""

		switch record.kind {
		case evaluationRecordApplicable:
			name = "applicable"
		case evaluationRecordObserved:
			name = "observed"
			suffix = "|" + record.level
		case evaluationRecordComposition:
			name = record.identity.rule
			suffix = "|" + formatTruthVectorForTest(record.branches)
		case evaluationRecordFailure:
			name = "failure"
		}

		values = append(values, name+"|"+record.identity.project().String()+suffix)

		return true
	})

	return values
}

func formatTruthVectorForTest(branches []bool) string {
	values := make([]string, len(branches))
	for index, branch := range branches {
		values[index] = fmt.Sprintf("%t", branch)
	}

	return strings.Join(values, ",")
}
