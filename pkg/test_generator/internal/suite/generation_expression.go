//nolint:godoclint // Private expression algebra names are intentionally concise and local.
package suite

import (
	"errors"
	"fmt"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

func choose(values ...generationExpression) generationExpression {
	branches := make([]generationExpression, 0, len(values))
	for _, value := range values {
		if !generationExpressionEmpty(value) {
			branches = append(branches, value)
		}
	}

	if len(branches) == 0 {
		return generationExpression{}
	}

	if len(branches) == 1 {
		return branches[0]
	}

	return generationExpression{choice: &generationChoice{branches: branches}}
}

func domainGenerationExpression(
	domain DomainID,
	use *schemaUse,
	stringLanguage *stringLanguageOccurrence,
) generationExpression {
	var languages []*stringLanguageOccurrence
	if stringLanguage != nil {
		languages = []*stringLanguageOccurrence{stringLanguage}
	}

	return generationExpression{term: &generationTerm{
		domain: domain, use: use, stringLanguages: languages,
	}}
}

func liftedDomainGenerationExpression(
	domain DomainID,
	use *schemaUse,
	child generationExpression,
) generationExpression {
	result := domainGenerationExpression(domain, use, nil)
	if child.term != nil {
		result.term.stringLanguages = append(
			[]*stringLanguageOccurrence(nil), child.term.stringLanguages...,
		)
	}

	return result
}

func generationExpressionDomain(expression generationExpression) DomainID {
	if expression.term == nil {
		return NoDomain
	}

	return expression.term.domain
}

func meet(values ...generationExpression) (generationExpression, error) {
	if len(values) == 0 {
		return generationExpression{}, nil
	}

	result := values[0]
	for _, value := range values[1:] {
		var err error

		result, err = meetGenerationExpressions(result, value)
		if err != nil {
			return generationExpression{}, err
		}
	}

	return result, nil
}

//nolint:cyclop,gocognit // Choice distribution and term conjunction are the complete expression algebra.
func meetGenerationExpressions(
	left generationExpression,
	right generationExpression,
) (generationExpression, error) {
	if generationExpressionEmpty(left) || generationExpressionEmpty(right) {
		return generationExpression{}, nil
	}

	if left.choice != nil {
		branches := make([]generationExpression, 0, len(left.choice.branches))
		for _, branch := range left.choice.branches {
			combined, err := meetGenerationExpressions(branch, right)
			if err != nil {
				return generationExpression{}, err
			}

			branches = append(branches, combined)
		}

		return choose(branches...), nil
	}

	if right.choice != nil {
		branches := make([]generationExpression, 0, len(right.choice.branches))
		for _, branch := range right.choice.branches {
			combined, err := meetGenerationExpressions(left, branch)
			if err != nil {
				return generationExpression{}, err
			}

			branches = append(branches, combined)
		}

		return choose(branches...), nil
	}

	if left.term == nil || right.term == nil || left.term.use == nil || left.term.use.domains == nil {
		return generationExpression{}, errors.New("meet generation expressions: term is incomplete")
	}

	domain, err := left.term.use.domains.IntersectDomains(left.term.domain, right.term.domain)
	if err != nil {
		return generationExpression{}, err
	}

	if domain == EmptyDomainID {
		return generationExpression{}, nil
	}

	use := mergeGenerationUses(left.term.use, right.term.use, domain)

	term := &generationTerm{domain: domain, use: use}
	term.stringLanguages = append(
		append([]*stringLanguageOccurrence(nil), left.term.stringLanguages...),
		right.term.stringLanguages...,
	)
	term.excludedValues = append(
		append([]jsonvalue.Value(nil), left.term.excludedValues...),
		right.term.excludedValues...,
	)
	term.numberFailures = append(
		append([]numberFailure(nil), left.term.numberFailures...),
		right.term.numberFailures...,
	)

	term.items, err = meetOptionalExpressions(left.term.items, right.term.items)
	if err != nil {
		return generationExpression{}, err
	}

	term.additional, err = meetOptionalExpressions(left.term.additional, right.term.additional)
	if err != nil {
		return generationExpression{}, err
	}

	term.properties, err = meetPropertyExpressions(left.term.properties, right.term.properties)
	if err != nil {
		return generationExpression{}, err
	}

	if term.items != nil && generationExpressionEmpty(*term.items) ||
		term.additional != nil && generationExpressionEmpty(*term.additional) {
		return generationExpression{}, nil
	}

	compiledDomain, ok := left.term.use.domains.Domain(domain)
	if !ok {
		return generationExpression{}, fmt.Errorf("meet generation expressions: Domain %d does not exist", domain)
	}

	for name, expression := range term.properties {
		if !generationExpressionEmpty(expression) {
			continue
		}

		required := false

		for _, property := range compiledDomain.Object.Properties {
			if property.Name == name {
				required = property.Required

				break
			}
		}

		if required {
			return generationExpression{}, nil
		}

		delete(term.properties, name)
	}

	return generationExpression{term: term}, nil
}

func mergeGenerationUses(left *schemaUse, right *schemaUse, domain DomainID) *schemaUse {
	if left == nil {
		return right
	}

	if right == nil {
		return left
	}

	result := *left
	result.domain = domain

	result.stringLanguages = append(
		append([]stringLanguageOccurrence(nil), left.stringLanguages...),
		right.stringLanguages...,
	)

	result.allOf = append(append([]*schemaUse(nil), left.allOf...), right)
	if result.items == nil {
		result.items = right.items
	}

	if result.additional == nil {
		result.additional = right.additional
	}

	if len(result.properties) == 0 {
		result.properties = append([]schemaPropertyUse(nil), right.properties...)
	}

	return &result
}

func meetOptionalExpressions(
	left *generationExpression,
	right *generationExpression,
) (*generationExpression, error) {
	if left == nil {
		return right, nil
	}

	if right == nil {
		return left, nil
	}

	combined, err := meet(*left, *right)
	if err != nil {
		return nil, err
	}

	return &combined, nil
}

func meetPropertyExpressions(
	left map[string]generationExpression,
	right map[string]generationExpression,
) (map[string]generationExpression, error) {
	result := make(map[string]generationExpression, len(left)+len(right))
	for name, value := range left {
		result[name] = value
	}

	for name, value := range right {
		if existing, ok := result[name]; ok {
			combined, err := meet(existing, value)
			if err != nil {
				return nil, err
			}

			result[name] = combined
		} else {
			result[name] = value
		}
	}

	return result, nil
}

func not(use *schemaUse) (generationExpression, error) {
	if use == nil || use.domains == nil {
		return generationExpression{}, errors.New("complement schema occurrence is nil")
	}

	values, err := atomicComplementExpressions(use)
	if err != nil {
		return generationExpression{}, err
	}

	compositionFailures, err := anyOfComplementExpressions(use)
	if err != nil {
		return generationExpression{}, err
	}

	values = append(values, compositionFailures...)

	childFailures, err := childComplementExpressions(use)
	if err != nil {
		return generationExpression{}, err
	}

	values = append(values, childFailures...)

	return choose(values...), nil
}

//nolint:cyclop,gocognit // Each schema keyword contributes one distinct exact atomic complement.
func atomicComplementExpressions(use *schemaUse) ([]generationExpression, error) {
	planner := &CasePlanner{Domains: use.domains, rootUse: use}
	seen := make(map[ConstraintSource]struct{})
	result := make([]generationExpression, 0, len(use.constraints))

	for _, source := range use.constraints {
		if _, duplicate := seen[source]; duplicate {
			continue
		}

		seen[source] = struct{}{}

		if source.Keyword == "allOf" || source.Keyword == "items" || source.Keyword == "properties" {
			continue
		}

		occurrence := use.find(source.Pointer)
		if occurrence == nil {
			occurrence = use
		}

		constraint, include, err := planner.atomicConstraint(source, occurrence)
		if err != nil {
			return nil, err
		}

		if !include {
			continue
		}

		if source.Keyword != "format" {
			for _, domain := range constraint.Fail {
				result = append(result, generationExpression{term: &generationTerm{domain: domain, use: occurrence}})
			}
		}

		switch source.Keyword {
		case "enum":
			domain, ok := use.domains.Domain(constraint.Pass)
			if !ok || domain.Enum == nil {
				return nil, fmt.Errorf("cannot construct exact complement of enum at %s", source.Pointer)
			}

			result = append(result, generationExpression{term: &generationTerm{
				domain: AnyJSONDomainID, use: occurrence,
				excludedValues: cloneJSONValues(domain.Enum.Values),
			}})
		case "type":
			pass, ok := use.domains.Domain(constraint.Pass)
			if ok && pass.Number.IntegersOnly {
				result = append(result, generationExpression{term: &generationTerm{
					domain: use.domains.FindOrAddEquivalentDomain(singleKindDomain(jsonvalue.KindNumber)),
					use:    occurrence, numberFailures: []numberFailure{{integer: true}},
				}})
			}
		case "multipleOf":
			pass, ok := use.domains.Domain(constraint.Pass)
			if !ok || pass.Number.MultipleOf == nil {
				return nil, fmt.Errorf("cannot construct exact complement of multipleOf at %s", source.Pointer)
			}

			result = append(result, generationExpression{term: &generationTerm{
				domain: use.domains.FindOrAddEquivalentDomain(singleKindDomain(jsonvalue.KindNumber)),
				use:    occurrence, numberFailures: []numberFailure{{multipleOf: cloneNumber(pass.Number.MultipleOf)}},
			}})
		case "pattern":
			target := planner.stringLanguageOccurrence(source)
			if target == nil {
				return nil, fmt.Errorf("cannot construct exact complement of pattern at %s", source.Pointer)
			}

			targetCopy := *target
			result = append(result, generationExpression{term: &generationTerm{
				domain: use.domains.FindOrAddEquivalentDomain(singleKindDomain(jsonvalue.KindString)),
				use:    occurrence, stringLanguages: []*stringLanguageOccurrence{&targetCopy},
			}})
		case "format":
			pass, ok := use.domains.Domain(constraint.Pass)
			if ok && domainHasStringFormat(pass) {
				target := planner.stringLanguageOccurrence(source)
				if target == nil {
					return nil, fmt.Errorf("cannot construct exact complement of format at %s", source.Pointer)
				}

				targetCopy := *target
				result = append(result, generationExpression{term: &generationTerm{
					domain: use.domains.FindOrAddEquivalentDomain(singleKindDomain(jsonvalue.KindString)),
					use:    occurrence, stringLanguages: []*stringLanguageOccurrence{&targetCopy},
				}})
			} else if ok && pass.Number.State != KindExcluded {
				complements, complementErr := numericFormatComplementExpressions(use.domains, occurrence, pass.Number)
				if complementErr != nil {
					return nil, fmt.Errorf("numeric format complement at %s: %w", source.Pointer, complementErr)
				}

				result = append(result, complements...)
			} else {
				return nil, fmt.Errorf("cannot construct exact complement of numeric format at %s", source.Pointer)
			}
		}
	}

	return result, nil
}

func numericFormatComplementExpressions(
	domains *DomainRegistry,
	use *schemaUse,
	format NumberConstraints,
) ([]generationExpression, error) {
	result := make([]generationExpression, 0)

	if format.IntegersOnly {
		result = append(result, generationExpression{term: &generationTerm{
			domain: domains.FindOrAddEquivalentDomain(singleKindDomain(jsonvalue.KindNumber)),
			use:    use, numberFailures: []numberFailure{{integer: true}},
		}})
	}

	if format.Minimum != nil {
		domain := singleKindDomain(jsonvalue.KindNumber)
		domain.Number.State = KindRestricted
		domain.Number.Maximum = &NumberBound{
			Value: format.Minimum.Value, Exclusive: !format.Minimum.Exclusive,
		}
		result = append(result, domainGenerationExpression(
			domains.FindOrAddEquivalentDomain(domain), use, nil,
		))
	}

	if format.Maximum != nil {
		domain := singleKindDomain(jsonvalue.KindNumber)
		domain.Number.State = KindRestricted
		domain.Number.Minimum = &NumberBound{
			Value: format.Maximum.Value, Exclusive: !format.Maximum.Exclusive,
		}
		result = append(result, domainGenerationExpression(
			domains.FindOrAddEquivalentDomain(domain), use, nil,
		))
	}

	if len(result) == 0 {
		return nil, errors.New("numeric format has no restrictive semantics")
	}

	return result, nil
}

func anyOfComplementExpressions(use *schemaUse) ([]generationExpression, error) {
	result := make([]generationExpression, 0)
	for _, group := range anyOfGroups(use) {
		branches := make([]generationExpression, 0, len(group))
		for _, branch := range group {
			complement, err := not(branch)
			if err != nil {
				return nil, err
			}

			branches = append(branches, complement)
		}

		failure, err := meet(branches...)
		if err != nil {
			return nil, err
		}

		result = append(result, failure)
	}

	return result, nil
}

func childComplementExpressions(use *schemaUse) ([]generationExpression, error) {
	result := make([]generationExpression, 0)

	if use.items != nil {
		complement, err := not(use.items)
		if err != nil {
			return nil, err
		}

		if !generationExpressionEmpty(complement) {
			domain := singleKindDomain(jsonvalue.KindArray)
			domain.Array.State = KindRestricted
			domain.Array.MinItems = 1
			domain.Array.Items = AnyJSONDomainID
			result = append(result, generationExpression{term: &generationTerm{
				domain: use.domains.FindOrAddEquivalentDomain(domain), use: use, items: &complement,
			}})
		}
	}

	for _, property := range use.properties {
		complement, err := not(property.use)
		if err != nil {
			return nil, err
		}

		if generationExpressionEmpty(complement) {
			continue
		}

		domain := singleKindDomain(jsonvalue.KindObject)
		domain.Object.State = KindRestricted
		domain.Object.Properties = []NamedProperty{{
			Name: property.name, Required: true, State: PropertyAllowed, Values: AnyJSONDomainID,
		}}
		result = append(result, generationExpression{term: &generationTerm{
			domain: use.domains.FindOrAddEquivalentDomain(domain), use: use,
			properties: map[string]generationExpression{property.name: complement},
		}})
	}

	if use.additional != nil {
		complement, err := not(use.additional)
		if err != nil {
			return nil, err
		}

		if !generationExpressionEmpty(complement) {
			domain := singleKindDomain(jsonvalue.KindObject)
			domain.Object.State = KindRestricted
			domain.Object.MinProps = 1
			result = append(result, generationExpression{term: &generationTerm{
				domain: use.domains.FindOrAddEquivalentDomain(domain), use: use, additional: &complement,
			}})
		}
	}

	return result, nil
}

func validGenerationExpression(use *schemaUse) (generationExpression, error) {
	base, err := generationExpressionForUse(use)
	if err != nil {
		return generationExpression{}, err
	}

	values := []generationExpression{base}

	for _, branchGroup := range anyOfGroups(use) {
		branches := make([]generationExpression, 0, len(branchGroup))
		for _, branch := range branchGroup {
			valid, err := validGenerationExpression(branch)
			if err != nil {
				return generationExpression{}, err
			}

			branches = append(branches, valid)
		}

		values = append(values, choose(branches...))
	}

	return meet(values...)
}

func invalidAnyOfExpression(use *schemaUse) (generationExpression, error) {
	base, err := generationExpressionForUse(use)
	if err != nil {
		return generationExpression{}, err
	}

	values := []generationExpression{base}

	found := false
	for _, branchGroup := range anyOfGroups(use) {
		found = true

		for _, branch := range branchGroup {
			complement, complementErr := not(branch)
			if complementErr != nil {
				return generationExpression{}, complementErr
			}

			values = append(values, complement)
		}
	}

	direct, err := meet(values...)
	if err != nil {
		return generationExpression{}, err
	}

	nested, err := invalidNestedAnyOfExpressions(use)
	if err != nil {
		return generationExpression{}, err
	}

	if !found {
		return choose(nested...), nil
	}

	return choose(append([]generationExpression{direct}, nested...)...), nil
}

func generationExpressionForUse(use *schemaUse) (generationExpression, error) {
	term := &generationTerm{domain: use.domain, use: use}
	if containsAnyOfUse(use.items) {
		items, err := validGenerationExpression(use.items)
		if err != nil {
			return generationExpression{}, err
		}

		term.items = &items
	}

	for _, property := range use.properties {
		if !containsAnyOfUse(property.use) {
			continue
		}

		valid, err := validGenerationExpression(property.use)
		if err != nil {
			return generationExpression{}, err
		}

		if term.properties == nil {
			term.properties = make(map[string]generationExpression)
		}

		term.properties[property.name] = valid
	}

	if containsAnyOfUse(use.additional) {
		additional, err := validGenerationExpression(use.additional)
		if err != nil {
			return generationExpression{}, err
		}

		term.additional = &additional
	}

	return generationExpression{term: term}, nil
}

//nolint:cyclop // Direct and recursive child requirements are assembled at one schema occurrence.
func anyOfRequirementsExpression(use *schemaUse) (generationExpression, error) {
	values := make([]generationExpression, 0)
	for _, branchGroup := range anyOfGroups(use) {
		branches := make([]generationExpression, 0, len(branchGroup))
		for _, branch := range branchGroup {
			valid, err := validGenerationExpression(branch)
			if err != nil {
				return generationExpression{}, err
			}

			branches = append(branches, valid)
		}

		values = append(values, choose(branches...))
	}

	term := &generationTerm{domain: AnyJSONDomainID, use: use}
	hasChildren := false

	if containsAnyOfUse(use.items) {
		valid, err := anyOfRequirementsExpression(use.items)
		if err != nil {
			return generationExpression{}, err
		}

		term.items = &valid
		hasChildren = true
	}

	for _, property := range use.properties {
		if !containsAnyOfUse(property.use) {
			continue
		}

		valid, err := anyOfRequirementsExpression(property.use)
		if err != nil {
			return generationExpression{}, err
		}

		if term.properties == nil {
			term.properties = make(map[string]generationExpression)
		}

		term.properties[property.name] = valid
		hasChildren = true
	}

	if containsAnyOfUse(use.additional) {
		valid, err := anyOfRequirementsExpression(use.additional)
		if err != nil {
			return generationExpression{}, err
		}

		term.additional = &valid
		hasChildren = true
	}

	if hasChildren {
		values = append(values, generationExpression{term: term})
	}

	if len(values) == 0 {
		return generationExpression{term: &generationTerm{domain: AnyJSONDomainID, use: use}}, nil
	}

	return meet(values...)
}

//nolint:cyclop,gocognit,nestif // Container lifting has separate exact preconditions.
func invalidNestedAnyOfExpressions(use *schemaUse) ([]generationExpression, error) {
	var result []generationExpression

	if containsAnyOfUse(use.items) {
		invalid, err := invalidAnyOfExpression(use.items)
		if err != nil {
			return nil, err
		}

		if !generationExpressionEmpty(invalid) {
			domain, ok := use.domains.Domain(use.domain)
			if ok && domain.Array.State != KindExcluded && (domain.Array.MaxItems == nil || *domain.Array.MaxItems > 0) {
				domain.Array.MinItems = max(1, domain.Array.MinItems)
				id := use.domains.FindOrAddEquivalentDomain(domain)
				term := &generationTerm{domain: id, use: use, items: &invalid}
				result = append(result, generationExpression{term: term})
			}
		}
	}

	for _, property := range use.properties {
		if !containsAnyOfUse(property.use) {
			continue
		}

		invalid, err := invalidAnyOfExpression(property.use)
		if err != nil {
			return nil, err
		}

		if generationExpressionEmpty(invalid) {
			continue
		}

		domain, ok := use.domains.Domain(use.domain)
		if !ok || domain.Object.State == KindExcluded {
			continue
		}

		for index := range domain.Object.Properties {
			if domain.Object.Properties[index].Name == property.name &&
				domain.Object.Properties[index].State == PropertyAllowed {
				domain.Object.Properties[index].Required = true
			}
		}

		id := use.domains.FindOrAddEquivalentDomain(domain)
		term := &generationTerm{
			domain: id, use: use, properties: map[string]generationExpression{property.name: invalid},
		}
		result = append(result, generationExpression{term: term})
	}

	if containsAnyOfUse(use.additional) {
		invalid, err := invalidAnyOfExpression(use.additional)
		if err != nil {
			return nil, err
		}

		if !generationExpressionEmpty(invalid) {
			domain, ok := use.domains.Domain(use.domain)
			if ok && domain.Object.State != KindExcluded && domain.Object.Additional.Values != EmptyDomainID {
				required := 0

				for index := range domain.Object.Properties {
					property := &domain.Object.Properties[index]
					if property.Required && property.State == PropertyAllowed {
						required++
					} else {
						property.State = PropertyForbidden
						property.Values = EmptyDomainID
					}
				}

				domain.Object.MinProps = max(domain.Object.MinProps, required+1)
				if domain.Object.MaxProps == nil || *domain.Object.MaxProps >= domain.Object.MinProps {
					id := use.domains.FindOrAddEquivalentDomain(domain)
					term := &generationTerm{domain: id, use: use, additional: &invalid}
					result = append(result, generationExpression{term: term})
				}
			}
		}
	}

	return result, nil
}

func anyOfGroups(use *schemaUse) [][]*schemaUse {
	if use == nil {
		return nil
	}

	groups := make([][]*schemaUse, 0)
	if len(use.anyOf) != 0 {
		groups = append(groups, use.anyOf)
	}

	for _, child := range use.allOf {
		groups = append(groups, anyOfGroups(child)...)
	}

	return groups
}

func containsAnyOfUse(use *schemaUse) bool {
	if use == nil {
		return false
	}

	if len(use.anyOf) != 0 {
		return true
	}

	for _, child := range use.allOf {
		if containsAnyOfUse(child) {
			return true
		}
	}

	if containsAnyOfUse(use.items) || containsAnyOfUse(use.additional) {
		return true
	}

	for _, property := range use.properties {
		if containsAnyOfUse(property.use) {
			return true
		}
	}

	return false
}

func generationExpressionEmpty(value generationExpression) bool {
	return value.term == nil && (value.choice == nil || len(value.choice.branches) == 0)
}
