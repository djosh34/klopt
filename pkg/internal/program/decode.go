//nolint:godoclint // Private decode work stays behind Program.Decode.
package program

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type decodeWork struct {
	limits           Limits
	steps            uint64
	solverWork       uint64
	solverBytes      uint64
	productive       map[string]bool
	known            map[string]bool
	arrayProductive  map[string]bool
	arrayKnown       map[string]bool
	objectProductive map[string]bool
	objectKnown      map[string]bool
	fault            faultState
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
		limits:     limits,
		productive: make(map[string]bool), known: make(map[string]bool),
		arrayProductive: make(map[string]bool), arrayKnown: make(map[string]bool),
		objectProductive: make(map[string]bool), objectKnown: make(map[string]bool),
	}
	operation := OperationID(reader.word() % uint64(len(program.roots)))

	expect, err := program.chooseVerdict(program.roots[operation], reader.word(), &work)
	if err != nil {
		return Sample{}, err
	}

	if expect == ExpectInvalid {
		work.fault = readFaultState(&reader)
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

//nolint:cyclop // The walk keeps every terminal and typed error explicit.
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

		terminal, branching, possible, err := program.normalize(current, work)
		if err != nil {
			return jsonvalue.Value{}, false, err
		}

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

		selected, err := weightedIndex(word, edges)
		if err != nil {
			return jsonvalue.Value{}, false, err
		}

		current = edges[selected].next
	}
}

func (work *decodeWork) step() error {
	observed, ok := checkedAdd(work.steps, 1)
	if !ok {
		return &LimitError{
			Resource: "steps", Limit: work.limits.MaxSteps, Observed: ^uint64(0),
		}
	}

	work.steps = observed

	return checkLimit("steps", work.limits.MaxSteps, work.steps)
}

func (work *decodeWork) solver(bytes uint64) error {
	return work.chargeSolver("work", "bytes", 1, bytes)
}

func (work *decodeWork) stringCharge(resource string, amount uint64, bytes uint64) error {
	return work.chargeSolver(resource, resource+" bytes", amount, bytes)
}

func (work *decodeWork) chargeGoals(resource string, count int) error {
	const goalBytes = 8

	bytes, ok := checkedMul(uint64(count), goalBytes)
	if !ok {
		return &ResourceError{
			Resource: resource + " bytes", Limit: work.limits.MaxSolverBytes,
			Observed: ^uint64(0),
		}
	}

	return work.chargeSolver(resource, resource+" bytes", uint64(count), bytes)
}

func (work *decodeWork) chargeExactValues(resource string, values []jsonvalue.Value) error {
	bytes := uint64(0)

	for _, value := range values {
		size, err := exactValueBytes(value)
		if err != nil {
			return err
		}

		var ok bool

		bytes, ok = checkedAdd(bytes, size)
		if !ok {
			return &ResourceError{
				Resource: resource, Limit: work.limits.MaxSolverBytes, Observed: ^uint64(0),
			}
		}
	}

	return work.chargeSolver(resource, resource, uint64(len(values)), bytes)
}

func (work *decodeWork) chargeSolver(
	workResource string,
	byteResource string,
	workAmount uint64,
	byteAmount uint64,
) error {
	observedWork, ok := checkedAdd(work.solverWork, workAmount)
	if !ok || observedWork > work.limits.MaxSolverWork {
		return &ResourceError{
			Resource: workResource, Limit: work.limits.MaxSolverWork, Observed: observedWork,
		}
	}

	observedBytes, ok := checkedAdd(work.solverBytes, byteAmount)
	if !ok || observedBytes > work.limits.MaxSolverBytes {
		return &ResourceError{
			Resource: byteResource, Limit: work.limits.MaxSolverBytes, Observed: observedBytes,
		}
	}

	work.solverWork = observedWork
	work.solverBytes = observedBytes

	return nil
}
