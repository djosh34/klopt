//nolint:cyclop,godoclint // Closed format frontiers and constraint traversal are intentionally explicit.
package schematest

import (
	"fmt"
	"strings"
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

func basicStringFormatPattern(format schemaFormat) (string, bool) {
	specification, exists := stringFormatSpecificationFor(format)
	if !exists || specification.inert || specification.program == nil {
		return "", false
	}

	return specification.program.pattern, true
}

func (product *basicStringProduct) addFormats(formats []activeStringFormat, directed int) error {
	product.formats = formats
	product.directedFormat = directed
	product.formatPrograms = make([]*stringFormatProgram, len(formats))

	for index, format := range formats {
		specification, exists := stringFormatSpecificationFor(format.format)
		if !exists {
			return fmt.Errorf("schematest: format %d has no string specification", format.format)
		}

		if !specification.inert {
			product.formatPrograms[index] = specification.program
		}

		if index == directed {
			continue
		}

		source, ok := basicStringFormatPattern(format.format)
		if !ok {
			return fmt.Errorf("schematest: format %d has no string product", format.format)
		}

		pattern, err := parseECMAPattern(source)
		if err != nil {
			return fmt.Errorf("schematest: parse format %d product: %w", format.format, err)
		}

		machines, err := compileBasicStringPatternMachines(pattern)
		if err != nil {
			return fmt.Errorf("schematest: compile format %d product: %w", format.format, err)
		}

		product.machines = append(product.machines, machines...)
	}

	return product.setBounds()
}

func (product *basicStringProduct) formatsAllowLength(length uint64) bool {
	for index, format := range product.formats {
		if index == product.directedFormat {
			continue
		}

		specification, exists := stringFormatSpecificationFor(format.format)
		if !exists {
			return false
		}

		if !specification.inert && !specification.bounds.allows(length) {
			return false
		}
	}

	return true
}
