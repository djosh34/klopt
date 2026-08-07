//nolint:cyclop,godoclint // Closed format frontiers and constraint traversal are intentionally explicit.
package schematest

import (
	"errors"
	"strings"
	"unicode/utf8"
)

type activeStringFormat struct {
	format     schemaFormat
	node       *schemaNode
	occurrence schemaOccurrence
}

const (
	emailLocalLimit              = 64
	emailDomainLabelLimit        = 63
	emailFinalBoundaryLabelLimit = 61
)

// simpleStringFormatWitnesses returns the finite canonical frontier for the
// retained formats.
func simpleStringFormatWitnesses(format schemaFormat, valid bool) []string {
	if valid {
		switch format {
		case schemaFormatByte:
			return []string{"YQ=="}
		case schemaFormatDate:
			return []string{"1970-01-01", "2000-02-29", "1900-02-28", "9999-12-31"}
		case schemaFormatDateTime:
			return []string{
				"1970-01-01T00:00:00Z",
				"2000-02-29T23:59:59.0Z",
				"1900-02-28T00:00:00+23:59",
				"9999-12-31T23:59:59-23:59",
			}
		case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
			return []string{"00000000-0000-4000-8000-000000000000"}
		case schemaFormatEmail:
			return []string{
				"a@b",
				strings.Repeat("a", emailLocalLimit) + "@b",
				strings.Repeat("a", emailLocalLimit) + "@" + strings.Repeat("b", emailDomainLabelLimit) + "." +
					strings.Repeat("c", emailDomainLabelLimit) + "." +
					strings.Repeat("d", emailFinalBoundaryLabelLimit),
			}
		case schemaFormatIPv4:
			return []string{"0.0.0.0", "255.255.255.255"}
		case schemaFormatCIDR, schemaFormatIPv4CIDR:
			return []string{"192.0.2.7/0", "192.0.2.7/32"}
		default:
			return nil
		}
	}

	switch format {
	case schemaFormatByte:
		return []string{"YQ="}
	case schemaFormatDate:
		return []string{"2001-02-29", "1900-02-29", "1970-13-01", "1970-01-32"}
	case schemaFormatDateTime:
		return []string{
			"1970-01-01t00:00:00Z",
			"1970-01-01T00:00:60Z",
			"1970-01-01T00:00:00.Z",
			"1970-01-01T00:00:00+24:00",
		}
	case schemaFormatUUID:
		return []string{"00000000-0000-1000-8000-000000000000"}
	case schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return []string{"00000000-0000-4000-7000-000000000000"}
	case schemaFormatEmail:
		return []string{
			"a..b@example.com",
			strings.Repeat("a", emailLocalLimit+1) + "@b",
			strings.Repeat("a", emailLocalLimit) + "@" + strings.Repeat("b", emailDomainLabelLimit) + "." +
				strings.Repeat("c", emailDomainLabelLimit) + "." +
				strings.Repeat("d", emailFinalBoundaryLabelLimit+1),
			"é@example.com",
		}
	case schemaFormatIPv4:
		return []string{"00.0.0.0", "256.255.255.255"}
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return []string{"192.0.2.7/33", "192.0.2.7/00"}
	default:
		return nil
	}
}

func collectActiveStringFormats(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	objective *stringSearchObjective,
	formats *[]activeStringFormat,
) error {
	if node == nil || node.schemaShape == nil {
		return errors.New("schematest: string format schema has no shape")
	}

	if len(simpleStringFormatWitnesses(node.format, true)) > 0 {
		*formats = append(*formats, activeStringFormat{
			format: node.format, node: node, occurrence: occurrence,
		})
	}

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if err := collectActiveStringFormats(child, childOccurrence, pins, objective, formats); err != nil {
			return err
		}
	}

	states, pinned := rowCompositionTruthStates(pins, occurrence, "anyOf", len(node.anyOf))
	for index, child := range node.anyOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence.usePointer+"/anyOf/"+itoa(index),
			occurrence.instanceTemplate,
		)
		if pinned && !states[index] && !stringObjectiveWithin(objective, childOccurrence) {
			continue
		}

		if err := collectActiveStringFormats(child, childOccurrence, pins, objective, formats); err != nil {
			return err
		}
	}

	return nil
}

func directedStringFormatIndex(formats []activeStringFormat, objective *stringSearchObjective) int {
	for index, format := range formats {
		if rowOccurrenceMatches(format.occurrence, objective.occurrence) {
			return index
		}
	}

	return -1
}

func basicStringLengthsFromActive(constraints []activeStringLength) (basicStringLengths, error) {
	var lengths basicStringLengths

	for _, constraint := range constraints {
		var err error
		if constraint.minimum {
			err = lengths.addMinimum(constraint.count)
		} else {
			err = lengths.addMaximum(constraint.count)
		}

		if err != nil {
			return basicStringLengths{}, err
		}
	}

	return lengths, nil
}

func simpleStringCandidateAllowed(
	candidate string,
	patterns []activeStringPattern,
	lengths []activeStringLength,
	formats []activeStringFormat,
	directedFormat int,
) (bool, error) {
	for _, pattern := range patterns {
		matches, err := cleanPatternMatches(pattern.pattern, candidate)
		if err != nil {
			return false, err
		}

		if !matches {
			return false, nil
		}
	}

	length := utf8.RuneCountInString(candidate)

	for _, constraint := range lengths {
		bound, fits, err := exactCountUint64(constraint.count)
		if err != nil {
			return false, err
		}

		if constraint.minimum {
			if !fits || uint64(length) < bound {
				return false, nil
			}
		} else if fits && uint64(length) > bound {
			return false, nil
		}
	}

	for index, format := range formats {
		matches, err := cleanStringFormatMatches(candidate, format.format)
		if err != nil {
			return false, err
		}

		if matches == (index == directedFormat) {
			return false, nil
		}
	}

	return true, nil
}

func (s *search) walkActiveSimpleStringFormats(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	visit rowVisit,
) (bool, error) {
	formats := make([]activeStringFormat, 0)
	if err := collectActiveStringFormats(node, occurrence, pins, nil, &formats); err != nil {
		return false, err
	}

	if len(formats) == 0 {
		return false, nil
	}

	patterns := make([]activeStringPattern, 0)

	lengths := make([]activeStringLength, 0)
	if err := collectActiveStringConstraints(node, occurrence, pins, nil, &patterns, &lengths); err != nil {
		return false, err
	}

	return s.walkSimpleStringFormatCandidates(patterns, lengths, formats, -1, visit)
}

func (s *search) walkSimpleStringFormatCandidates(
	patterns []activeStringPattern,
	lengths []activeStringLength,
	formats []activeStringFormat,
	directedFormat int,
	visit rowVisit,
) (bool, error) {
	candidates := make([]string, 0)
	if directedFormat >= 0 {
		candidates = append(candidates, simpleStringFormatWitnesses(formats[directedFormat].format, false)...)
	} else {
		for _, format := range formats {
			candidates = append(candidates, simpleStringFormatWitnesses(format.format, true)...)
		}
	}

	for index, candidate := range candidates {
		duplicate := false

		for _, earlier := range candidates[:index] {
			if earlier == candidate {
				duplicate = true

				break
			}
		}

		if duplicate {
			continue
		}

		if err := s.assign(); err != nil {
			return false, err
		}

		allowed, err := simpleStringCandidateAllowed(candidate, patterns, lengths, formats, directedFormat)
		if err != nil {
			return false, err
		}

		if !allowed {
			continue
		}

		complete, err := visit(&jsonValue{kind: jsonString, text: candidate})
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}
