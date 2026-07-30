//nolint:godoclint // Private representation constants stay behind lowering metrics.
package stringlanguage

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/internal/program" //nolint:depguard // The hosted plan requires this one lowering seam.
)

const (
	loweredNodeBytes  = 16
	loweredEdgeBytes  = 24
	loweredRangeBytes = 8
)

// LoweringMetrics describes the immutable Program representation appended by Set.
type LoweringMetrics struct {
	Nodes          uint64
	Transitions    uint64
	Bytes          uint64
	UnicodeClasses uint64
}

// ProgramMetrics returns the exact node, transition, and scalar-class counts for AppendTo.
func (set Set) ProgramMetrics() (LoweringMetrics, error) {
	if len(set.product.states) == 0 || len(set.product.shortest) != len(set.product.states) {
		return LoweringMetrics{}, fmt.Errorf("measure invalid string language set")
	}

	metrics := LoweringMetrics{Nodes: uint64(len(set.product.states))}
	for _, state := range set.product.states {
		if state.accepting {
			metrics.Transitions++
		}

		for _, edge := range state.edges {
			if set.product.shortest[edge.next] < 0 {
				continue
			}

			metrics.Transitions++
			metrics.UnicodeClasses += uint64(len(edge.ranges))
		}
	}

	metrics.Bytes = metrics.Nodes*loweredNodeBytes + metrics.Transitions*loweredEdgeBytes +
		metrics.UnicodeClasses*loweredRangeBytes

	return metrics, nil
}

// AppendTo appends the exact language as scalar-range and string-stop transitions.
//
//nolint:cyclop // The private automaton walk validates and appends one transaction.
func (set Set) AppendTo(builder *program.Builder) (program.NodeID, error) {
	if builder == nil {
		return 0, fmt.Errorf("append string language to nil program builder")
	}

	if len(set.product.states) == 0 || len(set.product.shortest) != len(set.product.states) {
		return 0, fmt.Errorf("append invalid string language set")
	}

	nodes := make([]program.NodeID, len(set.product.states))
	for index := range nodes {
		nodes[index] = builder.AddNode()
	}

	for stateID, state := range set.product.states {
		if state.accepting {
			if err := builder.AddStringStop(nodes[stateID]); err != nil {
				return 0, fmt.Errorf("append string stop: %w", err)
			}
		}

		for _, edge := range state.edges {
			if set.product.shortest[edge.next] < 0 {
				continue
			}

			ranges := make([]program.ScalarRange, len(edge.ranges))
			for index, item := range edge.ranges {
				ranges[index] = program.ScalarRange{First: item.first, Last: item.last}
			}

			if err := builder.AddScalarRanges(nodes[stateID], ranges, nodes[edge.next]); err != nil {
				return 0, fmt.Errorf("append scalar ranges: %w", err)
			}
		}
	}

	return nodes[0], nil
}
