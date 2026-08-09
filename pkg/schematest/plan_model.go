//nolint:godoclint // The planner is private and has no exported identity types.
package schematest

import "strings"

const (
	planFaultPrefix = "fault:"

	planLevelAllTrue = "all-true"
	planLevelMask    = "mask:"
)

const (
	requirementNoPresence requirementPresence = iota
	requirementPresent
	requirementAbsent
)

// obligation is one private report identity. Its string form is the only
// representation that leaves the planner.
type obligation struct {
	ruleIdentity

	component string
	ruleRank  uint8
	order     uint64
	orderKey  *planOrderKey
}

// String renders schema use site, instance template, rule, and level or fault.
func (identity obligation) String() string {
	return identity.ruleIdentity.String() + "|" + identity.component
}

// requirementKind tags each concrete declarative constraint.
type requirementKind uint8

const (
	requirementNone requirementKind = iota
	requirementActiveRules
	requirementTargetLevel
	requirementExactEnumMember
	requirementJSONKind
	requirementPresenceState
	requirementExactCount
	requirementBranchTruth
)

// requirement is one declarative search constraint. A schema reference points
// only to immutable admitted model input; enum values are authored input, never
// generated candidates.
type requirement struct {
	tag requirementKind

	occurrence schemaOccurrence
	active     *schemaNode
	target     levelIdentity
	enumMember *enumMember
	count      *exactCount

	kind        jsonKind
	hasKind     bool
	presence    requirementPresence
	canonical   bool // Canonical structural assignments remain repairable during search.
	composition string
	branch      int
	truth       bool
	hasBranch   bool
}

// requirementPresence identifies whether a structural child must be supplied.
type requirementPresence uint8

// validIntent is one cataloged valid obligation and its complete requirements.
type validIntent struct {
	obligation   obligation
	expected     levelIdentity
	requirements []requirement
	stringFormat schemaFormat
}

// validRequest is one additive row request. The baseline has no focus; every
// later request replaces exactly targets[focus] in that baseline vector.
type validRequest struct {
	targets          []validIntent
	components       [][]requirement
	requirements     []requirement
	stringObjectives []levelIdentity
	formatBoundary   *formatBoundaryObjective
	focus            int
}

// formatBoundaryObjective is one registry-owned valid witness request.
type formatBoundaryObjective struct {
	identity levelIdentity
	format   schemaFormat
	boundary stringFormatBoundary
}

// faultClosure is one immutable authoritative expected identity set.
type faultClosure = []evaluationRecordIdentity

// faultClosureProgram is one branch-local closure domain. Its linked
// alternatives and successor domains describe a lazy product without storing
// any product tuple.
type faultClosureProgram struct {
	alternatives *faultClosureAlternative
	next         *faultClosureProgram
}

// faultClosureAlternative is one declarative way to make a branch false.
type faultClosureAlternative struct {
	requirements []requirement
	expected     faultClosure
	closure      *faultClosureProgram
	next         *faultClosureAlternative
}

// faultProgram is one isolated invalid obligation and its requirements.
type faultProgram struct {
	obligation   obligation
	requirements []requirement
	expected     faultClosure
	alternatives *faultClosureProgram
}

// searchPlan keeps report, valid execution, and fault execution order separate.
// It retains no rows or scalar candidates.
type searchPlan struct {
	validCatalog     []validIntent
	validSchedule    []validRequest
	stringObjectives []levelIdentity
	faultSchedule    []faultProgram
	faultExecution   []int
	obligations      []obligation
}

// obligationIDs returns report order without exposing planner types publicly.
func (plan *searchPlan) obligationIDs() []string {
	if plan == nil || len(plan.obligations) == 0 {
		return nil
	}

	result := make([]string, 0, len(plan.obligations))
	for _, obligation := range plan.obligations {
		result = append(result, obligation.String())
	}

	return result
}

// validObligationIDs returns the focused-valid portion of report order.
func (plan *searchPlan) validObligationIDs() []string {
	if plan == nil || len(plan.validCatalog) == 0 {
		return nil
	}

	result := make([]string, 0, len(plan.validCatalog))
	for _, target := range plan.validCatalog {
		result = append(result, target.obligation.String())
	}

	return result
}

// faultObligationIDs returns the isolated-fault portion of report order.
func (plan *searchPlan) faultObligationIDs() []string {
	if plan == nil || len(plan.faultSchedule) == 0 {
		return nil
	}

	result := make([]string, 0, len(plan.faultSchedule))
	for _, target := range plan.faultSchedule {
		result = append(result, target.obligation.String())
	}

	return result
}

// makeLevelObligation creates a valid-level obligation.
func makeLevelObligation(identity ruleIdentity, level string) obligation {
	return obligation{
		ruleIdentity: identity,
		component:    oracleLevelPrefix + level,
	}
}

// makeFaultObligation creates a fault obligation.
func makeFaultObligation(identity ruleIdentity, fault string) obligation {
	return obligation{
		ruleIdentity: identity,
		component:    planFaultPrefix + fault,
	}
}

// makeLevelIdentity creates the expected clean-oracle level.
func makeLevelIdentity(identity ruleIdentity, level string) levelIdentity {
	return levelIdentity{ruleIdentity: identity, level: level}
}

// planComponentIsFault reports whether an obligation component is a fault.
func planComponentIsFault(component string) bool {
	return strings.HasPrefix(component, planFaultPrefix)
}
