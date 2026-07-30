//nolint:cyclop,gocognit,godoclint // Recursive inspection helpers mirror recursive Schema Objects.
package testgenerator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestGeneratedSchemaPublicResultHasOnlyTwoFacts(t *testing.T) {
	t.Parallel()

	typeOfResult := reflect.TypeFor[GeneratedSchema]()
	require.Equal(t, 2, typeOfResult.NumField())
	require.Equal(t, "OpenAPIJSON", typeOfResult.Field(0).Name)
	require.Equal(t, "Valid", typeOfResult.Field(1).Name)
}

// TestGeneratedSchemasExerciseRequiredShapes keeps generation pressure visible
// without leaking counters into the public generator result.
func TestGeneratedSchemasExerciseRequiredShapes(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	counters := generatedCoverageCounters{refPositions: make(map[string]int)}

	for seed := 0; seed < 2000; seed++ {
		generated := rapid.Custom(func(t *rapid.T) []GeneratedSchema {
			return GenerateSchemas(t)
		}).Example(seed)
		counters.legal++
		counters.invalid += len(generated) - 1
		collectGeneratedCoverage(t, generated[0].OpenAPIJSON, seen, &counters)
	}

	for _, feature := range []string{
		"typeless", "nullable", "nullable-object-false", "nullable-object-true",
		"number", "exact-number", "enum", "pattern", "format",
		"array", "object", "additional-schema", "allOf", "nested-allOf", "ref", "escaped-ref", "unicode-ref",
	} {
		require.Truef(t, seen[feature], "feature %s was not generated", feature)
	}

	for _, position := range []string{"items", "property", "additionalProperties", "allOf"} {
		require.Positivef(t, counters.refPositions[position], "ref position %s", position)
	}

	require.Positive(t, counters.keywordPairs)
	require.Positive(t, counters.keywordTriples)
	t.Logf(
		"generation counters: legal=%d invalid=%d max-depth=%d max-allOf-depth=%d "+
			"keyword-pairs=%d keyword-triples=%d ref-positions=%v",
		counters.legal,
		counters.invalid,
		counters.maxDepth,
		counters.maxAllOfDepth,
		counters.keywordPairs,
		counters.keywordTriples,
		counters.refPositions,
	)
}

type generatedCoverageCounters struct {
	legal          int
	invalid        int
	maxDepth       int
	maxAllOfDepth  int
	keywordPairs   int
	keywordTriples int
	refPositions   map[string]int
}

func collectGeneratedCoverage(
	t *testing.T,
	raw []byte,
	seen map[string]bool,
	counters *generatedCoverageCounters,
) {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var document map[string]any
	require.NoError(t, decoder.Decode(&document))

	root, err := generatedRequestSchemaFromJSON(document)
	require.NoError(t, err)
	collectSchemaCoverage(root, seen, counters, 0, 0, "root")
}

func collectSchemaCoverage(
	value any,
	seen map[string]bool,
	counters *generatedCoverageCounters,
	depth int,
	allOfDepth int,
	position string,
) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}

	counters.maxDepth = max(counters.maxDepth, depth)

	counters.maxAllOfDepth = max(counters.maxAllOfDepth, allOfDepth)
	if len(object) >= 2 {
		counters.keywordPairs++
	}

	if len(object) >= 3 {
		counters.keywordTriples++
	}

	if _, typed := object["type"]; !typed {
		seen["typeless"] = true
	}

	if object["type"] == "object" {
		if nullable, ok := object["nullable"].(bool); ok {
			seen[fmt.Sprintf("nullable-object-%t", nullable)] = true
		}
	}

	for keyword, feature := range map[string]string{
		"nullable": "nullable", "enum": "enum", "pattern": "pattern", "format": "format",
		"items": "array", "properties": "object", "$ref": "ref",
	} {
		if _, ok := object[keyword]; ok {
			seen[feature] = true
		}
	}

	if reference, ok := object["$ref"].(string); ok &&
		(strings.Contains(reference, "~0") || strings.Contains(reference, "~1")) {
		seen["escaped-ref"] = true
	}

	if reference, ok := object["$ref"].(string); ok && strings.Contains(reference, "%CE%BB") {
		seen["unicode-ref"] = true
	}

	if _, ok := object["$ref"].(string); ok {
		counters.refPositions[position]++
	}

	for _, keyword := range []string{"minimum", "maximum", "multipleOf"} {
		if number, ok := object[keyword].(json.Number); ok {
			seen["number"] = true
			if len(number.String()) > 16 || strings.Contains(number.String(), "e") {
				seen["exact-number"] = true
			}
		}
	}

	if additional, ok := object["additionalProperties"].(map[string]any); ok {
		seen["additional-schema"] = true
		collectSchemaCoverage(additional, seen, counters, depth+1, allOfDepth, "additionalProperties")
	}

	if items, ok := object["items"]; ok {
		collectSchemaCoverage(items, seen, counters, depth+1, allOfDepth, "items")
	}

	if properties, ok := object["properties"].(map[string]any); ok {
		for _, property := range properties {
			collectSchemaCoverage(property, seen, counters, depth+1, allOfDepth, "property")
		}
	}

	if children, ok := object["allOf"].([]any); ok {
		seen["allOf"] = true
		if allOfDepth > 0 {
			seen["nested-allOf"] = true
		}

		for _, child := range children {
			collectSchemaCoverage(child, seen, counters, depth+1, allOfDepth+1, "allOf")
		}
	}
}

func generatedRequestSchemaFromJSON(document map[string]any) (map[string]any, error) {
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		return nil, errors.New("paths is not an object")
	}

	path, ok := paths["/things"].(map[string]any)
	if !ok {
		return nil, errors.New("generated path is not an object")
	}

	post, ok := path["post"].(map[string]any)
	if !ok {
		return nil, errors.New("generated operation is not an object")
	}

	requestBody, ok := post["requestBody"].(map[string]any)
	if !ok {
		return nil, errors.New("generated requestBody is not an object")
	}

	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		return nil, errors.New("generated content is not an object")
	}

	mediaType, ok := content["application/json"].(map[string]any)
	if !ok {
		return nil, errors.New("generated media type is not an object")
	}

	schema, ok := mediaType["schema"].(map[string]any)
	if !ok {
		return nil, errors.New("generated schema is not an object")
	}

	return schema, nil
}
