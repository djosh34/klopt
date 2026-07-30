//nolint:cyclop,godoclint // Behavior tests keep verdict classification in one scenario.
package program_test

import (
	"encoding/binary"
	"errors"
	"strconv"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // External-package tests exercise the public deep seam.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestDecodeAcceptsEveryTapeAndHonorsSignedRoot(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "string"},
		StringValidation: validation.StringValidation{
			MinLength: countBound(t, "1"),
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	limits := program.Limits{
		MaxSteps: 10_000, MaxOutputBytes: 1_000, MaxDepth: 16,
		MaxSolverWork: 10_000, MaxSolverBytes: 1_000_000,
	}

	valid, err := compiled.Decode(nil, limits)
	require.NoError(t, err)
	require.Equal(t, program.ExpectValid, valid.Expect)
	requireVerdict(t, root, valid.Value, true)

	invalidTape := make([]byte, 9)
	invalidTape[8] = 1
	invalid, err := compiled.Decode(invalidTape, limits)
	require.NoError(t, err)
	require.Equal(t, program.ExpectInvalid, invalid.Expect)
	requireVerdict(t, root, invalid.Value, false)

	replayed, err := compiled.Decode(invalidTape, limits)
	require.NoError(t, err)
	require.Equal(t, invalid, replayed)
}

func TestDecodeWalksTheCompleteSignedUnicodeLanguage(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "string"},
		StringValidation: validation.StringValidation{
			MinLength:       countBound(t, "1"),
			MaxLength:       countBound(t, "3"),
			Pattern:         "^a+$",
			CompiledPattern: mustPattern(t, "^a+$"),
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	limits := generousLimits()
	reached := make(map[string]bool)

	for first := uint64(0); first < 4; first++ {
		for second := uint64(0); second < 4; second++ {
			input := words(0, 0, 0, 0, 0, first, 0, second, 0)
			sample, decodeErr := compiled.Decode(input, limits)
			require.NoError(t, decodeErr)
			require.Equal(t, program.ExpectValid, sample.Expect)
			requireVerdict(t, root, sample.Value, true)
			reached[sample.Value.String] = true
		}
	}

	require.True(t, reached["a"])
	require.True(t, reached["aa"])
	require.True(t, reached["aaa"])

	for choice := uint64(0); choice < 32; choice++ {
		input := words(0, 1, choice, choice, choice, choice)
		sample, decodeErr := compiled.Decode(input, limits)
		require.NoError(t, decodeErr)
		require.Equal(t, program.ExpectInvalid, sample.Expect)
		requireVerdict(t, root, sample.Value, false)
	}
}

func TestDecodeUnranksEveryExactNumberOnABoundedLattice(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "number"},
		NumberValidation: validation.NumberValidation{
			Minimum:         numberBound(t, "0", false),
			Maximum:         numberBound(t, "1", false),
			MultipleOf:      "0.25",
			ExactMultipleOf: new(jsonvalue.Number(mustNumber(t, "0.25"))),
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	reached := make(map[string]bool)

	for rank := uint64(0); rank < 64; rank++ {
		sample, decodeErr := compiled.Decode(words(0, 0, 0, rank), generousLimits())
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, true)
		reached[sample.Value.Number.Lexeme] = true
	}

	for _, value := range []string{"0", "0.25", "0.5", "0.75", "1"} {
		require.Truef(t, reached[value], "number %s was unreachable", value)
	}

	for rank := uint64(0); rank < 32; rank++ {
		sample, decodeErr := compiled.Decode(words(0, 1, rank, rank, rank), generousLimits())
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, false)
	}
}

func TestDecodeZeroTailCompletesSignedNumberFormats(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"int32", "int64", "float", "double"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			root := &validation.Validation{
				KindValidation:   validation.KindValidation{Type: "number"},
				NumberValidation: validation.NumberValidation{Format: format},
			}
			compiled, err := program.Compile([]*validation.Validation{root})
			require.NoError(t, err)

			valid, err := compiled.Decode(nil, generousLimits())
			require.NoError(t, err)
			requireVerdict(t, root, valid.Value, true)

			invalid, err := compiled.Decode(words(0, 1), generousLimits())
			require.NoError(t, err)
			require.Equal(t, program.ExpectInvalid, invalid.Expect)
			requireVerdict(t, root, invalid.Value, false)
		})
	}
}

func TestDecodeSelectsEveryHeterogeneousEnumMemberWithoutPrecomputedCases(t *testing.T) {
	t.Parallel()

	object, err := jsonvalue.Object(nil)
	require.NoError(t, err)
	root := &validation.Validation{
		EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{
			jsonvalue.Null(),
			jsonvalue.Bool(true),
			{Kind: jsonvalue.KindNumber, Number: mustNumber(t, "1")},
			jsonvalue.String("λ"),
			jsonvalue.Array(nil),
			object,
		}},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	reached := make(map[string]bool)

	for rank := uint64(0); rank < 12; rank++ {
		sample, decodeErr := compiled.Decode(words(0, 0, rank), generousLimits())
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, true)
		body, marshalErr := sample.Value.MarshalJSON()
		require.NoError(t, marshalErr)

		reached[string(body)] = true
	}

	for _, value := range []string{`null`, `true`, `1`, `"λ"`, `[]`, `{}`} {
		require.Truef(t, reached[value], "enum member %s was unreachable", value)
	}

	for rank := uint64(0); rank < 16; rank++ {
		sample, decodeErr := compiled.Decode(words(0, 1, rank, rank, rank), generousLimits())
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, false)
	}
}

func TestDecodeComplementsExactContainerEnums(t *testing.T) {
	t.Parallel()

	boolean := &validation.Validation{KindValidation: validation.KindValidation{Type: "boolean"}}
	emptyObject, err := jsonvalue.Object(nil)
	require.NoError(t, err)
	falseObject, err := jsonvalue.Object([]jsonvalue.Member{{
		Name: "a", Value: jsonvalue.Bool(false),
	}})
	require.NoError(t, err)
	reorderedObject, err := jsonvalue.Object([]jsonvalue.Member{
		{Name: "aa", Value: jsonvalue.Bool(false)},
		{Name: "b", Value: jsonvalue.Bool(false)},
	})
	require.NoError(t, err)

	tests := []struct {
		name string
		root *validation.Validation
		kind jsonvalue.Kind
	}{
		{
			name: "empty array",
			root: &validation.Validation{
				KindValidation: validation.KindValidation{Type: "array"},
				EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{
					jsonvalue.Array(nil),
				}},
				ArrayValidation: validation.ArrayValidation{Items: boolean},
			},
			kind: jsonvalue.KindArray,
		},
		{
			name: "fixed array value",
			root: &validation.Validation{
				KindValidation: validation.KindValidation{Type: "array"},
				EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{
					jsonvalue.Array([]jsonvalue.Value{jsonvalue.Bool(false)}),
				}},
				ArrayValidation: validation.ArrayValidation{
					MinItems: countBound(t, "1"), MaxItems: countBound(t, "1"), Items: boolean,
				},
			},
			kind: jsonvalue.KindArray,
		},
		{
			name: "empty object",
			root: &validation.Validation{
				KindValidation: validation.KindValidation{Type: "object"},
				EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{
					emptyObject,
				}},
				ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true},
			},
			kind: jsonvalue.KindObject,
		},
		{
			name: "fixed object value",
			root: &validation.Validation{
				KindValidation: validation.KindValidation{Type: "object"},
				EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{
					falseObject,
				}},
				ObjectValidation: validation.ObjectValidation{
					MinProperties: countBound(t, "1"), MaxProperties: countBound(t, "1"),
					Required:                    []string{"a"},
					Properties:                  []validation.PropertyValidation{{Name: "a", Validation: boolean}},
					AdditionalPropertiesAllowed: false,
				},
			},
			kind: jsonvalue.KindObject,
		},
		{
			name: "object member order is semantic",
			root: &validation.Validation{
				KindValidation: validation.KindValidation{Type: "object"},
				EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{
					reorderedObject,
				}},
				ObjectValidation: validation.ObjectValidation{
					MinProperties: countBound(t, "2"), MaxProperties: countBound(t, "2"),
					Required: []string{"aa", "b"},
					Properties: []validation.PropertyValidation{
						{Name: "aa", Validation: boolean},
						{Name: "b", Validation: boolean},
					},
					AdditionalPropertiesAllowed: false,
				},
			},
			kind: jsonvalue.KindObject,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiled, compileErr := program.Compile([]*validation.Validation{test.root})
			require.NoError(t, compileErr)

			sample, decodeErr := compiled.Decode(words(0, 1), generousLimits())
			require.NoError(t, decodeErr)
			require.Equal(t, program.ExpectInvalid, sample.Expect)
			require.Equal(t, test.kind, sample.Value.Kind)
			requireVerdict(t, test.root, sample.Value, false)
		})
	}
}

func TestDecodePropagatesAndOrTruthWithoutEnumeratingAssignments(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "string"},
		AnyOfValidations: []*validation.Validation{
			{StringValidation: validation.StringValidation{MinLength: countBound(t, "2")}},
			{StringValidation: validation.StringValidation{MaxLength: countBound(t, "0")}},
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	for verdict := uint64(0); verdict < 2; verdict++ {
		for choice := uint64(0); choice < 64; choice++ {
			sample, decodeErr := compiled.Decode(
				words(0, verdict, choice, choice, choice, choice, choice),
				generousLimits(),
			)
			require.NoError(t, decodeErr)
			requireVerdict(t, root, sample.Value, verdict == 0)
		}
	}
}

func TestDecodeSelectsNearStructuralWrongKindAndMultiFaultsFromTape(t *testing.T) {
	t.Parallel()

	name := &validation.Validation{
		KindValidation:   validation.KindValidation{Type: "string"},
		StringValidation: validation.StringValidation{MinLength: countBound(t, "1")},
	}
	age := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "integer"},
		NumberValidation: validation.NumberValidation{
			Minimum: numberBound(t, "18", false),
			Maximum: numberBound(t, "120", false),
		},
	}
	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			Required: []string{"name", "age"},
			Properties: []validation.PropertyValidation{
				{Name: "name", Validation: name},
				{Name: "age", Validation: age},
			},
			AdditionalPropertiesAllowed: false,
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	var nearBoundary, oneMissing, wrongKind, severalFaults bool

	for seed := uint64(0); seed < 1_000; seed++ {
		sample, decodeErr := compiled.Decode(invalidPseudoTape(seed, 24), generousLimits())
		if decodeErr != nil {
			var (
				limit    *program.LimitError
				resource *program.ResourceError
			)
			if errors.As(decodeErr, &limit) || errors.As(decodeErr, &resource) {
				continue
			}

			require.NoError(t, decodeErr)
		}

		require.Equal(t, program.ExpectInvalid, sample.Expect)
		requireVerdict(t, root, sample.Value, false)

		if sample.Value.Kind != jsonvalue.KindObject {
			wrongKind = true
		} else {
			nameValue, hasName := testObjectMember(sample.Value, "name")
			ageValue, hasAge := testObjectMember(sample.Value, "age")

			if hasName != hasAge {
				oneMissing = true
			}

			if !hasName && !hasAge {
				severalFaults = true
			}

			if hasName && hasAge {
				nameValid := len(name.Validate(mustMarshalValue(t, nameValue))) == 0
				ageValid := len(age.Validate(mustMarshalValue(t, ageValue))) == 0
				nearName := ageValid && nameValue.Kind == jsonvalue.KindString && nameValue.String == ""
				nearAge := nameValid && ageValue.Kind == jsonvalue.KindNumber &&
					(ageValue.Number.Lexeme == "17" || ageValue.Number.Lexeme == "121")
				nearBoundary = nearBoundary || nearName || nearAge
			}
		}

		if nearBoundary && oneMissing && wrongKind && severalFaults {
			break
		}
	}

	require.True(t, nearBoundary, "no near numeric boundary fault was reachable")
	require.True(t, oneMissing, "no one-required-property fault was reachable")
	require.True(t, wrongKind, "no wrong-kind fault was reachable")
	require.True(t, severalFaults, "no larger fault budget was reachable")
}

func TestDecodeFailedAnyOfMakesEveryChildFalse(t *testing.T) {
	t.Parallel()

	text := &validation.Validation{
		KindValidation:   validation.KindValidation{Type: "string"},
		StringValidation: validation.StringValidation{MinLength: countBound(t, "1")},
	}
	number := &validation.Validation{
		KindValidation:   validation.KindValidation{Type: "number"},
		NumberValidation: validation.NumberValidation{Minimum: numberBound(t, "0", false)},
	}
	root := &validation.Validation{AnyOfValidations: []*validation.Validation{text, number}}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	for seed := uint64(0); seed < 16; seed++ {
		sample, decodeErr := compiled.Decode(invalidPseudoTape(seed, 12), generousLimits())
		if decodeErr != nil {
			var (
				limit    *program.LimitError
				resource *program.ResourceError
			)
			if errors.As(decodeErr, &limit) || errors.As(decodeErr, &resource) {
				continue
			}

			require.NoError(t, decodeErr)
		}

		body := mustMarshalValue(t, sample.Value)
		require.NotEmpty(t, text.Validate(body))
		require.NotEmpty(t, number.Validate(body))
	}
}

func TestDecodeBuildsArraysIncrementallyAndFaultsOneItem(t *testing.T) {
	t.Parallel()

	item := &validation.Validation{
		KindValidation:   validation.KindValidation{Type: "string"},
		StringValidation: validation.StringValidation{MinLength: countBound(t, "1")},
	}
	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "array"},
		ArrayValidation: validation.ArrayValidation{
			MinItems: countBound(t, "1"),
			MaxItems: countBound(t, "2"),
			Items:    item,
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	reachedLengths := make(map[int]bool)

	for choice := uint64(0); choice < 64; choice++ {
		sample, decodeErr := compiled.Decode(
			words(0, 0, 0, choice, choice, choice, choice, choice, choice),
			generousLimits(),
		)
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, true)
		reachedLengths[len(sample.Value.Array)] = true
	}

	require.True(t, reachedLengths[1])
	require.True(t, reachedLengths[2])

	for choice := uint64(0); choice < 64; choice++ {
		sample, decodeErr := compiled.Decode(
			words(0, 1, choice, choice, choice, choice, choice, choice),
			generousLimits(),
		)
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, false)
	}
}

func TestDecodeReachesEveryBoundedArrayValue(t *testing.T) {
	t.Parallel()

	item := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "boolean"},
		EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{
			jsonvalue.Bool(false), jsonvalue.Bool(true),
		}},
	}
	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "array"},
		ArrayValidation: validation.ArrayValidation{
			MaxItems: countBound(t, "2"), Items: item,
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	reached := make(map[string]bool)

	for seed := uint64(0); seed < 10_000 && len(reached) < 7; seed++ {
		sample, decodeErr := compiled.Decode(validPseudoTape(seed, 12), generousLimits())
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, sample.Expect == program.ExpectValid)

		if sample.Expect != program.ExpectValid {
			continue
		}

		body, marshalErr := sample.Value.MarshalJSON()
		require.NoError(t, marshalErr)

		reached[string(body)] = true
	}

	require.Equal(t, map[string]bool{
		`[]`:            true,
		`[false]`:       true,
		`[true]`:        true,
		`[false,false]`: true,
		`[false,true]`:  true,
		`[true,false]`:  true,
		`[true,true]`:   true,
	}, reached)
}

func TestDecodeBuildsCanonicalObjectsAndPreservesSiblingsAroundFaults(t *testing.T) {
	t.Parallel()

	name := &validation.Validation{
		KindValidation:   validation.KindValidation{Type: "string"},
		StringValidation: validation.StringValidation{MinLength: countBound(t, "1")},
	}
	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			MinProperties:               countBound(t, "1"),
			MaxProperties:               countBound(t, "2"),
			Required:                    []string{"name"},
			Properties:                  []validation.PropertyValidation{{Name: "name", Validation: name}},
			AdditionalPropertiesAllowed: false,
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	for choice := uint64(0); choice < 64; choice++ {
		sample, decodeErr := compiled.Decode(
			words(0, 0, 0, choice, choice, choice, choice, choice, choice),
			generousLimits(),
		)
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, true)
		require.Len(t, sample.Value.Object, 1)
		require.Equal(t, "name", sample.Value.Object[0].Name)
	}

	for choice := uint64(0); choice < 64; choice++ {
		sample, decodeErr := compiled.Decode(
			words(0, 1, choice, choice, choice, choice, choice, choice, choice),
			generousLimits(),
		)
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, false)
	}
}

func TestDecodeReachesEverySmallBoundedObject(t *testing.T) {
	t.Parallel()

	boolean := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "boolean"},
		EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{
			jsonvalue.Bool(false), jsonvalue.Bool(true),
		}},
	}
	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			MaxProperties:               countBound(t, "2"),
			AdditionalPropertiesAllowed: false,
			Properties: []validation.PropertyValidation{
				{Name: "a", Validation: boolean},
				{Name: "b", Validation: boolean},
			},
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	reached := make(map[string]bool)

	for seed := uint64(0); seed < 20_000 && len(reached) < 9; seed++ {
		sample, decodeErr := compiled.Decode(validPseudoTape(seed, 16), generousLimits())
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, sample.Expect == program.ExpectValid)

		if sample.Expect != program.ExpectValid {
			continue
		}

		body, marshalErr := sample.Value.MarshalJSON()
		require.NoError(t, marshalErr)

		reached[string(body)] = true
	}

	require.Equal(t, map[string]bool{
		`{}`:                    true,
		`{"a":false}`:           true,
		`{"a":true}`:            true,
		`{"b":false}`:           true,
		`{"b":true}`:            true,
		`{"a":false,"b":false}`: true,
		`{"a":false,"b":true}`:  true,
		`{"a":true,"b":false}`:  true,
		`{"a":true,"b":true}`:   true,
	}, reached)
}

func TestDecodeUnranksUnicodeAdditionalPropertyNames(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			MaxProperties:               countBound(t, "1"),
			AdditionalPropertiesAllowed: true,
			AdditionalPropertiesValidation: &validation.Validation{
				EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{jsonvalue.Bool(true)}},
			},
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	for rank, expected := range map[uint64]string{
		0:      `{"":true}`,
		956:    `{"λ":true}`,
		126465: `{"😀":true}`,
	} {
		sample, decodeErr := compiled.Decode(words(0, 0, 0, 8, rank, 0, 0, 0), generousLimits())
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, true)

		body, marshalErr := sample.Value.MarshalJSON()
		require.NoError(t, marshalErr)
		require.JSONEq(t, expected, string(body))
	}
}

func TestDecodeKeepsEveryTapeProductiveForUniversalAndEmptySchemas(t *testing.T) {
	t.Parallel()

	universal, err := program.Compile([]*validation.Validation{{
		ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true},
	}})
	require.NoError(t, err)
	universalSample, err := universal.Decode(words(0, 1), generousLimits())
	require.NoError(t, err)
	require.Equal(t, program.ExpectValid, universalSample.Expect)

	impossibleRoot := &validation.Validation{AllOfValidations: []*validation.Validation{
		{KindValidation: validation.KindValidation{Type: "string"}},
		{KindValidation: validation.KindValidation{Type: "number"}},
	}}
	impossible, err := program.Compile([]*validation.Validation{impossibleRoot})
	require.NoError(t, err)
	impossibleSample, err := impossible.Decode(nil, generousLimits())
	require.NoError(t, err)
	require.Equal(t, program.ExpectInvalid, impossibleSample.Expect)
	requireVerdict(t, impossibleRoot, impossibleSample.Value, false)
}

func TestDecodeReportsUnknownProductivityAsResourceError(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{AnyOfValidations: []*validation.Validation{
		{KindValidation: validation.KindValidation{Type: "string"}},
		{KindValidation: validation.KindValidation{Type: "number"}},
	}}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	_, err = compiled.Decode(nil, program.Limits{
		MaxSteps: 100, MaxOutputBytes: 100, MaxDepth: 4,
		MaxSolverWork: 0, MaxSolverBytes: 100,
	})

	var resource *program.ResourceError
	require.True(t, errors.As(err, &resource), err)
}

func TestDecodeChargesNormalizationBeforeExpandingAConjunction(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{}
	for minimum := range 64 {
		root.AllOfValidations = append(root.AllOfValidations, &validation.Validation{
			StringValidation: validation.StringValidation{
				MinLength: countBound(t, strconv.Itoa(minimum)),
			},
		})
	}

	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	limits := program.Limits{
		MaxSteps: 10_000, MaxOutputBytes: 1_000, MaxDepth: 16,
		MaxSolverWork: 10_000, MaxSolverBytes: 128,
	}

	_, firstErr := compiled.Decode(nil, limits)
	_, secondErr := compiled.Decode(nil, limits)

	var first, second *program.ResourceError
	require.ErrorAs(t, firstErr, &first)
	require.ErrorAs(t, secondErr, &second)
	require.Equal(t, "normalize goals bytes", first.Resource)
	require.Equal(t, first, second)
}

func TestCompileChargesGraphBeforeExceedingItsBudget(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "string"},
		StringValidation: validation.StringValidation{
			MinLength: countBound(t, "1"),
		},
	}

	_, err := program.Compile([]*validation.Validation{root}, program.CompileLimits{
		MaxNodes: 1, MaxFacts: 100, MaxProgramBytes: 100_000,
	})

	var resource *program.ResourceError
	require.ErrorAs(t, err, &resource)
	require.Equal(t, "compile nodes", resource.Resource)
}

func TestCompileChargesExactValuesBeforeCopyingProgramData(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{EnumValidation: validation.EnumValidation{
		ExactValues: []jsonvalue.Value{jsonvalue.String("a deliberately nontrivial value")},
	}}

	_, err := program.Compile([]*validation.Validation{root}, program.CompileLimits{
		MaxNodes: 100, MaxFacts: 100, MaxProgramBytes: 220,
	})

	var resource *program.ResourceError
	require.ErrorAs(t, err, &resource)
	require.Equal(t, "compile bytes", resource.Resource)
}

func TestDecodeChargesLazyStringProductivityToRuntimeBudget(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "string"},
		StringValidation: validation.StringValidation{
			Pattern:         "^a+$",
			CompiledPattern: mustPattern(t, "^a+$"),
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	_, err = compiled.Decode(nil, program.Limits{
		MaxSteps: 100, MaxOutputBytes: 100, MaxDepth: 4,
		MaxSolverWork: 10, MaxSolverBytes: 1_000,
	})

	var resource *program.ResourceError
	require.ErrorAs(t, err, &resource)
	require.Contains(t, resource.Resource, "string")
}

func TestDecodeDoesNotMaterializeAnUnchosenObjectName(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			AdditionalPropertiesAllowed: true,
		},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	sample, err := compiled.Decode(words(0, 0, 0, uint64(1)<<62), program.Limits{
		MaxSteps: 100, MaxOutputBytes: 100, MaxDepth: 4,
		MaxSolverWork: 100, MaxSolverBytes: 1_000,
	})
	require.NoError(t, err)
	require.Empty(t, sample.Value.Object)
}

func TestCompileAndDecodeStayLinearAcrossIndependentAnyOfChoices(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true},
	}
	for index := 0; index < 24; index++ {
		root.AllOfValidations = append(root.AllOfValidations, &validation.Validation{
			AnyOfValidations: []*validation.Validation{
				{EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{jsonvalue.Bool(false)}}},
				{EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{jsonvalue.Bool(true)}}},
			},
			ObjectValidation: validation.ObjectValidation{AdditionalPropertiesAllowed: true},
		})
	}

	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	firstFingerprint := compiled.Fingerprint()
	recompiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)
	require.Equal(t, firstFingerprint, recompiled.Fingerprint())

	sample, err := compiled.Decode(make([]byte, 1024), generousLimits())
	require.NoError(t, err)
	requireVerdict(t, root, sample.Value, sample.Expect == program.ExpectValid)
}

func TestDecodeReachesEverySmallIndependentAnyOfCombination(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		KindValidation: validation.KindValidation{Type: "object"},
		ObjectValidation: validation.ObjectValidation{
			Required:                    []string{"a", "b", "c"},
			AdditionalPropertiesAllowed: false,
		},
	}
	for _, name := range root.ObjectValidation.Required {
		root.ObjectValidation.Properties = append(
			root.ObjectValidation.Properties,
			validation.PropertyValidation{
				Name: name,
				Validation: &validation.Validation{AnyOfValidations: []*validation.Validation{
					{EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{jsonvalue.Bool(false)}}},
					{EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{jsonvalue.Bool(true)}}},
				}},
			},
		)
	}

	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	reached := make(map[string]bool)

	for seed := uint64(0); seed < 20_000 && len(reached) < 8; seed++ {
		sample, decodeErr := compiled.Decode(validPseudoTape(seed, 20), generousLimits())
		require.NoError(t, decodeErr)
		requireVerdict(t, root, sample.Value, sample.Expect == program.ExpectValid)

		if sample.Expect != program.ExpectValid {
			continue
		}

		body, marshalErr := sample.Value.MarshalJSON()
		require.NoError(t, marshalErr)

		reached[string(body)] = true
	}

	require.Equal(t, map[string]bool{
		`{"a":false,"b":false,"c":false}`: true,
		`{"a":false,"b":false,"c":true}`:  true,
		`{"a":false,"b":true,"c":false}`:  true,
		`{"a":false,"b":true,"c":true}`:   true,
		`{"a":true,"b":false,"c":false}`:  true,
		`{"a":true,"b":false,"c":true}`:   true,
		`{"a":true,"b":true,"c":false}`:   true,
		`{"a":true,"b":true,"c":true}`:    true,
	}, reached)
}

func TestDecodeIgnoresUnusedTapeSuffix(t *testing.T) {
	t.Parallel()

	root := &validation.Validation{
		EnumValidation: validation.EnumValidation{ExactValues: []jsonvalue.Value{jsonvalue.Bool(true)}},
	}
	compiled, err := program.Compile([]*validation.Validation{root})
	require.NoError(t, err)

	prefix := words(0, 0, 0)
	first, err := compiled.Decode(prefix, generousLimits())
	require.NoError(t, err)

	withSuffix := append(append([]byte(nil), prefix...), 1, 2, 3, 4, 5, 6, 7, 8)
	second, err := compiled.Decode(withSuffix, generousLimits())
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func countBound(t *testing.T, value string) *validation.CountBound {
	t.Helper()

	exact, err := jsonvalue.ParseNumber(value)
	require.NoError(t, err)

	return &validation.CountBound{Value: value, ExactValue: exact}
}

func numberBound(t *testing.T, value string, exclusive bool) *validation.NumberBound {
	t.Helper()

	return &validation.NumberBound{
		Value: value, Exclusive: exclusive, ExactValue: mustNumber(t, value),
	}
}

func mustNumber(t *testing.T, value string) jsonvalue.Number {
	t.Helper()

	exact, err := jsonvalue.ParseNumber(value)
	require.NoError(t, err)

	return exact
}

func generousLimits() program.Limits {
	return program.Limits{
		MaxSteps: 100_000, MaxOutputBytes: 1_000_000, MaxDepth: 64,
		MaxSolverWork: 100_000, MaxSolverBytes: 16_000_000,
	}
}

func mustPattern(t *testing.T, source string) *patternvalidator.PatternValidation {
	t.Helper()

	compiled, err := patternvalidator.Parse(source)
	require.NoError(t, err)

	return compiled
}

func words(values ...uint64) []byte {
	result := make([]byte, len(values)*8)
	for index, value := range values {
		binary.LittleEndian.PutUint64(result[index*8:], value)
	}

	return result
}

func pseudoTape(seed uint64, count int) []byte {
	result := make([]byte, count*8)
	state := seed

	for index := range count {
		state += 0x9e3779b97f4a7c15
		value := state
		value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
		value = (value ^ (value >> 27)) * 0x94d049bb133111eb
		value ^= value >> 31
		binary.LittleEndian.PutUint64(result[index*8:], value)
	}

	return result
}

func validPseudoTape(seed uint64, count int) []byte {
	result := pseudoTape(seed, count)
	clear(result[:min(len(result), 16)])

	return result
}

func invalidPseudoTape(seed uint64, count int) []byte {
	result := pseudoTape(seed, count)
	if len(result) >= 16 {
		clear(result[:16])
		result[8] = 1
	}

	return result
}

func testObjectMember(object jsonvalue.Value, name string) (jsonvalue.Value, bool) {
	for _, member := range object.Object {
		if member.Name == name {
			return member.Value, true
		}
	}

	return jsonvalue.Value{}, false
}

func mustMarshalValue(t *testing.T, value jsonvalue.Value) []byte {
	t.Helper()

	encoded, err := value.MarshalJSON()
	require.NoError(t, err)

	return encoded
}

func requireVerdict(t *testing.T, root *validation.Validation, value jsonvalue.Value, want bool) {
	t.Helper()

	body, err := value.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, want, len(root.Validate(body)) == 0, string(body))
}
