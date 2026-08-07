package schematest

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// BenchmarkBuildStress exercises the real Build path for representative
// structural and composition stress. The callback counts and discards each
// value; no emitted corpus is retained.
func BenchmarkBuildStress(b *testing.B) {
	benchmarks := []struct {
		name     string
		schema   string
		maxSteps uint64
	}{
		{name: "deep_wide", schema: deepWideStressSchema(), maxSteps: 100_000},
		{name: "composition_15", schema: compositionStressSchema(15), maxSteps: 100_000},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			document := []byte(documentWithJSONSchema(benchmark.schema))
			input := Input{OpenAPI: document, OperationID: "selected", MaxSteps: benchmark.maxSteps}

			// Warm one run before observing retained heap so parser and runtime
			// initialization do not become Build retention.
			warmCases := 0

			_, err := Build(input, func(Case) error {
				warmCases++

				return nil
			})
			if err != nil {
				b.Fatal(err)
			}

			measurement, err := measureStressBuild(input, warmCases)
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_, buildErr := Build(input, func(Case) error { return nil })
				if buildErr != nil {
					b.Fatal(buildErr)
				}
			}

			b.StopTimer()

			b.ReportMetric(float64(measurement.cases), "cases/op")
			b.ReportMetric(float64(measurement.steps), "steps/op")
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N), "build-ns/op")
			b.ReportMetric(float64(measurement.retained), "retained-B/op")
			b.Logf("real Build stop: %s", measurement.stop)
		})
	}
}

// stressBuildMeasurement records one instrumented real Build run.
type stressBuildMeasurement struct {
	cases    int
	steps    uint64
	stop     StopReason
	retained uint64
}

// measureStressBuild records work and live heap while callback values are discarded.
func measureStressBuild(input Input, measureAtCase int) (stressBuildMeasurement, error) {
	runtime.GC()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	measurement := stressBuildMeasurement{}

	report, err := Build(input, func(Case) error {
		measurement.cases++
		if measurement.cases == measureAtCase {
			runtime.GC()

			var live runtime.MemStats
			runtime.ReadMemStats(&live)

			if live.HeapAlloc > before.HeapAlloc {
				measurement.retained = live.HeapAlloc - before.HeapAlloc
			}
		}

		return nil
	})
	if err != nil {
		return stressBuildMeasurement{}, err
	}

	measurement.steps = report.Steps
	measurement.stop = report.Stop

	// The measured callback retained no prior bytes, so the live delta is the
	// model, plan, active search state, and current value rather than a corpus.
	return measurement, nil
}

// deepWideStressSchema constructs eight nested, eight-member object levels.
func deepWideStressSchema() string {
	schema := `{"type":"string","minLength":1,"maxLength":8,"pattern":"^[a-z]+$"}`

	for depth := range 8 {
		properties := []string{`"next":` + schema}
		required := []string{`"next"`}

		for width := range 8 {
			name := fmt.Sprintf("value_%02d_%02d", depth, width)
			properties = append(properties, fmt.Sprintf("%q:{\"type\":\"boolean\"}", name))
			required = append(required, fmt.Sprintf("%q", name))
		}

		schema = fmt.Sprintf(
			`{"type":"object","properties":{%s},"required":[%s],"additionalProperties":false}`,
			strings.Join(properties, ","),
			strings.Join(required, ","),
		)
	}

	return schema
}

// compositionStressSchema constructs mutually exclusive anyOf object branches.
func compositionStressSchema(branchCount int) string {
	branches := make([]string, 0, branchCount)
	for branch := range branchCount {
		name := fmt.Sprintf("branch_%02d", branch)
		branches = append(branches, fmt.Sprintf(
			`{"type":"object","properties":{%q:{"type":"boolean"}},"required":[%q],"additionalProperties":false}`,
			name,
			name,
		))
	}

	return fmt.Sprintf(`{"type":"object","anyOf":[%s]}`, strings.Join(branches, ","))
}
