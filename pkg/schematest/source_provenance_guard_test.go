//nolint:cyclop,gocognit,gocyclo,godoclint,lll // Whole-program provenance checks require complete SSA and graph traversals.
package schematest

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/tools/go/ssa" //nolint:depguard // SSA is required for the source-local whole-program guard.

	"github.com/stretchr/testify/require"
)

func TestOperationIDFlowStaysInsideOperationSelection(t *testing.T) {
	t.Parallel()

	require.Empty(t, operationIDFlowViolations(productionGuardPackage(t)))
}

func TestOperationIDFlowGuardRejectsSourceSpecificBranching(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type Input struct { OperationID string }
		func Build(input Input) bool { return sourceSpecific(input.OperationID) }
		func sourceSpecific(operationID string) bool {
			return len(operationID) >= 5 && operationID[:5] == "admin"
		}
	`

	violations := operationIDFlowViolations(parseGuardPackage(t, map[string]string{"branch.go": source}))
	require.NotEmpty(t, violations)
}

func TestOperationIDFlowGuardRejectsTrustedFunctionBypasses(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"parse prefix": `package schematest
			type Input struct { OperationID string }
			func Build(input Input) bool { return parseInput(input) }
			func parseInput(input Input) bool { return len(input.OperationID) > 2 && input.OperationID[:2] == "x-" }`,
		"selection constant equality": `package schematest
			type Input struct { OperationID string }
			func Build(input Input) bool { return selectRequestSchema(input.OperationID) }
			func selectRequestSchema(operationID string) bool { return operationID == "copiedFixtureOperation" }`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"guard.go": source})))
		})
	}
}

func TestOperationIDFlowGuardRejectsLookupAndComputedEquality(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"verdict lookup": `package schematest
			type Input struct { OperationID string }
			func Build(input Input) bool { verdicts := map[string]bool{"copied": true}; return verdicts[input.OperationID] }`,
		"package equality": `package schematest
			type Input struct { OperationID string }; var copied = "copied"
			func Build(input Input) bool { return input.OperationID == copied }`,
		"computed equality": `package schematest
			type Input struct { OperationID string }
			func Build(input Input) bool { copied := "co" + input.OperationID[:0] + "pied"; return input.OperationID == copied }`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"guard.go": source})))
		})
	}
}

func TestOperationIDFlowGuardAllowsNormalSelection(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type Input struct { OperationID string }
		type schemaModel struct{}
		func Build(input Input) *schemaModel { return parseInput(input) }
		func parseInput(input Input) *schemaModel { return selectRequestSchema(input.OperationID) }
		func selectRequestSchema(operationID string) *schemaModel {
			for _, authored := range []string{"first", "second"} {
				if authored == operationID { return new(schemaModel) }
			}
			return nil
		}
	`

	require.Empty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"selection.go": source})))
}

func TestCopiedSemanticGuardRejectsNamedHelperTable(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type answer struct { keyword string; valid bool }
		var renamed = copiedTable()
		func copiedTable() []answer {
			return []answer{{keyword: "type", valid: true}, {keyword: "required", valid: false}}
		}
	`

	guardPackage := parseGuardPackage(t, map[string]string{"answers.go": source})
	require.NotEmpty(t, copiedAnswerViolations(guardPackage, nil))
}

func TestCopiedSemanticGuardRejectsNamedHelperGraph(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type rule struct { children []*rule; answer string }
		var answers = copiedAnswers()
		func copiedAnswers() []*rule {
			rows := make([]*rule, 2)
			for index := range rows { rows[index] = new(rule) }
			*rows[0] = rule{children: []*rule{rows[1]}, answer: "object"}
			*rows[1] = rule{answer: "string"}
			return rows
		}
	`

	guardPackage := parseGuardPackage(t, map[string]string{"answers.go": source})
	require.NotEmpty(t, copiedAnswerViolations(guardPackage, nil))
}

func TestCopiedSemanticGuardRejectsIdentityAndShapeBypasses(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"approved name wrong role": `package schematest
			type answer struct { keyword string; valid bool }
			var operationMethods = []answer{{keyword: "type", valid: true}, {keyword: "enum", valid: false}}`,
		"approved name copied data": `package schematest
			var operationMethods = []string{"copiedAnswer"}`,
		"numeric boolean answer table": `package schematest
			type answer struct { code int; valid bool }
			var renamed = []answer{{code: 1, valid: true}, {code: 2, valid: false}}`,
		"local helper return": `package schematest
			type answer struct { keyword string; valid bool }
			func copied() []answer { return []answer{{keyword: "type", valid: true}, {keyword: "enum", valid: false}} }
			func Build() { _ = copied() }`,
		"one element helper chain": `package schematest
			type answer struct { valid bool }
			func copied() []answer { rows := []answer{{valid: true}}; return rows }
			func forwarded() []answer { return copied() }
			func Build() { _ = forwarded() }`,
		"map key and value": `package schematest
			var renamed = map[string]bool{"copied": true}`,
		"non fingerprint graph": `package schematest
			type node struct { next *node; answer string }
			var copied = &node{answer: "object", next: &node{answer: "string"}}`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, copiedAnswerViolations(parseGuardPackage(t, map[string]string{"guard.go": source}), nil))
		})
	}
}

func TestCopiedSemanticGuardAllowsIndependentGenericSpecifications(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type runeRange struct { first, last rune }
		var unicodeGrammar = []runeRange{{first: 'a', last: 'z'}, {first: 0x80, last: 0x10ffff}}
		type transition struct { state, next uint8; class runeRange }
		var formatTransitions = []transition{{state: 0, next: 1, class: unicodeGrammar[0]}}
	`

	guardPackage := parseGuardPackage(t, map[string]string{"grammar.go": source})
	require.Empty(t, copiedAnswerViolations(guardPackage, nil))
}

func operationIDFlowViolations(guardPackage *sourceGuardPackage) []string {
	ssaPackage := buildGuardSSA(guardPackage)

	build := ssaPackage.Func("Build")
	if build == nil {
		return nil
	}

	functions := operationIDRuntimeFunctions(build)
	tainted := make(map[ssa.Value]bool)
	returned := make(map[*ssa.Function]bool)

	for function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				value, ok := instruction.(ssa.Value)
				if ok && operationIDField(value, guardPackage.pkg) {
					tainted[value] = true
				}
			}
		}
	}

	for changed := true; changed; {
		changed = false

		for function := range functions {
			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					call, isCall := instruction.(ssa.CallInstruction)
					if isCall {
						callee := call.Common().StaticCallee()
						if callee != nil && functions[callee] {
							for index, argument := range call.Common().Args {
								if index < len(callee.Params) && tainted[argument] && !tainted[callee.Params[index]] {
									tainted[callee.Params[index]] = true
									changed = true
								}
							}
						}
					}

					if result, ok := instruction.(*ssa.Return); ok {
						for _, value := range result.Results {
							if tainted[value] && !returned[function] {
								returned[function] = true
								changed = true
							}
						}
					}

					value, ok := instruction.(ssa.Value)
					if !ok || tainted[value] {
						continue
					}

					if binary, binaryOK := instruction.(*ssa.BinOp); binaryOK &&
						operationIDSelectionComparison(function, binary, tainted) {
						continue
					}

					if isCall {
						callee := call.Common().StaticCallee()
						if callee != nil && functions[callee] && !returned[callee] {
							continue
						}
					}

					for _, operand := range instruction.Operands(nil) {
						if operand != nil && tainted[*operand] {
							tainted[value] = true
							changed = true

							break
						}
					}
				}
			}
		}
	}

	var violations []string

	for function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				for _, operand := range instruction.Operands(nil) {
					if operand == nil || !tainted[*operand] || operationIDAllowedUse(function, instruction, tainted) {
						continue
					}

					violations = append(violations, generatedValuePosition(instruction)+": OperationID escapes operation selection: "+instruction.String())

					break
				}
			}
		}
	}

	slices.Sort(violations)

	return slices.Compact(violations)
}

func operationIDAllowedUse(
	function *ssa.Function,
	instruction ssa.Instruction,
	tainted map[ssa.Value]bool,
) bool {
	switch typed := instruction.(type) {
	case *ssa.BinOp:
		return operationIDSelectionComparison(function, typed, tainted)
	case *ssa.Phi, *ssa.ChangeType, *ssa.Convert, *ssa.MakeInterface, *ssa.Return, *ssa.UnOp:
		return true
	case *ssa.Store:
		return operationIDLocalAddress(typed.Addr)
	}

	call, ok := instruction.(ssa.CallInstruction)
	if !ok {
		return false
	}

	callee := call.Common().StaticCallee()

	return callee != nil && callee.Pkg == function.Pkg
}

func operationIDSelectionComparison(
	function *ssa.Function,
	binary *ssa.BinOp,
	tainted map[ssa.Value]bool,
) bool {
	if function.Name() != "selectRequestSchema" || binary.Op != token.EQL {
		return false
	}

	var authored ssa.Value

	switch {
	case tainted[binary.X] && !tainted[binary.Y]:
		authored = binary.Y
	case tainted[binary.Y] && !tainted[binary.X]:
		authored = binary.X
	default:
		return false
	}

	return operationIDAuthoredSelectionValue(authored, make(map[ssa.Value]bool))
}

func operationIDAuthoredSelectionValue(value ssa.Value, seen map[ssa.Value]bool) bool {
	if value == nil || seen[value] {
		return false
	}

	seen[value] = true

	switch typed := value.(type) {
	case *ssa.Field, *ssa.Extract, *ssa.Index, *ssa.Lookup:
		return true
	case *ssa.UnOp:
		return operationIDAuthoredSelectionValue(typed.X, seen)
	case *ssa.FieldAddr:
		return operationIDAuthoredSelectionValue(typed.X, seen)
	case *ssa.IndexAddr:
		return operationIDAuthoredSelectionValue(typed.X, seen)
	case *ssa.ChangeType:
		return operationIDAuthoredSelectionValue(typed.X, seen)
	case *ssa.Convert:
		return operationIDAuthoredSelectionValue(typed.X, seen)
	case *ssa.MakeInterface:
		return operationIDAuthoredSelectionValue(typed.X, seen)
	case *ssa.Phi:
		for _, edge := range typed.Edges {
			if !operationIDAuthoredSelectionValue(edge, seen) {
				return false
			}
		}

		return len(typed.Edges) > 0
	case *ssa.Alloc:
		return true
	case *ssa.Const, *ssa.BinOp, *ssa.Call, *ssa.Global:
		return false
	default:
		return true
	}
}

func operationIDLocalAddress(value ssa.Value) bool {
	switch typed := value.(type) {
	case *ssa.Alloc:
		return true
	case *ssa.FieldAddr:
		return operationIDLocalAddress(typed.X)
	case *ssa.IndexAddr:
		return operationIDLocalAddress(typed.X)
	}

	return false
}

func operationIDRuntimeFunctions(build *ssa.Function) map[*ssa.Function]bool {
	functions := make(map[*ssa.Function]bool)
	pending := []*ssa.Function{build}

	for len(pending) > 0 {
		function := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		if function == nil || functions[function] {
			continue
		}

		functions[function] = true

		pending = append(pending, function.AnonFuncs...)
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}

				callee := call.Common().StaticCallee()
				if callee != nil && callee.Pkg == build.Pkg {
					pending = append(pending, callee)
				}
			}
		}
	}

	return functions
}

func operationIDField(value ssa.Value, currentPackage *types.Package) bool {
	var (
		aggregate types.Type
		index     int
	)

	switch field := value.(type) {
	case *ssa.Field:
		aggregate = field.X.Type()
		index = field.Field
	case *ssa.FieldAddr:
		pointer, ok := types.Unalias(field.X.Type()).Underlying().(*types.Pointer)
		if !ok {
			return false
		}

		aggregate = pointer.Elem()
		index = field.Field
	default:
		return false
	}

	structure, ok := types.Unalias(aggregate).Underlying().(*types.Struct)
	if !ok || index >= structure.NumFields() {
		return false
	}

	inputType := packageObjectTypeFromTypes(currentPackage, "Input")

	return inputType != nil && sameGuardType(aggregate, inputType) && structure.Field(index).Name() == "OperationID"
}

func copiedSemanticGraphViolations(guardPackage *sourceGuardPackage) []string {
	violations := packageSemanticDataViolations(guardPackage)
	violations = append(violations, localSemanticTableViolations(guardPackage)...)

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}

			if generatedGraphBody(function.Body) {
				violations = append(violations, function.Name.Name+": copied generated semantic graph")
			}
		}
	}

	slices.Sort(violations)

	return slices.Compact(violations)
}

// allowedSemanticData is the literal provenance allowlist for independently authored admission and format specifications.
var allowedSemanticData = map[string]string{ //nolint:gochecknoglobals // Exact identities and roles form the provenance boundary.
	"operationMethods":            "[]string",
	"schemaKinds":                 "map[string]schemaKind",
	"schemaKeywords":              "map[string]bool",
	"numericFormatNames":          "map[string]schemaFormat",
	"base64FormatSpecification":   "*stringFormatSpecification",
	"dateFormatSpecification":     "*stringFormatSpecification",
	"dateTimeFormatSpecification": "*stringFormatSpecification",
	"emailFormatSpecification":    "*stringFormatSpecification",
	"ipv4FormatSpecification":     "*stringFormatSpecification",
	"uuidFormatSpecification":     "*stringFormatSpecification",
	"cidrFormatSpecification":     "*stringFormatSpecification",
	"passwordFormatSpecification": "*stringFormatSpecification",
}

func packageSemanticDataViolations(guardPackage *sourceGuardPackage) []string {
	var violations []string

	for _, name := range guardPackage.pkg.Scope().Names() {
		object, ok := guardPackage.pkg.Scope().Lookup(name).(*types.Var)
		if !ok || !aggregateSemanticData(object.Type()) {
			continue
		}

		role := types.TypeString(object.Type(), ownershipTypeQualifier(guardPackage.pkg))
		if allowedSemanticData[name] == role && allowedSemanticSource(name, object, guardPackage) ||
			independentlyAuthoredGenericSpecification(name, object.Type()) {
			continue
		}

		violations = append(violations, name+": unapproved package semantic data")
	}

	return violations
}

func aggregateSemanticData(owned types.Type) bool {
	switch types.Unalias(owned).Underlying().(type) {
	case *types.Array, *types.Map, *types.Pointer, *types.Slice, *types.Struct:
		return true
	default:
		return false
	}
}

func allowedSemanticSource(name string, object *types.Var, guardPackage *sourceGuardPackage) bool {
	expectedFile := "string_format_spec.go"

	switch name {
	case "operationMethods":
		expectedFile = "oas_parse.go"
	case "schemaKinds", "schemaKeywords", "numericFormatNames":
		expectedFile = "oas_schema.go"
	}

	return filepath.Base(guardPackage.fset.Position(object.Pos()).Filename) == expectedFile
}

func independentlyAuthoredGenericSpecification(name string, owned types.Type) bool {
	switch name {
	case "unicodeGrammar", "formatTransitions":
		return aggregateSemanticData(owned)
	default:
		return false
	}
}

func genericNumericSpecification(owned types.Type) bool {
	var numeric, boolean bool
	if !genericNumericSpecificationSeen(owned, make(map[types.Type]bool), &numeric, &boolean) {
		return false
	}

	return numeric != boolean
}

func genericNumericSpecificationSeen(
	owned types.Type,
	seen map[types.Type]bool,
	numeric, boolean *bool,
) bool {
	owned = types.Unalias(owned)
	if seen[owned] {
		return true
	}

	seen[owned] = true

	switch typed := owned.Underlying().(type) {
	case *types.Basic:
		if typed.Info()&types.IsBoolean != 0 {
			*boolean = true

			return true
		}

		if typed.Info()&(types.IsInteger|types.IsFloat) != 0 {
			*numeric = true

			return true
		}

		return false
	case *types.Array:
		return genericNumericSpecificationSeen(typed.Elem(), seen, numeric, boolean)
	case *types.Slice:
		return genericNumericSpecificationSeen(typed.Elem(), seen, numeric, boolean)
	case *types.Struct:
		for index := range typed.NumFields() {
			if !genericNumericSpecificationSeen(typed.Field(index).Type(), seen, numeric, boolean) {
				return false
			}
		}

		return true
	default:
		return false
	}
}

func localSemanticTableViolations(guardPackage *sourceGuardPackage) []string {
	var violations []string

	allowedLocalSpecifications := map[string]bool{
		"anyOfValidRequirements":           true,
		"canonicalJSONKinds":               true,
		"compileValidSchedule":             true,
		"defaultArrayPresenceRequirements": true,
		"enumFaultKinds":                   true,
		"matchSequenceEnds":                true,
		"mergePatternRanges":               true,
		"normalizePatternMatcherRanges":    true,
		"orderedTypeKinds":                 true,
		"parseEscape":                      true,
		"projectedMemberPresenceChoices":   true,
		"rowKindChoices":                   true,
	}

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || allowedLocalSpecifications[function.Name.Name] {
				continue
			}

			localTables := make(map[*types.Var]bool)

			ast.Inspect(function.Body, func(node ast.Node) bool {
				assignment, assignmentOK := node.(*ast.AssignStmt)
				if assignmentOK {
					for index, right := range assignment.Rhs {
						if index >= len(assignment.Lhs) || !semanticCollectionLiteral(right, guardPackage) {
							continue
						}

						identifier, ok := assignment.Lhs[index].(*ast.Ident)
						if ok {
							if variable, variableOK := guardPackage.info.Defs[identifier].(*types.Var); variableOK {
								localTables[variable] = true
							}
						}
					}
				}

				returned, returnOK := node.(*ast.ReturnStmt)
				if !returnOK {
					return true
				}

				for _, result := range returned.Results {
					if identifier, ok := result.(*ast.Ident); ok {
						variable, variableOK := guardPackage.info.Uses[identifier].(*types.Var)
						if variableOK && localTables[variable] {
							violations = append(violations, function.Name.Name+": copied local semantic table")
						}
					}

					literal, literalOK := result.(*ast.CompositeLit)
					if !literalOK || len(literal.Elts) == 0 {
						continue
					}

					valueType := guardPackage.info.TypeOf(literal)

					var element types.Type

					switch typed := types.Unalias(valueType).Underlying().(type) {
					case *types.Array:
						element = typed.Elem()
					case *types.Slice:
						element = typed.Elem()
					case *types.Map:
						element = typed.Elem()
					default:
						continue
					}

					categories := make(map[string]bool)
					semanticPrimitiveCategories(element, categories, make(map[types.Type]bool))

					if len(categories) > 0 {
						violations = append(violations, function.Name.Name+": copied local semantic table")
					}
				}

				return false
			})
		}
	}

	return violations
}

func semanticCollectionLiteral(expression ast.Expr, guardPackage *sourceGuardPackage) bool {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || len(literal.Elts) == 0 {
		return false
	}

	switch types.Unalias(guardPackage.info.TypeOf(literal)).Underlying().(type) {
	case *types.Array, *types.Map, *types.Slice:
		return true
	default:
		return false
	}
}

func semanticPrimitiveCategories(owned types.Type, categories map[string]bool, seen map[types.Type]bool) {
	owned = types.Unalias(owned)
	if seen[owned] {
		return
	}

	seen[owned] = true

	switch typed := owned.Underlying().(type) {
	case *types.Basic:
		switch {
		case typed.Info()&types.IsBoolean != 0:
			categories["boolean"] = true
		case typed.Info()&(types.IsInteger|types.IsFloat) != 0:
			categories["numeric"] = true
		case typed.Info()&types.IsString != 0:
			categories["string"] = true
		}
	case *types.Pointer:
		semanticPrimitiveCategories(typed.Elem(), categories, seen)
	case *types.Array:
		semanticPrimitiveCategories(typed.Elem(), categories, seen)
	case *types.Slice:
		semanticPrimitiveCategories(typed.Elem(), categories, seen)
	case *types.Map:
		semanticPrimitiveCategories(typed.Key(), categories, seen)
		semanticPrimitiveCategories(typed.Elem(), categories, seen)
	case *types.Struct:
		for index := range typed.NumFields() {
			semanticPrimitiveCategories(typed.Field(index).Type(), categories, seen)
		}
	}
}

func generatedGraphBody(body *ast.BlockStmt) bool {
	for _, statement := range body.List {
		assignment, ok := statement.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			continue
		}

		name, ok := assignment.Lhs[0].(*ast.Ident)
		if !ok || !isFixedPointerSliceMake(assignment.Rhs[0]) {
			continue
		}

		allocated, filled, linked, returned := generatedGraphOperations(body, name.Name)
		if allocated && filled && linked && returned {
			return true
		}
	}

	return false
}
