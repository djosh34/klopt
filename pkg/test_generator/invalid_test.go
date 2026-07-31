//nolint:godoclint // Package-private tests document focused invalid generation.
package testgenerator

import (
	"encoding/binary"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // The test selects the shared rune stream deterministically.
	"github.com/stretchr/testify/require"
)

func TestDecodeInvalidMaximumUsesOneExactWitness(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"maximum":10}`)
	tape := make([]byte, 24)
	tape[8] = 1
	tape[16] = 1

	sample, status, err := generator.Decode(tape)
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.Equal(t, "11", string(sample.Body))

	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidRequiredPropertyKeepsOtherProperties(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"object",
		"required":["name","enabled"],
		"properties":{"enabled":{"type":"boolean"},"name":{"type":"string"}}
	}`)

	tape := make([]byte, 24)
	tape[8] = 1
	tape[16] = 1

	sample, status, err := generator.Decode(tape)
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.JSONEq(t, `{"name":""}`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidNestedPropertyKeepsLaterSiblingValid(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"object",
		"required":["a","b"],
		"properties":{
			"a":{"type":"object","required":["value"],"properties":{"value":{"type":"number","maximum":10}}},
			"b":{"type":"string"}
		}
	}`)

	tape := make([]byte, 24)
	tape[8] = 1
	tape[16] = 14

	sample, status, err := generator.Decode(tape)
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.JSONEq(t, `{"a":{"value":11},"b":""}`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidAllOfFailsOneChildAndKeepsSiblingValid(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"string",
		"allOf":[{"minLength":2},{"pattern":"^a"}]
	}`)

	root := generator.operations[0].root
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAll && plan.target == root.children[1] && plan.failedChild == 1
	})
	tape := tapeForInvalidPlan(planIndex)
	tape = append(tape, 1)
	tape = append(tape, make([]byte, 8)...)
	patternLanguage := root.children[2].children[1].atom.language
	walk, walkErr := stringlanguage.Begin([]stringlanguage.Requirement{{
		Language: patternLanguage, WantMatch: true,
	}})
	require.NoError(t, walkErr)

	word, ok := wordForRune(walk.Ranges(), 'a')
	require.True(t, ok)
	binary.LittleEndian.PutUint64(tape[25:], word)

	sample, status, err := generator.Decode(tape)
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.Equal(t, `"a"`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))

	value := mustValue(t, sample.Body)
	failed, failedErr := expressionHolds(root.children[1], value)
	require.NoError(t, failedErr)
	require.False(t, failed)

	passing, passingErr := expressionHolds(root.children[2], value)
	require.NoError(t, passingErr)
	require.True(t, passing)
}

func TestDecodeInvalidAnyOfRejectsOneSharedCandidateInEveryBranch(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"string",
		"anyOf":[{"minLength":3},{"pattern":"^a"}]
	}`)

	root := generator.operations[0].root
	anyExpression := root.children[len(root.children)-1]
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAny && plan.target == anyExpression
	})
	tape := tapeForInvalidPlan(planIndex)
	tape = append(tape, make([]byte, 16)...)
	tape[24] = 1
	tape[32] = 1

	sample, status, err := generator.Decode(tape)
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.Equal(t, `""`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))

	value := mustValue(t, sample.Body)
	for _, branch := range anyExpression.children {
		holds, branchErr := expressionHolds(branch, value)
		require.NoError(t, branchErr)
		require.False(t, holds)
	}
}

func TestDecodeInvalidAnyOfRequiredBranchesShareOneObject(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"object",
		"minProperties":2,
		"properties":{"a":{"type":"string"},"b":{"type":"string"}},
		"anyOf":[{"required":["a"]},{"required":["b"]}]
	}`)

	root := generator.operations[0].root
	anyExpression := root.children[len(root.children)-1]
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAny && plan.target == anyExpression
	})

	tape := tapeForInvalidPlan(planIndex)
	tape = append(tape, make([]byte, 16)...)
	tape[24] = 1
	tape[32] = 1

	sample, status, err := generator.Decode(tape)
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)

	value := mustValue(t, sample.Body)
	for _, branch := range anyExpression.children {
		holds, branchErr := expressionHolds(branch, value)
		require.NoError(t, branchErr)
		require.False(t, holds)
	}

	minProperties, minErr := expressionHolds(root.children[1], value)
	require.NoError(t, minErr)
	require.True(t, minProperties)
}

func TestDecodeInvalidNestedArrayItemUsesEveryRequiredPathContainer(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"object",
		"required":["items"],
		"properties":{"items":{
			"type":"array","minItems":1,
			"items":{"type":"array","minItems":1,"items":{"type":"number","maximum":10}}
		}}
	}`)

	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && len(plan.schemaPath) == 3 &&
			plan.target.kind == expressionAtom && plan.target.atom.kind == atomNumberMaximum
	})

	sample, status, err := generator.Decode(tapeForInvalidPlan(planIndex))
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.JSONEq(t, `{"items":[[11]]}`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidStringPatternUsesSharedWitnessWalk(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"string","pattern":"^a"}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && len(plan.schemaPath) == 0 &&
			plan.target.kind == expressionAtom && plan.target.atom.kind == atomStringLanguage
	})

	sample, status, err := generator.Decode(tapeForInvalidPlan(planIndex))
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.Equal(t, `""`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidEnumChoosesAValueOutsideTheEnum(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"string","enum":[""]}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && plan.target.atom.kind == atomEnum
	})

	sample, status, err := generator.Decode(tapeForInvalidPlan(planIndex))
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.Equal(t, `"a"`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidArrayLengthPreservesItemConstruction(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"array","minItems":2,"maxItems":3,"items":{"type":"boolean"}
	}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && plan.target.atom.kind == atomArrayMinItems
	})

	sample, status, err := generator.Decode(tapeForInvalidPlan(planIndex))
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.Equal(t, `[]`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidAdditionalPropertyAddsOneUnknownMember(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"object","additionalProperties":false}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && plan.target.atom.kind == atomObjectAdditional
	})

	sample, status, err := generator.Decode(tapeForInvalidPlan(planIndex))
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.JSONEq(t, `{"additional0":null}`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidNestedAdditionalPropertyUsesOneSharedName(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"object",
		"additionalProperties":{"type":"number","maximum":10}
	}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && len(plan.schemaPath) == 1 &&
			plan.schemaPath[0].kind == faultAdditionalProperty &&
			plan.target.atom.kind == atomNumberMaximum
	})

	sample, status, err := generator.Decode(tapeForInvalidPlan(planIndex))
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.JSONEq(t, `{"additional0":11}`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidObjectPropertyKeepsTheFaultLocal(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{
		"type":"object",
		"properties":{"a":{"type":"string"},"b":{"type":"boolean"}}
	}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && len(plan.schemaPath) == 0 &&
			plan.target.atom.kind == atomObjectProperty && plan.target.atom.name == "a"
	})

	sample, status, err := generator.Decode(tapeForInvalidPlan(planIndex))
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	require.JSONEq(t, `{"a":null}`, string(sample.Body))
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeInvalidShortTapeStillBuildsTheFocusedStringWitness(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"string","minLength":2}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && plan.target.atom.kind == atomStringMinLength
	})

	sample, status, err := generator.Decode(tapeForInvalidPlan(planIndex))
	require.NoError(t, err)
	require.Equal(t, Generated, status)
	require.False(t, sample.ExpectValid)
	value := mustValue(t, sample.Body)
	require.Equal(t, "", value.String)
	require.NotEmpty(t, generator.operations[0].runtime.Body.Validate(sample.Body))
}

func TestDecodeSameInvalidTapeIsDeterministic(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"number","maximum":10}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && plan.target.atom.kind == atomNumberMaximum
	})
	tape := tapeForInvalidPlan(planIndex)

	first, firstStatus, firstErr := generator.Decode(tape)
	second, secondStatus, secondErr := generator.Decode(tape)

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Equal(t, firstStatus, secondStatus)
	require.Equal(t, first, second)
}

func TestFocusedInvalidBuilderMarksOneFaultAfterConstruction(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"number","maximum":10}`)
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAtom && plan.target.atom.kind == atomNumberMaximum
	})
	tape := tapeForInvalidPlan(planIndex)
	cursor := newTapeCursor(tape)
	cursor.takeWord()
	cursor.takeWord()
	cursor.takeWord()

	token := newFaultToken(&generator.operations[0].faultPlans[planIndex])
	built := buildValueWithFault(generator.operations[0].root, token, 0, cursor)
	require.Equal(t, buildComplete, built.state)
	require.NoError(t, built.err)
	require.Equal(t, faultInjected, token.state)

	accepted, err := expressionHolds(generator.operations[0].root, built.value)
	require.NoError(t, err)
	require.False(t, accepted)
}

func TestDecodeImpossibleFocusedWitnessReturnsExhausted(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"number","allOf":[{"type":"number"}]}`)
	root := generator.operations[0].root
	planIndex := findFaultPlan(t, generator, func(plan faultPlan) bool {
		return plan.targetKind == faultAll && plan.target == root.children[1] && plan.failedChild == 0
	})
	tape := tapeForInvalidPlan(planIndex)
	tape = append(tape, make([]byte, 64)...)

	sample, status, err := generator.Decode(tape)
	require.NoError(t, err)
	require.Equal(t, Exhausted, status)
	require.Equal(t, Sample{}, sample)
}

func findFaultPlan(t *testing.T, generator *Generator, match func(faultPlan) bool) int {
	t.Helper()

	for index, plan := range generator.operations[0].faultPlans {
		if match(plan) {
			return index
		}
	}

	t.Fatalf("fault plan not found")

	return 0
}

func tapeForInvalidPlan(planIndex int) []byte {
	tape := make([]byte, 24)
	tape[8] = 1
	tape[16] = byte(planIndex)

	return tape
}

func wordForRune(ranges []stringlanguage.ScalarRange, target rune) (uint64, bool) {
	for index, selected := range ranges {
		if target < selected.First || target > selected.Last {
			continue
		}

		width := uint64(selected.Last-selected.First) + 1

		return uint64(target-selected.First)*uint64(len(ranges)) + uint64(index), width > 0
	}

	return 0, false
}
