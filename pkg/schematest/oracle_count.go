package schematest

import "strconv"

// countBoundViolated compares one collection count with an exact lower or upper bound.
func countBoundViolated(count int, bound *exactCount, minimum bool) (bool, error) {
	actual, err := parseExactNumber(strconv.Itoa(count))
	if err != nil {
		return false, err
	}

	comparison, err := actual.compare(bound.number)
	if err != nil {
		return false, err
	}

	if minimum {
		return comparison < 0, nil
	}

	return comparison > 0, nil
}
