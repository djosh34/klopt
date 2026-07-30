// Package testgenerator generates structured JSON request bodies and checks a validator.
//
//nolint:godoclint,mnd // Private native-fuzz adapter constants stay local to this deep module.
package testgenerator

import (
	"encoding/binary"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // Runner executes final sealed CasePlans only.
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/djosh34/klopt/pkg/test_generator/internal/suite"
)

const selectorBytes = 8

var fuzzLimits = program.Limits{
	MaxSteps:       100_000,
	MaxOutputBytes: 1_000_000,
	MaxDepth:       64,
}

type executableCase struct {
	operationID string
	plan        suite.CasePlan
}

// CheckJSONRequestBodies runs every canonical structured request-body seed.
func CheckJSONRequestBodies(
	t *testing.T,
	openAPIYAML []byte,
	validate func(operationID string, body []byte) error,
	patternOption patternvalidator.Option,
) {
	t.Helper()

	plans := compileCases(t, openAPIYAML, validate, patternOption)
	for _, planned := range plans {
		t.Run(planned.operationID+"/"+planned.plan.Name, func(t *testing.T) {
			t.Parallel()

			for _, seed := range planned.plan.Seeds {
				checkCase(t, planned, seed, validate)
			}
		})
	}
}

// FuzzJSONRequestBodies registers every CasePlan seed and runs one native Go fuzz callback.
func FuzzJSONRequestBodies(
	f *testing.F,
	openAPIYAML []byte,
	validate func(operationID string, body []byte) error,
	patternOption patternvalidator.Option,
) {
	f.Helper()

	plans := compileCases(f, openAPIYAML, validate, patternOption)
	for index, planned := range plans {
		for _, seed := range planned.plan.Seeds {
			input := make([]byte, selectorBytes+len(seed))
			binary.LittleEndian.PutUint64(input, uint64(index))
			copy(input[selectorBytes:], seed)
			f.Add(input)
		}
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(plans) == 0 {
			return
		}

		var selector [selectorBytes]byte
		copy(selector[:], input)
		index := binary.LittleEndian.Uint64(selector[:]) % uint64(len(plans))

		var tape []byte
		if len(input) > selectorBytes {
			tape = input[selectorBytes:]
		}

		checkCase(t, plans[index], tape, validate)
	})
}

func compileCases(
	tb testing.TB,
	openAPIYAML []byte,
	validate func(operationID string, body []byte) error,
	patternOption patternvalidator.Option,
) []executableCase {
	tb.Helper()

	if validate == nil {
		tb.Fatal("validator is nil")
	}

	if patternOption == nil {
		tb.Fatal("pattern option is nil")
	}

	sources, err := oas.Parse(openAPIYAML)
	if err != nil {
		tb.Fatal(err)
	}

	result := make([]executableCase, 0)

	for _, operationID := range slices.Sorted(maps.Keys(sources)) {
		compiled, compileErr := suite.CompileSuite(sources[operationID], suite.CompilerOptions{
			PatternOption: patternOption,
		})
		if compileErr != nil {
			tb.Fatalf("compile operation %q: %v", operationID, compileErr)
		}

		for _, plan := range compiled.Cases {
			result = append(result, executableCase{operationID: operationID, plan: plan})
		}
	}

	return result
}

func checkCase(
	t *testing.T,
	planned executableCase,
	tape []byte,
	validate func(operationID string, body []byte) error,
) {
	t.Helper()

	value, err := planned.plan.Program.Decode(tape, fuzzLimits)
	if err != nil {
		var limit *program.LimitError
		if errors.As(err, &limit) {
			return
		}

		t.Fatalf("%s/%s decode tape %x: %v", planned.operationID, planned.plan.Name, tape, err)
	}

	body, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("%s/%s marshal: %v", planned.operationID, planned.plan.Name, err)
	}

	validationErr := validate(planned.operationID, body)
	if planned.plan.Expect == suite.ExpectAccepted && validationErr != nil {
		t.Fatalf("%s/%s rejected valid body %s: %v", planned.operationID, planned.plan.Name, body, validationErr)
	}

	if planned.plan.Expect == suite.ExpectRejected && validationErr == nil {
		t.Fatalf("%s/%s accepted invalid body %s", planned.operationID, planned.plan.Name, body)
	}
}
