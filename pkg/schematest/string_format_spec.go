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

type stringFormatBoundary struct {
	kind    stringFormatObjective
	bounds  stringFormatBounds
	matches func(stringFormatProgramState) bool
	viable  func(stringFormatProgramState) bool
}

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
	length   int
	position int
	alive    bool
	phase    uint8
	flags    uint16
	counts   [8]uint16
	values   [8]uint16
}

type stringFormatProgram struct {
	alphabet     string
	alphabetLow  uint16
	alphabetHigh uint16
	bounds       stringFormatBounds
	advanceState func(stringFormatProgramState, uint16) stringFormatProgramState
	acceptState  func(stringFormatProgramState) bool
}

func (program *stringFormatProgram) start(length int) stringFormatProgramState {
	return stringFormatProgramState{length: length, alive: length >= 0}
}

func (program *stringFormatProgram) advance(
	state stringFormatProgramState,
	unit uint16,
) stringFormatProgramState {
	if !state.alive || !program.hasUnit(unit) || state.position == state.length ||
		program.bounds.bounded && uint64(state.position+1) > program.bounds.maximum {
		return stringFormatProgramState{length: state.length}
	}

	return program.advanceState(state, unit)
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
	return state.alive && state.position == state.length &&
		program.bounds.allows(uint64(state.position)) && program.acceptState(state)
}

func (program *stringFormatProgram) transition(state stringFormatProgramState, unit uint16) uint32 {
	if !program.advance(state, unit).alive {
		return 0
	}

	return uint32(unit) + 1
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
	formats            [3]schemaFormat
	names              [3]string
	registrationCount  uint8
	program            *stringFormatProgram
	bounds             stringFormatBounds
	objectives         [4]stringFormatBoundary
	objectiveCount     uint8
	negativeObjectives [3][4]string
	negativeCounts     [3]uint8
	inert              bool
}

var (
	base64FormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatByte}, []string{"byte"},
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",
		stringFormatBounds{multiple: 4},
		[][]string{{"YQ="}},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical},
		stringFormatBoundary{kind: stringFormatObjectivePadding},
	)
	dateFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatDate}, []string{"date"},
		"0123456789-", exactStringFormatBounds(10),
		[][]string{{"2001-02-29", "1900-02-29", "1970-13-01", "1970-01-32"}},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical},
		stringFormatBoundary{kind: stringFormatObjectiveLeapBoundary},
		stringFormatBoundary{kind: stringFormatObjectiveCenturyBoundary},
		stringFormatBoundary{kind: stringFormatObjectiveUpperBoundary},
	)
	dateTimeFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatDateTime}, []string{"date-time"},
		"0123456789-T:.Z+", stringFormatBounds{minimum: 20},
		[][]string{{"1970-01-01t00:00:00Z", "1970-01-01T00:00:60Z", "1970-01-01T00:00:00.Z", "1970-01-01T00:00:00+24:00"}},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical},
		stringFormatBoundary{kind: stringFormatObjectiveLeapBoundary},
		stringFormatBoundary{kind: stringFormatObjectiveCenturyBoundary},
		stringFormatBoundary{kind: stringFormatObjectiveUpperBoundary},
	)
	emailFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatEmail}, []string{"email"},
		"", stringFormatBounds{minimum: 3, maximum: 254, bounded: true},
		[][]string{{
			"a..b@example.com", strings.Repeat("a", 65) + "@b",
			strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." +
				strings.Repeat("c", 63) + "." + strings.Repeat("d", 62),
			"é@example.com",
		}},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical},
		stringFormatBoundary{kind: stringFormatObjectiveLocalLimit},
		stringFormatBoundary{kind: stringFormatObjectiveDomainLimit},
	)
	ipv4FormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatIPv4}, []string{"ipv4"},
		"0123456789.", stringFormatBounds{minimum: 7, maximum: 15, bounded: true},
		[][]string{{"00.0.0.0", "256.255.255.255"}},
		stringFormatBoundary{kind: stringFormatObjectiveLowerBoundary},
		stringFormatBoundary{kind: stringFormatObjectiveUpperBoundary},
	)
	uuidFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4},
		[]string{"uuid", "uuidv4", "uuid-v4"},
		"0123456789ABCDEFabcdef-", exactStringFormatBounds(36),
		[][]string{
			{"00000000-0000-1000-8000-000000000000"},
			{"00000000-0000-4000-7000-000000000000"},
			{"00000000-0000-4000-7000-000000000000"},
		},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical},
	)
	cidrFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatCIDR, schemaFormatIPv4CIDR}, []string{"cidr", "ipv4-cidr"},
		"0123456789./", stringFormatBounds{minimum: 9, maximum: 18, bounded: true},
		[][]string{
			{"192.0.2.7/33", "192.0.2.7/00"},
			{"192.0.2.7/33", "192.0.2.7/00"},
		},
		stringFormatBoundary{kind: stringFormatObjectiveLowerBoundary},
		stringFormatBoundary{kind: stringFormatObjectiveUpperBoundary},
	)
	passwordFormatSpecification = newInertStringFormatSpecification(schemaFormatPassword, "password")
)

func newStringFormatSpecification(
	formats []schemaFormat,
	names []string,
	alphabet string,
	bounds stringFormatBounds,
	negativeObjectives [][]string,
	objectives ...stringFormatBoundary,
) *stringFormatSpecification {
	for index := range objectives {
		objectives[index].bounds = stringFormatObjectiveBounds(formats[0], objectives[index].kind)
		objectives[index].matches = stringFormatObjectiveStateMatcher(formats[0], objectives[index].kind)
		objectives[index].viable = stringFormatObjectiveStateViable(formats[0], objectives[index].kind)
	}

	program := newStringFormatProgram(formats[0], alphabet, bounds)

	specification := &stringFormatSpecification{
		registrationCount: uint8(len(formats)),
		program:           program,
		bounds:            bounds,
		objectiveCount:    uint8(len(objectives)),
	}
	copy(specification.formats[:], formats)
	copy(specification.names[:], names)
	copy(specification.objectives[:], objectives)

	for formatIndex := range negativeObjectives {
		specification.negativeCounts[formatIndex] = uint8(len(negativeObjectives[formatIndex]))
		copy(specification.negativeObjectives[formatIndex][:], negativeObjectives[formatIndex])
	}

	return specification
}

func newInertStringFormatSpecification(format schemaFormat, name string) *stringFormatSpecification {
	return &stringFormatSpecification{
		formats: [3]schemaFormat{format}, names: [3]string{name}, registrationCount: 1, inert: true,
	}
}

func exactStringFormatBounds(length uint64) stringFormatBounds {
	return stringFormatBounds{minimum: length, maximum: length, bounded: true}
}

func eachStringFormatSpecification(visit func(*stringFormatSpecification) bool) {
	if visit(base64FormatSpecification) || visit(dateFormatSpecification) ||
		visit(dateTimeFormatSpecification) || visit(emailFormatSpecification) ||
		visit(ipv4FormatSpecification) || visit(uuidFormatSpecification) ||
		visit(cidrFormatSpecification) {
		return
	}

	visit(passwordFormatSpecification)
}

func stringFormatSpecificationFor(format schemaFormat) (*stringFormatSpecification, bool) {
	var found *stringFormatSpecification

	eachStringFormatSpecification(func(specification *stringFormatSpecification) bool {
		for index := 0; index < int(specification.registrationCount); index++ {
			if specification.formats[index] == format {
				found = specification

				return true
			}
		}

		return false
	})

	return found, found != nil
}

func stringFormatByName(name string) (schemaFormat, bool) {
	format := schemaFormatNone

	eachStringFormatSpecification(func(specification *stringFormatSpecification) bool {
		for index := 0; index < int(specification.registrationCount); index++ {
			if specification.names[index] == name {
				format = specification.formats[index]

				return true
			}
		}

		return false
	})

	return format, format != schemaFormatNone
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
	if separator > 0 && value[separator-1] == ':' {
		hexPart = value[:separator+1]
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
