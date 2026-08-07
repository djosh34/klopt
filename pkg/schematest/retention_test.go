package schematest

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// retainedMemoryNoiseTolerance absorbs per-process runtime measurement noise.
const retainedMemoryNoiseTolerance = int64(512 << 10)

// buildMemoryMeasurement uses one definition for tests and benchmarks: retained
// is the signed difference between a post-GC pre-run HeapAlloc sample and the
// post-GC HeapAlloc observed in the requested callback. Raw samples are kept so
// runtime noise can never be hidden by clamping. TotalAllocated is cumulative
// allocation between the pre-run sample and Build return.
type buildMemoryMeasurement struct {
	budget         uint64
	cases          int
	steps          uint64
	stop           StopReason
	emittedBytes   uint64
	preRunHeap     uint64
	callbackHeap   uint64
	totalAllocated uint64
	retained       int64
}

// TestBuildRetainedMemoryIsFlatWithEmittedCount compares live execution state
// after earlier callback values are unreachable. It intentionally does not run
// in parallel because runtime memory statistics cover the whole process.
//
//nolint:paralleltest // Per-process memory statistics require isolation from concurrent tests.
func TestBuildRetainedMemoryIsFlatWithEmittedCount(t *testing.T) {
	padding := strings.Repeat("x", 64<<10)
	schema := fmt.Sprintf(`{
		"type":"object",
		"properties":{
			"a":{"type":"boolean"},
			"b":{"type":"boolean"},
			"c":{"type":"boolean"},
			"d":{"type":"boolean"},
			"e":{"type":"boolean"},
			"f":{"type":"boolean"},
			"g":{"type":"boolean"},
			"h":{"type":"boolean"},
			"padding":{"type":"string","enum":[%q]}
		},
		"required":["a","b","c","d","e","f","g","h","padding"],
		"additionalProperties":false
	}`, padding)
	document := []byte(documentWithJSONSchema(schema))

	for _, budget := range []uint64{100, 5_000} {
		_, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: budget}, func(Case) error {
			return nil
		})
		require.NoError(t, err)
	}

	short, err := measureBuildMemory(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 100}, 3,
	)
	require.NoError(t, err, "measurement=%+v", short)
	long, err := measureBuildMemory(
		Input{OpenAPI: document, OperationID: "selected", MaxSteps: 5_000}, 39,
	)
	require.NoError(t, err, "measurement=%+v", long)

	t.Logf("retention measurements: short=%+v long=%+v", short, long)

	diagnostic := "short=%+v long=%+v"
	require.Equal(t, 3, short.cases, diagnostic, short, long)
	require.Equal(t, 39, long.cases, diagnostic, short, long)
	require.Equal(t, uint64(100), short.steps, diagnostic, short, long)
	require.Equal(t, uint64(2_901), long.steps, diagnostic, short, long)
	require.Equal(t, MaxStepsReached, short.stop, diagnostic, short, long)
	require.Equal(t, SpaceExhausted, long.stop, diagnostic, short, long)
	require.NotZero(t, short.preRunHeap, diagnostic, short, long)
	require.NotZero(t, short.callbackHeap, diagnostic, short, long)
	require.NotZero(t, long.preRunHeap, diagnostic, short, long)
	require.NotZero(t, long.callbackHeap, diagnostic, short, long)
	require.Equal(t, signedHeapDifference(short.callbackHeap, short.preRunHeap), short.retained)
	require.Equal(t, signedHeapDifference(long.callbackHeap, long.preRunHeap), long.retained)
	require.Greater(t, long.emittedBytes-short.emittedBytes, uint64(1<<20), diagnostic, short, long)
	require.Greater(t, long.totalAllocated, short.totalAllocated, diagnostic, short, long)
	require.LessOrEqual(
		t,
		long.retained,
		short.retained+retainedMemoryNoiseTolerance,
		diagnostic,
		short,
		long,
	)
}

// measureBuildMemory counts and discards callback values. It retains only
// scalar raw measurements and fails unless the requested callback is observed.
func measureBuildMemory(input Input, measureAtCase int) (buildMemoryMeasurement, error) {
	if measureAtCase <= 0 {
		return buildMemoryMeasurement{}, errors.New("memory sample callback must be positive")
	}

	runtime.GC()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	measurement := buildMemoryMeasurement{
		budget:     input.MaxSteps,
		preRunHeap: before.HeapAlloc,
	}

	report, err := Build(input, func(testCase Case) error {
		measurement.cases++
		measurement.emittedBytes += uint64(len(testCase.JSON))

		if measurement.cases == measureAtCase {
			runtime.GC()

			var live runtime.MemStats
			runtime.ReadMemStats(&live)
			measurement.callbackHeap = live.HeapAlloc
			measurement.retained = signedHeapDifference(live.HeapAlloc, before.HeapAlloc)
		}

		return nil
	})
	if err != nil {
		return buildMemoryMeasurement{}, err
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	measurement.steps = report.Steps
	measurement.stop = report.Stop
	measurement.totalAllocated = after.TotalAlloc - before.TotalAlloc

	if measurement.callbackHeap == 0 {
		return measurement, fmt.Errorf(
			"memory sample callback %d was not reached; measurement=%+v", measureAtCase, measurement,
		)
	}

	return measurement, nil
}

// TestBuildMemoryMeasurementPreservesRawResultsAndRequiresItsSample pins the shared methodology.
//
//nolint:paralleltest // The helper reads process-wide memory statistics.
func TestBuildMemoryMeasurementPreservesRawResultsAndRequiresItsSample(t *testing.T) {
	require.Equal(t, int64(25), signedHeapDifference(125, 100))
	require.Equal(t, int64(-25), signedHeapDifference(100, 125))

	measurement, err := measureBuildMemory(Input{
		OpenAPI:     []byte(documentWithJSONSchema(`{"type":"boolean"}`)),
		OperationID: "selected",
		MaxSteps:    5,
	}, 100)
	require.ErrorContains(t, err, "memory sample callback 100 was not reached")
	require.Equal(t, uint64(5), measurement.budget)
	require.Equal(t, 2, measurement.cases)
	require.Equal(t, uint64(5), measurement.steps)
	require.Equal(t, SpaceExhausted, measurement.stop)
	require.NotZero(t, measurement.preRunHeap)
	require.Zero(t, measurement.callbackHeap)
	require.Zero(t, measurement.retained)
	require.Positive(t, measurement.totalAllocated)
	require.Contains(t, err.Error(), fmt.Sprintf("measurement=%+v", measurement))
}

// signedHeapDifference preserves both positive and negative raw heap deltas.
func signedHeapDifference(after, before uint64) int64 {
	if after >= before {
		return int64(after - before)
	}

	return -int64(before - after)
}
