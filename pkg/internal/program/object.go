//nolint:cyclop,gocognit,godoclint // Object theory translates signed atoms into dumb rules.
package program

import (
	"slices"

	"github.com/djosh34/klopt/pkg/jsonvalue"
)

type objectRules struct {
	cacheKey           string
	goals              []goal
	minimum            uint64
	maximum            uint64
	absent             map[string]struct{}
	forced             map[string]struct{}
	faults             map[string][]goal
	positiveAdditional []atom
	negativeAdditional []atom
	excluded           []jsonvalue.Value
}

func (program *Program) sampleObject(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
	depth uint64,
) (jsonvalue.Value, bool, error) {
	rules, possible, err := program.compileObjectRules(goals, excluded, reader, work)
	if err != nil || !possible {
		return jsonvalue.Value{}, possible, err
	}

	forced := sortedObjectNames(rules.forced)

	initial := objectState{remainingForced: forced, relevant: rules.excluded}
	if reader == nil {
		finishPossible, finishErr := program.objectCanFinish(initial, &rules, work)

		empty, objectErr := jsonvalue.Object(nil)
		if objectErr != nil {
			return jsonvalue.Value{}, false, objectErr
		}

		return empty, finishPossible, finishErr
	}

	members, err := program.walkObject(initial, &rules, reader, work, depth)
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

func (program *Program) compileObjectRules(
	goals []goal,
	excluded []jsonvalue.Value,
	reader *tapeReader,
	work *decodeWork,
) (objectRules, bool, error) {
	rules := objectRules{
		goals: goals, maximum: ^uint64(0),
		absent: make(map[string]struct{}), forced: make(map[string]struct{}),
		faults: make(map[string][]goal),
	}

	cacheKey, err := stateKey(canonicalStateWithExclusions(goals, excluded))
	if err != nil {
		return objectRules{}, false, err
	}

	rules.cacheKey = cacheKey

	for _, exact := range excluded {
		if exact.Kind == jsonvalue.KindObject {
			rules.excluded = append(rules.excluded, exact)
		}
	}

	for _, current := range goals {
		item := program.nodes[current.node].atom
		switch item.kind {
		case atomObjectMinProperties:
			if current.want {
				rules.minimum = max(rules.minimum, item.count)
			} else if item.count == 0 {
				return objectRules{}, false, nil
			} else {
				rules.maximum = min(rules.maximum, item.count-1)
			}
		case atomObjectMaxProperties:
			if current.want {
				rules.maximum = min(rules.maximum, item.count)
			} else if item.count == ^uint64(0) {
				return objectRules{}, false, nil
			} else {
				rules.minimum = max(rules.minimum, item.count+1)
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
			return objectRules{}, false, nil
		}
	}

	finiteNames, finite := rules.finiteNames()
	for _, failure := range rules.negativeAdditional {
		name, faults, possible, err := program.additionalFaultName(
			failure, &rules, finiteNames, finite, reader, work,
		)
		if err != nil || !possible {
			return objectRules{}, possible, err
		}

		rules.forced[name] = struct{}{}
		rules.faults[name] = append(rules.faults[name], faults...)
	}

	if uint64(len(rules.forced)) > rules.maximum || rules.minimum > rules.maximum {
		return objectRules{}, false, nil
	}

	return rules, true, nil
}

func sortedObjectNames(source map[string]struct{}) []string {
	result := make([]string, 0, len(source))
	for name := range source {
		result = append(result, name)
	}

	slices.SortFunc(result, compareShortlex)

	return result
}
