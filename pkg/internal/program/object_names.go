//nolint:cyclop,godoclint,mnd // Canonical Unicode ranking is one explicit digit walk.
package program

import (
	"math/big"
	"slices"
	"strings"
	"unicode/utf8"
)

const unicodeScalarCount = 0x110000 - 0x800

func (program *Program) objectCandidateNames(rules *objectRules) ([]string, bool) {
	if finite, ok := rules.finiteNames(); ok {
		return finite, true
	}

	set := make(map[string]struct{})

	for _, current := range rules.goals {
		item := program.nodes[current.node].atom
		if item.kind == atomObjectProperty {
			set[item.name] = struct{}{}
		}
	}

	for name := range rules.forced {
		set[name] = struct{}{}
	}

	result := sortedObjectNames(set)

	return result, false
}

func (program *Program) chooseDynamicObjectName(
	previous string,
	hasPrevious bool,
	upper string,
	rules *objectRules,
	reader *tapeReader,
	work *decodeWork,
) (string, bool, error) {
	lower := new(big.Int)

	if hasPrevious {
		rank, err := rankName(previous, work)
		if err != nil {
			return "", false, err
		}

		lower.Add(rank, big.NewInt(1))
	}

	var upperRank *big.Int

	if upper != "" {
		var err error

		upperRank, err = rankName(upper, work)
		if err != nil {
			return "", false, err
		}

		if lower.Cmp(upperRank) >= 0 {
			return "", false, nil
		}
	}

	offset, err := readNatural(reader, work)
	if err != nil {
		return "", false, err
	}

	if upperRank != nil {
		span := new(big.Int).Sub(upperRank, lower)
		offset.Mod(offset, span)
	}

	rank := new(big.Int).Add(lower, offset)
	known, _ := program.objectCandidateNames(rules)

	for upperRank == nil || rank.Cmp(upperRank) < 0 {
		name, unrankErr := unrankName(rank, work)
		if unrankErr != nil {
			return "", false, unrankErr
		}

		_, absent := rules.absent[name]

		_, forced := rules.forced[name]
		if !absent && !forced && !slices.Contains(known, name) {
			possible, possibleErr := program.objectNamePossible(name, nil, rules, work)
			if possibleErr != nil {
				return "", false, possibleErr
			}

			if possible {
				return name, true, nil
			}
		}

		rankBytes, ok := checkedAdd(uint64(len(rank.Bytes())), 1)
		if !ok {
			return "", false, &ResourceError{
				Resource: "object name rank bytes", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		if err := work.solver(rankBytes); err != nil {
			return "", false, err
		}

		rank.Add(rank, big.NewInt(1))
	}

	return "", false, nil
}

func (program *Program) objectOptionalCapacity(
	current objectState,
	rules *objectRules,
	work *decodeWork,
) (uint64, bool, error) {
	names, finite := program.objectCandidateNames(rules)

	forced := make(map[string]struct{}, len(current.remainingForced))
	for _, name := range current.remainingForced {
		forced[name] = struct{}{}
	}

	count := uint64(0)

	for _, name := range names {
		if current.hasPrevious && compareShortlex(name, current.previous) <= 0 {
			continue
		}

		if _, required := forced[name]; required {
			continue
		}

		if _, absent := rules.absent[name]; absent {
			continue
		}

		possible, err := program.objectNamePossible(name, nil, rules, work)
		if err != nil {
			return 0, false, err
		}

		if possible {
			count++
		}
	}

	if finite {
		return count, false, nil
	}

	_, dynamic, err := program.chooseDynamicObjectName(
		current.previous, current.hasPrevious, "", rules, nil, work,
	)

	return count, dynamic, err
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

		workingBytes, ok := checkedAdd(uint64(len(countAtLength.Bytes())), 8)
		if !ok {
			return "", &ResourceError{
				Resource: "object name bytes", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		if err := work.solver(workingBytes); err != nil {
			return "", err
		}

		if uint64(length) > work.limits.MaxOutputBytes {
			return "", &LimitError{
				Resource: "property name bytes", Limit: work.limits.MaxOutputBytes,
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

	nameBytes, ok := checkedAdd(uint64(len(name)), 2)
	if !ok {
		return "", &LimitError{
			Resource: "property name bytes", Limit: work.limits.MaxOutputBytes,
			Observed: ^uint64(0),
		}
	}

	if nameBytes > work.limits.MaxOutputBytes {
		return "", &LimitError{
			Resource: "property name bytes", Limit: work.limits.MaxOutputBytes,
			Observed: nameBytes,
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

		workingBytes, ok := checkedAdd(uint64(len(countAtLength.Bytes())), 8)
		if !ok {
			return nil, &ResourceError{
				Resource: "object name bytes", Limit: work.limits.MaxSolverBytes,
				Observed: ^uint64(0),
			}
		}

		if err := work.solver(workingBytes); err != nil {
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
