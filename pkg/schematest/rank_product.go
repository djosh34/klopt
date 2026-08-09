package schematest

import (
	"errors"
	"fmt"
)

// rankProductCursor enumerates rank tuples by increasing sum and canonical
// lexicographic projection. It retains only the current tuple and finite domain
// endpoints; candidate values remain owned by their individual cursors.
type rankProductCursor struct {
	tuple       []uint64
	finiteSizes []uint64
	finite      []bool
	diagonal    uint64
	started     bool
	exhausted   bool
}

// newRankProductCursor creates one product of open rank domains.
func newRankProductCursor(dimensions int) (*rankProductCursor, error) {
	if dimensions <= 0 {
		return nil, errors.New("schematest: rank product requires at least one dimension")
	}

	return &rankProductCursor{
		tuple:       make([]uint64, dimensions),
		finiteSizes: make([]uint64, dimensions),
		finite:      make([]bool, dimensions),
	}, nil
}

// SetFinite records the full size of one exhausted domain.
func (cursor *rankProductCursor) SetFinite(dimension int, size uint64) error {
	if cursor == nil || dimension < 0 || dimension >= len(cursor.tuple) {
		return errors.New("schematest: invalid rank product dimension")
	}

	if cursor.finite[dimension] && cursor.finiteSizes[dimension] != size {
		return fmt.Errorf(
			"schematest: rank product domain %d size changed from %d to %d",
			dimension, cursor.finiteSizes[dimension], size,
		)
	}

	cursor.finite[dimension] = true

	cursor.finiteSizes[dimension] = size
	if size == 0 {
		cursor.exhausted = true
	}

	return nil
}

// Next returns the next tuple. The tuple is valid until the next call.
func (cursor *rankProductCursor) Next() ([]uint64, bool) {
	if cursor == nil || cursor.exhausted {
		return nil, false
	}

	if !cursor.started {
		cursor.started = true
	} else if !cursor.advanceTuple() {
		cursor.exhausted = true

		return nil, false
	}

	for {
		if cursor.tupleFits() {
			return cursor.tuple, true
		}

		if !cursor.advanceTuple() {
			cursor.exhausted = true

			return nil, false
		}
	}
}

// tupleFits skips ranks beyond known finite domains.
func (cursor *rankProductCursor) tupleFits() bool {
	for index, rank := range cursor.tuple {
		if cursor.finite[index] && rank >= cursor.finiteSizes[index] {
			return false
		}
	}

	return true
}

// advanceTuple advances within one diagonal, then starts the next diagonal.
func (cursor *rankProductCursor) advanceTuple() bool {
	last := len(cursor.tuple) - 1
	for index := last - 1; index >= 0; index-- {
		var suffix uint64
		for _, rank := range cursor.tuple[index+1:] {
			suffix += rank
		}

		if suffix == 0 {
			continue
		}

		cursor.tuple[index]++
		for reset := index + 1; reset < len(cursor.tuple)-1; reset++ {
			cursor.tuple[reset] = 0
		}

		cursor.tuple[len(cursor.tuple)-1] = suffix - 1

		return true
	}

	if cursor.allFinite() && cursor.diagonal >= cursor.maximumFiniteDiagonal() || cursor.diagonal == ^uint64(0) {
		return false
	}

	cursor.diagonal++
	clear(cursor.tuple)
	cursor.tuple[len(cursor.tuple)-1] = cursor.diagonal

	return true
}

// allFinite reports whether every component domain has reached its endpoint.
func (cursor *rankProductCursor) allFinite() bool {
	for _, finite := range cursor.finite {
		if !finite {
			return false
		}
	}

	return true
}

// maximumFiniteDiagonal returns the last sum reachable by the finite domains.
func (cursor *rankProductCursor) maximumFiniteDiagonal() uint64 {
	var maximum uint64

	for _, size := range cursor.finiteSizes {
		addend := size - 1
		if ^uint64(0)-maximum < addend {
			return ^uint64(0)
		}

		maximum += addend
	}

	return maximum
}
