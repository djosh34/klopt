//nolint:cyclop,gocognit,godoclint,mnd // Closed incremental format automata are intentionally explicit.
package schematest

import "strings"

const (
	formatFlagLocalLastDot uint16 = 1 << iota
	formatFlagIPv4
	formatFlagGeneral
	formatFlagIPv6
	formatFlagGeneralLastAlphaNumeric
	formatFlagGeneralMatchesIPv6
	formatFlagIPv6Compressed
	formatFlagIPv6PreviousColon
)

const (
	emailPhaseLocal uint8 = iota
	emailPhaseQuoted
	emailPhaseQuotedEscape
	emailPhaseAfterQuote
	emailPhaseDomainName
	emailPhaseLiteral
)

func stringFormatObjectiveBounds(format schemaFormat, objective stringFormatObjective) stringFormatBounds {
	switch format {
	case schemaFormatByte:
		return stringFormatBounds{minimum: 4, multiple: 4}
	case schemaFormatDate:
		return exactStringFormatBounds(10)
	case schemaFormatDateTime:
		return stringFormatBounds{minimum: 20}
	case schemaFormatEmail:
		switch objective {
		case stringFormatObjectiveCanonical:
			return exactStringFormatBounds(3)
		case stringFormatObjectiveLocalLimit:
			return stringFormatBounds{minimum: 66, maximum: 254, bounded: true}
		case stringFormatObjectiveDomainLimit:
			return exactStringFormatBounds(254)
		}
	case schemaFormatIPv4:
		if objective == stringFormatObjectiveLowerBoundary {
			return exactStringFormatBounds(7)
		}

		return exactStringFormatBounds(15)
	case schemaFormatUUID:
		return exactStringFormatBounds(36)
	case schemaFormatCIDR:
		if objective == stringFormatObjectiveLowerBoundary {
			return stringFormatBounds{minimum: 9, maximum: 17, bounded: true}
		}

		return stringFormatBounds{minimum: 10, maximum: 18, bounded: true}
	}

	return stringFormatBounds{}
}

func stringFormatObjectiveStateViable(
	format schemaFormat,
	objective stringFormatObjective,
) func(stringFormatProgramState) bool {
	return func(state stringFormatProgramState) bool {
		if !state.alive || !stringFormatObjectiveBounds(format, objective).allows(uint64(state.length)) {
			return false
		}

		switch format {
		case schemaFormatDate, schemaFormatDateTime:
			return dateObjectiveStateViable(state, objective)
		case schemaFormatEmail:
			if objective == stringFormatObjectiveLocalLimit && state.phase >= emailPhaseDomainName {
				return state.counts[0] == emailLocalLimit
			}
		case schemaFormatIPv4:
			if objective == stringFormatObjectiveLowerBoundary {
				return state.counts[6] == state.counts[1] &&
					(state.counts[0] == 0 || state.counts[0] == 1 && state.values[0] == 0)
			}

			currentMaximumPrefix := state.counts[0] == 0 ||
				state.counts[0] == 1 && state.values[0] == 2 ||
				state.counts[0] == 2 && state.values[0] == 25 ||
				state.counts[0] == 3 && state.values[0] == 255

			return state.counts[7] == state.counts[1] && currentMaximumPrefix
		case schemaFormatCIDR:
			if state.phase == 1 && state.counts[2] > 0 {
				if objective == stringFormatObjectiveLowerBoundary {
					return state.values[1] == 0
				}

				return state.values[1] <= 32 && (state.counts[2] == 1 && state.values[1] == 3 || state.values[1] == 32)
			}
		}

		return true
	}
}

//nolint:nestif // Progressive year, month, and day constraints are one semantic state check.
func dateObjectiveStateViable(state stringFormatProgramState, objective stringFormatObjective) bool {
	if state.position >= 4 {
		year := state.values[0]

		switch objective {
		case stringFormatObjectiveCanonical:
			if year != 1970 {
				return false
			}
		case stringFormatObjectiveCenturyBoundary:
			if year%100 != 0 || year%400 == 0 {
				return false
			}
		case stringFormatObjectiveUpperBoundary:
			if year != 9999 {
				return false
			}
		}
	} else if objective == stringFormatObjectiveCanonical || objective == stringFormatObjectiveUpperBoundary {
		target := uint16(1970)
		if objective == stringFormatObjectiveUpperBoundary {
			target = 9999
		}

		divisor := [...]uint16{1000, 100, 10, 1}[state.position]
		if state.values[0] != target/(divisor*10) {
			return false
		}
	}

	if state.position >= 7 {
		month := state.values[1]

		februaryBoundary := objective == stringFormatObjectiveLeapBoundary ||
			objective == stringFormatObjectiveCenturyBoundary
		if februaryBoundary && month != 2 || objective == stringFormatObjectiveUpperBoundary && month != 12 {
			return false
		}
	}

	if state.position >= 10 {
		day := state.values[2]

		switch objective {
		case stringFormatObjectiveLeapBoundary:
			return day == 29 && searchLeapYear(int(state.values[0]))
		case stringFormatObjectiveCenturyBoundary:
			return day == 28
		case stringFormatObjectiveUpperBoundary:
			return day == 31
		}
	}

	return true
}

func stringFormatObjectiveStateMatcher(
	format schemaFormat,
	objective stringFormatObjective,
) func(stringFormatProgramState) bool {
	return func(state stringFormatProgramState) bool {
		switch format {
		case schemaFormatByte:
			if objective == stringFormatObjectivePadding {
				return state.counts[2] == 1
			}

			return state.counts[2] == 2
		case schemaFormatDate, schemaFormatDateTime:
			year, month, day := state.values[0], state.values[1], state.values[2]

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
			switch objective {
			case stringFormatObjectiveCanonical:
				return state.counts[0] == 1 && state.length == 3
			case stringFormatObjectiveLocalLimit:
				return state.counts[0] == emailLocalLimit
			case stringFormatObjectiveDomainLimit:
				return state.length == 254
			}
		case schemaFormatIPv4:
			zeros, maximums := ipv4ObjectiveCounts(state)
			if objective == stringFormatObjectiveLowerBoundary {
				return zeros == 4
			}

			return maximums == 4
		case schemaFormatUUID:
			return true
		case schemaFormatCIDR:
			if objective == stringFormatObjectiveLowerBoundary {
				return state.values[1] == 0
			}

			return state.values[1] == 32
		}

		return false
	}
}

func newStringFormatProgram(format schemaFormat, alphabet string, bounds stringFormatBounds) *stringFormatProgram {
	program := &stringFormatProgram{alphabet: alphabet, bounds: bounds}
	if alphabet == "" {
		program.alphabetLow = 0x20
		program.alphabetHigh = 0x7e
	}

	switch format {
	case schemaFormatByte:
		program.advanceState, program.acceptState = advanceBase64Format, acceptBase64Format
	case schemaFormatDate:
		program.advanceState, program.acceptState = advanceDateFormat, acceptDateFormat
	case schemaFormatDateTime:
		program.advanceState, program.acceptState = advanceDateTimeFormat, acceptDateTimeFormat
	case schemaFormatEmail:
		program.advanceState, program.acceptState = advanceEmailFormat, acceptEmailFormat
	case schemaFormatIPv4:
		program.advanceState, program.acceptState = advanceIPv4Format, acceptIPv4Format
	case schemaFormatUUID:
		program.advanceState, program.acceptState = advanceUUIDFormat, acceptUUIDFormat
	case schemaFormatCIDR:
		program.advanceState, program.acceptState = advanceCIDRFormat, acceptCIDRFormat
	default:
		program.advanceState = func(state stringFormatProgramState, _ uint16) stringFormatProgramState {
			return deadStringFormatState(state)
		}
		program.acceptState = func(stringFormatProgramState) bool { return false }
	}

	return program
}

func deadStringFormatState(state stringFormatProgramState) stringFormatProgramState {
	return stringFormatProgramState{length: state.length}
}

func advancedStringFormatState(state stringFormatProgramState) stringFormatProgramState {
	state.position++

	return state
}

func base64UnitValue(unit uint16) int {
	if unit > 0xff {
		return -1
	}

	return searchBase64Value(byte(unit))
}

func advanceBase64Format(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	quartetPosition := state.position % 4
	value := base64UnitValue(unit)

	switch quartetPosition {
	case 0:
		if value < 0 {
			return deadStringFormatState(state)
		}
	case 1:
		if value < 0 {
			return deadStringFormatState(state)
		}

		state.values[0] = uint16(value)
	case 2:
		if unit == '=' {
			if state.values[0]&15 != 0 || state.position+2 != state.length {
				return deadStringFormatState(state)
			}

			state.phase = 1
			state.counts[2] = 1
		} else if value < 0 {
			return deadStringFormatState(state)
		} else {
			state.values[1] = uint16(value)
		}
	case 3:
		switch {
		case state.phase == 1:
			if unit != '=' {
				return deadStringFormatState(state)
			}

			state.phase = 2
			state.counts[2] = 2
		case unit == '=':
			if state.values[1]&3 != 0 || state.position+1 != state.length {
				return deadStringFormatState(state)
			}

			state.phase = 2
			state.counts[2] = 1
		case value < 0:
			return deadStringFormatState(state)
		}
	}

	return advancedStringFormatState(state)
}

func acceptBase64Format(state stringFormatProgramState) bool {
	return state.position%4 == 0 && state.phase != 1
}

func advanceDateFormat(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	if !advanceDatePrefix(&state, unit) {
		return deadStringFormatState(state)
	}

	return advancedStringFormatState(state)
}

func advanceDatePrefix(state *stringFormatProgramState, unit uint16) bool {
	position := state.position
	if position == 4 || position == 7 {
		return unit == '-'
	}

	if unit < '0' || unit > '9' {
		return false
	}

	digit := unit - '0'

	switch {
	case position < 4:
		state.values[0] = state.values[0]*10 + digit
	case position < 7:
		state.values[1] = state.values[1]*10 + digit
	case position < 10:
		state.values[2] = state.values[2]*10 + digit
	default:
		return false
	}

	return true
}

func acceptDateFormat(state stringFormatProgramState) bool {
	return state.position == 10 && validFormatDate(state.values[0], state.values[1], state.values[2])
}

func validFormatDate(year, month, day uint16) bool {
	if month < 1 || month > 12 || day < 1 {
		return false
	}

	days := [...]uint16{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

	maximum := days[month]
	if month == 2 && searchLeapYear(int(year)) {
		maximum = 29
	}

	return day <= maximum
}

//nolint:gocyclo,nestif // Fixed date/time fields and variable suffix phases form one automaton transition.
func advanceDateTimeFormat(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	position := state.position
	if position < 10 {
		if !advanceDatePrefix(&state, unit) {
			return deadStringFormatState(state)
		}

		return advancedStringFormatState(state)
	}

	if position == 10 {
		if unit != 'T' || !validFormatDate(state.values[0], state.values[1], state.values[2]) {
			return deadStringFormatState(state)
		}

		return advancedStringFormatState(state)
	}

	if position < 19 {
		if position == 13 || position == 16 {
			if unit != ':' {
				return deadStringFormatState(state)
			}
		} else {
			if unit < '0' || unit > '9' {
				return deadStringFormatState(state)
			}

			valueIndex := 3 + (position-11)/3
			state.values[valueIndex] = state.values[valueIndex]*10 + unit - '0'
		}

		return advancedStringFormatState(state)
	}

	if state.values[3] > 23 || state.values[4] > 59 || state.values[5] > 59 {
		return deadStringFormatState(state)
	}

	switch state.phase {
	case 0:
		switch unit {
		case 'Z':
			if position+1 != state.length {
				return deadStringFormatState(state)
			}

			state.phase = 3
		case '.':
			state.phase = 1
		case '+', '-':
			state.phase = 2
			state.counts[0] = 1
		default:
			return deadStringFormatState(state)
		}
	case 1:
		if unit >= '0' && unit <= '9' {
			state.counts[1]++
		} else if state.counts[1] > 0 && unit == 'Z' && position+1 == state.length {
			state.phase = 3
		} else if state.counts[1] > 0 && (unit == '+' || unit == '-') {
			state.phase = 2
			state.counts[0] = 1
		} else {
			return deadStringFormatState(state)
		}
	case 2:
		offsetPosition := state.counts[0]
		if offsetPosition == 3 {
			if unit != ':' {
				return deadStringFormatState(state)
			}
		} else {
			if unit < '0' || unit > '9' || offsetPosition > 5 {
				return deadStringFormatState(state)
			}

			valueIndex := 6
			if offsetPosition > 3 {
				valueIndex = 7
			}

			state.values[valueIndex] = state.values[valueIndex]*10 + unit - '0'
		}

		state.counts[0]++
	default:
		return deadStringFormatState(state)
	}

	return advancedStringFormatState(state)
}

func acceptDateTimeFormat(state stringFormatProgramState) bool {
	if state.position < 20 || !validFormatDate(state.values[0], state.values[1], state.values[2]) ||
		state.values[3] > 23 || state.values[4] > 59 || state.values[5] > 59 {
		return false
	}

	return state.phase == 3 || state.phase == 2 && state.counts[0] == 6 &&
		state.values[6] <= 23 && state.values[7] <= 59
}

func advanceIPv4Format(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	if !advanceIPv4Component(&state, unit, 0, 0) {
		return deadStringFormatState(state)
	}

	return advancedStringFormatState(state)
}

func advanceIPv4Component(state *stringFormatProgramState, unit uint16, countIndex, valueIndex int) bool {
	width := state.counts[countIndex]
	value := state.values[valueIndex]
	octets := state.counts[countIndex+1]

	if unit == '.' {
		if width == 0 || octets >= 3 || width > 1 && state.flags&1 != 0 {
			return false
		}

		if value == 0 {
			state.counts[6]++
		}

		if value == 255 {
			state.counts[7]++
		}

		state.counts[countIndex] = 0
		state.counts[countIndex+1]++
		state.values[valueIndex] = 0
		state.flags &^= 1

		return true
	}

	if unit < '0' || unit > '9' || width == 3 || width > 0 && state.flags&1 != 0 {
		return false
	}

	value = value*10 + unit - '0'
	if value > 255 {
		return false
	}

	if width == 0 && unit == '0' {
		state.flags |= 1
	}

	state.counts[countIndex]++
	state.values[valueIndex] = value

	return true
}

func acceptIPv4Format(state stringFormatProgramState) bool {
	return state.counts[1] == 3 && state.counts[0] > 0
}

func ipv4ObjectiveCounts(state stringFormatProgramState) (uint16, uint16) {
	zeros, maximums := state.counts[6], state.counts[7]
	if state.values[0] == 0 {
		zeros++
	}

	if state.values[0] == 255 {
		maximums++
	}

	return zeros, maximums
}

func advanceCIDRFormat(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	if state.phase == 0 {
		if unit == '/' {
			if !acceptIPv4Format(state) {
				return deadStringFormatState(state)
			}

			state.phase = 1
			state.flags = 0

			return advancedStringFormatState(state)
		}

		if !advanceIPv4Component(&state, unit, 0, 0) {
			return deadStringFormatState(state)
		}

		return advancedStringFormatState(state)
	}

	if unit < '0' || unit > '9' || state.counts[2] == 2 ||
		state.counts[2] > 0 && state.flags&1 != 0 {
		return deadStringFormatState(state)
	}

	if state.counts[2] == 0 && unit == '0' {
		state.flags |= 1
	}

	state.values[1] = state.values[1]*10 + unit - '0'
	if state.values[1] > 32 {
		return deadStringFormatState(state)
	}

	state.counts[2]++

	return advancedStringFormatState(state)
}

func acceptCIDRFormat(state stringFormatProgramState) bool {
	return state.phase == 1 && state.counts[2] > 0 && state.values[1] <= 32
}

func advanceUUIDFormat(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	position := state.position
	switch position {
	case 8, 13, 18, 23:
		if unit != '-' {
			return deadStringFormatState(state)
		}
	case 14:
		if unit != '4' {
			return deadStringFormatState(state)
		}
	case 19:
		if !strings.ContainsRune("89ABab", rune(unit)) {
			return deadStringFormatState(state)
		}
	default:
		if unit > 0xff || !searchHexCharacter(byte(unit)) {
			return deadStringFormatState(state)
		}
	}

	return advancedStringFormatState(state)
}

func acceptUUIDFormat(state stringFormatProgramState) bool {
	return state.position == 36
}

func advanceEmailFormat(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	switch state.phase {
	case emailPhaseLocal:
		return advanceEmailLocal(state, unit)
	case emailPhaseQuoted:
		if unit == '\\' {
			state.phase = emailPhaseQuotedEscape
		} else if unit == '"' {
			state.phase = emailPhaseAfterQuote
		} else if unit < 0x20 || unit == 0x22 || unit == 0x5c || unit > 0x7e {
			return deadStringFormatState(state)
		}
	case emailPhaseQuotedEscape:
		if unit < 0x20 || unit > 0x7e {
			return deadStringFormatState(state)
		}

		state.phase = emailPhaseQuoted
	case emailPhaseAfterQuote:
		if unit != '@' || state.position > 64 {
			return deadStringFormatState(state)
		}

		state.phase = emailPhaseDomainName
		state.counts[0] = uint16(state.position)
		state.counts[1] = 0
	case emailPhaseDomainName:
		return advanceEmailDomain(state, unit)
	case emailPhaseLiteral:
		return advanceEmailLiteral(state, unit)
	default:
		return deadStringFormatState(state)
	}

	return advancedStringFormatState(state)
}

func advanceEmailLocal(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	if state.position == 0 && unit == '"' {
		state.phase = emailPhaseQuoted

		return advancedStringFormatState(state)
	}

	if unit == '@' {
		if state.position == 0 || state.position > 64 || state.flags&formatFlagLocalLastDot != 0 {
			return deadStringFormatState(state)
		}

		state.phase = emailPhaseDomainName
		state.counts[0] = uint16(state.position)
		state.counts[1] = 0
		state.flags = 0

		return advancedStringFormatState(state)
	}

	if unit == '.' {
		if state.position == 0 || state.flags&formatFlagLocalLastDot != 0 {
			return deadStringFormatState(state)
		}

		state.flags |= formatFlagLocalLastDot
	} else {
		if unit > 0xff || !searchEmailAtext(byte(unit)) {
			return deadStringFormatState(state)
		}

		state.flags &^= formatFlagLocalLastDot
	}

	if state.position+1 > 64 {
		return deadStringFormatState(state)
	}

	return advancedStringFormatState(state)
}

func advanceEmailDomain(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	if state.counts[1] == 0 && unit == '[' {
		state.phase = emailPhaseLiteral
		state.flags = formatFlagIPv4 | formatFlagGeneral | formatFlagIPv6 | formatFlagGeneralMatchesIPv6

		return advancedStringFormatState(state)
	}

	if unit == '.' {
		if state.counts[1] == 0 || state.flags&formatFlagGeneralLastAlphaNumeric == 0 {
			return deadStringFormatState(state)
		}

		state.counts[1] = 0
		state.flags = 0

		return advancedStringFormatState(state)
	}

	if unit > 0xff || state.counts[1] == 63 ||
		!searchASCIIAlphaNumeric(byte(unit)) && unit != '-' || state.counts[1] == 0 && unit == '-' {
		return deadStringFormatState(state)
	}

	state.counts[1]++
	if searchASCIIAlphaNumeric(byte(unit)) {
		state.flags |= formatFlagGeneralLastAlphaNumeric
	} else {
		state.flags &^= formatFlagGeneralLastAlphaNumeric
	}

	return advancedStringFormatState(state)
}

func acceptEmailFormat(state stringFormatProgramState) bool {
	return state.phase == emailPhaseDomainName && state.counts[1] > 0 &&
		state.flags&formatFlagGeneralLastAlphaNumeric != 0 ||
		state.phase == emailPhaseLiteral && acceptEmailLiteral(state)
}

func advanceEmailLiteral(state stringFormatProgramState, unit uint16) stringFormatProgramState {
	if unit == ']' {
		if state.position+1 != state.length || !acceptEmailLiteral(state) {
			return deadStringFormatState(state)
		}

		state.phase = emailPhaseLiteral
		state.counts[2] = 0xffff

		return advancedStringFormatState(state)
	}

	if unit == '[' || unit == '\\' || unit > 0x7e {
		return deadStringFormatState(state)
	}

	advanceEmailLiteralIPv4(&state, unit)
	advanceEmailLiteralGeneral(&state, unit)
	advanceEmailLiteralIPv6(&state, unit)

	if state.flags&(formatFlagIPv4|formatFlagGeneral|formatFlagIPv6) == 0 {
		return deadStringFormatState(state)
	}

	state.counts[2]++

	return advancedStringFormatState(state)
}

func acceptEmailLiteral(state stringFormatProgramState) bool {
	if state.counts[2] == 0xffff {
		return true
	}

	ipv4 := state.flags&formatFlagIPv4 != 0 && state.counts[4] == 3 && state.counts[3] > 0
	general := state.flags&formatFlagGeneral != 0 && state.counts[5] == 1 && state.counts[6] > 0
	ipv6 := state.flags&formatFlagIPv6 != 0 && acceptEmailIPv6State(state)

	return ipv4 || general || ipv6
}

func advanceEmailLiteralIPv4(state *stringFormatProgramState, unit uint16) {
	if state.flags&formatFlagIPv4 == 0 {
		return
	}

	width, octets := state.counts[3], state.counts[4]
	if unit == '.' {
		if width == 0 || octets >= 3 {
			state.flags &^= formatFlagIPv4

			return
		}

		state.counts[3] = 0
		state.counts[4]++
		state.values[0] = 0

		return
	}

	if unit < '0' || unit > '9' || width == 3 {
		state.flags &^= formatFlagIPv4

		return
	}

	state.values[0] = state.values[0]*10 + unit - '0'
	if state.values[0] > 255 {
		state.flags &^= formatFlagIPv4

		return
	}

	state.counts[3]++
}

//nolint:nestif // General-tag and literal-data phases share one compact alternative state.
func advanceEmailLiteralGeneral(state *stringFormatProgramState, unit uint16) {
	if state.flags&formatFlagGeneral == 0 {
		return
	}

	if state.counts[5] == 0 {
		if unit == ':' {
			isIPv6Tag := state.counts[6] == 4 && state.flags&formatFlagGeneralMatchesIPv6 != 0
			if state.counts[6] == 0 || state.flags&formatFlagGeneralLastAlphaNumeric == 0 || isIPv6Tag {
				state.flags &^= formatFlagGeneral

				return
			}

			state.counts[5] = 1
			state.counts[6] = 0

			return
		}

		if unit > 0xff || !searchASCIIAlphaNumeric(byte(unit)) && unit != '-' {
			state.flags &^= formatFlagGeneral

			return
		}

		index := state.counts[6]
		if index >= 4 || strings.ToLower(string(rune(unit))) != string("ipv6"[index]) {
			state.flags &^= formatFlagGeneralMatchesIPv6
		}

		state.counts[6]++
		if searchASCIIAlphaNumeric(byte(unit)) {
			state.flags |= formatFlagGeneralLastAlphaNumeric
		} else {
			state.flags &^= formatFlagGeneralLastAlphaNumeric
		}

		return
	}

	if unit < '!' || unit > '~' || unit == '[' || unit == '\\' {
		state.flags &^= formatFlagGeneral

		return
	}

	state.counts[6]++
}

func advanceEmailLiteralIPv6(state *stringFormatProgramState, unit uint16) {
	if state.flags&formatFlagIPv6 == 0 {
		return
	}

	prefixIndex := state.counts[7]
	if prefixIndex < 5 {
		wanted := "ipv6:"
		if unit > 0xff || byte(strings.ToLower(string(rune(unit)))[0]) != wanted[prefixIndex] {
			state.flags &^= formatFlagIPv6

			return
		}

		state.counts[7]++

		return
	}

	if state.values[4] > 0 {
		advanceEmailIPv6Mixed(state, unit)

		return
	}

	if unit == '.' {
		if state.values[3] == 0 || state.values[3] > 3 || state.values[1] > 255 {
			state.flags &^= formatFlagIPv6

			return
		}

		state.values[4] = 1
		state.values[5] = 0
		state.values[1] = 0
		state.values[3] = 0

		return
	}

	if unit == ':' {
		if state.values[3] > 0 {
			state.values[2]++
			state.values[3] = 0
			state.values[1] = 0
			state.flags |= formatFlagIPv6PreviousColon

			return
		}

		if state.flags&formatFlagIPv6PreviousColon == 0 {
			state.flags |= formatFlagIPv6PreviousColon

			return
		}

		if state.flags&formatFlagIPv6Compressed != 0 {
			state.flags &^= formatFlagIPv6

			return
		}

		state.flags |= formatFlagIPv6Compressed
		state.flags &^= formatFlagIPv6PreviousColon

		return
	}

	if unit > 0xff || !searchHexCharacter(byte(unit)) || state.values[3] == 4 {
		state.flags &^= formatFlagIPv6

		return
	}

	state.flags &^= formatFlagIPv6PreviousColon
	state.values[3]++
	state.values[1] = state.values[1]*16 + uint16(searchHexValue(byte(unit)))
}

func searchHexValue(unit byte) int {
	switch {
	case unit >= '0' && unit <= '9':
		return int(unit - '0')
	case unit >= 'a' && unit <= 'f':
		return int(unit-'a') + 10
	default:
		return int(unit-'A') + 10
	}
}

func advanceEmailIPv6Mixed(state *stringFormatProgramState, unit uint16) {
	if unit == '.' {
		if state.values[5] == 0 || state.values[4] >= 4 {
			state.flags &^= formatFlagIPv6

			return
		}

		state.values[4]++
		state.values[5] = 0
		state.values[1] = 0

		return
	}

	if unit < '0' || unit > '9' || state.values[5] == 3 {
		state.flags &^= formatFlagIPv6

		return
	}

	state.values[1] = state.values[1]*10 + unit - '0'
	if state.values[1] > 255 {
		state.flags &^= formatFlagIPv6

		return
	}

	state.values[5]++
}

func acceptEmailIPv6State(state stringFormatProgramState) bool {
	if state.counts[7] != 5 {
		return false
	}

	groups := state.values[2]
	if state.values[4] > 0 {
		if state.values[4] != 4 || state.values[5] == 0 {
			return false
		}

		if state.flags&formatFlagIPv6Compressed != 0 {
			return groups <= 4
		}

		return groups == 6
	}

	if state.values[3] > 0 {
		groups++
	} else if state.flags&formatFlagIPv6PreviousColon != 0 {
		return false
	}

	if state.flags&formatFlagIPv6Compressed != 0 {
		return groups <= 6
	}

	return groups == 8
}
