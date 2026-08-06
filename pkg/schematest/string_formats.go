//nolint:cyclop,godoclint,lll,mnd,wsl_v5 // Private format transitions spell out the locked Delivery 1 languages.
package schematest

import (
	"sort"
	"strings"
)

func stringFormatIntervals(format schemaFormat, prefix []uint16) []stringUnitInterval {
	if !stringPrefixASCII(prefix) {
		return nil
	}

	value := stringASCIIValue(prefix)

	switch format {
	case schemaFormatByte:
		return stringBase64Intervals(value)
	case schemaFormatDate:
		return stringDateIntervals(value)
	case schemaFormatDateTime:
		return stringDateTimeIntervals(value)
	case schemaFormatEmail:
		return stringEmailIntervals(value)
	case schemaFormatIPv4:
		return stringIPv4Intervals(value, false)
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return stringUUIDIntervals(value)
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return stringIPv4Intervals(value, true)
	case schemaFormatPassword:
		return stringAllUnitIntervals()
	default:
		return nil
	}
}

func stringFormatSeedUnits(format schemaFormat) []uint16 {
	switch format {
	case schemaFormatDate:
		return []uint16{'0', '1', '2', '4', '9'}
	case schemaFormatDateTime:
		return []uint16{'0', '1', '2', '4', '9', 'T', ':', 'Z', '+', '-', '.'}
	case schemaFormatEmail:
		return []uint16{'!', '0', 'a', '@', '.'}
	case schemaFormatIPv4:
		return []uint16{'0', '.'}
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return []uint16{'0', '4', '8', 'a', 'A'}
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return []uint16{'0', '.', '/', '1', '2', '3'}
	default:
		return nil
	}
}

func stringFormatMaximumLength(format schemaFormat) (uint64, bool) {
	switch format {
	case schemaFormatDate:
		return 10, true
	case schemaFormatEmail:
		return 254, true
	case schemaFormatIPv4:
		return 15, true
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return 36, true
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return 18, true
	default:
		return 0, false
	}
}

func stringFormatPinnedLengths(format schemaFormat) []uint64 {
	switch format {
	case schemaFormatDate:
		return []uint64{10}
	case schemaFormatDateTime:
		return []uint64{20}
	case schemaFormatEmail:
		return []uint64{3}
	case schemaFormatIPv4:
		return []uint64{7}
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return []uint64{36}
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return []uint64{9}
	default:
		return nil
	}
}

func stringPrefixASCII(prefix []uint16) bool {
	for _, unit := range prefix {
		if unit > 0x7f {
			return false
		}
	}

	return true
}

func stringASCIIValue(prefix []uint16) string {
	value := make([]byte, len(prefix))
	for index, unit := range prefix {
		value[index] = byte(unit)
	}

	return string(value)
}

func stringAllUnitIntervals() []stringUnitInterval {
	return []stringUnitInterval{{low: 0, high: 0xffff}}
}

func stringASCIIIntervals(characters string) []stringUnitInterval {
	if characters == "" {
		return nil
	}

	values := make([]int, 0, len(characters))
	for index := 0; index < len(characters); index++ {
		values = append(values, int(characters[index]))
	}

	sort.Ints(values)
	intervals := make([]stringUnitInterval, 0, len(values))
	for _, value := range uniqueInts(values) {
		unit := uint16(value)
		if len(intervals) > 0 && intervals[len(intervals)-1].high+1 == unit {
			intervals[len(intervals)-1].high = unit

			continue
		}

		intervals = append(intervals, stringUnitInterval{low: unit, high: unit})
	}

	return intervals
}

func stringBase64Intervals(prefix string) []stringUnitInterval {
	if strings.IndexByte(prefix, '=') >= 0 {
		if len(prefix)%4 == 0 {
			return nil
		}

		return stringASCIIIntervals("=")
	}

	switch len(prefix) % 4 {
	case 2, 3:
		return stringASCIIIntervals("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=")
	default:
		return stringASCIIIntervals("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")
	}
}

func stringDateIntervals(prefix string) []stringUnitInterval {
	if len(prefix) >= 10 {
		return nil
	}

	switch len(prefix) {
	case 4, 7:
		return stringASCIIIntervals("-")
	case 5:
		return stringASCIIIntervals("01")
	case 6:
		if prefix[5] == '0' {
			return stringASCIIIntervals("123456789")
		}

		return stringASCIIIntervals("012")
	case 8:
		return stringASCIIIntervals("0123")
	case 9:
		switch prefix[8] {
		case '0':
			return stringASCIIIntervals("123456789")
		case '3':
			return stringASCIIIntervals("01")
		default:
			return stringASCIIIntervals("0123456789")
		}
	default:
		return stringASCIIIntervals("0123456789")
	}
}

func stringDateTimeIntervals(prefix string) []stringUnitInterval {
	if len(prefix) < 10 {
		return stringDateIntervals(prefix)
	}

	if len(prefix) == 10 {
		return stringASCIIIntervals("T")
	}

	if len(prefix) < 19 {
		switch len(prefix) {
		case 13, 16:
			return stringASCIIIntervals(":")
		case 11, 14, 17:
			return stringASCIIIntervals("012345")
		case 12:
			if prefix[11] == '2' {
				return stringASCIIIntervals("0123")
			}

			return stringASCIIIntervals("0123456789")
		default:
			return stringASCIIIntervals("0123456789")
		}
	}

	if len(prefix) == 19 {
		return stringASCIIIntervals(".Z+-")
	}

	if prefix[19] == '.' {
		for _, character := range prefix[20:] {
			if character < '0' || character > '9' {
				return nil
			}
		}

		return stringASCIIIntervals("0123456789Z+-")
	}

	if prefix[19] == 'Z' {
		return nil
	}

	if prefix[19] != '+' && prefix[19] != '-' {
		return nil
	}

	zone := prefix[20:]
	if len(zone) == 0 || len(zone) == 1 {
		return stringASCIIIntervals("0123456789")
	}

	if len(zone) == 2 {
		return stringASCIIIntervals(":")
	}

	if len(zone) < 5 {
		return stringASCIIIntervals("0123456789")
	}

	return nil
}

func stringUUIDIntervals(prefix string) []stringUnitInterval {
	if len(prefix) >= 36 {
		return nil
	}

	if prefixPositionIsUUIDDash(len(prefix)) {
		return stringASCIIIntervals("-")
	}

	if len(prefix) == 14 {
		return stringASCIIIntervals("4")
	}

	if len(prefix) == 19 {
		return stringASCIIIntervals("89ABab")
	}

	return stringASCIIIntervals("0123456789abcdefABCDEF")
}

func prefixPositionIsUUIDDash(position int) bool {
	return position == 8 || position == 13 || position == 18 || position == 23
}

func stringIPv4Intervals(prefix string, cidr bool) []stringUnitInterval {
	address := prefix
	suffix := ""
	if slash := strings.IndexByte(prefix, '/'); slash >= 0 {
		if !cidr || strings.IndexByte(prefix[slash+1:], '/') >= 0 {
			return nil
		}

		address = prefix[:slash]
		suffix = prefix[slash+1:]
	}

	if !stringIPv4AddressPrefixAllowed(address) {
		return nil
	}

	if suffix != "" || strings.HasSuffix(prefix, "/") {
		if !cidr || !stringCIDRPrefixAllowed(suffix) {
			return nil
		}

		if len(suffix) >= 2 {
			return nil
		}

		return stringASCIIIntervals("0123456789")
	}

	parts := strings.Split(address, ".")
	if len(parts) == 4 && cleanIPv4Octet(parts[3], false) {
		continuation := ""
		if parts[3] != "0" && len(parts[3]) < 3 {
			continuation = "0123456789"
		}
		if cidr {
			continuation += "/"
		}

		return stringASCIIIntervals(continuation)
	}

	current := parts[len(parts)-1]
	if current == "" || current == "0" {
		return stringASCIIIntervals("0123456789.")
	}

	if len(current) >= 3 {
		return stringASCIIIntervals(".")
	}

	return stringASCIIIntervals("0123456789.")
}

func stringIPv4AddressPrefixAllowed(prefix string) bool {
	parts := strings.Split(prefix, ".")
	if len(parts) > 4 {
		return false
	}

	for index, part := range parts {
		last := index == len(parts)-1
		if part == "" {
			return last
		}

		if len(part) > 3 || len(part) > 1 && part[0] == '0' {
			return false
		}

		for _, character := range part {
			if character < '0' || character > '9' {
				return false
			}
		}

		if !last && !cleanIPv4Octet(part, false) {
			return false
		}
	}

	return true
}

func stringCIDRPrefixAllowed(prefix string) bool {
	if len(prefix) > 2 || len(prefix) > 1 && prefix[0] == '0' {
		return false
	}

	for _, character := range prefix {
		if character < '0' || character > '9' {
			return false
		}
	}

	return prefix == "" || decimalDigits(prefix) <= 32
}

func stringEmailIntervals(prefix string) []stringUnitInterval {
	if prefix == "" {
		return stringASCIIIntervals("\"!#$%&'*+/=?^_`{|}~-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	}

	if prefix[0] == '"' {
		return stringQuotedEmailIntervals(prefix)
	}

	separator := strings.IndexByte(prefix, '@')
	if separator < 0 {
		return stringUnquotedEmailLocalIntervals(prefix)
	}

	if !stringUnquotedEmailLocal(prefix[:separator]) {
		return nil
	}

	return stringEmailDomainIntervals(prefix[separator+1:])
}

func stringQuotedEmailIntervals(prefix string) []stringUnitInterval {
	position := 1
	closed := false
	for position < len(prefix) {
		switch prefix[position] {
		case '"':
			closed = true
			position++

		case '\\':
			position++
			if position == len(prefix) || prefix[position] < 0x20 || prefix[position] > 0x7e {
				return nil
			}
			position++
		default:
			if !isEmailQuotedCharacter(prefix[position]) {
				return nil
			}

			position++
		}

		if closed {
			break
		}
	}

	if !closed {
		if prefix[len(prefix)-1] == '\\' {
			return stringASCIIIntervals(" !#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\\\]^_`abcdefghijklmnopqrstuvwxyz{|}~")
		}

		return stringASCIIIntervals(" !#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~\\\"")
	}

	if position == len(prefix) {
		return stringASCIIIntervals("@")
	}

	if prefix[position] != '@' || position+1 > len(prefix) {
		return nil
	}

	return stringEmailDomainIntervals(prefix[position+1:])
}

func stringUnquotedEmailLocalIntervals(prefix string) []stringUnitInterval {
	if !stringUnquotedEmailLocalPrefix(prefix) {
		return nil
	}

	if prefix == "" || prefix[len(prefix)-1] == '.' {
		return stringASCIIIntervals("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$%&'*+/=?^_`{|}~-")
	}

	return stringASCIIIntervals("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$%&'*+/=?^_`{|}~-.@")
}

func stringUnquotedEmailLocalPrefix(value string) bool {
	if value == "" || value[0] == '.' {
		return value == ""
	}

	previousDot := false
	for _, character := range value {
		if character == '.' {
			if previousDot {
				return false
			}

			previousDot = true

			continue
		}

		if !isEmailAtext(byte(character)) {
			return false
		}

		previousDot = false
	}

	return true
}

func stringUnquotedEmailLocal(value string) bool {
	return stringUnquotedEmailLocalPrefix(value) &&
		value != "" && value[len(value)-1] != '.'
}

func stringEmailDomainIntervals(prefix string) []stringUnitInterval {
	if prefix == "" {
		return stringASCIIIntervals("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz[")
	}

	if prefix[0] == '[' {
		if strings.Contains(prefix[1:], "]") {
			return nil
		}

		return stringASCIIIntervals("!#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~")
	}

	if !stringEmailDomainPrefix(prefix) {
		return nil
	}

	last := prefix[len(prefix)-1]
	if last == '.' {
		return stringASCIIIntervals("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	}

	return stringASCIIIntervals("-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz.")
}

func stringEmailDomainPrefix(prefix string) bool {
	labels := strings.Split(prefix, ".")
	for _, label := range labels {
		if label == "" {
			continue
		}

		if label[0] == '-' {
			return false
		}

		for _, character := range label {
			if !isEmailDomainCharacter(byte(character)) {
				return false
			}
		}
	}

	return true
}
