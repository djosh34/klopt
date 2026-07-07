package domain

import "decode_and_validate_generator/pkg/test_generator/hashables"

func toHashableNumbers(numbers []Number) []hashables.Number {
	if numbers == nil {
		return nil
	}

	hashableNumbers := make([]hashables.Number, 0, len(numbers))
	for _, number := range numbers {
		hashableNumbers = append(hashableNumbers, hashables.Number(number))
	}

	return hashableNumbers
}

func toHashableNumberPtr(number *Number) *hashables.Number {
	if number == nil {
		return nil
	}

	hashableNumber := hashables.Number(*number)
	return &hashableNumber
}
