//nolint:cyclop,godoclint // Closed format frontiers and constraint traversal are intentionally explicit.
package schematest

import (
	"encoding/base64"
	"strings"
	"unicode/utf16"
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
			return []string{"1970-01-01", "2000-02-29", "1900-02-28", "9999-12-31", "2024-01-01"}
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

//nolint:gocognit,mnd // Each retained format has a small explicit length construction.
func stringFormatWitnessAtLength(format schemaFormat, length uint64) (string, bool) {
	if length > uint64(^uint(0)>>1) {
		return "", false
	}

	size := int(length)

	switch format {
	case schemaFormatByte:
		if size == 0 {
			return "", true
		}

		if size%4 != 0 {
			return "", false
		}

		decodedLength := size/4*3 - 2
		for decodedLength <= size/4*3 {
			candidate := base64.StdEncoding.EncodeToString(make([]byte, decodedLength))
			if len(candidate) == size {
				return candidate, true
			}

			decodedLength++
		}
	case schemaFormatDate:
		if size == len("1970-01-01") {
			return "1970-01-01", true
		}
	case schemaFormatDateTime:
		if size == len("1970-01-01T00:00:00Z") {
			return "1970-01-01T00:00:00Z", true
		}

		if size >= len("1970-01-01T00:00:00.0Z") {
			return "1970-01-01T00:00:00." + strings.Repeat("0", size-21) + "Z", true
		}
	case schemaFormatEmail:
		if size >= 3 {
			return "a@" + strings.Repeat("b", size-2), true
		}
	case schemaFormatIPv4:
		if size >= 7 && size <= 15 {
			remaining := size - 3

			parts := make([]string, 4)
			for index := range parts {
				digits := min(3, remaining-(len(parts)-index-1))
				switch digits {
				case 1:
					parts[index] = "0"
				case 2:
					parts[index] = "10"
				case 3:
					parts[index] = "100"
				}

				remaining -= digits
			}

			return strings.Join(parts, "."), true
		}
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		if size == 36 {
			return "00000000-0000-4000-8000-000000000000", true
		}
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		for _, candidate := range []string{"0.0.0.0/0", "192.0.2.7/32"} {
			if len(candidate) == size {
				return candidate, true
			}
		}
	}

	return "", false
}

//nolint:gocognit // Candidate construction, deduplication, edge charging, and product traversal are one phase.
func (s *search) walkBasicStringFormatCandidates(
	product *basicStringProduct,
	runeLength uint64,
	_ uint64,
	visit rowVisit,
) (bool, error) {
	if len(product.formats) == 0 || runeLength > s.maxSteps-s.steps {
		return false, nil
	}

	candidates := make([]string, 0, len(product.formats))
	for index, format := range product.formats {
		valid := index != product.directedFormat
		if valid {
			candidate, exists := stringFormatWitnessAtLength(format.format, runeLength)
			if exists {
				candidates = append(candidates, candidate)
			}

			for _, candidate := range simpleStringFormatWitnesses(format.format, true) {
				if uint64(utf8.RuneCountInString(candidate)) == runeLength {
					candidates = append(candidates, candidate)
				}
			}

			continue
		}

		for _, candidate := range simpleStringFormatWitnesses(format.format, false) {
			if uint64(utf8.RuneCountInString(candidate)) == runeLength {
				candidates = append(candidates, candidate)
			}
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

		allowed, err := simpleStringCandidateAllowed(candidate, product.formats, product.directedFormat)
		if err != nil {
			return false, err
		}

		if !allowed {
			continue
		}

		units := utf16.Encode([]rune(candidate))

		state := product.start(len(units))
		for position, unit := range units {
			if assignErr := s.assign(); assignErr != nil {
				return false, assignErr
			}

			state = product.advance(state, unit, position, len(units))
		}

		if !product.accepting(state) {
			continue
		}

		complete, err := visit(&jsonValue{kind: jsonString, text: candidate})
		if err != nil || complete {
			return complete, err
		}
	}

	return false, nil
}

func simpleStringCandidateAllowed(
	candidate string,
	formats []activeStringFormat,
	directedFormat int,
) (bool, error) {
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
