package testgenerator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/goccy/go-yaml"
)

const (
	// corpusCategoryTypes contains type, nullable, and enum schemas.
	corpusCategoryTypes = "types-nullable-enums"
	// corpusCategoryNumeric contains numeric boundary and combination schemas.
	corpusCategoryNumeric = "numeric-boundaries-combinations"
	// corpusCategoryStrings contains string, format, and pattern schemas.
	corpusCategoryStrings = "strings-formats-patterns"
	// corpusCategoryArrays contains collection and item schemas.
	corpusCategoryArrays = "arrays"
	// corpusCategoryObjects contains object and additional-property schemas.
	corpusCategoryObjects = "objects-additional-properties"
	// corpusCategoryRefs contains local-reference and allOf schemas.
	corpusCategoryRefs = "local-refs-allof"
	// corpusCategoryCross contains realistic cross-family schemas.
	corpusCategoryCross = "cross-family"
)

// validatorCorpusFixture is one deterministic request-body schema shared by both adapters.
type validatorCorpusFixture struct {
	ID         string
	Category   string
	Schema     string
	Components string
}

// spec builds the complete OpenAPI document used for duplicate checks, generator compilation, and both adapters.
func (fixture validatorCorpusFixture) spec() []byte {
	return append(requestBodySpec(fixture.Schema), fixture.Components...)
}

// corpusFixture gives each row a category-qualified, stable identifier.
func corpusFixture(category string, dimensions string, schema string) validatorCorpusFixture {
	return validatorCorpusFixture{
		ID: category + "/" + dimensions, Category: category, Schema: schema,
	}
}

// corpusFixtureWithComponents gives a fixture its local component schemas without adapter-specific copies.
func corpusFixtureWithComponents(
	category string,
	dimensions string,
	schema string,
	components string,
) validatorCorpusFixture {
	fixture := corpusFixture(category, dimensions, schema)
	fixture.Components = components

	return fixture
}

// validatorCorpus is the sole shared fixture table. Its builders append in a fixed category and dimension order.
func validatorCorpus() []validatorCorpusFixture {
	fixtures := make([]validatorCorpusFixture, 0, 320)
	fixtures = append(fixtures, typeNullableEnumCorpus()...)
	fixtures = append(fixtures, numericCorpus()...)
	fixtures = append(fixtures, stringCorpus()...)
	fixtures = append(fixtures, arrayCorpus()...)
	fixtures = append(fixtures, objectCorpus()...)
	fixtures = append(fixtures, referenceAllOfCorpus()...)
	fixtures = append(fixtures, crossFamilyCorpus()...)

	return fixtures
}

// TestValidatorCorpusSelfCheck catches accidental duplicate fixtures and count drift before the broad matrix runs.
func TestValidatorCorpusSelfCheck(t *testing.T) {
	t.Parallel()

	if err := validateValidatorCorpus(validatorCorpus()); err != nil {
		t.Fatal(err)
	}
}

// validateValidatorCorpus verifies the exact agreed category matrix and semantic complete-spec uniqueness.
func validateValidatorCorpus(fixtures []validatorCorpusFixture) error {
	expectedCounts := map[string]int{
		corpusCategoryTypes:   44,
		corpusCategoryNumeric: 64,
		corpusCategoryStrings: 52,
		corpusCategoryArrays:  36,
		corpusCategoryObjects: 52,
		corpusCategoryRefs:    34,
		corpusCategoryCross:   38,
	}
	expectedCategories := []string{
		corpusCategoryTypes,
		corpusCategoryNumeric,
		corpusCategoryStrings,
		corpusCategoryArrays,
		corpusCategoryObjects,
		corpusCategoryRefs,
		corpusCategoryCross,
	}

	if len(fixtures) != 320 {
		return fmt.Errorf("validator corpus total is %d, want 320", len(fixtures))
	}

	var (
		counts = make(map[string]int, len(expectedCounts))
		ids    = make(map[string]struct{}, len(fixtures))
		specs  = make(map[string]string, len(fixtures))
	)

	for _, fixture := range fixtures {
		if fixture.ID == "" {
			return errors.New("validator corpus has an empty fixture ID")
		}

		if _, knownCategory := expectedCounts[fixture.Category]; !knownCategory {
			return fmt.Errorf("validator corpus has unexpected category %q", fixture.Category)
		}

		if _, duplicate := ids[fixture.ID]; duplicate {
			return fmt.Errorf("validator corpus has duplicate fixture ID %q", fixture.ID)
		}

		ids[fixture.ID] = struct{}{}

		canonical, canonicalErr := canonicalCorpusSpec(fixture.spec())
		if canonicalErr != nil {
			return fmt.Errorf("canonicalize fixture %q: %w", fixture.ID, canonicalErr)
		}

		if previousID, duplicate := specs[canonical]; duplicate {
			return fmt.Errorf("validator corpus fixtures %q and %q have duplicate complete specs", previousID, fixture.ID)
		}

		specs[canonical] = fixture.ID
		counts[fixture.Category]++
	}

	for _, category := range expectedCategories {
		if counts[category] != expectedCounts[category] {
			return fmt.Errorf(
				"validator corpus category %q has %d fixtures, want %d",
				category,
				counts[category],
				expectedCounts[category],
			)
		}
	}

	return nil
}

// canonicalCorpusSpec turns YAML into canonical JSON so formatting cannot conceal duplicate complete specifications.
func canonicalCorpusSpec(spec []byte) (string, error) {
	document, err := yaml.YAMLToJSON(spec)
	if err != nil {
		return "", fmt.Errorf("convert YAML to JSON: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()

	var value any
	if decodeErr := decoder.Decode(&value); decodeErr != nil {
		return "", fmt.Errorf("decode document JSON: %w", decodeErr)
	}

	canonical, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal canonical document JSON: %w", err)
	}

	return string(canonical), nil
}

// typeNullableEnumCorpus covers omitted/single types, nullable, and enum partitions in 44 rows.
func typeNullableEnumCorpus() []validatorCorpusFixture {
	fixtures := []validatorCorpusFixture{
		corpusFixture(corpusCategoryTypes, "omitted-type-any-json", `
      nullable: true
      minLength: 0
`),
		corpusFixture(corpusCategoryTypes, "type-boolean", `
      type: boolean
`),
		corpusFixture(corpusCategoryTypes, "type-integer", `
      type: integer
`),
		corpusFixture(corpusCategoryTypes, "type-number", `
      type: number
`),
		corpusFixture(corpusCategoryTypes, "type-string", `
      type: string
`),
		corpusFixture(corpusCategoryTypes, "type-array", `
      type: array
      items: {type: string}
`),
		corpusFixture(corpusCategoryTypes, "type-object", `
      type: object
      additionalProperties: true
`),
	}

	for _, variant := range []struct {
		id     string
		schema string
	}{
		{id: "nullable-true-boolean", schema: "type: boolean\nnullable: true"},
		{id: "nullable-false-boolean", schema: "type: boolean\nnullable: false"},
		{id: "nullable-true-integer", schema: "type: integer\nnullable: true"},
		{id: "nullable-false-integer", schema: "type: integer\nnullable: false"},
		{id: "nullable-true-number", schema: "type: number\nnullable: true"},
		{id: "nullable-false-number", schema: "type: number\nnullable: false"},
		{id: "nullable-true-string", schema: "type: string\nnullable: true"},
		{id: "nullable-false-string", schema: "type: string\nnullable: false"},
		{id: "nullable-true-array", schema: "type: array\nnullable: true\nitems: {type: string}"},
		{id: "nullable-false-array", schema: "type: array\nnullable: false\nitems: {type: string}"},
		{id: "nullable-true-object", schema: "type: object\nnullable: true\nadditionalProperties: false"},
		{id: "nullable-false-object", schema: "type: object\nnullable: false\nadditionalProperties: false"},
	} {
		fixtures = append(fixtures, corpusFixture(corpusCategoryTypes, variant.id, variant.schema))
	}

	fixtures = append(fixtures,
		corpusFixture(corpusCategoryTypes, "enum-homogeneous-boolean", `
      type: boolean
      enum: [true, false]
`),
		corpusFixture(corpusCategoryTypes, "enum-homogeneous-integer", `
      type: integer
      enum: [1, 2]
`),
		corpusFixture(corpusCategoryTypes, "enum-homogeneous-number", `
      type: number
      enum: [0.5, 1.5]
`),
		corpusFixture(corpusCategoryTypes, "enum-homogeneous-string", `
      type: string
      enum: [red, blue]
`),
		corpusFixture(corpusCategoryTypes, "enum-homogeneous-array", `
      type: array
      items: {type: string}
      enum: [[one], [two]]
`),
		corpusFixture(corpusCategoryTypes, "enum-homogeneous-object", `
      type: object
      required: [a]
      properties:
        a: {type: string}
      additionalProperties: false
      enum: [{a: one}, {a: two}]
`),
		corpusFixture(corpusCategoryTypes, "enum-mixed-json-kinds", `
      nullable: true
      enum: [null, true, 1, "λ", [], {}]
`),
		// The official OAS 3.0 schema requires enum minItems: 1. These rows cover empty JSON values as members;
		// an invalid zero-member enum cannot enter the strict consensus corpus.
		corpusFixture(corpusCategoryTypes, "enum-empty-string-value", `
      type: string
      enum: [""]
`),
		corpusFixture(corpusCategoryTypes, "enum-empty-array-value", `
      type: array
      items: {type: string}
      enum: [[]]
`),
		corpusFixture(corpusCategoryTypes, "enum-empty-object-value", `
      type: object
      enum: [{}]
`),
		corpusFixture(corpusCategoryTypes, "enum-singleton-boolean", `
      type: boolean
      enum: [true]
`),
		corpusFixture(corpusCategoryTypes, "enum-singleton-integer", `
      type: integer
      enum: [7]
`),
		corpusFixture(corpusCategoryTypes, "enum-singleton-string", `
      type: string
      enum: [solo]
`),
		corpusFixture(corpusCategoryTypes, "enum-singleton-object", `
      type: object
      properties:
        kind: {type: string}
      additionalProperties: false
      enum: [{kind: only}]
`),
		corpusFixture(corpusCategoryTypes, "enum-singleton-array", `
      type: array
      items: {type: string}
      enum: [[only]]
`),
		corpusFixture(corpusCategoryTypes, "enum-null-containing-string", `
      type: string
      nullable: true
      enum: [null, text]
`),
		corpusFixture(corpusCategoryTypes, "enum-null-only-untyped", `
      nullable: true
      enum: [null]
`),
		corpusFixture(corpusCategoryTypes, "enum-integer-bounds", `
      type: integer
      minimum: 0
      maximum: 2
      enum: [-1, 0, 1, 2, 3]
`),
		corpusFixture(corpusCategoryTypes, "enum-number-multiple", `
      type: number
      minimum: -1
      maximum: 1
      multipleOf: 0.5
      enum: [-1, -0.5, 0, 0.5, 1]
`),
		corpusFixture(corpusCategoryTypes, "enum-string-length", `
      type: string
      minLength: 2
      maxLength: 3
      enum: [a, ok, yes, tool]
`),
		corpusFixture(corpusCategoryTypes, "enum-string-pattern", `
      type: string
      pattern: '^OK$'
      x-valid-examples: [OK]
      x-invalid-examples: [bad]
      enum: [OK]
`),
		corpusFixture(corpusCategoryTypes, "enum-array-bounds", `
      type: array
      minItems: 1
      maxItems: 2
      items: {type: string}
      enum: [[one], [one, two]]
`),
		corpusFixture(corpusCategoryTypes, "enum-object-required", `
      type: object
      required: [state]
      properties:
        state: {type: string, enum: [open, closed]}
      additionalProperties: false
      enum: [{state: open}, {state: closed}]
`),
		corpusFixture(corpusCategoryTypes, "enum-nullable-number", `
      type: number
      nullable: true
      minimum: 0
      enum: [null, 0, 1]
`),
		corpusFixture(corpusCategoryTypes, "enum-untyped-sibling-constraint", `
      nullable: true
      minLength: 1
      enum: ["", x, 1, null]
`),
	)

	return fixtures
}

// numericCorpus covers inclusive/exclusive boundaries, lattice combinations, and safe numeric spellings in 64 rows.
func numericCorpus() []validatorCorpusFixture {
	fixtures := make([]validatorCorpusFixture, 0, 64)

	for _, typeName := range []string{"integer", "number"} {
		for _, bound := range []struct {
			keyword          string
			exclusiveKeyword string
		}{
			{keyword: "minimum", exclusiveKeyword: "exclusiveMinimum"},
			{keyword: "maximum", exclusiveKeyword: "exclusiveMaximum"},
		} {
			for _, value := range []int{-2, 0, 2} {
				for _, exclusive := range []bool{false, true} {
					fixtures = append(fixtures, corpusFixture(
						corpusCategoryNumeric,
						fmt.Sprintf("%s-%s-%d-exclusive-%t", typeName, bound.keyword, value, exclusive),
						fmt.Sprintf(
							"type: %s\n%s: %d\n%s: %t",
							typeName,
							bound.keyword,
							value,
							bound.exclusiveKeyword,
							exclusive,
						),
					))
				}
			}
		}
	}

	for _, typeName := range []string{"integer", "number"} {
		for _, interval := range []struct {
			id               string
			minimum          int
			maximum          int
			exclusiveMinimum bool
			exclusiveMaximum bool
		}{
			{id: "negative-positive-inclusive", minimum: -2, maximum: 2},
			{id: "negative-positive-exclusive", minimum: -2, maximum: 2, exclusiveMinimum: true, exclusiveMaximum: true},
			{id: "zero-positive-exclusive-maximum", minimum: 0, maximum: 2, exclusiveMaximum: true},
			{id: "negative-zero-exclusive-minimum", minimum: -2, maximum: 0, exclusiveMinimum: true},
			{id: "zero-exact", minimum: 0, maximum: 0},
			{id: "negative-one-positive-one-mixed", minimum: -1, maximum: 1, exclusiveMinimum: true},
		} {
			fixtures = append(fixtures, corpusFixture(
				corpusCategoryNumeric,
				fmt.Sprintf("%s-range-%s", typeName, interval.id),
				fmt.Sprintf(
					"type: %s\nminimum: %d\nmaximum: %d\nexclusiveMinimum: %t\nexclusiveMaximum: %t",
					typeName,
					interval.minimum,
					interval.maximum,
					interval.exclusiveMinimum,
					interval.exclusiveMaximum,
				),
			))
		}
	}

	for _, typeName := range []string{"integer", "number"} {
		for _, multiplier := range []string{"2", "3", "0.5", "0.25"} {
			for _, rangeName := range []struct {
				id      string
				minimum int
				maximum int
			}{
				{id: "negative-positive", minimum: -8, maximum: 8},
				{id: "zero-positive", minimum: 0, maximum: 8},
			} {
				fixtures = append(fixtures, corpusFixture(
					corpusCategoryNumeric,
					fmt.Sprintf("%s-multiple-of-%s-%s", typeName, multiplier, rangeName.id),
					fmt.Sprintf(
						"type: %s\nminimum: %d\nmaximum: %d\nmultipleOf: %s",
						typeName,
						rangeName.minimum,
						rangeName.maximum,
						multiplier,
					),
				))
			}
		}
	}

	fixtures = append(fixtures,
		corpusFixture(corpusCategoryNumeric, "integer-nullable-enum-bounds", `
      type: integer
      nullable: true
      minimum: 0
      maximum: 2
      enum: [null, 0, 1, 2]
`),
		corpusFixture(corpusCategoryNumeric, "number-nullable-enum-bounds", `
      type: number
      nullable: true
      minimum: -1
      maximum: 1
      enum: [null, -1, 0, 1]
`),
		corpusFixture(corpusCategoryNumeric, "integer-enum-minimum", `
      type: integer
      minimum: -1
      enum: [-2, -1, 0, 1]
`),
		corpusFixture(corpusCategoryNumeric, "number-enum-exclusive-maximum", `
      type: number
      maximum: 1
      exclusiveMaximum: true
      enum: [-1, 0, 0.5]
`),
		corpusFixture(corpusCategoryNumeric, "integer-enum-multiple-of-three", `
      type: integer
      minimum: 0
      maximum: 9
      multipleOf: 3
      enum: [0, 3, 6, 9]
`),
		corpusFixture(corpusCategoryNumeric, "number-enum-multiple-of-quarter", `
      type: number
      minimum: -0.5
      maximum: 0.5
      multipleOf: 0.25
      enum: [-0.5, -0.25, 0, 0.25, 0.5]
`),
		corpusFixture(corpusCategoryNumeric, "integer-nullable-exclusive-minimum", `
      type: integer
      nullable: true
      minimum: 0
      exclusiveMinimum: true
      maximum: 3
`),
		corpusFixture(corpusCategoryNumeric, "number-nullable-multiple-of-half", `
      type: number
      nullable: true
      minimum: -2
      maximum: 2
      multipleOf: 0.5
`),
		corpusFixture(corpusCategoryNumeric, "number-safe-exponent-bounds", `
      type: number
      minimum: 1.0e2
      maximum: 1.0e3
`),
		corpusFixture(corpusCategoryNumeric, "number-negative-zero-bound", `
      type: number
      minimum: -0
      maximum: 1
`),
		corpusFixture(corpusCategoryNumeric, "integer-negative-zero-enum", `
      type: integer
      enum: [-0, 0, 1]
`),
		corpusFixture(corpusCategoryNumeric, "number-exponent-negative-zero-multiple", `
      type: number
      minimum: -0
      maximum: 1.0e1
      multipleOf: 0.25
`),
	)

	return fixtures
}

// stringCorpus covers string lengths, portable patterns, trusted formats, and combinations in 52 rows.
func stringCorpus() []validatorCorpusFixture {
	fixtures := []validatorCorpusFixture{
		corpusFixture(corpusCategoryStrings, "length-minimum-zero", "type: string\nminLength: 0"),
		corpusFixture(corpusCategoryStrings, "length-minimum-one", "type: string\nminLength: 1"),
		corpusFixture(corpusCategoryStrings, "length-minimum-three", "type: string\nminLength: 3"),
		corpusFixture(corpusCategoryStrings, "length-maximum-zero", "type: string\nmaxLength: 0"),
		corpusFixture(corpusCategoryStrings, "length-maximum-one", "type: string\nmaxLength: 1"),
		corpusFixture(corpusCategoryStrings, "length-maximum-three", "type: string\nmaxLength: 3"),
		corpusFixture(corpusCategoryStrings, "length-equal-zero", "type: string\nminLength: 0\nmaxLength: 0"),
		corpusFixture(corpusCategoryStrings, "length-equal-one", "type: string\nminLength: 1\nmaxLength: 1"),
		corpusFixture(corpusCategoryStrings, "length-range-two-four", "type: string\nminLength: 2\nmaxLength: 4"),
		corpusFixture(corpusCategoryStrings, "length-range-zero-four", "type: string\nminLength: 0\nmaxLength: 4"),
		corpusFixture(corpusCategoryStrings, "length-impossible-untyped", "nullable: true\nminLength: 2\nmaxLength: 1"),
		corpusFixture(corpusCategoryStrings, "length-unicode-single-rune", `
      type: string
      minLength: 1
      maxLength: 1
      enum: [λ]
`),
		corpusFixture(corpusCategoryStrings, "length-unicode-two-runes", `
      type: string
      minLength: 2
      maxLength: 2
      enum: [λx]
`),
		corpusFixture(corpusCategoryStrings, "length-empty-string-enum", "type: string\nmaxLength: 0\nenum: [\"\"]"),
	}

	for _, pattern := range []struct {
		id      string
		value   string
		valid   string
		invalid string
	}{
		{id: "anchored", value: "^OK$", valid: "OK", invalid: "bad"},
		{id: "unanchored", value: "cat", valid: "bobcat", invalid: "dog"},
		{id: "escaped-dot", value: `^[0-9]{2}\.[0-9]{2}$`, valid: "\"12.34\"", invalid: "\"1234\""},
		{id: "alternation", value: "^(red|blue)$", valid: "red", invalid: "green"},
		{id: "letters-digits", value: "^[A-Z]{2}-[0-9]{2}$", valid: "AB-12", invalid: "ab-12"},
		{id: "unicode", value: "^λ[0-9]$", valid: "λ7", invalid: "λ"},
		{id: "lowercase", value: "^[a-z]+$", valid: "word", invalid: "WORD"},
		{id: "escaped-plus", value: "^a[+]b$", valid: "a+b", invalid: "ab"},
		{id: "optional-suffix", value: "^item(-[0-9]+)?$", valid: "item-2", invalid: "other"},
		{id: "underscore", value: "^[a-z0-9_]+$", valid: "a_1", invalid: "a-1"},
		{id: "yes-no", value: "^(yes|no)$", valid: "yes", invalid: "maybe"},
		{id: "empty", value: "^$", valid: "\"\"", invalid: "x"},
		{id: "not-slash", value: "^[^/]+$", valid: "plain", invalid: "a/b"},
		{id: "repetition", value: "^x{2,3}$", valid: "xx", invalid: "x"},
		{id: "digits", value: "^[0-9]+$", valid: "\"42\"", invalid: "forty"},
		{id: "direction", value: "^(up|down)-[0-9]$", valid: "up-1", invalid: "side-1"},
	} {
		fixtures = append(fixtures, corpusFixture(
			corpusCategoryStrings,
			"pattern-"+pattern.id,
			fmt.Sprintf(
				"type: string\npattern: '%s'\nx-valid-examples: [%s]\nx-invalid-examples: [%s]",
				pattern.value,
				pattern.valid,
				pattern.invalid,
			),
		))
	}

	fixtures = append(fixtures,
		corpusFixture(corpusCategoryStrings, "format-byte-valid", `
      type: string
      format: byte
      x-valid-examples: [aGVsbG8=]
`),
		corpusFixture(corpusCategoryStrings, "format-date-valid", `
      type: string
      format: date
      x-valid-examples: ["2026-07-11"]
`),
		corpusFixture(corpusCategoryStrings, "format-date-time-valid", `
      type: string
      format: date-time
      x-valid-examples: ["2026-07-11T12:34:56Z"]
`),
		corpusFixture(corpusCategoryStrings, "format-email-valid", `
      type: string
      format: email
      x-valid-examples: [a@example.com]
`),
		corpusFixture(corpusCategoryStrings, "combo-enum-minimum-length", `
      type: string
      minLength: 2
      enum: [ok, yes]
`),
		corpusFixture(corpusCategoryStrings, "combo-enum-maximum-length", `
      type: string
      maxLength: 3
      enum: [a, bee]
`),
		corpusFixture(corpusCategoryStrings, "combo-enum-unicode-length", `
      type: string
      minLength: 1
      maxLength: 1
      enum: [λ, β]
`),
		corpusFixture(corpusCategoryStrings, "combo-enum-empty-and-nonempty", `
      type: string
      minLength: 0
      maxLength: 1
      enum: ["", x]
`),
		corpusFixture(corpusCategoryStrings, "combo-enum-pattern", `
      type: string
      pattern: '^AB[0-9]$'
      x-valid-examples: [AB1]
      x-invalid-examples: [AB, ZZ1]
      enum: [AB1]
`),
		corpusFixture(corpusCategoryStrings, "combo-enum-email", `
      type: string
      format: email
      x-valid-examples: [a@example.com]
      enum: [a@example.com]
`),
		corpusFixture(corpusCategoryStrings, "combo-nullable-length", `
      type: string
      nullable: true
      minLength: 1
      maxLength: 3
`),
		corpusFixture(corpusCategoryStrings, "combo-nullable-enum", `
      type: string
      nullable: true
      enum: [null, x]
`),
		corpusFixture(corpusCategoryStrings, "combo-nullable-pattern", `
      type: string
      nullable: true
      pattern: '^N[0-9]$'
      x-valid-examples: [N1]
      x-invalid-examples: [N]
`),
		corpusFixture(corpusCategoryStrings, "combo-nullable-format", `
      type: string
      nullable: true
      format: date
      x-valid-examples: ["2026-07-11"]
`),
		corpusFixture(corpusCategoryStrings, "combo-pattern-length", `
      type: string
      minLength: 3
      maxLength: 3
      pattern: '^Q[0-9]{2}$'
      x-valid-examples: [Q12]
      x-invalid-examples: [Q1, R12]
`),
		corpusFixture(corpusCategoryStrings, "combo-byte-length", `
      type: string
      minLength: 4
      maxLength: 4
      format: byte
      x-valid-examples: [YWJj]
`),
		corpusFixture(corpusCategoryStrings, "combo-date-length", `
      type: string
      minLength: 10
      maxLength: 10
      format: date
      x-valid-examples: ["2026-07-11"]
`),
		corpusFixture(corpusCategoryStrings, "combo-date-time-length", `
      type: string
      minLength: 20
      maxLength: 20
      format: date-time
      x-valid-examples: ["2026-07-11T12:34:56Z"]
`),
		corpusFixture(corpusCategoryStrings, "combo-email-maximum-length", `
      type: string
      maxLength: 8
      format: email
      x-valid-examples: [a@b.co]
`),
		corpusFixture(corpusCategoryStrings, "combo-empty-pattern-length", `
      type: string
      minLength: 0
      maxLength: 0
      pattern: '^$'
      x-valid-examples: [""]
      x-invalid-examples: [x]
`),
		corpusFixture(corpusCategoryStrings, "combo-alternation-length", `
      type: string
      minLength: 3
      maxLength: 3
      pattern: '^(cat|dog)$'
      x-valid-examples: [cat]
      x-invalid-examples: [cow]
`),
		corpusFixture(corpusCategoryStrings, "combo-byte-enum", `
      type: string
      format: byte
      x-valid-examples: [YWJj]
      enum: [YWJj]
`),
	)

	return fixtures
}

// arrayCorpus covers collection bounds, item forms, nullable/enum items,
// nesting, and impossible item schemas in 36 rows.
func arrayCorpus() []validatorCorpusFixture {
	fixtures := []validatorCorpusFixture{
		corpusFixture(corpusCategoryArrays, "bounds-minimum-zero", "type: array\nminItems: 0\nitems: {type: integer}"),
		corpusFixture(corpusCategoryArrays, "bounds-minimum-one", "type: array\nminItems: 1\nitems: {type: integer}"),
		corpusFixture(corpusCategoryArrays, "bounds-minimum-three", "type: array\nminItems: 3\nitems: {type: integer}"),
		corpusFixture(corpusCategoryArrays, "bounds-maximum-zero", "type: array\nmaxItems: 0\nitems: {type: integer}"),
		corpusFixture(corpusCategoryArrays, "bounds-maximum-one", "type: array\nmaxItems: 1\nitems: {type: integer}"),
		corpusFixture(corpusCategoryArrays, "bounds-maximum-three", "type: array\nmaxItems: 3\nitems: {type: integer}"),
		corpusFixture(corpusCategoryArrays, "bounds-equal-zero", `
      type: array
      minItems: 0
      maxItems: 0
      items: {type: integer}
`),
		corpusFixture(corpusCategoryArrays, "bounds-equal-one", `
      type: array
      minItems: 1
      maxItems: 1
      items: {type: integer}
`),
		corpusFixture(corpusCategoryArrays, "bounds-equal-three", `
      type: array
      minItems: 3
      maxItems: 3
      items: {type: integer}
`),
		corpusFixture(corpusCategoryArrays, "bounds-range-one-three", `
      type: array
      minItems: 1
      maxItems: 3
      items: {type: integer}
`),
		corpusFixture(corpusCategoryArrays, "bounds-range-zero-three", `
      type: array
      minItems: 0
      maxItems: 3
      items: {type: integer}
`),
		corpusFixture(corpusCategoryArrays, "bounds-impossible-untyped", `
      nullable: true
      minItems: 2
      maxItems: 1
      items: {type: string}
`),
		corpusFixture(corpusCategoryArrays, "items-string-length", `
      type: array
      minItems: 1
      maxItems: 2
      items: {type: string, minLength: 1, maxLength: 3}
`),
		corpusFixture(corpusCategoryArrays, "items-integer-bounds", `
      type: array
      items: {type: integer, minimum: -2, maximum: 2}
`),
		corpusFixture(corpusCategoryArrays, "items-number-multiple", `
      type: array
      items: {type: number, minimum: -1, maximum: 1, multipleOf: 0.5}
`),
		corpusFixture(corpusCategoryArrays, "items-boolean-enum", `
      type: array
      items: {type: boolean, enum: [true]}
`),
		corpusFixture(corpusCategoryArrays, "items-object-required", `
      type: array
      items:
        type: object
        required: [id]
        properties:
          id: {type: integer, minimum: 1}
        additionalProperties: false
`),
		corpusFixture(corpusCategoryArrays, "items-array-nested", `
      type: array
      items:
        type: array
        minItems: 1
        maxItems: 2
        items: {type: string, minLength: 1}
`),
		corpusFixture(corpusCategoryArrays, "items-string-pattern", `
      type: array
      items:
        type: string
        pattern: '^X[0-9]$'
        x-valid-examples: [X1]
        x-invalid-examples: [X]
`),
		corpusFixture(corpusCategoryArrays, "items-object-properties", `
      type: array
      items:
        type: object
        minProperties: 1
        properties:
          enabled: {type: boolean}
        additionalProperties: false
`),
		corpusFixture(corpusCategoryArrays, "items-untyped-enum", `
      type: array
      items: {nullable: true, enum: [null, x, 1]}
`),
		corpusFixture(corpusCategoryArrays, "items-nullable-string", `
      type: array
      items: {type: string, nullable: true, minLength: 1}
`),
		corpusFixture(corpusCategoryArrays, "items-nullable-number", `
      type: array
      items: {type: number, nullable: true, minimum: 0}
`),
		corpusFixture(corpusCategoryArrays, "items-enum-string", `
      type: array
      items: {type: string, enum: [red, blue]}
`),
		corpusFixture(corpusCategoryArrays, "root-nullable", `
      type: array
      nullable: true
      maxItems: 2
      items: {type: string}
`),
		corpusFixture(corpusCategoryArrays, "root-enum", `
      type: array
      items: {type: string}
      enum: [[], [red]]
`),
		corpusFixture(corpusCategoryArrays, "items-nullable-enum", `
      type: array
      items: {type: string, nullable: true, enum: [null, x]}
`),
		corpusFixture(corpusCategoryArrays, "nested-array-integer", `
      type: array
      minItems: 1
      items:
        type: array
        items: {type: integer, minimum: 0}
`),
		corpusFixture(corpusCategoryArrays, "nested-array-array-boolean", `
      type: array
      items:
        type: array
        items:
          type: array
          maxItems: 1
          items: {type: boolean}
`),
		corpusFixture(corpusCategoryArrays, "nested-object-array", `
      type: array
      items:
        type: object
        required: [tags]
        properties:
          tags:
            type: array
            minItems: 1
            items: {type: string, minLength: 1}
        additionalProperties: false
`),
		corpusFixture(corpusCategoryArrays, "nested-array-object", `
      type: array
      items:
        type: array
        maxItems: 2
        items:
          type: object
          properties:
            value: {type: integer}
          additionalProperties: false
`),
		corpusFixture(corpusCategoryArrays, "empty-array-impossible-items", `
      type: array
      minItems: 0
      maxItems: 0
      items: {type: string, minLength: 3, maxLength: 1}
`),
		corpusFixture(corpusCategoryArrays, "combo-item-and-collection-bounds", `
      type: array
      minItems: 1
      maxItems: 3
      items: {type: integer, minimum: 1, maximum: 3}
`),
		corpusFixture(corpusCategoryArrays, "combo-object-item-and-collection-bounds", `
      type: array
      minItems: 1
      maxItems: 2
      items:
        type: object
        required: [code]
        properties:
          code: {type: string, minLength: 2}
        additionalProperties: false
`),
		corpusFixture(corpusCategoryArrays, "combo-enum-item-and-collection-bounds", `
      type: array
      minItems: 1
      maxItems: 2
      items: {type: string, enum: [a, b]}
`),
		corpusFixture(corpusCategoryArrays, "combo-nested-item-and-collection-bounds", `
      type: array
      minItems: 1
      maxItems: 2
      items:
        type: array
        minItems: 1
        maxItems: 1
        items: {type: number, multipleOf: 0.5}
`),
	}

	return fixtures
}

// objectCorpus covers object counts, request readOnly omission, all additionalProperties modes, and names in 52 rows.
func objectCorpus() []validatorCorpusFixture {
	fixtures := []validatorCorpusFixture{
		corpusFixture(corpusCategoryObjects, "bounds-minimum-zero", "type: object\nminProperties: 0"),
		corpusFixture(corpusCategoryObjects, "bounds-minimum-one", "type: object\nminProperties: 1"),
		corpusFixture(corpusCategoryObjects, "bounds-minimum-three", "type: object\nminProperties: 3"),
		corpusFixture(corpusCategoryObjects, "bounds-maximum-zero", "type: object\nmaxProperties: 0"),
		corpusFixture(corpusCategoryObjects, "bounds-maximum-one", "type: object\nmaxProperties: 1"),
		corpusFixture(corpusCategoryObjects, "bounds-maximum-three", "type: object\nmaxProperties: 3"),
		corpusFixture(corpusCategoryObjects, "bounds-equal-zero", "type: object\nminProperties: 0\nmaxProperties: 0"),
		corpusFixture(corpusCategoryObjects, "bounds-equal-one", "type: object\nminProperties: 1\nmaxProperties: 1"),
		corpusFixture(corpusCategoryObjects, "bounds-equal-three", "type: object\nminProperties: 3\nmaxProperties: 3"),
		corpusFixture(corpusCategoryObjects, "bounds-range-one-three", "type: object\nminProperties: 1\nmaxProperties: 3"),
		corpusFixture(corpusCategoryObjects, "bounds-range-zero-three", "type: object\nminProperties: 0\nmaxProperties: 3"),
		corpusFixture(corpusCategoryObjects, "bounds-impossible-untyped", `
      nullable: true
      minProperties: 2
      maxProperties: 1
`),
		corpusFixture(corpusCategoryObjects, "required-single", `
      type: object
      required: [name]
      properties:
        name: {type: string}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "required-two", `
      type: object
      required: [id, enabled]
      properties:
        id: {type: integer}
        enabled: {type: boolean}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "optional-single", `
      type: object
      properties:
        note: {type: string, maxLength: 4}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "required-and-optional", `
      type: object
      required: [id]
      properties:
        id: {type: integer, minimum: 1}
        note: {type: string}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "request-read-only-required-omitted", `
      type: object
      required: [id]
      maxProperties: 0
      properties:
        id: {type: string, readOnly: true}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "write-only-required", `
      type: object
      required: [secret]
      properties:
        secret: {type: string, writeOnly: true}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "required-implicit-property", `
      type: object
      required: [missing]
      additionalProperties: true
`),
		corpusFixture(corpusCategoryObjects, "required-blank-property-name", `
      type: object
      required: [blank]
      properties:
        blank: {type: string}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "required-enum-property", `
      type: object
      required: [state]
      properties:
        state: {type: string, enum: [new, old]}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "optional-nullable-property", `
      type: object
      properties:
        note: {type: string, nullable: true, maxLength: 3}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "additional-omitted", `
      type: object
      properties:
        known: {type: string}
`),
		corpusFixture(corpusCategoryObjects, "additional-true", `
      type: object
      properties:
        known: {type: boolean}
      additionalProperties: true
`),
		corpusFixture(corpusCategoryObjects, "additional-false", `
      type: object
      properties:
        known: {type: string}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "additional-schema-integer", `
      type: object
      additionalProperties: {type: integer, minimum: 0}
`),
		corpusFixture(corpusCategoryObjects, "additional-schema-string", `
      type: object
      additionalProperties: {type: string, minLength: 1}
`),
		corpusFixture(corpusCategoryObjects, "additional-schema-object", `
      type: object
      additionalProperties:
        type: object
        required: [value]
        properties:
          value: {type: boolean}
        additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "additional-schema-nullable", `
      type: object
      additionalProperties: {type: string, nullable: true, maxLength: 2}
`),
		corpusFixture(corpusCategoryObjects, "additional-true-with-required", `
      type: object
      required: [known]
      properties:
        known: {type: integer}
      additionalProperties: true
`),
		corpusFixture(corpusCategoryObjects, "additional-schema-with-declared", `
      type: object
      properties:
        fixed: {type: string}
      additionalProperties: {type: number, maximum: 10}
`),
		corpusFixture(corpusCategoryObjects, "additional-false-no-declared", `
      type: object
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "nested-required-object", `
      type: object
      required: [profile]
      properties:
        profile:
          type: object
          required: [name]
          properties:
            name: {type: string, minLength: 1}
          additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "nested-array-property", `
      type: object
      required: [tags]
      properties:
        tags:
          type: array
          minItems: 1
          items: {type: string, minLength: 1}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "nested-closed-object", `
      type: object
      properties:
        settings:
          type: object
          properties:
            enabled: {type: boolean}
          additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "nested-three-level", `
      type: object
      required: [outer]
      properties:
        outer:
          type: object
          required: [inner]
          properties:
            inner:
              type: object
              required: [value]
              properties:
                value: {type: integer, minimum: 0}
              additionalProperties: false
          additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "declared-and-additional-integer", `
      type: object
      required: [fixed]
      properties:
        fixed: {type: string}
      additionalProperties: {type: integer, minimum: 1}
`),
		corpusFixture(corpusCategoryObjects, "declared-and-additional-array", `
      type: object
      properties:
        fixed: {type: boolean}
      additionalProperties:
        type: array
        maxItems: 1
        items: {type: string}
`),
		corpusFixture(corpusCategoryObjects, "nested-enum-property", `
      type: object
      properties:
        payload:
          type: object
          required: [kind]
          properties:
            kind: {type: string, enum: [a, b]}
          additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "nested-nullable-property", `
      type: object
      properties:
        payload:
          type: object
          properties:
            note: {type: string, nullable: true}
          additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "empty-value-name", `
      type: object
      required: [empty]
      properties:
        empty: {type: string, maxLength: 0}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "escaped-slash-name", `
      type: object
      required: ["a/b"]
      properties:
        "a/b": {type: integer}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "escaped-tilde-name", `
      type: object
      required: ["t~n"]
      properties:
        "t~n": {type: boolean}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "escaped-combined-names", `
      type: object
      required: ["a/b", "t~n"]
      properties:
        "a/b": {type: string}
        "t~n": {type: string}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "escaped-space-name", `
      type: object
      required: ["space name"]
      properties:
        "space name": {type: string, minLength: 1}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "required-count-enum", `
      type: object
      required: [state]
      minProperties: 1
      maxProperties: 1
      properties:
        state: {type: string, enum: [open, closed]}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "required-count-enum-two-fields", `
      type: object
      required: [kind, level]
      minProperties: 2
      maxProperties: 2
      properties:
        kind: {type: string, enum: [a, b]}
        level: {type: integer, enum: [1, 2]}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "object-enum-required", `
      type: object
      required: [state]
      properties:
        state: {type: string, enum: [ready]}
      additionalProperties: false
      enum: [{state: ready}]
`),
		corpusFixture(corpusCategoryObjects, "object-enum-count", `
      type: object
      minProperties: 1
      maxProperties: 1
      properties:
        value: {type: string}
      additionalProperties: false
      enum: [{value: one}]
`),
		corpusFixture(corpusCategoryObjects, "required-nullable-enum", `
      type: object
      required: [value]
      properties:
        value: {type: string, nullable: true, enum: [null, x]}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "required-array-enum", `
      type: object
      required: [values]
      properties:
        values: {type: array, items: {type: string}, enum: [[one], [two]]}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryObjects, "required-additional-count", `
      type: object
      required: [fixed]
      minProperties: 2
      maxProperties: 2
      properties:
        fixed: {type: string, enum: [x]}
      additionalProperties: {type: integer, enum: [1]}
`),
	}

	return fixtures
}

// referenceAllOfCorpus covers local pointers and allOf intersections in 34 rows.
func referenceAllOfCorpus() []validatorCorpusFixture {
	return []validatorCorpusFixture{
		corpusFixtureWithComponents(corpusCategoryRefs, "direct-ref", `
      $ref: '#/components/schemas/Text'
`, `
components:
  schemas:
    Text:
      type: string
      minLength: 1
      maxLength: 4
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "chained-ref", `
      $ref: '#/components/schemas/A'
`, `
components:
  schemas:
    A: {$ref: '#/components/schemas/B'}
    B: {$ref: '#/components/schemas/C'}
    C: {type: integer, minimum: 1, maximum: 3}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "reused-ref-properties", `
      type: object
      required: [left, right]
      properties:
        left: {$ref: '#/components/schemas/Text'}
        right: {$ref: '#/components/schemas/Text'}
      additionalProperties: false
`, `
components:
  schemas:
    Text: {type: string, minLength: 2, maxLength: 3}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-in-property", `
      type: object
      required: [id]
      properties:
        id: {$ref: '#/components/schemas/Identifier'}
      additionalProperties: false
`, `
components:
  schemas:
    Identifier: {type: string, minLength: 2, maxLength: 5}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-in-items", `
      type: array
      minItems: 1
      items: {$ref: '#/components/schemas/Identifier'}
`, `
components:
  schemas:
    Identifier: {type: integer, minimum: 0, maximum: 3}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "refs-in-allof", `
      allOf:
        - {$ref: '#/components/schemas/Lower'}
        - {$ref: '#/components/schemas/Upper'}
`, `
components:
  schemas:
    Lower: {type: integer, minimum: 1}
    Upper: {maximum: 3}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "escaped-pointer-slash", `
      $ref: '#/components/schemas/Container/properties/a~1b'
`, `
components:
  schemas:
    Container:
      type: object
      properties:
        "a/b": {type: string, minLength: 1}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "escaped-pointer-tilde", `
      $ref: '#/components/schemas/Container/properties/t~0n'
`, `
components:
  schemas:
    Container:
      type: object
      properties:
        "t~n": {type: integer, minimum: 0}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-component-short-text", `
      $ref: '#/components/schemas/Text'
`, `
components:
  schemas:
    Text: {type: string, minLength: 1, maxLength: 3}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-component-unbounded-text", `
      $ref: '#/components/schemas/Text'
`, `
components:
  schemas:
    Text: {type: string, minLength: 1}
`),
		corpusFixture(corpusCategoryRefs, "allof-numeric-inclusive", `
      type: number
      allOf:
        - {minimum: -2}
        - {maximum: 2}
`),
		corpusFixture(corpusCategoryRefs, "allof-numeric-exclusive-multiple", `
      type: number
      allOf:
        - {minimum: -2, exclusiveMinimum: true}
        - {maximum: 2, exclusiveMaximum: true, multipleOf: 0.5}
`),
		corpusFixture(corpusCategoryRefs, "allof-string-length", `
      type: string
      allOf:
        - {minLength: 2}
        - {maxLength: 4}
`),
		corpusFixture(corpusCategoryRefs, "allof-string-pattern-length", `
      type: string
      x-valid-examples: [A1]
      allOf:
        - {pattern: '^A[0-9]$', x-valid-examples: [A1], x-invalid-examples: [B1]}
        - {minLength: 2, maxLength: 2}
`),
		corpusFixture(corpusCategoryRefs, "allof-object-required", `
      type: object
      properties:
        id: {type: integer, minimum: 2}
        score: {type: number, minimum: 0, maximum: 9}
      additionalProperties: false
      allOf:
        - type: object
          required: [id]
          properties:
            id: {type: integer, minimum: 2}
        - type: object
          required: [score]
          properties:
            score: {type: number, minimum: 0, maximum: 9}
`),
		corpusFixture(corpusCategoryRefs, "nested-allof-numeric", `
      type: integer
      allOf:
        - minimum: 0
          allOf:
            - {minimum: 1}
        - {maximum: 3}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-numeric-intersection", `
      allOf:
        - {$ref: '#/components/schemas/Minimum'}
        - {$ref: '#/components/schemas/Maximum'}
`, `
components:
  schemas:
    Minimum: {type: number, minimum: 0}
    Maximum: {maximum: 2}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-string-intersection", `
      allOf:
        - {$ref: '#/components/schemas/Short'}
        - {$ref: '#/components/schemas/LongEnough'}
`, `
components:
  schemas:
    Short: {type: string, maxLength: 4}
    LongEnough: {minLength: 2}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-object-intersection", `
      allOf:
        - {$ref: '#/components/schemas/Base'}
        - {$ref: '#/components/schemas/Extra'}
`, `
components:
  schemas:
    Base:
      type: object
      required: [id]
      properties:
        id: {type: integer}
    Extra:
      type: object
      required: [name]
      properties:
        name: {type: string}
`),
		corpusFixture(corpusCategoryRefs, "closed-object-interaction", `
      type: object
      required: [id, state]
      properties:
        id: {type: integer}
        state: {type: string, enum: [open, closed]}
      additionalProperties: false
      allOf:
        - type: object
          required: [id]
          properties:
            id: {type: integer}
        - type: object
          required: [state]
          properties:
            state: {type: string, enum: [open, closed]}
`),
		corpusFixture(corpusCategoryRefs, "allof-enum-and-bounds", `
      type: integer
      enum: [0, 1, 2, 3]
      allOf:
        - {minimum: 1}
        - {maximum: 2}
`),
		corpusFixture(corpusCategoryRefs, "allof-nullable-false", `
      type: string
      nullable: false
      allOf:
        - {type: string, nullable: false, minLength: 1}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "reused-ref-array-properties", `
      type: object
      required: [first, second]
      properties:
        first:
          type: array
          items: {$ref: '#/components/schemas/Tag'}
        second:
          type: array
          items: {$ref: '#/components/schemas/Tag'}
      additionalProperties: false
`, `
components:
  schemas:
    Tag: {type: string, enum: [a, b]}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-additional-properties", `
      type: object
      additionalProperties: {$ref: '#/components/schemas/Value'}
`, `
components:
  schemas:
    Value: {type: integer, minimum: 1}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-nested-property-pointer", `
      $ref: '#/components/schemas/Envelope/properties/payload/properties/code'
`, `
components:
  schemas:
    Envelope:
      type: object
      properties:
        payload:
          type: object
          properties:
            code: {type: string, minLength: 2}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "direct-enum-ref", `
      $ref: '#/components/schemas/State'
`, `
components:
  schemas:
    State: {type: string, enum: [draft, sent]}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "component-allof", `
      $ref: '#/components/schemas/Composed'
`, `
components:
  schemas:
    Composed:
      allOf:
        - {type: integer, minimum: 0}
        - {maximum: 4}
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "nested-ref-allof-object", `
      $ref: '#/components/schemas/Composed'
`, `
components:
  schemas:
    Base:
      type: object
      required: [id]
      properties:
        id: {type: integer}
    Composed:
      allOf:
        - {$ref: '#/components/schemas/Base'}
        - type: object
          required: [name]
          properties:
            name: {type: string}
`),
		corpusFixture(corpusCategoryRefs, "inline-allof-object-child", `
      type: object
      required: [value]
      properties:
        value:
          allOf:
            - {type: integer, minimum: 1}
            - {maximum: 3}
      additionalProperties: false
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "direct-object-ref", `
      $ref: '#/components/schemas/Record'
`, `
components:
  schemas:
    Record:
      type: object
      required: [name]
      properties:
        name: {type: string, minLength: 1}
      additionalProperties: false
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "chained-object-ref", `
      $ref: '#/components/schemas/Outer'
`, `
components:
  schemas:
    Outer: {$ref: '#/components/schemas/Middle'}
    Middle: {$ref: '#/components/schemas/Inner'}
    Inner:
      type: object
      properties:
        active: {type: boolean}
      additionalProperties: false
`),
		corpusFixtureWithComponents(corpusCategoryRefs, "ref-item-allof", `
      type: array
      minItems: 1
      items: {$ref: '#/components/schemas/Bounded'}
`, `
components:
  schemas:
    Bounded:
      allOf:
        - {type: integer, minimum: 1}
        - {maximum: 3}
`),
		corpusFixture(corpusCategoryRefs, "allof-array-constraints", `
      type: array
      items: {type: string}
      allOf:
        - {minItems: 1}
        - {maxItems: 2}
`),
		corpusFixture(corpusCategoryRefs, "closed-object-allof-shared-properties", `
      type: object
      properties:
        id: {type: integer, minimum: 1}
        code: {type: string, minLength: 2}
      additionalProperties: false
      allOf:
        - type: object
          required: [id]
          properties:
            id: {type: integer, minimum: 1}
        - type: object
          required: [code]
          properties:
            code: {type: string, minLength: 2}
`),
	}
}

// crossFamilyCorpus contains 13 compatible existing fixtures, two typeless replacements, and 23 realistic models.
// The two original typeless fixtures are fixed characterizations because kin-openapi rejects their valid null case.
func crossFamilyCorpus() []validatorCorpusFixture {
	fixtures := existingCrossFamilyCorpus()

	return append(fixtures, crossFamilyModelCorpus()...)
}

// existingCrossFamilyCorpus retains the compatible pre-existing integration fixtures in the shared corpus.
func existingCrossFamilyCorpus() []validatorCorpusFixture {
	return []validatorCorpusFixture{
		corpusFixture(corpusCategoryCross, "existing-realistic-nested-order", `
      type: object
      required: [customer, lines, state]
      properties:
        customer:
          type: object
          required: [id, contact]
          properties:
            id: {type: integer, minimum: 1}
            contact: {type: string, format: email, x-valid-examples: [buyer@example.com]}
            note: {type: string, nullable: true, maxLength: 12}
          additionalProperties: false
        lines:
          type: array
          minItems: 1
          maxItems: 3
          items:
            type: object
            required: [sku, quantity]
            properties:
              sku: {type: string, minLength: 1, maxLength: 8}
              quantity: {type: integer, minimum: 1, maximum: 20}
            additionalProperties: false
        state: {type: string, enum: [draft, submitted]}
        tags:
          type: array
          maxItems: 2
          items: {type: string, minLength: 1}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "existing-array-item-constraints", `
      type: array
      minItems: 1
      maxItems: 3
      items:
        type: integer
        minimum: -2
        maximum: 2
`),
		corpusFixture(corpusCategoryCross, "existing-additional-properties-true", `
      type: object
      required: [known]
      properties:
        known: {type: boolean}
      additionalProperties: true
`),
		corpusFixture(corpusCategoryCross, "existing-additional-properties-schema", `
      type: object
      properties:
        fixed: {type: string}
      additionalProperties:
        type: integer
        minimum: 0
`),
		corpusFixtureWithComponents(corpusCategoryCross, "existing-local-references", `
      $ref: '#/components/schemas/Envelope'
`, `
components:
  schemas:
    Identifier:
      type: string
      minLength: 2
      maxLength: 6
    Envelope:
      type: object
      required: [id, payload]
      properties:
        id: {$ref: '#/components/schemas/Identifier'}
        payload:
          type: array
          minItems: 1
          items: {$ref: '#/components/schemas/Identifier'}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "existing-allof-object-intersection", `
      type: object
      properties:
        id: {type: integer, minimum: 1}
        score: {type: number, minimum: 0, maximum: 10}
      additionalProperties: false
      allOf:
        - type: object
          required: [id]
          properties:
            id: {type: integer, minimum: 1}
        - type: object
          required: [score]
          properties:
            score: {type: number, minimum: 0, maximum: 10}
`),
		corpusFixture(corpusCategoryCross, "existing-allof-number-intersection", `
      allOf:
        - {type: number, minimum: -2, exclusiveMinimum: true}
        - {maximum: 2, exclusiveMaximum: true}
`),
		corpusFixture(corpusCategoryCross, "existing-trusted-strings-unicode", `
      type: object
      required: [code, email]
      properties:
        code:
          type: string
          minLength: 2
          maxLength: 3
          pattern: '^λ[0-9]$'
          x-valid-examples: [λ7]
          x-invalid-examples: [λ, xx]
        email:
          type: string
          format: email
          x-valid-examples: [a@example.com]
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "typeless-keyword-families-in-properties", `
      type: object
      required: [text, count, items, metadata]
      properties:
        text: {type: string, minLength: 1, maxLength: 2}
        count: {type: number, minimum: -1, maximum: 1}
        items: {type: array, minItems: 1, maxItems: 2, items: {type: boolean}}
        metadata: {type: object, minProperties: 1, maxProperties: 2}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "zero-collection-bounds-in-properties", `
      type: object
      required: [text, items, metadata]
      properties:
        text: {type: string, maxLength: 0}
        items: {type: array, maxItems: 0, items: {}}
        metadata: {type: object, maxProperties: 0}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "existing-decimal-multiple-of", `
      type: number
      minimum: -2
      maximum: 2
      multipleOf: 0.25
`),
		corpusFixture(corpusCategoryCross, "existing-null-only", `
      type: string
      nullable: true
      enum: [null]
`),
		corpusFixture(corpusCategoryCross, "existing-optional-impossible-property", `
      type: object
      properties:
        impossible: {type: string, minLength: 2, maxLength: 1}
        live: {type: boolean}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "existing-empty-array-impossible-items", `
      type: array
      minItems: 0
      maxItems: 0
      items: {type: string, minLength: 2, maxLength: 1}
`),
		corpusFixture(corpusCategoryCross, "existing-adversarial-property-names", `
      type: object
      required: ['a/b', 't~n', empty]
      minProperties: 3
      maxProperties: 3
      properties:
        'a/b': {type: string, minLength: 0, maxLength: 0}
        't~n': {type: array, minItems: 0, maxItems: 0, items: {}}
        empty: {type: object, minProperties: 0, maxProperties: 0, additionalProperties: false}
      additionalProperties: false
`),
	}
}

// crossFamilyModelCorpus adds the remaining realistic and adversarial cross-family schemas.
func crossFamilyModelCorpus() []validatorCorpusFixture {
	return []validatorCorpusFixture{
		corpusFixtureWithComponents(corpusCategoryCross, "customer-profile", `
      type: object
      required: [customer, preferences]
      properties:
        customer: {$ref: '#/components/schemas/Customer'}
        preferences:
          type: array
          minItems: 1
          maxItems: 2
          items: {type: string, enum: [email, sms]}
      additionalProperties: false
`, `
components:
  schemas:
    Customer:
      type: object
      required: [id, name]
      properties:
        id: {type: integer, minimum: 1}
        name: {type: string, minLength: 1, maxLength: 12}
      additionalProperties: false
`),
		corpusFixtureWithComponents(corpusCategoryCross, "invoice-with-money-ref", `
      type: object
      required: [lines, total]
      properties:
        lines:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [sku, amount]
            properties:
              sku: {type: string, pattern: '^[A-Z][0-9]$', x-valid-examples: [A1], x-invalid-examples: [a1]}
              amount: {$ref: '#/components/schemas/Money'}
            additionalProperties: false
        total: {$ref: '#/components/schemas/Money'}
      additionalProperties: false
`, `
components:
  schemas:
    Money:
      type: number
      minimum: 0
      maximum: 100
      multipleOf: 0.25
`),
		corpusFixture(corpusCategoryCross, "shipment-status", `
      type: object
      required: [address, parcels, status]
      properties:
        address:
          type: object
          required: [country]
          properties:
            country: {type: string, enum: [US, CA]}
            note: {type: string, nullable: true, maxLength: 8}
          additionalProperties: false
        parcels:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [weight]
            properties:
              weight: {type: number, minimum: 0.25, maximum: 20, multipleOf: 0.25}
            additionalProperties: false
        status: {type: string, enum: [queued, shipped]}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "catalog-search-filter", `
      type: object
      required: [query]
      properties:
        query: {type: string, minLength: 1, maxLength: 10}
        filters:
          type: array
          maxItems: 2
          items:
            type: object
            required: [field, value]
            properties:
              field: {type: string, enum: [brand, color]}
              value: {type: string, minLength: 1}
            additionalProperties: false
        page: {type: integer, minimum: 0, maximum: 5}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "feature-flags", `
      type: object
      required: [environment]
      properties:
        environment: {type: string, enum: [dev, prod]}
        flags:
          type: object
          additionalProperties: {type: boolean}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "audit-event", `
      type: object
      required: [action, actor, metadata]
      properties:
        action: {type: string, pattern: '^[a-z]+$', x-valid-examples: [create], x-invalid-examples: [CREATE]}
        actor:
          type: object
          required: [id]
          properties:
            id: {type: integer, minimum: 1}
          additionalProperties: false
        metadata:
          type: array
          maxItems: 2
          items:
            type: object
            required: [key, value]
            properties:
              key: {type: string, minLength: 1}
              value: {type: string, nullable: true}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "quiz-answers", `
      type: object
      required: [answers]
      properties:
        answers:
          type: array
          minItems: 1
          maxItems: 3
          items:
            type: object
            required: [question, choice]
            properties:
              question: {type: integer, minimum: 1, maximum: 5}
              choice: {type: string, enum: [a, b, c]}
            additionalProperties: false
        submitted: {type: boolean}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "article-document", `
      type: object
      required: [title, sections]
      properties:
        title: {type: string, minLength: 1, maxLength: 20}
        sections:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [heading, paragraphs]
            properties:
              heading: {type: string, minLength: 1}
              paragraphs:
                type: array
                minItems: 1
                maxItems: 2
                items: {type: string, minLength: 1, maxLength: 20}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "metric-batch", `
      type: object
      required: [series]
      properties:
        series:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [name, points]
            properties:
              name: {type: string, enum: [cpu, memory]}
              points:
                type: array
                minItems: 1
                maxItems: 2
                items: {type: number, minimum: 0, maximum: 100, multipleOf: 0.5}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixtureWithComponents(corpusCategoryCross, "entitlement-composition", `
      type: object
      required: [user, grant]
      properties:
        user: {type: string, minLength: 1}
        grant: {$ref: '#/components/schemas/Grant'}
      additionalProperties: false
`, `
components:
  schemas:
    Grant:
      allOf:
        - type: object
          required: [role]
          properties:
            role: {type: string, enum: [reader, writer]}
        - type: object
          required: [level]
          properties:
            level: {type: integer, minimum: 1, maximum: 3}
`),
		corpusFixture(corpusCategoryCross, "account-settings", `
      type: object
      required: [account]
      properties:
        account:
          type: object
          required: [id, settings]
          properties:
            id: {type: integer, minimum: 1}
            settings:
              type: object
              properties:
                theme: {type: string, enum: [light, dark]}
                alerts: {type: boolean}
              additionalProperties: false
          additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "workflow-steps", `
      type: object
      required: [steps]
      properties:
        steps:
          type: array
          minItems: 1
          maxItems: 3
          items:
            type: object
            required: [id, state]
            properties:
              id: {type: string, pattern: '^S[0-9]$', x-valid-examples: [S1], x-invalid-examples: [s1]}
              state: {type: string, enum: [pending, done]}
              retries: {type: integer, nullable: true, minimum: 0, maximum: 2}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "subscription-request", `
      type: object
      required: [plan, contacts]
      properties:
        plan: {type: string, enum: [free, pro]}
        contacts:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [email]
            properties:
              email: {type: string, format: email, x-valid-examples: [a@example.com]}
              primary: {type: boolean}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "project-board", `
      type: object
      required: [columns]
      properties:
        columns:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [name, cards]
            properties:
              name: {type: string, enum: [todo, done]}
              cards:
                type: array
                maxItems: 2
                items:
                  type: object
                  required: [title]
                  properties:
                    title: {type: string, minLength: 1, maxLength: 12}
                  additionalProperties: false
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "notification-preferences", `
      type: object
      required: [channels]
      properties:
        channels:
          type: object
          additionalProperties:
            type: object
            required: [enabled]
            properties:
              enabled: {type: boolean}
              frequency: {type: string, enum: [daily, weekly]}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "health-report", `
      type: object
      required: [service, checks]
      properties:
        service: {type: string, minLength: 1}
        checks:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [name, latency]
            properties:
              name: {type: string, enum: [db, cache]}
              latency: {type: number, minimum: 0, maximum: 10, multipleOf: 0.5}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "delivery-route", `
      type: object
      required: [stops]
      properties:
        stops:
          type: array
          minItems: 1
          maxItems: 3
          items:
            type: object
            required: [sequence, address]
            properties:
              sequence: {type: integer, minimum: 1, maximum: 3}
              address:
                type: object
                required: [line]
                properties:
                  line: {type: string, minLength: 1}
                  note: {type: string, nullable: true, maxLength: 8}
                additionalProperties: false
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixtureWithComponents(corpusCategoryCross, "media-library", `
      type: object
      required: [items]
      properties:
        items:
          type: array
          minItems: 1
          maxItems: 2
          items: {$ref: '#/components/schemas/Media'}
      additionalProperties: false
`, `
components:
  schemas:
    Media:
      type: object
      required: [name, size]
      properties:
        name: {type: string, minLength: 1}
        size: {type: integer, minimum: 0, maximum: 1000}
        kind: {type: string, enum: [image, video]}
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "commerce-checkout", `
      type: object
      required: [items, payment]
      properties:
        items:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [sku, quantity]
            properties:
              sku: {type: string, pattern: '^[A-Z][0-9]$', x-valid-examples: [A1], x-invalid-examples: [AA]}
              quantity: {type: integer, minimum: 1, maximum: 5}
            additionalProperties: false
        payment:
          type: object
          required: [method]
          properties:
            method: {type: string, enum: [card, cash]}
            amount: {type: number, minimum: 0, multipleOf: 0.25}
          additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "support-ticket", `
      type: object
      required: [subject, messages]
      properties:
        subject: {type: string, minLength: 1, maxLength: 20}
        priority: {type: string, enum: [low, high]}
        messages:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [body]
            properties:
              body: {type: string, minLength: 1, maxLength: 30}
              attachments:
                type: array
                maxItems: 1
                items: {type: string, format: byte, x-valid-examples: [YWJj]}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "access-policy", `
      type: object
      required: [rules]
      properties:
        rules:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [resource, actions]
            properties:
              resource: {type: string, enum: [project, report]}
              actions:
                type: array
                minItems: 1
                maxItems: 2
                items: {type: string, enum: [read, write]}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "sensor-snapshot", `
      type: object
      required: [device, readings]
      properties:
        device:
          type: object
          required: [id]
          properties:
            id: {type: string, pattern: '^D[0-9]$', x-valid-examples: [D1], x-invalid-examples: [d1]}
          additionalProperties: false
        readings:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [name, value]
            properties:
              name: {type: string, enum: [temp, humidity]}
              value: {type: number, minimum: -10, maximum: 100, multipleOf: 0.5}
            additionalProperties: false
      additionalProperties: false
`),
		corpusFixture(corpusCategoryCross, "partner-import", `
      type: object
      required: [source, records]
      properties:
        source: {type: string, enum: [crm, erp]}
        records:
          type: array
          minItems: 1
          maxItems: 2
          items:
            type: object
            required: [externalId, fields]
            properties:
              externalId: {type: string, minLength: 1, maxLength: 8}
              fields:
                type: object
                additionalProperties: {type: string, nullable: true, maxLength: 10}
            additionalProperties: false
      additionalProperties: false
`),
	}
}
