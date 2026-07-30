//nolint:cyclop,gocognit,godoclint,nestif // Private object rules keep schema obligations out of name sampling.
package program

import (
	"math/big"
	"slices"
)

func (rules *objectRules) finiteNames() ([]string, bool) {
	var result []string

	finite := false

	for _, additional := range rules.positiveAdditional {
		if additional.allowedAdditional {
			continue
		}

		if !finite {
			result = slices.Clone(additional.names)
			finite = true

			continue
		}

		filtered := result[:0]
		for _, name := range result {
			if slices.Contains(additional.names, name) {
				filtered = append(filtered, name)
			}
		}

		result = filtered
	}

	slices.Sort(result)

	return result, finite
}

func (program *Program) additionalFaultName(
	failure atom,
	rules *objectRules,
	finiteNames []string,
	finite bool,
	reader *tapeReader,
	work *decodeWork,
) (string, []goal, bool, error) {
	faults := make([]goal, 0, 1)
	if failure.hasChild {
		faults = append(faults, goal{node: failure.child, want: false})
	}

	existing := make([]string, 0, len(rules.forced))
	for name := range rules.forced {
		existing = append(existing, name)
	}

	slices.SortFunc(existing, compareShortlex)

	for _, name := range existing {
		if slices.Contains(failure.names, name) {
			continue
		}

		combined := appendCopy(rules.faults[name], faults...)

		possible, err := program.objectNamePossible(name, combined, rules, work)
		if err != nil {
			return "", nil, false, err
		}

		if possible {
			return name, faults, true, nil
		}
	}

	if finite {
		if len(finiteNames) == 0 {
			return "", nil, false, nil
		}

		start := 0
		if reader != nil {
			start = int(reader.word() % uint64(len(finiteNames)))
		}

		for offset := range finiteNames {
			name := finiteNames[(start+offset)%len(finiteNames)]
			if slices.Contains(failure.names, name) {
				continue
			}

			if _, absent := rules.absent[name]; absent {
				continue
			}

			possible, err := program.objectNamePossible(name, faults, rules, work)
			if err != nil {
				return "", nil, false, err
			}

			if possible {
				return name, faults, true, nil
			}
		}

		return "", nil, false, nil
	}

	rank, err := readNatural(reader, work)
	if err != nil {
		return "", nil, false, err
	}

	for {
		name, unrankErr := unrankName(rank, work)
		if unrankErr != nil {
			return "", nil, false, unrankErr
		}

		if !slices.Contains(failure.names, name) {
			if _, absent := rules.absent[name]; !absent {
				possible, possibleErr := program.objectNamePossible(name, faults, rules, work)
				if possibleErr != nil {
					return "", nil, false, possibleErr
				}

				if possible {
					return name, faults, true, nil
				}
			}
		}

		if err := work.solver(uint64(len(rank.Bytes())) + 1); err != nil {
			return "", nil, false, err
		}

		rank.Add(rank, big.NewInt(1))
	}
}

func (program *Program) objectNamePossible(
	name string,
	faults []goal,
	rules *objectRules,
	work *decodeWork,
) (bool, error) {
	nameGoals, allowed := program.objectNameGoals(name, faults, rules)
	if !allowed {
		return false, nil
	}

	return program.productive(canonicalState(nameGoals), work)
}

func (program *Program) objectNameGoals(
	name string,
	faults []goal,
	rules *objectRules,
) ([]goal, bool) {
	result := appendCopy(faults)

	for _, current := range rules.goals {
		item := program.nodes[current.node].atom
		if item.kind == atomObjectProperty && item.name == name {
			result = append(result, goal{node: item.child, want: current.want})
		}
	}

	for _, additional := range rules.positiveAdditional {
		if slices.Contains(additional.names, name) {
			continue
		}

		if !additional.allowedAdditional {
			return nil, false
		}

		if additional.hasChild {
			result = append(result, goal{node: additional.child, want: true})
		}
	}

	return result, true
}
