//nolint:godoclint // Private fault helpers stay behind Build.
package schematest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// applyNonCompositionFault builds one isolated non-composition derivative.
func applyNonCompositionFault(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, error) {
	if parent == nil {
		return nil, errors.New("schematest: nil fault parent")
	}

	if s == nil || s.model == nil || s.model.root == nil {
		return nil, errors.New("schematest: fault application has no model")
	}

	if fault.obligation.rule == oracleRuleAllOf || fault.obligation.rule == oracleRuleAnyOf {
		return nil, fmt.Errorf("schematest: composition fault requires aggregate handling: %s", fault.obligation.String())
	}

	candidate, found, err := findNonCompositionDerivative(parent, fault, s)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("%w: %s", errFaultNotFound, fault.obligation.String())
	}

	return candidate, nil
}

// findNonCompositionDerivative selects a deterministic witness without retaining candidates.
//
//nolint:cyclop // Closed fault-family dispatch is intentionally explicit.
func findNonCompositionDerivative(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, bool, error) {
	if fault.obligation.rule == oracleRuleRequired {
		return findRequiredDerivative(parent, fault, s)
	}

	if fault.obligation.rule == oracleRuleAdditionalProperties {
		return findAdditionalPropertyDerivative(parent, fault, s)
	}

	node, occurrence, found := resolveExactFaultTarget(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence,
	)
	if !found {
		return nil, false, fmt.Errorf("schematest: fault target was not found: %s", fault.obligation.String())
	}

	switch fault.obligation.rule {
	case oracleRuleType:
		return findTypeDerivative(parent, fault, node, s.model, s)
	case oracleRuleEnum:
		return findEnumDerivative(parent, fault, node, s.model, s)
	case oracleRuleMinimum, oracleRuleExclusiveMinimum, oracleRuleMaximum,
		oracleRuleExclusiveMaximum, oracleRuleMultipleOf:
		return findNumberDerivative(parent, fault, s)
	case oracleRuleFormat:
		if nodeCanHaveKind(node, jsonString) {
			return findStringDerivative(parent, fault, s)
		}

		return findNumberDerivative(parent, fault, s)
	case oracleRuleMinLength, oracleRuleMaxLength, oracleRulePattern:
		return findStringDerivative(parent, fault, s)
	case oracleRuleMinItems, oracleRuleMaxItems:
		return findArrayCountDerivative(parent, fault, node, occurrence, s)
	case oracleRuleMinProperties, oracleRuleMaxProperties:
		return findObjectCountDerivative(parent, fault, node, occurrence, s)
	default:
		return nil, false, fmt.Errorf("schematest: unsupported fault rule %q", fault.obligation.rule)
	}
}

func findTypeDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	model *schemaModel,
	s *search,
) (*jsonValue, bool, error) {
	withoutType := schemaNodeWithoutLocalRule(node, oracleRuleType)
	for _, kind := range canonicalJSONKinds() {
		if !typeFaultCanUseKind(node, kind) {
			continue
		}

		candidates, err := canonicalAnyOfWitnesses(withoutType, kind)
		if err != nil {
			return nil, false, err
		}

		if derivative, found, err := firstReplacementDerivative(parent, fault, candidates, model, s); err != nil || found {
			return derivative, found, err
		}
	}

	return nil, false, nil
}

func findEnumDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	model *schemaModel,
	s *search,
) (*jsonValue, bool, error) {
	for _, kind := range enumFaultKinds(node) {
		candidates, err := canonicalEnumFaultWitnesses(node, kind)
		if err != nil {
			return nil, false, err
		}

		if derivative, found, err := firstReplacementDerivative(parent, fault, candidates, model, s); err != nil || found {
			return derivative, found, err
		}
	}

	return nil, false, nil
}

func findNumberDerivative(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, bool, error) {
	container, occurrence, found := resolveFaultContainer(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence, jsonNumber,
	)
	if !found {
		return nil, false, nil
	}

	var derivative *jsonValue

	complete, err := s.walkActiveNumberRules(
		container, occurrence, fault.pins, nil, func(candidate *jsonValue) (bool, error) {
			selected, matched, selectErr := firstReplacementDerivative(
				parent, fault, []*jsonValue{candidate}, s.model, s,
			)
			if selectErr != nil || !matched {
				return false, selectErr
			}

			derivative = selected

			return true, nil
		},
	)
	if err != nil {
		return nil, false, err
	}

	return derivative, complete, nil
}

func findStringDerivative(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, bool, error) {
	candidate, found, err := findStringFaultRow(fault, s)
	if err != nil || !found {
		return nil, false, err
	}

	return firstReplacementDerivative(parent, fault, []*jsonValue{candidate}, s.model, s)
}

func firstReplacementDerivative(
	parent *jsonValue,
	fault faultTarget,
	candidates []*jsonValue,
	model *schemaModel,
	s *search,
) (*jsonValue, bool, error) {
	paths := matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate)
	for _, path := range paths {
		for _, candidate := range candidates {
			if err := s.assign(); err != nil {
				return nil, false, err
			}

			derivative, err := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
			if err != nil {
				return nil, false, err
			}

			copyCandidate, err := copyJSONValue(candidate, make(map[*jsonValue]*jsonValue))
			if err != nil {
				return nil, false, err
			}

			if !replaceValueAtPath(derivative, path, copyCandidate) {
				continue
			}

			matched, err := derivativeHasClosure(model, derivative, fault.closure)
			if err != nil {
				return nil, false, err
			}

			if matched {
				return derivative, true, nil
			}
		}
	}

	return nil, false, nil
}

//nolint:cyclop,gocognit // Exact count conversion, resize, and item repair are one mutation.
func findArrayCountDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	occurrence schemaOccurrence,
	s *search,
) (*jsonValue, bool, error) {
	paths := matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate)
	for _, path := range paths {
		value := valueAtPath(parent, path)
		if value == nil || value.kind != jsonArray {
			continue
		}

		bound := node.minItems
		if fault.obligation.rule == oracleRuleMaxItems {
			bound = node.maxItems
		}

		count, fits, err := exactCountUint64(bound)
		if err != nil {
			return nil, false, err
		}

		if !fits || count > uint64(maxInt()) {
			return nil, false, exhaustFaultStructuralBudget(s)
		}

		desired := int(count)
		if fault.obligation.rule == oracleRuleMinItems {
			if desired == 0 {
				continue
			}

			desired--
		} else {
			if desired == maxInt() {
				return nil, false, exhaustFaultStructuralBudget(s)
			}

			desired++
		}

		witnesses := []*jsonValue{nil}
		if len(value.array) < desired {
			witnesses, err = activeItemWitnesses(s.model.root, occurrence.instanceTemplate)
			if err != nil {
				return nil, false, err
			}
		}

		for _, witness := range witnesses {
			if err := s.assign(); err != nil {
				return nil, false, err
			}

			candidate, err := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
			if err != nil {
				return nil, false, err
			}

			array := valueAtPath(candidate, path)
			if len(array.array) > desired {
				if err := s.assign(); err != nil {
					return nil, false, err
				}

				array.array = array.array[:desired]
			}

			for len(array.array) < desired {
				if err := s.assign(); err != nil {
					return nil, false, err
				}

				item, err := copyJSONValue(witness, make(map[*jsonValue]*jsonValue))
				if err != nil {
					return nil, false, err
				}

				array.array = append(array.array, item)
			}

			matched, matchErr := derivativeHasClosure(s.model, candidate, fault.closure)
			if matchErr != nil || matched {
				return candidate, matched, matchErr
			}
		}
	}

	return nil, false, nil
}

func activeItemWitnesses(root *schemaNode, instancePointer string) ([]*jsonValue, error) {
	var schemas []*schemaNode
	collectItemSchemas(root, root.occurrence, instancePointer, &schemas)

	var witnesses []*jsonValue

	for _, schema := range schemas {
		for _, kind := range canonicalJSONKinds() {
			candidates, err := canonicalAnyOfWitnesses(schema, kind)
			if err != nil {
				return nil, err
			}

			for _, candidate := range candidates {
				witnesses, err = appendUniqueJSONWitness(witnesses, candidate)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if len(witnesses) == 0 {
		return []*jsonValue{{kind: jsonNull}}, nil
	}

	return witnesses, nil
}

func collectItemSchemas(
	node *schemaNode,
	occurrence schemaOccurrence,
	instancePointer string,
	result *[]*schemaNode,
) {
	if instanceTemplateMatches(occurrence.instanceTemplate, instancePointer) && node.items != nil {
		*result = append(*result, node.items)
	}

	for _, child := range faultSchemaChildren(node, occurrence) {
		collectItemSchemas(child.node, child.occurrence, instancePointer, result)
	}
}

//nolint:cyclop // Exact count conversion, resize, and member repair are one mutation.
func findObjectCountDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	occurrence schemaOccurrence,
	s *search,
) (*jsonValue, bool, error) {
	paths := matchingValuePaths(parent, fault.obligation.occurrence.instanceTemplate)
	if fault.obligation.rule == oracleRuleMinProperties {
		return findObjectShrinkDerivative(parent, fault, node, paths, s)
	}

	for _, path := range paths {
		if err := s.assign(); err != nil {
			return nil, false, err
		}

		candidate, err := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
		if err != nil {
			return nil, false, err
		}

		object := valueAtPath(candidate, path)
		if object == nil || object.kind != jsonObject {
			continue
		}

		bound := node.maxProperties

		count, fits, err := exactCountUint64(bound)
		if err != nil {
			return nil, false, err
		}

		if !fits || count > uint64(maxInt()) {
			return nil, false, exhaustFaultStructuralBudget(s)
		}

		desired := int(count)
		if desired == maxInt() {
			return nil, false, exhaustFaultStructuralBudget(s)
		}

		desired++
		if err := addValidObjectMembers(object, node, occurrence, fault.pins, desired, s); err != nil {
			return nil, false, err
		}

		if len(object.object) != desired {
			continue
		}

		matched, matchErr := derivativeHasClosure(s.model, candidate, fault.closure)
		if matchErr != nil || matched {
			return candidate, matched, matchErr
		}
	}

	return nil, false, nil
}

//nolint:cyclop // Subset search, charging, and closure checking form one bounded repair.
func findObjectShrinkDerivative(
	parent *jsonValue,
	fault faultTarget,
	node *schemaNode,
	paths [][]string,
	s *search,
) (*jsonValue, bool, error) {
	count, fits, countErr := exactCountUint64(node.minProperties)
	if countErr != nil {
		return nil, false, countErr
	}

	if !fits || count > uint64(maxInt()) {
		return nil, false, exhaustFaultStructuralBudget(s)
	}

	if count == 0 {
		return nil, false, nil
	}

	desired := int(count) - 1

	for _, path := range paths {
		object := valueAtPath(parent, path)
		if object == nil || object.kind != jsonObject || len(object.object) < desired {
			continue
		}

		names := sortedObjectNames(object.object)
		removeCount := len(names) - desired

		subsetFound, subsetErr := visitNameSubsets(names, removeCount, func(remove []string) (bool, error) {
			if assignErr := s.assign(); assignErr != nil {
				return false, assignErr
			}

			candidate, copyErr := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
			if copyErr != nil {
				return false, copyErr
			}

			candidateObject := valueAtPath(candidate, path)

			for _, name := range remove {
				if assignErr := s.assign(); assignErr != nil {
					return false, assignErr
				}

				delete(candidateObject.object, name)
			}

			matched, matchErr := derivativeHasClosure(s.model, candidate, fault.closure)
			if matchErr != nil || !matched {
				return false, matchErr
			}

			object = candidate

			return true, nil
		})
		if subsetErr != nil {
			return nil, false, subsetErr
		}

		if subsetFound {
			return object, true, nil
		}
	}

	return nil, false, nil
}

func visitNameSubsets(names []string, size int, visit func([]string) (bool, error)) (bool, error) {
	selected := make([]string, 0, size)

	var walk func(int) (bool, error)

	walk = func(start int) (bool, error) {
		if len(selected) == size {
			return visit(selected)
		}

		for index := start; index <= len(names)-(size-len(selected)); index++ {
			selected = append(selected, names[index])
			found, err := walk(index + 1)
			selected = selected[:len(selected)-1]

			if err != nil || found {
				return found, err
			}
		}

		return false, nil
	}

	return walk(0)
}

//nolint:cyclop // Declared and additional member alternatives share one deterministic fill.
func addValidObjectMembers(
	object *jsonValue,
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	desired int,
	s *search,
) error {
	for _, name := range sortedSchemaPropertyNames(node.properties) {
		if len(object.object) == desired {
			return nil
		}

		if _, exists := object.object[name]; exists {
			continue
		}

		property := node.properties[name]
		propertyOccurrence := rebasePlanOccurrence(
			property,
			occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)

		value, found, err := firstValidNodeValue(property, propertyOccurrence, pins, s)
		if err != nil {
			return err
		}

		if !found {
			continue
		}

		if err := s.assign(); err != nil {
			return err
		}

		object.object[name] = value
	}

	for len(object.object) < desired &&
		(node.allowAdditionalProperties || node.additionalProperties != nil) {
		name := nextAdditionalPropertyName(node, object.object)
		value := &jsonValue{kind: jsonNull}

		if node.additionalProperties != nil {
			additionalOccurrence := rebasePlanOccurrence(
				node.additionalProperties,
				occurrence.usePointer+"/additionalProperties",
				appendInstanceToken(occurrence.instanceTemplate, name),
			)

			var err error

			var found bool

			value, found, err = firstValidNodeValue(node.additionalProperties, additionalOccurrence, pins, s)
			if err != nil {
				return err
			}

			if !found {
				break
			}
		}

		if err := s.assign(); err != nil {
			return err
		}

		object.object[name] = value
	}

	return nil
}

func nextAdditionalPropertyName(node *schemaNode, members map[string]*jsonValue) string {
	base := additionalPropertyWitnessName(node)
	if _, exists := members[base]; !exists {
		return base
	}

	for suffix := 1; ; suffix++ {
		name := fmt.Sprintf("%s_%d", base, suffix)
		if _, exists := members[name]; !exists {
			return name
		}
	}
}

func firstValidNodeValue(
	node *schemaNode,
	occurrence schemaOccurrence,
	pins []applicabilityPin,
	s *search,
) (*jsonValue, bool, error) {
	var found *jsonValue

	complete, err := s.walkNode(node, occurrence, pins, rowSearchContext{}, func(value *jsonValue) (bool, error) {
		result := evaluateNode(node, value, occurrence)
		if result.err != nil || !result.valid {
			return false, result.err
		}

		found = value

		return true, nil
	})
	if err != nil {
		return nil, false, err
	}

	if !complete {
		return nil, false, nil
	}

	return found, true, nil
}

func findRequiredDerivative(parent *jsonValue, fault faultTarget, s *search) (*jsonValue, bool, error) {
	tokens, ok := rowPointerTokens(fault.obligation.occurrence.instanceTemplate)
	if !ok || len(tokens) == 0 {
		return nil, false, errors.New("schematest: required fault has no member path")
	}

	name := tokens[len(tokens)-1]

	parentPointer := pointerFromTokens(tokens[:len(tokens)-1])
	for _, path := range matchingValuePaths(parent, parentPointer) {
		if err := s.assign(); err != nil {
			return nil, false, err
		}

		candidate, err := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
		if err != nil {
			return nil, false, err
		}

		object := valueAtPath(candidate, path)
		if object == nil || object.kind != jsonObject {
			continue
		}

		delete(object.object, name)

		matched, err := derivativeHasClosure(s.model, candidate, fault.closure)
		if err != nil || matched {
			return candidate, matched, err
		}
	}

	return nil, false, nil
}

//nolint:cyclop // Path and active-witness retries form one mutation search.
func findAdditionalPropertyDerivative(
	parent *jsonValue,
	fault faultTarget,
	s *search,
) (*jsonValue, bool, error) {
	tokens, ok := rowPointerTokens(fault.obligation.occurrence.instanceTemplate)
	if !ok || len(tokens) == 0 {
		return nil, false, errors.New("schematest: additional-property fault has no member path")
	}

	parentPointer := pointerFromTokens(tokens[:len(tokens)-1])

	node, found := resolveFaultNodeAtInstance(
		s.model.root, s.model.root.occurrence, fault.obligation.occurrence.usePointer, parentPointer,
	)
	if !found {
		return nil, false, errors.New("schematest: additional-property parent schema was not found")
	}

	witnesses, err := additionalPropertyWitnesses(s.model.root, parentPointer)
	if err != nil {
		return nil, false, err
	}

	for _, path := range matchingValuePaths(parent, parentPointer) {
		for _, witness := range witnesses {
			if err := s.assign(); err != nil {
				return nil, false, err
			}

			candidate, err := copyJSONValue(parent, make(map[*jsonValue]*jsonValue))
			if err != nil {
				return nil, false, err
			}

			object := valueAtPath(candidate, path)
			if object == nil || object.kind != jsonObject {
				continue
			}

			value, err := copyJSONValue(witness, make(map[*jsonValue]*jsonValue))
			if err != nil {
				return nil, false, err
			}

			object.object[additionalPropertyWitnessName(node)] = value

			matched, err := derivativeHasClosure(s.model, candidate, fault.closure)
			if err != nil || matched {
				return candidate, matched, err
			}
		}
	}

	return nil, false, nil
}

func additionalPropertyWitnesses(root *schemaNode, instancePointer string) ([]*jsonValue, error) {
	var schemas []*schemaNode
	collectAdditionalPropertySchemas(root, root.occurrence, instancePointer, &schemas)

	var witnesses []*jsonValue

	for _, schema := range schemas {
		for _, kind := range canonicalJSONKinds() {
			candidates, err := canonicalAnyOfWitnesses(schema, kind)
			if err != nil {
				return nil, err
			}

			for _, candidate := range candidates {
				witnesses, err = appendUniqueJSONWitness(witnesses, candidate)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if len(witnesses) == 0 {
		for _, kind := range canonicalJSONKinds() {
			candidates, err := canonicalKindWitnesses(kind)
			if err != nil {
				return nil, err
			}

			witnesses = append(witnesses, candidates...)
		}
	}

	return witnesses, nil
}

func collectAdditionalPropertySchemas(
	node *schemaNode,
	occurrence schemaOccurrence,
	instancePointer string,
	result *[]*schemaNode,
) {
	if instanceTemplateMatches(occurrence.instanceTemplate, instancePointer) && node.additionalProperties != nil {
		*result = append(*result, node.additionalProperties)
	}

	for _, child := range faultSchemaChildren(node, occurrence) {
		collectAdditionalPropertySchemas(child.node, child.occurrence, instancePointer, result)
	}
}

func derivativeHasClosure(model *schemaModel, derivative *jsonValue, closure []failureIdentity) (bool, error) {
	result := evaluate(model, derivative)
	if result.err != nil {
		return false, fmt.Errorf("evaluate fault derivative: %w", result.err)
	}

	if result.valid {
		return false, nil
	}

	return exactFailureClosure(result.failures, closure)
}

// resolveExactFaultTarget follows canonical schema children to one authored rule occurrence.
func resolveExactFaultTarget(
	node *schemaNode,
	occurrence schemaOccurrence,
	target schemaOccurrence,
) (*schemaNode, schemaOccurrence, bool) {
	if ruleOccurrenceMatches(occurrence, target) {
		return node, occurrence, true
	}

	for _, child := range faultSchemaChildren(node, occurrence) {
		if target.usePointer != child.occurrence.usePointer &&
			!strings.HasPrefix(target.usePointer, child.occurrence.usePointer+"/") {
			continue
		}

		if foundNode, foundOccurrence, found := resolveExactFaultTarget(child.node, child.occurrence, target); found {
			return foundNode, foundOccurrence, true
		}
	}

	return nil, schemaOccurrence{}, false
}

func resolveFaultNodeAtInstance(
	node *schemaNode,
	occurrence schemaOccurrence,
	usePointer,
	instanceTemplate string,
) (*schemaNode, bool) {
	if occurrence.usePointer == usePointer &&
		instanceTemplateMatches(occurrence.instanceTemplate, instanceTemplate) {
		return node, true
	}

	for _, child := range faultSchemaChildren(node, occurrence) {
		if foundNode, found := resolveFaultNodeAtInstance(
			child.node, child.occurrence, usePointer, instanceTemplate,
		); found {
			return foundNode, true
		}
	}

	return nil, false
}

func resolveFaultContainer(
	node *schemaNode,
	occurrence schemaOccurrence,
	target schemaOccurrence,
	kind jsonKind,
) (*schemaNode, schemaOccurrence, bool) {
	if instanceTemplateMatches(occurrence.instanceTemplate, target.instanceTemplate) &&
		nodeAcceptsKindForTarget(node, kind) &&
		(target.usePointer == occurrence.usePointer || strings.HasPrefix(target.usePointer, occurrence.usePointer+"/")) {
		return node, occurrence, true
	}

	for _, child := range faultSchemaChildren(node, occurrence) {
		if foundNode, foundOccurrence, found := resolveFaultContainer(child.node, child.occurrence, target, kind); found {
			return foundNode, foundOccurrence, true
		}
	}

	return nil, schemaOccurrence{}, false
}

type faultSchemaChild struct {
	node       *schemaNode
	occurrence schemaOccurrence
}

func faultSchemaChildren(node *schemaNode, occurrence schemaOccurrence) []faultSchemaChild {
	children := make([]faultSchemaChild, 0)
	if node.items != nil {
		children = append(children, faultSchemaChild{node.items, rebasePlanOccurrence(
			node.items, occurrence.usePointer+"/items", appendInstanceToken(occurrence.instanceTemplate, "*"),
		)})
	}

	for _, name := range sortedSchemaPropertyNames(node.properties) {
		property := node.properties[name]
		children = append(children, faultSchemaChild{property, rebasePlanOccurrence(
			property, occurrence.usePointer+"/properties/"+escapePointerToken(name),
			appendInstanceToken(occurrence.instanceTemplate, name),
		)})
	}

	if node.additionalProperties != nil {
		children = append(children, faultSchemaChild{node.additionalProperties, rebasePlanOccurrence(
			node.additionalProperties, occurrence.usePointer+"/additionalProperties",
			appendInstanceToken(occurrence.instanceTemplate, "*"),
		)})
	}

	for index, child := range node.allOf {
		children = append(children, faultSchemaChild{child, rebasePlanOccurrence(
			child, occurrence.usePointer+"/allOf/"+itoa(index), occurrence.instanceTemplate,
		)})
	}

	for index, child := range node.anyOf {
		children = append(children, faultSchemaChild{child, rebasePlanOccurrence(
			child, occurrence.usePointer+"/anyOf/"+itoa(index), occurrence.instanceTemplate,
		)})
	}

	return children
}

func matchingValuePaths(root *jsonValue, template string) [][]string {
	tokens, ok := rowPointerTokens(template)
	if !ok {
		return nil
	}

	var result [][]string
	collectMatchingValuePaths(root, tokens, nil, &result)

	return result
}

//nolint:cyclop // Pointer completion and two container kinds form one traversal.
func collectMatchingValuePaths(value *jsonValue, tokens, path []string, result *[][]string) {
	if len(tokens) == 0 {
		*result = append(*result, append([]string(nil), path...))

		return
	}

	if value == nil {
		return
	}

	token := tokens[0]

	switch value.kind {
	case jsonArray:
		if token == "*" {
			for index, child := range value.array {
				collectMatchingValuePaths(child, tokens[1:], append(path, strconv.Itoa(index)), result)
			}

			return
		}

		index, err := strconv.Atoi(token)
		if err == nil && index >= 0 && index < len(value.array) {
			collectMatchingValuePaths(value.array[index], tokens[1:], append(path, token), result)
		}
	case jsonObject:
		if token == "*" {
			names := sortedObjectNames(value.object)
			for _, name := range names {
				collectMatchingValuePaths(value.object[name], tokens[1:], append(path, name), result)
			}

			return
		}

		if child, exists := value.object[token]; exists {
			collectMatchingValuePaths(child, tokens[1:], append(path, token), result)
		}
	}
}

func valueAtPath(root *jsonValue, path []string) *jsonValue {
	value := root
	for _, token := range path {
		if value == nil {
			return nil
		}

		switch value.kind {
		case jsonArray:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(value.array) {
				return nil
			}

			value = value.array[index]
		case jsonObject:
			value = value.object[token]
		default:
			return nil
		}
	}

	return value
}

func replaceValueAtPath(root *jsonValue, path []string, replacement *jsonValue) bool {
	if len(path) == 0 {
		*root = *replacement

		return true
	}

	parent := valueAtPath(root, path[:len(path)-1])
	if parent == nil {
		return false
	}

	last := path[len(path)-1]

	switch parent.kind {
	case jsonArray:
		index, err := strconv.Atoi(last)
		if err != nil || index < 0 || index >= len(parent.array) {
			return false
		}

		parent.array[index] = replacement
	case jsonObject:
		if _, exists := parent.object[last]; !exists {
			return false
		}

		parent.object[last] = replacement
	default:
		return false
	}

	return true
}

func pointerFromTokens(tokens []string) string {
	if len(tokens) == 0 {
		return "#"
	}

	encoded := make([]string, len(tokens))
	for index, token := range tokens {
		encoded[index] = escapePointerToken(token)
	}

	return "#/" + strings.Join(encoded, "/")
}

func exhaustFaultStructuralBudget(s *search) error {
	for {
		if err := s.assign(); err != nil {
			return err
		}
	}
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
