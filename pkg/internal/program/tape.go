//nolint:godoclint // Private fixed-width tape mechanics stay behind Decode.
package program

import (
	"encoding/binary"
	"math/big"
)

const decisionWordBytes = 8

type tapeReader struct {
	tape   []byte
	offset int
}

func (reader *tapeReader) word() uint64 {
	if reader.offset >= len(reader.tape) {
		return 0
	}

	remaining := reader.tape[reader.offset:]
	if len(remaining) >= decisionWordBytes {
		reader.offset += decisionWordBytes

		return binary.LittleEndian.Uint64(remaining[:decisionWordBytes])
	}

	var padded [decisionWordBytes]byte
	copy(padded[:], remaining)

	reader.offset = len(reader.tape)

	return binary.LittleEndian.Uint64(padded[:])
}

func (reader *tapeReader) natural(charge func() error) (*big.Int, error) {
	result := new(big.Int)
	shift := uint(0)

	for {
		if err := charge(); err != nil {
			return nil, err
		}

		word := reader.word()

		payload := word & (^uint64(0) >> 1)
		if payload != 0 {
			part := new(big.Int).SetUint64(payload)
			part.Lsh(part, shift)
			result.Or(result, part)
		}

		if word>>63 == 0 {
			return result, nil
		}

		shift += 63
	}
}
