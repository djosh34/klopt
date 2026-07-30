//nolint:godoclint,mnd // Private weights are provisional sampling policy.
package program

type faultStyle uint8

const (
	faultBoundary faultStyle = iota
	faultStructural
	faultWrongKind
)

type faultState struct {
	budget uint32
	style  faultStyle
}

func readFaultState(reader *tapeReader) faultState {
	budgetChoice := reader.word() % 16

	budget := uint32(1)
	if budgetChoice >= 12 {
		budget = 2
	}

	if budgetChoice == 15 {
		budget = 3
	}

	styleChoice := reader.word() % 16

	style := faultBoundary
	if styleChoice >= 12 {
		style = faultStructural
	}

	if styleChoice == 15 {
		style = faultWrongKind
	}

	return faultState{budget: budget, style: style}
}
