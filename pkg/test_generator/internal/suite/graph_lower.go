//nolint:godoclint // Graph lowering stays behind CompileSuite.
package suite

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"sort"

	"github.com/djosh34/klopt/pkg/internal/program"        //nolint:depguard // S3 lowers exact sets into the shared program module.
	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Empty products are exact.
	"github.com/djosh34/klopt/pkg/jsonvalue"
)

var errEmptyConstructiveSet = errors.New("exact obligation has no constructive value")

type graphCompiler struct {
	arena   *SetArena
	finder  *valueFinder
	builder program.Builder
	source  ConstraintSource
	limits  WorkLimits
	memo    map[SetRef]program.NodeID
	nodes   uint64
	edges   uint64
	proofs  uint64
	bytes   uint64
	classes uint64
}

func compileCaseProgram(
	arena *SetArena,
	values SetRef,
	source ConstraintSource,
	limits WorkLimits,
) (program.Program, error) {
	compiler := graphCompiler{
		arena: arena, finder: newValueFinder(arena), source: source, limits: limits,
		memo: make(map[SetRef]program.NodeID),
	}

	root, err := compiler.compileSet(values)
	if err != nil {
		return program.Program{}, err
	}

	table, err := compiler.builder.SemanticSampling(root)
	if err != nil {
		return program.Program{}, err
	}

	sealed, err := compiler.builder.Seal(root, table)
	if err != nil {
		return program.Program{}, err
	}

	return sealed, nil
}

//nolint:cyclop // Each JSON kind lowers through its exact atom theory without distributing the set tree.
func (compiler *graphCompiler) compileSet(ref SetRef) (program.NodeID, error) {
	if root, ok := compiler.memo[ref]; ok {
		return root, nil
	}

	root, err := compiler.addNode()
	if err != nil {
		return 0, err
	}

	compiler.memo[ref] = root
	added := false

	for kind := jsonvalue.KindNull; kind <= jsonvalue.KindObject; kind++ {
		assignments := compiler.finder.solve(ref, kind, map[AtomID]bool{}, int(^uint(0)>>1))
		for _, assignment := range assignments {
			if err := compiler.chargeProof(); err != nil {
				return 0, err
			}

			productive, productivityErr := compiler.arena.assignmentProductive(kind, assignment)
			if productivityErr != nil {
				return 0, productivityErr
			}

			if !productive {
				continue
			}

			assignmentAdded, appendErr := compiler.appendAssignment(root, ref, kind, assignment)
			if appendErr != nil {
				return 0, appendErr
			}

			added = added || assignmentAdded
		}
	}

	if !added {
		return 0, errEmptyConstructiveSet
	}

	return root, nil
}

//nolint:cyclop,gocognit // The closed kind vocabulary chooses one structural lowering action.
func (compiler *graphCompiler) appendAssignment(
	root program.NodeID,
	ref SetRef,
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
) (bool, error) {
	if enumeration, found := compiler.positiveEnumeration(kind, assignment); found {
		return compiler.appendExactCandidates(root, ref, assignment, enumeration)
	}

	switch kind {
	case jsonvalue.KindNull:
		return compiler.appendExactCandidates(
			root, ref, assignment, []jsonvalue.Value{jsonvalue.Null()},
		)
	case jsonvalue.KindBoolean:
		return compiler.appendExactCandidates(root, ref, assignment, []jsonvalue.Value{
			jsonvalue.Bool(false), jsonvalue.Bool(true),
		})
	case jsonvalue.KindNumber:
		if intervals, exact := compiler.integerIntervals(assignment); exact && len(intervals) != 0 {
			for _, interval := range intervals {
				if err := compiler.addIntegerRange(root, interval.minimum, interval.maximum); err != nil {
					return false, err
				}
			}

			return true, nil
		}
	case jsonvalue.KindString:
		set, err := compiler.finder.combinedStringSet(assignment)
		if err != nil {
			var empty *stringlanguage.EmptyError
			if errors.As(err, &empty) {
				return false, nil
			}

			return false, err
		}

		metrics, err := set.ProgramMetrics()
		if err != nil {
			return false, err
		}

		if reserveErr := compiler.reserveLanguage(metrics); reserveErr != nil {
			return false, reserveErr
		}

		languageRoot, err := set.AppendTo(&compiler.builder)
		if err != nil {
			return false, err
		}

		if err := compiler.addBeginString(root, languageRoot); err != nil {
			return false, err
		}

		return true, nil
	case jsonvalue.KindArray:
		added, exact, err := compiler.appendArrayAssignment(root, assignment)
		if err != nil || exact {
			return added, err
		}
	case jsonvalue.KindObject:
		added, exact, err := compiler.appendObjectAssignment(root, assignment)
		if err != nil || exact {
			return added, err
		}
	default:
		return false, fmt.Errorf("unknown JSON kind %d", kind)
	}

	candidates, err := compiler.finder.assignmentValues(kind, assignment)
	if err != nil {
		return false, err
	}

	if len(candidates) == 0 {
		return false, nil
	}

	slices.SortFunc(candidates, compareCandidateValues)

	return compiler.appendExactCandidates(root, ref, assignment, candidates[:1])
}

// Signed object facts become one canonical name residual with dynamic child calls.
//
//nolint:cyclop,gocognit,gocyclo // Atom variants are normalized together.
func (compiler *graphCompiler) appendObjectAssignment(
	root program.NodeID,
	assignment map[AtomID]bool,
) (bool, bool, error) {
	required := make(map[string]struct{})
	forbidden := make(map[string]struct{})
	known := make(map[string]struct{})
	constraints := make(map[string][]SetRef)
	allowedNames := []string(nil)
	hasAllowed := false

	additionalRules, complete := appendObjectValueRules(compiler.arena, assignment)
	if !complete {
		return false, false, nil
	}

	for identifier, want := range assignment {
		switch value := compiler.arena.Atoms[identifier].(type) {
		case enumAtom:
			if !want {
				for _, excluded := range value.Values {
					if excluded.Kind == jsonvalue.KindObject {
						return false, false, nil
					}
				}
			}
		case requiredPropertyAtom:
			known[value.Name] = struct{}{}
			if want {
				required[value.Name] = struct{}{}
			} else {
				forbidden[value.Name] = struct{}{}
			}
		case allowedPropertyNamesAtom:
			for _, name := range value.Names {
				known[name] = struct{}{}
			}

			if !want {
				return false, false, nil
			}

			if !hasAllowed {
				allowedNames = append([]string(nil), value.Names...)
				hasAllowed = true
			} else {
				allowedNames = intersectNames(allowedNames, value.Names)
			}
		case propertyValuesAtom:
			known[value.Name] = struct{}{}
			if want {
				constraints[value.Name] = append(constraints[value.Name], value.Values)
			} else {
				required[value.Name] = struct{}{}
				constraints[value.Name] = append(
					constraints[value.Name], Complement(value.Values),
				)
			}
		case additionalPropertyValuesAtom:
			for _, name := range value.Names {
				known[name] = struct{}{}
			}
		case additionalSomePropertyAtom:
			for _, name := range value.Names {
				known[name] = struct{}{}
			}
		}
	}

	for name := range required {
		if _, absent := forbidden[name]; absent || hasAllowed && !slices.Contains(allowedNames, name) {
			return false, true, nil
		}
	}

	lower, upper, excluded, exact := collectionCountFacts(compiler.arena, assignment, false)
	if !exact {
		return false, false, nil
	}

	lower = maximumBigInt(lower, big.NewInt(int64(len(required))))
	if hasAllowed {
		upper = minimumBigInt(upper, big.NewInt(int64(len(allowedNames))))
	}

	count, ok := firstCount(lower, upper, excluded)
	if !ok {
		return false, true, nil
	}

	if !count.IsInt64() || count.Sign() < 0 {
		return false, false, nil
	}

	wantedCount := int(count.Int64())

	selected := slices.Sorted(maps.Keys(required))
	for _, name := range slices.Sorted(maps.Keys(known)) {
		if len(selected) >= wantedCount {
			break
		}

		if _, absent := forbidden[name]; absent || slices.Contains(selected, name) ||
			hasAllowed && !slices.Contains(allowedNames, name) {
			continue
		}

		selected = append(selected, name)
	}

	for len(selected) < wantedCount && !hasAllowed {
		name := nextAdditionalName(known)
		known[name] = struct{}{}
		selected = append(selected, name)
	}

	if len(selected) != wantedCount {
		return false, true, nil
	}

	sort.Strings(selected)

	body, err := compiler.addNode()
	if err != nil {
		return false, false, err
	}

	if err := compiler.addBeginObject(root, body); err != nil {
		return false, false, err
	}

	for _, name := range selected {
		refs := objectValueConstraints(constraints[name], name, additionalRules)

		values, err := compiler.arena.Intersect(refs...)
		if err != nil {
			return false, false, err
		}

		child, err := compiler.compileSet(values)
		if err != nil {
			if errors.Is(err, errEmptyConstructiveSet) {
				return false, true, nil
			}

			return false, false, err
		}

		next, err := compiler.addNode()
		if err != nil {
			return false, false, err
		}

		if err := compiler.addObjectMember(body, name, child, next); err != nil {
			return false, false, err
		}

		body = next
	}

	if err := compiler.addStop(body); err != nil {
		return false, false, err
	}

	return true, true, nil
}

func firstCount(
	lower *big.Int,
	upper *big.Int,
	excluded []countInterval,
) (*big.Int, bool) {
	intervals := subtractCountIntervals([]countInterval{{minimum: lower, maximum: upper}}, excluded)
	if len(intervals) == 0 {
		return nil, false
	}

	result := intervals[0].minimum
	if result == nil || result.Sign() < 0 {
		result = big.NewInt(0)
	}

	if intervals[0].maximum != nil && result.Cmp(intervals[0].maximum) > 0 {
		return nil, false
	}

	return result, true
}

//nolint:cyclop,gocognit // Signed all-item and count facts are normalized in one pass.
func (compiler *graphCompiler) appendArrayAssignment(
	root program.NodeID,
	assignment map[AtomID]bool,
) (bool, bool, error) {
	for identifier, want := range assignment {
		if enumeration, ok := compiler.arena.Atoms[identifier].(enumAtom); ok && !want {
			for _, excluded := range enumeration.Values {
				if excluded.Kind == jsonvalue.KindArray {
					return false, false, nil
				}
			}
		}
	}

	lower, upper, excluded, exact := collectionCountFacts(compiler.arena, assignment, true)
	if !exact {
		return false, false, nil
	}

	allowed := compiler.arena.True()
	hasSomeRequirement := false

	for identifier, want := range assignment {
		switch value := compiler.arena.Atoms[identifier].(type) {
		case arrayItemsAtom:
			if !want {
				return false, false, nil
			}

			var err error

			allowed, err = compiler.arena.Intersect(allowed, value.Values)
			if err != nil {
				return false, false, err
			}
		case arraySomeItemsAtom:
			if want {
				hasSomeRequirement = true

				continue
			}

			var err error

			allowed, err = compiler.arena.Intersect(allowed, Complement(value.Values))
			if err != nil {
				return false, false, err
			}
		}
	}

	if hasSomeRequirement {
		return false, false, nil
	}

	intervals := subtractCountIntervals([]countInterval{{minimum: lower, maximum: upper}}, excluded)
	if len(intervals) == 0 {
		return false, true, nil
	}

	emptyItems, err := compiler.arena.IsEmpty(allowed)
	if err != nil {
		return false, false, err
	}

	added := false

	for _, interval := range intervals {
		minimum := maximumBigInt(interval.minimum, big.NewInt(0))

		maximum := interval.maximum
		if maximum != nil && minimum.Cmp(maximum) > 0 {
			continue
		}

		if minimum.Sign() == 0 {
			if err := compiler.addExactValue(root, jsonvalue.Array(nil)); err != nil {
				return false, false, err
			}

			added = true
			minimum = big.NewInt(1)
		}

		if maximum != nil && minimum.Cmp(maximum) > 0 {
			continue
		}

		if emptyItems {
			continue
		}

		child, err := compiler.compileSet(allowed)
		if err != nil {
			return false, false, err
		}

		if err := compiler.addArraySequence(root, child, minimum, maximum); err != nil {
			return false, false, err
		}

		added = true
	}

	return added, true, nil
}

//nolint:cyclop // Each signed numeric atom narrows or subtracts one interval.
func (compiler *graphCompiler) integerIntervals(
	assignment map[AtomID]bool,
) ([]countInterval, bool) {
	intervals := []countInterval{{}}

	for identifier, want := range assignment {
		switch value := compiler.arena.Atoms[identifier].(type) {
		case integerAtom:
			if !want {
				return nil, false
			}
		case numberRangeAtom:
			allowed := integerRange(value)
			if want {
				intervals = intersectCountIntervals(intervals, allowed)
			} else {
				intervals = subtractCountIntervals(intervals, []countInterval{allowed})
			}
		case enumAtom:
			if want {
				return nil, false
			}

			for _, excluded := range value.Values {
				if excluded.Kind != jsonvalue.KindNumber || !excluded.Number.IsInteger() ||
					excluded.Number.Rational == nil {
					continue
				}

				integer := new(big.Int).Set(excluded.Number.Rational.Num())
				intervals = subtractCountIntervals(intervals, []countInterval{{
					minimum: integer, maximum: integer,
				}})
			}
		case multipleOfAtom, floatFormatAtom:
			return nil, false
		}
	}

	return intervals, true
}

func integerRange(value numberRangeAtom) countInterval {
	var minimum *big.Int
	if value.Minimum != nil && value.Minimum.Rational != nil {
		minimum = ceilRat(value.Minimum.Rational)
		if value.ExclusiveMinimum && value.Minimum.Rational.IsInt() {
			minimum.Add(minimum, big.NewInt(1))
		}
	}

	var maximum *big.Int
	if value.Maximum != nil && value.Maximum.Rational != nil {
		maximum = floorRat(value.Maximum.Rational)
		if value.ExclusiveMaximum && value.Maximum.Rational.IsInt() {
			maximum.Sub(maximum, big.NewInt(1))
		}
	}

	return countInterval{minimum: minimum, maximum: maximum}
}

func intersectCountIntervals(left []countInterval, right countInterval) []countInterval {
	result := make([]countInterval, 0, len(left))
	for _, item := range left {
		minimum := maximumBigInt(item.minimum, right.minimum)

		maximum := minimumBigInt(item.maximum, right.maximum)
		if minimum == nil || maximum == nil || minimum.Cmp(maximum) <= 0 {
			result = append(result, countInterval{minimum: minimum, maximum: maximum})
		}
	}

	return result
}

func subtractCountIntervals(
	source []countInterval,
	excluded []countInterval,
) []countInterval {
	result := append([]countInterval(nil), source...)
	for _, removed := range excluded {
		next := make([]countInterval, 0, len(result)+1)
		for _, item := range result {
			overlap := intersectCountIntervals([]countInterval{item}, removed)
			if len(overlap) == 0 {
				next = append(next, item)

				continue
			}

			if removed.minimum != nil && (item.minimum == nil || item.minimum.Cmp(removed.minimum) < 0) {
				maximum := new(big.Int).Sub(removed.minimum, big.NewInt(1))
				next = append(next, countInterval{minimum: cloneInteger(item.minimum), maximum: maximum})
			}

			if removed.maximum != nil && (item.maximum == nil || item.maximum.Cmp(removed.maximum) > 0) {
				minimum := new(big.Int).Add(removed.maximum, big.NewInt(1))
				next = append(next, countInterval{minimum: minimum, maximum: cloneInteger(item.maximum)})
			}
		}

		result = next
	}

	return result
}

func cloneInteger(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}

	return new(big.Int).Set(value)
}

func (compiler *graphCompiler) positiveEnumeration(
	kind jsonvalue.Kind,
	assignment map[AtomID]bool,
) ([]jsonvalue.Value, bool) {
	var result []jsonvalue.Value

	found := false

	for identifier, want := range assignment {
		enumeration, ok := compiler.arena.Atoms[identifier].(enumAtom)
		if !ok || !want {
			continue
		}

		candidates := make([]jsonvalue.Value, 0)

		for _, candidate := range enumeration.Values {
			if candidate.Kind == kind {
				candidates = append(candidates, candidate)
			}
		}

		if !found {
			result = candidates
			found = true

			continue
		}

		filtered := result[:0]
		for _, candidate := range result {
			matches, err := enumeration.matches(compiler.arena, candidate)
			if err == nil && matches {
				filtered = append(filtered, candidate)
			}
		}

		result = filtered
	}

	return result, found
}

func (compiler *graphCompiler) appendExactCandidates(
	root program.NodeID,
	ref SetRef,
	assignment map[AtomID]bool,
	candidates []jsonvalue.Value,
) (bool, error) {
	added := false

	for _, candidate := range candidates {
		matchesAssignment, err := compiler.arena.candidateMatchesAssignment(candidate, assignment)
		if err != nil {
			return false, err
		}

		matchesSet, err := compiler.arena.Contains(ref, candidate)
		if err != nil {
			return false, err
		}

		if !matchesAssignment || !matchesSet {
			continue
		}

		if err := compiler.addExactValue(root, candidate); err != nil {
			return false, err
		}

		added = true
	}

	return added, nil
}

func (compiler *graphCompiler) addNode() (program.NodeID, error) {
	compiler.nodes++
	if err := compiler.charge("nodes", compiler.limits.GraphNodes, compiler.nodes); err != nil {
		return 0, err
	}

	return compiler.builder.AddNode(), nil
}

func (compiler *graphCompiler) reserveLanguage(metrics stringlanguage.LoweringMetrics) error {
	if metrics.Nodes > ^uint64(0)-compiler.nodes {
		return compiler.charge("nodes", compiler.limits.GraphNodes, ^uint64(0))
	}

	compiler.nodes += metrics.Nodes
	if err := compiler.charge("nodes", compiler.limits.GraphNodes, compiler.nodes); err != nil {
		return err
	}

	if metrics.Transitions > ^uint64(0)-compiler.edges {
		return compiler.charge("edges", compiler.limits.GraphEdges, ^uint64(0))
	}

	compiler.edges += metrics.Transitions
	if err := compiler.charge("edges", compiler.limits.GraphEdges, compiler.edges); err != nil {
		return err
	}

	if metrics.UnicodeClasses > ^uint64(0)-compiler.classes {
		return compiler.charge("Unicode classes", compiler.limits.UnicodeClasses, ^uint64(0))
	}

	compiler.classes += metrics.UnicodeClasses
	if err := compiler.charge(
		"Unicode classes", compiler.limits.UnicodeClasses, compiler.classes,
	); err != nil {
		return err
	}

	if metrics.Bytes > ^uint64(0)-compiler.bytes {
		return compiler.charge("transition bytes", compiler.limits.TransitionBytes, ^uint64(0))
	}

	compiler.bytes += metrics.Bytes
	if err := compiler.charge(
		"transition bytes", compiler.limits.TransitionBytes, compiler.bytes,
	); err != nil {
		return err
	}

	programBytes := new(big.Int).SetUint64(compiler.nodes)
	programBytes.Add(programBytes, new(big.Int).SetUint64(compiler.edges))
	programBytes.Add(programBytes, new(big.Int).SetUint64(compiler.bytes))

	if !programBytes.IsUint64() {
		return compiler.charge("program bytes", compiler.limits.ProgramBytes, ^uint64(0))
	}

	return compiler.charge("program bytes", compiler.limits.ProgramBytes, programBytes.Uint64())
}

func (compiler *graphCompiler) addExactValue(root program.NodeID, value jsonvalue.Value) error {
	encoded, err := value.MarshalJSON()
	if err != nil {
		return err
	}

	if err := compiler.addEdge(uint64(len(encoded))); err != nil {
		return err
	}

	return compiler.builder.AddExactValue(root, value)
}

func (compiler *graphCompiler) addIntegerRange(
	root program.NodeID,
	minimum *big.Int,
	maximum *big.Int,
) error {
	if err := compiler.addEdge(bigIntBytes(minimum) + bigIntBytes(maximum)); err != nil {
		return err
	}

	return compiler.builder.AddIntegerRange(root, minimum, maximum)
}

func (compiler *graphCompiler) addBeginString(root program.NodeID, next program.NodeID) error {
	if err := compiler.addEdge(1); err != nil {
		return err
	}

	return compiler.builder.AddBeginString(root, next)
}

func (compiler *graphCompiler) addBeginObject(root program.NodeID, next program.NodeID) error {
	if err := compiler.addEdge(1); err != nil {
		return err
	}

	return compiler.builder.AddBeginObject(root, next)
}

func (compiler *graphCompiler) addObjectMember(
	root program.NodeID,
	name string,
	child program.NodeID,
	resume program.NodeID,
) error {
	if err := compiler.addEdge(uint64(len(name)) + 1); err != nil {
		return err
	}

	return compiler.builder.AddObjectMember(root, name, child, resume)
}

func (compiler *graphCompiler) addStop(root program.NodeID) error {
	if err := compiler.addEdge(1); err != nil {
		return err
	}

	return compiler.builder.AddStop(root)
}

func (compiler *graphCompiler) addArraySequence(
	root program.NodeID,
	child program.NodeID,
	minimum *big.Int,
	maximum *big.Int,
) error {
	if err := compiler.addEdge(bigIntBytes(minimum) + bigIntBytes(maximum)); err != nil {
		return err
	}

	return compiler.builder.AddArraySequence(root, child, minimum, maximum)
}

func (compiler *graphCompiler) addEdge(bytes uint64) error {
	compiler.edges++
	if err := compiler.charge("edges", compiler.limits.GraphEdges, compiler.edges); err != nil {
		return err
	}

	if bytes > ^uint64(0)-compiler.bytes {
		return compiler.charge("transition bytes", compiler.limits.TransitionBytes, ^uint64(0))
	}

	compiler.bytes += bytes
	if err := compiler.charge(
		"transition bytes", compiler.limits.TransitionBytes, compiler.bytes,
	); err != nil {
		return err
	}

	return compiler.charge(
		"program bytes", compiler.limits.ProgramBytes, compiler.nodes+compiler.edges+compiler.bytes,
	)
}

func (compiler *graphCompiler) chargeProof() error {
	compiler.proofs++

	return compiler.charge("proof steps", compiler.limits.ProofSteps, compiler.proofs)
}

func (compiler *graphCompiler) charge(resource string, limit uint64, observed uint64) error {
	return chargeWork("graph", resource, compiler.source.Pointer, limit, observed)
}

func bigIntBytes(value *big.Int) uint64 {
	if value == nil {
		return 1
	}

	return uint64(len(value.Bytes())) + 1
}

func compareCandidateValues(left jsonvalue.Value, right jsonvalue.Value) int {
	leftJSON, leftErr := left.MarshalJSON()

	rightJSON, rightErr := right.MarshalJSON()
	if leftErr != nil || rightErr != nil {
		panic("candidate values were validated before sorting")
	}

	if len(leftJSON) != len(rightJSON) {
		return len(leftJSON) - len(rightJSON)
	}

	return bytes.Compare(leftJSON, rightJSON)
}

func chargeWork(pass string, resource string, pointer string, limit uint64, observed uint64) error {
	if limit == 0 || observed <= limit {
		return nil
	}

	return &ResourceError{
		Pass: pass, Resource: resource, Pointer: pointer, Limit: limit, Observed: observed,
	}
}
