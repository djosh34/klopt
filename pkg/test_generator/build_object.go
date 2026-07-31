//nolint:godoclint // Private object construction is covered by semantic tests.
package testgenerator

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

//nolint:cyclop,gocyclo,gocognit,maintidx // Object construction follows the required, fill, optional, additional order.
func buildObject(selected []demand, tape *tapeCursor) buildResult {
	if tape == nil {
		return failedBuild(errors.New("build object with nil tape cursor"))
	}

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

	domain, err := collectObjectDomain(selected)
	if err != nil {
		return failedBuild(err)
	}

	if domain.maximum != nil && (domain.maximum.Sign() < 0 || domain.minimum.Cmp(domain.maximum) > 0) {
		return missedBuild()
	}

	members := make([]jsonvalue.Member, 0)
	present := make(map[string]struct{})

	include := func(name string, child *expression) buildResult {
		if _, exists := present[name]; exists {
			return failedBuild(fmt.Errorf("object name %q selected twice", name))
		}

		if domain.maximum != nil && big.NewInt(int64(len(members))).Cmp(domain.maximum) >= 0 {
			return missedBuild()
		}

		built := buildPositiveChild(child, tape)
		if built.err != nil || built.state != buildComplete {
			return built
		}

		present[name] = struct{}{}
		members = append(members, jsonvalue.Member{Name: name, Value: built.value})

		return built
	}

	for _, name := range domain.knownNames {
		if domain.hasOmitRequired && name == domain.omitRequired {
			continue
		}

		if !domain.required[name] {
			continue
		}

		built := include(name, domain.properties[name])
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	for _, name := range domain.requiredNames {
		if domain.hasOmitRequired && name == domain.omitRequired {
			continue
		}

		if _, known := domain.properties[name]; known {
			continue
		}

		if !domain.additionalAllowed {
			return missedBuild()
		}

		built := include(name, domain.additionalChild)
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	for big.NewInt(int64(len(members))).Cmp(domain.minimum) < 0 {
		added := false

		for _, name := range domain.knownNames {
			if domain.hasOmitRequired && name == domain.omitRequired {
				continue
			}

			if _, exists := present[name]; exists {
				continue
			}

			built := include(name, domain.properties[name])
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

		built := include(name, domain.additionalChild)
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	for _, name := range domain.knownNames {
		if domain.hasOmitRequired && name == domain.omitRequired {
			continue
		}

		if _, exists := present[name]; exists {
			continue
		}

		if domain.maximum != nil && big.NewInt(int64(len(members))).Cmp(domain.maximum) >= 0 {
			break
		}

		if tape.takeByte() == 0 {
			continue
		}

		built := include(name, domain.properties[name])
		if built.err != nil || built.state != buildComplete {
			return built
		}
	}

	additionalIndex := 0

	for domain.maximum == nil || big.NewInt(int64(len(members))).Cmp(domain.maximum) < 0 {
		if tape.takeByte() == 0 {
			break
		}

		if !domain.additionalAllowed {
			return missedBuild()
		}

		name := nextAdditionalName(present, domain.knownNames, additionalIndex)
		additionalIndex++

		built := include(name, domain.additionalChild)
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

type objectDomain struct {
	minimum           *big.Int
	maximum           *big.Int
	required          map[string]bool
	requiredNames     []string
	omitRequired      string
	hasOmitRequired   bool
	knownNames        []string
	properties        map[string]*expression
	additionalChild   *expression
	additionalAllowed bool
}

//nolint:cyclop,gocognit // Object atom fields map directly to the object construction domain.
func collectObjectDomain(selected []demand) (objectDomain, error) {
	domain := objectDomain{
		minimum:           big.NewInt(0),
		required:          make(map[string]bool),
		properties:        make(map[string]*expression),
		additionalAllowed: true,
	}

	for index, selectedDemand := range selected {
		if selectedDemand.expression == nil || selectedDemand.expression.kind != expressionAtom {
			return objectDomain{}, fmt.Errorf("object demand %d is not an atom", index)
		}

		rule := selectedDemand.expression.atom
		switch rule.kind {
		case atomObjectMinProperties:
			count, err := countBigInt(rule.count, "minProperties")
			if err != nil {
				return objectDomain{}, err
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
				return objectDomain{}, err
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
				return objectDomain{}, errors.New("required atom has no names")
			}

			if selectedDemand.wantPass {
				for _, name := range rule.names {
					domain.required[name] = true
				}
			} else if !domain.hasOmitRequired {
				domain.omitRequired = rule.names[0]
				domain.hasOmitRequired = true
			}
		case atomObjectProperty:
			if rule.child == nil {
				return objectDomain{}, errors.New("object property atom has nil child")
			}

			if existing, exists := domain.properties[rule.name]; exists && existing != rule.child {
				return objectDomain{}, fmt.Errorf("object property %q has conflicting children", rule.name)
			}

			domain.properties[rule.name] = rule.child
		case atomObjectAdditional:
			if rule.hasChild && rule.child == nil {
				return objectDomain{}, errors.New("additional properties atom has nil child")
			}

			domain.additionalAllowed = rule.allowedAdditional
			domain.additionalChild = rule.child
		}
	}

	for name := range domain.required {
		domain.requiredNames = append(domain.requiredNames, name)
	}

	for name := range domain.properties {
		domain.knownNames = append(domain.knownNames, name)
	}

	sort.Strings(domain.requiredNames)
	sort.Strings(domain.knownNames)

	return domain, nil
}

func nextAdditionalName(present map[string]struct{}, known []string, start int) string {
	for index := start; ; index++ {
		name := fmt.Sprintf("additional%d", index)
		if _, exists := present[name]; exists {
			continue
		}

		if containsName(known, name) {
			continue
		}

		return name
	}
}
