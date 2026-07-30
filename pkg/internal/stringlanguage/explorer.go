//nolint:cyclop,gocognit,godoclint,mnd // Exact automata exploration is an explicit state machine.
package stringlanguage

import (
	"encoding/binary"
	"fmt"
	"slices"
)

// Charge accounts one unit of exact exploration before its allocation.
type Charge func(resource string, work uint64, bytes uint64) error

// Explorer lazily interns the signed product states reached by one string walk.
type Explorer struct {
	machines     []*dfa
	requirements []Requirement
	length       Length
	states       []explorerTuple
	byKey        map[string]State
	transitions  map[State][]ScalarRange
	distance     map[State]uint64
	dead         map[State]bool
}

type explorerTuple struct {
	patterns []uint32
	length   int
}

// NewExplorer prepares one implicit signed product without enumerating its reachable states.
func NewExplorer(
	requirements []Requirement,
	length Length,
	charge Charge,
) (*Explorer, error) {
	if err := validateLength(length); err != nil {
		return nil, err
	}

	if len(requirements) > maximumRequirements {
		return nil, limitError(
			"input", "requirements", maximumRequirements, uint64(len(requirements)),
		)
	}

	if charge == nil {
		return nil, &CompileError{
			Operation: "create lazy explorer", Err: fmt.Errorf("nil resource charge"),
		}
	}

	if err := charge("string states", 1, explorerStateBytes(len(requirements))); err != nil {
		return nil, err
	}

	machines := make([]*dfa, len(requirements))
	for index := range requirements {
		if len(requirements[index].Language.dfa.states) == 0 {
			return nil, &CompileError{
				Operation: "create lazy explorer",
				Err:       fmt.Errorf("requirement %d has an invalid language", index),
			}
		}

		machines[index] = &requirements[index].Language.dfa
	}

	initial := explorerTuple{patterns: make([]uint32, len(machines))}
	explorer := &Explorer{
		machines:     machines,
		requirements: slices.Clone(requirements),
		length:       length,
		states:       []explorerTuple{initial},
		byKey:        map[string]State{explorerKey(initial): 0},
		transitions:  make(map[State][]ScalarRange),
		distance:     make(map[State]uint64),
		dead:         make(map[State]bool),
	}

	return explorer, nil
}

// Start returns the canonical initial product state.
func (*Explorer) Start() State {
	return 0
}

// Choices reports whether finish is available and the immediate productive scalar ranges.
// A shortest completion edge is first so an exhausted tape always terminates.
func (explorer *Explorer) Choices(
	state State,
	charge Charge,
) (bool, []ScalarRange, error) {
	if int(state) >= len(explorer.states) {
		return false, nil, &CompileError{
			Operation: "expand lazy explorer", Err: fmt.Errorf("unknown state %d", state),
		}
	}

	accepting := explorer.accepting(state)

	transitions, err := explorer.immediate(state, charge)
	if err != nil {
		return false, nil, err
	}

	productive := make([]ScalarRange, 0, len(transitions))
	for _, transition := range transitions {
		distance, possible, distanceErr := explorer.distanceToAcceptance(transition.Next, charge)
		if distanceErr != nil {
			return false, nil, distanceErr
		}

		if possible {
			transition.distance = distance
			productive = append(productive, transition)
		}
	}

	slices.SortStableFunc(productive, func(left ScalarRange, right ScalarRange) int {
		if left.distance != right.distance {
			if left.distance < right.distance {
				return -1
			}

			return 1
		}

		if left.First < right.First {
			return -1
		}

		if left.First > right.First {
			return 1
		}

		return 0
	})

	return accepting, productive, nil
}

func (explorer *Explorer) accepting(state State) bool {
	tuple := explorer.states[state]

	return productAccepts(
		productTuple(tuple),
		explorer.machines,
		explorer.requirements,
		explorer.length.Min,
		explorer.length.Max,
	)
}

func (explorer *Explorer) immediate(state State, charge Charge) ([]ScalarRange, error) {
	if existing, ok := explorer.transitions[state]; ok {
		return existing, nil
	}

	tuple := explorer.states[state]
	if explorer.length.Max != nil && tuple.length == *explorer.length.Max {
		explorer.transitions[state] = nil

		return nil, nil
	}

	alphabetBytes, err := explorer.alphabetBytes(tuple)
	if err != nil {
		return nil, err
	}

	if err := charge("string transitions", 1, alphabetBytes); err != nil {
		return nil, err
	}

	alphabet := productAlphabet(explorer.machines, tuple.patterns)
	result := make([]ScalarRange, 0, len(alphabet))

	for _, scalarClass := range alphabet {
		if err := charge("string transitions", 1, uint64(len(explorer.machines))*4+24); err != nil {
			return nil, err
		}

		next := explorerTuple{
			patterns: make([]uint32, len(explorer.machines)),
			length: nextLength(
				tuple.length,
				explorer.length.Min,
				explorer.length.Max,
			),
		}
		for index, machine := range explorer.machines {
			next.patterns[index] = advanceScalar(machine, tuple.patterns[index], scalarClass.first)
		}

		nextID, internErr := explorer.intern(next, charge)
		if internErr != nil {
			return nil, internErr
		}

		if len(result) != 0 {
			last := &result[len(result)-1]
			if last.Next == nextID && last.Last+1 == scalarClass.first {
				last.Last = scalarClass.last

				continue
			}
		}

		result = append(result, ScalarRange{
			First: scalarClass.first, Last: scalarClass.last, Next: nextID,
		})
	}

	explorer.transitions[state] = result

	return result, nil
}

func (explorer *Explorer) intern(tuple explorerTuple, charge Charge) (State, error) {
	key := explorerKey(tuple)
	if existing, ok := explorer.byKey[key]; ok {
		return existing, nil
	}

	if len(explorer.states) == int(^uint32(0)) {
		return 0, &ComplexityError{
			Phase: "lazy product", Resource: "state identifiers",
			Limit: ^uint64(0), Observed: ^uint64(0),
		}
	}

	if err := charge("string states", 1, explorerStateBytes(len(tuple.patterns))); err != nil {
		return 0, err
	}

	identifier := State(len(explorer.states))
	explorer.states = append(explorer.states, tuple)
	explorer.byKey[key] = identifier

	return identifier, nil
}

func (explorer *Explorer) distanceToAcceptance(
	start State,
	charge Charge,
) (uint64, bool, error) {
	if distance, known := explorer.distance[start]; known {
		return distance, true, nil
	}

	if explorer.dead[start] {
		return 0, false, nil
	}

	if explorer.accepting(start) {
		explorer.distance[start] = 0

		return 0, true, nil
	}

	queue := []State{start}
	parents := make(map[State]State)
	seen := map[State]bool{start: true}

	for head := 0; head < len(queue); head++ {
		current := queue[head]

		transitions, err := explorer.immediate(current, charge)
		if err != nil {
			return 0, false, err
		}

		for _, transition := range transitions {
			next := transition.Next
			if seen[next] || explorer.dead[next] {
				continue
			}

			if err := charge("string search", 1, 32); err != nil {
				return 0, false, err
			}

			parents[next] = current

			if knownDistance, known := explorer.distance[next]; known {
				distance := explorer.recordPath(start, next, parents, knownDistance)

				return distance, true, nil
			}

			if explorer.accepting(next) {
				explorer.distance[next] = 0
				distance := explorer.recordPath(start, next, parents, 0)

				return distance, true, nil
			}

			seen[next] = true
			queue = append(queue, next)
		}
	}

	for state := range seen {
		explorer.dead[state] = true
	}

	return 0, false, nil
}

func (explorer *Explorer) recordPath(
	start State,
	end State,
	parents map[State]State,
	distance uint64,
) uint64 {
	current := end
	for current != start {
		parent := parents[current]
		distance++
		explorer.distance[parent] = distance
		current = parent
	}

	return distance
}

func (explorer *Explorer) alphabetBytes(tuple explorerTuple) (uint64, error) {
	ranges := uint64(0)

	for index, machine := range explorer.machines {
		state := tuple.patterns[index]

		if !machine.utf16 {
			var ok bool

			ranges, ok = addExplorerAmount(ranges, uint64(len(machine.states[state].edges)))
			if !ok {
				return 0, &ComplexityError{
					Phase: "lazy product", Resource: "transition bytes",
					Limit: ^uint64(0), Observed: ^uint64(0),
				}
			}

			continue
		}

		for _, edge := range machine.states[state].edges {
			if edge.first <= firstSurrogate-1 && edge.last >= 0 ||
				edge.first <= maximumCodeUnit && edge.last >= lastSurrogate+1 {
				var ok bool

				ranges, ok = addExplorerAmount(ranges, 1)
				if !ok {
					return 0, &ComplexityError{
						Phase: "lazy product", Resource: "transition bytes",
						Limit: ^uint64(0), Observed: ^uint64(0),
					}
				}
			}
		}

		for high := rune(0xd800); high <= 0xdbff; high++ {
			afterHigh := machine.advance(state, high)
			for _, edge := range machine.states[afterHigh].edges {
				if edge.first > 0xdfff || edge.last < 0xdc00 {
					continue
				}

				var ok bool

				ranges, ok = addExplorerAmount(ranges, 1)
				if !ok {
					return 0, &ComplexityError{
						Phase: "lazy product", Resource: "transition bytes",
						Limit: ^uint64(0), Observed: ^uint64(0),
					}
				}
			}
		}
	}

	bytes, ok := multiplyExplorerAmount(ranges, 24)
	if !ok {
		return 0, &ComplexityError{
			Phase: "lazy product", Resource: "transition bytes",
			Limit: ^uint64(0), Observed: ^uint64(0),
		}
	}

	return bytes, nil
}

func explorerStateBytes(patterns int) uint64 {
	return 64 + uint64(patterns)*4
}

func explorerKey(tuple explorerTuple) string {
	key := binary.AppendUvarint(nil, uint64(tuple.length)+1)
	for _, state := range tuple.patterns {
		key = binary.AppendUvarint(key, uint64(state)+1)
	}

	return string(key)
}

func addExplorerAmount(left uint64, right uint64) (uint64, bool) {
	if right > ^uint64(0)-left {
		return ^uint64(0), false
	}

	return left + right, true
}

func multiplyExplorerAmount(left uint64, right uint64) (uint64, bool) {
	if left != 0 && right > ^uint64(0)/left {
		return ^uint64(0), false
	}

	return left * right, true
}
