//nolint:godoclint,lll // Private guard vocabulary and literal fully-qualified ownership rows are self-describing.
package schematest

import (
	"fmt"
	"go/ast"
	"go/types"
	"maps"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa" //nolint:depguard // SSA is required for source-local owner discovery.

	"github.com/stretchr/testify/require"
)

type ownershipForm string

const (
	ownershipAdmittedMetadata ownershipForm = "immutable admitted metadata"
	ownershipPlanMetadata     ownershipForm = "immutable plan identity or requirement"
	ownershipAuthoredValue    ownershipForm = "authored enum or default"
	ownershipMachineState     ownershipForm = "scalar, ordinal, or machine cursor"
	ownershipCurrentValue     ownershipForm = "singular current value"
)

type ownershipLifetime string

const (
	ownershipProcessLifetime    ownershipLifetime = "process immutable"
	ownershipBuildLifetime      ownershipLifetime = "one Build invocation"
	ownershipAttemptLifetime    ownershipLifetime = "current attempt"
	ownershipCallLifetime       ownershipLifetime = "current call"
	ownershipBeforeContinuation ownershipLifetime = "cleared before continuation"
)

type ownershipAllowance struct {
	typeName string
	form     ownershipForm
	lifetime ownershipLifetime
}

// exactOwnershipAllowlist is the complete field-level ownership contract. Every key is a
// fully-qualified package variable or ownerType.field; there are no owner, wildcard, or
// inferred-name entries.
var exactOwnershipAllowlist = exactOwnershipRows(
	ownershipRowGroup{form: ownershipAdmittedMetadata, lifetime: ownershipProcessLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.base64FormatSpecification|*stringFormatSpecification",
		"github.com/djosh34/klopt/pkg/schematest.cidrFormatSpecification|*stringFormatSpecification",
		"github.com/djosh34/klopt/pkg/schematest.dateFormatSpecification|*stringFormatSpecification",
		"github.com/djosh34/klopt/pkg/schematest.dateTimeFormatSpecification|*stringFormatSpecification",
		"github.com/djosh34/klopt/pkg/schematest.emailFormatSpecification|*stringFormatSpecification",
		"github.com/djosh34/klopt/pkg/schematest.ipv4FormatSpecification|*stringFormatSpecification",
		"github.com/djosh34/klopt/pkg/schematest.numericFormatNames|map[string]schemaFormat",
		"github.com/djosh34/klopt/pkg/schematest.operationMethods|[]string",
		"github.com/djosh34/klopt/pkg/schematest.passwordFormatSpecification|*stringFormatSpecification",
		"github.com/djosh34/klopt/pkg/schematest.schemaKeywords|map[string]bool",
		"github.com/djosh34/klopt/pkg/schematest.schemaKinds|map[string]schemaKind",
		"github.com/djosh34/klopt/pkg/schematest.uuidFormatSpecification|*stringFormatSpecification",
	}},
	ownershipRowGroup{form: ownershipMachineState, lifetime: ownershipProcessLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.errBuildNotImplemented|error",
		"github.com/djosh34/klopt/pkg/schematest.errCompositionEditInapplicable|error",
		"github.com/djosh34/klopt/pkg/schematest.errFaultNotFound|error",
		"github.com/djosh34/klopt/pkg/schematest.errMaxSteps|error",
	}},
	ownershipRowGroup{form: ownershipMachineState, lifetime: ownershipCallLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternMatcher.atomMemo|map[cleanAtomMemoKey][]int",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternMatcher.expressionMemo|map[cleanExpressionMemoKey][]int",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternMatcher.sequenceMemo|map[cleanSequenceMemoKey][]int",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternMatcher.units|[]uint16",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgramState.alive|bool",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgramState.counts|[8]uint16",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgramState.flags|uint16",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgramState.length|int",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgramState.phase|uint8",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgramState.position|int",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgramState.values|[8]uint16",
		"github.com/djosh34/klopt/pkg/schematest.cleanAtomMemoKey.atom|*cleanPatternAtom",
		"github.com/djosh34/klopt/pkg/schematest.cleanAtomMemoKey.position|int",
		"github.com/djosh34/klopt/pkg/schematest.cleanExpressionMemoKey.expression|*cleanPatternExpression",
		"github.com/djosh34/klopt/pkg/schematest.cleanExpressionMemoKey.start|int",
		"github.com/djosh34/klopt/pkg/schematest.cleanSequenceMemoKey.sequence|*cleanPatternSequence",
		"github.com/djosh34/klopt/pkg/schematest.cleanSequenceMemoKey.start|int",
	}},
	ownershipRowGroup{form: ownershipAdmittedMetadata, lifetime: ownershipBuildLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternAssertion.expression|*cleanPatternExpression",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternAssertion.positive|bool",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternAtom.class|cleanPatternClass",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternAtom.expression|*cleanPatternExpression",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternAtom.kind|cleanPatternAtomKind",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternAtom.literal|uint16",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternClass.negated|bool",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternClass.parts|[]cleanPatternClassPart",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternClassPart.negated|bool",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternClassPart.ranges|[]cleanPatternRange",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternExpression.alternatives|[]*cleanPatternSequence",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternProgram.assertions|[]cleanPatternAssertion",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternProgram.expression|*cleanPatternExpression",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternRange.high|uint16",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternRange.low|uint16",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternSequence.terms|[]*cleanPatternTerm",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternTerm.atom|*cleanPatternAtom",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternTerm.greedy|bool",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternTerm.maximum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternTerm.minimum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternTerm.quantified|bool",
		"github.com/djosh34/klopt/pkg/schematest.cleanPatternTerm.unbounded|bool",
		"github.com/djosh34/klopt/pkg/schematest.enumMember.authoredIndex|int",
		"github.com/djosh34/klopt/pkg/schematest.exactCount.number|*exactNumber",
		"github.com/djosh34/klopt/pkg/schematest.patternAST.cleanProgram|*cleanPatternProgram",
		"github.com/djosh34/klopt/pkg/schematest.patternAST.expression|*patternExpression",
		"github.com/djosh34/klopt/pkg/schematest.patternAST.leadingAssertions|[]patternLookahead",
		"github.com/djosh34/klopt/pkg/schematest.patternAST.matcherBytes|int",
		"github.com/djosh34/klopt/pkg/schematest.patternAST.nodeCount|int",
		"github.com/djosh34/klopt/pkg/schematest.patternAST.searchMachines|[]basicStringMachine",
		"github.com/djosh34/klopt/pkg/schematest.patternAST.source|string",
		"github.com/djosh34/klopt/pkg/schematest.patternAtom.class|patternClass",
		"github.com/djosh34/klopt/pkg/schematest.patternAtom.expression|*patternExpression",
		"github.com/djosh34/klopt/pkg/schematest.patternAtom.kind|patternAtomKind",
		"github.com/djosh34/klopt/pkg/schematest.patternAtom.literal|uint16",
		"github.com/djosh34/klopt/pkg/schematest.patternClass.negated|bool",
		"github.com/djosh34/klopt/pkg/schematest.patternClass.parts|[]patternClassPart",
		"github.com/djosh34/klopt/pkg/schematest.patternClassPart.negated|bool",
		"github.com/djosh34/klopt/pkg/schematest.patternClassPart.ranges|[]patternRange",
		"github.com/djosh34/klopt/pkg/schematest.patternExpression.alternatives|[]*patternSequence",
		"github.com/djosh34/klopt/pkg/schematest.patternLookahead.expression|*patternExpression",
		"github.com/djosh34/klopt/pkg/schematest.patternLookahead.positive|bool",
		"github.com/djosh34/klopt/pkg/schematest.patternRange.high|uint16",
		"github.com/djosh34/klopt/pkg/schematest.patternRange.low|uint16",
		"github.com/djosh34/klopt/pkg/schematest.patternSequence.terms|[]*patternTerm",
		"github.com/djosh34/klopt/pkg/schematest.patternTerm.atom|*patternAtom",
		"github.com/djosh34/klopt/pkg/schematest.patternTerm.counted|bool",
		"github.com/djosh34/klopt/pkg/schematest.patternTerm.greedy|bool",
		"github.com/djosh34/klopt/pkg/schematest.patternTerm.maximum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.patternTerm.minimum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.patternTerm.quantified|bool",
		"github.com/djosh34/klopt/pkg/schematest.patternTerm.unbounded|bool",
		"github.com/djosh34/klopt/pkg/schematest.schemaModel.root|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.schemaNode.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.schemaNode.schemaShape|*schemaShape",
		"github.com/djosh34/klopt/pkg/schematest.schemaOccurrence.instanceTemplate|string",
		"github.com/djosh34/klopt/pkg/schematest.schemaOccurrence.reference|bool",
		"github.com/djosh34/klopt/pkg/schematest.schemaOccurrence.structured|*evaluationOccurrencePaths",
		"github.com/djosh34/klopt/pkg/schematest.schemaOccurrence.targetPointer|string",
		"github.com/djosh34/klopt/pkg/schematest.schemaOccurrence.usePointer|string",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.additionalProperties|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.allOf|[]*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.allowAdditionalProperties|bool",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.anyOf|[]*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.enum|[]enumMember",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.exclusiveMaximum|bool",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.exclusiveMinimum|bool",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.format|schemaFormat",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.items|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.kind|schemaKind",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.maxItems|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.maxLength|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.maxProperties|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.maximum|*exactNumber",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.minItems|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.minLength|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.minProperties|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.minimum|*exactNumber",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.multipleOf|*exactNumber",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.nullable|bool",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.pattern|*patternAST",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.properties|map[string]*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.readOnly|bool",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.required|[]string",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.schemaJSON|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.writeOnly|bool",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatBoundary.bounds|stringFormatBounds",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatBoundary.kind|stringFormatObjective",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatBoundary.matches|func(stringFormatProgramState) bool",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatBoundary.viable|func(stringFormatProgramState) bool",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatBounds.bounded|bool",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatBounds.maximum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatBounds.minimum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatBounds.multiple|uint64",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgram.acceptState|func(stringFormatProgramState) bool",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgram.advanceState|func(stringFormatProgramState, uint16) stringFormatProgramState",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgram.alphabetHigh|uint16",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgram.alphabetLow|uint16",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgram.alphabet|string",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatProgram.bounds|stringFormatBounds",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.bounds|stringFormatBounds",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.formats|[3]schemaFormat",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.inert|bool",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.names|[3]string",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.negativeCounts|[3]uint8",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.negativeObjectives|[3][4]string",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.objectiveCount|uint8",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.objectives|[4]stringFormatBoundary",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.program|*stringFormatProgram",
		"github.com/djosh34/klopt/pkg/schematest.stringFormatSpecification.registrationCount|uint8",
	}},
	ownershipRowGroup{form: ownershipPlanMetadata, lifetime: ownershipBuildLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.Report.Covered|[]string",
		"github.com/djosh34/klopt/pkg/schematest.Report.Steps|uint64",
		"github.com/djosh34/klopt/pkg/schematest.Report.Stop|StopReason",
		"github.com/djosh34/klopt/pkg/schematest.Report.Uncovered|[]string",
		"github.com/djosh34/klopt/pkg/schematest.compositionTruth.branches|[]bool",
		"github.com/djosh34/klopt/pkg/schematest.compositionTruth.ruleIdentity|ruleIdentity",
		"github.com/djosh34/klopt/pkg/schematest.evaluationOccurrencePaths.instance|evaluationPointer",
		"github.com/djosh34/klopt/pkg/schematest.evaluationOccurrencePaths.reference|bool",
		"github.com/djosh34/klopt/pkg/schematest.evaluationOccurrencePaths.targetRoot|evaluationPointer",
		"github.com/djosh34/klopt/pkg/schematest.evaluationOccurrencePaths.target|evaluationPointer",
		"github.com/djosh34/klopt/pkg/schematest.evaluationOccurrencePaths.use|evaluationPointer",
		"github.com/djosh34/klopt/pkg/schematest.evaluationPointer.tokens|[]string",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecord.branches|[]bool",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecord.identity|evaluationRecordIdentity",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecord.kind|evaluationRecordKind",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecord.level|string",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecordIdentity.occurrence|evaluationOccurrencePaths",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecordIdentity.rule|string",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecordPart.count|int",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecordPart.filter|evaluationRecordFilter",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecordPart.nested|*evaluationRecords",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecordPart.records|[]evaluationRecord",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecordPart.transform|occurrenceTransform",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecords.count|int",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecords.nonFailureCount|int",
		"github.com/djosh34/klopt/pkg/schematest.evaluationRecords.parts|[]evaluationRecordPart",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureAlternative.closure|*faultClosureProgram",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureAlternative.expected|faultClosure",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureAlternative.next|*faultClosureAlternative",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureAlternative.requirements|[]requirement",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureProgram.alternatives|*faultClosureAlternative",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureProgram.next|*faultClosureProgram",
		"github.com/djosh34/klopt/pkg/schematest.faultProgram.alternatives|*faultClosureProgram",
		"github.com/djosh34/klopt/pkg/schematest.faultProgram.expected|faultClosure",
		"github.com/djosh34/klopt/pkg/schematest.faultProgram.obligation|obligation",
		"github.com/djosh34/klopt/pkg/schematest.faultProgram.requirements|[]requirement",
		"github.com/djosh34/klopt/pkg/schematest.formatBoundaryObjective.boundary|stringFormatBoundary",
		"github.com/djosh34/klopt/pkg/schematest.formatBoundaryObjective.format|schemaFormat",
		"github.com/djosh34/klopt/pkg/schematest.formatBoundaryObjective.identity|levelIdentity",
		"github.com/djosh34/klopt/pkg/schematest.levelIdentity.level|string",
		"github.com/djosh34/klopt/pkg/schematest.levelIdentity.ruleIdentity|ruleIdentity",
		"github.com/djosh34/klopt/pkg/schematest.obligation.component|string",
		"github.com/djosh34/klopt/pkg/schematest.obligation.orderKey|*planOrderKey",
		"github.com/djosh34/klopt/pkg/schematest.obligation.order|uint64",
		"github.com/djosh34/klopt/pkg/schematest.obligation.ruleIdentity|ruleIdentity",
		"github.com/djosh34/klopt/pkg/schematest.obligation.ruleRank|uint8",
		"github.com/djosh34/klopt/pkg/schematest.occurrenceTransform.from|evaluationOccurrencePaths",
		"github.com/djosh34/klopt/pkg/schematest.occurrenceTransform.to|evaluationOccurrencePaths",
		"github.com/djosh34/klopt/pkg/schematest.planOccurrenceOrderKey.instance|[]planPointerToken",
		"github.com/djosh34/klopt/pkg/schematest.planOccurrenceOrderKey.reference|bool",
		"github.com/djosh34/klopt/pkg/schematest.planOccurrenceOrderKey.target|[]planPointerToken",
		"github.com/djosh34/klopt/pkg/schematest.planOccurrenceOrderKey.use|[]planPointerToken",
		"github.com/djosh34/klopt/pkg/schematest.planOrderKey.component|string",
		"github.com/djosh34/klopt/pkg/schematest.planOrderKey.fault|bool",
		"github.com/djosh34/klopt/pkg/schematest.planOrderKey.level|uint64",
		"github.com/djosh34/klopt/pkg/schematest.planOrderKey.occurrence|planOccurrenceOrderKey",
		"github.com/djosh34/klopt/pkg/schematest.planOrderKey.ruleRank|int",
		"github.com/djosh34/klopt/pkg/schematest.planOrderKey.rule|string",
		"github.com/djosh34/klopt/pkg/schematest.planPointerToken.arrayIndex|uint64",
		"github.com/djosh34/klopt/pkg/schematest.planPointerToken.array|bool",
		"github.com/djosh34/klopt/pkg/schematest.planPointerToken.decoded|string",
		"github.com/djosh34/klopt/pkg/schematest.planPointerToken.raw|string",
		"github.com/djosh34/klopt/pkg/schematest.requirement.active|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.requirement.branch|int",
		"github.com/djosh34/klopt/pkg/schematest.requirement.canonical|bool",
		"github.com/djosh34/klopt/pkg/schematest.requirement.composition|string",
		"github.com/djosh34/klopt/pkg/schematest.requirement.count|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.requirement.enumMember|*enumMember",
		"github.com/djosh34/klopt/pkg/schematest.requirement.hasBranch|bool",
		"github.com/djosh34/klopt/pkg/schematest.requirement.hasKind|bool",
		"github.com/djosh34/klopt/pkg/schematest.requirement.kind|jsonKind",
		"github.com/djosh34/klopt/pkg/schematest.requirement.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.requirement.presence|requirementPresence",
		"github.com/djosh34/klopt/pkg/schematest.requirement.tag|requirementKind",
		"github.com/djosh34/klopt/pkg/schematest.requirement.target|levelIdentity",
		"github.com/djosh34/klopt/pkg/schematest.requirement.truth|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowSearchContext.scalarFault|*faultProgram",
		"github.com/djosh34/klopt/pkg/schematest.rowSearchContext.validRequest|*validRequest",
		"github.com/djosh34/klopt/pkg/schematest.ruleIdentity.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.ruleIdentity.rule|string",
		"github.com/djosh34/klopt/pkg/schematest.ruleIdentity.structured|*evaluationOccurrencePaths",
		"github.com/djosh34/klopt/pkg/schematest.searchPlan.faultExecution|[]int",
		"github.com/djosh34/klopt/pkg/schematest.searchPlan.faultSchedule|[]faultProgram",
		"github.com/djosh34/klopt/pkg/schematest.searchPlan.obligations|[]obligation",
		"github.com/djosh34/klopt/pkg/schematest.searchPlan.stringObjectives|[]levelIdentity",
		"github.com/djosh34/klopt/pkg/schematest.searchPlan.validCatalog|[]validIntent",
		"github.com/djosh34/klopt/pkg/schematest.searchPlan.validSchedule|[]validRequest",
		"github.com/djosh34/klopt/pkg/schematest.stringSearchObjective.closure|faultClosure",
		"github.com/djosh34/klopt/pkg/schematest.stringSearchObjective.kind|stringSearchObjectiveKind",
		"github.com/djosh34/klopt/pkg/schematest.stringSearchObjective.level|string",
		"github.com/djosh34/klopt/pkg/schematest.stringSearchObjective.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.stringSearchObjective.rule|string",
		"github.com/djosh34/klopt/pkg/schematest.validIntent.expected|levelIdentity",
		"github.com/djosh34/klopt/pkg/schematest.validIntent.obligation|obligation",
		"github.com/djosh34/klopt/pkg/schematest.validIntent.requirements|[]requirement",
		"github.com/djosh34/klopt/pkg/schematest.validIntent.stringFormat|schemaFormat",
		"github.com/djosh34/klopt/pkg/schematest.validRequest.components|[][]requirement",
		"github.com/djosh34/klopt/pkg/schematest.validRequest.focus|int",
		"github.com/djosh34/klopt/pkg/schematest.validRequest.formatBoundary|*formatBoundaryObjective",
		"github.com/djosh34/klopt/pkg/schematest.validRequest.requirements|[]requirement",
		"github.com/djosh34/klopt/pkg/schematest.validRequest.stringObjectives|[]levelIdentity",
		"github.com/djosh34/klopt/pkg/schematest.validRequest.targets|[]validIntent",
	}},
	ownershipRowGroup{form: ownershipAuthoredValue, lifetime: ownershipBuildLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.enumMember.value|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.schemaShape.defaultValue|*jsonValue",
	}},
	ownershipRowGroup{form: ownershipMachineState, lifetime: ownershipAttemptLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.activeNumberRule.node|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.activeStringFormat.format|schemaFormat",
		"github.com/djosh34/klopt/pkg/schematest.activeStringFormat.node|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.activeStringFormat.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.activeStringLength.count|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.activeStringLength.minimum|bool",
		"github.com/djosh34/klopt/pkg/schematest.activeStringLength.node|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.activeStringLength.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.activeStringPattern.node|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.activeStringPattern.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.activeStringPattern.pattern|*patternAST",
		"github.com/djosh34/klopt/pkg/schematest.activeStringRules.formats|[]activeStringFormat",
		"github.com/djosh34/klopt/pkg/schematest.activeStringRules.lengths|[]activeStringLength",
		"github.com/djosh34/klopt/pkg/schematest.activeStringRules.patterns|[]activeStringPattern",
		"github.com/djosh34/klopt/pkg/schematest.activeStringRules.supported|bool",
		"github.com/djosh34/klopt/pkg/schematest.arrayCombinationCursor.indexes|[]int",
		"github.com/djosh34/klopt/pkg/schematest.arrayCombinationCursor.length|int",
		"github.com/djosh34/klopt/pkg/schematest.arrayCombinationCursor.started|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringConfiguration.repeats|[]basicStringRepeatState",
		"github.com/djosh34/klopt/pkg/schematest.basicStringConfiguration.state|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringEdge.frame|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringEdge.high|uint16",
		"github.com/djosh34/klopt/pkg/schematest.basicStringEdge.kind|basicStringEdgeKind",
		"github.com/djosh34/klopt/pkg/schematest.basicStringEdge.low|uint16",
		"github.com/djosh34/klopt/pkg/schematest.basicStringEdge.target|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringInterval.high|uint16",
		"github.com/djosh34/klopt/pkg/schematest.basicStringInterval.low|uint16",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengthBounds.maximum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengthBounds.minimum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengthBounds.unbounded|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengthObjective.constrained|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengthObjective.length|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengths.boundaries|[]uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengths.hasMaximum|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengths.maximum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengths.minimumTooLarge|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringLengths.minimum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.accept|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.expected|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.hasBoundary|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.hasSurrogate|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.maxUnits|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.minUnits|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.repeats|[]basicStringRepeat",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.required|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.restartStates|[]int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.start|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.states|[][]basicStringEdge",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.unbounded|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringMachine.wholeBound|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringPatternState.active|[]basicStringConfiguration",
		"github.com/djosh34/klopt/pkg/schematest.basicStringPatternState.matched|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.directedFormat|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.formatPrograms|[]*stringFormatProgram",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.formats|[]activeStringFormat",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.hasSurrogate|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.lengthBounded|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.lengthInfeasible|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.lengthMultiple|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.machines|[]basicStringMachine",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.maxUnits|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.maximumRunes|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.minimumRunes|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.needsPadding|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.objectiveFormat|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.objective|*stringFormatBoundary",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.surrogatePadding|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProduct.unbounded|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProductState.formats|[]stringFormatProgramState",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProductState.length|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProductState.patterns|[]basicStringPatternState",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProductState.pendingHigh|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProductState.position|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringProductState.previousWord|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringRepeat.maximum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringRepeat.minimum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringRepeat.unbounded|bool",
		"github.com/djosh34/klopt/pkg/schematest.basicStringRepeatState.blockedAt|int",
		"github.com/djosh34/klopt/pkg/schematest.basicStringRepeatState.count|uint64",
		"github.com/djosh34/klopt/pkg/schematest.basicStringRepeatState.enteredAt|int",
		"github.com/djosh34/klopt/pkg/schematest.compositionAssignmentCoordinate.projectionRank|uint64",
		"github.com/djosh34/klopt/pkg/schematest.compositionAssignmentCoordinate.rowRank|uint64",
		"github.com/djosh34/klopt/pkg/schematest.compositionAssignmentMachine.assignments|map[compositionAssignmentCoordinate]*compositionRankedSubsetCursor",
		"github.com/djosh34/klopt/pkg/schematest.compositionAssignmentMachine.direct|*compositionRankedSubsetCursor",
		"github.com/djosh34/klopt/pkg/schematest.compositionAssignmentMachine.search|*search",
		"github.com/djosh34/klopt/pkg/schematest.compositionEdit.append|bool",
		"github.com/djosh34/klopt/pkg/schematest.compositionEdit.path|[]string",
		"github.com/djosh34/klopt/pkg/schematest.compositionEdit.remove|bool",
		"github.com/djosh34/klopt/pkg/schematest.compositionEdit.replacement|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSource.cursor|func() compositionEditCursor",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetCursor.levels|[]compositionEditSubsetLevel",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetCursor.seen|int",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetCursor.size|int",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetCursor.source|compositionEditSource",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetCursor.s|*search",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetLevel.cursor|compositionEditCursor",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetLevel.edit|compositionEdit",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetMachine.count|int",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetMachine.cursor|*compositionEditSubsetCursor",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetMachine.size|int",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetMachine.source|compositionEditSource",
		"github.com/djosh34/klopt/pkg/schematest.compositionEditSubsetMachine.s|*search",
		"github.com/djosh34/klopt/pkg/schematest.compositionRankedSubsetCursor.exhausted|bool",
		"github.com/djosh34/klopt/pkg/schematest.compositionRankedSubsetCursor.observed|uint64",
		"github.com/djosh34/klopt/pkg/schematest.compositionRankedSubsetCursor.search|*search",
		"github.com/djosh34/klopt/pkg/schematest.compositionRankedSubsetCursor.source|compositionEditSource",
		"github.com/djosh34/klopt/pkg/schematest.compositionRankedSubsetCursor.stream|*compositionEditSubsetMachine",
		"github.com/djosh34/klopt/pkg/schematest.directRankTupleDecoder.dimensions|int",
		"github.com/djosh34/klopt/pkg/schematest.directRankTupleDecoder.dimension|int",
		"github.com/djosh34/klopt/pkg/schematest.directRankTupleDecoder.ordinal|uint64",
		"github.com/djosh34/klopt/pkg/schematest.directRankTupleDecoder.remaining|uint64",
		"github.com/djosh34/klopt/pkg/schematest.evaluation.err|error",
		"github.com/djosh34/klopt/pkg/schematest.evaluation.failed|bool",
		"github.com/djosh34/klopt/pkg/schematest.evaluation.records|*evaluationRecords",
		"github.com/djosh34/klopt/pkg/schematest.evaluation.valid|bool",
		"github.com/djosh34/klopt/pkg/schematest.evaluationCacheEntry.result|evaluation",
		"github.com/djosh34/klopt/pkg/schematest.evaluationCacheKey.shape|*schemaShape",
		"github.com/djosh34/klopt/pkg/schematest.evaluationContext.base|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.evaluationContext.cache|map[evaluationCacheKey]evaluationCacheEntry",
		"github.com/djosh34/klopt/pkg/schematest.exactDecimalTerm.coefficient|*math/big.Int",
		"github.com/djosh34/klopt/pkg/schematest.exactDecimalTerm.exponent|*math/big.Int",
		"github.com/djosh34/klopt/pkg/schematest.exactNumber.denominator|*math/big.Int",
		"github.com/djosh34/klopt/pkg/schematest.exactNumber.exponent|*math/big.Int",
		"github.com/djosh34/klopt/pkg/schematest.exactNumber.numerator|*math/big.Int",
		"github.com/djosh34/klopt/pkg/schematest.exactNumber.scale|*math/big.Int",
		"github.com/djosh34/klopt/pkg/schematest.exactNumberParts.authoredExponent|*math/big.Int",
		"github.com/djosh34/klopt/pkg/schematest.exactNumberParts.digits|string",
		"github.com/djosh34/klopt/pkg/schematest.exactNumberParts.end|int",
		"github.com/djosh34/klopt/pkg/schematest.exactNumberParts.fractionDigits|int",
		"github.com/djosh34/klopt/pkg/schematest.exactNumberParts.negative|bool",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureExhaustion.parentFinite|bool",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureExhaustion.parentSize|uint64",
		"github.com/djosh34/klopt/pkg/schematest.faultClosureExhaustion.parents|map[uint64]*faultParentExhaustion",
		"github.com/djosh34/klopt/pkg/schematest.faultOccurrenceExhaustion.compositionAssignments|*compositionAssignmentMachine",
		"github.com/djosh34/klopt/pkg/schematest.faultOccurrenceExhaustion.mutationFinite|bool",
		"github.com/djosh34/klopt/pkg/schematest.faultOccurrenceExhaustion.mutationSize|uint64",
		"github.com/djosh34/klopt/pkg/schematest.faultParentExhaustion.occurrenceFinite|bool",
		"github.com/djosh34/klopt/pkg/schematest.faultParentExhaustion.occurrenceSize|uint64",
		"github.com/djosh34/klopt/pkg/schematest.faultParentExhaustion.occurrences|map[uint64]*faultOccurrenceExhaustion",
		"github.com/djosh34/klopt/pkg/schematest.faultProductExhaustion.closureFinite|bool",
		"github.com/djosh34/klopt/pkg/schematest.faultProductExhaustion.closureSize|uint64",
		"github.com/djosh34/klopt/pkg/schematest.faultProductExhaustion.closures|map[uint64]*faultClosureExhaustion",
		"github.com/djosh34/klopt/pkg/schematest.faultSchemaChild.node|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.faultSchemaChild.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.faultSearchMachines.compositionAssignments|*compositionAssignmentMachine",
		"github.com/djosh34/klopt/pkg/schematest.faultSearchMachines.parentRows|parentRowMachine",
		"github.com/djosh34/klopt/pkg/schematest.faultSearchMachines.scalarCandidates|scalarCandidateMachine",
		"github.com/djosh34/klopt/pkg/schematest.faultSearchMachines.search|*search",
		"github.com/djosh34/klopt/pkg/schematest.jsonCloneFrame.context|string",
		"github.com/djosh34/klopt/pkg/schematest.jsonCloneFrame.exit|bool",
		"github.com/djosh34/klopt/pkg/schematest.jsonMarshalFrame.entered|bool",
		"github.com/djosh34/klopt/pkg/schematest.jsonMarshalFrame.index|int",
		"github.com/djosh34/klopt/pkg/schematest.jsonMarshalFrame.names|[]string",
		"github.com/djosh34/klopt/pkg/schematest.jsonValidationFrame.context|string",
		"github.com/djosh34/klopt/pkg/schematest.jsonValidationFrame.exit|bool",
		"github.com/djosh34/klopt/pkg/schematest.jsonValuePair.left|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.jsonValuePair.right|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.numberCandidateEmitter.index|uint64",
		"github.com/djosh34/klopt/pkg/schematest.numberCandidateEmitter.schedule|numberSchedule",
		"github.com/djosh34/klopt/pkg/schematest.numberCandidateEmitter.search|*search",
		"github.com/djosh34/klopt/pkg/schematest.numberCandidateEmitter.visit|rowVisit",
		"github.com/djosh34/klopt/pkg/schematest.numberEdge.base|*exactNumber",
		"github.com/djosh34/klopt/pkg/schematest.numberEdge.increment|*exactNumber",
		"github.com/djosh34/klopt/pkg/schematest.numberEdge.sign|int64",
		"github.com/djosh34/klopt/pkg/schematest.numberEdgeAddress.family|numberEdgeFamily",
		"github.com/djosh34/klopt/pkg/schematest.numberEdgeAddress.local|uint64",
		"github.com/djosh34/klopt/pkg/schematest.numberEdgeAddress.ruleIndex|uint64",
		"github.com/djosh34/klopt/pkg/schematest.numberSchedule.hasEnum|bool",
		"github.com/djosh34/klopt/pkg/schematest.numberSchedule.quantum|*exactNumber",
		"github.com/djosh34/klopt/pkg/schematest.numberSchedule.rules|[]activeNumberRule",
		"github.com/djosh34/klopt/pkg/schematest.numberSchedule.seeded|bool",
		"github.com/djosh34/klopt/pkg/schematest.parentReplayGroup.count|int",
		"github.com/djosh34/klopt/pkg/schematest.parentReplayGroup.indexes|[]int",
		"github.com/djosh34/klopt/pkg/schematest.parentRowMachine.search|*search",
		"github.com/djosh34/klopt/pkg/schematest.prospectiveCompositionSource.container|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.prospectiveCompositionSource.path|[]string",
		"github.com/djosh34/klopt/pkg/schematest.rankProductCursor.diagonal|uint64",
		"github.com/djosh34/klopt/pkg/schematest.rankProductCursor.exhausted|bool",
		"github.com/djosh34/klopt/pkg/schematest.rankProductCursor.finiteSizes|[]uint64",
		"github.com/djosh34/klopt/pkg/schematest.rankProductCursor.finite|[]bool",
		"github.com/djosh34/klopt/pkg/schematest.rankProductCursor.started|bool",
		"github.com/djosh34/klopt/pkg/schematest.rankProductCursor.tuple|[]uint64",
		"github.com/djosh34/klopt/pkg/schematest.rankedArrayStructure.active|[]requirement",
		"github.com/djosh34/klopt/pkg/schematest.rankedArrayStructure.length|rowArrayCount",
		"github.com/djosh34/klopt/pkg/schematest.rankedArrayStructure.view|rowProjectionView",
		"github.com/djosh34/klopt/pkg/schematest.rankedObjectStructure.active|[]requirement",
		"github.com/djosh34/klopt/pkg/schematest.rankedObjectStructure.extraCount|uint64",
		"github.com/djosh34/klopt/pkg/schematest.rankedObjectStructure.present|[]bool",
		"github.com/djosh34/klopt/pkg/schematest.rankedObjectStructure.shape|*rowProjectedObject",
		"github.com/djosh34/klopt/pkg/schematest.rankedObjectStructure.view|rowProjectionView",
		"github.com/djosh34/klopt/pkg/schematest.resolvedScalarObjective.identity|levelIdentity",
		"github.com/djosh34/klopt/pkg/schematest.resolvedScalarObjective.node|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayCount.beyond|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayCount.exact|*exactCount",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayCount.value|uint64",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayLengthDomain.exact|rowArrayCount",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayLengthDomain.hasExact|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayLengthDomain.hasMaximum|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayLengthDomain.infeasible|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayLengthDomain.maximum|rowArrayCount",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayLengthDomain.minimum|rowArrayCount",
		"github.com/djosh34/klopt/pkg/schematest.rowArrayLengthDomain.view|rowProjectionView",
		"github.com/djosh34/klopt/pkg/schematest.rowMember.name|string",
		"github.com/djosh34/klopt/pkg/schematest.rowMember.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.rowMember.required|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowMember.schemas|rowSchemaConjunction",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.allowsExtra|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.countConflict|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.declared|map[string]bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.exactBeyond|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.exact|uint64",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.hasExact|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.hasMaximum|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.infeasible|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.maximum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.members|[]rowMember",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.minimumBeyond|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.minimum|uint64",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.owners|[]rowSchemaSource",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.requiresExtra|bool",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectedObject.rootOccurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectionCursor.next|func() (rowProjectionView, error, bool)",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectionCursor.stop|func()",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectionView.sources|[]rowSchemaSource",
		"github.com/djosh34/klopt/pkg/schematest.rowProjectionWalk.requirements|[]requirement",
		"github.com/djosh34/klopt/pkg/schematest.rowSchemaConjunction.fallback|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.rowSchemaConjunction.sources|[]rowSchemaSource",
		"github.com/djosh34/klopt/pkg/schematest.rowSchemaSource.node|*schemaNode",
		"github.com/djosh34/klopt/pkg/schematest.rowSchemaSource.occurrence|schemaOccurrence",
		"github.com/djosh34/klopt/pkg/schematest.scalarCandidateMachine.search|*search",
		"github.com/djosh34/klopt/pkg/schematest.search.maxSteps|uint64",
		"github.com/djosh34/klopt/pkg/schematest.search.model|*schemaModel",
		"github.com/djosh34/klopt/pkg/schematest.search.steps|uint64",
	}},
	ownershipRowGroup{form: ownershipMachineState, lifetime: ownershipCallLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.arrayEditCharges.indexes|[]int",
		"github.com/djosh34/klopt/pkg/schematest.arrayEditCharges.itemValues|int",
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceCursor.stack|[]compositionDifferenceFrame",
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceFrame.arrayIndex|int",
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceFrame.entered|bool",
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceFrame.hasLastName|bool",
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceFrame.lastName|string",
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceFrame.path|[]string",
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceFrame.stage|uint8",
		"github.com/djosh34/klopt/pkg/schematest.compositionDirectEditCursor.pathRank|uint64",
		"github.com/djosh34/klopt/pkg/schematest.compositionDirectEditCursor.requirementIndex|int",
		"github.com/djosh34/klopt/pkg/schematest.compositionDirectEditCursor.requirements|[]requirement",
	}},
	ownershipRowGroup{form: ownershipCurrentValue, lifetime: ownershipCallLifetime, rows: []string{
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceFrame.assignment|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.compositionDifferenceFrame.parent|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.compositionDirectEditCursor.parent|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.evaluationCacheKey.value|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.jsonCloneFrame.clone|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.jsonCloneFrame.source|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.jsonMarshalFrame.value|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.jsonValidationFrame.value|*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.jsonValue.array|[]*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.jsonValue.boolean|bool",
		"github.com/djosh34/klopt/pkg/schematest.jsonValue.kind|jsonKind",
		"github.com/djosh34/klopt/pkg/schematest.jsonValue.number|*exactNumber",
		"github.com/djosh34/klopt/pkg/schematest.jsonValue.object|map[string]*jsonValue",
		"github.com/djosh34/klopt/pkg/schematest.jsonValue.text|string",
	}},
)

type ownershipRowGroup struct {
	form     ownershipForm
	lifetime ownershipLifetime
	rows     []string
}

func exactOwnershipRows(groups ...ownershipRowGroup) map[string]ownershipAllowance {
	rows := make(map[string]ownershipAllowance)
	for _, group := range groups {
		for _, row := range group.rows {
			key, typeName, ok := strings.Cut(row, "|")
			if !ok || key == "" || typeName == "" {
				panic("invalid exact ownership row: " + row)
			}

			if _, exists := rows[key]; exists {
				panic("duplicate exact ownership row: " + key)
			}

			rows[key] = ownershipAllowance{typeName: typeName, form: group.form, lifetime: group.lifetime}
		}
	}

	return rows
}

func TestExactLongLivedOwnershipShapes(t *testing.T) {
	t.Parallel()

	require.Empty(t, exactOwnershipViolations(productionGuardPackage(t), exactOwnershipAllowlist))
}

func TestProductionCurrentOwnershipRowsAreLifecycleGuarded(t *testing.T) {
	t.Parallel()

	rows := 0

	for _, allowance := range exactOwnershipAllowlist {
		if allowance.form == ownershipCurrentValue {
			rows++
		}
	}

	require.Positive(t, rows)
	require.Empty(t, generatedValueEscapeViolations(productionGuardPackage(t)))
}

func TestExactOwnershipGuardRejectsUnlistedAndChangedFields(t *testing.T) {
	t.Parallel()

	const prefix = modulePath + "/pkg/schematest."

	allowed := map[string]ownershipAllowance{
		prefix + "search.steps": {typeName: "uint64", form: ownershipMachineState, lifetime: ownershipAttemptLifetime},
	}

	tests := []string{
		`package schematest; type search struct { steps uint64; archive []string }; func Build() { _ = new(search) }`,
		`package schematest; type search struct { steps []string }; func Build() { _ = new(search) }`,
		`package schematest; type search struct { steps []byte }; func Build() { _ = new(search) }`,
		`package schematest; type search struct { steps any }; func Build() { _ = new(search) }`,
		`package schematest; type Case struct{}; type search struct { steps []Case }; func Build() { _ = new(search) }`,
		`package schematest; type jsonValue struct{}; type search struct { steps []*jsonValue }; func Build() { _ = new(search) }`,
	}
	for _, source := range tests {
		guardPackage := parseGuardPackage(t, map[string]string{"guard.go": source})
		require.NotEmpty(t, exactOwnershipViolations(guardPackage, allowed), source)
	}
}

func TestExactOwnershipGuardRejectsPlanSearchAndPackageEncodings(t *testing.T) {
	t.Parallel()

	const prefix = modulePath + "/pkg/schematest."

	tests := []struct {
		source   string
		allowed  map[string]ownershipAllowance
		rejected string
	}{
		{
			source:   `package schematest; var x []string; func Build() {}`,
			allowed:  map[string]ownershipAllowance{},
			rejected: prefix + "x",
		},
		{
			source: `package schematest; type box struct { x []string }; var state box; func Build() {}`,
			allowed: map[string]ownershipAllowance{
				prefix + "state": {
					typeName: "box", form: ownershipMachineState, lifetime: ownershipProcessLifetime,
				},
			},
			rejected: prefix + "box.x",
		},
		{
			source: `package schematest; type searchPlan struct { identity uint64; x string }; ` +
				`func Build() { plan := new(searchPlan); _ = plan }`,
			allowed: map[string]ownershipAllowance{
				prefix + "searchPlan.identity": {
					typeName: "uint64", form: ownershipPlanMetadata, lifetime: ownershipBuildLifetime,
				},
			},
			rejected: prefix + "searchPlan.x",
		},
		{
			source: `package schematest; type search struct { ordinal uint64; x any }; ` +
				`func Build() { active := new(search); _ = active }`,
			allowed: map[string]ownershipAllowance{
				prefix + "search.ordinal": {
					typeName: "uint64", form: ownershipMachineState, lifetime: ownershipAttemptLifetime,
				},
			},
			rejected: prefix + "search.x",
		},
		{
			source: `package schematest; type search struct { ordinal uint64; x [][]byte }; ` +
				`func Build() { active := new(search); _ = active }`,
			allowed: map[string]ownershipAllowance{
				prefix + "search.ordinal": {
					typeName: "uint64", form: ownershipMachineState, lifetime: ownershipAttemptLifetime,
				},
			},
			rejected: prefix + "search.x",
		},
	}
	for _, test := range tests {
		violations := exactOwnershipViolations(
			parseGuardPackage(t, map[string]string{"guard.go": test.source}), test.allowed,
		)
		require.Contains(t, violations, "unlisted owner field: "+test.rejected, test.source)
	}
}

func TestExactOwnershipGuardDiscoversInlineSignatureAndReturnOwners(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type inlineOwner struct { value int }
		type signatureOwner struct { value int }
		type returnedOwner struct { value int }
		func makeOwner() returnedOwner { return returnedOwner{} }
		func passOwner(value signatureOwner) { _ = value }
		func consume(*inlineOwner) {}
		func Build() { consume(new(inlineOwner)); passOwner(signatureOwner{}); _ = makeOwner() }
	`

	violations := exactOwnershipViolations(parseGuardPackage(t, map[string]string{"guard.go": source}), nil)
	for _, owner := range []string{"inlineOwner.value", "signatureOwner.value", "returnedOwner.value"} {
		require.Contains(t, violations, "unlisted owner field: "+modulePath+"/pkg/schematest."+owner)
	}
}

func TestExactOwnershipGuardRejectsMisleadingCarrierRows(t *testing.T) {
	t.Parallel()

	const prefix = modulePath + "/pkg/schematest."

	tests := []struct {
		typeName string
		field    string
	}{
		{typeName: "[]Case", field: "values"},
		{typeName: "[]string", field: "values"},
		{typeName: "any", field: "value"},
		{typeName: "[]*jsonValue", field: "values"},
	}
	for _, test := range tests {
		source := `package schematest; type Case struct{}; type jsonValue struct{}; type owner struct { ` +
			test.field + ` ` + test.typeName + ` }; func Build() { _ = new(owner) }`
		allowed := map[string]ownershipAllowance{
			prefix + "owner." + test.field: {
				typeName: test.typeName, form: ownershipMachineState, lifetime: ownershipCallLifetime,
			},
		}
		require.NotEmpty(t, exactOwnershipViolations(parseGuardPackage(t, map[string]string{"guard.go": source}), allowed))
	}
}

func TestExactOwnershipGuardSeparatesAuthoredValuesFromGeneratedState(t *testing.T) {
	t.Parallel()

	const prefix = modulePath + "/pkg/schematest."

	source := `package schematest
        type jsonValue struct{}
        type enumMember struct { value *jsonValue }
        type schemaShape struct { defaultValue *jsonValue; enum []enumMember }
        type search struct { generated *jsonValue }
        func Build() { model, active := new(schemaShape), new(search); _, _ = model, active }
    `
	allowed := map[string]ownershipAllowance{
		prefix + "enumMember.value":         {typeName: "*jsonValue", form: ownershipAuthoredValue, lifetime: ownershipBuildLifetime},
		prefix + "schemaShape.defaultValue": {typeName: "*jsonValue", form: ownershipAuthoredValue, lifetime: ownershipBuildLifetime},
		prefix + "schemaShape.enum":         {typeName: "[]enumMember", form: ownershipAdmittedMetadata, lifetime: ownershipBuildLifetime},
	}

	violations := exactOwnershipViolations(parseGuardPackage(t, map[string]string{"guard.go": source}), allowed)
	require.Contains(t, violations, "unlisted owner field: "+prefix+"search.generated")
}

func TestSingularCurrentOwnershipRequiresClearBeforeContinuation(t *testing.T) {
	t.Parallel()

	const prefix = modulePath + "/pkg/schematest."

	allowed := map[string]ownershipAllowance{
		prefix + "state.current": {
			typeName: "*jsonValue", form: ownershipCurrentValue, lifetime: ownershipBeforeContinuation,
		},
	}
	good := `package schematest
        type jsonValue struct{}; type state struct { current *jsonValue }
        func Build(yield func()) { active := new(state); active.current = new(jsonValue); active.current = nil; yield() }
    `
	bad := `package schematest
        type jsonValue struct{}; type state struct { current *jsonValue }
        func Build(yield func()) { active := new(state); active.current = new(jsonValue); yield() }
    `
	replaced := `package schematest
        type jsonValue struct{}; type state struct { current *jsonValue }
        func Build(yield func()) { active := new(state); active.current = new(jsonValue); active.current = new(jsonValue); active.current = nil; yield() }
    `

	require.Empty(t, exactOwnershipViolations(parseGuardPackage(t, map[string]string{"guard.go": good}), allowed))
	require.Empty(t, exactOwnershipViolations(parseGuardPackage(t, map[string]string{"guard.go": replaced}), allowed))
	require.NotEmpty(t, exactOwnershipViolations(parseGuardPackage(t, map[string]string{"guard.go": bad}), allowed))

	badPaths := []string{
		`package schematest; type jsonValue struct{}; type state struct { current *jsonValue }; func Build(yield func()) { active := new(state); active.current = new(jsonValue); if true { yield() }; active.current = nil }`,
		`package schematest; type jsonValue struct{}; type state struct { current *jsonValue }; func Build(yield func()) { active := new(state); active.current = new(jsonValue); switch 1 { case 1: yield() }; active.current = nil }`,
		`package schematest; type jsonValue struct{}; type state struct { current *jsonValue }; func Build(yield func()) { active := new(state); active.current = new(jsonValue); select { default: yield() }; active.current = nil }`,
		`package schematest; type jsonValue struct{}; type state struct { current *jsonValue }; func next() int { return 1 }; func Build() { active := new(state); active.current = new(jsonValue); _ = next(); active.current = nil }`,
		`package schematest; type jsonValue struct{}; type state struct { current *jsonValue }; func mutate(*state) {}; func Build() { active := new(state); active.current = new(jsonValue); mutate(active); active.current = nil }`,
	}
	for _, source := range badPaths {
		require.NotEmpty(t, exactOwnershipViolations(parseGuardPackage(t, map[string]string{"guard.go": source}), allowed), source)
	}
}

func exactOwnershipViolations(
	guardPackage *sourceGuardPackage,
	allowed map[string]ownershipAllowance,
) []string {
	discovered := buildOwnershipFields(guardPackage)

	var violations []string

	for key, field := range discovered {
		allowance, exists := allowed[key]
		if !exists {
			violations = append(violations, "unlisted owner field: "+key)

			continue
		}

		got := types.TypeString(field.Type(), ownershipTypeQualifier(guardPackage.pkg))
		if got != allowance.typeName {
			violations = append(violations, fmt.Sprintf("owner field %s has type %s, want %s", key, got, allowance.typeName))
		}

		if !validOwnershipAllowance(key, field.Type(), allowance, guardPackage) {
			violations = append(violations, "invalid allowed form: "+key)
		}
	}

	for key := range allowed {
		if _, exists := discovered[key]; !exists {
			violations = append(violations, "allowlist row has no owner field: "+key)
		}
	}

	violations = append(violations, currentValueLifecycleViolations(guardPackage, allowed)...)
	slices.Sort(violations)

	return violations
}

func ownershipTypeQualifier(current *types.Package) types.Qualifier {
	return func(pkg *types.Package) string {
		if pkg == current {
			return ""
		}

		return pkg.Path()
	}
}

//nolint:cyclop,gocognit // Package, SSA value, and structural field discovery form one ownership graph.
func buildOwnershipFields(guardPackage *sourceGuardPackage) map[string]*types.Var {
	fields := make(map[string]*types.Var)
	prefix := guardPackage.pkg.Path() + "."

	owners := make(map[*types.TypeName]bool)

	for _, name := range guardPackage.pkg.Scope().Names() {
		if variable, ok := guardPackage.pkg.Scope().Lookup(name).(*types.Var); ok {
			fields[prefix+variable.Name()] = variable
			collectNamedOwnerTypes(variable.Type(), owners)
		}
	}

	ssaPackage := buildGuardSSA(guardPackage)

	build := ssaPackage.Func("Build")
	if build == nil {
		return fields
	}

	for function := range generatedRuntimeFunctions(build) {
		for _, parameter := range function.Params {
			collectNamedOwnerTypes(parameter.Type(), owners)
		}

		for _, freeVariable := range function.FreeVars {
			collectNamedOwnerTypes(freeVariable.Type(), owners)
		}

		if function.Signature != nil {
			collectNamedOwnerTypes(function.Signature.Results(), owners)
		}

		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				if value, ok := instruction.(ssa.Value); ok && value.Type() != nil {
					collectNamedOwnerTypes(value.Type(), owners)
				}
			}
		}
	}

	for owner := range owners {
		if owner.Name() == "Input" || owner.Name() == "Case" {
			delete(owners, owner)
		}
	}

	for owner := range owners {
		if owner.Pkg() != guardPackage.pkg || owner.Parent() != guardPackage.pkg.Scope() {
			continue
		}

		structure, ok := types.Unalias(owner.Type()).Underlying().(*types.Struct)
		if !ok {
			continue
		}

		for index := range structure.NumFields() {
			field := structure.Field(index)
			fields[prefix+owner.Name()+"."+field.Name()] = field
		}
	}

	return fields
}

//nolint:cyclop // Every declared form and carrier shape is checked explicitly.
func validOwnershipAllowance(
	key string,
	owned types.Type,
	allowance ownershipAllowance,
	guardPackage *sourceGuardPackage,
) bool {
	validLifetime := map[ownershipForm]map[ownershipLifetime]bool{
		ownershipAdmittedMetadata: {ownershipProcessLifetime: true, ownershipBuildLifetime: true},
		ownershipPlanMetadata:     {ownershipBuildLifetime: true},
		ownershipAuthoredValue:    {ownershipBuildLifetime: true},
		ownershipMachineState: {
			ownershipProcessLifetime: true, ownershipBuildLifetime: true,
			ownershipAttemptLifetime: true, ownershipCallLifetime: true,
		},
		ownershipCurrentValue: {ownershipCallLifetime: true, ownershipBeforeContinuation: true},
	}
	if !validLifetime[allowance.form][allowance.lifetime] ||
		recursivelyContainsGuardType(owned, packageObjectType(guardPackage, "Case"), make(map[types.Type]bool)) ||
		containsOwnershipInterface(owned, make(map[types.Type]bool)) {
		return false
	}

	if slice, ok := types.Unalias(owned).Underlying().(*types.Slice); ok &&
		directOwnershipType(slice.Elem(), packageObjectType(guardPackage, "jsonValue")) &&
		(allowance.form != ownershipCurrentValue || key != modulePath+"/pkg/schematest.jsonValue.array") {
		return false
	}

	if slice, ok := types.Unalias(owned).Underlying().(*types.Slice); ok && basicTypeKind(slice.Elem()) == types.String &&
		allowance.form == ownershipMachineState {
		allowedStringRoles := map[string]bool{
			modulePath + "/pkg/schematest.compositionDifferenceFrame.path":   true,
			modulePath + "/pkg/schematest.compositionEdit.path":              true,
			modulePath + "/pkg/schematest.jsonMarshalFrame.names":            true,
			modulePath + "/pkg/schematest.prospectiveCompositionSource.path": true,
		}
		if !allowedStringRoles[key] {
			return false
		}
	}

	if allowance.form == ownershipAuthoredValue {
		return recursivelyContainsGuardType(owned, packageObjectType(guardPackage, "jsonValue"), make(map[types.Type]bool))
	}

	if allowance.form == ownershipCurrentValue && allowance.lifetime == ownershipBeforeContinuation {
		pointer, ok := types.Unalias(owned).Underlying().(*types.Pointer)

		return ok && sameGuardType(pointer.Elem(), packageObjectType(guardPackage, "jsonValue"))
	}

	return true
}

func directOwnershipType(owned, wanted types.Type) bool {
	owned = types.Unalias(owned)
	if pointer, ok := owned.(*types.Pointer); ok {
		owned = types.Unalias(pointer.Elem())
	}

	return sameGuardType(owned, wanted)
}

func containsOwnershipInterface(owned types.Type, visiting map[types.Type]bool) bool {
	owned = types.Unalias(owned)
	if visiting[owned] {
		return false
	}

	visiting[owned] = true

	switch typed := owned.Underlying().(type) {
	case *types.Interface:
		return typed.NumMethods() == 0
	case *types.Pointer:
		return containsOwnershipInterface(typed.Elem(), visiting)
	case *types.Slice:
		return containsOwnershipInterface(typed.Elem(), visiting)
	case *types.Array:
		return containsOwnershipInterface(typed.Elem(), visiting)
	case *types.Map:
		return containsOwnershipInterface(typed.Key(), visiting) || containsOwnershipInterface(typed.Elem(), visiting)
	case *types.Struct:
		// A concrete semantic struct is not an interface carrier merely because
		// one of its implementation fields is an error or callback.
		return false
	}

	return false
}

//nolint:cyclop // Every Go ownership wrapper participates in authored-value provenance.
func recursivelyContainsGuardType(owned, wanted types.Type, visiting map[types.Type]bool) bool {
	if sameGuardType(owned, wanted) {
		return true
	}

	owned = types.Unalias(owned)
	if visiting[owned] {
		return false
	}

	visiting[owned] = true
	defer delete(visiting, owned)

	switch typed := owned.Underlying().(type) {
	case *types.Pointer:
		return recursivelyContainsGuardType(typed.Elem(), wanted, visiting)
	case *types.Slice:
		return recursivelyContainsGuardType(typed.Elem(), wanted, visiting)
	case *types.Array:
		return recursivelyContainsGuardType(typed.Elem(), wanted, visiting)
	case *types.Map:
		return recursivelyContainsGuardType(typed.Key(), wanted, visiting) ||
			recursivelyContainsGuardType(typed.Elem(), wanted, visiting)
	case *types.Struct:
		for index := range typed.NumFields() {
			if recursivelyContainsGuardType(typed.Field(index).Type(), wanted, visiting) {
				return true
			}
		}
	}

	return false
}

//nolint:cyclop // Package declarations and watched lifecycle rows are independent checks.
func currentValueLifecycleViolations(
	guardPackage *sourceGuardPackage,
	allowed map[string]ownershipAllowance,
) []string {
	watched := make(map[*types.Var]string)

	for key, allowance := range allowed {
		if allowance.form != ownershipCurrentValue || allowance.lifetime != ownershipBeforeContinuation {
			continue
		}

		field := buildOwnershipFields(guardPackage)[key]
		if field != nil {
			watched[field] = key
		}
	}

	if len(watched) == 0 {
		return nil
	}

	var violations []string

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}

			live := make(map[*types.Var]bool)
			inspectOwnershipStatements(guardPackage, function.Body.List, watched, live, &violations)

			for field := range live {
				violations = append(violations, "current value survives function return: "+watched[field])
			}
		}
	}

	return violations
}

//nolint:cyclop,gocognit,gocyclo // All structured control-flow boundaries share one lifecycle pass.
func inspectOwnershipStatements(
	guardPackage *sourceGuardPackage,
	statements []ast.Stmt,
	watched map[*types.Var]string,
	live map[*types.Var]bool,
	violations *[]string,
) {
	for _, statement := range statements {
		if assignment, ok := statement.(*ast.AssignStmt); ok {
			for index, left := range assignment.Lhs {
				selector, selectorOK := left.(*ast.SelectorExpr)
				if !selectorOK {
					continue
				}

				field, watchedField := guardPackage.info.Uses[selector.Sel].(*types.Var)
				if !watchedField || watched[field] == "" || index >= len(assignment.Rhs) {
					continue
				}

				live[field] = !isNilExpression(assignment.Rhs[index])
				if !live[field] {
					delete(live, field)
				}
			}
		}

		switch typed := statement.(type) {
		case *ast.AssignStmt:
			if ownershipContinuationCall(typed) {
				appendLiveOwnershipViolations(live, watched, "continuation", violations)
			}
		case *ast.ExprStmt:
			if ownershipContinuationCall(typed) {
				appendLiveOwnershipViolations(live, watched, "continuation", violations)
			}
		case *ast.IfStmt:
			if ownershipContinuationCall(typed.Init) || ownershipContinuationCall(typed.Cond) {
				appendLiveOwnershipViolations(live, watched, "continuation", violations)
			}

			thenLive := maps.Clone(live)
			inspectOwnershipStatements(guardPackage, typed.Body.List, watched, thenLive, violations)

			elseLive := maps.Clone(live)
			if typed.Else != nil {
				inspectOwnershipStatements(guardPackage, []ast.Stmt{typed.Else}, watched, elseLive, violations)
			}

			clear(live)

			for field := range thenLive {
				live[field] = true
			}

			for field := range elseLive {
				live[field] = true
			}
		case *ast.BlockStmt:
			inspectOwnershipStatements(guardPackage, typed.List, watched, live, violations)
		case *ast.SwitchStmt:
			if ownershipContinuationCall(typed.Init) || ownershipContinuationCall(typed.Tag) {
				appendLiveOwnershipViolations(live, watched, "continuation", violations)
			}

			for _, clause := range typed.Body.List {
				caseClause, ok := clause.(*ast.CaseClause)
				if !ok {
					continue
				}

				branchLive := maps.Clone(live)
				inspectOwnershipStatements(guardPackage, caseClause.Body, watched, branchLive, violations)
			}
		case *ast.TypeSwitchStmt:
			if ownershipContinuationCall(typed.Init) || ownershipContinuationCall(typed.Assign) {
				appendLiveOwnershipViolations(live, watched, "continuation", violations)
			}

			for _, clause := range typed.Body.List {
				caseClause, ok := clause.(*ast.CaseClause)
				if ok {
					inspectOwnershipStatements(guardPackage, caseClause.Body, watched, maps.Clone(live), violations)
				}
			}
		case *ast.SelectStmt:
			for _, clause := range typed.Body.List {
				communication, ok := clause.(*ast.CommClause)
				if !ok {
					continue
				}

				branchLive := maps.Clone(live)
				if ownershipContinuationCall(communication.Comm) {
					appendLiveOwnershipViolations(branchLive, watched, "continuation", violations)
				}

				inspectOwnershipStatements(guardPackage, communication.Body, watched, branchLive, violations)
			}
		case *ast.ReturnStmt:
			for field := range live {
				*violations = append(*violations, "current value survives function return: "+watched[field])
			}

			clear(live)
		case *ast.ForStmt:
			nested := maps.Clone(live)
			inspectOwnershipStatements(guardPackage, typed.Body.List, watched, nested, violations)

			for field := range nested {
				*violations = append(*violations, "current value survives retry: "+watched[field])
			}
		case *ast.RangeStmt:
			nested := maps.Clone(live)
			inspectOwnershipStatements(guardPackage, typed.Body.List, watched, nested, violations)

			for field := range nested {
				*violations = append(*violations, "current value survives sibling: "+watched[field])
			}
		}
	}
}

func appendLiveOwnershipViolations(
	live map[*types.Var]bool,
	watched map[*types.Var]string,
	boundary string,
	violations *[]string,
) {
	for field := range live {
		*violations = append(*violations, "current value survives "+boundary+": "+watched[field])
	}
}

func ownershipContinuationCall(node ast.Node) bool {
	if node == nil {
		return false
	}

	found := false

	ast.Inspect(node, func(child ast.Node) bool {
		call, ok := child.(*ast.CallExpr)
		if !ok {
			return true
		}

		if identifier, identifierOK := call.Fun.(*ast.Ident); identifierOK {
			switch identifier.Name {
			case "new", "make", "len", "cap", "append", "copy", "delete", "clear", "min", "max":
				return true
			}
		}

		found = true

		return false
	})

	return found
}

func isNilExpression(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)

	return ok && identifier.Name == "nil"
}
