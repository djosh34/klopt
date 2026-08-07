package schematest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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

// TestOnlyLockedDeclarationsArePublic keeps the complete public module interface unchanged.
func TestOnlyLockedDeclarationsArePublic(t *testing.T) {
	t.Parallel()

	files := productionASTs(t)
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
	require.Empty(t, publicAPIViolations(files))
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
		{name: "exported field", suffix: `type ExtraInput struct { Extra string }`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := base + test.suffix
			if test.name == "exported field" {
				source = strings.Replace(base, "MaxSteps uint64", "MaxSteps uint64; Extra string", 1)
			}

			require.NotEmpty(t, publicAPIViolations([]*ast.File{parseGoSource(t, source)}))
		})
	}
}

// TestProductionSourceDoesNotEmbedFixtureAnswers rejects source-specific oracle paths.
func TestProductionSourceDoesNotEmbedFixtureAnswers(t *testing.T) {
	t.Parallel()

	operationIDs := canonicalCorpusOperationIDs(t)
	for _, file := range productionGoFiles(t) {
		source, err := os.ReadFile(file)
		require.NoError(t, err)

		violations := copiedAnswerViolations(parseGoFile(t, file), source, operationIDs)
		require.Emptyf(t, violations, "%s contains copied or fixture-derived semantics: %v", file, violations)
	}
}

// TestCopiedAnswerGuardRejectsConcreteBypasses pins semantic selectors and renamed generated structures.
func TestCopiedAnswerGuardRejectsConcreteBypasses(t *testing.T) {
	t.Parallel()

	operationIDs := map[string]bool{"allOfObject": true}
	tests := []string{
		`package schematest; const suffix = "Object"; func f(input Input) { if input.OperationID == "allOf" + suffix {} }`,
		`package schematest; type Validation struct{}; type KindValidation struct{}; ` +
			`func renamed() []*Validation { return []*Validation{{}} }`,
	}

	for _, source := range tests {
		parsed := parseGoSource(t, source)
		require.NotEmpty(t, copiedAnswerViolations(parsed, []byte(source), operationIDs))
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

	violations := corpusOwnershipViolations(productionASTs(t)...)
	require.Emptyf(t, violations, "production contains corpus-bearing execution state: %v", violations)
}

// TestCorpusOwnershipGuardCoversLocalsClosuresAndPrivateTypes pins representative retention bypasses.
func TestCorpusOwnershipGuardCoversLocalsClosuresAndPrivateTypes(t *testing.T) {
	t.Parallel()

	tests := []string{
		`package schematest; type Case struct{}; func Build() { var cases []Case; ` +
			`_ = func(c Case) { cases = append(cases, c) } }`,
		`package schematest; type Case struct{}; func Build() { stream() }; func stream() { var saved []Case; _ = saved }`,
		`package schematest; type Case struct{}; type buildState struct { emitted []Case }`,
		`package schematest; type buildState struct { encoded [][]byte }`,
	}
	for _, source := range tests {
		require.NotEmpty(t, corpusOwnershipViolations(parseGoSource(t, source)))
	}

	harmless := parseGoSource(t, `package schematest; type buildState struct { counter uint64; path []int }`)
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

// copiedAnswerViolations finds semantic fixture constants and generated validation table shapes.
func copiedAnswerViolations(file *ast.File, source []byte, operationIDs map[string]bool) []string {
	var violations []string

	constants := stringConstants(file)

	ast.Inspect(file, func(node ast.Node) bool {
		expression, ok := node.(ast.Expr)
		if ok {
			if value, found := evaluateStringExpression(expression, constants); found && operationIDs[value] {
				violations = append(violations, "fixture operation selector: "+value)
			}
		}

		return true
	})

	text := string(source)
	for _, forbidden := range []string{"testdata/", "alpha_zeta", "request_bodies", "Code generated", "//go:generate"} {
		if strings.Contains(text, forbidden) {
			violations = append(violations, "forbidden source marker: "+forbidden)
		}
	}

	generatedVocabulary := map[string]bool{
		"RequestValidation": true,
		"Validation":        true,
		"KindValidation":    true,
		"EnumValidation":    true,
		"NumberValidation":  true,
		"StringValidation":  true,
	}
	seenGeneratedTypes := make(map[string]bool)

	ast.Inspect(file, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && generatedVocabulary[identifier.Name] {
			seenGeneratedTypes[identifier.Name] = true
		}

		return true
	})

	if len(seenGeneratedTypes) >= 2 {
		violations = append(violations, "copied generated validation structure")
	}

	return violations
}

// stringConstants resolves package constants used to hide fixture selectors.
//
//nolint:cyclop,gocognit // Declarations, grouped names, and expression resolution form one fixed-point pass.
func stringConstants(file *ast.File) map[string]string {
	constants := make(map[string]string)

	for range len(file.Decls) + 1 {
		changed := false

		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.CONST {
				continue
			}

			for _, specification := range general.Specs {
				values, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}

				for index, name := range values.Names {
					if index >= len(values.Values) {
						continue
					}

					value, found := evaluateStringExpression(values.Values[index], constants)
					if found && constants[name.Name] != value {
						constants[name.Name] = value
						changed = true
					}
				}
			}
		}

		if !changed {
			break
		}
	}

	return constants
}

// evaluateStringExpression evaluates literal, constant, parenthesized, and concatenated strings.
func evaluateStringExpression(expression ast.Expr, constants map[string]string) (string, bool) {
	switch typed := expression.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return "", false
		}

		value, err := strconv.Unquote(typed.Value)

		return value, err == nil
	case *ast.Ident:
		value, found := constants[typed.Name]

		return value, found
	case *ast.ParenExpr:
		return evaluateStringExpression(typed.X, constants)
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}

		left, leftOK := evaluateStringExpression(typed.X, constants)
		right, rightOK := evaluateStringExpression(typed.Y, constants)

		return left + right, leftOK && rightOK
	default:
		return "", false
	}
}

// corpusOwnershipViolations rejects collections capable of retaining emitted cases or encoded bodies.
func corpusOwnershipViolations(files ...*ast.File) []string {
	var violations []string

	functions := make(map[string][]*ast.FuncDecl)

	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.StructType:
				for _, field := range typed.Fields.List {
					for _, name := range field.Names {
						if forbiddenCorpusCollection(name.Name, field.Type) {
							violations = append(violations, "corpus-bearing field: "+name.Name)
						}
					}
				}
			case *ast.FuncDecl:
				functions[typed.Name.Name] = append(functions[typed.Name.Name], typed)
			}

			return true
		})
	}

	pending := []string{"Build"}
	reachable := make(map[*ast.FuncDecl]bool)

	for len(pending) > 0 {
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		for _, function := range functions[name] {
			if reachable[function] {
				continue
			}

			reachable[function] = true
			violations = append(violations, functionLocalCorpusViolations(function.Body)...)
			pending = append(pending, calledFunctionNames(function.Body)...)
		}
	}

	return violations
}

// calledFunctionNames returns direct package-function and method call names.
func calledFunctionNames(body *ast.BlockStmt) []string {
	var names []string

	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		switch function := call.Fun.(type) {
		case *ast.Ident:
			names = append(names, function.Name)
		case *ast.SelectorExpr:
			names = append(names, function.Sel.Name)
		}

		return true
	})

	return names
}

// functionLocalCorpusViolations finds emitted-value collections in one reachable function.
//
//nolint:cyclop // Explicit declarations and inferred make calls are the two local ownership forms.
func functionLocalCorpusViolations(body *ast.BlockStmt) []string {
	var violations []string

	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ValueSpec:
			for _, name := range typed.Names {
				if forbiddenEmittedCollection(typed.Type) {
					violations = append(violations, "reachable local corpus: "+name.Name)
				}
			}
		case *ast.AssignStmt:
			for index, left := range typed.Lhs {
				name, ok := left.(*ast.Ident)
				if !ok || index >= len(typed.Rhs) {
					continue
				}

				call, ok := typed.Rhs[index].(*ast.CallExpr)
				if ok && len(call.Args) > 0 && forbiddenEmittedCollection(call.Args[0]) {
					violations = append(violations, "reachable local corpus: "+name.Name)
				}
			}
		}

		return true
	})

	return violations
}

// forbiddenCorpusCollection identifies retained corpus categories by type or ownership name.
func forbiddenCorpusCollection(name string, expression ast.Expr) bool {
	if forbiddenEmittedCollection(expression) {
		return true
	}

	if expression == nil || !isCollectionType(expression) {
		return false
	}

	lower := strings.ToLower(name)
	for _, word := range []string{"corpus", "case", "parent", "derivative", "pair", "visited", "witness", "guidance"} {
		if strings.Contains(lower, word) {
			return true
		}
	}

	return false
}

// forbiddenEmittedCollection identifies Case and encoded-byte collections independent of names.
func forbiddenEmittedCollection(expression ast.Expr) bool {
	return expression != nil && isCollectionType(expression) &&
		(containsTypeName(expression, "Case") || containsEncodedByteCollection(expression))
}

// isCollectionType reports whether an expression directly owns multiple values.
func isCollectionType(expression ast.Expr) bool {
	switch expression.(type) {
	case *ast.ArrayType, *ast.MapType, *ast.ChanType:
		return true
	default:
		return false
	}
}

// containsTypeName reports whether a collection recursively contains a named type.
func containsTypeName(expression ast.Expr, wanted string) bool {
	found := false

	ast.Inspect(expression, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && identifier.Name == wanted {
			found = true

			return false
		}

		return true
	})

	return found
}

// containsEncodedByteCollection reports whether a type directly owns multiple byte slices.
func containsEncodedByteCollection(expression ast.Expr) bool {
	outer, ok := expression.(*ast.ArrayType)
	if !ok || outer.Len != nil {
		return false
	}

	inner, ok := outer.Elt.(*ast.ArrayType)

	return ok && inner.Len == nil && expressionName(inner.Elt) == "byte"
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

// productionASTs parses the complete production package for cross-file ownership checks.
func productionASTs(t *testing.T) []*ast.File {
	t.Helper()

	files := productionGoFiles(t)

	parsed := make([]*ast.File, 0, len(files))
	for _, file := range files {
		parsed = append(parsed, parseGoFile(t, file))
	}

	return parsed
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
