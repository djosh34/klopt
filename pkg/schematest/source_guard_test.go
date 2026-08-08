package schematest

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

// modulePath is the local module import prefix checked by source guards.
const modulePath = "github.com/djosh34/klopt"

// TestProductionImportsStayCleanRoom forbids semantic production dependencies in non-test sources.
func TestProductionImportsStayCleanRoom(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"regexp",
		modulePath + "/pkg/internal/oas",
		modulePath + "/pkg/validation",
		modulePath + "/pkg/jsonvalue",
		modulePath + "/pkg/patternvalidator",
		modulePath + "/pkg/internal/patternsyntax",
		modulePath + "/pkg/internal/stringlanguage",
	}

	for _, file := range productionGoFiles(t) {
		for _, imported := range parseGoFile(t, file).Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			require.NoError(t, err)
			requireCleanRoomImport(t, file, path, forbidden)
		}
	}
}

// requireCleanRoomImport fails for a forbidden semantic or generated local package.
func requireCleanRoomImport(t *testing.T, file, path string, forbidden []string) {
	t.Helper()

	for _, blocked := range forbidden {
		blockedImport := path == blocked || strings.HasPrefix(path, blocked+"/")
		require.Falsef(t, blockedImport, "%s imports forbidden production package %s", file, path)
	}

	require.Falsef(t, isGeneratedImport(path), "%s imports generated package %s", file, path)
}

// isGeneratedImport reports whether a local import names a generator or generated package.
func isGeneratedImport(path string) bool {
	generatedConsumer := modulePath + "/pkg/decode/example"
	if path == generatedConsumer || strings.HasPrefix(path, generatedConsumer+"/") {
		return true
	}

	if !strings.HasPrefix(path, modulePath+"/") {
		return false
	}

	for _, segment := range strings.Split(strings.TrimPrefix(path, modulePath+"/"), "/") {
		if segment == "generate" || strings.HasPrefix(segment, "generated") {
			return true
		}
	}

	return false
}

// TestGeneratedImportGuardRejectsCheckedInConsumer pins the actual generated semantic package root.
func TestGeneratedImportGuardRejectsCheckedInConsumer(t *testing.T) {
	t.Parallel()

	require.True(t, isGeneratedImport(modulePath+"/pkg/decode/example"))
	require.False(t, isGeneratedImport("github.com/goccy/go-yaml"))
}

// TestOnlyLockedDeclarationsArePublic keeps the complete public module interface unchanged.
func TestOnlyLockedDeclarationsArePublic(t *testing.T) {
	t.Parallel()

	guardPackage := productionGuardPackage(t)
	files := guardPackage.files
	want := []string{
		"Build",
		"Case",
		"Input",
		"MaxStepsReached",
		"Report",
		"SpaceExhausted",
		"StopReason",
	}

	var got []string
	for _, file := range files {
		got = append(got, topLevelExportedNames(file)...)
	}

	slices.Sort(got)
	require.Equal(t, want, got)
	require.Empty(t, append(publicAPIViolations(files), publicMethodSetViolations(guardPackage)...))
}

// TestLockedPublicAPIGuardRejectsGrowth proves methods and fields cannot bypass the declaration list.
func TestLockedPublicAPIGuardRejectsGrowth(t *testing.T) {
	t.Parallel()

	base := `package schematest
	type Input struct { OpenAPI []byte; OperationID string; MaxSteps uint64 }
	type Case struct { JSON []byte; Valid bool }
	type StopReason string
	const ( SpaceExhausted StopReason = "space_exhausted"; MaxStepsReached StopReason = "max_steps_reached" )
	type Report struct { Stop StopReason; Steps uint64; Covered []string; Uncovered []string }
	func Build(Input, func(Case) error) (Report, error) { return Report{}, nil }
	`

	tests := []struct {
		name   string
		suffix string
	}{
		{name: "exported method", suffix: `func (Input) Extra() {}`},
		{name: "alias receiver method", suffix: `type inputAlias = Input; func (inputAlias) Extra() {}`},
		{name: "exported field", suffix: `type ExtraInput struct { Extra string }`},
		{name: "StopReason alias"},
		{name: "Input alias"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := base + test.suffix
			switch test.name {
			case "exported field":
				source = strings.Replace(base, "MaxSteps uint64", "MaxSteps uint64; Extra string", 1)
			case "StopReason alias":
				source = strings.Replace(base, "type StopReason string", "type StopReason = string", 1)
			case "Input alias":
				source = strings.Replace(base, "type Input struct", "type Input = struct", 1)
			}

			guardPackage := parseGuardPackage(t, map[string]string{"meta.go": source})
			require.NotEmpty(t, append(
				publicAPIViolations(guardPackage.files),
				publicMethodSetViolations(guardPackage)...,
			))
		})
	}
}

// TestOracleEvaluationHasNoParallelSemanticStores keeps records as the sole fact payload.
func TestOracleEvaluationHasNoParallelSemanticStores(t *testing.T) {
	t.Parallel()

	require.Empty(t, parallelOracleStoreViolations(productionGuardPackage(t)))
}

// TestOracleStoreGuardRejectsCompatibilityFields proves renamed projections cannot return.
func TestOracleStoreGuardRejectsCompatibilityFields(t *testing.T) {
	t.Parallel()

	base := `package schematest
		type evaluationRecord struct{}; type evaluationRecordIdentity struct{}; type evaluationOccurrencePaths struct{}
		type ruleIdentity struct{}; type levelIdentity struct{}; type compositionTruth struct{}
		type occurrenceTransform struct{}; type evaluationRecordFilter uint8
		type evaluationRecordPart struct {
			records []evaluationRecord; nested *evaluationRecords; transform occurrenceTransform
			filter evaluationRecordFilter; count int
		}
		type evaluationRecords struct { parts []evaluationRecordPart; count int; nonFailureCount int }
		type evaluation struct { valid bool; failed bool; records *evaluationRecords; err error }
	`
	tests := []string{
		strings.Replace(base, "err error", "err error; shadow legacyFailures", 1) +
			`type legacyFailures struct { values []ruleIdentity }`,
		strings.Replace(base, "count int\n\t\t}", "count int; identities []evaluationRecordIdentity\n\t\t}", 1),
		strings.Replace(base, "valid bool", "valid string", 1),
		base + `type evaluationCacheEntry struct { values []evaluationRecordIdentity }`,
		base + `type pathBox struct { values []evaluationOccurrencePaths }; ` +
			`type evaluationCacheResult struct { boxes []pathBox }`,
	}

	for _, source := range tests {
		guardPackage := parseGuardPackage(t, map[string]string{"oracle.go": source})
		require.NotEmpty(t, parallelOracleStoreViolations(guardPackage), source)
	}
}

// parallelOracleStoreViolations proves evaluation.records is the sole heterogeneous semantic store.
//
//nolint:cyclop // Exact shapes and cache/result ownership are independent checks.
func parallelOracleStoreViolations(guardPackage *sourceGuardPackage) []string {
	evaluationType := packageObjectType(guardPackage, "evaluation")
	recordsType := packageObjectType(guardPackage, "evaluationRecords")
	partType := packageObjectType(guardPackage, "evaluationRecordPart")
	recordType := packageObjectType(guardPackage, "evaluationRecord")
	transformType := packageObjectType(guardPackage, "occurrenceTransform")

	filterType := packageObjectType(guardPackage, "evaluationRecordFilter")
	if evaluationType == nil || recordsType == nil || partType == nil || recordType == nil ||
		transformType == nil || filterType == nil {
		return []string{"missing oracle record types"}
	}

	booleanType := types.Typ[types.Bool]
	integerType := types.Typ[types.Int]
	errorType := types.Universe.Lookup("error").Type()
	violations := exactOracleStructViolations(evaluationType, "evaluation", []oracleFieldShape{
		{name: "valid", owned: booleanType},
		{name: "failed", owned: booleanType},
		{name: "records", owned: types.NewPointer(recordsType)},
		{name: "err", owned: errorType},
	})
	violations = append(violations, exactOracleStructViolations(recordsType, "evaluationRecords", []oracleFieldShape{
		{name: "parts", owned: types.NewSlice(partType)},
		{name: "count", owned: integerType},
		{name: "nonFailureCount", owned: integerType},
	})...)
	violations = append(violations, exactOracleStructViolations(partType, "evaluationRecordPart", []oracleFieldShape{
		{name: "records", owned: types.NewSlice(recordType)},
		{name: "nested", owned: types.NewPointer(recordsType)},
		{name: "transform", owned: transformType},
		{name: "filter", owned: filterType},
		{name: "count", owned: integerType},
	})...)

	for _, name := range guardPackage.pkg.Scope().Names() {
		object := guardPackage.pkg.Scope().Lookup(name)

		structure, ok := types.Unalias(object.Type()).Underlying().(*types.Struct)
		if !ok || !strings.Contains(strings.ToLower(name), "cache") &&
			!strings.Contains(strings.ToLower(name), "result") {
			continue
		}

		for index := range structure.NumFields() {
			if oracleSemanticCollection(structure.Field(index).Type(), guardPackage) {
				violations = append(violations, name+"."+structure.Field(index).Name())
			}
		}
	}

	return violations
}

// oracleFieldShape is one required field in an authoritative oracle struct.
type oracleFieldShape struct {
	name  string
	owned types.Type
}

// exactOracleStructViolations reports the first mismatch from one authoritative struct.
func exactOracleStructViolations(owned types.Type, name string, fields []oracleFieldShape) []string {
	structure, ok := types.Unalias(owned).Underlying().(*types.Struct)
	if !ok || structure.NumFields() != len(fields) {
		return []string{name}
	}

	for index, field := range fields {
		if structure.Field(index).Name() != field.name || !types.Identical(structure.Field(index).Type(), field.owned) {
			return []string{name + "." + field.name}
		}
	}

	return nil
}

// oracleSemanticCollection reports whether a type directly or transitively owns a typed oracle list.
func oracleSemanticCollection(owned types.Type, guardPackage *sourceGuardPackage) bool {
	semantic := make(map[types.Type]bool)

	for _, name := range []string{
		"evaluationRecord", "evaluationRecordIdentity", "evaluationOccurrencePaths",
		"ruleIdentity", "levelIdentity", "compositionTruth",
	} {
		if candidate := packageObjectType(guardPackage, name); candidate != nil {
			semantic[candidate] = true
		}
	}

	authoritative := map[types.Type]bool{
		packageObjectType(guardPackage, "evaluation"):        true,
		packageObjectType(guardPackage, "evaluationRecords"): true,
		packageObjectType(guardPackage, "schemaShape"):       true,
		packageObjectType(guardPackage, "schemaNode"):        true,
		packageObjectType(guardPackage, "jsonValue"):         true,
	}

	return ownsOracleSemanticCollection(owned, semantic, authoritative, false, make(map[types.Type]bool))
}

// ownsOracleSemanticCollection recursively recognizes direct and wrapped typed oracle lists.
//
//nolint:cyclop // Every Go wrapper form must participate in the semantic-store guard.
func ownsOracleSemanticCollection(
	owned types.Type,
	semantic, authoritative map[types.Type]bool,
	insideCollection bool,
	visiting map[types.Type]bool,
) bool {
	owned = types.Unalias(owned)
	if authoritative[owned] {
		return false
	}

	if semantic[owned] {
		return insideCollection
	}

	if visiting[owned] {
		return false
	}

	visiting[owned] = true
	defer delete(visiting, owned)

	switch typed := owned.Underlying().(type) {
	case *types.Pointer:
		return ownsOracleSemanticCollection(typed.Elem(), semantic, authoritative, insideCollection, visiting)
	case *types.Slice:
		return ownsOracleSemanticCollection(typed.Elem(), semantic, authoritative, true, visiting)
	case *types.Array:
		return ownsOracleSemanticCollection(typed.Elem(), semantic, authoritative, true, visiting)
	case *types.Map:
		return ownsOracleSemanticCollection(typed.Key(), semantic, authoritative, true, visiting) ||
			ownsOracleSemanticCollection(typed.Elem(), semantic, authoritative, true, visiting)
	case *types.Struct:
		for index := range typed.NumFields() {
			if ownsOracleSemanticCollection(
				typed.Field(index).Type(), semantic, authoritative, insideCollection, visiting,
			) {
				return true
			}
		}
	}

	return false
}

// TestCanonicalMetadataHasNoParallelOrHotPathRepresentations locks admission-owned metadata.
func TestCanonicalMetadataHasNoParallelOrHotPathRepresentations(t *testing.T) {
	t.Parallel()

	require.Empty(t, canonicalMetadataViolations(productionGuardPackage(t)))

	tests := []string{
		`package schematest; type jsonValue struct{}; type enumMember struct { value *jsonValue; authoredIndex int }; ` +
			`type schemaShape struct { enum []enumMember; required []string; sourcePositions []int }`,
		`package schematest; type jsonValue struct{}; type enumMember struct { value *jsonValue; authoredIndex int }; ` +
			`type schemaShape struct { enum []enumMember; required []string; orderedRequired []string }`,
		`package schematest
			import "sort"
			type jsonValue struct{}; type enumMember struct { value *jsonValue; authoredIndex int }
			type schemaShape struct { enum []enumMember; required []string }
			func evaluateRequiredMembers(node *schemaShape) { sort.Strings(node.required) }`,
		`package schematest
			type jsonValue struct{}; type enumMember struct { value *jsonValue; authoredIndex int }
			type schemaShape struct { enum []enumMember; required []string }
			func appendUniqueJSONWitness([]*jsonValue, *jsonValue) ([]*jsonValue, error) { return nil, nil }
			func collectAnyOfWitnesses(node *schemaShape) {
				for _, member := range node.enum { _, _ = appendUniqueJSONWitness(nil, member.value) }
			}
			func jsonValidatedSemanticEqual(*jsonValue, *jsonValue) (bool, error) { return false, nil }
			func evaluateEnumRule(node *schemaShape, value *jsonValue) {
				for _, member := range node.enum {
					_, _ = jsonValidatedSemanticEqual(value, member.value)
					_, _ = jsonValidatedSemanticEqual(value, member.value)
				}
			}
			func compileObjectRules(node *schemaShape) {
				ordered := append([]string(nil), node.required...); _ = ordered
			}`,
	}
	for _, source := range tests {
		guardPackage := parseGuardPackage(t, map[string]string{"model.go": source})
		require.NotEmpty(t, canonicalMetadataViolations(guardPackage), source)
	}
}

// canonicalMetadataViolations proves canonical metadata shape and direct hot-path consumption.
//
//nolint:cyclop,gocognit,gocyclo // Type and AST evidence intentionally cover independent bypass forms.
func canonicalMetadataViolations(guardPackage *sourceGuardPackage) []string {
	var violations []string

	shapeType := packageObjectType(guardPackage, "schemaShape")
	memberType := packageObjectType(guardPackage, "enumMember")

	jsonType := packageObjectType(guardPackage, "jsonValue")
	if shapeType == nil || memberType == nil || jsonType == nil {
		return []string{"missing canonical metadata types"}
	}

	shape, ok := types.Unalias(shapeType).Underlying().(*types.Struct)
	if !ok {
		return []string{"schemaShape is not a struct"}
	}

	enumFound, requiredFound := false, false

	for index := range shape.NumFields() {
		field := shape.Field(index)

		slice, sliceOK := types.Unalias(field.Type()).Underlying().(*types.Slice)
		if !sliceOK {
			continue
		}

		switch field.Name() {
		case "enum":
			enumFound = types.Identical(slice.Elem(), memberType)
			if !enumFound {
				violations = append(violations, "schemaShape.enum")
			}
		case "required":
			requiredFound = basicTypeKind(slice.Elem()) == types.String
			if !requiredFound {
				violations = append(violations, "schemaShape.required")
			}
		default:
			if types.Identical(slice.Elem(), memberType) || basicTypeKind(slice.Elem()) == types.Int ||
				basicTypeKind(slice.Elem()) == types.String {
				violations = append(violations, "parallel canonical metadata: schemaShape."+field.Name())
			}
		}
	}

	if !enumFound || !requiredFound {
		violations = append(violations, "missing canonical schemaShape metadata")
	}

	member, ok := types.Unalias(memberType).Underlying().(*types.Struct)
	if !ok || member.NumFields() != 2 || member.Field(0).Name() != "value" ||
		!types.Identical(member.Field(0).Type(), types.NewPointer(jsonType)) ||
		member.Field(1).Name() != "authoredIndex" || basicTypeKind(member.Field(1).Type()) != types.Int {
		violations = append(violations, "enumMember")
	}

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			function, functionOK := declaration.(*ast.FuncDecl)
			if !functionOK || function.Body == nil {
				continue
			}

			switch function.Name.Name {
			case "collectAnyOfWitnesses":
				if enumRangeRecanonicalizes(function) ||
					functionCallUsesWrongFirstArgument(function, "appendUniqueJSONWitness", "generated") {
					violations = append(violations, "planner re-deduplicates admitted enum")
				}
			case "canonicalAnyOfWitnesses":
				if functionCallUsesWrongFirstArgument(function, "appendUniqueJSONWitness", "generated") {
					violations = append(violations, "canonical witnesses compare against admitted enum")
				}
			case "evaluateEnumRule":
				if countFunctionCalls(function, "jsonValidatedSemanticEqual") != 1 ||
					functionCallsAny(function, "jsonSemanticEqual", "sort", "Sort", "SortFunc") {
					violations = append(violations, "evaluator enum equality path")
				}
			case "compileEnumRules":
				if functionCallsAny(function, "jsonSemanticEqual", "jsonValidatedSemanticEqual", "sort", "Sort", "SortFunc") {
					violations = append(violations, "planner enum recanonicalization")
				}
			case "compileObjectRules", "evaluateRequiredMembers":
				if functionCallsAny(function, "sort", "Sort", "SortFunc") ||
					functionPassesSelectorToCall(function, "required") {
					violations = append(violations, "required-name resort")
				}
			}
		}
	}

	return violations
}

// basicTypeKind returns a type's basic kind or types.Invalid.
func basicTypeKind(owned types.Type) types.BasicKind {
	basic, ok := types.Unalias(owned).Underlying().(*types.Basic)
	if !ok {
		return types.Invalid
	}

	return basic.Kind()
}

// enumRangeRecanonicalizes detects calls other than direct append in the admitted enum loop.
func enumRangeRecanonicalizes(function *ast.FuncDecl) bool {
	violates := false

	ast.Inspect(function.Body, func(node ast.Node) bool {
		rangeStatement, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}

		selector, ok := rangeStatement.X.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "enum" {
			return true
		}

		ast.Inspect(rangeStatement.Body, func(child ast.Node) bool {
			call, callOK := child.(*ast.CallExpr)
			if callOK && identName(call.Fun) != "append" {
				violates = true

				return false
			}

			return true
		})

		return !violates
	})

	return violates
}

// functionCallUsesWrongFirstArgument checks a selected ownership seam's accumulator.
func functionCallUsesWrongFirstArgument(function *ast.FuncDecl, calledName, wanted string) bool {
	wrong := false

	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || identName(call.Fun) != calledName || len(call.Args) == 0 {
			return true
		}

		wrong = strings.TrimPrefix(expressionName(call.Args[0]), "*") != wanted

		return !wrong
	})

	return wrong
}

// functionPassesSelectorToCall detects metadata handed to a recanonicalization helper.
func functionPassesSelectorToCall(function *ast.FuncDecl, selectorName string) bool {
	found := false

	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		for _, argument := range call.Args {
			ast.Inspect(argument, func(child ast.Node) bool {
				selector, selectorOK := child.(*ast.SelectorExpr)
				if selectorOK && selector.Sel.Name == selectorName {
					found = true

					return false
				}

				return true
			})
		}

		return !found
	})

	return found
}

// functionCallsAny reports whether a function calls any selected name.
func functionCallsAny(function *ast.FuncDecl, names ...string) bool {
	for _, name := range names {
		if countFunctionCalls(function, name) > 0 {
			return true
		}
	}

	return false
}

// countFunctionCalls counts calls with one identifier or selector name.
func countFunctionCalls(function *ast.FuncDecl, name string) int {
	count := 0

	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		called := identName(call.Fun)
		if selector, selectorOK := call.Fun.(*ast.SelectorExpr); selectorOK {
			called = selector.Sel.Name
		}

		if called == name {
			count++
		}

		return true
	})

	return count
}

// TestStructuredOracleOccurrencesAreParsedOnlyAtOwnershipSeams locks record/cache path reuse.
func TestStructuredOracleOccurrencesAreParsedOnlyAtOwnershipSeams(t *testing.T) {
	t.Parallel()

	guardPackage := productionGuardPackage(t)
	checked := map[string]bool{
		"newEvaluationRecordIdentity": false,
		"rebased":                     false,
	}

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}

			if _, wanted := checked[function.Name.Name]; !wanted {
				continue
			}

			require.False(t, functionCallsAny(function, "mustParseEvaluationOccurrence", "parseEvaluationPointer"))
			checked[function.Name.Name] = true
		}
	}

	require.Equal(t, map[string]bool{"newEvaluationRecordIdentity": true, "rebased": true}, checked)
}

// TestValidatedJSONEqualityDoesNotSortObjects locks the allocation-free object comparison path.
func TestValidatedJSONEqualityDoesNotSortObjects(t *testing.T) {
	t.Parallel()

	guardPackage := productionGuardPackage(t)
	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "jsonValidatedSemanticEqual" {
				continue
			}

			require.False(t, functionCallsAny(function, "sortedObjectNames", "sort", "Sort", "SortFunc"))

			return
		}
	}

	require.Fail(t, "jsonValidatedSemanticEqual is missing")
}

// TestProductionSourceDoesNotEmbedFixtureAnswers rejects source-specific oracle paths.
func TestProductionSourceDoesNotEmbedFixtureAnswers(t *testing.T) {
	t.Parallel()

	guardPackage := productionGuardPackage(t)
	violations := copiedAnswerViolations(guardPackage, canonicalCorpusOperationIDs(t))
	require.Emptyf(t, violations, "production contains copied or fixture-derived semantics: %v", violations)
}

// TestCopiedAnswerGuardRejectsConcreteBypasses pins semantic selectors and renamed generated structures.
func TestCopiedAnswerGuardRejectsConcreteBypasses(t *testing.T) {
	t.Parallel()

	operationIDs := map[string]bool{"allOfObject": true, "alphaRequest": true}
	tests := []map[string]string{
		{"selector.go": `package schematest; type Input struct { OperationID string }; const suffix = "Object"; ` +
			`func f(input Input) { if input.OperationID == "allOf" + suffix {} }`},
		{
			"graph.go": `package schematest
				type vertex struct { links []*vertex; label string }
				var privateTable = func() []*vertex {
					graph := make([]*vertex, 2)
					for index := range graph { graph[index] = new(vertex) }
					*graph[0] = vertex{links: []*vertex{graph[1]}, label: "object"}
					*graph[1] = vertex{label: "string"}
					return graph
				}()
			`,
		},
		{
			"suffix.go": `package schematest; const suffix = "Request"`,
			"selector.go": `package schematest; type Input struct { OperationID string }; ` +
				`func f(input Input) { if input.OperationID == "alpha" + suffix {} }`,
		},
	}

	for _, sources := range tests {
		guardPackage := parseGuardPackage(t, sources)
		require.NotEmpty(t, copiedAnswerViolations(guardPackage, operationIDs))
	}
}

// TestCanonicalCorpusOperationIDsAreSemanticAndComplete covers resources and YAML scalar syntax.
func TestCanonicalCorpusOperationIDsAreSemanticAndComplete(t *testing.T) {
	t.Parallel()

	identifiers := canonicalCorpusOperationIDs(t)
	require.Len(t, identifiers, 16)
	require.True(t, identifiers["alphaRequest"])
	require.True(t, identifiers["referencedRequest"])
	require.True(t, identifiers["allOfObject"])
	require.True(t, identifiers["stringNoFormatNullable"])

	quoted := []byte(`paths:
  /quoted:
    post:
      operationId: "quotedRequest" # a valid inline comment
      requestBody:
        content:
          application/json:
            schema: {type: string}
`)
	got, err := operationIDsFromCorpusDocument(quoted)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"quotedRequest": true}, got)
}

// TestBuildReachableStateOwnsNoCaseCorpus rejects retained callback products without freezing private layouts.
func TestBuildReachableStateOwnsNoCaseCorpus(t *testing.T) {
	t.Parallel()

	violations := corpusOwnershipViolations(productionGuardPackage(t))
	require.Emptyf(t, violations, "production contains corpus-bearing execution state: %v", violations)
}

// TestCorpusOwnershipGuardCoversLocalsClosuresAndPrivateTypes pins representative retention bypasses.
func TestCorpusOwnershipGuardCoversLocalsClosuresAndPrivateTypes(t *testing.T) {
	t.Parallel()

	tests := []string{
		`package schematest; type Case struct{}; var saved []Case; func Build() {}`,
		`package schematest; type Case struct{}; var saved = []Case{}; func Build() {}`,
		`package schematest; type Case struct{}; func Build() { saved := []Case{}; saved = append(saved, Case{}) }`,
		`package schematest; type Case struct{}; var saved [][]Case; func Build() {}`,
		`package schematest; type Case struct{}; var saved map[string][]Case; func Build() {}`,
		`package schematest; type Case struct{}; type savedCase struct { value Case }; ` +
			`var saved []savedCase; func Build() {}`,
		`package schematest; type Case struct{}; type caseList []Case; var saved caseList; func Build() {}`,
		`package schematest; type Case struct{}; type caseList = []Case; var saved caseList; func Build() {}`,
		`package schematest; type Case struct{}; func Build() { var saved []Case; ` +
			`_ = func(c Case) { saved = append(saved, c) } }`,
		`package schematest; type savedBody struct { encoded []byte }; var saved []savedBody; func Build() {}`,
		`package schematest; type jsonValue struct{}; type savedValue struct { value *jsonValue }; ` +
			`var saved []savedValue; func Build() {}`,
		`package schematest; type jsonValue struct{}; func Build() { parents := []jsonValue{}; _ = parents }`,
		`package schematest; type jsonValue struct{}; func Build() { parents := [][]*jsonValue{}; _ = parents }`,
		`package schematest; type jsonValue struct{}; func Build(yield func()) { stream(yield) }; ` +
			`func stream(yield func()) { outputs := []*jsonValue{}; ` +
			`for range 2 { outputs = append(outputs, new(jsonValue)); yield() }; _ = outputs }`,
		`package schematest; type jsonValue struct{}; func Build(yield func()) { ` +
			`callback := yield; stream(callback) }; func stream(yield func()) { ` +
			`rows := []*jsonValue{}; for range 2 { rows = append(rows, new(jsonValue)); yield() }; _ = rows }`,
		`package schematest; type jsonValue struct{}; func Build() { stream() }; ` +
			`func stream() { parents := []*jsonValue{}; parents = append(parents, new(jsonValue)); _ = parents }`,
		`package schematest; type jsonValue struct{}; func Build() { stream() }; ` +
			`func stream() { rows := []*jsonValue{}; rows = append(rows, new(jsonValue)); _ = rows }`,
		`package schematest; type jsonValue struct{}; type helperState struct { outputs []*jsonValue }; ` +
			`func Build() { stream() }; func stream() { state := new(helperState); _ = state }`,
		`package schematest; type Case struct{}; type helperState struct { saved []Case }; ` +
			`func Build(yield func()) { stream(yield) }; func stream(yield func()) { ` +
			`state := new(helperState); yield(); _ = state }`,
		`package schematest; type helperState struct { encoded [][]byte }; ` +
			`func Build(yield func()) { stream(yield) }; func stream(yield func()) { ` +
			`state := new(helperState); yield(); _ = state }`,
	}
	for _, source := range tests {
		guardPackage := parseGuardPackage(t, map[string]string{"meta.go": source})
		require.NotEmpty(t, corpusOwnershipViolations(guardPackage), source)
	}

	harmless := parseGuardPackage(t, map[string]string{
		"meta.go": `package schematest
			type jsonValue struct { array []*jsonValue }
			type schemaShape struct { enum []*jsonValue }
			type schemaModel struct { root *schemaShape }
			type searchPlan struct { targets []int }
			type search struct { path []int }
			func Build() {
				model := new(schemaModel); plan := new(searchPlan); active := new(search)
				_, _, _ = model, plan, active
			}
		`,
	})
	require.Empty(t, corpusOwnershipViolations(harmless))
}

// publicAPIViolations validates exact public type shapes, Build's signature, and public method sets.
//
//nolint:cyclop,gocognit // Declaration, type-shape, method-set, and function-shape checks form one API guard.
func publicAPIViolations(files []*ast.File) []string {
	var violations []string

	types := make(map[string]*ast.TypeSpec)

	var build *ast.FuncDecl

	for _, file := range files {
		for _, declaration := range file.Decls {
			switch typed := declaration.(type) {
			case *ast.FuncDecl:
				if typed.Recv == nil && typed.Name.Name == "Build" {
					build = typed
				}

				if typed.Recv != nil && ast.IsExported(typed.Name.Name) && receiverIsPublicType(typed.Recv) {
					violations = append(violations, "exported method on public type: "+typed.Name.Name)
				}
			case *ast.GenDecl:
				for _, specification := range typed.Specs {
					if typeSpec, ok := specification.(*ast.TypeSpec); ok {
						types[typeSpec.Name.Name] = typeSpec
					}
				}
			}
		}
	}

	wantFields := map[string][]fieldShape{
		"Input": {{"OpenAPI", "[]byte"}, {"OperationID", "string"}, {"MaxSteps", "uint64"}},
		"Case":  {{"JSON", "[]byte"}, {"Valid", "bool"}},
		"Report": {
			{"Stop", "StopReason"},
			{"Steps", "uint64"},
			{"Covered", "[]string"},
			{"Uncovered", "[]string"},
		},
	}
	for name, fields := range wantFields {
		if !structHasExactFields(types[name], fields) {
			violations = append(violations, name+" fields differ from the locked API")
		}
	}

	stopReason, ok := types["StopReason"]
	if !ok || stopReason.Assign.IsValid() || expressionName(stopReason.Type) != "string" {
		violations = append(violations, "StopReason is not a defined string")
	}

	if !buildHasLockedSignature(build) {
		violations = append(violations, "Build signature differs from the locked API")
	}

	return violations
}

// fieldShape describes one locked public struct field.
type fieldShape struct {
	name     string
	typeName string
}

// structHasExactFields reports whether a public struct exactly matches its locked fields.
func structHasExactFields(specification *ast.TypeSpec, want []fieldShape) bool {
	if specification == nil || specification.Assign.IsValid() {
		return false
	}

	structure, ok := specification.Type.(*ast.StructType)
	if !ok || len(structure.Fields.List) != len(want) {
		return false
	}

	for index, field := range structure.Fields.List {
		if len(field.Names) != 1 || field.Names[0].Name != want[index].name ||
			expressionName(field.Type) != want[index].typeName {
			return false
		}
	}

	return true
}

// buildHasLockedSignature reports whether Build retains the sole execution seam.
func buildHasLockedSignature(build *ast.FuncDecl) bool {
	if build == nil || build.Recv != nil {
		return false
	}

	parameters := fieldTypes(build.Type.Params)
	results := fieldTypes(build.Type.Results)

	return len(parameters) == 2 && expressionName(parameters[0]) == "Input" && callbackTypeIsLocked(parameters[1]) &&
		len(results) == 2 && expressionName(results[0]) == "Report" && expressionName(results[1]) == "error"
}

// callbackTypeIsLocked reports whether Build's callback has the required shape.
func callbackTypeIsLocked(expression ast.Expr) bool {
	function, ok := expression.(*ast.FuncType)
	if !ok {
		return false
	}

	parameters := fieldTypes(function.Params)
	results := fieldTypes(function.Results)

	return len(parameters) == 1 && expressionName(parameters[0]) == "Case" &&
		len(results) == 1 && expressionName(results[0]) == "error"
}

// fieldTypes expands grouped fields into one expression per parameter or result.
func fieldTypes(fields *ast.FieldList) []ast.Expr {
	if fields == nil {
		return nil
	}

	var result []ast.Expr

	for _, field := range fields.List {
		count := max(1, len(field.Names))
		for range count {
			result = append(result, field.Type)
		}
	}

	return result
}

// expressionName renders the small type vocabulary used by the locked API.
func expressionName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.ArrayType:
		if typed.Len == nil {
			return "[]" + expressionName(typed.Elt)
		}
	case *ast.StarExpr:
		return "*" + expressionName(typed.X)
	}

	return ""
}

// receiverIsPublicType reports whether a method receiver belongs to a locked public type.
func receiverIsPublicType(receivers *ast.FieldList) bool {
	for _, receiver := range fieldTypes(receivers) {
		name := strings.TrimPrefix(expressionName(receiver), "*")
		if name == "Input" || name == "Case" || name == "StopReason" || name == "Report" {
			return true
		}
	}

	return false
}

// copiedAnswerViolations finds package-wide fixture constants and generated table construction.
func copiedAnswerViolations(guardPackage *sourceGuardPackage, operationIDs map[string]bool) []string {
	var violations []string

	for _, file := range guardPackage.files {
		ast.Inspect(file, func(node ast.Node) bool {
			expression, ok := node.(ast.Expr)
			if !ok {
				return true
			}

			value := guardPackage.info.Types[expression].Value
			if value != nil && value.Kind() == constant.String && operationIDs[constant.StringVal(value)] {
				violations = append(violations, "fixture operation selector: "+constant.StringVal(value))
			}

			return true
		})

		text := string(guardPackage.sources[file])
		for _, forbidden := range []string{
			"testdata/", "alpha_zeta", "request_bodies", "Code generated", "//go:generate",
		} {
			if strings.Contains(text, forbidden) {
				violations = append(violations, "forbidden source marker: "+forbidden)
			}
		}

		if containsGeneratedGraphFingerprint(file) {
			violations = append(violations, "copied generated validation graph construction")
		}
	}

	return violations
}

// containsGeneratedGraphFingerprint detects the generated consumer's allocate-link-fill IIFE shape.
func containsGeneratedGraphFingerprint(file *ast.File) bool {
	found := false

	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return true
		}

		function, ok := call.Fun.(*ast.FuncLit)
		if !ok {
			return true
		}

		if generatedGraphIIFE(function) {
			found = true

			return false
		}

		return true
	})

	return found
}

// generatedGraphIIFE requires the generated table's fixed allocation, initialization, links, and return.
//
//nolint:cyclop // Allocation discovery and the four-part graph fingerprint form one structural check.
func generatedGraphIIFE(function *ast.FuncLit) bool {
	for _, statement := range function.Body.List {
		assignment, ok := statement.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			continue
		}

		name, ok := assignment.Lhs[0].(*ast.Ident)
		if !ok || !isFixedPointerSliceMake(assignment.Rhs[0]) {
			continue
		}

		allocated, filled, linked, returned := generatedGraphOperations(function.Body, name.Name)
		if allocated && filled && linked && returned {
			return true
		}
	}

	return false
}

// isFixedPointerSliceMake identifies make([]*T, N), the generated graph's indexed storage.
func isFixedPointerSliceMake(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return false
	}

	makeName, ok := call.Fun.(*ast.Ident)
	if !ok || makeName.Name != "make" {
		return false
	}

	slice, ok := call.Args[0].(*ast.ArrayType)
	if !ok || slice.Len != nil {
		return false
	}

	_, pointerElements := slice.Elt.(*ast.StarExpr)

	return pointerElements
}

// generatedGraphOperations finds allocation of every node, indexed graph filling, graph links, and return.
//
//nolint:cyclop // Indexed allocation, fill, linking, and return are the required generated graph fingerprint.
func generatedGraphOperations(body *ast.BlockStmt, graph string) (bool, bool, bool, bool) {
	var allocated, filled, linked, returned bool

	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for index, left := range typed.Lhs {
				if indexedGraph(left, graph) && index < len(typed.Rhs) && isNewCall(typed.Rhs[index]) {
					allocated = true
				}

				star, ok := left.(*ast.StarExpr)
				if ok && indexedGraph(star.X, graph) && index < len(typed.Rhs) {
					if _, composite := typed.Rhs[index].(*ast.CompositeLit); composite {
						filled = true
						linked = linked || expressionContainsGraphIndex(typed.Rhs[index], graph)
					}
				}
			}
		case *ast.ReturnStmt:
			returned = returned || len(typed.Results) == 1 && identName(typed.Results[0]) == graph
		}

		return true
	})

	return allocated, filled, linked, returned
}

// indexedGraph reports whether an expression indexes the selected graph variable.
func indexedGraph(expression ast.Expr, graph string) bool {
	index, ok := expression.(*ast.IndexExpr)

	return ok && identName(index.X) == graph
}

// isNewCall reports whether an expression allocates one graph node.
func isNewCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)

	return ok && identName(call.Fun) == "new"
}

// expressionContainsGraphIndex finds a graph edge inside an indexed composite assignment.
func expressionContainsGraphIndex(expression ast.Expr, graph string) bool {
	found := false

	ast.Inspect(expression, func(node ast.Node) bool {
		candidate, ok := node.(ast.Expr)
		if ok && indexedGraph(candidate, graph) {
			found = true

			return false
		}

		return true
	})

	return found
}

// identName returns an identifier's spelling or an empty string.
func identName(expression ast.Expr) string {
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return ""
	}

	return identifier.Name
}

// corpusOwnershipViolations performs package-wide typed ownership inspection from Build.
//
//nolint:cyclop // Package roots, private state graph, and typed function reachability form one ownership proof.
func corpusOwnershipViolations(guardPackage *sourceGuardPackage) []string {
	var violations []string

	caseType := packageObjectType(guardPackage, "Case")
	jsonValueType := packageObjectType(guardPackage, "jsonValue")

	for _, name := range guardPackage.pkg.Scope().Names() {
		variable, ok := guardPackage.pkg.Scope().Lookup(name).(*types.Var)
		if ok && forbiddenOwnedCollection(
			variable.Type(), variable.Name(), caseType, jsonValueType, nil, true, true,
		) {
			violations = append(violations, "package corpus owner: "+variable.Name())
		}
	}

	functions := guardFunctions(guardPackage)
	buildObject := guardPackage.pkg.Scope().Lookup("Build")

	build, ok := buildObject.(*types.Func)
	if !ok {
		return append(violations, "Build function is missing from typed package")
	}

	reachable := reachableGuardFunctions(guardPackage, functions, build)
	runtimeFunctions := runtimeGuardFunctions(guardPackage, functions, build)
	streaming := streamingGuardFunctions(guardPackage, functions, reachable, build)
	ownerTypes := reachableOwnerTypes(guardPackage, functions, runtimeFunctions)
	authorized := authorizedOwnerTypes(guardPackage)

	for owner := range ownerTypes {
		structure, structureOK := types.Unalias(owner.Type()).Underlying().(*types.Struct)
		if !structureOK {
			continue
		}

		for index := range structure.NumFields() {
			field := structure.Field(index)
			if forbiddenOwnedCollection(
				field.Type(), field.Name(), caseType, jsonValueType, authorized,
				!authorized[owner], !authorized[owner],
			) {
				violations = append(violations, "corpus-bearing field: "+owner.Name()+"."+field.Name())
			}
		}
	}

	for function := range reachable {
		declaration := functions[function]
		if declaration == nil {
			continue
		}

		violations = append(violations, reachableLocalOwnershipViolations(
			guardPackage, declaration, runtimeFunctions[function], streaming[function],
			authorized, caseType, jsonValueType,
		)...)
	}

	return violations
}

// reachableGuardFunctions computes the complete same-package call graph below Build.
func reachableGuardFunctions(
	guardPackage *sourceGuardPackage,
	functions map[*types.Func]*ast.FuncDecl,
	build *types.Func,
) map[*types.Func]bool {
	reachable := make(map[*types.Func]bool)

	pending := []*types.Func{build}
	for len(pending) > 0 {
		function := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		if function == nil || reachable[function] {
			continue
		}

		reachable[function] = true

		declaration := functions[function]
		if declaration == nil {
			continue
		}

		pending = append(pending, calledGuardFunctions(guardPackage, declaration.Body)...)
	}

	return reachable
}

// runtimeGuardFunctions identifies Build execution excluding only explicit admission and planning roots.
func runtimeGuardFunctions(
	guardPackage *sourceGuardPackage,
	functions map[*types.Func]*ast.FuncDecl,
	build *types.Func,
) map[*types.Func]bool {
	runtimeFunctions := map[*types.Func]bool{build: true}

	declaration := functions[build]
	if declaration == nil {
		return runtimeFunctions
	}

	pending := slices.DeleteFunc(calledGuardFunctions(guardPackage, declaration.Body), func(function *types.Func) bool {
		return function.Name() == "parseInput" || function.Name() == "makePlan"
	})
	for len(pending) > 0 {
		function := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		if runtimeFunctions[function] {
			continue
		}

		runtimeFunctions[function] = true

		calledDeclaration := functions[function]
		if calledDeclaration != nil {
			pending = append(pending, calledGuardFunctions(guardPackage, calledDeclaration.Body)...)
		}
	}

	return runtimeFunctions
}

// streamingGuardFunctions propagates callback ownership through Build-reachable call arguments.
//
//nolint:cyclop,gocognit // Callback dataflow reaches parameters through calls and captured closure arguments.
func streamingGuardFunctions(
	guardPackage *sourceGuardPackage,
	functions map[*types.Func]*ast.FuncDecl,
	reachable map[*types.Func]bool,
	build *types.Func,
) map[*types.Func]bool {
	callbacks := make(map[*types.Func]map[*types.Var]bool)
	callbacks[build] = functionTypedParameters(build)

	changed := true
	for changed {
		changed = false

		for function, callbackVariables := range callbacks {
			declaration := functions[function]
			if declaration == nil {
				continue
			}

			callbackVariables = expandedCallbackVariables(guardPackage, declaration.Body, callbackVariables)
			callbacks[function] = callbackVariables

			ast.Inspect(declaration.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}

				called := calledGuardFunction(guardPackage, call)
				if called == nil || !reachable[called] {
					return true
				}

				signature, ok := called.Type().(*types.Signature)
				if !ok {
					return true
				}

				for index, argument := range call.Args {
					if index >= signature.Params().Len() || !expressionUsesVariables(
						guardPackage, argument, callbackVariables,
					) {
						continue
					}

					if callbacks[called] == nil {
						callbacks[called] = make(map[*types.Var]bool)
					}

					parameter := signature.Params().At(index)
					if !callbacks[called][parameter] {
						callbacks[called][parameter] = true
						changed = true
					}
				}

				return true
			})
		}
	}

	streaming := make(map[*types.Func]bool, len(callbacks))
	for function := range callbacks {
		streaming[function] = true
	}

	return streaming
}

// expandedCallbackVariables follows callback identity through local declarations and assignments.
//
//nolint:cyclop,gocognit // Declarations and assignments share one callback-alias fixed-point pass.
func expandedCallbackVariables(
	guardPackage *sourceGuardPackage,
	body *ast.BlockStmt,
	initial map[*types.Var]bool,
) map[*types.Var]bool {
	variables := maps.Clone(initial)

	changed := true
	for changed {
		changed = false

		ast.Inspect(body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.AssignStmt:
				for index, right := range typed.Rhs {
					if index < len(typed.Lhs) && expressionUsesVariables(guardPackage, right, variables) {
						changed = addCallbackVariable(
							assignedGuardVariable(guardPackage, typed.Lhs[index]), variables,
						) || changed
					}
				}
			case *ast.ValueSpec:
				for index, right := range typed.Values {
					if index < len(typed.Names) && expressionUsesVariables(guardPackage, right, variables) {
						variable, variableOK := guardPackage.info.Defs[typed.Names[index]].(*types.Var)
						if variableOK {
							changed = addCallbackVariable(variable, variables) || changed
						}
					}
				}
			}

			return true
		})
	}

	return variables
}

// addCallbackVariable records one newly discovered function-typed callback alias.
func addCallbackVariable(variable *types.Var, variables map[*types.Var]bool) bool {
	if variable == nil || !guardVariableIsCallback(variable) || variables[variable] {
		return false
	}

	variables[variable] = true

	return true
}

// assignedGuardVariable resolves one declaration or assignment target.
func assignedGuardVariable(guardPackage *sourceGuardPackage, expression ast.Expr) *types.Var {
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return nil
	}

	if variable, defined := guardPackage.info.Defs[identifier].(*types.Var); defined {
		return variable
	}

	variable, ok := guardPackage.info.Uses[identifier].(*types.Var)
	if !ok {
		return nil
	}

	return variable
}

// guardVariableIsCallback reports whether a local can carry callback identity.
func guardVariableIsCallback(variable *types.Var) bool {
	_, callback := types.Unalias(variable.Type()).Underlying().(*types.Signature)

	return callback
}

// functionTypedParameters returns Build's function-typed callback parameters.
func functionTypedParameters(function *types.Func) map[*types.Var]bool {
	parameters := make(map[*types.Var]bool)

	signature, ok := function.Type().(*types.Signature)
	if !ok {
		return parameters
	}

	for index := range signature.Params().Len() {
		parameter := signature.Params().At(index)
		if _, callback := types.Unalias(parameter.Type()).Underlying().(*types.Signature); callback {
			parameters[parameter] = true
		}
	}

	return parameters
}

// calledGuardFunction resolves one direct same-package call.
func calledGuardFunction(guardPackage *sourceGuardPackage, call *ast.CallExpr) *types.Func {
	var object types.Object

	switch called := call.Fun.(type) {
	case *ast.Ident:
		object = guardPackage.info.Uses[called]
	case *ast.SelectorExpr:
		object = guardPackage.info.Uses[called.Sel]
	}

	function, ok := object.(*types.Func)
	if !ok || function.Pkg() != guardPackage.pkg {
		return nil
	}

	return function
}

// expressionUsesVariables reports whether an argument forwards a callback variable or captures it in a closure.
func expressionUsesVariables(
	guardPackage *sourceGuardPackage,
	expression ast.Expr,
	variables map[*types.Var]bool,
) bool {
	found := false

	ast.Inspect(expression, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok {
			variable, variableOK := guardPackage.info.Uses[identifier].(*types.Var)
			if variableOK && variables[variable] {
				found = true

				return false
			}
		}

		return true
	})

	return found
}

// reachableOwnerTypes follows state allocated by every Build runtime function.
func reachableOwnerTypes(
	guardPackage *sourceGuardPackage,
	functions map[*types.Func]*ast.FuncDecl,
	runtimeFunctions map[*types.Func]bool,
) map[*types.TypeName]bool {
	owned := make(map[*types.TypeName]bool)

	for function := range runtimeFunctions {
		declaration := functions[function]
		if declaration == nil {
			continue
		}

		ast.Inspect(declaration.Body, func(node ast.Node) bool {
			identifier, identifierOK := node.(*ast.Ident)
			if !identifierOK {
				return true
			}

			variable, variableOK := guardPackage.info.Defs[identifier].(*types.Var)
			if variableOK {
				collectNamedOwnerTypes(variable.Type(), owned)
			}

			return true
		})
	}

	return owned
}

// authorizedOwnerTypes identifies parsed model, plan, current-value traversal, and bounded search state graphs.
func authorizedOwnerTypes(guardPackage *sourceGuardPackage) map[*types.TypeName]bool {
	authorized := make(map[*types.TypeName]bool)
	for _, root := range []string{
		"schemaModel", "searchPlan", "jsonValue", "search", "evaluationContext",
		"jsonActivePath", "jsonValuePair", "jsonValidationFrame", "jsonCloneFrame", "jsonMarshalFrame",
		"strictJSONContainerFrame",
	} {
		collectNamedOwnerTypes(packageObjectType(guardPackage, root), authorized)
	}

	return authorized
}

// collectNamedOwnerTypes recursively follows fields from one named Build-owned type.
func collectNamedOwnerTypes(owned types.Type, found map[*types.TypeName]bool) {
	owned = types.Unalias(owned)
	switch typed := owned.(type) {
	case *types.Pointer:
		collectNamedOwnerTypes(typed.Elem(), found)
	case *types.Named:
		object := typed.Obj()
		if found[object] {
			return
		}

		found[object] = true

		structure, ok := typed.Underlying().(*types.Struct)
		if !ok {
			return
		}

		for index := range structure.NumFields() {
			collectNamedOwnerTypes(structure.Field(index).Type(), found)
		}
	case *types.Slice:
		collectNamedOwnerTypes(typed.Elem(), found)
	case *types.Array:
		collectNamedOwnerTypes(typed.Elem(), found)
	case *types.Map:
		collectNamedOwnerTypes(typed.Key(), found)
		collectNamedOwnerTypes(typed.Elem(), found)
	}
}

// guardFunctions indexes package function declarations by their typed objects.
func guardFunctions(guardPackage *sourceGuardPackage) map[*types.Func]*ast.FuncDecl {
	functions := make(map[*types.Func]*ast.FuncDecl)

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}

			object, ok := guardPackage.info.Defs[function.Name].(*types.Func)
			if ok {
				functions[object] = function
			}
		}
	}

	return functions
}

// calledGuardFunctions returns typed same-package calls from one function body.
func calledGuardFunctions(guardPackage *sourceGuardPackage, body *ast.BlockStmt) []*types.Func {
	var functions []*types.Func

	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		var object types.Object

		switch called := call.Fun.(type) {
		case *ast.Ident:
			object = guardPackage.info.Uses[called]
		case *ast.SelectorExpr:
			object = guardPackage.info.Uses[called.Sel]
		}

		function, ok := object.(*types.Func)
		if ok && function.Pkg() == guardPackage.pkg {
			functions = append(functions, function)
		}

		return true
	})

	return functions
}

// reachableLocalOwnershipViolations inspects explicit, inferred, aliased, and closure-captured locals.
func reachableLocalOwnershipViolations(
	guardPackage *sourceGuardPackage,
	declaration *ast.FuncDecl,
	runtimeFunction bool,
	streaming bool,
	authorized map[*types.TypeName]bool,
	caseType,
	jsonValueType types.Type,
) []string {
	var violations []string

	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok {
			return true
		}

		variable, ok := guardPackage.info.Defs[identifier].(*types.Var)
		if !ok {
			return true
		}

		activeSearchLocal := !streaming && authorizedActiveLocalName(variable.Name())

		checkGeneratedValues := runtimeFunction && !activeSearchLocal
		if !guardTypeIsAuthorized(variable.Type(), authorized) && forbiddenOwnedCollection(
			variable.Type(), variable.Name(), caseType, jsonValueType, authorized,
			runtimeFunction && !activeSearchLocal, checkGeneratedValues,
		) {
			violations = append(violations, "reachable local corpus: "+variable.Name())
		}

		return true
	})

	return violations
}

// authorizedActiveLocalName identifies bounded scalar frontiers that are live only during active search.
func authorizedActiveLocalName(name string) bool {
	lower := strings.ToLower(name)

	for _, category := range []string{
		"admitted", "candidate", "generated", "witness", "parentpins", "parenttokens", "canonical",
		"derived", "edits", "elements", "filtered", "seeded", "selected", "values",
	} {
		if strings.Contains(lower, category) {
			return true
		}
	}

	return false
}

// guardTypeIsAuthorized reports whether a local is one explicit model, plan, current-value, or search category.
func guardTypeIsAuthorized(owned types.Type, authorized map[*types.TypeName]bool) bool {
	owned = types.Unalias(owned)
	if pointer, ok := owned.(*types.Pointer); ok {
		owned = types.Unalias(pointer.Elem())
	}

	named, ok := owned.(*types.Named)

	return ok && authorized[named.Obj()]
}

// forbiddenOwnedCollection classifies corpus ownership through named and aliased collection types.
func forbiddenOwnedCollection(
	owned types.Type,
	name string,
	caseType,
	jsonValueType types.Type,
	authorized map[*types.TypeName]bool,
	checkCorpusCategory bool,
	checkGeneratedValues bool,
) bool {
	if _, collection := collectionElementTypes(owned); collection &&
		checkCorpusCategory && forbiddenCorpusCategoryName(name) {
		return true
	}

	return recursivelyOwnsForbidden(
		owned, caseType, jsonValueType, authorized, checkCorpusCategory, checkGeneratedValues,
		0, false, make(map[types.Type]bool),
	)
}

// recursivelyOwnsForbidden traverses nested collections, wrappers, aliases, pointers, and value forms.
//
//nolint:cyclop // Every Go ownership type form must participate in the recursive classification.
func recursivelyOwnsForbidden(
	owned,
	caseType,
	jsonValueType types.Type,
	authorized map[*types.TypeName]bool,
	checkCorpusCategory,
	checkGeneratedValues bool,
	collectionDepth int,
	directCollectionElement bool,
	visiting map[types.Type]bool,
) bool {
	owned = types.Unalias(owned)
	if collectionDepth > 0 && sameGuardType(owned, caseType) {
		return true
	}

	if collectionDepth > 0 && checkGeneratedValues && sameGuardType(owned, jsonValueType) {
		return true
	}

	if named, ok := owned.(*types.Named); ok && authorized[named.Obj()] {
		return false
	}

	if visiting[owned] {
		return false
	}

	visiting[owned] = true
	defer delete(visiting, owned)

	switch typed := owned.Underlying().(type) {
	case *types.Basic:
		return collectionDepth > 1 && directCollectionElement && typed.Kind() == types.Byte
	case *types.Pointer:
		return recursivelyOwnsForbidden(
			typed.Elem(), caseType, jsonValueType, authorized, checkCorpusCategory, checkGeneratedValues,
			collectionDepth, directCollectionElement, visiting,
		)
	case *types.Slice:
		return recursivelyOwnsForbidden(
			typed.Elem(), caseType, jsonValueType, authorized, checkCorpusCategory, checkGeneratedValues,
			collectionDepth+1, true, visiting,
		)
	case *types.Array:
		return recursivelyOwnsForbidden(
			typed.Elem(), caseType, jsonValueType, authorized, checkCorpusCategory, checkGeneratedValues,
			collectionDepth+1, true, visiting,
		)
	case *types.Map:
		return recursivelyOwnsForbidden(
			typed.Key(), caseType, jsonValueType, authorized, checkCorpusCategory, checkGeneratedValues,
			collectionDepth+1, true, visiting,
		) || recursivelyOwnsForbidden(
			typed.Elem(), caseType, jsonValueType, authorized, checkCorpusCategory, checkGeneratedValues,
			collectionDepth+1, true, visiting,
		)
	case *types.Chan:
		return recursivelyOwnsForbidden(
			typed.Elem(), caseType, jsonValueType, authorized, checkCorpusCategory, checkGeneratedValues,
			collectionDepth+1, true, visiting,
		)
	case *types.Struct:
		for index := range typed.NumFields() {
			field := typed.Field(index)
			if _, collection := collectionElementTypes(field.Type()); collection &&
				checkCorpusCategory && forbiddenCorpusCategoryName(field.Name()) {
				return true
			}

			if recursivelyOwnsForbidden(
				field.Type(), caseType, jsonValueType, authorized,
				checkCorpusCategory, checkGeneratedValues, collectionDepth, false, visiting,
			) {
				return true
			}
		}
	}

	return false
}

// forbiddenCorpusCategoryName identifies prohibited corpus ownership roles in streaming state.
func forbiddenCorpusCategoryName(name string) bool {
	lower := strings.ToLower(name)
	for _, category := range []string{
		"corpus", "case", "output", "parent", "derivative", "pair", "visited", "witness", "guidance",
	} {
		if strings.Contains(lower, category) {
			return true
		}
	}

	return false
}

// collectionElementTypes unwraps named and aliased slices, arrays, maps, and channels.
func collectionElementTypes(owned types.Type) ([]types.Type, bool) {
	underlying := types.Unalias(owned).Underlying()
	switch typed := underlying.(type) {
	case *types.Slice:
		return []types.Type{typed.Elem()}, true
	case *types.Array:
		return []types.Type{typed.Elem()}, true
	case *types.Map:
		return []types.Type{typed.Key(), typed.Elem()}, true
	case *types.Chan:
		return []types.Type{typed.Elem()}, true
	default:
		return nil, false
	}
}

// sameGuardType compares types after resolving aliases.
func sameGuardType(left, right types.Type) bool {
	return left != nil && right != nil && types.Identical(types.Unalias(left), types.Unalias(right))
}

// packageObjectType returns one package-level type when present.
func packageObjectType(guardPackage *sourceGuardPackage, name string) types.Type {
	object := guardPackage.pkg.Scope().Lookup(name)
	if object == nil {
		return nil
	}

	return object.Type()
}

// canonicalCorpusOperationIDs derives selectors from every authoritative checked-in corpus document.
func canonicalCorpusOperationIDs(t *testing.T) map[string]bool {
	t.Helper()

	identifiers := make(map[string]bool)

	for _, path := range []string{
		"testdata/alpha_zeta.yaml",
		"testdata/request_bodies.yaml",
		"../../resources/openapi.yaml",
	} {
		document, err := os.ReadFile(path)
		require.NoError(t, err)

		found, decodeErr := operationIDsFromCorpusDocument(document)
		require.NoError(t, decodeErr)

		for identifier := range found {
			identifiers[identifier] = true
		}
	}

	return identifiers
}

// operationIDsFromCorpusDocument semantically extracts JSON request-body selectors from YAML.
func operationIDsFromCorpusDocument(document []byte) (map[string]bool, error) {
	var root map[string]any
	if err := yaml.Unmarshal(document, &root); err != nil {
		return nil, err
	}

	paths, err := corpusObject(root, "paths")
	if err != nil {
		return nil, err
	}

	identifiers := make(map[string]bool)

	for path, pathValue := range paths {
		pathItem, ok := pathValue.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("corpus path %q is not an object", path)
		}

		for _, method := range []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"} {
			operationValue, exists := pathItem[method]
			if !exists {
				continue
			}

			operation, ok := operationValue.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("corpus operation %s %s is not an object", method, path)
			}

			operationID, found, operationErr := corpusJSONBodyOperationID(operation)
			if operationErr != nil {
				return nil, fmt.Errorf("corpus operation %s %s: %w", method, path, operationErr)
			}

			if found {
				identifiers[operationID] = true
			}
		}
	}

	return identifiers, nil
}

// corpusJSONBodyOperationID returns one operation ID only for a concrete JSON request-body schema.
func corpusJSONBodyOperationID(operation map[string]any) (string, bool, error) {
	requestBodyValue, exists := operation["requestBody"]
	if !exists {
		return "", false, nil
	}

	requestBody, ok := requestBodyValue.(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("requestBody is not an object")
	}

	content, err := corpusObject(requestBody, "content")
	if err != nil {
		return "", false, err
	}

	mediaValue, exists := content["application/json"]
	if !exists {
		return "", false, nil
	}

	media, ok := mediaValue.(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("application/json media is not an object")
	}

	if _, exists := media["schema"]; !exists {
		return "", false, nil
	}

	operationID, ok := operation["operationId"].(string)
	if !ok || operationID == "" {
		return "", false, fmt.Errorf("JSON request-body operationId is not a non-empty string")
	}

	return operationID, true, nil
}

// corpusObject reads one required YAML object member without ignoring shape errors.
func corpusObject(parent map[string]any, name string) (map[string]any, error) {
	value, exists := parent[name]
	if !exists {
		return nil, fmt.Errorf("%s is missing", name)
	}

	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not an object", name)
	}

	return object, nil
}

// topLevelExportedNames returns exported package-scope declarations, excluding methods.
//
//nolint:cyclop // Functions, types, constants, and variables comprise the package declaration surface.
func topLevelExportedNames(file *ast.File) []string {
	var names []string

	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			if typed.Recv == nil && ast.IsExported(typed.Name.Name) {
				names = append(names, typed.Name.Name)
			}
		case *ast.GenDecl:
			for _, specification := range typed.Specs {
				switch spec := specification.(type) {
				case *ast.TypeSpec:
					if ast.IsExported(spec.Name.Name) {
						names = append(names, spec.Name.Name)
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if ast.IsExported(name.Name) {
							names = append(names, name.Name)
						}
					}
				}
			}
		}
	}

	return names
}

// sourceGuardPackage holds one package's shared syntax and semantic type information.
type sourceGuardPackage struct {
	files   []*ast.File
	fset    *token.FileSet
	pkg     *types.Package
	info    *types.Info
	sources map[*ast.File][]byte
}

// productionGuardPackage loads every production file into one semantic package.
func productionGuardPackage(t *testing.T) *sourceGuardPackage {
	t.Helper()

	sources := make(map[string]string)

	for _, file := range productionGoFiles(t) {
		source, err := os.ReadFile(file)
		require.NoError(t, err)

		sources[file] = string(source)
	}

	return parseGuardPackage(t, sources)
}

// parseGuardPackage parses and type-checks a package-wide synthetic or production source set.
func parseGuardPackage(t *testing.T, sources map[string]string) *sourceGuardPackage {
	t.Helper()

	fset := token.NewFileSet()

	filenames := make([]string, 0, len(sources))
	for filename := range sources {
		filenames = append(filenames, filename)
	}

	slices.Sort(filenames)

	guardPackage := &sourceGuardPackage{
		fset:    fset,
		sources: make(map[*ast.File][]byte, len(sources)),
		info: &types.Info{
			Types:      make(map[ast.Expr]types.TypeAndValue),
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
		},
	}
	for _, filename := range filenames {
		source := []byte(sources[filename])
		file, err := parser.ParseFile(fset, filename, source, 0)
		require.NoError(t, err)

		guardPackage.files = append(guardPackage.files, file)
		guardPackage.sources[file] = source
	}

	configuration := types.Config{Importer: moduleSourceImporter(fset)}
	checked, err := configuration.Check(
		"github.com/djosh34/klopt/pkg/schematest",
		fset,
		guardPackage.files,
		guardPackage.info,
	)
	require.NoError(t, err)

	guardPackage.pkg = checked

	return guardPackage
}

// moduleSourceImporter resolves module dependencies through their Go export data.
func moduleSourceImporter(fset *token.FileSet) types.Importer {
	cache := make(map[string]string)
	lookup := func(path string) (io.ReadCloser, error) {
		exportFile, found := cache[path]
		if !found {
			command := exec.Command("go", "list", "-export", "-f", "{{.Export}}", path)

			output, err := command.Output()
			if err != nil {
				return nil, fmt.Errorf("locate export data for %s: %w", path, err)
			}

			exportFile = strings.TrimSpace(string(output))
			if exportFile == "" {
				return nil, fmt.Errorf("locate export data for %s: empty path", path)
			}

			cache[path] = exportFile
		}

		reader, err := os.Open(exportFile)
		if err != nil {
			return nil, fmt.Errorf("open export data for %s: %w", path, err)
		}

		return reader, nil
	}

	return importer.ForCompiler(fset, "gc", lookup)
}

// publicMethodSetViolations resolves aliases before checking locked public method sets.
func publicMethodSetViolations(guardPackage *sourceGuardPackage) []string {
	var violations []string

	seen := make(map[*types.Func]bool)

	for _, name := range []string{"Input", "Case", "StopReason", "Report"} {
		object := guardPackage.pkg.Scope().Lookup(name)
		if object == nil {
			continue
		}

		for _, receiver := range []types.Type{object.Type(), types.NewPointer(object.Type())} {
			methods := types.NewMethodSet(receiver)
			for index := range methods.Len() {
				method, ok := methods.At(index).Obj().(*types.Func)
				if ok && method.Exported() && method.Pkg() == guardPackage.pkg && !seen[method] {
					seen[method] = true
					violations = append(violations, "exported method on public type: "+method.Name())
				}
			}
		}
	}

	return violations
}

// productionGoFiles lists package implementation files from the checked-out directory.
func productionGoFiles(t *testing.T) []string {
	t.Helper()

	_, current, _, ok := runtime.Caller(0)
	require.True(t, ok)

	files, err := filepath.Glob(filepath.Join(filepath.Dir(current), "*.go"))
	require.NoError(t, err)

	production := make([]string, 0, len(files))
	for _, file := range files {
		if !strings.HasSuffix(file, "_test.go") {
			production = append(production, file)
		}
	}

	return production
}

// parseGoFile parses production source for one source guard.
func parseGoFile(t *testing.T, file string) *ast.File {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	require.NoError(t, err)

	return parsed
}

// parseGoSource parses one synthetic negative/meta sample.
func parseGoSource(t *testing.T, source string) *ast.File {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), "meta.go", source, 0)
	require.NoError(t, err)

	return parsed
}
