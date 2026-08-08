package schematest

import (
	"errors"
	"fmt"
	"math/big"
	"unicode/utf16"
)

// typeFaultCanUseKind reports whether one JSON kind can fail an explicit type.
func typeFaultCanUseKind(node *schemaNode, kind jsonKind) bool {
	if node.kind == schemaInteger && kind == jsonNumber {
		return true
	}

	return !nodeAcceptsKindForTarget(node, kind)
}

// additionalPropertyWitnessName chooses a stable member absent from authored properties.
func additionalPropertyWitnessName(node *schemaNode) string {
	const base = "__schematest_extra__"

	if _, exists := node.properties[base]; !exists {
		return base
	}

	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s_%d", base, index)
		if _, exists := node.properties[candidate]; !exists {
			return candidate
		}
	}
}

// schemaNodeWithoutLocalRule disables one local rule without changing children.
//
//nolint:cyclop // Every supported local rule has one explicit clone operation.
func schemaNodeWithoutLocalRule(node *schemaNode, rule string) *schemaNode {
	shape := *node.schemaShape

	switch rule {
	case oracleRuleType:
		shape.kind = schemaAny
		shape.nullable = false
	case oracleRuleEnum:
		shape.enum = nil
	case oracleRuleMinimum, oracleRuleExclusiveMinimum:
		shape.minimum = nil
		shape.exclusiveMinimum = false
	case oracleRuleMaximum, oracleRuleExclusiveMaximum:
		shape.maximum = nil
		shape.exclusiveMaximum = false
	case oracleRuleMultipleOf:
		shape.multipleOf = nil
	case oracleRuleFormat:
		shape.format = schemaFormatNone
	case oracleRuleMinLength:
		shape.minLength = nil
	case oracleRuleMaxLength:
		shape.maxLength = nil
	case oracleRulePattern:
		shape.pattern = nil
	case oracleRuleMinItems:
		shape.minItems = nil
	case oracleRuleMaxItems:
		shape.maxItems = nil
	case oracleRuleMinProperties:
		shape.minProperties = nil
	case oracleRuleMaxProperties:
		shape.maxProperties = nil
	}

	return &schemaNode{schemaShape: &shape, occurrence: node.occurrence}
}

// canonicalEnumFaultWitnesses lazily yields generated values outside the authored enum.
func canonicalEnumFaultWitnesses(node *schemaNode, kind jsonKind) jsonValueSource {
	return uniqueJSONValueSource(func(yield func(*jsonValue) bool) error {
		stopped := false
		emit := func(value *jsonValue) bool {
			if !yield(value) {
				stopped = true
			}

			return !stopped
		}

		if err := walkGeneratedCompositionWitnesses(node, kind, make(map[*schemaNode]bool), emit); err != nil {
			return err
		}

		if stopped {
			return nil
		}

		if err := walkCanonicalKindWitnesses(kind, emit); err != nil {
			return err
		}

		if kind != jsonString {
			return nil
		}

		const base = "__schematest_enum_fault__"
		for index := 0; ; index++ {
			candidateText := base
			if index > 0 {
				candidateText = fmt.Sprintf("%s_%d", base, index)
			}

			candidate := &jsonValue{kind: jsonString, text: candidateText}

			contains, err := enumContainsValue(node, candidate)
			if err != nil {
				return err
			}

			if !contains {
				emit(candidate)

				return nil
			}
		}
	})
}

// enumFaultKinds returns kinds that can fail enum without also failing type.
func enumFaultKinds(node *schemaNode) []jsonKind {
	if node.kind == schemaAny {
		return canonicalJSONKinds()
	}

	kind := schemaNodeJSONKind(node.kind)

	result := []jsonKind{kind}
	if node.nullable {
		result = append(result, jsonNull)
	}

	return result
}

// schemaNodeWithoutEnum shallow-copies one node while disabling only its enum.
func schemaNodeWithoutEnum(node *schemaNode) *schemaNode {
	shape := *node.schemaShape
	shape.enum = nil

	return &schemaNode{schemaShape: &shape, occurrence: node.occurrence}
}

// enumContainsValue reports whether one value is semantically listed by an enum.
func enumContainsValue(node *schemaNode, value *jsonValue) (bool, error) {
	for _, member := range node.enum {
		equal, err := jsonValidatedSemanticEqual(member.value, value)
		if err != nil {
			return false, err
		}

		if equal {
			return true, nil
		}
	}

	return false, nil
}

// jsonValueSource advances one candidate only when its consumer requests it.
type jsonValueSource func(yield func(*jsonValue) bool) error

// canonicalAnyOfWitnesses lazily yields authored values, derived values, then fixed canonical values.
func canonicalAnyOfWitnesses(node *schemaNode, kind jsonKind) jsonValueSource {
	return uniqueJSONValueSource(func(yield func(*jsonValue) bool) error {
		stopped := false
		emit := func(value *jsonValue) bool {
			if !yield(value) {
				stopped = true
			}

			return !stopped
		}

		if err := walkAuthoredEnumWitnesses(node, kind, make(map[*schemaNode]bool), emit); err != nil {
			return err
		}

		if stopped {
			return nil
		}

		if err := walkGeneratedCompositionWitnesses(node, kind, make(map[*schemaNode]bool), emit); err != nil {
			return err
		}

		if stopped {
			return nil
		}

		return walkCanonicalKindWitnesses(kind, emit)
	})
}

// walkAuthoredEnumWitnesses traverses authored enum members lazily.
func walkAuthoredEnumWitnesses(
	node *schemaNode,
	kind jsonKind,
	visiting map[*schemaNode]bool,
	yield func(*jsonValue) bool,
) error {
	_, err := walkCompositionWitnessNodes(node, visiting, func(current *schemaNode) bool {
		for _, member := range current.enum {
			if member.value.kind == kind && !yield(member.value) {
				return false
			}
		}

		return true
	})

	return err
}

// walkGeneratedCompositionWitnesses traverses authored derived-value sources lazily.
//
//nolint:cyclop // Each authored scalar source is yielded in its locked local order.
func walkGeneratedCompositionWitnesses(
	node *schemaNode,
	kind jsonKind,
	visiting map[*schemaNode]bool,
	yield func(*jsonValue) bool,
) error {
	_, err := walkCompositionWitnessNodes(node, visiting, func(current *schemaNode) bool {
		if current.defaultValue != nil && current.defaultValue.kind == kind && !yield(current.defaultValue) {
			return false
		}

		if kind == jsonString {
			if witness, exists := canonicalStringPatternWitness(current.pattern); exists && !yield(witness) {
				return false
			}
		}

		if kind == jsonNumber {
			for _, bound := range []*exactNumber{current.minimum, current.maximum, current.multipleOf} {
				if bound != nil && !yield(&jsonValue{kind: jsonNumber, number: bound}) {
					return false
				}
			}
		}

		return true
	})

	return err
}

// walkCompositionWitnessNodes traverses composition nodes until the consumer stops.
//
//nolint:cyclop // Node validation, stopping, and both composition families form one DFS seam.
func walkCompositionWitnessNodes(
	node *schemaNode,
	visiting map[*schemaNode]bool,
	visit func(*schemaNode) bool,
) (bool, error) {
	if node == nil || node.schemaShape == nil {
		return false, errors.New("composition witness source has no shape")
	}

	if visiting[node] {
		return false, fmt.Errorf("recursive composition witness schema at %s", node.occurrence.usePointer)
	}

	visiting[node] = true
	defer delete(visiting, node)

	if !visit(node) {
		return false, nil
	}

	for _, child := range node.allOf {
		complete, err := walkCompositionWitnessNodes(child, visiting, visit)
		if err != nil || !complete {
			return complete, err
		}
	}

	for _, child := range node.anyOf {
		complete, err := walkCompositionWitnessNodes(child, visiting, visit)
		if err != nil || !complete {
			return complete, err
		}
	}

	return true, nil
}

// uniqueJSONValueSource deduplicates the traversed prefix without retaining candidate values.
func uniqueJSONValueSource(source jsonValueSource) jsonValueSource {
	return func(yield func(*jsonValue) bool) error {
		seen := make(map[string]bool)

		var sourceErr error

		err := source(func(candidate *jsonValue) bool {
			encoded, marshalErr := marshalStrict(candidate)
			if marshalErr != nil {
				sourceErr = marshalErr

				return false
			}

			key := string(encoded)
			if seen[key] {
				return true
			}

			seen[key] = true

			return yield(candidate)
		})
		if err != nil {
			return err
		}

		return sourceErr
	}
}

// walkCanonicalKindWitnesses emits the locked kind alternatives directly.
//
//nolint:cyclop // Locked small alternatives are emitted directly, never retained as a candidate slice.
func walkCanonicalKindWitnesses(kind jsonKind, yield func(*jsonValue) bool) error {
	emitNumber := func(source string) (bool, error) {
		number, err := parseExactNumber(source)
		if err != nil {
			return false, err
		}

		return yield(&jsonValue{kind: jsonNumber, number: number}), nil
	}

	switch kind {
	case jsonNull:
		yield(&jsonValue{kind: jsonNull})
	case jsonBoolean:
		if yield(&jsonValue{kind: jsonBoolean}) {
			yield(&jsonValue{kind: jsonBoolean, boolean: true})
		}
	case jsonNumber:
		for _, source := range [...]string{"-1", "0", "0.5", "1", "2", "3"} {
			complete, err := emitNumber(source)
			if err != nil || !complete {
				return err
			}
		}
	case jsonString:
		for _, text := range [...]string{"", "a", "b", "text"} {
			if !yield(&jsonValue{kind: jsonString, text: text}) {
				return nil
			}
		}
	case jsonArray:
		number, err := parseExactNumber("0")
		if err != nil {
			return err
		}

		if !yield(&jsonValue{kind: jsonArray, array: []*jsonValue{}}) ||
			!yield(&jsonValue{kind: jsonArray, array: []*jsonValue{{kind: jsonBoolean}}}) ||
			!yield(&jsonValue{kind: jsonArray, array: []*jsonValue{{kind: jsonString, text: "a"}}}) {
			return nil
		}

		yield(&jsonValue{kind: jsonArray, array: []*jsonValue{{kind: jsonNumber, number: number}}})
	case jsonObject:
		if yield(&jsonValue{kind: jsonObject, object: map[string]*jsonValue{}}) {
			yield(&jsonValue{kind: jsonObject, object: map[string]*jsonValue{
				"a": {kind: jsonString, text: "a"},
			}})
		}
	default:
		return fmt.Errorf("unknown JSON kind %d", kind)
	}

	return nil
}

// canonicalStringPatternWitness derives a witness for one literal-only pattern.
//
//nolint:cyclop // Literal-only AST validation is one bounded witness pass.
func canonicalStringPatternWitness(pattern *patternAST) (*jsonValue, bool) {
	if pattern == nil || pattern.expression == nil || len(pattern.expression.alternatives) != 1 {
		return nil, false
	}

	sequence := pattern.expression.alternatives[0]
	units := make([]uint16, 0, len(sequence.terms))

	for _, term := range sequence.terms {
		if term == nil || term.atom == nil || term.quantified || term.minimum != 1 || term.maximum != 1 {
			return nil, false
		}

		switch term.atom.kind {
		case patternStart, patternEnd:
		case patternLiteral:
			units = append(units, term.atom.literal)
		default:
			return nil, false
		}
	}

	return &jsonValue{kind: jsonString, text: string(utf16.Decode(units))}, true
}

// exactCountUint64 converts a non-negative integral count without parsing a display lexeme.
//
//nolint:cyclop // Exact representation validation and bounded conversion are one phase.
func exactCountUint64(count *exactCount) (uint64, bool, error) {
	if count == nil || count.number == nil {
		return 0, false, nil
	}

	integer, err := count.number.isInteger()
	if err != nil {
		return 0, false, err
	}

	if !integer || count.number.numerator.Sign() < 0 {
		return 0, false, nil
	}

	if count.number.numerator.Sign() == 0 {
		return 0, true, nil
	}

	if count.number.exponent.Sign() > 0 {
		if !count.number.exponent.IsUint64() || count.number.exponent.Uint64() > 20 {
			return 0, false, nil
		}
	}

	coefficient, exponent, err := count.number.finiteDecimal()
	if err != nil {
		return 0, false, err
	}

	if coefficient.Sign() < 0 || exponent.Sign() < 0 || !exponent.IsUint64() || exponent.Uint64() > 20 {
		return 0, false, nil
	}

	value := new(big.Int).Set(coefficient)
	if exponent.Sign() > 0 {
		value.Mul(value, integerPower(decimalRadix, exponent.Uint64()))
	}

	if !value.IsUint64() {
		return 0, false, nil
	}

	return value.Uint64(), true, nil
}

// appendUniqueJSONWitness retains semantic distinctness without changing authored values.
func appendUniqueJSONWitness(witnesses []*jsonValue, candidate *jsonValue) ([]*jsonValue, error) {
	for _, existing := range witnesses {
		equal, err := jsonSemanticEqual(existing, candidate)
		if err != nil {
			return nil, err
		}

		if equal {
			return witnesses, nil
		}
	}

	return append(witnesses, candidate), nil
}

// canonicalKindWitnesses returns small deterministic values for each JSON kind.
//
//nolint:cyclop // Each JSON kind has an explicit canonical witness family.
func canonicalKindWitnesses(kind jsonKind) ([]*jsonValue, error) {
	switch kind {
	case jsonNull:
		return []*jsonValue{{kind: jsonNull}}, nil
	case jsonBoolean:
		return []*jsonValue{{kind: jsonBoolean}, {kind: jsonBoolean, boolean: true}}, nil
	case jsonNumber:
		sources := []string{"-1", "0", "0.5", "1", "2", "3"}
		values := make([]*jsonValue, 0, len(sources))

		for _, source := range sources {
			number, err := parseExactNumber(source)
			if err != nil {
				return nil, err
			}

			values = append(values, &jsonValue{kind: jsonNumber, number: number})
		}

		return values, nil
	case jsonString:
		return []*jsonValue{
			{kind: jsonString, text: ""},
			{kind: jsonString, text: "a"},
			{kind: jsonString, text: "b"},
			{kind: jsonString, text: "text"},
		}, nil
	case jsonArray:
		number, err := parseExactNumber("0")
		if err != nil {
			return nil, err
		}

		return []*jsonValue{
			{kind: jsonArray, array: []*jsonValue{}},
			{kind: jsonArray, array: []*jsonValue{{kind: jsonBoolean}}},
			{kind: jsonArray, array: []*jsonValue{{kind: jsonString, text: "a"}}},
			{kind: jsonArray, array: []*jsonValue{{kind: jsonNumber, number: number}}},
		}, nil
	case jsonObject:
		return []*jsonValue{
			{kind: jsonObject, object: map[string]*jsonValue{}},
			{kind: jsonObject, object: map[string]*jsonValue{
				"a": {kind: jsonString, text: "a"},
			}},
		}, nil
	default:
		return nil, fmt.Errorf("unknown JSON kind %d", kind)
	}
}
