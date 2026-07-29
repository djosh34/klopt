//nolint:godoclint // Private expression algebra names are intentionally concise and local.
package suite

import (
	"errors"
	"fmt"
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

//nolint:cyclop // Choice distribution and term conjunction are the complete expression algebra.
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

//nolint:cyclop // Complement construction has distinct empty, unsupported, and composed outcomes.
func not(use *schemaUse) (generationExpression, error) {
	if use == nil || use.domains == nil {
		return generationExpression{}, errors.New("complement schema occurrence is nil")
	}

	planner := &CasePlanner{Domains: use.domains}

	cases, err := planner.planWithoutAnyOf(use)
	if err != nil {
		return generationExpression{}, err
	}

	for _, constraint := range planner.Constraints {
		if constraint.Outcome == ObligationUnconstructible {
			return generationExpression{}, fmt.Errorf(
				"cannot construct exact complement of %s at %s: %s",
				constraint.Source.Keyword, constraint.Source.Pointer, constraint.Reason,
			)
		}
	}

	values := make([]generationExpression, 0)

	for _, plannedCase := range cases {
		if plannedCase.Expect != ExpectRejected {
			continue
		}

		values = append(values, generationExpression{term: &generationTerm{
			domain: plannedCase.Values, use: use,
			stringLanguages: []*stringLanguageOccurrence{plannedCase.stringLanguage},
		}})
	}

	if containsAnyOfUse(use) {
		compositionFailure, err := invalidAnyOfExpression(use)
		if err != nil {
			return generationExpression{}, err
		}

		values = append(values, compositionFailure)
	}

	if len(values) == 0 && len(use.constraints) != 0 {
		return generationExpression{}, fmt.Errorf("cannot construct exact complement of schema at %s", use.pointer)
	}

	return choose(values...), nil
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
