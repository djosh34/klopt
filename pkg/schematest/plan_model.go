//nolint:godoclint // The planner is private and has no exported identity types.
package schematest

import "strings"

const (
	planFaultPrefix = "fault:"

	planLevelAllTrue = "all-true"
	planLevelMask    = "mask:"
)

const (
	planPinNoPresence pinPresence = iota
	planPinPresent
	planPinAbsent
)

// obligation is one private report identity. Its string form is the only
// representation that leaves the planner.
type obligation struct {
	ruleIdentity

	component string
	ruleRank  uint8
	order     uint64
}

// String renders schema use site, instance template, rule, and level or fault.
func (identity obligation) String() string {
	return identity.ruleIdentity.String() + "|" + identity.component
}

// applicabilityPin is a structural precondition for one planned target.
type applicabilityPin struct {
	occurrence  schemaOccurrence
	kind        jsonKind
	hasKind     bool
	presence    pinPresence
	composition string
	branch      int
	truth       bool
	hasBranch   bool
}

// pinPresence identifies whether a structural child must be supplied.
type pinPresence uint8

// validTarget is one focused valid obligation and its clean-oracle pins.
type validTarget struct {
	obligation obligation
	expected   levelIdentity
	pins       []applicabilityPin
}

// faultTarget is one isolated invalid obligation and its exact failure closure.
type faultTarget struct {
	obligation obligation
	pins       []applicabilityPin
	closure    []failureIdentity
}

// searchPlan contains canonical valid and fault schedules. It retains no rows
// or scalar witnesses.
type searchPlan struct {
	validTargets []validTarget
	faultTargets []faultTarget
	obligations  []obligation
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
	if plan == nil || len(plan.validTargets) == 0 {
		return nil
	}

	result := make([]string, 0, len(plan.validTargets))
	for _, target := range plan.validTargets {
		result = append(result, target.obligation.String())
	}

	return result
}

// faultObligationIDs returns the isolated-fault portion of report order.
func (plan *searchPlan) faultObligationIDs() []string {
	if plan == nil || len(plan.faultTargets) == 0 {
		return nil
	}

	result := make([]string, 0, len(plan.faultTargets))
	for _, target := range plan.faultTargets {
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
