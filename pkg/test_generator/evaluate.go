//nolint:godoclint // Private evaluator vocabulary is tested within the package.
package testgenerator

import (
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Evaluation uses the retained exact language.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:cyclop,gocyclo,gocognit,maintidx // Atom semantics are one explicit closed dispatch.
func atomHolds(rule atom, value jsonvalue.Value) (bool, error) {
	if err := validateJSONValue(value); err != nil {
		return false, fmt.Errorf("atom %s received invalid JSON value: %w", rule.schemaPointer, err)
	}

	switch rule.kind {
	case atomKinds:
		return kindAtomHolds(rule, value)
	case atomEnum:
		if len(rule.values) == 0 {
			return false, errors.New("enum atom has no values")
		}

		for index, allowed := range rule.values {
			if err := validateJSONValue(allowed); err != nil {
				return false, fmt.Errorf("enum atom member %d: %w", index, err)
			}

			if value.Equal(allowed) {
				return true, nil
			}
		}

		return false, nil
	case atomNumberMinimum:
		number, err := exactRuleNumber(rule.number, "minimum")
		if err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindNumber {
			return true, nil
		}

		comparison := value.Number.Compare(number)

		return comparison > 0 || comparison == 0 && !rule.exclusive, nil
	case atomNumberMaximum:
		number, err := exactRuleNumber(rule.number, "maximum")
		if err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindNumber {
			return true, nil
		}

		comparison := value.Number.Compare(number)

		return comparison < 0 || comparison == 0 && !rule.exclusive, nil
	case atomNumberMultipleOf:
		multiple, err := exactRuleNumber(rule.number, "multipleOf")
		if err != nil {
			return false, err
		}

		zero, err := jsonvalue.ParseNumber("0")
		if err != nil {
			return false, fmt.Errorf("parse zero: %w", err)
		}

		if multiple.Compare(zero) <= 0 {
			return false, errors.New("multipleOf atom is not positive")
		}

		if value.Kind != jsonvalue.KindNumber {
			return true, nil
		}

		return value.Number.IsMultipleOf(multiple), nil
	case atomNumberFormat:
		if value.Kind != jsonvalue.KindNumber {
			if err := validateNumberFormatName(rule.text); err != nil {
				return false, err
			}

			return true, nil
		}

		return numberMatchesFormat(value.Number, rule.text)
	case atomStringMinLength:
		count, err := exactCount(rule.count, "minLength")
		if err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindString {
			return true, nil
		}

		return compareLength(utf8.RuneCountInString(value.String), count) >= 0, nil
	case atomStringMaxLength:
		count, err := exactCount(rule.count, "maxLength")
		if err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindString {
			return true, nil
		}

		return compareLength(utf8.RuneCountInString(value.String), count) <= 0, nil
	case atomStringLanguage:
		if _, err := stringlanguage.Begin([]stringlanguage.Requirement{{
			Language: rule.language, WantMatch: true,
		}}); err != nil {
			return false, fmt.Errorf("invalid string language: %w", err)
		}

		if value.Kind != jsonvalue.KindString {
			return true, nil
		}

		return rule.language.Matches(value.String), nil
	case atomArrayMinItems:
		count, err := exactCount(rule.count, "minItems")
		if err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindArray {
			return true, nil
		}

		return compareLength(len(value.Array), count) >= 0, nil
	case atomArrayMaxItems:
		count, err := exactCount(rule.count, "maxItems")
		if err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindArray {
			return true, nil
		}

		return compareLength(len(value.Array), count) <= 0, nil
	case atomArrayItems:
		if rule.child == nil {
			return false, errors.New("array items atom has nil child")
		}

		if value.Kind != jsonvalue.KindArray {
			return true, nil
		}

		for index, item := range value.Array {
			holds, err := expressionHolds(rule.child, item)
			if err != nil {
				return false, fmt.Errorf("array item %d: %w", index, err)
			}

			if !holds {
				return false, nil
			}
		}

		return true, nil
	case atomObjectMinProperties:
		count, err := exactCount(rule.count, "minProperties")
		if err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindObject {
			return true, nil
		}

		return compareLength(len(value.Object), count) >= 0, nil
	case atomObjectMaxProperties:
		count, err := exactCount(rule.count, "maxProperties")
		if err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindObject {
			return true, nil
		}

		return compareLength(len(value.Object), count) <= 0, nil
	case atomObjectRequired:
		if len(rule.names) == 0 {
			return false, errors.New("required atom has no names")
		}

		if value.Kind != jsonvalue.KindObject {
			return true, nil
		}

		if err := validateUniqueNames(rule.names); err != nil {
			return false, err
		}

		for _, name := range rule.names {
			if !hasMember(value.Object, name) {
				return false, nil
			}
		}

		return true, nil
	case atomObjectProperty:
		if rule.child == nil {
			return false, errors.New("object property atom has nil child")
		}

		if value.Kind != jsonvalue.KindObject {
			return true, nil
		}

		member, found := memberByName(value.Object, rule.name)
		if !found {
			return true, nil
		}

		return expressionHolds(rule.child, member)
	case atomObjectAdditional:
		if rule.hasChild && rule.child == nil {
			return false, errors.New("additional properties atom has nil child")
		}

		if err := validateUniqueNames(rule.names); err != nil {
			return false, err
		}

		if value.Kind != jsonvalue.KindObject {
			return true, nil
		}

		for index, member := range value.Object {
			if containsName(rule.names, member.Name) {
				continue
			}

			if rule.hasChild {
				holds, err := expressionHolds(rule.child, member.Value)
				if err != nil {
					return false, fmt.Errorf("additional property %d %q: %w", index, member.Name, err)
				}

				if !holds {
					return false, nil
				}

				continue
			}

			if !rule.allowedAdditional {
				return false, nil
			}
		}

		return true, nil
	default:
		return false, fmt.Errorf("unknown atom kind %d at %s", rule.kind, rule.schemaPointer)
	}
}

//nolint:cyclop // All and any semantics are one explicit closed dispatch.
func expressionHolds(expression *expression, value jsonvalue.Value) (bool, error) {
	if expression == nil {
		return false, errors.New("evaluate nil expression")
	}

	if err := validateJSONValue(value); err != nil {
		return false, fmt.Errorf("evaluate invalid JSON value: %w", err)
	}

	switch expression.kind {
	case expressionAtom:
		return atomHolds(expression.atom, value)
	case expressionAll:
		for index, child := range expression.children {
			holds, err := expressionHolds(child, value)
			if err != nil {
				return false, fmt.Errorf("all child %d: %w", index, err)
			}

			if !holds {
				return false, nil
			}
		}

		return true, nil
	case expressionAny:
		for index, child := range expression.children {
			holds, err := expressionHolds(child, value)
			if err != nil {
				return false, fmt.Errorf("any child %d: %w", index, err)
			}

			if holds {
				return true, nil
			}
		}

		return false, nil
	default:
		return false, fmt.Errorf("unknown expression kind %d", expression.kind)
	}
}

func demandsHold(selected []demand, value jsonvalue.Value) (bool, error) {
	for index, demand := range selected {
		if demand.expression == nil {
			return false, fmt.Errorf("demand %d has nil expression", index)
		}

		holds, err := expressionHolds(demand.expression, value)
		if err != nil {
			return false, fmt.Errorf("demand %d: %w", index, err)
		}

		if holds != demand.wantPass {
			return false, nil
		}
	}

	return true, nil
}

func kindAtomHolds(rule atom, value jsonvalue.Value) (bool, error) {
	hasAllowed := false

	for _, allowed := range rule.allowed {
		if allowed {
			hasAllowed = true

			break
		}
	}

	if !hasAllowed {
		return false, errors.New("kind atom has no allowed kinds")
	}

	if rule.integer && !rule.allowed[kindNumber] {
		return false, errors.New("integer kind atom does not allow numbers")
	}

	if !rule.allowed[valueKind(value)] {
		return false, nil
	}

	if rule.integer && value.Kind == jsonvalue.KindNumber && !value.Number.IsInteger() {
		return false, nil
	}

	return true, nil
}

func validateJSONValue(value jsonvalue.Value) error {
	_, err := value.MarshalJSON()

	return err
}

func exactRuleNumber(number jsonvalue.Number, keyword string) (jsonvalue.Number, error) {
	if number.Lexeme == "" {
		return jsonvalue.Number{}, fmt.Errorf("%s atom has an empty number", keyword)
	}

	parsed, err := jsonvalue.ParseNumber(number.Lexeme)
	if err != nil {
		return jsonvalue.Number{}, fmt.Errorf("%s atom number: %w", keyword, err)
	}

	return parsed, nil
}

func exactCount(number jsonvalue.Number, keyword string) (jsonvalue.Number, error) {
	count, err := exactRuleNumber(number, keyword)
	if err != nil {
		return jsonvalue.Number{}, err
	}

	zero, err := jsonvalue.ParseNumber("0")
	if err != nil {
		return jsonvalue.Number{}, fmt.Errorf("parse zero: %w", err)
	}

	if !count.IsInteger() || count.Compare(zero) < 0 {
		return jsonvalue.Number{}, fmt.Errorf("%s atom count is not a non-negative integer", keyword)
	}

	return count, nil
}

func compareLength(length int, bound jsonvalue.Number) int {
	actual, err := jsonvalue.ParseNumber(strconv.Itoa(length))
	if err != nil {
		return 0
	}

	return actual.Compare(bound)
}

func validateNumberFormatName(format string) error {
	switch format {
	case "int32", "int64", "float", "double":
		return nil
	default:
		return fmt.Errorf("unsupported numeric format %q", format)
	}
}

func numberMatchesFormat(number jsonvalue.Number, format string) (bool, error) {
	if err := validateNumberFormatName(format); err != nil {
		return false, err
	}

	switch format {
	case "int32":
		return signedIntegerInRange(number, "-2147483648", "2147483647"), nil
	case "int64":
		return signedIntegerInRange(number, "-9223372036854775808", "9223372036854775807"), nil
	case "float":
		_, err := strconv.ParseFloat(number.Lexeme, 32)

		return err == nil, nil
	case "double":
		_, err := strconv.ParseFloat(number.Lexeme, 64)

		return err == nil, nil
	default:
		return false, errors.New("unreachable numeric format")
	}
}

func signedIntegerInRange(number jsonvalue.Number, minimum string, maximum string) bool {
	minimumNumber, minimumErr := jsonvalue.ParseNumber(minimum)

	maximumNumber, maximumErr := jsonvalue.ParseNumber(maximum)
	if minimumErr != nil || maximumErr != nil {
		return false
	}

	return number.IsInteger() && number.Compare(minimumNumber) >= 0 && number.Compare(maximumNumber) <= 0
}

func valueKind(value jsonvalue.Value) jsonKind {
	switch value.Kind {
	case jsonvalue.KindNull:
		return kindNull
	case jsonvalue.KindBoolean:
		return kindBoolean
	case jsonvalue.KindNumber:
		return kindNumber
	case jsonvalue.KindString:
		return kindString
	case jsonvalue.KindArray:
		return kindArray
	case jsonvalue.KindObject:
		return kindObject
	default:
		return jsonKindCount
	}
}

func hasMember(members []jsonvalue.Member, name string) bool {
	_, found := memberByName(members, name)

	return found
}

func memberByName(members []jsonvalue.Member, name string) (jsonvalue.Value, bool) {
	for _, member := range members {
		if member.Name == name {
			return member.Value, true
		}
	}

	return jsonvalue.Value{}, false
}

func validateUniqueNames(names []string) error {
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate object name %q", name)
		}

		seen[name] = struct{}{}
	}

	return nil
}

func containsName(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}

	return false
}
