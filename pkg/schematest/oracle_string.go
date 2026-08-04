//nolint:cyclop,godoclint // String-rule dispatch keeps each independent verdict in order.
package schematest

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

const oracleStringValidLevel = "valid"

// evaluateStringRules applies string constraints only to JSON string values.
func evaluateStringRules(
	result *evaluation,
	node *schemaNode,
	occurrence schemaOccurrence,
	value *jsonValue,
) {
	if value.kind != jsonString {
		return
	}

	if node.minLength != nil {
		if err := evaluateStringLengthRule(
			result, occurrence, node.minLength, utf8.RuneCountInString(value.text), oracleRuleMinLength, true,
		); err != nil {
			result.err = err

			return
		}
	}

	if node.maxLength != nil {
		if err := evaluateStringLengthRule(
			result, occurrence, node.maxLength, utf8.RuneCountInString(value.text), oracleRuleMaxLength, false,
		); err != nil {
			result.err = err

			return
		}
	}

	if node.pattern != nil {
		identity := makeRuleIdentity(occurrence, oracleRulePattern)
		appendApplicable(result, identity)

		matches, err := cleanPatternMatches(node.pattern, value.text)
		if err != nil {
			result.err = fmt.Errorf("%s: %w", identity, err)

			return
		}

		if matches {
			appendStringObservation(result, identity)
		} else {
			appendFailure(result, identity)
		}
	}

	if isStringSchemaFormat(node.format) {
		identity := makeRuleIdentity(occurrence, oracleRuleFormat)
		appendApplicable(result, identity)

		matches, err := cleanStringFormatMatches(value.text, node.format)
		if err != nil {
			result.err = fmt.Errorf("%s: %w", identity, err)

			return
		}

		if matches {
			appendStringObservation(result, identity)
		} else {
			appendFailure(result, identity)
		}
	}
}

// evaluateStringLengthRule evaluates one rune-counted string bound.
func evaluateStringLengthRule(
	result *evaluation,
	occurrence schemaOccurrence,
	bound *exactCount,
	length int,
	rule string,
	minimum bool,
) error {
	identity := makeRuleIdentity(occurrence, rule)
	appendApplicable(result, identity)

	actual, err := parseExactNumber(strconv.Itoa(length))
	if err != nil {
		return fmt.Errorf("%s: parse string length: %w", identity, err)
	}

	comparison, err := actual.compare(bound.number)
	if err != nil {
		return fmt.Errorf("%s: compare string length: %w", identity, err)
	}

	violated := comparison < 0
	if !minimum {
		violated = comparison > 0
	}

	if violated {
		appendFailure(result, identity)
	} else {
		appendStringObservation(result, identity)
	}

	return nil
}

// appendStringObservation records one successful string rule at its stable level.
func appendStringObservation(result *evaluation, identity ruleIdentity) {
	appendObserved(result, levelIdentity{
		ruleIdentity: identity,
		level:        oracleStringValidLevel,
	})
}

// isStringSchemaFormat reports whether format has active string semantics.
func isStringSchemaFormat(format schemaFormat) bool {
	switch format {
	case schemaFormatByte, schemaFormatDate, schemaFormatDateTime, schemaFormatEmail,
		schemaFormatIPv4, schemaFormatUUID, schemaFormatUUIDv4, schemaFormatUUIDDashV4,
		schemaFormatCIDR, schemaFormatIPv4CIDR:
		return true
	default:
		return false
	}
}
