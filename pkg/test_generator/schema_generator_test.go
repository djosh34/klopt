//nolint:cyclop,gocognit,godoclint // Recursive inspection helpers mirror recursive Schema Objects.
package testgenerator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/test_generator/internal/suite"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func compileGeneratedSchema(t require.TestingT, candidate GeneratedSchema) (*suite.CompiledSuite, error) {
	sources, err := oas.Parse(candidate.OpenAPIJSON)
	if err != nil {
		if candidate.Valid {
			require.NoError(t, err)
		}

		return nil, err
	}

	source, ok := sources["checkThing"]
	if !ok {
		err = errors.New(`operationId "checkThing" is missing`)
		if candidate.Valid {
			require.NoError(t, err)
		}

		return nil, err
	}

	compiled, err := suite.NewCompiler(source).CompileSuite()
	if !candidate.Valid {
		require.Error(t, err)

		return compiled, err
	}

	if err == nil {
		require.NotEmpty(t, compiled.Cases)

		return compiled, nil
	}

	var compileError *suite.Error
	require.ErrorAs(t, err, &compileError)
	require.Equal(t, "unconstructible", compileError.Code, "%v\n%s", err, candidate.OpenAPIJSON)

	return nil, err
}

// TestGenerateSchemasMatchesCompilerContract checks valid supported syntax and
// every independently generated invalid clone against the existing compiler.
func TestGenerateSchemasMatchesCompilerContract(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		generated := GenerateSchemas(t)
		require.GreaterOrEqual(t, len(generated), 2)
		require.True(t, generated[0].Valid)

		for index, candidate := range generated {
			require.Truef(t, json.Valid(candidate.OpenAPIJSON), "candidate %d", index)

			if index > 0 {
				require.False(t, candidate.Valid)
			}

			compiled, err := compileGeneratedSchema(t, candidate)
			if candidate.Valid && err == nil {
				require.NotEmpty(t, compiled.Cases)
			}

			if !candidate.Valid {
				require.Error(t, err)
			}
		}
	})
}

func TestGeneratedSchemaPublicResultHasOnlyTwoFacts(t *testing.T) {
	t.Parallel()

	typeOfResult := reflect.TypeFor[GeneratedSchema]()
	require.Equal(t, 2, typeOfResult.NumField())
	require.Equal(t, "OpenAPIJSON", typeOfResult.Field(0).Name)
	require.Equal(t, "Valid", typeOfResult.Field(1).Name)
}

func TestEveryGeneratedMutationIsRejectedIndependently(t *testing.T) {
	t.Parallel()

	clean := generatedOpenAPIDocument(generatedSchemaObject{"type": "string"})

	for mutationID := 0; mutationID < generatedMutationCount; mutationID++ {
		t.Run(fmt.Sprintf("mutation-%02d", mutationID), func(t *testing.T) {
			t.Parallel()

			mutated, ok := cloneGeneratedValue(clean).(map[string]any)
			require.True(t, ok)
			require.NoError(t, mutateGeneratedDocument(mutated, mutationID))

			var encoded bytes.Buffer
			require.NoError(t, encodeGeneratedValue(&encoded, mutated))
			require.True(t, json.Valid(encoded.Bytes()))
			_, err := compileGeneratedSchema(t, GeneratedSchema{OpenAPIJSON: encoded.Bytes(), Valid: false})
			require.Error(t, err)
		})
	}
}

func TestGeneratedSchemasAgainstValidators(t *testing.T) {
	t.Parallel()

	validatedSchemas := 0
	validBodies := 0
	invalidBodies := 0

	rapid.Check(t, func(t *rapid.T) {
		candidate := GenerateSchemas(t)[0]

		compiled, err := compileGeneratedSchema(t, candidate)
		if err != nil {
			return
		}

		runtimeAdapter, err := newRuntimeValidationRequestBodyValidator(candidate.OpenAPIJSON)
		require.NoError(t, err, "%s", candidate.OpenAPIJSON)

		adapters := []validatorAdapter{runtimeAdapter}

		if !hasCharacterizedExternalValidatorLimitation(candidate.OpenAPIJSON) {
			externalAdapters, externalErr := newExternalValidatorAdapters(candidate.OpenAPIJSON)
			if !isCharacterizedExternalSetupFailure(candidate.OpenAPIJSON, externalErr) {
				require.NoError(t, externalErr, "%s", candidate.OpenAPIJSON)

				adapters = append(adapters, externalAdapters...)
			}
		}

		defer releaseValidatorAdapters(adapters)

		validatedSchemas++

		for _, plannedCase := range compiled.Cases {
			body, err := drawPlannedBody(t, plannedCase)
			require.NoError(t, err)

			if plannedCase.Expect == suite.ExpectAccepted {
				validBodies++
			} else {
				invalidBodies++
			}

			for _, adapter := range adapters {
				if !validatorChecksCase(adapter, plannedCase) {
					continue
				}

				validationErr := adapter.validator.Validate(body)
				require.Truef(
					t,
					validatorVerdictMatches(plannedCase.Expect, validationErr),
					"schema: %s\nCasePlan: %s\nbody: %s\nadapter: %s\nerror: %v",
					candidate.OpenAPIJSON,
					plannedCase.Name,
					body,
					adapter.name,
					validationErr,
				)
			}
		}
	})

	require.Positive(t, validatedSchemas)
	require.Positive(t, validBodies)
	require.Positive(t, invalidBodies)
	t.Logf(
		"validator totals: schemas=%d valid bodies=%d invalid bodies=%d",
		validatedSchemas,
		validBodies,
		invalidBodies,
	)
}

func isCharacterizedExternalSetupFailure(schema []byte, err error) bool {
	if err == nil {
		return false
	}

	if hasCharacterizedExternalValidatorLimitation(schema) {
		return true
	}

	return strings.Contains(err.Error(), "request schema failed to compile") &&
		generatedSchemaHasEmptyProperty(schema)
}

func generatedSchemaHasEmptyProperty(raw []byte) bool {
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return false
	}

	schema, err := generatedRequestSchemaFromJSON(document)
	if err != nil {
		return false
	}

	return schemaHasEmptyProperty(schema)
}

func schemaHasEmptyProperty(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}

	if properties, ok := object["properties"].(map[string]any); ok {
		if _, empty := properties[""]; empty {
			return true
		}

		for _, child := range properties {
			if schemaHasEmptyProperty(child) {
				return true
			}
		}
	}

	if schemaHasEmptyProperty(object["items"]) || schemaHasEmptyProperty(object["additionalProperties"]) {
		return true
	}

	if children, ok := object["allOf"].([]any); ok {
		for _, child := range children {
			if schemaHasEmptyProperty(child) {
				return true
			}
		}
	}

	return false
}

// hasCharacterizedExternalValidatorLimitation routes pinned cases away from third-party adapters only.
func hasCharacterizedExternalValidatorLimitation(schema []byte) bool {
	withoutPassword := bytes.ReplaceAll(schema, []byte(`"format":"password"`), nil)

	return bytes.Contains(withoutPassword, []byte(`"format":`)) ||
		bytes.Contains(schema, []byte("1e400")) ||
		bytes.Contains(schema, []byte("-1e400")) ||
		bytes.Contains(schema, []byte("1e300")) ||
		bytes.Contains(schema, []byte("0.0000000000000000000000000001")) ||
		bytes.Contains(schema, []byte("900719925474")) ||
		bytes.Contains(schema, []byte("1.234567890123456789")) ||
		bytes.Contains(schema, []byte("9.876543210987654321")) ||
		bytes.Contains(schema, []byte("Reference Object siblings are ignored")) ||
		generatedSchemaContainsNullableAllOf(schema) ||
		generatedSchemaContainsTypelessOccurrence(schema)
}

func TestPasswordDoesNotDisableExternalValidatorCoverage(t *testing.T) {
	t.Parallel()

	require.False(t, hasCharacterizedExternalValidatorLimitation([]byte(`{"type":"string","format":"password"}`)))
	require.True(t, hasCharacterizedExternalValidatorLimitation([]byte(`{"type":"string","format":"uuid"}`)))
}

func generatedSchemaContainsNullableAllOf(raw []byte) bool {
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return false
	}

	schema, err := generatedRequestSchemaFromJSON(document)
	if err != nil {
		return false
	}

	return schemaContainsNullableAllOf(schema)
}

func schemaContainsNullableAllOf(value any) bool {
	return schemaContainsNullableWithinAllOf(value, false)
}

func schemaContainsNullableWithinAllOf(value any, withinAllOf bool) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}

	_, hasAllOf := object["allOf"].([]any)
	if nullable, ok := object["nullable"].(bool); ok && nullable && (withinAllOf || hasAllOf) {
		return true
	}

	for _, keyword := range []string{"items", "additionalProperties"} {
		if schemaContainsNullableWithinAllOf(object[keyword], withinAllOf) {
			return true
		}
	}

	if properties, ok := object["properties"].(map[string]any); ok {
		for _, child := range properties {
			if schemaContainsNullableWithinAllOf(child, withinAllOf) {
				return true
			}
		}
	}

	if children, ok := object["allOf"].([]any); ok {
		for _, child := range children {
			if schemaContainsNullableWithinAllOf(child, true) {
				return true
			}
		}
	}

	return false
}

func generatedSchemaContainsTypelessOccurrence(raw []byte) bool {
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return false
	}

	schema, err := generatedRequestSchemaFromJSON(document)
	if err != nil {
		return false
	}

	return schemaContainsTypelessOccurrence(schema)
}

func schemaContainsTypelessOccurrence(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}

	if _, reference := object["$ref"]; !reference {
		if _, typed := object["type"]; !typed {
			return true
		}
	}

	for _, keyword := range []string{"items", "additionalProperties"} {
		if schemaContainsTypelessOccurrence(object[keyword]) {
			return true
		}
	}

	if properties, ok := object["properties"].(map[string]any); ok {
		for _, child := range properties {
			if schemaContainsTypelessOccurrence(child) {
				return true
			}
		}
	}

	if children, ok := object["allOf"].([]any); ok {
		for _, child := range children {
			if schemaContainsTypelessOccurrence(child) {
				return true
			}
		}
	}

	return false
}

func TestGeneratedSchemaConstructibilityBacktest(t *testing.T) {
	t.Parallel()

	seeds := []int{17001, 27011, 37021, 47031, 57041, 67051, 77061, 87071, 97081, 107091}
	generator := rapid.Custom(func(t *rapid.T) []GeneratedSchema {
		return GenerateSchemas(t)
	})

	total := atomic.Int32{}
	constructible := atomic.Int32{}

	wg := sync.WaitGroup{}

	for _, seed := range seeds {
		wg.Go(func() {
			for check := 0; check < 100; check++ {
				candidate := generator.Example(seed + check*1_000_003)[0]
				compiled, err := compileGeneratedSchema(t, candidate)

				total.Add(1)

				if err == nil && len(compiled.Cases) != 0 {
					constructible.Add(1)
				}
			}
		})
	}

	wg.Wait()

	require.Equal(t, 1_000, int(total.Load()))
	require.GreaterOrEqual(t, int(constructible.Load()), 10)
	t.Logf(
		"constructibility aggregate: constructible=%d unconstructible=%d total=%d rate=%.2f%%",
		constructible.Load(),
		total.Load()-constructible.Load(),
		total.Load(),
		float64(constructible.Load())*100/float64(total.Load()),
	)
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
