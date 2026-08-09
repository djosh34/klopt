package schematest

import (
	"errors"
	"iter"
	"math/big"
)

// maximumProjectionOrdinal is the saturated endpoint of the uint64 rank frontier.
const maximumProjectionOrdinal = ^uint64(0)

// rowProjectionView is one ephemeral same-instance conjunction. Sources stay in
// authored traversal order and include local siblings, allOf branches, and only
// the true branches of each anyOf mask.
type rowProjectionView struct {
	sources []rowSchemaSource
}

// eachSource visits the active schemas without exposing retained projection state.
func (view rowProjectionView) eachSource(yield func(rowSchemaSource) bool) {
	for _, source := range view.sources {
		if !yield(source) {
			return
		}
	}
}

// appendBranchRequirements projects every active anyOf mask back into declarative constraints.
func (view rowProjectionView) appendBranchRequirements(
	requirements []requirement,
	assign func() error,
) ([]requirement, error) {
	for _, source := range view.sources {
		for branch, child := range source.node.anyOf {
			childOccurrence := rebasePlanOccurrence(
				child,
				source.occurrence,
				source.occurrence.usePointer+"/anyOf/"+itoa(branch),
				source.occurrence.instanceTemplate,
			)
			truth := view.hasSourceOccurrence(childOccurrence)

			pinnedTruth, pinned, err := rowProjectionBranchPin(requirements, source.occurrence, branch)
			if err != nil {
				return nil, err
			}

			if pinned && pinnedTruth != truth {
				return nil, errors.New("schematest: projection violates composition branch requirement")
			}

			if !pinned {
				if err := assign(); err != nil {
					return nil, err
				}

				requirements = append(requirements, requirement{
					tag:         requirementBranchTruth,
					occurrence:  childOccurrence,
					composition: "anyOf",
					branch:      branch,
					truth:       truth,
					hasBranch:   true,
				})
			}
		}
	}

	return requirements, nil
}

// hasSourceOccurrence reports whether one exact composed source is active.
func (view rowProjectionView) hasSourceOccurrence(wanted schemaOccurrence) bool {
	for _, source := range view.sources {
		if ruleOccurrenceMatches(source.occurrence, wanted) {
			return true
		}
	}

	return false
}

// eachDirectValue exposes complete authored enum/default witnesses from active sources.
func (view rowProjectionView) eachDirectValue(yield func(rowSchemaSource, *jsonValue) bool) error {
	for _, source := range view.sources {
		if source.node == nil || source.node.schemaShape == nil {
			return errors.New("schematest: projection source has no shape")
		}

		if source.node.enum != nil {
			for _, member := range source.node.enum {
				if member.value == nil {
					return errors.New("schematest: nil projected enum value")
				}

				if !yield(source, member.value) {
					return nil
				}
			}

			continue
		}

		if source.node.defaultValue != nil && !yield(source, source.node.defaultValue) {
			return nil
		}
	}

	return nil
}

// rowProjectionAt decodes one numeric-mask projection without visiting earlier projections.
func rowProjectionAt(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	wanted uint64,
) (rowProjectionView, bool, error) {
	count, err := rowProjectionNodeCount(node, occurrence, requirements)
	if err != nil || wanted >= count {
		return rowProjectionView{}, false, err
	}

	sources, err := rowProjectionDecodeNode(node, occurrence, requirements, wanted, nil)
	if err != nil {
		return rowProjectionView{}, false, err
	}

	return rowProjectionView{sources: sources}, true, nil
}

// rowProjectionNodeCount returns the saturated number of views rooted at one source.
func rowProjectionNodeCount(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) (uint64, error) {
	if node == nil || node.schemaShape == nil {
		return 0, errors.New("schematest: projected row schema has no shape")
	}

	count := uint64(1)

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childCount, err := rowProjectionNodeCount(child, childOccurrence, requirements)
		if err != nil {
			return 0, err
		}

		count = saturatedProjectionProduct(count, childCount)
	}

	anyCount, err := rowProjectionAnyOfCount(node, occurrence, requirements, len(node.anyOf)-1, false)
	if err != nil {
		return 0, err
	}

	return saturatedProjectionProduct(count, anyCount), nil
}

// rowProjectionAnyOfCount counts weighted lower-bit assignments without enumerating masks.
//
//nolint:cyclop // Pinned and unpinned weighted bits are decoded in one pass.
func rowProjectionAnyOfCount(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	last int,
	selected bool,
) (uint64, error) {
	if len(node.anyOf) == 0 {
		return 1, nil
	}

	count := uint64(1)
	fixedTrue := selected

	for branch := 0; branch <= last; branch++ {
		truth, pinned, err := rowProjectionBranchPin(requirements, occurrence, branch)
		if err != nil {
			return 0, err
		}

		childCount := uint64(0)

		if !pinned || truth {
			child := node.anyOf[branch]
			childOccurrence := rebasePlanOccurrence(
				child,
				occurrence,
				occurrence.usePointer+"/anyOf/"+itoa(branch),
				occurrence.instanceTemplate,
			)

			childCount, err = rowProjectionNodeCount(child, childOccurrence, requirements)
			if err != nil {
				return 0, err
			}
		}

		switch {
		case pinned && truth:
			fixedTrue = true
			count = saturatedProjectionProduct(count, childCount)
		case !pinned:
			count = saturatedProjectionProduct(count, saturatedProjectionSum(1, childCount))
		}
	}

	if !fixedTrue && count > 0 {
		count--
	}

	return count, nil
}

// rowProjectionDecodeNode decodes mixed-radix allOf and anyOf choices into one transient source list.
//
//nolint:cyclop,gocognit // Mixed-radix conjunction and numeric mask decoding form one recursive operation.
func rowProjectionDecodeNode(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	wanted uint64,
	sources []rowSchemaSource,
) ([]rowSchemaSource, error) {
	sources = append(sources, rowSchemaSource{node: node, occurrence: occurrence})

	for index, child := range node.allOf {
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		suffix, err := rowProjectionNodeSuffixCount(node, occurrence, requirements, index+1)
		if err != nil {
			return nil, err
		}

		childRank := wanted / suffix
		wanted %= suffix

		sources, err = rowProjectionDecodeNode(child, childOccurrence, requirements, childRank, sources)
		if err != nil {
			return nil, err
		}
	}

	if len(node.anyOf) == 0 {
		return sources, nil
	}

	mask := new(big.Int)
	highWeight := uint64(1)
	selected := false

	for branch := len(node.anyOf) - 1; branch >= 0; branch-- {
		truth, pinned, err := rowProjectionBranchPin(requirements, occurrence, branch)
		if err != nil {
			return nil, err
		}

		if pinned {
			if truth {
				childCount, countErr := rowProjectionBranchCount(node, occurrence, requirements, branch)
				if countErr != nil {
					return nil, countErr
				}

				mask.SetBit(mask, branch, 1)

				selected = true
				highWeight = saturatedProjectionProduct(highWeight, childCount)
			}

			continue
		}

		lowerCount, countErr := rowProjectionAnyOfCount(
			node, occurrence, requirements, branch-1, selected,
		)
		if countErr != nil {
			return nil, countErr
		}

		zeroBlock := saturatedProjectionProduct(highWeight, lowerCount)
		if wanted < zeroBlock {
			continue
		}

		wanted -= zeroBlock

		childCount, countErr := rowProjectionBranchCount(node, occurrence, requirements, branch)
		if countErr != nil {
			return nil, countErr
		}

		mask.SetBit(mask, branch, 1)

		selected = true
		highWeight = saturatedProjectionProduct(highWeight, childCount)
	}

	for branch, child := range node.anyOf {
		if mask.Bit(branch) == 0 {
			continue
		}

		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/anyOf/"+itoa(branch),
			occurrence.instanceTemplate,
		)

		suffix, err := rowProjectionSelectedBranchSuffixCount(
			node, occurrence, requirements, mask, branch+1,
		)
		if err != nil {
			return nil, err
		}

		childRank := wanted / suffix
		wanted %= suffix

		sources, err = rowProjectionDecodeNode(child, childOccurrence, requirements, childRank, sources)
		if err != nil {
			return nil, err
		}
	}

	return sources, nil
}

// rowProjectionNodeSuffixCount counts the dimensions following one allOf branch.
func rowProjectionNodeSuffixCount(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	first int,
) (uint64, error) {
	count, err := rowProjectionAnyOfCount(node, occurrence, requirements, len(node.anyOf)-1, false)
	if err != nil {
		return 0, err
	}

	for index := first; index < len(node.allOf); index++ {
		child := node.allOf[index]
		childOccurrence := rebasePlanOccurrence(
			child,
			occurrence,
			occurrence.usePointer+"/allOf/"+itoa(index),
			occurrence.instanceTemplate,
		)

		childCount, countErr := rowProjectionNodeCount(child, childOccurrence, requirements)
		if countErr != nil {
			return 0, countErr
		}

		count = saturatedProjectionProduct(count, childCount)
	}

	return count, nil
}

// rowProjectionBranchCount returns one anyOf branch's saturated view count.
func rowProjectionBranchCount(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	branch int,
) (uint64, error) {
	child := node.anyOf[branch]
	childOccurrence := rebasePlanOccurrence(
		child,
		occurrence,
		occurrence.usePointer+"/anyOf/"+itoa(branch),
		occurrence.instanceTemplate,
	)

	return rowProjectionNodeCount(child, childOccurrence, requirements)
}

// rowProjectionSelectedBranchSuffixCount counts selected branches after one numeric mask bit.
func rowProjectionSelectedBranchSuffixCount(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
	mask *big.Int,
	first int,
) (uint64, error) {
	count := uint64(1)

	for branch := first; branch < len(node.anyOf); branch++ {
		if mask.Bit(branch) == 0 {
			continue
		}

		childCount, err := rowProjectionBranchCount(node, occurrence, requirements, branch)
		if err != nil {
			return 0, err
		}

		count = saturatedProjectionProduct(count, childCount)
	}

	return count, nil
}

// saturatedProjectionProduct multiplies projection counts without ordinal wraparound.
func saturatedProjectionProduct(left uint64, right uint64) uint64 {
	if left == 0 || right == 0 {
		return 0
	}

	if left > maximumProjectionOrdinal/right {
		return maximumProjectionOrdinal
	}

	return left * right
}

// saturatedProjectionSum adds projection counts without ordinal wraparound.
func saturatedProjectionSum(left uint64, right uint64) uint64 {
	if maximumProjectionOrdinal-left < right {
		return maximumProjectionOrdinal
	}

	return left + right
}

// rowProjectionCursor pulls one projection at a time and retains no yielded corpus.
type rowProjectionCursor struct {
	next func() (rowProjectionView, error, bool)
	stop func()
}

// newRowProjectionCursor starts the one shared composition traversal.
func newRowProjectionCursor(
	node *schemaNode,
	occurrence schemaOccurrence,
	requirements []requirement,
) *rowProjectionCursor {
	sequence := func(yield func(rowProjectionView, error) bool) {
		walk := rowProjectionWalk{requirements: requirements}
		if _, err := walk.node(node, occurrence, nil, func(sources []rowSchemaSource) bool {
			return yield(rowProjectionView{sources: sources}, nil)
		}); err != nil {
			yield(rowProjectionView{}, err)
		}
	}

	next, stop := iter.Pull2(iter.Seq2[rowProjectionView, error](sequence))

	return &rowProjectionCursor{next: next, stop: stop}
}

// Next returns a view valid until the next Next or Close call.
func (cursor *rowProjectionCursor) Next() (rowProjectionView, bool, error) {
	if cursor == nil || cursor.next == nil {
		return rowProjectionView{}, false, errors.New("schematest: projection cursor is not initialized")
	}

	view, err, ok := cursor.next()
	if err != nil {
		cursor.stop()

		return rowProjectionView{}, false, err
	}

	return view, ok, nil
}

// Close releases a projection traversal stopped before exhaustion.
func (cursor *rowProjectionCursor) Close() {
	if cursor != nil && cursor.stop != nil {
		cursor.stop()
	}
}

// rowProjectionWalk is the suspended depth-first traversal behind one cursor.
type rowProjectionWalk struct {
	requirements []requirement
}

// rowProjectionContinuation consumes one transient active-source prefix.
type rowProjectionContinuation func([]rowSchemaSource) bool

// node adds local rules, then composes allOf children and one complete anyOf mask.
func (walk rowProjectionWalk) node(
	node *schemaNode,
	occurrence schemaOccurrence,
	sources []rowSchemaSource,
	continueWith rowProjectionContinuation,
) (bool, error) {
	if node == nil || node.schemaShape == nil {
		return false, errors.New("schematest: projected row schema has no shape")
	}

	sources = append(sources, rowSchemaSource{node: node, occurrence: occurrence})

	var continuationErr error

	continued, err := walk.allOf(node, occurrence, sources, 0, func(active []rowSchemaSource) bool {
		var keepGoing bool

		keepGoing, continuationErr = walk.anyOf(node, occurrence, active, continueWith)

		return keepGoing && continuationErr == nil
	})
	if err != nil {
		return false, err
	}

	return continued, continuationErr
}

// allOf keeps every branch active and lazily multiplies only requested nested views.
func (walk rowProjectionWalk) allOf(
	node *schemaNode,
	occurrence schemaOccurrence,
	sources []rowSchemaSource,
	index int,
	continueWith rowProjectionContinuation,
) (bool, error) {
	if index == len(node.allOf) {
		return continueWith(sources), nil
	}

	child := node.allOf[index]
	childOccurrence := rebasePlanOccurrence(
		child,
		occurrence,
		occurrence.usePointer+"/allOf/"+itoa(index),
		occurrence.instanceTemplate,
	)

	var continuationErr error

	continued, err := walk.node(child, childOccurrence, sources, func(active []rowSchemaSource) bool {
		var keepGoing bool

		keepGoing, continuationErr = walk.allOf(node, occurrence, active, index+1, continueWith)

		return keepGoing && continuationErr == nil
	})
	if err != nil {
		return false, err
	}

	return continued, continuationErr
}

// anyOf advances a compressed arbitrary-precision counter over only unpinned bits.
//
//nolint:cyclop // Fixed pins and compressed bit placement form one mask operation.
func (walk rowProjectionWalk) anyOf(
	node *schemaNode,
	occurrence schemaOccurrence,
	sources []rowSchemaSource,
	continueWith rowProjectionContinuation,
) (bool, error) {
	if len(node.anyOf) == 0 {
		return continueWith(sources), nil
	}

	fixed := new(big.Int)

	unpinned := make([]int, 0, len(node.anyOf))
	for index := range node.anyOf {
		truth, pinned, err := rowProjectionBranchPin(walk.requirements, occurrence, index)
		if err != nil {
			return false, err
		}

		if !pinned {
			unpinned = append(unpinned, index)
		} else if truth {
			fixed.SetBit(fixed, index, 1)
		}
	}

	limit := new(big.Int).Lsh(big.NewInt(1), uint(len(unpinned)))
	for compressed := new(big.Int); compressed.Cmp(limit) < 0; compressed.Add(compressed, big.NewInt(1)) {
		mask := new(big.Int).Set(fixed)
		for compressedBit, branch := range unpinned {
			mask.SetBit(mask, branch, compressed.Bit(compressedBit))
		}

		if mask.Sign() == 0 {
			continue
		}

		continued, err := walk.anyOfBranches(node, occurrence, sources, mask, 0, continueWith)
		if err != nil || !continued {
			return continued, err
		}
	}

	return true, nil
}

// anyOfBranches conjunctively traverses each true branch of one current mask.
func (walk rowProjectionWalk) anyOfBranches(
	node *schemaNode,
	occurrence schemaOccurrence,
	sources []rowSchemaSource,
	mask *big.Int,
	index int,
	continueWith rowProjectionContinuation,
) (bool, error) {
	if index == len(node.anyOf) {
		return continueWith(sources), nil
	}

	if mask.Bit(index) == 0 {
		return walk.anyOfBranches(node, occurrence, sources, mask, index+1, continueWith)
	}

	child := node.anyOf[index]
	childOccurrence := rebasePlanOccurrence(
		child,
		occurrence,
		occurrence.usePointer+"/anyOf/"+itoa(index),
		occurrence.instanceTemplate,
	)

	var continuationErr error

	continued, err := walk.node(child, childOccurrence, sources, func(active []rowSchemaSource) bool {
		var keepGoing bool

		keepGoing, continuationErr = walk.anyOfBranches(
			node, occurrence, active, mask, index+1, continueWith,
		)

		return keepGoing && continuationErr == nil
	})
	if err != nil {
		return false, err
	}

	return continued, continuationErr
}

// rowProjectionBranchPin returns one explicit bit constraint without defaulting absent bits.
func rowProjectionBranchPin(
	requirements []requirement,
	occurrence schemaOccurrence,
	branch int,
) (bool, bool, error) {
	branchUsePointer := occurrence.usePointer + "/anyOf/" + itoa(branch)

	var (
		truth  bool
		pinned bool
	)

	for _, requirement := range requirements {
		if !requirement.hasBranch || requirement.composition != "anyOf" || requirement.branch != branch ||
			requirement.occurrence.usePointer != branchUsePointer ||
			!instanceTemplateMatches(requirement.occurrence.instanceTemplate, occurrence.instanceTemplate) {
			continue
		}

		if pinned && truth != requirement.truth {
			return false, false, errors.New("schematest: conflicting composition branch requirements")
		}

		truth = requirement.truth
		pinned = true
	}

	return truth, pinned, nil
}
