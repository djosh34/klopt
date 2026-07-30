//nolint:godoclint // Private decode work stays behind Program.Decode.
package program

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type decodeWork struct {
	limits      Limits
	steps       uint64
	solverWork  uint64
	solverBytes uint64
	productive  map[string]bool
	known       map[string]bool
}

// Decode maps every arbitrary byte slice to one deterministic graph walk.
func (program *Program) Decode(input []byte, limits Limits) (Sample, error) {
	if program == nil {
		return Sample{}, fmt.Errorf("decode nil program")
	}

	if len(program.roots) == 0 {
		return Sample{}, fmt.Errorf("decode program with no operations")
	}

	reader := tapeReader{tape: input}
	work := decodeWork{
		limits: limits, productive: make(map[string]bool), known: make(map[string]bool),
	}
	operation := OperationID(reader.word() % uint64(len(program.roots)))

	expect, err := program.chooseVerdict(program.roots[operation], reader.word(), &work)
	if err != nil {
		return Sample{}, err
	}

	value, possible, err := program.decodeState(canonicalState([]goal{{
		node: program.roots[operation], want: expect == ExpectValid,
	}}), &reader, &work, 0)
	if err != nil {
		return Sample{}, err
	}

	if !possible {
		return Sample{}, fmt.Errorf("selected signed root has no JSON values")
	}

	encoded, err := value.MarshalJSON()
	if err != nil {
		return Sample{}, fmt.Errorf("marshal decoded value: %w", err)
	}

	if err := checkLimit("output bytes", limits.MaxOutputBytes, uint64(len(encoded))); err != nil {
		return Sample{}, err
	}

	return Sample{Operation: operation, Expect: expect, Value: value}, nil
}

func (program *Program) decodeState(
	current state,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, bool, error) {
	if err := checkLimit("depth", work.limits.MaxDepth, depth); err != nil {
		return jsonvalue.Value{}, false, err
	}

	for {
		if err := work.step(); err != nil {
			return jsonvalue.Value{}, false, err
		}

		terminal, branching, possible := program.normalize(current)
		if !possible {
			return jsonvalue.Value{}, false, nil
		}

		if branching == nil {
			return program.sampleAtoms(
				terminal.goals, terminal.excluded, reader, work, depth,
			)
		}

		edges, err := program.expand(*branching, work)
		if err != nil {
			return jsonvalue.Value{}, false, err
		}

		if len(edges) == 0 {
			return jsonvalue.Value{}, false, nil
		}

		word := uint64(0)
		if reader != nil {
			word = reader.word()
		}

		current = edges[weightedIndex(word, edges)].next
	}
}

func (work *decodeWork) step() error {
	work.steps++

	return checkLimit("steps", work.limits.MaxSteps, work.steps)
}

func (work *decodeWork) solver(bytes uint64) error {
	work.solverWork++
	if work.solverWork > work.limits.MaxSolverWork {
		return &ResourceError{
			Resource: "work", Limit: work.limits.MaxSolverWork, Observed: work.solverWork,
		}
	}

	work.solverBytes += bytes
	if work.solverBytes > work.limits.MaxSolverBytes {
		return &ResourceError{
			Resource: "bytes", Limit: work.limits.MaxSolverBytes, Observed: work.solverBytes,
		}
	}

	return nil
}
