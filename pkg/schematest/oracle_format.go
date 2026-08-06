//nolint:cyclop,godoclint,mnd // Closed format grammars are intentionally explicit and independent.
package schematest

import "strings"

// cleanStringFormatMatches applies the admitted string-format languages without production state.
func cleanStringFormatMatches(value string, format schemaFormat) (bool, error) {
	switch format {
	case schemaFormatByte:
		return cleanByteFormatMatches(value), nil
	case schemaFormatDate:
		return cleanDateFormatMatches(value), nil
	case schemaFormatDateTime:
		return cleanDateTimeFormatMatches(value), nil
	case schemaFormatEmail:
		return cleanEmailFormatMatches(value), nil
	case schemaFormatIPv4:
		return cleanIPv4FormatMatches(value), nil
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return cleanUUIDFormatMatches(value), nil
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return cleanCIDRFormatMatches(value), nil
	case schemaFormatPassword:
		return true, nil
	default:
		return false, nil
	}
}

func cleanByteFormatMatches(value string) bool {
	if len(value)%4 != 0 {
		return false
	}

	if value == "" {
		return true
	}

	for position := 0; position < len(value); position += 4 {
		first, second := value[position], value[position+1]
		third, fourth := value[position+2], value[position+3]

		if base64Value(first) < 0 || base64Value(second) < 0 {
			return false
		}

		thirdValue := base64Value(third)
		fourthValue := base64Value(fourth)
		last := position+4 == len(value)

		switch {
		case thirdValue >= 0 && fourthValue >= 0:
			if !last {
				continue
			}
		case thirdValue >= 0 && fourth == '=':
			if !last || thirdValue&3 != 0 {
				return false
			}
		case third == '=' && fourth == '=':
			if !last || base64Value(second)&15 != 0 {
				return false
			}
		default:
			return false
		}
	}

	return true
}

func base64Value(value byte) int {
	switch {
	case value >= 'A' && value <= 'Z':
		return int(value - 'A')
	case value >= 'a' && value <= 'z':
		return int(value-'a') + 26
	case value >= '0' && value <= '9':
		return int(value-'0') + 52
	case value == '+':
		return 62
	case value == '/':
		return 63
	default:
		return -1
	}
}

func cleanDateFormatMatches(value string) bool {
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return false
	}

	for position, character := range []byte(value) {
		if position == 4 || position == 7 {
			continue
		}

		if character < '0' || character > '9' {
			return false
		}
	}

	year := decimalDigits(value[0:4])
	month := decimalDigits(value[5:7])

	day := decimalDigits(value[8:10])
	if month < 1 || month > 12 || day < 1 {
		return false
	}

	days := [...]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

	maximum := days[month]
	if month == 2 && isLeapYear(year) {
		maximum = 29
	}

	return day <= maximum
}

func cleanDateTimeFormatMatches(value string) bool {
	if len(value) < 20 || value[10] != 'T' || !cleanDateFormatMatches(value[:10]) {
		return false
	}

	if !isTwoDigits(value[11:13]) || value[13] != ':' || !isTwoDigits(value[14:16]) ||
		value[16] != ':' || !isTwoDigits(value[17:19]) {
		return false
	}

	hour := decimalDigits(value[11:13])
	minute := decimalDigits(value[14:16])

	second := decimalDigits(value[17:19])
	if hour > 23 || minute > 59 || second > 59 {
		return false
	}

	position := 19
	if position < len(value) && value[position] == '.' {
		position++

		start := position
		for position < len(value) && value[position] >= '0' && value[position] <= '9' {
			position++
		}

		if position == start {
			return false
		}
	}

	if position == len(value) || value[position] == 'Z' {
		return position+1 == len(value)
	}

	if value[position] != '+' && value[position] != '-' {
		return false
	}

	if position+6 != len(value) || !isTwoDigits(value[position+1:position+3]) || value[position+3] != ':' ||
		!isTwoDigits(value[position+4:position+6]) {
		return false
	}

	return decimalDigits(value[position+1:position+3]) <= 23 && decimalDigits(value[position+4:position+6]) <= 59
}

func cleanIPv4FormatMatches(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if !cleanIPv4Octet(part, false) {
			return false
		}
	}

	return true
}

func cleanCIDRFormatMatches(value string) bool {
	address, prefix, found := strings.Cut(value, "/")
	if !found || strings.ContainsRune(prefix, '/') || !cleanIPv4FormatMatches(address) {
		return false
	}

	if prefix == "" || len(prefix) > 2 {
		return false
	}

	if len(prefix) > 1 && prefix[0] == '0' {
		return false
	}

	for _, character := range prefix {
		if character < '0' || character > '9' {
			return false
		}
	}

	return decimalDigits(prefix) <= 32
}

func cleanIPv4Octet(value string, leadingZeros bool) bool {
	if value == "" || len(value) > 3 {
		return false
	}

	if !leadingZeros && len(value) > 1 && value[0] == '0' {
		return false
	}

	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}

	return decimalDigits(value) <= 255
}

func cleanUUIDFormatMatches(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' ||
		value[14] != '4' || !isUUIDVariant(value[19]) {
		return false
	}

	for position, character := range []byte(value) {
		if position == 8 || position == 13 || position == 18 || position == 23 {
			continue
		}

		if !isHexCharacter(character) {
			return false
		}
	}

	return true
}

func isUUIDVariant(value byte) bool {
	return value == '8' || value == '9' || value == 'A' || value == 'B' || value == 'a' || value == 'b'
}

func cleanEmailFormatMatches(value string) bool {
	if len(value) == 0 || len(value) > 254 || !isASCII(value) {
		return false
	}

	separator, ok := cleanEmailLocalEnd(value)
	if !ok || separator == 0 || separator >= len(value) || value[separator] != '@' ||
		separator+1 >= len(value) || separator > 64 {
		return false
	}

	return cleanEmailDomain(value[separator+1:])
}

func cleanEmailLocalEnd(value string) (int, bool) {
	if value[0] == '"' {
		position := 1
		for position < len(value) {
			character := value[position]
			switch character {
			case '"':
				return position + 1, true
			case '\\':
				position++
				if position == len(value) || value[position] < 0x20 || value[position] > 0x7e {
					return 0, false
				}
			default:
				if !isEmailQuotedCharacter(character) {
					return 0, false
				}
			}

			position++
		}

		return 0, false
	}

	position := 0
	if !isEmailAtext(value[position]) {
		return 0, false
	}

	for position < len(value) && value[position] != '@' {
		if !isEmailAtext(value[position]) {
			return 0, false
		}

		position++
		if position < len(value) && value[position] == '.' {
			position++
			if position == len(value) || !isEmailAtext(value[position]) {
				return 0, false
			}
		}
	}

	return position, position < len(value) && value[position] == '@'
}

func cleanEmailDomain(value string) bool {
	if value[0] == '[' {
		return len(value) > 2 && value[len(value)-1] == ']' && cleanEmailAddressLiteral(value[1:len(value)-1])
	}

	labels := strings.Split(value, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || !isASCII(label) || label[0] == '-' || label[len(label)-1] == '-' {
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

func cleanEmailAddressLiteral(value string) bool {
	if len(value) >= 5 && strings.EqualFold(value[:5], "ipv6:") {
		return cleanEmailIPv6(value[5:])
	}

	if cleanEmailIPv4(value) {
		return true
	}

	separator := strings.IndexByte(value, ':')
	if separator <= 0 || separator+1 == len(value) || !cleanEmailGeneralTag(value[:separator]) {
		return false
	}

	for _, character := range value[separator+1:] {
		if character < '!' || character > '~' || character == '[' || character == '\\' || character == ']' {
			return false
		}
	}

	return true
}

func cleanEmailIPv4(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if !cleanIPv4Octet(part, true) {
			return false
		}
	}

	return true
}

func cleanEmailIPv6(value string) bool {
	if strings.Count(value, "::") > 1 {
		return false
	}

	if strings.Contains(value, ".") {
		separator := strings.LastIndexByte(value, ':')
		if separator < 0 || !cleanEmailIPv4(value[separator+1:]) {
			return false
		}

		hexPart := value[:separator]
		switch {
		case strings.HasSuffix(hexPart, "::"):
			return false
		case strings.HasSuffix(hexPart, ":"):
			hexPart += ":"
		}

		if strings.Contains(hexPart, "::") {
			return cleanCompressedIPv6(hexPart, 4)
		}

		return cleanIPv6Groups(hexPart, 6)
	}

	if strings.Contains(value, "::") {
		return cleanCompressedIPv6(value, 6)
	}

	return cleanIPv6Groups(value, 8)
}

func cleanCompressedIPv6(value string, maximumExplicit int) bool {
	parts := strings.Split(value, "::")
	if len(parts) != 2 {
		return false
	}

	left := splitIPv6Groups(parts[0])

	right := splitIPv6Groups(parts[1])
	if (parts[0] != "" && len(left) == 0) || (parts[1] != "" && len(right) == 0) ||
		len(left)+len(right) > maximumExplicit {
		return false
	}

	for _, group := range append(left, right...) {
		if !cleanIPv6Group(group) {
			return false
		}
	}

	return true
}

func cleanIPv6Groups(value string, expected int) bool {
	groups := splitIPv6Groups(value)
	if len(groups) != expected {
		return false
	}

	for _, group := range groups {
		if !cleanIPv6Group(group) {
			return false
		}
	}

	return true
}

func splitIPv6Groups(value string) []string {
	if value == "" {
		return nil
	}

	return strings.Split(value, ":")
}

func cleanIPv6Group(value string) bool {
	if len(value) == 0 || len(value) > 4 {
		return false
	}

	for _, character := range value {
		if !isHexCharacter(byte(character)) {
			return false
		}
	}

	return true
}

func cleanEmailGeneralTag(value string) bool {
	if len(value) == 0 || len(value) == 4 && strings.EqualFold(value, "ipv6") {
		return false
	}

	return isEmailTagCharacters(value) && isASCIIAlphaNumeric(value[len(value)-1])
}

func isEmailTagCharacters(value string) bool {
	for _, character := range value {
		if !isASCIIAlphaNumeric(byte(character)) && character != '-' {
			return false
		}
	}

	return true
}

func isEmailAtext(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9' ||
		strings.ContainsRune("!#$%&'*+/=?^_`{|}~-", rune(value))
}

func isEmailQuotedCharacter(value byte) bool {
	return value >= 0x20 && value <= 0x21 || value >= 0x23 && value <= 0x5b || value >= 0x5d && value <= 0x7e
}

func isEmailDomainCharacter(value byte) bool {
	return isASCIIAlphaNumeric(value) || value == '-'
}

func isASCIIAlphaNumeric(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func isASCII(value string) bool {
	for _, character := range value {
		if character > 0x7f {
			return false
		}
	}

	return true
}

func isHexCharacter(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'A' && value <= 'F' || value >= 'a' && value <= 'f'
}

func isTwoDigits(value string) bool {
	return len(value) == 2 && value[0] >= '0' && value[0] <= '9' && value[1] >= '0' && value[1] <= '9'
}

func decimalDigits(value string) int {
	result := 0
	for _, character := range value {
		result = result*10 + int(character-'0')
	}

	return result
}

func isLeapYear(year int) bool {
	return year%400 == 0 || year%4 == 0 && year%100 != 0
}
