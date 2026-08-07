package schematest

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// retainedMemoryNoiseTolerance absorbs per-process runtime measurement noise.
const retainedMemoryNoiseTolerance = 512 << 10

// buildMemoryMeasurement separates cumulative allocation from retained heap.
type buildMemoryMeasurement struct {
	budget         uint64
	cases          int
	steps          uint64
	stop           StopReason
	emittedBytes   uint64
	totalAllocated uint64
	retained       uint64
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

	// Warm parser/runtime caches before comparing the two runs. Warmup callbacks
	// count and discard exactly like measured callbacks.
	for _, budget := range []uint64{100, 5_000} {
		_, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: budget}, func(Case) error {
			return nil
		})
		require.NoError(t, err)
	}

	short, err := measureBuildMemory(document, 100, 3)
	require.NoError(t, err)
	long, err := measureBuildMemory(document, 5_000, 39)
	require.NoError(t, err)

	t.Logf("retention measurements: short=%+v long=%+v", short, long)

	diagnostic := "short=%+v long=%+v"
	require.Equal(t, 3, short.cases, diagnostic, short, long)
	require.Equal(t, 39, long.cases, diagnostic, short, long)
	require.Equal(t, uint64(100), short.steps, diagnostic, short, long)
	require.Equal(t, uint64(2_901), long.steps, diagnostic, short, long)
	require.Equal(t, MaxStepsReached, short.stop, diagnostic, short, long)
	require.Equal(t, SpaceExhausted, long.stop, diagnostic, short, long)
	require.Greater(t, long.emittedBytes-short.emittedBytes, uint64(1<<20), diagnostic, short, long)
	require.Greater(t, long.totalAllocated, short.totalAllocated, diagnostic, short, long)

	// A full GC at each run's final callback measures the model, plan, search
	// state, and one current value while all prior callback values are
	// unreachable. The 512 KiB allowance absorbs process/runtime measurement
	// noise but remains well below the measured stream-size difference.
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
// scalar measurements, so the observer cannot turn the stream into a corpus.
func measureBuildMemory(document []byte, budget uint64, measureAtCase int) (buildMemoryMeasurement, error) {
	runtime.GC()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	measurement := buildMemoryMeasurement{budget: budget}

	report, err := Build(Input{OpenAPI: document, OperationID: "selected", MaxSteps: budget}, func(testCase Case) error {
		measurement.cases++
		measurement.emittedBytes += uint64(len(testCase.JSON))

		if measurement.cases == measureAtCase {
			runtime.GC()

			var live runtime.MemStats
			runtime.ReadMemStats(&live)
			measurement.retained = live.HeapAlloc
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

	return measurement, nil
}
