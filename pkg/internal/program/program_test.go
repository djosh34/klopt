//nolint:godoclint // Test names document the public behavior under test.
package program_test

import (
	"encoding/binary"
	"errors"
	"math/big"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // Public-seam test of the internal program module.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

func TestDecodeBuildsEveryJSONKindAndNestedContainers(t *testing.T) {
	t.Parallel()

	falseValue := jsonvalue.Bool(false)
	number, err := jsonvalue.ParseNumber("123456789012345678901234567890")
	require.NoError(t, err)

	tests := []struct {
		name  string
		value jsonvalue.Value
	}{
		{name: "null", value: jsonvalue.Null()},
		{name: "boolean", value: falseValue},
		{name: "number", value: jsonvalue.Value{Kind: jsonvalue.KindNumber, Number: number}},
		{name: "string", value: jsonvalue.String("界")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var builder program.Builder

			root := builder.AddNode()
			require.NoError(t, builder.AddExactValue(root, test.value))
			sealed, sealErr := builder.Seal(root, builder.UniformSampling())
			require.NoError(t, sealErr)

			got, decodeErr := sealed.Decode(nil, generousLimits)
			require.NoError(t, decodeErr)
			require.True(t, got.Equal(test.value))
		})
	}

	var builder program.Builder

	root := builder.AddNode()
	objectBody := builder.AddNode()
	arrayRoot := builder.AddNode()
	arrayBody := builder.AddNode()
	child := builder.AddNode()
	arrayDone := builder.AddNode()
	objectDone := builder.AddNode()

	require.NoError(t, builder.AddBeginObject(root, objectBody))
	require.NoError(t, builder.AddObjectMember(objectBody, "items", arrayRoot, objectDone))
	require.NoError(t, builder.AddBeginArray(arrayRoot, arrayBody))
	require.NoError(t, builder.AddArrayItem(arrayBody, child, arrayDone))
	require.NoError(t, builder.AddExactValue(child, falseValue))
	require.NoError(t, builder.AddStop(arrayDone))
	require.NoError(t, builder.AddStop(objectDone))

	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)
	got, err := sealed.Decode(nil, generousLimits)
	require.NoError(t, err)
	want, err := jsonvalue.Parse([]byte(`{"items":[false]}`))
	require.NoError(t, err)
	require.True(t, got.Equal(want))
}

func TestDecodeUsesCanonicalArbitrarySignedIntegerCodec(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	minimum, ok := new(big.Int).SetString("100000000000000000000", 10)
	require.True(t, ok)
	maximum, ok := new(big.Int).SetString("100000000000000000002", 10)
	require.True(t, ok)
	require.NoError(t, builder.AddIntegerRange(
		root,
		minimum,
		maximum,
	))

	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)

	for index, want := range []string{
		"1e20",
		"100000000000000000001",
		"100000000000000000002",
	} {
		tape := make([]byte, 8)
		binary.LittleEndian.PutUint64(tape, uint64(index))
		value, decodeErr := sealed.Decode(tape, generousLimits)
		require.NoError(t, decodeErr)
		require.Equal(t, want, value.Number.Lexeme)
	}
}

func TestDecodeBuildsArbitraryLengthArrayWithIterativeSequence(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	child := builder.AddNode()
	require.NoError(t, builder.AddArraySequence(root, child, big.NewInt(0), nil))
	require.NoError(t, builder.AddExactValue(child, jsonvalue.Null()))

	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)

	for count, want := range []string{"[]", "[null]", "[null,null]"} {
		tape := make([]byte, 8)
		binary.LittleEndian.PutUint64(tape, uint64(count))

		value, decodeErr := sealed.Decode(tape, generousLimits)
		require.NoError(t, decodeErr)

		encoded, encodeErr := value.MarshalJSON()
		require.NoError(t, encodeErr)
		require.JSONEq(t, want, string(encoded))
	}
}

func TestDecodeRejectsHugeFixedArrayBeforeConstruction(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	child := builder.AddNode()
	count := big.NewInt(20_000_000)
	require.NoError(t, builder.AddArraySequence(root, child, count, count))
	require.NoError(t, builder.AddExactValue(child, jsonvalue.Null()))

	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)

	value, decodeErr := sealed.Decode(nil, program.Limits{
		MaxSteps: 10, MaxOutputBytes: 100, MaxDepth: 2,
	})
	require.Equal(t, jsonvalue.Value{}, value)

	var limitError *program.LimitError
	require.ErrorAs(t, decodeErr, &limitError)
	require.Equal(t, "steps", limitError.Resource)
}

var generousLimits = program.Limits{
	MaxSteps:       100,
	MaxOutputBytes: 100,
	MaxDepth:       10,
}

func TestDecodeUsesStableFixedWidthChoices(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	second := builder.AddNode()
	done := builder.AddNode()

	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: 'a', Last: 'a'}}, second))
	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: 'b', Last: 'b'}}, second))
	require.NoError(t, builder.AddScalarRanges(second, []program.ScalarRange{{First: 'x', Last: 'x'}}, done))
	require.NoError(t, builder.AddScalarRanges(second, []program.ScalarRange{{First: 'y', Last: 'y'}}, done))
	require.NoError(t, builder.AddStringStop(done))

	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)

	tape := make([]byte, 16)
	binary.LittleEndian.PutUint64(tape[:8], ^uint64(0))
	binary.LittleEndian.PutUint64(tape[8:], ^uint64(0))

	value, err := sealed.Decode(tape, generousLimits)
	require.NoError(t, err)
	require.True(t, value.Equal(jsonvalue.String("by")))

	binary.LittleEndian.PutUint64(tape[:8], 0)
	value, err = sealed.Decode(tape, generousLimits)
	require.NoError(t, err)
	require.True(t, value.Equal(jsonvalue.String("ay")))
}

func TestDecodeZeroSelectsShortestCompletion(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	long := builder.AddNode()
	done := builder.AddNode()

	// Add the longer route first so insertion order cannot accidentally define the default.
	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: 'b', Last: 'b'}}, long))
	require.NoError(t, builder.AddScalarRanges(long, []program.ScalarRange{{First: 'c', Last: 'c'}}, done))
	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: 'a', Last: 'a'}}, done))
	require.NoError(t, builder.AddStringStop(done))

	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)

	for _, tape := range [][]byte{nil, {}, {0}, make([]byte, 7)} {
		value, decodeErr := sealed.Decode(tape, generousLimits)
		require.NoError(t, decodeErr)
		require.True(t, value.Equal(jsonvalue.String("a")))
	}
}

func TestDecodeSingleRouteConsumesNoChoiceWordsAndIgnoresSuffix(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	done := builder.AddNode()
	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: '界', Last: '界'}}, done))
	require.NoError(t, builder.AddStringStop(done))

	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)

	for _, tape := range [][]byte{nil, {0xff}, make([]byte, 64)} {
		value, decodeErr := sealed.Decode(tape, generousLimits)
		require.NoError(t, decodeErr)
		require.True(t, value.Equal(jsonvalue.String("界")))
	}
}

func TestSealRejectsDeterministicCycleWithoutCompletion(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: 'a', Last: 'a'}}, root))

	_, err := builder.Seal(root, builder.UniformSampling())
	require.ErrorContains(t, err, "does not terminate")
}

func TestSealValidatesAndFingerprintsBoundSamplingWeights(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	done := builder.AddNode()
	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: 'a', Last: 'a'}}, done))
	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: 'b', Last: 'b'}}, done))
	require.NoError(t, builder.AddStringStop(done))

	_, err := builder.Seal(root, program.SamplingTable{})
	require.ErrorContains(t, err, "sampling table")
	_, err = builder.Seal(root, program.SamplingTable{Weights: []uint32{1, 0, 1}})
	require.ErrorContains(t, err, "weight 1 is zero")

	uniform, err := builder.Seal(root, program.SamplingTable{Weights: []uint32{1, 1, 1}})
	require.NoError(t, err)
	weighted, err := builder.Seal(root, program.SamplingTable{Weights: []uint32{1, 2, 1}})
	require.NoError(t, err)
	require.NotEqual(t, uniform.Fingerprint(), weighted.Fingerprint())

	tape := make([]byte, 8)
	binary.LittleEndian.PutUint64(tape, uint64(1)<<63)
	value, err := weighted.Decode(tape, generousLimits)
	require.NoError(t, err)
	require.True(t, value.Equal(jsonvalue.String("b")))
}

func TestDecodeReturnsTypedLimitsWithoutPartialValues(t *testing.T) {
	t.Parallel()

	var builder program.Builder

	root := builder.AddNode()
	second := builder.AddNode()
	done := builder.AddNode()
	require.NoError(t, builder.AddScalarRanges(root, []program.ScalarRange{{First: 'é', Last: 'é'}}, second))
	require.NoError(t, builder.AddScalarRanges(second, []program.ScalarRange{{First: '😀', Last: '😀'}}, done))
	require.NoError(t, builder.AddStringStop(done))
	sealed, err := builder.Seal(root, builder.UniformSampling())
	require.NoError(t, err)

	tests := []struct {
		name     string
		limits   program.Limits
		resource string
		observed uint64
	}{
		{
			name: "depth", limits: program.Limits{MaxSteps: 3, MaxOutputBytes: 6, MaxDepth: 0},
			resource: "depth", observed: 1,
		},
		{
			name: "steps", limits: program.Limits{MaxSteps: 2, MaxOutputBytes: 8, MaxDepth: 1},
			resource: "steps", observed: 3,
		},
		{
			name: "output bytes", limits: program.Limits{MaxSteps: 3, MaxOutputBytes: 7, MaxDepth: 1},
			resource: "output bytes", observed: 8,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			value, decodeErr := sealed.Decode(nil, test.limits)
			require.Equal(t, jsonvalue.Value{}, value)

			var limitError *program.LimitError
			require.True(t, errors.As(decodeErr, &limitError))
			require.Equal(t, test.resource, limitError.Resource)
			require.Equal(t, test.observed, limitError.Observed)
		})
	}
}
