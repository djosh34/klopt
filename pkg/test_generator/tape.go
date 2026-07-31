//nolint:godoclint,mnd // The private cursor and exact eight-byte word are the contract.
package testgenerator

import "errors"

const tapeWordBytes = 8

type tapeCursor struct {
	tape   []byte
	offset int
}

func newTapeCursor(tape []byte) *tapeCursor {
	return &tapeCursor{tape: tape}
}

func (cursor *tapeCursor) takeByte() byte {
	if cursor.offset >= len(cursor.tape) {
		return 0
	}

	value := cursor.tape[cursor.offset]
	cursor.offset++

	return value
}

func (cursor *tapeCursor) takeWord() uint64 {
	var value uint64
	for index := 0; index < tapeWordBytes; index++ {
		value |= uint64(cursor.takeByte()) << (8 * uint(index))
	}

	return value
}

func (cursor *tapeCursor) choose(count int) (int, error) {
	if count <= 0 {
		return 0, errors.New("choose called with no alternatives")
	}

	return int(cursor.takeWord() % uint64(count)), nil
}
