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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := base + test.suffix
			if test.name == "exported field" {
				source = strings.Replace(base, "MaxSteps uint64", "MaxSteps uint64; Extra string", 1)
			}

			guardPackage := parseGuardPackage(t, map[string]string{"meta.go": source})
			require.NotEmpty(t, append(
				publicAPIViolations(guardPackage.files),
				publicMethodSetViolations(guardPackage)...,
			))
		})
	}
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
		`package schematest; type Case struct{}; type caseList []Case; var saved caseList; func Build() {}`,
		`package schematest; type Case struct{}; type caseList = []Case; var saved caseList; func Build() {}`,
		`package schematest; type Case struct{}; func Build() { var saved []Case; ` +
			`_ = func(c Case) { saved = append(saved, c) } }`,
		`package schematest; type jsonValue struct{}; func Build() { parents := []*jsonValue{}; _ = parents }`,
		`package schematest; type jsonValue struct{}; type buildState struct { outputs []*jsonValue }; ` +
			`func Build() { state := new(buildState); _ = state }`,
		`package schematest; type buildState struct { encoded [][]byte }; ` +
			`func Build() { state := new(buildState); _ = state }`,
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
	if !ok || expressionName(stopReason.Type) != "string" {
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
	if specification == nil {
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
		if ok && forbiddenOwnedCollection(variable.Type(), caseType, jsonValueType, true) {
			violations = append(violations, "package corpus owner: "+variable.Name())
		}
	}

	ownerTypes := buildOwnedTypes(guardPackage)
	for owner := range ownerTypes {
		structure, ok := types.Unalias(owner.Type()).Underlying().(*types.Struct)
		if !ok {
			continue
		}

		for index := range structure.NumFields() {
			field := structure.Field(index)

			allowModelValue := owner.Name() == "jsonValue" || owner.Name() == "schemaShape"
			if forbiddenOwnedCollection(field.Type(), caseType, jsonValueType, !allowModelValue) {
				violations = append(violations, "corpus-bearing field: "+owner.Name()+"."+field.Name())
			}
		}
	}

	functions := guardFunctions(guardPackage)
	buildObject := guardPackage.pkg.Scope().Lookup("Build")

	build, ok := buildObject.(*types.Func)
	if !ok {
		return append(violations, "Build function is missing from typed package")
	}

	pending := []*types.Func{build}
	reachable := make(map[*types.Func]bool)

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

		violations = append(violations, reachableLocalOwnershipViolations(
			guardPackage, declaration, function == build, caseType, jsonValueType,
		)...)

		called := calledGuardFunctions(guardPackage, declaration.Body)
		if function == build {
			called = slices.DeleteFunc(called, func(candidate *types.Func) bool {
				return candidate.Name() == "parseInput" || candidate.Name() == "makePlan"
			})
		}

		pending = append(pending, called...)
	}

	return violations
}

// buildOwnedTypes follows named private state types rooted in Build's local ownership.
func buildOwnedTypes(guardPackage *sourceGuardPackage) map[*types.TypeName]bool {
	owned := make(map[*types.TypeName]bool)

	buildObject, ok := guardPackage.pkg.Scope().Lookup("Build").(*types.Func)
	if !ok {
		return owned
	}

	declaration := guardFunctions(guardPackage)[buildObject]
	if declaration == nil {
		return owned
	}

	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok {
			return true
		}

		variable, ok := guardPackage.info.Defs[identifier].(*types.Var)
		if ok {
			collectNamedOwnerTypes(variable.Type(), owned)
		}

		return true
	})

	return owned
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
	build bool,
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

		if forbiddenOwnedCollection(variable.Type(), caseType, jsonValueType, build) {
			violations = append(violations, "reachable local corpus: "+variable.Name())
		}

		return true
	})

	return violations
}

// forbiddenOwnedCollection classifies corpus ownership through named and aliased collection types.
func forbiddenOwnedCollection(
	owned types.Type,
	caseType,
	jsonValueType types.Type,
	checkGeneratedValues bool,
) bool {
	elements, collection := collectionElementTypes(owned)
	if !collection {
		return false
	}

	for _, element := range elements {
		if sameGuardType(element, caseType) || isByteCollection(element) ||
			checkGeneratedValues && isPointerToGuardType(element, jsonValueType) {
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

// isByteCollection reports whether one collection element is itself a byte collection.
func isByteCollection(owned types.Type) bool {
	elements, collection := collectionElementTypes(owned)

	return collection && len(elements) == 1 && sameGuardType(elements[0], types.Typ[types.Byte])
}

// isPointerToGuardType reports whether a collection element points to one package type.
func isPointerToGuardType(owned, wanted types.Type) bool {
	pointer, ok := types.Unalias(owned).Underlying().(*types.Pointer)

	return ok && sameGuardType(pointer.Elem(), wanted)
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
