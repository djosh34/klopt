// Package program compiles request schemas into one signed constraint graph and decodes byte tapes.
//
//nolint:godoclint // Private graph serialization stays behind Program.Fingerprint.
package program

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/validation"
)

// OperationID identifies one operation root in a document program.
type OperationID uint32

// ExpectedResult is the verdict requested by a decoded tape.
type ExpectedResult uint8

const (
	// ExpectValid asks the graph for a member of the operation schema.
	ExpectValid ExpectedResult = iota
	// ExpectInvalid asks the same graph for a member of its complement.
	ExpectInvalid
)

// Sample is one exact JSON value and the verdict selected by its tape.
type Sample struct {
	Operation OperationID
	Expect    ExpectedResult
	Value     jsonvalue.Value
}

// Program is one immutable document-wide signed constraint graph.
type Program struct {
	nodes       []node
	roots       []nodeID
	fingerprint [sha256.Size]byte
}

// Compile lowers operation request schemas into one immutable graph.
func Compile(roots []*validation.Validation) (*Program, error) {
	lower := graphLowerer{
		byValidation: make(map[*validation.Validation]nodeID),
		byAtom:       make(map[string]nodeID),
	}
	compiledRoots := make([]nodeID, len(roots))

	for index, root := range roots {
		if root == nil {
			return nil, fmt.Errorf("compile operation %d: request schema must not be nil", index)
		}

		compiled, err := lower.validation(root)
		if err != nil {
			return nil, fmt.Errorf("compile operation %d: %w", index, err)
		}

		compiledRoots[index] = compiled
	}

	compiled := &Program{nodes: lower.nodes, roots: compiledRoots}

	fingerprint, err := compiled.hash()
	if err != nil {
		return nil, err
	}

	compiled.fingerprint = fingerprint

	return compiled, nil
}

// Fingerprint identifies the immutable graph contents.
func (program *Program) Fingerprint() [sha256.Size]byte {
	if program == nil {
		return [sha256.Size]byte{}
	}

	return program.fingerprint
}

//nolint:cyclop // Stable graph serialization writes each atom field explicitly.
func (program *Program) hash() ([sha256.Size]byte, error) {
	encoded := appendUint64(nil, uint64(len(program.roots)))

	for _, root := range program.roots {
		encoded = appendUint64(encoded, uint64(root))
	}

	encoded = appendUint64(encoded, uint64(len(program.nodes)))

	for _, item := range program.nodes {
		encoded = append(encoded, byte(item.kind), byte(item.atom.kind))

		if item.atom.integer {
			encoded = append(encoded, 1)
		} else {
			encoded = append(encoded, 0)
		}

		encoded = appendUint64(encoded, uint64(len(item.children)))
		for _, child := range item.children {
			encoded = appendUint64(encoded, uint64(child))
		}

		encoded = appendUint64(encoded, item.atom.count)
		encoded = appendUint64(encoded, uint64(item.atom.child))
		encoded = appendBytes(encoded, []byte(item.atom.name))

		encoded = appendUint64(encoded, uint64(len(item.atom.names)))
		for _, name := range item.atom.names {
			encoded = appendBytes(encoded, []byte(name))
		}

		if item.atom.allowedAdditional {
			encoded = append(encoded, 1)
		} else {
			encoded = append(encoded, 0)
		}

		if item.atom.hasChild {
			encoded = append(encoded, 1)
		} else {
			encoded = append(encoded, 0)
		}

		encoded = appendBytes(encoded, []byte(item.atom.number.Lexeme))

		if item.atom.exclusive {
			encoded = append(encoded, 1)
		} else {
			encoded = append(encoded, 0)
		}

		encoded = appendBytes(encoded, []byte(item.atom.text))

		for _, allowed := range item.atom.allowed {
			if allowed {
				encoded = append(encoded, 1)
			} else {
				encoded = append(encoded, 0)
			}
		}

		encoded = appendUint64(encoded, uint64(len(item.atom.values)))
		for _, value := range item.atom.values {
			raw, err := value.MarshalJSON()
			if err != nil {
				return [sha256.Size]byte{}, fmt.Errorf("hash graph enum: %w", err)
			}

			encoded = appendBytes(encoded, raw)
		}
	}

	return sha256.Sum256(encoded), nil
}

func appendUint64(target []byte, value uint64) []byte {
	var encoded [8]byte
	binary.LittleEndian.PutUint64(encoded[:], value)

	return append(target, encoded[:]...)
}

func appendBytes(target []byte, value []byte) []byte {
	target = appendUint64(target, uint64(len(value)))

	return append(target, value...)
}
