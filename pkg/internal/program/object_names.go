//nolint:godoclint,mnd // Private canonical name ranking is independent of object rules.
package program

import (
	"math/big"
	"strings"
	"unicode/utf8"
)

const unicodeScalarCount = 0x110000 - 0x800

func chooseFiniteNames(available []string, count int, reader *tapeReader) []string {
	result := make([]string, 0, count)
	next := 0

	for remaining := count; remaining > 0; remaining-- {
		last := len(available) - remaining

		offset := 0
		if reader != nil {
			offset = int(reader.word() % uint64(last-next+1))
		}

		selected := next + offset
		result = append(result, available[selected])
		next = selected + 1
	}

	return result
}

func (program *Program) chooseInfiniteNames(
	count int,
	rules *objectRules,
	reader *tapeReader,
	work *decodeWork,
) ([]string, error) {
	result := make([]string, 0, count)
	previous := big.NewInt(-1)

	for range count {
		gap, err := readNatural(reader, work)
		if err != nil {
			return nil, err
		}

		rank := new(big.Int).Add(previous, big.NewInt(1))
		rank.Add(rank, gap)

		for {
			name, unrankErr := unrankName(rank, work)
			if unrankErr != nil {
				return nil, unrankErr
			}

			_, forced := rules.forced[name]
			_, absent := rules.absent[name]

			possible := false
			if !forced && !absent {
				possible, err = program.objectNamePossible(name, nil, rules, work)
				if err != nil {
					return nil, err
				}
			}

			if possible {
				result = append(result, name)
				previous = new(big.Int).Set(rank)

				break
			}

			if err := work.solver(uint64(len(rank.Bytes())) + 1); err != nil {
				return nil, err
			}

			rank.Add(rank, big.NewInt(1))
		}
	}

	return result, nil
}

func (program *Program) nextInfiniteNames(
	current []string,
	rules *objectRules,
	work *decodeWork,
) ([]string, error) {
	if len(current) == 0 {
		return nil, nil
	}

	result := appendCopy(current)

	rank, err := rankName(result[len(result)-1], work)
	if err != nil {
		return nil, err
	}

	for {
		if err := work.solver(uint64(len(rank.Bytes())) + 1); err != nil {
			return nil, err
		}

		rank.Add(rank, big.NewInt(1))

		name, unrankErr := unrankName(rank, work)
		if unrankErr != nil {
			return nil, unrankErr
		}

		if _, forced := rules.forced[name]; forced {
			continue
		}

		if _, absent := rules.absent[name]; absent {
			continue
		}

		possible, possibleErr := program.objectNamePossible(name, nil, rules, work)
		if possibleErr != nil {
			return nil, possibleErr
		}

		if !possible {
			continue
		}

		result[len(result)-1] = name

		return result, nil
	}
}

func unrankName(rank *big.Int, work *decodeWork) (string, error) {
	remaining := new(big.Int).Set(rank)
	base := big.NewInt(unicodeScalarCount)
	countAtLength := big.NewInt(1)
	length := 0

	for remaining.Cmp(countAtLength) >= 0 {
		remaining.Sub(remaining, countAtLength)
		countAtLength.Mul(countAtLength, base)

		length++

		if err := work.solver(uint64(len(countAtLength.Bytes())) + 8); err != nil {
			return "", err
		}

		if uint64(length) > work.limits.MaxOutputBytes {
			return "", &LimitError{
				Resource: "property name bytes",
				Limit:    work.limits.MaxOutputBytes,
				Observed: uint64(length),
			}
		}
	}

	digits := make([]rune, length)
	for index := length - 1; index >= 0; index-- {
		digit := new(big.Int)
		remaining.QuoRem(remaining, base, digit)

		ordinal := digit.Int64()
		if ordinal >= 0xd800 {
			ordinal += 0x800
		}

		digits[index] = rune(ordinal)
	}

	name := string(digits)
	if uint64(len(name)+2) > work.limits.MaxOutputBytes {
		return "", &LimitError{
			Resource: "property name bytes",
			Limit:    work.limits.MaxOutputBytes,
			Observed: uint64(len(name) + 2),
		}
	}

	return name, nil
}

func rankName(name string, work *decodeWork) (*big.Int, error) {
	base := big.NewInt(unicodeScalarCount)
	offset := new(big.Int)
	countAtLength := big.NewInt(1)

	for range utf8.RuneCountInString(name) {
		offset.Add(offset, countAtLength)
		countAtLength.Mul(countAtLength, base)

		if err := work.solver(uint64(len(countAtLength.Bytes())) + 8); err != nil {
			return nil, err
		}
	}

	digits := new(big.Int)

	for _, scalar := range name {
		ordinal := int64(scalar)
		if scalar >= 0xe000 {
			ordinal -= 0x800
		}

		digits.Mul(digits, base)
		digits.Add(digits, big.NewInt(ordinal))
	}

	return offset.Add(offset, digits), nil
}

func compareShortlex(left string, right string) int {
	leftLength := utf8.RuneCountInString(left)

	rightLength := utf8.RuneCountInString(right)
	if leftLength != rightLength {
		return leftLength - rightLength
	}

	return strings.Compare(left, right)
}
