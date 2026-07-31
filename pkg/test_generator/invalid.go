//nolint:godoclint // Focused invalid construction is private generator machinery.
package testgenerator

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type faultBuildContext struct {
	token    *faultToken
	planStep int
	route    bool
}

func buildFocusedInvalid(
	root *expression,
	plan *faultPlan,
	tape *tapeCursor,
) buildResult {
	if plan == nil {
		return failedBuild(errors.New("build invalid value from nil fault plan"))
	}

	token := newFaultToken(plan)

	built := buildValueWithFault(root, token, 0, tape)
	if built.err != nil || built.state != buildComplete {
		return built
	}

	if token.state != faultInjected {
		return missedBuild()
	}

	return built
}

//nolint:cyclop // Fault state has one fixed pending, routed, and injected path.
func buildValueWithFault(
	currentExpression *expression,
	token *faultToken,
	planStep int,
	tape *tapeCursor,
) buildResult {
	if currentExpression == nil {
		return failedBuild(errors.New("build invalid value from nil expression"))
	}

	if token == nil || token.plan == nil {
		return failedBuild(errors.New("build invalid value with nil fault token"))
	}

	if tape == nil {
		return failedBuild(errors.New("build invalid value with nil tape cursor"))
	}

	if token.state == faultInjected {
		return buildPositiveExpression(currentExpression, tape)
	}

	selected := make([]demand, 0)

	faultHere, route, err := selectFaultDemands(
		currentExpression, token, planStep, tape, &selected,
	)
	if err != nil {
		return failedBuild(err)
	}

	built := buildFaultValue(selected, tape, faultBuildContext{
		token: token, planStep: planStep, route: route,
	})
	if built.err != nil || built.state != buildComplete {
		return built
	}

	if faultHere {
		if err := token.markInjected(); err != nil {
			return failedBuild(err)
		}
	}

	return built
}

func buildFaultValue(
	selected []demand,
	tape *tapeCursor,
	context faultBuildContext,
) buildResult {
	if tape == nil {
		return failedBuild(errors.New("build fault value with nil tape cursor"))
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

	_, hasPositiveEnum, err := selectedEnumValues(selected)
	if err != nil {
		return failedBuild(err)
	}

	negativeEnums, err := selectedNegativeEnumValues(selected)
	if err != nil {
		return failedBuild(err)
	}

	if len(negativeEnums) > 0 && !hasPositiveEnum {
		return buildNegativeEnumValue(selected, selectedKind, negativeEnums, tape, context)
	}

	return buildFaultKind(selected, selectedKind, tape, context)
}

func buildFaultKind(
	selected []demand,
	selectedKind jsonKind,
	tape *tapeCursor,
	context faultBuildContext,
) buildResult {
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
		return buildFaultArray(selected, tape, context)
	case kindObject:
		return buildFaultObject(selected, tape, context)
	default:
		return failedBuild(fmt.Errorf("unknown selected JSON kind %d", selectedKind))
	}
}

//nolint:cyclop // Enum witnesses use one fixed fallback per JSON kind.
func buildNegativeEnumValue(
	selected []demand,
	selectedKind jsonKind,
	negative []jsonvalue.Value,
	tape *tapeCursor,
	context faultBuildContext,
) buildResult {
	withoutEnums := make([]demand, 0, len(selected))
	for _, selectedDemand := range selected {
		if selectedDemand.expression.kind == expressionAtom &&
			selectedDemand.expression.atom.kind == atomEnum {
			continue
		}

		withoutEnums = append(withoutEnums, selectedDemand)
	}

	base := buildFaultKind(withoutEnums, selectedKind, tape, context)
	if base.err != nil {
		return base
	}

	if base.state == buildComplete && !containsJSONValue(negative, base.value) {
		return buildSelectedValue(selected, base.value)
	}

	fallbacks, err := enumFallbacks(selectedKind)
	if err != nil {
		return failedBuild(err)
	}

	for _, candidate := range fallbacks {
		if containsJSONValue(negative, candidate) {
			continue
		}

		holds, holdErr := demandsHold(withoutEnums, candidate)
		if holdErr != nil {
			return failedBuild(holdErr)
		}

		if holds {
			return buildSelectedValue(selected, candidate)
		}
	}

	return missedBuild()
}

//nolint:cyclop // Enum fallback selection is one fixed fallback per JSON kind.
func enumFallbacks(kind jsonKind) ([]jsonvalue.Value, error) {
	switch kind {
	case kindNull:
		return []jsonvalue.Value{jsonvalue.Null()}, nil
	case kindBoolean:
		return []jsonvalue.Value{jsonvalue.Bool(false), jsonvalue.Bool(true)}, nil
	case kindNumber:
		zero, err := jsonvalue.ParseNumber("0")
		if err != nil {
			return nil, err
		}

		one, err := jsonvalue.ParseNumber("1")
		if err != nil {
			return nil, err
		}

		return []jsonvalue.Value{{Kind: jsonvalue.KindNumber, Number: zero}, {Kind: jsonvalue.KindNumber, Number: one}}, nil
	case kindString:
		return []jsonvalue.Value{jsonvalue.String(""), jsonvalue.String("a")}, nil
	case kindArray:
		return []jsonvalue.Value{jsonvalue.Array(nil), jsonvalue.Array([]jsonvalue.Value{jsonvalue.Null()})}, nil
	case kindObject:
		empty, err := jsonvalue.Object(nil)
		if err != nil {
			return nil, err
		}

		withAdditional, err := jsonvalue.Object([]jsonvalue.Member{{
			Name: "additional0", Value: jsonvalue.Null(),
		}})
		if err != nil {
			return nil, err
		}

		return []jsonvalue.Value{empty, withAdditional}, nil
	default:
		return nil, fmt.Errorf("unknown enum fallback kind %d", kind)
	}
}

//nolint:cyclop,gocognit,gocyclo,nestif // Array fault construction has one required-first traversal.
func buildFaultArray(
	selected []demand,
	tape *tapeCursor,
	context faultBuildContext,
) buildResult {
	enumValues, hasEnum, err := selectedEnumValues(selected)
	if err != nil {
		return failedBuild(err)
	}

	if hasEnum {
		arrays := make([]jsonvalue.Value, 0, len(enumValues))
		for _, value := range enumValues {
			if value.Kind == jsonvalue.KindArray {
				arrays = append(arrays, value)
			}
		}

		if len(arrays) == 0 {
			return missedBuild()
		}

		candidate := arrays[0]
		if len(arrays) > 1 {
			candidate = arrays[tape.takeWord()%uint64(len(arrays))]
		}

		return buildSelectedValue(selected, candidate)
	}

	domain, err := collectFaultArrayDomain(selected)
	if err != nil {
		return failedBuild(err)
	}

	if domain.maximum != nil &&
		(domain.maximum.Sign() < 0 || domain.minimum.Cmp(domain.maximum) > 0) {
		return missedBuild()
	}

	faultIndex := -1

	var faultChild *expression

	if context.route {
		if context.token == nil || context.token.plan == nil ||
			context.planStep >= len(context.token.plan.schemaPath) {
			return failedBuild(errors.New("array fault route has no path step"))
		}

		step := context.token.plan.schemaPath[context.planStep]
		if step.kind != faultArrayItem {
			return failedBuild(errors.New("array fault route does not select an item"))
		}

		faultIndex = selectArrayFaultIndex(tape.takeWord(), domain.maximum)

		faultChild = domain.routedChild
		if faultChild == nil {
			return failedBuild(errors.New("array fault route has no item child"))
		}
	} else if context.token != nil && context.token.plan != nil {
		for _, negative := range domain.negativeItems {
			if negative.expression == context.token.plan.target {
				faultIndex = selectArrayFaultIndex(tape.takeWord(), domain.maximum)

				break
			}
		}
	}

	required := new(big.Int).Set(domain.minimum)
	if len(domain.negativeItems) > 0 && required.Sign() == 0 {
		required.SetInt64(1)
	}

	if faultIndex >= 0 {
		selectedCount := new(big.Int).SetUint64(uint64(faultIndex + 1))
		if selectedCount.Cmp(required) > 0 {
			required = selectedCount
		}
	}

	if domain.maximum != nil && required.Cmp(domain.maximum) > 0 {
		return missedBuild()
	}

	values := make([]jsonvalue.Value, 0)

	for count := big.NewInt(0); count.Cmp(required) < 0; count.Add(count, big.NewInt(1)) {
		index := count.Uint64()

		var built buildResult

		switch {
		case faultIndex >= 0 && index == uint64(faultIndex) && context.route:
			built = buildValueWithFault(
				faultChild, context.token, context.planStep+1, tape,
			)
		case faultIndex >= 0 && index == uint64(faultIndex):
			built = buildFaultArrayItem(domain.item, domain.negativeItems, tape)
		default:
			built = buildPositiveChild(domain.item, tape)
		}

		if built.err != nil || built.state != buildComplete {
			return built
		}

		values = append(values, built.value)
	}

	for domain.maximum == nil || new(big.Int).SetInt64(int64(len(values))).Cmp(domain.maximum) < 0 {
		if tape.takeByte() == 0 {
			break
		}

		built := buildPositiveChild(domain.item, tape)
		if built.err != nil || built.state != buildComplete {
			return built
		}

		values = append(values, built.value)
	}

	return buildSelectedValue(selected, jsonvalue.Array(values))
}

type faultArrayDomain struct {
	minimum       *big.Int
	maximum       *big.Int
	item          *expression
	negativeItems []faultNegativeArrayItem
	routedChild   *expression
}

type faultNegativeArrayItem struct {
	expression *expression
}

//nolint:cyclop,gocognit // Each array count/item atom contributes to one exact local domain.
func collectFaultArrayDomain(selected []demand) (faultArrayDomain, error) {
	domain := faultArrayDomain{minimum: big.NewInt(0)}

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil || selectedDemand.expression.kind != expressionAtom {
			return faultArrayDomain{}, fmt.Errorf("array demand %d is not an atom", index)
		}

		rule := selectedDemand.expression.atom
		switch rule.kind {
		case atomArrayMinItems:
			count, err := countBigInt(rule.count, "minItems")
			if err != nil {
				return faultArrayDomain{}, err
			}

			if selectedDemand.wantPass {
				if count.Cmp(domain.minimum) > 0 {
					domain.minimum = count
				}
			} else {
				upper := new(big.Int).Sub(count, big.NewInt(1))
				if domain.maximum == nil || upper.Cmp(domain.maximum) < 0 {
					domain.maximum = upper
				}
			}
		case atomArrayMaxItems:
			count, err := countBigInt(rule.count, "maxItems")
			if err != nil {
				return faultArrayDomain{}, err
			}

			if selectedDemand.wantPass {
				if domain.maximum == nil || count.Cmp(domain.maximum) < 0 {
					domain.maximum = count
				}
			} else {
				lower := new(big.Int).Add(count, big.NewInt(1))
				if lower.Cmp(domain.minimum) > 0 {
					domain.minimum = lower
				}
			}
		case atomArrayItems:
			if rule.child == nil {
				return faultArrayDomain{}, errors.New("array items has nil child")
			}

			if selectedDemand.wantPass {
				if domain.item != nil && domain.item != rule.child {
					return faultArrayDomain{}, errors.New("multiple positive array item children")
				}

				domain.item = rule.child
			} else {
				domain.negativeItems = append(domain.negativeItems, faultNegativeArrayItem{
					expression: rule.child,
				})
				if domain.routedChild == nil {
					domain.routedChild = rule.child
				}
			}
		}
	}

	return domain, nil
}

func selectArrayFaultIndex(word uint64, maximum *big.Int) int {
	if maximum == nil {
		return int(word)
	}

	count := new(big.Int).Add(maximum, big.NewInt(1))
	count.Mod(new(big.Int).SetUint64(word), count)

	return int(count.Int64())
}

func buildFaultArrayItem(
	positive *expression,
	negative []faultNegativeArrayItem,
	tape *tapeCursor,
) buildResult {
	selected := make([]demand, 0)
	if positive != nil {
		if err := selectPassingDemands(positive, tape, &selected); err != nil {
			return failedBuild(err)
		}
	}

	for index, item := range negative {
		if err := selectFailingDemands(item.expression, tape, &selected); err != nil {
			return failedBuild(fmt.Errorf("select failing array item %d: %w", index, err))
		}
	}

	built := buildValue(selected, tape)
	if built.err != nil || built.state != buildComplete {
		return built
	}

	holds, err := demandsHold(selected, built.value)
	if err != nil {
		return failedBuild(fmt.Errorf("evaluate array item: %w", err))
	}

	if !holds {
		return missedBuild()
	}

	return built
}

//nolint:cyclop,gocognit,gocyclo,nestif,maintidx // Object fault construction has one required, forced, optional order.
func buildFaultObject(
	selected []demand,
	tape *tapeCursor,
	context faultBuildContext,
) buildResult {
	enumValues, hasEnum, err := selectedEnumValues(selected)
	if err != nil {
		return failedBuild(err)
	}

	if hasEnum {
		objects := make([]jsonvalue.Value, 0, len(enumValues))
		for _, value := range enumValues {
			if value.Kind == jsonvalue.KindObject {
				objects = append(objects, value)
			}
		}

		if len(objects) == 0 {
			return missedBuild()
		}

		candidate := objects[0]
		if len(objects) > 1 {
			candidate = objects[tape.takeWord()%uint64(len(objects))]
		}

		return buildSelectedValue(selected, candidate)
	}

	domain, err := collectFaultObjectDomain(selected)
	if err != nil {
		return failedBuild(err)
	}

	if domain.maximum != nil &&
		(domain.maximum.Sign() < 0 || domain.minimum.Cmp(domain.maximum) > 0) {
		return missedBuild()
	}

	forcedAdditional := len(domain.negativeAdditional) > 0

	var (
		routedProperty   string
		routedAdditional bool
	)

	if context.route {
		if context.token == nil || context.token.plan == nil ||
			context.planStep >= len(context.token.plan.schemaPath) {
			return failedBuild(errors.New("object fault route has no path step"))
		}

		step := context.token.plan.schemaPath[context.planStep]
		switch step.kind {
		case faultProperty:
			if _, ok := domain.negativeProperties[step.property]; !ok {
				return failedBuild(fmt.Errorf("object fault property %q is not selected", step.property))
			}

			routedProperty = step.property
		case faultAdditionalProperty:
			routedAdditional = true
		default:
			return failedBuild(errors.New("object fault route does not select a property"))
		}
	}

	members := make([]jsonvalue.Member, 0)
	present := make(map[string]struct{})

	include := func(name string, build func() buildResult) buildResult {
		if _, exists := present[name]; exists {
			return failedBuild(fmt.Errorf("object name %q selected twice", name))
		}

		if domain.maximum != nil &&
			new(big.Int).SetInt64(int64(len(members))).Cmp(domain.maximum) >= 0 {
			return missedBuild()
		}

		built := build()
		if built.err != nil || built.state != buildComplete {
			return built
		}

		present[name] = struct{}{}
		members = append(members, jsonvalue.Member{Name: name, Value: built.value})

		return built
	}

	for _, name := range domain.knownNames {
		if domain.omittedRequired[name] {
			continue
		}

		if !domain.required[name] {
			continue
		}

		built := include(name, func() buildResult {
			return buildFaultPropertyValue(
				name, domain.properties[name], domain.negativeProperties[name],
				name == routedProperty, context, tape,
			)
		})
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	for _, name := range domain.requiredNames {
		if domain.omittedRequired[name] {
			continue
		}

		if _, known := domain.properties[name]; known {
			continue
		}

		if _, known := domain.negativeProperties[name]; known {
			continue
		}

		if !domain.additionalAllowed {
			return missedBuild()
		}

		built := include(name, func() buildResult {
			return buildPositiveChild(domain.additionalChild, tape)
		})
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	for new(big.Int).SetInt64(int64(len(members))).Cmp(domain.minimum) < 0 {
		added := false

		for _, name := range domain.knownNames {
			if domain.omittedRequired[name] {
				continue
			}

			if _, exists := present[name]; exists {
				continue
			}

			built := include(name, func() buildResult {
				if negative, forced := domain.negativeProperties[name]; forced {
					return buildFaultPropertyValue(
						name, domain.properties[name], negative,
						name == routedProperty, context, tape,
					)
				}

				return buildPositiveChild(domain.properties[name], tape)
			})
			if built.err != nil || built.state != buildComplete {
				return built
			}

			added = true

			break
		}

		if added {
			continue
		}

		if !domain.additionalAllowed {
			return missedBuild()
		}

		name := nextAdditionalName(present, domain.knownNames, 0)

		built := include(name, func() buildResult {
			return buildPositiveChild(domain.additionalChild, tape)
		})
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	for _, name := range domain.knownNames {
		if domain.omittedRequired[name] {
			continue
		}

		if _, exists := present[name]; exists {
			continue
		}

		if negative, forced := domain.negativeProperties[name]; forced {
			built := include(name, func() buildResult {
				return buildFaultPropertyValue(
					name, domain.properties[name], negative,
					name == routedProperty, context, tape,
				)
			})
			if built.err != nil || built.state != buildComplete {
				return built
			}

			continue
		}

		if domain.maximum != nil &&
			new(big.Int).SetInt64(int64(len(members))).Cmp(domain.maximum) >= 0 {
			break
		}

		if tape.takeByte() == 0 {
			continue
		}

		built := include(name, func() buildResult {
			return buildPositiveChild(domain.properties[name], tape)
		})
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	if forcedAdditional {
		if !domain.additionalAllowed {
			return missedBuild()
		}

		nameWord := tape.takeWord()

		name := fmt.Sprintf("additional%d", nameWord)
		if _, exists := present[name]; exists || containsName(domain.knownNames, name) {
			return missedBuild()
		}

		built := include(name, func() buildResult {
			if routedAdditional {
				child := firstNegativeAdditionalChild(domain.negativeAdditional)
				if child == nil {
					return missedBuild()
				}

				return buildValueWithFault(child, context.token, context.planStep+1, tape)
			}

			return buildFaultAdditionalValue(domain, tape)
		})
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	additionalIndex := 0

	for domain.additionalAllowed &&
		(domain.maximum == nil || new(big.Int).SetInt64(int64(len(members))).Cmp(domain.maximum) < 0) {
		if tape.takeByte() == 0 {
			break
		}

		name := nextAdditionalName(present, domain.knownNames, additionalIndex)
		additionalIndex++

		built := include(name, func() buildResult {
			return buildPositiveChild(domain.additionalChild, tape)
		})
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	value, err := jsonvalue.Object(members)
	if err != nil {
		return failedBuild(fmt.Errorf("construct object: %w", err))
	}

	return buildSelectedValue(selected, value)
}

type faultObjectDomain struct {
	minimum            *big.Int
	maximum            *big.Int
	required           map[string]bool
	requiredNames      []string
	omittedRequired    map[string]bool
	knownNames         []string
	properties         map[string]*expression
	negativeProperties map[string][]*expression
	additionalChild    *expression
	additionalAllowed  bool
	negativeAdditional []faultObjectAdditional
}

type faultObjectAdditional struct {
	hasChild bool
	child    *expression
}

//nolint:cyclop,gocognit,gocyclo,nestif // Object atoms map directly to one fixed construction domain.
func collectFaultObjectDomain(selected []demand) (faultObjectDomain, error) {
	domain := faultObjectDomain{
		minimum:            big.NewInt(0),
		required:           make(map[string]bool),
		omittedRequired:    make(map[string]bool),
		properties:         make(map[string]*expression),
		negativeProperties: make(map[string][]*expression),
		additionalAllowed:  true,
	}

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil || selectedDemand.expression.kind != expressionAtom {
			return faultObjectDomain{}, fmt.Errorf("object demand %d is not an atom", index)
		}

		rule := selectedDemand.expression.atom
		switch rule.kind {
		case atomObjectMinProperties:
			count, err := countBigInt(rule.count, "minProperties")
			if err != nil {
				return faultObjectDomain{}, err
			}

			if selectedDemand.wantPass {
				if count.Cmp(domain.minimum) > 0 {
					domain.minimum = count
				}
			} else {
				upper := new(big.Int).Sub(count, big.NewInt(1))
				if domain.maximum == nil || upper.Cmp(domain.maximum) < 0 {
					domain.maximum = upper
				}
			}
		case atomObjectMaxProperties:
			count, err := countBigInt(rule.count, "maxProperties")
			if err != nil {
				return faultObjectDomain{}, err
			}

			if selectedDemand.wantPass {
				if domain.maximum == nil || count.Cmp(domain.maximum) < 0 {
					domain.maximum = count
				}
			} else {
				lower := new(big.Int).Add(count, big.NewInt(1))
				if lower.Cmp(domain.minimum) > 0 {
					domain.minimum = lower
				}
			}
		case atomObjectRequired:
			if len(rule.names) == 0 {
				return faultObjectDomain{}, errors.New("required atom has no names")
			}

			if selectedDemand.wantPass {
				for _, name := range rule.names {
					domain.required[name] = true
				}
			} else {
				domain.omittedRequired[rule.names[0]] = true
				for _, name := range rule.names[1:] {
					domain.required[name] = true
				}
			}
		case atomObjectProperty:
			if rule.child == nil {
				return faultObjectDomain{}, errors.New("object property has nil child")
			}

			if selectedDemand.wantPass {
				if existing, exists := domain.properties[rule.name]; exists && existing != rule.child {
					return faultObjectDomain{}, fmt.Errorf("property %q has conflicting positive children", rule.name)
				}

				domain.properties[rule.name] = rule.child
			} else {
				domain.negativeProperties[rule.name] = append(
					domain.negativeProperties[rule.name], rule.child,
				)
			}
		case atomObjectAdditional:
			if selectedDemand.wantPass {
				if rule.hasChild {
					if rule.child == nil {
						return faultObjectDomain{}, errors.New("additional properties has nil child")
					}

					domain.additionalChild = rule.child
					domain.additionalAllowed = true
				} else {
					domain.additionalAllowed = rule.allowedAdditional
				}
			} else {
				domain.negativeAdditional = append(domain.negativeAdditional, faultObjectAdditional{
					hasChild: rule.hasChild, child: rule.child,
				})
			}
		}
	}

	for name := range domain.required {
		domain.requiredNames = append(domain.requiredNames, name)
	}

	for name := range domain.properties {
		domain.knownNames = append(domain.knownNames, name)
	}

	for name := range domain.negativeProperties {
		domain.knownNames = append(domain.knownNames, name)
	}

	sort.Strings(domain.requiredNames)
	sort.Strings(domain.knownNames)

	return domain, nil
}

//nolint:cyclop // Property witness selection combines one positive and negative route.
func buildFaultPropertyValue(
	name string,
	positive *expression,
	negative []*expression,
	route bool,
	context faultBuildContext,
	tape *tapeCursor,
) buildResult {
	if route {
		if positive == nil && len(negative) == 0 {
			return missedBuild()
		}

		child := positive
		if child == nil {
			child = negative[0]
		}

		return buildValueWithFault(child, context.token, context.planStep+1, tape)
	}

	selected := make([]demand, 0)
	if positive != nil {
		if err := selectPassingDemands(positive, tape, &selected); err != nil {
			return failedBuild(fmt.Errorf("select property %q demands: %w", name, err))
		}
	}

	for index, child := range negative {
		if err := selectFailingDemands(child, tape, &selected); err != nil {
			return failedBuild(fmt.Errorf("select property %q failure %d: %w", name, index, err))
		}
	}

	if len(selected) == 0 {
		return completeBuild(jsonvalue.Null())
	}

	built := buildFaultValue(selected, tape, faultBuildContext{})
	if built.err != nil || built.state != buildComplete {
		return built
	}

	holds, err := demandsHold(selected, built.value)
	if err != nil {
		return failedBuild(fmt.Errorf("evaluate property %q: %w", name, err))
	}

	if !holds {
		return missedBuild()
	}

	return built
}

func firstNegativeAdditionalChild(negative []faultObjectAdditional) *expression {
	for _, rule := range negative {
		if rule.hasChild {
			return rule.child
		}
	}

	return nil
}

//nolint:cyclop // Additional witness selection is one fixed demand combination.
func buildFaultAdditionalValue(domain faultObjectDomain, tape *tapeCursor) buildResult {
	selected := make([]demand, 0)
	if domain.additionalChild != nil {
		if err := selectPassingDemands(domain.additionalChild, tape, &selected); err != nil {
			return failedBuild(err)
		}
	}

	for index, rule := range domain.negativeAdditional {
		if !rule.hasChild {
			continue
		}

		if err := selectFailingDemands(rule.child, tape, &selected); err != nil {
			return failedBuild(fmt.Errorf("select additional failure %d: %w", index, err))
		}
	}

	if len(selected) == 0 {
		return completeBuild(jsonvalue.Null())
	}

	built := buildFaultValue(selected, tape, faultBuildContext{})
	if built.err != nil || built.state != buildComplete {
		return built
	}

	holds, err := demandsHold(selected, built.value)
	if err != nil {
		return failedBuild(fmt.Errorf("evaluate additional value: %w", err))
	}

	if !holds {
		return missedBuild()
	}

	return built
}

func buildPositiveExpression(expression *expression, tape *tapeCursor) buildResult {
	selected := make([]demand, 0)
	if err := selectPassingDemands(expression, tape, &selected); err != nil {
		return failedBuild(fmt.Errorf("select positive demands: %w", err))
	}

	built := buildValue(selected, tape)
	if built.err != nil || built.state != buildComplete {
		return built
	}

	holds, err := demandsHold(selected, built.value)
	if err != nil {
		return failedBuild(fmt.Errorf("evaluate positive demands: %w", err))
	}

	if !holds {
		return missedBuild()
	}

	return built
}

//nolint:cyclop,gocognit,gocyclo,nestif // Fault demand selection mirrors the closed expression model.
func selectFaultDemands(
	currentExpression *expression,
	token *faultToken,
	planStep int,
	tape *tapeCursor,
	selected *[]demand,
) (faultHere bool, route bool, err error) {
	if currentExpression == nil {
		return false, false, errors.New("select fault demands from nil expression")
	}

	if token == nil || token.plan == nil {
		return false, false, errors.New("select fault demands with nil token")
	}

	if tape == nil {
		return false, false, errors.New("select fault demands with nil tape cursor")
	}

	if selected == nil {
		return false, false, errors.New("select fault demands with nil output")
	}

	plan := token.plan
	if planStep < 0 || planStep > len(plan.schemaPath) {
		return false, false, fmt.Errorf("invalid fault path step %d", planStep)
	}

	if currentExpression == plan.target {
		if planStep != len(plan.schemaPath) {
			return false, false, fmt.Errorf(
				"fault target reached at path step %d of %d",
				planStep, len(plan.schemaPath),
			)
		}

		switch plan.targetKind {
		case faultAtom:
			if currentExpression.kind != expressionAtom {
				return false, false, errors.New("atomic fault target is not an atom")
			}

			*selected = append(*selected, newDemand(currentExpression, false))

			return true, false, nil
		case faultAll:
			if currentExpression.kind != expressionAll {
				return false, false, errors.New("all fault target is not an all expression")
			}

			if plan.failedChild < 0 || plan.failedChild >= len(currentExpression.children) {
				return false, false, fmt.Errorf("invalid failed all child %d", plan.failedChild)
			}

			for index, child := range currentExpression.children {
				if index == plan.failedChild {
					if err := selectFailingDemands(child, tape, selected); err != nil {
						return false, false, fmt.Errorf("select failed all child %d: %w", index, err)
					}

					continue
				}

				if err := selectPassingDemands(child, tape, selected); err != nil {
					return false, false, fmt.Errorf("select passing all child %d: %w", index, err)
				}
			}

			return true, false, nil
		case faultAny:
			if currentExpression.kind != expressionAny {
				return false, false, errors.New("any fault target is not an any expression")
			}

			if len(currentExpression.children) == 0 {
				return false, false, errors.New("any fault target has no branches")
			}

			for index, child := range currentExpression.children {
				if err := selectFailingDemands(child, tape, selected); err != nil {
					return false, false, fmt.Errorf("select failed any branch %d: %w", index, err)
				}
			}

			return true, false, nil
		default:
			return false, false, fmt.Errorf("unknown fault target kind %d", plan.targetKind)
		}
	}

	switch currentExpression.kind {
	case expressionAtom:
		reaches, reachErr := faultPathCanReach(currentExpression, plan.target, plan, planStep)
		if reachErr != nil {
			return false, false, reachErr
		}

		if reaches {
			if err := appendFaultCarrier(currentExpression, plan, planStep, selected); err != nil {
				return false, false, err
			}

			return false, true, nil
		}

		*selected = append(*selected, newDemand(currentExpression, true))

		return false, false, nil
	case expressionAll:
		var (
			foundFault bool
			foundRoute bool
		)

		for index, child := range currentExpression.children {
			reaches, reachErr := faultPathCanReach(child, plan.target, plan, planStep)
			if reachErr != nil {
				return false, false, fmt.Errorf("inspect all child %d: %w", index, reachErr)
			}

			if reaches {
				childFault, childRoute, childErr := selectFaultDemands(
					child, token, planStep, tape, selected,
				)
				if childErr != nil {
					return false, false, fmt.Errorf("select fault all child %d: %w", index, childErr)
				}

				if foundFault || foundRoute {
					return false, false, errors.New("fault plan reached more than once")
				}

				foundFault = childFault
				foundRoute = childRoute

				continue
			}

			if err := selectPassingDemands(child, tape, selected); err != nil {
				return false, false, fmt.Errorf("select passing all child %d: %w", index, err)
			}
		}

		return foundFault, foundRoute, nil
	case expressionAny:
		if len(currentExpression.children) == 0 {
			return false, false, errors.New("select empty any expression")
		}

		choice, chooseErr := chooseExpressionChild(currentExpression.children, tape)
		if chooseErr != nil {
			return false, false, chooseErr
		}

		return selectFaultDemands(currentExpression.children[choice], token, planStep, tape, selected)
	default:
		return false, false, fmt.Errorf("unknown expression kind %d", currentExpression.kind)
	}
}

func appendFaultCarrier(
	expression *expression,
	plan *faultPlan,
	planStep int,
	selected *[]demand,
) error {
	if expression.kind != expressionAtom {
		return errors.New("fault path carrier is not an atom")
	}

	if !faultPathMatchesAtom(expression.atom, plan.schemaPath[planStep]) {
		return fmt.Errorf("fault path does not match atom at step %d", planStep)
	}

	*selected = append(*selected, newDemand(expression, false))

	return nil
}

func faultPathMatchesAtom(rule atom, step faultPathStep) bool {
	switch step.kind {
	case faultProperty:
		return rule.kind == atomObjectProperty && rule.name == step.property
	case faultArrayItem:
		return rule.kind == atomArrayItems
	case faultAdditionalProperty:
		return rule.kind == atomObjectAdditional && rule.hasChild
	default:
		return false
	}
}

//nolint:cyclop // Path reachability follows one expression and structural route.
func faultPathCanReach(
	rootExpression *expression,
	target *expression,
	plan *faultPlan,
	planStep int,
) (bool, error) {
	active := make(map[*expression]bool)

	var inspect func(*expression, int) (bool, error)

	inspect = func(current *expression, step int) (bool, error) {
		if current == nil {
			return false, errors.New("fault path contains nil expression")
		}

		if current == target {
			return step == len(plan.schemaPath), nil
		}

		if active[current] {
			return false, fmt.Errorf("fault path expression cycle at %p", current)
		}

		active[current] = true
		defer delete(active, current)

		switch current.kind {
		case expressionAtom:
			if step >= len(plan.schemaPath) {
				return false, nil
			}

			next := plan.schemaPath[step]
			if !faultPathMatchesAtom(current.atom, next) {
				return false, nil
			}

			return inspect(current.atom.child, step+1)
		case expressionAll:
			for _, child := range current.children {
				reaches, err := inspect(child, step)
				if err != nil || reaches {
					return reaches, err
				}
			}

			return false, nil
		case expressionAny:
			return false, nil
		default:
			return false, fmt.Errorf("unknown expression kind %d", current.kind)
		}
	}

	return inspect(rootExpression, planStep)
}

func chooseExpressionChild(children []*expression, tape *tapeCursor) (int, error) {
	if len(children) == 0 {
		return 0, errors.New("choose expression child from empty list")
	}

	if len(children) == 1 {
		return 0, nil
	}

	return tape.choose(len(children))
}

//nolint:cyclop // Negative all and any semantics are one explicit closed dispatch.
func selectFailingDemands(
	expression *expression,
	tape *tapeCursor,
	selected *[]demand,
) error {
	if expression == nil {
		return errors.New("select failing demands from nil expression")
	}

	switch expression.kind {
	case expressionAtom:
		*selected = append(*selected, newDemand(expression, false))

		return nil
	case expressionAll:
		if len(expression.children) == 0 {
			return errors.New("cannot fail empty all expression")
		}

		failedChild, err := chooseExpressionChild(expression.children, tape)
		if err != nil {
			return err
		}

		for index, child := range expression.children {
			if index == failedChild {
				if err := selectFailingDemands(child, tape, selected); err != nil {
					return fmt.Errorf("select all failure child %d: %w", index, err)
				}

				continue
			}

			if err := selectPassingDemands(child, tape, selected); err != nil {
				return fmt.Errorf("select all passing child %d: %w", index, err)
			}
		}

		return nil
	case expressionAny:
		if len(expression.children) == 0 {
			return errors.New("cannot fail empty any expression")
		}

		for index, child := range expression.children {
			if err := selectFailingDemands(child, tape, selected); err != nil {
				return fmt.Errorf("select any failure branch %d: %w", index, err)
			}
		}

		return nil
	default:
		return fmt.Errorf("unknown expression kind %d", expression.kind)
	}
}
