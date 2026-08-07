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
	date := `[0-9]{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12][0-9]|3[01])`
	octet := `(?:0|[1-9][0-9]?|1[0-9]{2}|2[0-4][0-9]|25[0-5])`
	emailAtext := `[A-Za-z0-9!#$%&'*+/=?^_\x60{|}~-]`
	emailLocal := `(?:` + emailAtext + `|\.){1,64}`
	emailQuoted := `"(?:[\x20-\x21\x23-\x5b\x5d-\x7e]|\\[\x20-\x7e]){0,62}"`
	emailLabel := `[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?`
	emailDomain := emailLabel + `(?:\.` + emailLabel + `)*`
	emailLiteral := `\[(?:[0-9]{1,3}(?:\.[0-9]{1,3}){3}|` +
		`[Ii][Pp][Vv]6:[0-9A-Fa-f:.]+|[A-Za-z0-9-]+:[\x21-\x5a\x5e-\x7e]+)\]`

	switch format {
	case schemaFormatByte:
		return `^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`, true
	case schemaFormatDate:
		return `^` + date + `$`, true
	case schemaFormatDateTime:
		return `^` + date + `T(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]` +
			`(?:\.[0-9]+)?(?:Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])$`, true
	case schemaFormatEmail:
		return `^(?:` + emailLocal + `|` + emailQuoted + `)@(?:` + emailDomain + `|` + emailLiteral + `)$`, true
	case schemaFormatIPv4:
		return `^` + octet + `\.` + octet + `\.` + octet + `\.` + octet + `$`, true
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return `^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-4[0-9A-Fa-f]{3}-[89ABab][0-9A-Fa-f]{3}-[0-9A-Fa-f]{12}$`, true
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return `^` + octet + `\.` + octet + `\.` + octet + `\.` + octet +
			`/(?:0|[1-9]|[12][0-9]|3[0-2])$`, true
	default:
		return "", false
	}
}

func (product *basicStringProduct) addFormats(formats []activeStringFormat, directed int) error {
	product.formats = formats
	product.directedFormat = directed

	for index, format := range formats {
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

//nolint:mnd // Retained format grammar lengths are normative.
func (product *basicStringProduct) formatsAllowLength(length uint64) bool {
	for index, format := range product.formats {
		if index == product.directedFormat {
			continue
		}

		allowed := true

		switch format.format {
		case schemaFormatByte:
			allowed = length%4 == 0
		case schemaFormatDate:
			allowed = length == 10
		case schemaFormatDateTime:
			allowed = length >= 20
		case schemaFormatEmail:
			allowed = length >= 3 && length <= 254
		case schemaFormatIPv4:
			allowed = length >= 7 && length <= 15
		case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
			allowed = length == 36
		case schemaFormatCIDR, schemaFormatIPv4CIDR:
			allowed = length >= 9 && length <= 18
		}

		if !allowed {
			return false
		}
	}

	return true
}

func (product *basicStringProduct) formatsAccept(candidate string) (bool, error) {
	for index, format := range product.formats {
		matches, err := cleanStringFormatMatches(candidate, format.format)
		if err != nil {
			return false, err
		}

		if matches == (index == product.directedFormat) {
			return false, nil
		}
	}

	return true, nil
}
