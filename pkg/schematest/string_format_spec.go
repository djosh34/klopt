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
	witness string
	matches func(string) bool
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
	preferred       func(int, int) uint16
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
		stringFormatBounds{multiple: 4}, searchByteFormatMatches, searchByteFormatPrefixViable,
		searchByteFormatTransitionClass,
		[][]string{{"YQ="}},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical, witness: "YQ=="},
		stringFormatBoundary{kind: stringFormatObjectivePadding, witness: "YWI="},
	)
	dateFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatDate}, []string{"date"},
		"0123456789-", exactStringFormatBounds(10), searchDateFormatMatches, searchDateFormatPrefixViable,
		searchDateFormatTransitionClass,
		[][]string{{"2001-02-29", "1900-02-29", "1970-13-01", "1970-01-32"}},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical, witness: "1970-01-01"},
		stringFormatBoundary{kind: stringFormatObjectiveLeapBoundary, witness: "2000-02-29"},
		stringFormatBoundary{kind: stringFormatObjectiveCenturyBoundary, witness: "1900-02-28"},
		stringFormatBoundary{kind: stringFormatObjectiveUpperBoundary, witness: "9999-12-31"},
	)
	dateTimeFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatDateTime}, []string{"date-time"},
		"0123456789-T:.Z+", stringFormatBounds{minimum: 20}, searchDateTimeFormatMatches,
		searchDateTimeFormatPrefixViable, searchDateFormatTransitionClass,
		[][]string{{"1970-01-01t00:00:00Z", "1970-01-01T00:00:60Z", "1970-01-01T00:00:00.Z", "1970-01-01T00:00:00+24:00"}},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical, witness: "1970-01-01T00:00:00Z"},
		stringFormatBoundary{kind: stringFormatObjectiveLeapBoundary, witness: "2000-02-29T23:59:59.0Z"},
		stringFormatBoundary{kind: stringFormatObjectiveCenturyBoundary, witness: "1900-02-28T00:00:00+23:59"},
		stringFormatBoundary{kind: stringFormatObjectiveUpperBoundary, witness: "9999-12-31T23:59:59-23:59"},
	)
	emailFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatEmail}, []string{"email"},
		"", stringFormatBounds{minimum: 3, maximum: 254, bounded: true}, searchEmailFormatMatches,
		searchEmailFormatPrefixViable, searchEmailFormatTransitionClass,
		[][]string{{
			"a..b@example.com", strings.Repeat("a", 65) + "@b",
			strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." +
				strings.Repeat("c", 63) + "." + strings.Repeat("d", 62),
			"é@example.com",
		}},
		stringFormatBoundary{kind: stringFormatObjectiveCanonical, witness: "a@b"},
		stringFormatBoundary{kind: stringFormatObjectiveLocalLimit, witness: strings.Repeat("a", 64) + "@b"},
		stringFormatBoundary{
			kind: stringFormatObjectiveDomainLimit,
			witness: strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." +
				strings.Repeat("c", 63) + "." + strings.Repeat("d", 61),
		},
	)
	ipv4FormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatIPv4}, []string{"ipv4"},
		"0123456789.", stringFormatBounds{minimum: 7, maximum: 15, bounded: true}, searchIPv4FormatMatches,
		searchIPv4FormatPrefixViable, nil,
		[][]string{{"00.0.0.0", "256.255.255.255"}},
		stringFormatBoundary{kind: stringFormatObjectiveLowerBoundary, witness: "0.0.0.0"},
		stringFormatBoundary{kind: stringFormatObjectiveUpperBoundary, witness: "255.255.255.255"},
	)
	uuidFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4},
		[]string{"uuid", "uuidv4", "uuid-v4"},
		"0123456789ABCDEFabcdef-", exactStringFormatBounds(36), searchUUIDFormatMatches,
		searchUUIDFormatPrefixViable, nil,
		[][]string{
			{"00000000-0000-1000-8000-000000000000"},
			{"00000000-0000-4000-7000-000000000000"},
			{"00000000-0000-4000-7000-000000000000"},
		},
		stringFormatBoundary{
			kind:    stringFormatObjectiveCanonical,
			witness: "00000000-0000-4000-8000-000000000000",
		},
	)
	cidrFormatSpecification = newStringFormatSpecification(
		[]schemaFormat{schemaFormatCIDR, schemaFormatIPv4CIDR}, []string{"cidr", "ipv4-cidr"},
		"0123456789./", stringFormatBounds{minimum: 9, maximum: 18, bounded: true}, searchCIDRFormatMatches,
		searchCIDRFormatPrefixViable, nil,
		[][]string{
			{"192.0.2.7/33", "192.0.2.7/00"},
			{"192.0.2.7/33", "192.0.2.7/00"},
		},
		stringFormatBoundary{kind: stringFormatObjectiveLowerBoundary, witness: "192.0.2.7/0"},
		stringFormatBoundary{kind: stringFormatObjectiveUpperBoundary, witness: "192.0.2.7/32"},
	)
	passwordFormatSpecification = newInertStringFormatSpecification(schemaFormatPassword, "password")
)

func stringFormatPreferred(format schemaFormat) func(int, int) uint16 {
	return func(position, length int) uint16 {
		switch format {
		case schemaFormatByte:
			return '+'
		case schemaFormatDate:
			return uint16("0000-01-01"[position])
		case schemaFormatDateTime:
			if position < 19 {
				return uint16("0000-01-01T00:00:00"[position])
			}

			if position == length-1 {
				return 'Z'
			}

			if position == 19 {
				return '.'
			}

			return '0'
		case schemaFormatEmail:
			localLength := min(emailLocalLimit, length-2)
			if position < localLength {
				return 'a'
			}

			if position == localLength {
				return '@'
			}

			return 'b'
		case schemaFormatIPv4:
			return searchIPv4Preferred(position, length)
		case schemaFormatUUID:
			return uint16("00000000-0000-4000-8000-000000000000"[position])
		case schemaFormatCIDR:
			addressLength := min(15, length-2)
			if position < addressLength {
				return searchIPv4Preferred(position, addressLength)
			}

			if position == addressLength {
				return '/'
			}

			if length-addressLength-1 == 1 {
				return '0'
			}

			if position == addressLength+1 {
				return '1'
			}

			return '0'
		default:
			return 0
		}
	}
}

func searchIPv4Preferred(position, length int) uint16 {
	widths := [4]int{1, 1, 1, 1}

	remaining := length - 7
	for index := len(widths) - 1; index >= 0 && remaining > 0; index-- {
		add := min(2, remaining)
		widths[index] += add
		remaining -= add
	}

	current := 0
	for index, width := range widths {
		if position < current+width {
			if width == 1 {
				return '0'
			}

			if position == current {
				return '1'
			}

			return '0'
		}

		current += width
		if index < len(widths)-1 {
			if position == current {
				return '.'
			}

			current++
		}
	}

	return 0
}

//nolint:gocognit // Closed format objectives have one explicit semantic predicate each.
func stringFormatObjectiveMatcher(format schemaFormat, objective stringFormatObjective) func(string) bool {
	return func(candidate string) bool {
		switch format {
		case schemaFormatByte:
			if objective == stringFormatObjectivePadding {
				return strings.HasSuffix(candidate, "=") && !strings.HasSuffix(candidate, "==")
			}

			return strings.HasSuffix(candidate, "==")
		case schemaFormatDate, schemaFormatDateTime:
			if len(candidate) < 10 || !searchDateFormatMatches(candidate[:10]) {
				return false
			}

			year := searchDecimalDigits(candidate[:4])
			month := searchDecimalDigits(candidate[5:7])
			day := searchDecimalDigits(candidate[8:10])

			switch objective {
			case stringFormatObjectiveCanonical:
				return year == 1970
			case stringFormatObjectiveLeapBoundary:
				return month == 2 && day == 29
			case stringFormatObjectiveCenturyBoundary:
				return year%100 == 0 && year%400 != 0 && month == 2 && day == 28
			case stringFormatObjectiveUpperBoundary:
				return year == 9999 && month == 12 && day == 31
			}
		case schemaFormatEmail:
			separator, ok := searchEmailLocalEnd(candidate)
			if !ok {
				return false
			}

			switch objective {
			case stringFormatObjectiveCanonical:
				return separator == 1 && len(candidate) == 3
			case stringFormatObjectiveLocalLimit:
				return separator == emailLocalLimit
			case stringFormatObjectiveDomainLimit:
				return len(candidate) == 254
			}
		case schemaFormatIPv4:
			if objective == stringFormatObjectiveLowerBoundary {
				return candidate == "0.0.0.0"
			}

			return candidate == "255.255.255.255"
		case schemaFormatUUID:
			return true
		case schemaFormatCIDR:
			_, prefix, found := strings.Cut(candidate, "/")
			if !found {
				return false
			}

			if objective == stringFormatObjectiveLowerBoundary {
				return prefix == "0"
			}

			return prefix == "32"
		}

		return false
	}
}

func newStringFormatSpecification(
	formats []schemaFormat,
	names []string,
	alphabet string,
	bounds stringFormatBounds,
	match func(string) bool,
	prefix func(string, int) bool,
	transitionClass func(string, uint16) uint32,
	negativeObjectives [][]string,
	objectives ...stringFormatBoundary,
) *stringFormatSpecification {
	if prefix == nil {
		prefix = func(candidate string, length int) bool {
			return len(candidate) < length || match(candidate)
		}
	}

	for index := range objectives {
		objectives[index].matches = stringFormatObjectiveMatcher(formats[0], objectives[index].kind)
	}

	program := &stringFormatProgram{
		alphabet: alphabet, bounds: bounds, match: match, prefix: prefix,
		transitionClass: transitionClass, preferred: stringFormatPreferred(formats[0]),
	}
	if alphabet == "" {
		program.alphabetLow = 0x20
		program.alphabetHigh = 0x7e
	}

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

//nolint:gocognit,gocyclo // Date, time, fraction, and offset prefixes are one incremental grammar.
func searchDateTimeFormatPrefixViable(prefix string, length int) bool {
	if length < 20 || len(prefix) > length {
		return false
	}

	if len(prefix) <= 10 {
		return searchDateFormatPrefixViable(prefix, 10)
	}

	if !searchDateFormatMatches(prefix[:10]) || prefix[10] != 'T' {
		return false
	}

	for position := 11; position < len(prefix) && position < 19; position++ {
		if position == 13 || position == 16 {
			if prefix[position] != ':' {
				return false
			}
		} else if prefix[position] < '0' || prefix[position] > '9' {
			return false
		}
	}

	if len(prefix) >= 13 && searchDecimalDigits(prefix[11:13]) > 23 ||
		len(prefix) >= 16 && searchDecimalDigits(prefix[14:16]) > 59 ||
		len(prefix) >= 19 && searchDecimalDigits(prefix[17:19]) > 59 {
		return false
	}

	if len(prefix) <= 19 {
		return true
	}

	tail := prefix[19:]
	if tail[0] == 'Z' {
		return len(tail) == 1 && len(prefix) == length
	}

	if tail[0] == '+' || tail[0] == '-' {
		return searchDateTimeOffsetPrefixViable(tail, len(prefix) == length)
	}

	if tail[0] != '.' {
		return false
	}

	if len(tail) == 1 {
		return len(prefix) < length
	}

	position := 1
	for position < len(tail) && tail[position] >= '0' && tail[position] <= '9' {
		position++
	}

	if position == 1 {
		return false
	}

	if position == len(tail) {
		return len(prefix) < length
	}

	if tail[position] == 'Z' {
		return position+1 == len(tail) && len(prefix) == length
	}

	if tail[position] != '+' && tail[position] != '-' {
		return false
	}

	return searchDateTimeOffsetPrefixViable(tail[position:], len(prefix) == length)
}

func searchDateTimeOffsetPrefixViable(offset string, complete bool) bool {
	if len(offset) > 6 {
		return false
	}

	for position := 1; position < len(offset); position++ {
		if position == 3 {
			if offset[position] != ':' {
				return false
			}
		} else if offset[position] < '0' || offset[position] > '9' {
			return false
		}
	}

	return (!complete || len(offset) == 6) &&
		(len(offset) < 3 || searchDecimalDigits(offset[1:3]) <= 23) &&
		(len(offset) < 6 || searchDecimalDigits(offset[4:6]) <= 59)
}

func searchIPv4FormatPrefixViable(prefix string, length int) bool {
	if length < 7 || length > 15 || len(prefix) > length {
		return false
	}

	if len(prefix) == length {
		return searchIPv4FormatMatches(prefix)
	}

	parts := strings.Split(prefix, ".")
	if len(parts) > 4 {
		return false
	}

	for index, part := range parts {
		if index < len(parts)-1 {
			if !searchIPv4Octet(part, false) {
				return false
			}
		} else if len(part) > 3 || len(part) > 1 && part[0] == '0' ||
			len(part) == 3 && searchDecimalDigits(part) > 255 {
			return false
		}
	}

	return true
}

func searchCIDRFormatPrefixViable(prefix string, length int) bool {
	if length < 9 || length > 18 || len(prefix) > length {
		return false
	}

	if len(prefix) == length {
		return searchCIDRFormatMatches(prefix)
	}

	address, cidrPrefix, found := strings.Cut(prefix, "/")
	if !found {
		return searchIPv4FormatPrefixViable(address, max(7, min(15, len(address)+1)))
	}

	if !searchIPv4FormatMatches(address) || strings.ContainsRune(cidrPrefix, '/') || len(cidrPrefix) > 2 ||
		len(cidrPrefix) > 1 && cidrPrefix[0] == '0' {
		return false
	}

	for _, character := range cidrPrefix {
		if character < '0' || character > '9' {
			return false
		}
	}

	return cidrPrefix == "" || searchDecimalDigits(cidrPrefix) <= 32
}

func searchUUIDFormatPrefixViable(prefix string, length int) bool {
	if length != 36 || len(prefix) > length {
		return false
	}

	for position, character := range []byte(prefix) {
		switch position {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		case 14:
			if character != '4' {
				return false
			}
		case 19:
			if !strings.ContainsRune("89ABab", rune(character)) {
				return false
			}
		default:
			if !searchHexCharacter(character) {
				return false
			}
		}
	}

	return true
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
