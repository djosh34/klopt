//nolint:godoclint,mnd // Private accepted-path mechanics and fixed SplitMix64 constants are local details.
package stringlanguage

import "slices"

type drawAction struct {
	stop  bool
	next  uint32
	bytes []byte
}

func (graph *product) generate(seed uint64) string {
	state := uint32(0)
	shortest := graph.shortest[state]
	extraMaximum := min(maximumExtraLength, maximumGeneratedBytes-shortest)
	random := splitMix64{state: seed}
	fuel := shortest + random.intn(extraMaximum+1)
	value := make([]byte, 0, fuel)

	for {
		actions := graph.legalActions(state, fuel)

		action := actions[random.intn(len(actions))]
		if action.stop {
			return string(value)
		}

		value = append(value, action.bytes[random.intn(len(action.bytes))])
		state = action.next
		fuel--
	}
}

func (graph *product) legalActions(state uint32, fuel int) []drawAction {
	actions := make([]drawAction, 0, len(graph.states[state].edges)+1)
	if graph.states[state].accepting {
		actions = append(actions, drawAction{stop: true})
	}

	if fuel == 0 {
		return actions
	}

	edges := append([]productEdge(nil), graph.states[state].edges...)
	slices.SortFunc(edges, func(left productEdge, right productEdge) int {
		leftDistance := graph.shortest[left.next]

		rightDistance := graph.shortest[right.next]
		if comparison := leftDistance - rightDistance; comparison != 0 {
			return comparison
		}

		return int(left.bytes.values()[0]) - int(right.bytes.values()[0])
	})

	for _, edge := range edges {
		distance := graph.shortest[edge.next]
		if distance < 0 || distance > fuel-1 {
			continue
		}

		actions = append(actions, drawAction{next: edge.next, bytes: edge.bytes.values()})
	}

	return actions
}

type splitMix64 struct {
	state uint64
}

func (random *splitMix64) next() uint64 {
	random.state += 0x9e3779b97f4a7c15
	value := random.state
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb

	return value ^ (value >> 31)
}

func (random *splitMix64) intn(limit int) int {
	return int(random.next() % uint64(limit))
}
