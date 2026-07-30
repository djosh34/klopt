// Package program executes immutable schema-free value-generation graphs.
//
//nolint:godoclint // Private sealed graph vocabulary stays behind Program.
package program

import (
	"math/big"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

// NodeID identifies one builder node.
type NodeID uint32

// ScalarRange is one inclusive range of Unicode scalar values.
type ScalarRange struct {
	First rune
	Last  rune
}

// SamplingTable assigns one positive weight to each builder transition.
type SamplingTable struct {
	Weights []uint32
}

type transitionKind uint8

const (
	transitionScalar transitionKind = iota
	transitionStringStop
	transitionBeginString
	transitionExactValue
	transitionInteger
	transitionBeginArray
	transitionBeginObject
	transitionArrayItem
	transitionArraySequence
	transitionObjectMember
	transitionStop
)

type transition struct {
	kind       transitionKind
	next       NodeID
	child      NodeID
	resume     NodeID
	name       string
	nameJSON   []byte
	ranges     []ScalarRange
	valueJSON  []byte
	valueDepth uint64
	minimum    *big.Int
	maximum    *big.Int
	weight     uint32
}

type node struct {
	outgoing []uint32
}

type completionFact struct {
	productive bool
	cost       uint64
	first      uint32
}

// Program is one immutable, certified generation graph with bound sampling weights.
type Program struct {
	fingerprint [32]byte
	root        NodeID
	nodes       []node
	transitions []transition
	completion  []completionFact
}

// Fingerprint identifies the sealed graph and its bound sampling table.
func (program *Program) Fingerprint() [32]byte {
	if program == nil {
		return [32]byte{}
	}

	return program.fingerprint
}

// Decode deterministically executes tape and returns a complete JSON value.
func (program *Program) Decode(tape []byte, limits Limits) (jsonvalue.Value, error) {
	return program.decode(tape, limits)
}
