//nolint:godoclint,mnd // Private SMTP grammar vocabulary mirrors RFC 5321 constants.
package stringlanguage

import "strings"

const maximumEmailIntermediateStates = maximumProductStates

func emailLanguage() (Language, error) {
	machine, err := formatDFA(emailPattern())
	if err != nil {
		return Language{}, err
	}

	machine = minimizeDFA(machine)

	limited, err := limitEmailPartLengths(machine)
	if err != nil {
		return Language{}, &CompileError{Operation: "compile format", Err: err}
	}

	return Language{dfa: *minimizeDFA(limited)}, nil
}

func emailPattern() string {
	atext := `[A-Za-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]`
	dotString := atext + `+(\.` + atext + `+)*`
	quoted := `"([\x20-\x21\x23-\x5B\x5D-\x7E]|\\[\x20-\x7E])*"`
	local := `(` + dotString + `|` + quoted + `)`

	label := `[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?`
	domain := label + `(\.` + label + `)*`

	snum := `([0-9]|[0-9]{2}|[01][0-9]{2}|2[0-4][0-9]|25[0-5])`
	ipv4 := snum + `\.` + snum + `\.` + snum + `\.` + snum
	ipv6 := ipv6AddressPattern(ipv4)
	general := generalAddressLiteralPattern()
	literal := `\[(` + ipv4 + `|[Ii][Pp][Vv]6:` + ipv6 + `|` + general + `)\]`

	return `^` + local + `@(` + domain + `|` + literal + `)$`
}

func ipv6AddressPattern(ipv4 string) string {
	hex := `[0-9A-Fa-f]{1,4}`
	full := repeatedColonGroups(hex, 8)
	compressed := compressedIPv6Patterns(hex, 6, "")
	v4Full := repeatedColonGroups(hex, 6) + `:` + ipv4
	v4Compressed := compressedIPv6Patterns(hex, 4, ipv4)

	return `(` + strings.Join(append([]string{full, v4Full}, append(compressed, v4Compressed...)...), `|`) + `)`
}

func compressedIPv6Patterns(hex string, maximumGroups int, suffix string) []string {
	patterns := make([]string, 0)

	for left := 0; left <= maximumGroups; left++ {
		for right := 0; right <= maximumGroups-left; right++ {
			pattern := repeatedColonGroups(hex, left) + `::`
			if right > 0 {
				pattern += repeatedColonGroups(hex, right)
				if suffix != "" {
					pattern += `:`
				}
			}

			pattern += suffix
			patterns = append(patterns, pattern)
		}
	}

	return patterns
}

func repeatedColonGroups(group string, count int) string {
	if count == 0 {
		return ""
	}

	return group + strings.Repeat(`:`+group, count-1)
}

func generalAddressLiteralPattern() string {
	tagAlphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-"
	tagLastAlphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	tagCharacter := `[A-Za-z0-9-]`
	tagLast := `[A-Za-z0-9]`
	shortOrLong := `(` + tagCharacter + `{0,2}` + tagLast + `|` + tagCharacter + `{4,}` + tagLast + `)`
	fourNotIPv6 := `(` + charactersExcept(tagAlphabet, 'I', 'i') +
		tagCharacter + `{2}` + tagLast +
		`|[Ii]` + charactersExcept(tagAlphabet, 'P', 'p') +
		tagCharacter + tagLast +
		`|[Ii][Pp]` + charactersExcept(tagAlphabet, 'V', 'v') + tagLast +
		`|[Ii][Pp][Vv]` + charactersExcept(tagLastAlphabet, '6') + `)`

	return `(` + shortOrLong + `|` + fourNotIPv6 + `):[!-Z^-~]+`
}

func charactersExcept(alphabet string, excluded ...byte) string {
	characters := make([]string, 0, len(alphabet)-len(excluded))
	for index := range len(alphabet) {
		if !strings.ContainsRune(string(excluded), rune(alphabet[index])) {
			characters = append(characters, string(alphabet[index]))
		}
	}

	return `(` + strings.Join(characters, `|`) + `)`
}

type emailPart uint8

const (
	emailLocal emailPart = iota
	emailQuotedLocal
	emailQuotedEscape
	emailClosedQuote
	emailDomainStart
	emailDomain
)

type emailLengthState struct {
	machine uint32
	part    emailPart
	length  uint16
	total   uint16
	over    bool
}

func limitEmailPartLengths(machine *dfa) (*dfa, error) {
	initial := emailLengthState{}
	states := []emailLengthState{initial}
	ids := map[emailLengthState]uint32{initial: 0}
	result := &dfa{utf16: machine.utf16}

	asciiClasses := make(runeSet, 0, 128)
	for value := rune(0); value < 128; value++ {
		asciiClasses = append(asciiClasses, runeRange{first: value, last: value})
	}

	for current := 0; current < len(states); current++ {
		tracked := states[current]

		state := dfaState{accepting: machine.states[tracked.machine].accepting && !tracked.over}

		alphabet := partitionRuneSets(scalarUniverse, machine.scalarRanges(tracked.machine), asciiClasses)
		for _, characterClass := range alphabet {
			next := tracked
			if characterClass.first < 128 {
				next = advanceEmailLength(tracked, byte(characterClass.first))
			} else {
				next.over = true
			}

			next.machine = machine.advance(tracked.machine, characterClass.first)
			if deadDFAState(machine, next.machine) {
				next = emailLengthState{machine: next.machine, over: true}
			}

			nextID, ok := ids[next]
			if !ok {
				if len(states) >= maximumEmailIntermediateStates {
					return nil, limitError(
						"DFA construction", "DFA states", maximumEmailIntermediateStates, uint64(len(states)+1),
					)
				}

				nextID = uint32(len(states))
				ids[next] = nextID
				states = append(states, next)
			}

			appendDFAEdge(&state.edges, dfaEdge{
				first: characterClass.first, last: characterClass.last, target: nextID,
			})
		}

		result.states = append(result.states, state)
	}

	return result, nil
}

func deadDFAState(machine *dfa, state uint32) bool {
	if machine.states[state].accepting {
		return false
	}

	for _, edge := range machine.states[state].edges {
		if edge.target != state {
			return false
		}
	}

	return true
}

//nolint:cyclop // The states directly mirror quoted and dot-string SMTP local parts.
func advanceEmailLength(state emailLengthState, value byte) emailLengthState {
	if state.over {
		return state
	}

	state.incrementTotal(254)

	if state.over {
		return state
	}

	switch state.part {
	case emailLocal:
		if value == '@' {
			state.part = emailDomainStart
			state.length = 0

			return state
		}

		if state.length == 0 && value == '"' {
			state.part = emailQuotedLocal
		}

		state.incrementLocal(64)
	case emailQuotedLocal:
		state.incrementLocal(64)

		switch value {
		case '\\':
			state.part = emailQuotedEscape
		case '"':
			state.part = emailClosedQuote
		}
	case emailQuotedEscape:
		state.incrementLocal(64)
		state.part = emailQuotedLocal
	case emailClosedQuote:
		if value == '@' {
			state.part = emailDomainStart
			state.length = 0
		} else {
			state.incrementLocal(64)
		}
	case emailDomainStart:
		state.part = emailDomain
	case emailDomain:
	}

	return state
}

func (state *emailLengthState) incrementLocal(limit uint16) {
	if state.length == limit {
		state.over = true

		return
	}

	state.length++
}

func (state *emailLengthState) incrementTotal(limit uint16) {
	if state.total == limit {
		state.over = true

		return
	}

	state.total++
}
