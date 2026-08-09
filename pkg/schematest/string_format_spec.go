//nolint:cyclop,godoclint,mnd // The closed search-side format languages are intentionally explicit.
package schematest

import "strings"

type stringFormatObjective uint8

const (
	stringFormatObjectiveCanonical stringFormatObjective = iota
	stringFormatObjectiveLowerBoundary
	stringFormatObjectiveUpperBoundary
	stringFormatObjectiveLeapBoundary
	stringFormatObjectiveCenturyBoundary
	stringFormatObjectiveLocalLimit
	stringFormatObjectiveDomainLimit
	stringFormatObjectivePadding
	stringFormatObjectiveSemanticFailure
)

type stringFormatBounds struct {
	minimum  uint64
	maximum  uint64
	bounded  bool
	multiple uint64
}

func (bounds stringFormatBounds) allows(length uint64) bool {
	return length >= bounds.minimum && (!bounds.bounded || length <= bounds.maximum) &&
		(bounds.multiple == 0 || length%bounds.multiple == 0)
}

type stringFormatProgramState struct {
	text   string
	length int
	alive  bool
}

type stringFormatProgram struct {
	alphabet        string
	alphabetLow     uint16
	alphabetHigh    uint16
	bounds          stringFormatBounds
	match           func(string) bool
	prefix          func(string, int) bool
	transitionClass func(string, uint16) uint32
	pattern         string
}

func (program *stringFormatProgram) start(length int) stringFormatProgramState {
	return stringFormatProgramState{length: length, alive: length >= 0}
}

func (program *stringFormatProgram) advance(
	state stringFormatProgramState,
	unit uint16,
) stringFormatProgramState {
	if !state.alive || !program.hasUnit(unit) || len(state.text) == state.length ||
		program.bounds.bounded && uint64(len(state.text)+1) > program.bounds.maximum {
		return stringFormatProgramState{length: state.length}
	}

	text := state.text + string(rune(unit))
	if program.prefix != nil && !program.prefix(text, state.length) {
		return stringFormatProgramState{length: state.length}
	}

	return stringFormatProgramState{text: text, length: state.length, alive: true}
}

func (program *stringFormatProgram) accepts(candidate string) bool {
	state := program.start(len(candidate))

	for _, character := range candidate {
		if character > rune(basicStringMaxUnit) {
			return false
		}

		state = program.advance(state, uint16(character))
	}

	return program.accept(state)
}

func (program *stringFormatProgram) accept(state stringFormatProgramState) bool {
	return state.alive && len(state.text) == state.length &&
		program.bounds.allows(uint64(len(state.text))) && program.match(state.text)
}

func (program *stringFormatProgram) transition(state stringFormatProgramState, unit uint16) uint32 {
	if !program.advance(state, unit).alive {
		return 0
	}

	if program.transitionClass == nil {
		return 1
	}

	return program.transitionClass(state.text, unit)
}

func (program *stringFormatProgram) hasUnit(unit uint16) bool {
	if program.alphabetLow != 0 || program.alphabetHigh != 0 {
		return unit >= program.alphabetLow && unit <= program.alphabetHigh
	}

	return strings.ContainsRune(program.alphabet, rune(unit))
}

func (program *stringFormatProgram) eachUnit(yield func(uint16)) {
	if program.alphabetLow != 0 || program.alphabetHigh != 0 {
		for unit := program.alphabetLow; unit <= program.alphabetHigh; unit++ {
			yield(unit)
		}

		return
	}

	for _, unit := range program.alphabet {
		yield(uint16(unit))
	}
}

type stringFormatSpecification struct {
	program        *stringFormatProgram
	bounds         stringFormatBounds
	objectives     [4]stringFormatObjective
	objectiveCount uint8
	inert          bool
}

var (
	base64FormatSpecification = newStringFormatSpecification(
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",
		stringFormatBounds{multiple: 4}, searchByteFormatMatches, searchByteFormatPrefixViable,
		searchByteFormatTransitionClass,
		`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`,
		stringFormatObjectiveCanonical, stringFormatObjectivePadding,
	)
	dateFormatSpecification = newStringFormatSpecification(
		"0123456789-", exactStringFormatBounds(10), searchDateFormatMatches, searchDateFormatPrefixViable,
		searchDateFormatTransitionClass,
		`^[0-9]{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12][0-9]|3[01])$`,
		stringFormatObjectiveCanonical, stringFormatObjectiveLeapBoundary,
		stringFormatObjectiveCenturyBoundary, stringFormatObjectiveUpperBoundary,
	)
	dateTimeFormatSpecification = newStringFormatSpecification(
		"0123456789-T:.Z+", stringFormatBounds{minimum: 20}, searchDateTimeFormatMatches,
		searchDateTimeFormatPrefixViable, searchDateFormatTransitionClass,
		`^[0-9]{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12][0-9]|3[01])T`+
			`(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](?:\.[0-9]+)?`+
			`(?:Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])$`,
		stringFormatObjectiveCanonical, stringFormatObjectiveLeapBoundary,
		stringFormatObjectiveCenturyBoundary, stringFormatObjectiveUpperBoundary,
	)
	emailFormatSpecification = newStringFormatSpecification(
		"", stringFormatBounds{minimum: 3, maximum: 254, bounded: true}, searchEmailFormatMatches,
		searchEmailFormatPrefixViable, searchEmailFormatTransitionClass,
		`^(?:(?:[A-Za-z0-9!#$%&'*+/=?^_\x60{|}~-]|\.){1,64}|`+
			`"(?:[\x20-\x21\x23-\x5b\x5d-\x7e]|\\[\x20-\x7e]){0,62}")@(?:`+
			`[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.`+
			`[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*|`+
			`\[(?:[0-9]{1,3}(?:\.[0-9]{1,3}){3}|[Ii][Pp][Vv]6:[0-9A-Fa-f:.]+|`+
			`[A-Za-z0-9-]+:[\x21-\x5a\x5e-\x7e]+)\])$`,
		stringFormatObjectiveCanonical, stringFormatObjectiveLocalLimit, stringFormatObjectiveDomainLimit,
	)
	ipv4FormatSpecification = newStringFormatSpecification(
		"0123456789.", stringFormatBounds{minimum: 7, maximum: 15, bounded: true}, searchIPv4FormatMatches, nil, nil,
		`^(?:0|[1-9][0-9]?|1[0-9]{2}|2[0-4][0-9]|25[0-5])`+
			`(?:\.(?:0|[1-9][0-9]?|1[0-9]{2}|2[0-4][0-9]|25[0-5])){3}$`,
		stringFormatObjectiveLowerBoundary, stringFormatObjectiveUpperBoundary,
	)
	uuidFormatSpecification = newStringFormatSpecification(
		"0123456789ABCDEFabcdef-", exactStringFormatBounds(36), searchUUIDFormatMatches, nil, nil,
		`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-4[0-9A-Fa-f]{3}-[89ABab][0-9A-Fa-f]{3}-[0-9A-Fa-f]{12}$`,
		stringFormatObjectiveCanonical,
	)
	cidrFormatSpecification = newStringFormatSpecification(
		"0123456789./", stringFormatBounds{minimum: 9, maximum: 18, bounded: true}, searchCIDRFormatMatches, nil, nil,
		`^(?:0|[1-9][0-9]?|1[0-9]{2}|2[0-4][0-9]|25[0-5])`+
			`(?:\.(?:0|[1-9][0-9]?|1[0-9]{2}|2[0-4][0-9]|25[0-5])){3}/(?:0|[1-9]|[12][0-9]|3[0-2])$`,
		stringFormatObjectiveLowerBoundary, stringFormatObjectiveUpperBoundary,
	)
	passwordFormatSpecification = &stringFormatSpecification{inert: true}
)

func newStringFormatSpecification(
	alphabet string,
	bounds stringFormatBounds,
	match func(string) bool,
	prefix func(string, int) bool,
	transitionClass func(string, uint16) uint32,
	pattern string,
	objectives ...stringFormatObjective,
) *stringFormatSpecification {
	if prefix == nil {
		prefix = func(candidate string, length int) bool {
			return len(candidate) < length || match(candidate)
		}
	}

	program := &stringFormatProgram{
		alphabet: alphabet, bounds: bounds, match: match, prefix: prefix,
		transitionClass: transitionClass, pattern: pattern,
	}
	if alphabet == "" {
		program.alphabetLow = 0x20
		program.alphabetHigh = 0x7e
	}

	specification := &stringFormatSpecification{program: program, bounds: bounds, objectiveCount: uint8(len(objectives))}
	copy(specification.objectives[:], objectives)

	return specification
}

func exactStringFormatBounds(length uint64) stringFormatBounds {
	return stringFormatBounds{minimum: length, maximum: length, bounded: true}
}

func stringFormatSpecificationFor(format schemaFormat) (*stringFormatSpecification, bool) {
	switch format {
	case schemaFormatByte:
		return base64FormatSpecification, true
	case schemaFormatDate:
		return dateFormatSpecification, true
	case schemaFormatDateTime:
		return dateTimeFormatSpecification, true
	case schemaFormatEmail:
		return emailFormatSpecification, true
	case schemaFormatIPv4:
		return ipv4FormatSpecification, true
	case schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4:
		return uuidFormatSpecification, true
	case schemaFormatCIDR, schemaFormatIPv4CIDR:
		return cidrFormatSpecification, true
	case schemaFormatPassword:
		return passwordFormatSpecification, true
	default:
		return nil, false
	}
}

func stringFormatByName(name string) (schemaFormat, bool) {
	switch name {
	case "byte":
		return schemaFormatByte, true
	case "date":
		return schemaFormatDate, true
	case "date-time":
		return schemaFormatDateTime, true
	case "email":
		return schemaFormatEmail, true
	case "ipv4":
		return schemaFormatIPv4, true
	case "uuid":
		return schemaFormatUUID, true
	case "uuidv4":
		return schemaFormatUUIDv4, true
	case "uuid-v4":
		return schemaFormatUUIDDashV4, true
	case "cidr":
		return schemaFormatCIDR, true
	case "ipv4-cidr":
		return schemaFormatIPv4CIDR, true
	case "password":
		return schemaFormatPassword, true
	default:
		return schemaFormatNone, false
	}
}

func searchByteFormatTransitionClass(prefix string, unit uint16) uint32 {
	value := searchBase64Value(byte(unit))
	if value < 0 {
		return uint32(unit) + 1
	}

	switch len(prefix) % 4 {
	case 1:
		return uint32(value&15) + 1
	case 2:
		return uint32(value&3) + 1
	default:
		return 1
	}
}

func searchDateFormatTransitionClass(prefix string, unit uint16) uint32 {
	if unit < '0' || unit > '9' || len(prefix) >= 10 {
		return 1
	}

	digit := int(unit - '0')

	switch len(prefix) {
	case 0:
		return uint32(digit%2) + 1
	case 1:
		century := int(prefix[0]-'0')*10 + digit

		return uint32(century%4) + 1
	case 2:
		if digit == 0 {
			return 1
		}

		return uint32(digit%2) + 2
	case 3:
		year := searchDecimalDigits(prefix)*10 + digit
		if searchLeapYear(year) {
			return 2
		}

		return 1
	case 6:
		month := searchDecimalDigits(prefix[5:])*10 + digit
		switch month {
		case 2:
			return 1
		case 4, 6, 9, 11:
			return 2
		default:
			return 3
		}
	case 8:
		return uint32(digit) + 1
	default:
		return 1
	}
}

func searchEmailFormatTransitionClass(_ string, unit uint16) uint32 {
	if unit >= '0' && unit <= '9' {
		return uint32(unit-'0') + 1
	}

	switch unit {
	case '.', '@', '[', ']', ':', '\\', '"', '-':
		return uint32(unit) + 16
	default:
		return 11
	}
}

func searchEmailFormatPrefixViable(prefix string, length int) bool {
	if length < 3 || length > 254 || len(prefix) > length {
		return false
	}

	if len(prefix) == length {
		return searchEmailFormatMatches(prefix)
	}

	separator, viable := searchEmailLocalPrefix(prefix)
	if !viable || separator < 0 {
		return viable
	}

	domain := prefix[separator+1:]
	if domain == "" {
		return true
	}

	if domain[0] == '[' {
		return searchEmailLiteralPrefixViable(domain[1:])
	}

	labels := strings.Split(domain, ".")
	for index, label := range labels {
		if len(label) > 63 || len(label) > 0 && label[0] == '-' ||
			index < len(labels)-1 && (label == "" || label[len(label)-1] == '-') {
			return false
		}
	}

	return true
}

func searchEmailLocalPrefix(prefix string) (int, bool) {
	if prefix[0] != '"' {
		separator := strings.IndexByte(prefix, '@')
		if separator < 0 {
			return -1, len(prefix) <= 64 && prefix[0] != '.' && !strings.Contains(prefix, "..")
		}

		return separator, separator > 0 && separator <= 64 && prefix[separator-1] != '.'
	}

	localEnd, complete := searchEmailLocalEnd(prefix)
	if !complete {
		return -1, len(prefix) <= 64
	}

	if localEnd == len(prefix) {
		return -1, true
	}

	return localEnd, prefix[localEnd] == '@'
}

func searchEmailLiteralPrefixViable(literal string) bool {
	if strings.ContainsRune(literal, ']') {
		return false
	}

	separator := strings.IndexByte(literal, ':')
	if separator >= 0 && !strings.ContainsRune(literal[:separator], '.') {
		return searchEmailTaggedLiteralPrefixViable(literal, separator)
	}

	return searchEmailIPv4LiteralPrefixViable(literal)
}

func searchEmailTaggedLiteralPrefixViable(literal string, separator int) bool {
	tag := literal[:separator]
	if !strings.EqualFold(tag, "ipv6") {
		return tag != "" && searchASCIIAlphaNumeric(tag[len(tag)-1])
	}

	body := literal[separator+1:]
	if strings.Count(body, "::") > 1 {
		return false
	}

	explicit := 0

	for _, group := range strings.Split(body, ":") {
		if len(group) > 4 {
			return false
		}

		if group != "" {
			explicit++
		}
	}

	return explicit <= 8
}

func searchEmailIPv4LiteralPrefixViable(literal string) bool {
	for _, character := range literal {
		if character != '.' && (character < '0' || character > '9') {
			return true
		}
	}

	for _, part := range strings.Split(literal, ".") {
		if len(part) > 3 || len(part) == 3 && searchDecimalDigits(part) > 255 {
			return false
		}
	}

	return true
}

func searchByteFormatPrefixViable(prefix string, length int) bool {
	if length < 0 || length%4 != 0 || len(prefix) > length {
		return false
	}

	padding := strings.IndexByte(prefix, '=')
	if padding < 0 {
		return true
	}

	if padding != length-2 && padding != length-1 {
		return false
	}

	if padding == length-2 {
		if padding < 2 || searchBase64Value(prefix[padding-1])&15 != 0 {
			return false
		}

		for position := padding; position < len(prefix); position++ {
			if prefix[position] != '=' {
				return false
			}
		}

		return true
	}

	return padding >= 3 && searchBase64Value(prefix[padding-1])&3 == 0
}

func searchDateFormatPrefixViable(prefix string, length int) bool {
	if length != 10 || len(prefix) > length {
		return false
	}

	if len(prefix) >= 7 {
		month := searchDecimalDigits(prefix[5:7])
		if month < 1 || month > 12 {
			return false
		}
	}

	if len(prefix) < 9 {
		return true
	}

	year := searchDecimalDigits(prefix[:4])
	month := searchDecimalDigits(prefix[5:7])
	days := [...]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

	maximum := days[month]
	if month == 2 && searchLeapYear(year) {
		maximum = 29
	}

	minimumDay := searchDecimalDigits(prefix[8:])
	if len(prefix) == 9 {
		minimumDay *= 10
	}

	maximumDay := minimumDay
	if len(prefix) == 9 {
		maximumDay += 9
	}

	return maximumDay >= 1 && minimumDay <= maximum
}

func searchDateTimeFormatPrefixViable(prefix string, length int) bool {
	if length < 20 || len(prefix) > length {
		return false
	}

	if len(prefix) < 10 {
		return searchDateFormatPrefixViable(prefix, 10)
	}

	if !searchDateFormatMatches(prefix[:10]) {
		return false
	}

	return len(prefix) < length || searchDateTimeFormatMatches(prefix)
}

func searchByteFormatMatches(value string) bool {
	if len(value)%4 != 0 {
		return false
	}

	for position := 0; position < len(value); position += 4 {
		first, second, third, fourth := value[position], value[position+1], value[position+2], value[position+3]
		if searchBase64Value(first) < 0 || searchBase64Value(second) < 0 {
			return false
		}

		thirdValue, fourthValue := searchBase64Value(third), searchBase64Value(fourth)
		last := position+4 == len(value)

		switch {
		case thirdValue >= 0 && fourthValue >= 0:
		case thirdValue >= 0 && fourth == '=':
			if !last || thirdValue&3 != 0 {
				return false
			}
		case third == '=' && fourth == '=':
			if !last || searchBase64Value(second)&15 != 0 {
				return false
			}
		default:
			return false
		}
	}

	return true
}

func searchBase64Value(value byte) int {
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

func searchDateFormatMatches(value string) bool {
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return false
	}

	for position, character := range []byte(value) {
		if position != 4 && position != 7 && (character < '0' || character > '9') {
			return false
		}
	}

	year, month, day := searchDecimalDigits(value[:4]), searchDecimalDigits(value[5:7]), searchDecimalDigits(value[8:])
	if month < 1 || month > 12 || day < 1 {
		return false
	}

	days := [...]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

	maximum := days[month]
	if month == 2 && searchLeapYear(year) {
		maximum = 29
	}

	return day <= maximum
}

func searchDateTimeFormatMatches(value string) bool {
	if len(value) < 20 || value[10] != 'T' || !searchDateFormatMatches(value[:10]) {
		return false
	}

	if !searchTwoDigits(value[11:13]) || value[13] != ':' || !searchTwoDigits(value[14:16]) ||
		value[16] != ':' || !searchTwoDigits(value[17:19]) {
		return false
	}

	if searchDecimalDigits(value[11:13]) > 23 || searchDecimalDigits(value[14:16]) > 59 ||
		searchDecimalDigits(value[17:19]) > 59 {
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

	if position < len(value) && value[position] == 'Z' {
		return position+1 == len(value)
	}

	if position == len(value) || value[position] != '+' && value[position] != '-' || position+6 != len(value) ||
		!searchTwoDigits(value[position+1:position+3]) || value[position+3] != ':' ||
		!searchTwoDigits(value[position+4:position+6]) {
		return false
	}

	return searchDecimalDigits(value[position+1:position+3]) <= 23 &&
		searchDecimalDigits(value[position+4:position+6]) <= 59
}

func searchIPv4FormatMatches(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if !searchIPv4Octet(part, false) {
			return false
		}
	}

	return true
}

func searchCIDRFormatMatches(value string) bool {
	address, prefix, found := strings.Cut(value, "/")
	if !found || strings.ContainsRune(prefix, '/') || !searchIPv4FormatMatches(address) ||
		prefix == "" || len(prefix) > 2 || len(prefix) > 1 && prefix[0] == '0' {
		return false
	}

	for _, character := range prefix {
		if character < '0' || character > '9' {
			return false
		}
	}

	return searchDecimalDigits(prefix) <= 32
}

func searchIPv4Octet(value string, leadingZeros bool) bool {
	if value == "" || len(value) > 3 || !leadingZeros && len(value) > 1 && value[0] == '0' {
		return false
	}

	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}

	return searchDecimalDigits(value) <= 255
}

func searchUUIDFormatMatches(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' ||
		value[14] != '4' || !strings.ContainsRune("89ABab", rune(value[19])) {
		return false
	}

	for position, character := range []byte(value) {
		if position != 8 && position != 13 && position != 18 && position != 23 && !searchHexCharacter(character) {
			return false
		}
	}

	return true
}

func searchEmailFormatMatches(value string) bool {
	if len(value) == 0 || len(value) > 254 || !searchASCII(value) {
		return false
	}

	separator, ok := searchEmailLocalEnd(value)
	if !ok || separator == 0 || separator >= len(value) || value[separator] != '@' ||
		separator+1 >= len(value) || separator > 64 {
		return false
	}

	return searchEmailDomain(value[separator+1:])
}

func searchEmailLocalEnd(value string) (int, bool) {
	if value[0] == '"' {
		position := 1
		for position < len(value) {
			switch value[position] {
			case '"':
				return position + 1, true
			case '\\':
				position++
				if position == len(value) || value[position] < 0x20 || value[position] > 0x7e {
					return 0, false
				}
			default:
				if value[position] < 0x20 || value[position] == 0x22 || value[position] == 0x5c || value[position] > 0x7e {
					return 0, false
				}
			}

			position++
		}

		return 0, false
	}

	position := 0
	if !searchEmailAtext(value[position]) {
		return 0, false
	}

	for position < len(value) && value[position] != '@' {
		if !searchEmailAtext(value[position]) {
			return 0, false
		}

		position++
		if position < len(value) && value[position] == '.' {
			position++
			if position == len(value) || !searchEmailAtext(value[position]) {
				return 0, false
			}
		}
	}

	return position, position < len(value) && value[position] == '@'
}

func searchEmailDomain(value string) bool {
	// The search program deliberately implements the retained ASCII name grammar.
	// Address-literal alternatives remain independently implemented below.
	if value[0] == '[' {
		return len(value) > 2 && value[len(value)-1] == ']' && searchEmailAddressLiteral(value[1:len(value)-1])
	}

	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, character := range label {
			if !searchASCIIAlphaNumeric(byte(character)) && character != '-' {
				return false
			}
		}
	}

	return true
}

func searchEmailAddressLiteral(value string) bool {
	if len(value) >= 5 && strings.EqualFold(value[:5], "ipv6:") {
		return searchEmailIPv6(value[5:])
	}

	parts := strings.Split(value, ".")
	if len(parts) == 4 {
		valid := true
		for _, part := range parts {
			valid = valid && searchIPv4Octet(part, true)
		}

		if valid {
			return true
		}
	}

	separator := strings.IndexByte(value, ':')
	if separator <= 0 || separator+1 == len(value) || !searchEmailGeneralTag(value[:separator]) {
		return false
	}

	for _, character := range value[separator+1:] {
		if character < '!' || character > '~' || character == '[' || character == '\\' || character == ']' {
			return false
		}
	}

	return true
}

func searchEmailIPv6(value string) bool {
	if strings.Count(value, "::") > 1 {
		return false
	}

	if strings.Contains(value, ".") {
		return searchEmailIPv6WithIPv4Suffix(value)
	}

	if strings.Contains(value, "::") {
		return searchCompressedIPv6(value, 6)
	}

	return searchIPv6Groups(value, 8)
}

func searchEmailIPv6WithIPv4Suffix(value string) bool {
	separator := strings.LastIndexByte(value, ':')
	if separator < 0 {
		return false
	}

	parts := strings.Split(value[separator+1:], ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if !searchIPv4Octet(part, true) {
			return false
		}
	}

	hexPart := value[:separator]
	if strings.HasSuffix(hexPart, "::") {
		return false
	}

	if strings.HasSuffix(hexPart, ":") {
		hexPart += ":"
	}

	if strings.Contains(hexPart, "::") {
		return searchCompressedIPv6(hexPart, 4)
	}

	return searchIPv6Groups(hexPart, 6)
}

func searchCompressedIPv6(value string, maximumExplicit int) bool {
	parts := strings.Split(value, "::")
	if len(parts) != 2 {
		return false
	}

	left, right := searchSplitIPv6Groups(parts[0]), searchSplitIPv6Groups(parts[1])
	if parts[0] != "" && len(left) == 0 || parts[1] != "" && len(right) == 0 || len(left)+len(right) > maximumExplicit {
		return false
	}

	for _, group := range append(left, right...) {
		if !searchIPv6Group(group) {
			return false
		}
	}

	return true
}

func searchIPv6Groups(value string, expected int) bool {
	groups := searchSplitIPv6Groups(value)
	if len(groups) != expected {
		return false
	}

	for _, group := range groups {
		if !searchIPv6Group(group) {
			return false
		}
	}

	return true
}

func searchSplitIPv6Groups(value string) []string {
	if value == "" {
		return nil
	}

	return strings.Split(value, ":")
}

func searchIPv6Group(value string) bool {
	if len(value) == 0 || len(value) > 4 {
		return false
	}

	for _, character := range value {
		if !searchHexCharacter(byte(character)) {
			return false
		}
	}

	return true
}

func searchEmailGeneralTag(value string) bool {
	if value == "" || len(value) == 4 && strings.EqualFold(value, "ipv6") {
		return false
	}

	for _, character := range value {
		if !searchASCIIAlphaNumeric(byte(character)) && character != '-' {
			return false
		}
	}

	return searchASCIIAlphaNumeric(value[len(value)-1])
}

func searchEmailAtext(value byte) bool {
	return searchASCIIAlphaNumeric(value) || strings.ContainsRune("!#$%&'*+/=?^_`{|}~-", rune(value))
}

func searchASCIIAlphaNumeric(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func searchASCII(value string) bool {
	for _, character := range value {
		if character > 0x7f {
			return false
		}
	}

	return true
}

func searchHexCharacter(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'A' && value <= 'F' || value >= 'a' && value <= 'f'
}

func searchTwoDigits(value string) bool {
	return len(value) == 2 && value[0] >= '0' && value[0] <= '9' && value[1] >= '0' && value[1] <= '9'
}

func searchDecimalDigits(value string) int {
	result := 0
	for _, character := range value {
		result = result*10 + int(character-'0')
	}

	return result
}

func searchLeapYear(year int) bool {
	return year%400 == 0 || year%4 == 0 && year%100 != 0
}
