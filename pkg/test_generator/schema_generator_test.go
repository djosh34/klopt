//nolint:cyclop,gocognit,godoclint // Recursive inspection helpers mirror recursive Schema Objects.
package testgenerator

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"decode_and_validate_generator/pkg/internal/oas"
	"decode_and_validate_generator/pkg/test_generator/internal/suite"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestOpaqueStringCatalog verifies the checked-in construction recipe independently
// of the runtime schema generator. Generation only samples the resulting trusted data.
func TestOpaqueStringCatalog(t *testing.T) {
	t.Parallel()

	require.Len(t, opaqueStringCatalog, opaqueFamilyCount*opaqueFragmentsPerFamily)

	for family := 0; family < opaqueFamilyCount; family++ {
		fragments := opaqueStringCatalog[family*opaqueFragmentsPerFamily : (family+1)*opaqueFragmentsPerFamily]

		for fragmentIndex, fragment := range fragments {
			verifyOpaqueFragment(t, family, fragmentIndex, fragment)
		}

		verifyOpaqueFamilyOverlap(t, fragments)
	}
}

func verifyOpaqueFragment(
	t *testing.T,
	family int,
	fragmentIndex int,
	fragment opaqueStringFragment,
) {
	t.Helper()

	compiled, err := regexp.Compile(fragment.Pattern)
	require.NoError(t, err)
	require.NotEmpty(t, fragment.Pattern)
	require.GreaterOrEqual(t, len(fragment.ValidExamples), 100)
	require.GreaterOrEqual(t, len(fragment.InvalidExamples), 100)

	valid := make(map[string]struct{}, len(fragment.ValidExamples))
	for _, raw := range fragment.ValidExamples {
		value := decodeOpaqueString(t, raw)
		require.Truef(t, compiled.MatchString(value), "family %d fragment %d valid %q", family, fragmentIndex, value)
		require.NoError(t, verifyOpaqueFormat(fragment.Format, value))
		require.GreaterOrEqual(t, utf8.RuneCountInString(value), 1)
		require.LessOrEqual(t, utf8.RuneCountInString(value), 128)
		_, duplicate := valid[value]
		require.False(t, duplicate)

		valid[value] = struct{}{}
	}

	invalid := make(map[string]struct{}, len(fragment.InvalidExamples))
	for _, raw := range fragment.InvalidExamples {
		value := decodeOpaqueString(t, raw)
		require.Falsef(t, compiled.MatchString(value), "family %d fragment %d invalid %q", family, fragmentIndex, value)
		_, duplicate := invalid[value]
		require.False(t, duplicate)

		invalid[value] = struct{}{}
	}
}

func verifyOpaqueFamilyOverlap(t *testing.T, fragments []opaqueStringFragment) {
	t.Helper()

	validSets := make([]map[string]struct{}, len(fragments))

	invalidSets := make([]map[string]struct{}, len(fragments))
	for index, fragment := range fragments {
		validSets[index] = opaqueStringSet(t, fragment.ValidExamples)
		invalidSets[index] = opaqueStringSet(t, fragment.InvalidExamples)
	}

	require.GreaterOrEqual(t, commonOpaqueValues(validSets), 25)
	require.LessOrEqual(t, commonOpaqueValues(validSets), 50)
	require.GreaterOrEqual(t, commonOpaqueValues(invalidSets), 25)
	require.LessOrEqual(t, commonOpaqueValues(invalidSets), 50)

	for index := range fragments {
		require.GreaterOrEqual(t, valuesMissingFromSibling(validSets, index), 25)
		require.GreaterOrEqual(t, valuesMissingFromSibling(invalidSets, index), 25)
	}
}

func decodeOpaqueString(t *testing.T, raw json.RawMessage) string {
	t.Helper()

	var value string
	require.NoError(t, json.Unmarshal(raw, &value))

	return value
}

func verifyOpaqueFormat(format string, value string) error {
	switch format {
	case "":
		return nil
	case "byte":
		_, err := base64.StdEncoding.DecodeString(value)

		return err
	case "date":
		_, err := time.Parse(time.DateOnly, value)

		return err
	case "date-time":
		_, err := time.Parse(time.RFC3339, value)

		return err
	case "email":
		address, err := mail.ParseAddress(value)
		if err != nil {
			return err
		}

		if address.Address != value {
			return fmt.Errorf("email parser canonicalized %q to %q", value, address.Address)
		}

		return nil
	default:
		return fmt.Errorf("unverified opaque format %q", format)
	}
}

func opaqueStringSet(t *testing.T, values []json.RawMessage) map[string]struct{} {
	t.Helper()

	set := make(map[string]struct{}, len(values))
	for _, raw := range values {
		set[decodeOpaqueString(t, raw)] = struct{}{}
	}

	return set
}

func commonOpaqueValues(sets []map[string]struct{}) int {
	common := 0

	for value := range sets[0] {
		inEverySet := true

		for _, set := range sets[1:] {
			if _, ok := set[value]; !ok {
				inEverySet = false

				break
			}
		}

		if inEverySet {
			common++
		}
	}

	return common
}

func valuesMissingFromSibling(sets []map[string]struct{}, selected int) int {
	missing := 0

	for value := range sets[selected] {
		for index, set := range sets {
			if index == selected {
				continue
			}

			if _, ok := set[value]; !ok {
				missing++

				break
			}
		}
	}

	return missing
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

func compileGeneratedSchema(t require.TestingT, candidate GeneratedSchema) (*suite.CompiledSuite, error) {
	source, err := oas.Parse(candidate.OpenAPIJSON, "checkThing")
	if err != nil {
		if candidate.Valid {
			require.NoError(t, err)
		}

		return nil, err
	}

	compiled, err := suite.NewCompiler(source).CompileSuite(suite.MustHaveAllXValidCases)
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

// TestGeneratedSchemasAgainstValidators sends every subsequently generated body
// from a constructible valid schema to the runtime and applicable external validators.
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
	return bytes.Contains(schema, []byte("1e400")) ||
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

	total := 0
	constructible := 0

	for _, seed := range seeds {
		seedConstructible := 0

		for check := 0; check < 1000; check++ {
			candidate := generator.Example(seed + check*1_000_003)[0]
			compiled, err := compileGeneratedSchema(t, candidate)
			total++

			if err == nil && len(compiled.Cases) != 0 {
				seedConstructible++
				constructible++
			}
		}

		t.Logf("constructibility seed=%d constructible=%d total=1000", seed, seedConstructible)
	}

	require.Equal(t, 10_000, total)
	require.GreaterOrEqual(t, constructible, 100)
	t.Logf(
		"constructibility aggregate: constructible=%d unconstructible=%d total=%d rate=%.2f%%",
		constructible,
		total-constructible,
		total,
		float64(constructible)*100/float64(total),
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
