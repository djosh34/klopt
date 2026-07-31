//nolint:godoclint // Private builder results and dispatch are exercised by semantic tests.
package testgenerator

import (
	"errors"
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

const booleanValueCount = 2

type buildState uint8

const (
	buildMiss buildState = iota
	buildComplete
)

type buildResult struct {
	value jsonvalue.Value
	state buildState
	err   error
}

func completeBuild(value jsonvalue.Value) buildResult {
	return buildResult{value: value, state: buildComplete}
}

func missedBuild() buildResult {
	return buildResult{state: buildMiss}
}

func failedBuild(err error) buildResult {
	return buildResult{state: buildMiss, err: err}
}

//nolint:cyclop // Kind selection is a closed dispatch over JSON's six kinds.
func buildValue(selected []demand, tape *tapeCursor) buildResult {
	if tape == nil {
		return failedBuild(errors.New("build value with nil tape cursor"))
	}

	kinds, err := buildKinds(selected)
	if err != nil {
		return failedBuild(err)
	}

	if len(kinds) == 0 {
		return missedBuild()
	}

	selectedKind := kinds[0]
	if len(kinds) > 1 {
		choice, chooseErr := tape.choose(len(kinds))
		if chooseErr != nil {
			return failedBuild(chooseErr)
		}

		selectedKind = kinds[choice]
	}

	switch selectedKind {
	case kindNull:
		return buildNull(selected)
	case kindBoolean:
		return buildBoolean(selected, tape)
	case kindNumber:
		return buildNumber(selected, tape)
	case kindString:
		return buildString(selected, tape)
	case kindArray:
		return buildArray(selected, tape)
	case kindObject:
		return buildObject(selected, tape)
	default:
		return failedBuild(fmt.Errorf("unknown selected JSON kind %d", selectedKind))
	}
}

func buildNull(selected []demand) buildResult {
	return buildSelectedValue(selected, jsonvalue.Null())
}

func buildBoolean(selected []demand, tape *tapeCursor) buildResult {
	if tape == nil {
		return failedBuild(errors.New("build boolean with nil tape cursor"))
	}

	candidates := make([]jsonvalue.Value, 0, booleanValueCount)

	for _, value := range []jsonvalue.Value{jsonvalue.Bool(false), jsonvalue.Bool(true)} {
		holds, err := demandsHold(selected, value)
		if err != nil {
			return failedBuild(fmt.Errorf("check boolean candidate: %w", err))
		}

		if holds {
			candidates = append(candidates, value)
		}
	}

	if len(candidates) == 0 {
		return missedBuild()
	}

	if len(candidates) == 1 {
		return completeBuild(candidates[0])
	}

	return completeBuild(candidates[tape.takeWord()%uint64(len(candidates))])
}

//nolint:cyclop,gocognit // Kind restrictions are one explicit closed table.
func buildKinds(selected []demand) ([]jsonKind, error) {
	allowed := [jsonKindCount]bool{
		kindNull: true, kindBoolean: true, kindNumber: true,
		kindString: true, kindArray: true, kindObject: true,
	}

	enumValues, hasEnum, err := selectedEnumValues(selected)
	if err != nil {
		return nil, err
	}

	if hasEnum {
		allowed = restrictKindsToValues(allowed, enumValues)
		if !anyAllowedKind(allowed) {
			return nil, nil
		}
	}

	negativeEnumValues, err := selectedNegativeEnumValues(selected)
	if err != nil {
		return nil, err
	}

	removeExhaustedEnumKinds(&allowed, negativeEnumValues)

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil {
			return nil, fmt.Errorf("demand %d has nil expression", index)
		}

		if selectedDemand.expression.kind != expressionAtom {
			return nil, fmt.Errorf("demand %d is not an atom expression", index)
		}

		rule := selectedDemand.expression.atom
		if err := validateBuilderAtom(rule); err != nil {
			return nil, fmt.Errorf("demand %d: %w", index, err)
		}

		switch rule.kind {
		case atomKinds:
			if selectedDemand.wantPass {
				for kind := jsonKind(0); kind < jsonKindCount; kind++ {
					allowed[kind] = allowed[kind] && rule.allowed[kind]
				}
			} else if rule.integer {
				clearNonKind(&allowed, kindNumber)
			} else {
				for kind := jsonKind(0); kind < jsonKindCount; kind++ {
					allowed[kind] = allowed[kind] && !rule.allowed[kind]
				}
			}
		case atomEnum:
			// Positive and negative enum membership is checked on the complete value.
		case atomNumberMinimum, atomNumberMaximum, atomNumberMultipleOf, atomNumberFormat:
			if !selectedDemand.wantPass {
				clearNonKind(&allowed, kindNumber)
			}
		case atomStringMinLength, atomStringMaxLength, atomStringLanguage:
			if !selectedDemand.wantPass {
				clearNonKind(&allowed, kindString)
			}
		case atomArrayMinItems, atomArrayMaxItems, atomArrayItems:
			if !selectedDemand.wantPass {
				clearNonKind(&allowed, kindArray)
			}
		case atomObjectMinProperties, atomObjectMaxProperties, atomObjectRequired,
			atomObjectProperty, atomObjectAdditional:
			if !selectedDemand.wantPass {
				clearNonKind(&allowed, kindObject)
			}
		default:
			return nil, fmt.Errorf("unknown atom kind %d", rule.kind)
		}
	}

	kinds := make([]jsonKind, 0, jsonKindCount)
	for kind := jsonKind(0); kind < jsonKindCount; kind++ {
		if allowed[kind] {
			kinds = append(kinds, kind)
		}
	}

	return kinds, nil
}

func validateBuilderAtom(rule atom) error {
	if rule.kind == atomKinds {
		if !anyAllowedKind(rule.allowed) {
			return errors.New("kind atom has no allowed kinds")
		}

		if rule.integer && !rule.allowed[kindNumber] {
			return errors.New("integer kind atom does not allow numbers")
		}
	}

	switch rule.kind {
	case atomArrayItems, atomObjectProperty:
		if rule.child == nil {
			return errors.New("builder atom has nil child")
		}
	case atomObjectAdditional:
		if rule.hasChild && rule.child == nil {
			return errors.New("additional properties atom has nil child")
		}
	}

	return nil
}

func clearNonKind(allowed *[jsonKindCount]bool, retained jsonKind) {
	for kind := jsonKind(0); kind < jsonKindCount; kind++ {
		allowed[kind] = kind == retained && allowed[kind]
	}
}

func anyAllowedKind(allowed [jsonKindCount]bool) bool {
	for _, present := range allowed {
		if present {
			return true
		}
	}

	return false
}

func restrictKindsToValues(
	allowed [jsonKindCount]bool,
	values []jsonvalue.Value,
) [jsonKindCount]bool {
	var restricted [jsonKindCount]bool

	for _, value := range values {
		kind := valueKind(value)
		if kind < jsonKindCount {
			restricted[kind] = true
		}
	}

	for kind := jsonKind(0); kind < jsonKindCount; kind++ {
		allowed[kind] = allowed[kind] && restricted[kind]
	}

	return allowed
}

func buildSelectedValue(selected []demand, value jsonvalue.Value) buildResult {
	holds, err := demandsHold(selected, value)
	if err != nil {
		return failedBuild(fmt.Errorf("check selected demands: %w", err))
	}

	if !holds {
		return missedBuild()
	}

	return completeBuild(value)
}

//nolint:cyclop // Enum intersection follows the fixed positive-then-negative order.
func selectedEnumValues(selected []demand) ([]jsonvalue.Value, bool, error) {
	positive := make([][]jsonvalue.Value, 0)
	negative := make([][]jsonvalue.Value, 0)

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil {
			return nil, false, fmt.Errorf("demand %d has nil expression", index)
		}

		if selectedDemand.expression.kind != expressionAtom {
			return nil, false, fmt.Errorf("demand %d is not an atom expression", index)
		}

		rule := selectedDemand.expression.atom
		if rule.kind != atomEnum {
			continue
		}

		if len(rule.values) == 0 {
			return nil, false, errors.New("enum atom has no values")
		}

		if selectedDemand.wantPass {
			positive = append(positive, rule.values)
		} else {
			negative = append(negative, rule.values)
		}
	}

	if len(positive) == 0 {
		return nil, false, nil
	}

	intersection, err := cloneValues(positive[0])
	if err != nil {
		return nil, false, fmt.Errorf("clone enum values: %w", err)
	}

	for _, candidates := range positive[1:] {
		intersection = intersectEnumValues(intersection, candidates)
	}

	for _, candidates := range negative {
		intersection = removeEnumValues(intersection, candidates)
	}

	return intersection, true, nil
}

func intersectEnumValues(
	current []jsonvalue.Value,
	candidates []jsonvalue.Value,
) []jsonvalue.Value {
	filtered := make([]jsonvalue.Value, 0, len(current))
	for _, value := range current {
		if containsJSONValue(candidates, value) {
			filtered = append(filtered, value)
		}
	}

	return filtered
}

func removeEnumValues(
	current []jsonvalue.Value,
	removed []jsonvalue.Value,
) []jsonvalue.Value {
	filtered := make([]jsonvalue.Value, 0, len(current))
	for _, value := range current {
		if !containsJSONValue(removed, value) {
			filtered = append(filtered, value)
		}
	}

	return filtered
}

func containsJSONValue(values []jsonvalue.Value, target jsonvalue.Value) bool {
	for _, value := range values {
		if target.Equal(value) {
			return true
		}
	}

	return false
}

func selectedNegativeEnumValues(selected []demand) ([]jsonvalue.Value, error) {
	values := make([]jsonvalue.Value, 0)

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil {
			return nil, fmt.Errorf("demand %d has nil expression", index)
		}

		if selectedDemand.expression.kind != expressionAtom {
			return nil, fmt.Errorf("demand %d is not an atom expression", index)
		}

		rule := selectedDemand.expression.atom
		if rule.kind != atomEnum || selectedDemand.wantPass {
			continue
		}

		if len(rule.values) == 0 {
			return nil, errors.New("enum atom has no values")
		}

		values = append(values, rule.values...)
	}

	return values, nil
}

func removeExhaustedEnumKinds(allowed *[jsonKindCount]bool, removed []jsonvalue.Value) {
	if containsJSONValue(removed, jsonvalue.Null()) {
		allowed[kindNull] = false
	}

	if containsJSONValue(removed, jsonvalue.Bool(false)) && containsJSONValue(removed, jsonvalue.Bool(true)) {
		allowed[kindBoolean] = false
	}
}

func cloneValues(values []jsonvalue.Value) ([]jsonvalue.Value, error) {
	cloned := make([]jsonvalue.Value, 0, len(values))
	for index, value := range values {
		clonedValue, err := cloneJSONValue(value)
		if err != nil {
			return nil, fmt.Errorf("enum value %d: %w", index, err)
		}

		cloned = append(cloned, clonedValue)
	}

	return cloned, nil
}

//nolint:cyclop // Demand selection is one closed expression dispatch.
func selectPassingDemands(
	expression *expression,
	tape *tapeCursor,
	selected *[]demand,
) error {
	if expression == nil {
		return errors.New("select demands from nil expression")
	}

	if tape == nil {
		return errors.New("select demands with nil tape cursor")
	}

	if selected == nil {
		return errors.New("select demands with nil output")
	}

	switch expression.kind {
	case expressionAtom:
		*selected = append(*selected, newDemand(expression, true))

		return nil
	case expressionAll:
		for index, child := range expression.children {
			if err := selectPassingDemands(child, tape, selected); err != nil {
				return fmt.Errorf("select all child %d: %w", index, err)
			}
		}

		return nil
	case expressionAny:
		if len(expression.children) == 0 {
			return errors.New("select empty any expression")
		}

		choice := 0

		if len(expression.children) > 1 {
			var err error

			choice, err = tape.choose(len(expression.children))
			if err != nil {
				return err
			}
		}

		return selectPassingDemands(expression.children[choice], tape, selected)
	default:
		return fmt.Errorf("unknown expression kind %d", expression.kind)
	}
}

func buildValid(root *expression, tape *tapeCursor) buildResult {
	if root == nil {
		return failedBuild(errors.New("build valid value from nil root"))
	}

	if tape == nil {
		return failedBuild(errors.New("build valid value with nil tape cursor"))
	}

	selected := make([]demand, 0)
	if err := selectPassingDemands(root, tape, &selected); err != nil {
		return failedBuild(fmt.Errorf("select passing demands: %w", err))
	}

	built := buildValue(selected, tape)
	if built.err != nil || built.state != buildComplete {
		return built
	}

	holds, err := demandsHold(selected, built.value)
	if err != nil {
		return failedBuild(fmt.Errorf("evaluate selected demands: %w", err))
	}

	if !holds {
		return missedBuild()
	}

	return built
}
