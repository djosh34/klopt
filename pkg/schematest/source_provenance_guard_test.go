//nolint:cyclop,gocognit,gocyclo,godoclint,lll // Whole-program provenance checks require complete SSA and graph traversals.
package schematest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/constant"
	"go/format"
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

func TestOperationIDFlowGuardAllowsDecodedDocumentSelection(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type Input struct { OpenAPI []byte; OperationID string }; type jsonValue struct { text string; object map[string]*jsonValue }
		func decodeOpenAPIDocument(source []byte) (*jsonValue, error) { return &jsonValue{text: string(source)}, nil }
		func requireJSONObject(value *jsonValue) (map[string]*jsonValue, error) { return value.object, nil }
		func Build(input Input) bool {
			document, _ := decodeOpenAPIDocument(input.OpenAPI)
			root, _ := requireJSONObject(document)
			return selectRequestSchema(document, root, input.OperationID)
		}
		func selectRequestSchema(document *jsonValue, root map[string]*jsonValue, operationID string) bool {
			operation := root["operation"]
			identifier := operation.object["operationId"]
			_ = document
			return identifier.text == operationID
		}
	`

	require.Empty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"selection.go": source})))
}

func TestOperationIDFlowGuardRejectsSameNameFixedDecoder(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type Input struct { OpenAPI []byte; OperationID string }; type jsonValue struct { text string; object map[string]*jsonValue }
		func decodeOpenAPIDocument([]byte) (*jsonValue, error) {
			return &jsonValue{object: map[string]*jsonValue{"operationId": {text: "fixed"}}}, nil
		}
		func Build(input Input) bool {
			document, _ := decodeOpenAPIDocument(input.OpenAPI)
			return selectRequestSchema(document, input.OperationID)
		}
		func selectRequestSchema(document *jsonValue, operationID string) bool {
			return document.object["operationId"].text == operationID
		}
	`

	require.NotEmpty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"selection.go": source})))
}

func TestOperationIDFlowGuardRejectsSourceSiblingWithFixedOperationID(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type Input struct { OpenAPI []byte; OperationID string }; type jsonValue struct { text string; object map[string]*jsonValue }
		func decodeOpenAPIDocument(source []byte) (*jsonValue, error) {
			return &jsonValue{
				text: string(source),
				object: map[string]*jsonValue{"operationId": {text: "fixed"}},
			}, nil
		}
		func Build(input Input) bool {
			document, _ := decodeOpenAPIDocument(input.OpenAPI)
			return selectRequestSchema(document, input.OperationID)
		}
		func selectRequestSchema(document *jsonValue, operationID string) bool {
			return document.object["operationId"].text == operationID
		}
	`

	require.NotEmpty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"selection.go": source})))
}

func TestOperationIDFlowGuardRejectsMixedDocumentAndLocalSelectionValue(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type Input struct { OperationID string }; type jsonValue struct { text string; object map[string]*jsonValue }
		func Build(input Input, document *jsonValue, local bool) bool { return selectRequestSchema(document, input.OperationID, local) }
		func selectRequestSchema(document *jsonValue, operationID string, local bool) bool {
			identifier := document.object["operationId"]
			if local { identifier = &jsonValue{text: "fixed"} }
			return identifier.text == operationID
		}
	`

	require.NotEmpty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"selection.go": source})))
}

func TestOperationIDFlowGuardRejectsLocallyConstructedSelectionValues(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"manufactured document root": `package schematest
			type Input struct { OperationID string }; type jsonValue struct { text string; object map[string]*jsonValue }
			func Build(input Input) bool { root := map[string]*jsonValue{"operationId": {text: "fixed"}}; return selectRequestSchema(root, input.OperationID) }
			func selectRequestSchema(root map[string]*jsonValue, operationID string) bool { return root["operationId"].text == operationID }`,
		"document-consuming helper returns local data": `package schematest
			type Input struct { OperationID string }; type jsonValue struct { text string; object map[string]*jsonValue }
			func decodeOpenAPIDocument([]byte) (*jsonValue, error) { return new(jsonValue), nil }
			func fixed(*jsonValue) map[string]*jsonValue { return map[string]*jsonValue{"operationId": {text: "fixed"}} }
			func Build(input Input) bool { document, _ := decodeOpenAPIDocument(nil); return selectRequestSchema(fixed(document), input.OperationID) }
			func selectRequestSchema(root map[string]*jsonValue, operationID string) bool { return root["operationId"].text == operationID }`,
		"local jsonValue field": `package schematest
			type Input struct { OperationID string }; type jsonValue struct { text string }
			func Build(input Input) bool { return selectRequestSchema(input.OperationID) }
			func selectRequestSchema(operationID string) bool { local := &jsonValue{text: "fixed"}; return local.text == operationID }`,
		"fixed map": `package schematest
			type Input struct { OperationID string }
			func Build(input Input) bool { return selectRequestSchema(input.OperationID) }
			func selectRequestSchema(operationID string) bool { local := map[string]string{"id": "fixed"}; return local["id"] == operationID }`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"guard.go": source})))
		})
	}
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

func TestCopiedSemanticGuardRejectsLocalConstructionAndGenericRoleSpoofs(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"ValueSpec": `package schematest
			type answer struct { keyword string; valid bool }
			func copied() []answer { var rows = []answer{{keyword: "type", valid: true}}; return rows }
			func Build() { _ = copied() }`,
		"make append helper chain": `package schematest
			type answer struct { keyword string; valid bool }
			func copied() []answer { rows := make([]answer, 0); rows = append(rows, answer{keyword: "type", valid: true}); return rows }
			func forwarded() []answer { return copied() }
			func Build() { _ = forwarded() }`,
		"renamed three uint8 cell helper chain": `package schematest
			type cell struct { x, y, z uint8 }
			func copied() []cell { rows := make([]cell, 0); rows = append(rows, cell{x: 1, y: 2, z: 3}); return rows }
			func forwarded() []cell { return copied() }
			func Build() { _ = forwarded() }`,
		"renamed two uint16 cell helper chain": `package schematest
			type cell struct { x, y uint16 }
			func copied() []cell { rows := make([]cell, 0); rows = append(rows, cell{x: 1, y: 2}); return rows }
			func forwarded() []cell { return copied() }
			func Build() { _ = forwarded() }`,
		"primitive uint8 helper chain": `package schematest
			func copied() []uint8 { rows := make([]uint8, 0); rows = append(rows, 1, 2); return rows }
			func forwarded() []uint8 { return copied() }
			func Build() { _ = forwarded() }`,
		"primitive string helper chain": `package schematest
			func copied() []string { rows := []string{"type", "required"}; return rows }
			func forwarded() []string { return copied() }
			func Build() { _ = forwarded() }`,
		"primitive uint8 aliased append helper chain": `package schematest
			func copied() []uint8 { rows := make([]uint8, 0); first := uint8(1); second := first; rows = append(rows, second); return rows }
			func forwarded() []uint8 { return copied() }
			func Build() { _ = forwarded() }`,
		"primitive string aliased append helper chain": `package schematest
			func copied() []string { rows := make([]string, 0); first := "type"; second := first; rows = append(rows, second); return rows }
			func forwarded() []string { return copied() }
			func Build() { _ = forwarded() }`,
		"unicode role spoof": `package schematest
			type answer struct { keyword string; valid bool }
			var unicodeGrammar = []answer{{keyword: "type", valid: true}}`,
		"transition role spoof": `package schematest
			type answer struct { state, next int; class bool }
			var formatTransitions = []answer{{state: 1, next: 2, class: true}}`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, copiedAnswerViolations(parseGuardPackage(t, map[string]string{"guard.go": source}), nil))
		})
	}
}

func TestCopiedSemanticGuardRejectsInexactGenericSpecifications(t *testing.T) {
	t.Parallel()

	sources := []map[string]string{
		{"wrong.go": `package schematest
			type runeRange struct { first, last rune }
			var unicodeGrammar = []runeRange{{first: 'a', last: 'z'}, {first: 0x80, last: 0x10ffff}}`},
		{"grammar.go": `package schematest
			type runeRange struct { first, last rune }
			var unicodeGrammar = []runeRange{{first: 'b', last: 'z'}, {first: 0x80, last: 0x10ffff}}`},
	}

	for _, source := range sources {
		require.NotEmpty(t, copiedAnswerViolations(parseGuardPackage(t, source), nil))
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
	authored := operationIDDocumentValues(functions, guardPackage.pkg)
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
						operationIDSelectionComparison(function, binary, tainted, authored, guardPackage.pkg) {
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
					if operand == nil || !tainted[*operand] || operationIDAllowedUse(function, instruction, tainted, authored, guardPackage.pkg) {
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
	authored map[ssa.Value]bool,
	currentPackage *types.Package,
) bool {
	switch typed := instruction.(type) {
	case *ssa.BinOp:
		return operationIDSelectionComparison(function, typed, tainted, authored, currentPackage)
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
	documentValues map[ssa.Value]bool,
	currentPackage *types.Package,
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

	return operationIDAuthoredSelectionValue(authored, documentValues, currentPackage, make(map[ssa.Value]bool))
}

func operationIDDocumentValues(
	functions map[*ssa.Function]bool,
	currentPackage *types.Package,
) map[ssa.Value]bool {
	values := make(map[ssa.Value]bool)
	sources := make(map[ssa.Value]bool)
	returnsSource := make(map[*ssa.Function]bool)
	calls := make(map[*ssa.Function][]ssa.CallInstruction)

	for function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				value, hasValue := instruction.(ssa.Value)
				if hasValue && operationIDOpenAPIField(value, currentPackage) {
					sources[value] = true
				}

				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}

				callee := call.Common().StaticCallee()
				if callee != nil && functions[callee] {
					calls[callee] = append(calls[callee], call)
				}
			}
		}
	}

	for changed := true; changed; {
		changed = false

		for function := range functions {
			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					if stored, ok := instruction.(*ssa.Store); ok && sources[stored.Val] && !sources[stored.Addr] {
						sources[stored.Addr] = true
						changed = true
					}

					call, isCall := instruction.(ssa.CallInstruction)
					if isCall {
						callee := call.Common().StaticCallee()
						if callee != nil && functions[callee] {
							for index, argument := range call.Common().Args {
								if index < len(callee.Params) && sources[argument] && !sources[callee.Params[index]] {
									sources[callee.Params[index]] = true
									changed = true
								}
							}
						}
					}

					if returned, ok := instruction.(*ssa.Return); ok {
						for _, result := range returned.Results {
							if sources[result] && !returnsSource[function] {
								returnsSource[function] = true
								changed = true
							}
						}
					}

					value, hasValue := instruction.(ssa.Value)
					if hasValue && sources[value] {
						continue
					}

					if !hasValue {
						continue
					}

					if isCall {
						callee := call.Common().StaticCallee()
						if callee != nil && returnsSource[callee] {
							sources[value] = true
							changed = true

							continue
						}
					}

					for _, operand := range instruction.Operands(nil) {
						if operand != nil && sources[*operand] {
							sources[value] = true
							changed = true

							break
						}
					}
				}
			}

			for index, parameter := range function.Params {
				incoming := calls[function]
				if values[parameter] || len(incoming) == 0 {
					continue
				}

				pure := true

				for _, call := range incoming {
					if index >= len(call.Common().Args) || !values[call.Common().Args[index]] {
						pure = false

						break
					}
				}

				if pure {
					values[parameter] = true
					changed = true
				}
			}

			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					value, ok := instruction.(ssa.Value)
					if !ok || values[value] {
						continue
					}

					call, isCall := instruction.(ssa.CallInstruction)
					if isCall {
						callee := call.Common().StaticCallee()
						if callee != nil && callee.Object() == currentPackage.Scope().Lookup("decodeOpenAPIDocument") &&
							len(call.Common().Args) > 0 && sources[call.Common().Args[0]] &&
							operationIDDecoderConsumesSource(callee, sources) &&
							!operationIDDecoderAuthorsFixedIdentity(callee, functions, make(map[*ssa.Function]bool)) {
							values[value] = true
							changed = true

							continue
						}
					}

					if operationIDDocumentTransfer(instruction, values, functions) {
						values[value] = true
						changed = true
					}
				}
			}
		}
	}

	return values
}

func operationIDDecoderConsumesSource(function *ssa.Function, sources map[ssa.Value]bool) bool {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			for _, operand := range instruction.Operands(nil) {
				if operand != nil && sources[*operand] {
					return true
				}
			}
		}
	}

	return false
}

func operationIDDecoderAuthorsFixedIdentity(
	function *ssa.Function,
	functions map[*ssa.Function]bool,
	seen map[*ssa.Function]bool,
) bool {
	if function == nil || seen[function] {
		return false
	}

	seen[function] = true

	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			if update, ok := instruction.(*ssa.MapUpdate); ok {
				key, constantKey := update.Key.(*ssa.Const)
				if constantKey && key.Value != nil && constant.StringVal(key.Value) == "operationId" {
					return true
				}
			}

			call, ok := instruction.(ssa.CallInstruction)
			if !ok {
				continue
			}

			callee := call.Common().StaticCallee()
			if callee != nil && functions[callee] && operationIDDecoderAuthorsFixedIdentity(callee, functions, seen) {
				return true
			}
		}
	}

	return false
}

func operationIDDocumentTransfer(
	instruction ssa.Instruction,
	values map[ssa.Value]bool,
	functions map[*ssa.Function]bool,
) bool {
	switch typed := instruction.(type) {
	case *ssa.Extract:
		if typed.Index != 0 {
			return false
		}

		if values[typed.Tuple] {
			return true
		}

		call, ok := typed.Tuple.(ssa.CallInstruction)

		return ok && operationIDCallResult(call, typed.Index, values, functions)
	case *ssa.Lookup:
		return values[typed.X]
	case *ssa.Field:
		return values[typed.X]
	case *ssa.FieldAddr:
		return values[typed.X]
	case *ssa.UnOp:
		return values[typed.X]
	case *ssa.ChangeType:
		return values[typed.X]
	case *ssa.Convert:
		return values[typed.X]
	case *ssa.MakeInterface:
		return values[typed.X]
	case *ssa.Phi:
		return len(typed.Edges) > 0 && operationIDAllDocumentValues(typed.Edges, values)
	}

	call, ok := instruction.(ssa.CallInstruction)
	if !ok {
		return false
	}

	return operationIDCallResult(call, 0, values, functions)
}

func operationIDCallResult(
	call ssa.CallInstruction,
	resultIndex int,
	values map[ssa.Value]bool,
	functions map[*ssa.Function]bool,
) bool {
	callee := call.Common().StaticCallee()
	if callee == nil || !functions[callee] {
		return false
	}

	if callee.Name() == "requireJSONObject" {
		return resultIndex == 0 && len(call.Common().Args) > 0 && values[call.Common().Args[0]]
	}

	if callee.Name() == "resolvePathItemReference" {
		return resultIndex == 0 && len(call.Common().Args) > 1 &&
			values[call.Common().Args[0]] && values[call.Common().Args[1]]
	}

	found := false

	for _, block := range callee.Blocks {
		for _, candidate := range block.Instrs {
			returned, ok := candidate.(*ssa.Return)
			if !ok {
				continue
			}

			if resultIndex >= len(returned.Results) {
				return false
			}

			result := returned.Results[resultIndex]
			if constantResult, ok := result.(*ssa.Const); ok && constantResult.Value == nil {
				continue
			}

			if !operationIDDocumentCandidate(result, values, functions, make(map[ssa.Value]bool)) {
				return false
			}

			found = true
		}
	}

	return found
}

func operationIDDocumentCandidate(
	value ssa.Value,
	values map[ssa.Value]bool,
	functions map[*ssa.Function]bool,
	seen map[ssa.Value]bool,
) bool {
	if value == nil {
		return false
	}

	if values[value] || seen[value] {
		return true
	}

	if _, basic := types.Unalias(value.Type()).Underlying().(*types.Basic); basic {
		return false
	}

	seen[value] = true

	switch typed := value.(type) {
	case *ssa.Parameter:
		index := -1

		for parameterIndex, parameter := range typed.Parent().Params {
			if parameter == typed {
				index = parameterIndex

				break
			}
		}

		if index < 0 {
			return false
		}

		found := false

		for caller := range functions {
			for _, block := range caller.Blocks {
				for _, instruction := range block.Instrs {
					call, ok := instruction.(ssa.CallInstruction)
					if !ok || call.Common().StaticCallee() != typed.Parent() {
						continue
					}

					if index >= len(call.Common().Args) ||
						!operationIDDocumentCandidate(call.Common().Args[index], values, functions, seen) {
						return false
					}

					found = true
				}
			}
		}

		return found
	case *ssa.Lookup:
		return operationIDDocumentCandidate(typed.X, values, functions, seen)
	case *ssa.Field:
		return operationIDDocumentCandidate(typed.X, values, functions, seen)
	case *ssa.FieldAddr:
		return operationIDDocumentCandidate(typed.X, values, functions, seen)
	case *ssa.UnOp:
		return operationIDDocumentCandidate(typed.X, values, functions, seen)
	case *ssa.ChangeType:
		return operationIDDocumentCandidate(typed.X, values, functions, seen)
	case *ssa.Convert:
		return operationIDDocumentCandidate(typed.X, values, functions, seen)
	case *ssa.MakeInterface:
		return operationIDDocumentCandidate(typed.X, values, functions, seen)
	case *ssa.Phi:
		for _, edge := range typed.Edges {
			if !operationIDDocumentCandidate(edge, values, functions, seen) {
				return false
			}
		}

		return len(typed.Edges) > 0
	case *ssa.Extract:
		if typed.Index != 0 {
			return false
		}

		call, ok := typed.Tuple.(ssa.CallInstruction)

		return ok && operationIDCallResult(call, typed.Index, values, functions)
	}

	return false
}

func operationIDAllDocumentValues(candidates []ssa.Value, values map[ssa.Value]bool) bool {
	for _, candidate := range candidates {
		if !values[candidate] {
			return false
		}
	}

	return true
}

func operationIDDocumentRootType(valueType, jsonValueType types.Type) bool {
	mapping, ok := types.Unalias(valueType).Underlying().(*types.Map)
	if !ok || basicTypeKind(mapping.Key()) != types.String {
		return false
	}

	return sameGuardType(mapping.Elem(), types.NewPointer(jsonValueType))
}

func operationIDAuthoredSelectionValue(
	value ssa.Value,
	documentValues map[ssa.Value]bool,
	currentPackage *types.Package,
	seen map[ssa.Value]bool,
) bool {
	if !documentValues[value] {
		return false
	}

	switch typed := value.(type) {
	case *ssa.Field:
		return operationIDJSONTextField(typed.X.Type(), typed.Field, currentPackage) &&
			operationIDLookupProvenance(typed.X, documentValues, seen)
	case *ssa.UnOp:
		field, ok := typed.X.(*ssa.FieldAddr)

		return ok && operationIDJSONTextField(field.X.Type(), field.Field, currentPackage) &&
			operationIDLookupProvenance(field.X, documentValues, seen)
	default:
		return false
	}
}

func operationIDLookupProvenance(
	value ssa.Value,
	documentValues map[ssa.Value]bool,
	seen map[ssa.Value]bool,
) bool {
	if value == nil || seen[value] {
		return false
	}

	seen[value] = true

	switch typed := value.(type) {
	case *ssa.Lookup:
		key, constantKey := typed.Index.(*ssa.Const)

		return constantKey && key.Value != nil && constant.StringVal(key.Value) == "operationId" &&
			operationIDPureDocumentValue(typed.X, documentValues, make(map[ssa.Value]bool))
	case *ssa.Phi:
		if len(typed.Edges) == 0 {
			return false
		}

		for _, edge := range typed.Edges {
			if !operationIDLookupProvenance(edge, documentValues, seen) {
				return false
			}
		}

		return true
	case *ssa.ChangeType:
		return operationIDLookupProvenance(typed.X, documentValues, seen)
	case *ssa.Convert:
		return operationIDLookupProvenance(typed.X, documentValues, seen)
	case *ssa.MakeInterface:
		return operationIDLookupProvenance(typed.X, documentValues, seen)
	case *ssa.UnOp:
		return operationIDLookupProvenance(typed.X, documentValues, seen)
	case *ssa.Extract:
		return operationIDLookupProvenance(typed.Tuple, documentValues, seen)
	case *ssa.Field:
		return operationIDLookupProvenance(typed.X, documentValues, seen)
	case *ssa.FieldAddr:
		return operationIDLookupProvenance(typed.X, documentValues, seen)
	default:
		return false
	}
}

func operationIDPureDocumentValue(
	value ssa.Value,
	documentValues map[ssa.Value]bool,
	seen map[ssa.Value]bool,
) bool {
	if value == nil || !documentValues[value] {
		return false
	}

	if seen[value] {
		return true
	}

	seen[value] = true

	switch typed := value.(type) {
	case *ssa.Parameter:
		return documentValues[typed]
	case *ssa.Lookup:
		return operationIDPureDocumentValue(typed.X, documentValues, seen)
	case *ssa.Phi:
		if len(typed.Edges) == 0 {
			return false
		}

		for _, edge := range typed.Edges {
			if !operationIDPureDocumentValue(edge, documentValues, seen) {
				return false
			}
		}

		return true
	case *ssa.ChangeType:
		return operationIDPureDocumentValue(typed.X, documentValues, seen)
	case *ssa.Convert:
		return operationIDPureDocumentValue(typed.X, documentValues, seen)
	case *ssa.MakeInterface:
		return operationIDPureDocumentValue(typed.X, documentValues, seen)
	case *ssa.UnOp:
		return operationIDPureDocumentValue(typed.X, documentValues, seen)
	case *ssa.Field:
		return operationIDPureDocumentValue(typed.X, documentValues, seen)
	case *ssa.FieldAddr:
		return operationIDPureDocumentValue(typed.X, documentValues, seen)
	case *ssa.Extract:
		return operationIDPureDocumentValue(typed.Tuple, documentValues, seen)
	case *ssa.Call:
		return documentValues[typed]
	case *ssa.Alloc:
		return false
	default:
		return false
	}
}

func operationIDJSONTextField(valueType types.Type, field int, currentPackage *types.Package) bool {
	if pointer, ok := types.Unalias(valueType).Underlying().(*types.Pointer); ok {
		valueType = pointer.Elem()
	}

	structure, ok := types.Unalias(valueType).Underlying().(*types.Struct)
	if !ok || field >= structure.NumFields() {
		return false
	}

	return sameGuardType(valueType, packageObjectTypeFromTypes(currentPackage, "jsonValue")) &&
		structure.Field(field).Name() == "text"
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

func operationIDOpenAPIField(value ssa.Value, currentPackage *types.Package) bool {
	return operationIDInputField(value, currentPackage, "OpenAPI")
}

func operationIDField(value ssa.Value, currentPackage *types.Package) bool {
	return operationIDInputField(value, currentPackage, "OperationID")
}

func operationIDInputField(value ssa.Value, currentPackage *types.Package, name string) bool {
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

	return inputType != nil && sameGuardType(aggregate, inputType) && structure.Field(index).Name() == name
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
			independentlyAuthoredGenericSpecification(name, object, guardPackage) {
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

	expectedHash := map[string]string{
		"operationMethods":            "c85d6afd6ab5e9ff1df322a76e15ad12f08d7a4dc4cb0811b37e6685fdee459d",
		"schemaKinds":                 "3db5ddf04af2a47b30b65e919b9ef6717cdba9f9ac96f491fe2426ebad2112c0",
		"schemaKeywords":              "bdbc748c63ee6cf47d65dd9287d170ea2504686dd6609cb1cdab03cc3fe8ee44",
		"numericFormatNames":          "0f15016822eeddbdc6282bf284b06b2a7f04da321f9fb0340532626761baa8be",
		"base64FormatSpecification":   "0e4e42d8bfb183c1407bdb42dd7b789d8fcb32b09e4c96d9ace479a8c88a3c18",
		"cidrFormatSpecification":     "56169e95470642da695e1338314b673fdb32bc4ba5d5e4db134698ffd3ed8521",
		"dateFormatSpecification":     "fa3cca4defeda0f5a5330f95e9c0863ae0d8d87b903f7c4c01bd02c66552348b",
		"dateTimeFormatSpecification": "124ac4b7f4674bb5072f09088f373aac48badc69573e94227d7d76deb39526ac",
		"emailFormatSpecification":    "5c8fe3243cddd0f59d3e8a6e58058988cefd579916582bdffcb14617789caf67",
		"ipv4FormatSpecification":     "74670bdb69c1e1136b95eaa86fe35a6e0d6966ecd6c53ae54aa27c374615e80c",
		"uuidFormatSpecification":     "a6166eea6411bd42b06add0e619374243d66ed64f3325ebb0c4c99385085b7ea",
		"passwordFormatSpecification": "118892635ed0d9354b43f53e497fc87ae2ab2501db921a7ad64f2546150016d7",
	}[name]
	if filepath.Base(guardPackage.fset.Position(object.Pos()).Filename) != expectedFile || expectedHash == "" {
		return false
	}

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, specification := range general.Specs {
				valueSpec, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}

				for index, identifier := range valueSpec.Names {
					if guardPackage.info.Defs[identifier] == object && index < len(valueSpec.Values) {
						hash, hashOK := semanticNodeHash(valueSpec.Values[index], guardPackage)

						return hashOK && hash == expectedHash
					}
				}
			}
		}
	}

	return false
}

func independentlyAuthoredGenericSpecification(
	name string,
	object *types.Var,
	guardPackage *sourceGuardPackage,
) bool {
	expectedType := map[string]string{
		"unicodeGrammar":    "[]runeRange",
		"formatTransitions": "[]transition",
	}[name]
	expectedHash := map[string]string{
		"unicodeGrammar":    "d3c8f466d66703aa7d2dc09acb62b6b3dce3a5d797cccbc82125e35309542cff",
		"formatTransitions": "ece16af12d8bf3a1f419f1b3484d0931f90df49e44f6dc18511a8dcac99c0be4",
	}[name]

	if expectedType == "" || types.TypeString(object.Type(), ownershipTypeQualifier(guardPackage.pkg)) != expectedType ||
		filepath.Base(guardPackage.fset.Position(object.Pos()).Filename) != "grammar.go" {
		return false
	}

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, specification := range general.Specs {
				valueSpec, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}

				for index, identifier := range valueSpec.Names {
					if guardPackage.info.Defs[identifier] != object || index >= len(valueSpec.Values) {
						continue
					}

					hash, hashOK := semanticNodeHash(valueSpec.Values[index], guardPackage)

					return hashOK && hash == expectedHash
				}
			}
		}
	}

	return false
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
	functions := make(map[*types.Func]*ast.FuncDecl)

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}

			object, ok := guardPackage.info.Defs[function.Name].(*types.Func)
			if ok {
				functions[object] = function
			}
		}
	}

	semanticReturns := make(map[*types.Func]bool)

	for changed := true; changed; {
		changed = false

		for object, function := range functions {
			if semanticReturns[object] || !functionReturnsSemanticTable(function, semanticReturns, guardPackage) {
				continue
			}

			semanticReturns[object] = true
			changed = true
		}
	}

	var violations []string

	for object := range semanticReturns {
		if allowedLocalSemanticSpecification(object, guardPackage) {
			continue
		}

		violations = append(violations, object.Name()+": copied local semantic table")
	}

	return violations
}

func functionReturnsSemanticTable(
	function *ast.FuncDecl,
	semanticReturns map[*types.Func]bool,
	guardPackage *sourceGuardPackage,
) bool {
	localTables := make(map[*types.Var]bool)
	localSemanticValues := make(map[*types.Var]bool)
	returnedTable := false

	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ValueSpec:
			for index, value := range typed.Values {
				if index >= len(typed.Names) {
					continue
				}

				variable, ok := guardPackage.info.Defs[typed.Names[index]].(*types.Var)
				if !ok {
					continue
				}

				if semanticTableExpression(value, localTables, localSemanticValues, semanticReturns, guardPackage) {
					localTables[variable] = true
				}

				if semanticValueExpression(value, localSemanticValues, guardPackage) {
					localSemanticValues[variable] = true
				}
			}
		case *ast.AssignStmt:
			for index, value := range typed.Rhs {
				if index >= len(typed.Lhs) {
					continue
				}

				identifier, ok := typed.Lhs[index].(*ast.Ident)
				if !ok {
					continue
				}

				variable, ok := guardPackage.info.ObjectOf(identifier).(*types.Var)
				if !ok {
					continue
				}

				if semanticTableExpression(value, localTables, localSemanticValues, semanticReturns, guardPackage) {
					localTables[variable] = true
				}

				if semanticValueExpression(value, localSemanticValues, guardPackage) {
					localSemanticValues[variable] = true
				}
			}
		case *ast.ReturnStmt:
			for _, result := range typed.Results {
				returnedTable = returnedTable || semanticTableExpression(
					result, localTables, localSemanticValues, semanticReturns, guardPackage,
				)
			}
		}

		return !returnedTable
	})

	return returnedTable
}

func semanticTableExpression(
	expression ast.Expr,
	localTables map[*types.Var]bool,
	localSemanticValues map[*types.Var]bool,
	semanticReturns map[*types.Func]bool,
	guardPackage *sourceGuardPackage,
) bool {
	if semanticCollectionLiteral(expression, guardPackage) {
		return true
	}

	if identifier, ok := expression.(*ast.Ident); ok {
		variable, variableOK := guardPackage.info.ObjectOf(identifier).(*types.Var)

		return variableOK && localTables[variable]
	}

	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return false
	}

	identifier, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	if function, functionOK := guardPackage.info.Uses[identifier].(*types.Func); functionOK {
		return semanticReturns[function]
	}

	return semanticAppendExpression(call, localTables, localSemanticValues, semanticReturns, guardPackage)
}

func semanticAppendExpression(
	call *ast.CallExpr,
	localTables map[*types.Var]bool,
	localSemanticValues map[*types.Var]bool,
	semanticReturns map[*types.Func]bool,
	guardPackage *sourceGuardPackage,
) bool {
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	builtin, ok := guardPackage.info.Uses[identifier].(*types.Builtin)
	if !ok || builtin.Name() != "append" || len(call.Args) == 0 ||
		!semanticCollectionType(guardPackage.info.TypeOf(call)) {
		return false
	}

	if semanticTableExpression(call.Args[0], localTables, localSemanticValues, semanticReturns, guardPackage) {
		return true
	}

	for _, argument := range call.Args[1:] {
		if semanticValueExpression(argument, localSemanticValues, guardPackage) {
			return true
		}
	}

	return false
}

func semanticValueExpression(
	expression ast.Expr,
	localSemanticValues map[*types.Var]bool,
	guardPackage *sourceGuardPackage,
) bool {
	switch typed := expression.(type) {
	case *ast.BasicLit, *ast.CompositeLit:
		return true
	case *ast.Ident:
		variable, ok := guardPackage.info.ObjectOf(typed).(*types.Var)

		return ok && localSemanticValues[variable]
	case *ast.ParenExpr:
		return semanticValueExpression(typed.X, localSemanticValues, guardPackage)
	case *ast.CallExpr:
		return len(typed.Args) == 1 && semanticValueExpression(typed.Args[0], localSemanticValues, guardPackage)
	default:
		return false
	}
}

func semanticCollectionType(valueType types.Type) bool {
	switch types.Unalias(valueType).Underlying().(type) {
	case *types.Array, *types.Map, *types.Slice:
		return true
	default:
		return false
	}
}

func semanticPrimitiveType(valueType types.Type, seen map[types.Type]bool) bool {
	valueType = types.Unalias(valueType)
	if seen[valueType] {
		return true
	}

	seen[valueType] = true

	switch typed := valueType.Underlying().(type) {
	case *types.Basic:
		return typed.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat|types.IsString) != 0
	case *types.Array:
		return semanticPrimitiveType(typed.Elem(), seen)
	case *types.Struct:
		if typed.NumFields() == 0 {
			return false
		}

		for index := range typed.NumFields() {
			if !semanticPrimitiveType(typed.Field(index).Type(), seen) {
				return false
			}
		}

		return true
	default:
		return false
	}
}

func allowedLocalSemanticSpecification(function *types.Func, guardPackage *sourceGuardPackage) bool {
	expected := map[string]string{
		"allOfFaultRequirements":                     "ddd5c1c5364b89bd74283e0b582e5bde099b3f89f84da01540b942e7564cde4e",
		"allOfValidRequirements":                     "a4e66d9004cad0859c80ddcee522a0ca3d6b9248793a742f4a9ca2a808448245",
		"anyOfFaultRequirements":                     "4889344df06b545d9ef4ad966b9cc23b11beda241216e7a3105bcf7315499f64",
		"anyOfMaskRequirements":                      "c147c722b45ed9b0d42f9f58bc89cb573aaad1ea98ea91243a54d4930b667ff8",
		"anyOfValidRequirements":                     "0fc85f2cf4bc2a157892614c8a6af245c0a386030314776aec895e1a0e45699c",
		"arrayCombinationAt":                         "f5b7fbb50429f53d4ae0b03f1d97ff24694cd5278dec7f96f177471baaa88b26",
		"rowProjectionView.appendBranchRequirements": "ac3a4b0d0a23ccc6a66e86387b60ad0f5a36151996dde0d2973dca9a349efaf6",
		"appendCanonicalString":                      "cb2e7cca818a5e174d228b5a8ce3b2da148055cb3003cb1949866183ee341dd6",
		"appendExactDecimalTerm":                     "57c57a29049de187e337caa666a53b7516720ec333b82d551b4527e8ce1e560b",
		"appendJSONBoolean":                          "d1c250fc32aaf7d9bc7a5d1fb41088ac2c8b3fb4c6888ceef7c97681e6014db8",
		"appendJSONCharacter":                        "ee34f57139d30358be6c3b4187f1b33b1aec3bd682857890fa98e7ef30e7fb5d",
		"appendJSONString":                           "6d52c05e037eaeb32002e8cb17afdbb808eb9e650b005bf1493c4a97bb3aa6f8",
		"basicStringClassRanges":                     "ebe3b0741bfc7554abf2a2dd883baa477ee92fcf70775fa44cb7948310430954",
		"canonicalJSONKinds":                         "79e3f9dc3855882f59c3174539f1fd836b5da20342bb14ffb28368ab32ec2974",
		"cloneBasicStringRepeatStates":               "29017eb64cc65be9382560724a6924db9fd12daeabfd16bf7476b5f37770dfb5",
		"compileValidSchedule":                       "1ffba664901331c7fa5fde695cc6cc577b2029c46d76a1c4de65472d5351d27b",
		"complementBasicStringRanges":                "95078993453c77eab3550405408482def51a558f584f18451f56fc96b124af7a",
		"complementPatternMatcherRanges":             "068d436c64ebc91d19309b3cc1ff5bf848ab77f3f07007ff854c69d2d6bd83fe",
		"compositionFaultRequirements":               "fb0114768d74294b90e2d27c2721d806cadfb5aa8b0a67eebd4f2e5c20fd2b93",
		"compositionRequirements":                    "0e6fdf8c13fa7263d59afb5233a815f63a79b739e3b4286a511279fbbc1374fb",
		"defaultArrayPresenceRequirements":           "4e084352099455cc12ede093c87f2c0ba6c13bbf9b040ab4432c2b6c3bff0273",
		"defaultPresenceRequirementsForKind":         "9dc35793eb1886047e5923ee9898970c4d9fc46d3756438874b8e41f69f75527",
		"enumFaultKinds":                             "dad3667d89f84cf70220bcb7d2658c94ac9843f8d33f67955a7a57050460302e",
		"faultSchemaChildren":                        "76754b6916577eede5d6aa04fbec583ddd3b4905eedde7c07f426b922b1d508f",
		"marshalStrict":                              "da98656b52e423107cd0252b423bf0d07096e940e360af55672eb2338b90b4b9",
		"*cleanPatternMatcher.matchAtom":             "f0b11dc6bb258d0312d5aee06ef8bbebfb5f4a6d523d0e74dc15f04233920d0d",
		"*cleanPatternMatcher.matchSequenceEnds":     "73a9c035ac8216acef063c4b684c63375f489692e404f99f541140b9a879945d",
		"mergePatternRanges":                         "0e6afd27e5fd333290477b0be46e8183ee0a80f198b214c7df0acea9550f5f52",
		"normalizePatternMatcherRanges":              "75d85c64ace80e89812da001b7d87d2c35581e19a5fd8ffdcaf0af5eae5a733d",
		"orderedTypeKinds":                           "3107adac84b5c058d43aafd30a2c849acc683293f6f39fded3fbbd8436a4a41f",
		"parentReplayGroups":                         "546d00208747def18273b29de850e09ab07cbb68b4c4d22a6b2f3889393fe82f",
		"*strictJSONParser.parseEscape":              "32d85ea9b2fd7559b650d94cdb38bae16db9e6956d5dcca86da88a95ce104514",
		"*ecmaPatternParser.parseEscape":             "a69dcee79ff1830187d95a360eb61118759b171a2c92f0312996f3ddde396902",
		"parsePlanPointer":                           "410a015514f2cd96b5dea9809b40c5809405d07290695faeb988638bc429474b",
		"parseSchemaEnum":                            "6b4cf1a1c1d8ee08eb08da9c1b497489b3cdae0e8dd7f4a3dfd445aba9783752",
		"patternMatcherRanges":                       "459f7e4b0369daa6246a23e1f00c2ad6c6e76baec9ac10f90c529edfb31ead8b",
		"projectedMemberPresenceChoices":             "696dcc8d321d49e9f3aa2c23060ca8930e8697ba1cab59f54fe4466b1237b19f",
		"rowKindChoices":                             "aab020d359db217d9ac71deb2a90640e5ea39c42011c37d9f398a8ff739d8d29",
		"rowObjectMembers":                           "545f757e035866edfbbd94db203fd9b0909fd1cf7457dab77a4e19336ea2f904",
		"rowProjectionDecodeNode":                    "e3318c317c8fe3f83d355f5016f3587bccf4cfce753cf54277d062bf5e73b222",
	}[semanticFunctionKey(function, guardPackage.pkg)]
	if expected == "" {
		return false
	}

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			functionDeclaration, ok := declaration.(*ast.FuncDecl)
			if !ok || guardPackage.info.Defs[functionDeclaration.Name] != function {
				continue
			}

			hash, hashOK := semanticNodeHash(functionDeclaration.Body, guardPackage)

			return hashOK && hash == expected
		}
	}

	return false
}

func semanticFunctionKey(function *types.Func, currentPackage *types.Package) string {
	signature, ok := function.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return function.Name()
	}

	return types.TypeString(signature.Recv().Type(), ownershipTypeQualifier(currentPackage)) + "." + function.Name()
}

func semanticNodeHash(node ast.Node, guardPackage *sourceGuardPackage) (string, bool) {
	var source bytes.Buffer
	if err := format.Node(&source, guardPackage.fset, node); err != nil {
		return "", false
	}

	sum := sha256.Sum256(source.Bytes())

	return hex.EncodeToString(sum[:]), true
}

func semanticCollectionLiteral(expression ast.Expr, guardPackage *sourceGuardPackage) bool {
	literal, ok := expression.(*ast.CompositeLit)

	return ok && len(literal.Elts) > 0 && semanticCollectionType(guardPackage.info.TypeOf(literal))
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
