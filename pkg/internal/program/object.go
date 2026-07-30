//nolint:cyclop,gocognit,gocyclo,godoclint,mnd,nestif // Object obligations stay private.
package program

import (
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type objectRules struct {
	goals              []goal
	absent             map[string]struct{}
	forced             map[string]struct{}
	faults             map[string][]goal
	positiveAdditional []atom
	negativeAdditional []atom
}

func (program *Program) sampleObject(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, bool, error) {
	minimum := uint64(0)
	maximum := ^uint64(0)
	rules := objectRules{
		goals: goals, absent: make(map[string]struct{}), forced: make(map[string]struct{}),
		faults: make(map[string][]goal),
	}

	for _, current := range goals {
		item := program.nodes[current.node].atom
		switch item.kind {
		case atomObjectMinProperties:
			if current.want {
				minimum = max(minimum, item.count)
			} else if item.count == 0 {
				return jsonvalue.Value{}, false, nil
			} else {
				maximum = min(maximum, item.count-1)
			}
		case atomObjectMaxProperties:
			if current.want {
				maximum = min(maximum, item.count)
			} else if item.count == ^uint64(0) {
				return jsonvalue.Value{}, false, nil
			} else {
				minimum = max(minimum, item.count+1)
			}
		case atomObjectRequired:
			if current.want {
				rules.forced[item.name] = struct{}{}
			} else {
				rules.absent[item.name] = struct{}{}
			}
		case atomObjectProperty:
			if !current.want {
				rules.forced[item.name] = struct{}{}
			}
		case atomObjectAdditional:
			if current.want {
				rules.positiveAdditional = append(rules.positiveAdditional, item)
			} else {
				rules.negativeAdditional = append(rules.negativeAdditional, item)
			}
		}
	}

	for name := range rules.forced {
		if _, forbidden := rules.absent[name]; forbidden {
			return jsonvalue.Value{}, false, nil
		}
	}

	finiteNames, finite := rules.finiteNames()
	for _, failure := range rules.negativeAdditional {
		name, faults, possible, err := program.additionalFaultName(
			failure, &rules, finiteNames, finite, reader, work,
		)
		if err != nil {
			return jsonvalue.Value{}, false, err
		}

		if !possible {
			return jsonvalue.Value{}, false, nil
		}

		rules.forced[name] = struct{}{}
		rules.faults[name] = append(rules.faults[name], faults...)
	}

	forcedNames := sortedObjectNames(rules.forced)
	for _, name := range forcedNames {
		possible, err := program.objectNamePossible(name, rules.faults[name], &rules, work)
		if err != nil {
			return jsonvalue.Value{}, false, err
		}

		if !possible {
			return jsonvalue.Value{}, false, nil
		}
	}

	minimum = max(minimum, uint64(len(rules.forced)))
	if minimum > maximum {
		return jsonvalue.Value{}, false, nil
	}

	available := make([]string, 0)

	if finite {
		for _, name := range finiteNames {
			if _, present := rules.forced[name]; present {
				continue
			}

			if _, absent := rules.absent[name]; absent {
				continue
			}

			possible, err := program.objectNamePossible(name, nil, &rules, work)
			if err != nil {
				return jsonvalue.Value{}, false, err
			}

			if possible {
				available = append(available, name)
			}
		}

		maximum = min(maximum, uint64(len(rules.forced)+len(available)))
		if minimum > maximum {
			return jsonvalue.Value{}, false, nil
		}
	}

	names, possible, err := program.chooseProductiveObjectNames(
		minimum,
		maximum,
		forcedNames,
		available,
		finite,
		&rules,
		excluded,
		reader,
		work,
	)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	if !possible {
		return jsonvalue.Value{}, false, nil
	}

	count := uint64(len(names))
	if count+2 > work.limits.MaxOutputBytes {
		return jsonvalue.Value{}, false, &LimitError{
			Resource: "object properties", Limit: work.limits.MaxOutputBytes, Observed: count + 2,
		}
	}

	members, err := program.decodeObjectMembers(names, &rules, excluded, reader, work, depth)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	value, err := jsonvalue.Object(members)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	matches, err := program.valueAllowed(goals, excluded, value)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}

	return value, matches, nil
}

func sortedObjectNames(source map[string]struct{}) []string {
	result := make([]string, 0, len(source))
	for name := range source {
		result = append(result, name)
	}

	slices.SortFunc(result, compareShortlex)

	return result
}
