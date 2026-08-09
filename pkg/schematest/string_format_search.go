//nolint:godoclint // Closed format frontiers and constraint traversal are intentionally explicit.
package schematest

import (
	"errors"
	"fmt"
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

func stringFormatNegativeWitnesses(format schemaFormat) []string {
	specification, exists := stringFormatSpecificationFor(format)
	if !exists || specification.inert {
		return nil
	}

	for index := 0; index < int(specification.registrationCount); index++ {
		if specification.formats[index] == format {
			count := specification.negativeCounts[index]

			return append([]string(nil), specification.negativeObjectives[index][:count]...)
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
	}

	return product.setBounds()
}

func (product *basicStringProduct) setFormatObjective(objective *formatBoundaryObjective) error {
	if objective == nil {
		return nil
	}

	for index, active := range product.formats {
		if rowOccurrenceMatches(active.occurrence, objective.identity.occurrence) {
			product.objective = &objective.boundary
			product.objectiveFormat = index

			return product.setBounds()
		}
	}

	return errors.New("schematest: format objective has no active format")
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
