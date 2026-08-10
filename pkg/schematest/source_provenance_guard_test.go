//nolint:cyclop,gocognit,gocyclo,godoclint,lll // Whole-program provenance checks require complete SSA and graph traversals.
package schematest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa" //nolint:depguard // SSA is required for the source-local whole-program guard.

	"github.com/stretchr/testify/require"
)

func TestOperationIDFlowStaysInsideOperationSelection(t *testing.T) {
	t.Parallel()

	violations := operationIDFlowViolations(productionGuardPackage(t))
	for _, violation := range violations {
		t.Log(violation)
	}

	require.Empty(t, violations)
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

func TestOperationIDFlowGuardRejectsUntrustedDecodedDocumentSelection(t *testing.T) {
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

	require.NotEmpty(t, operationIDFlowViolations(parseGuardPackage(t, map[string]string{"selection.go": source})))
}

func TestOperationIDFlowGuardRejectsComputedDecoderIdentity(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type Input struct { OpenAPI []byte; OperationID string }; type jsonValue struct { text string; object map[string]*jsonValue }
		func decodeOpenAPIDocument(source []byte) (*jsonValue, error) {
			_ = len(source)
			key := string([]byte{'o', 'p', 'e', 'r', 'a', 't', 'i', 'o', 'n', 'I', 'd'})
			return &jsonValue{object: map[string]*jsonValue{key: {text: "fixed"}}}, nil
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
		"primitive computed alias append helper chain": `package schematest
			func copied() []uint8 { rows := make([]uint8, 0); first := uint8(1); second := first + 0; rows = append(rows, second); return rows }
			func forwarded() []uint8 { return copied() }
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
	violations := operationIDAuthorityViolations(operationIDAuthorityFunctions(functions), guardPackage)
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
						operationIDSelectionComparison(function, binary, tainted, guardPackage) {
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

	for function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				for _, operand := range instruction.Operands(nil) {
					if operand == nil || !tainted[*operand] || operationIDAllowedUse(function, instruction, tainted, guardPackage) {
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
	guardPackage *sourceGuardPackage,
) bool {
	switch typed := instruction.(type) {
	case *ssa.BinOp:
		return operationIDSelectionComparison(function, typed, tainted, guardPackage)
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

	return callee != nil && callee.Pkg == function.Pkg &&
		operationIDTrustedFunction(function, guardPackage) && operationIDTrustedFunction(callee, guardPackage)
}

func operationIDSelectionComparison(
	function *ssa.Function,
	binary *ssa.BinOp,
	tainted map[ssa.Value]bool,
	guardPackage *sourceGuardPackage,
) bool {
	object, ok := function.Object().(*types.Func)
	if !ok || binary.Op != token.EQL || !operationIDTrustedFunction(function, guardPackage) ||
		semanticFunctionKey(object, guardPackage.pkg) != "selectRequestSchema" {
		return false
	}

	return tainted[binary.X] != tainted[binary.Y]
}

// allowedOperationIDAuthorityFunctions is the reviewed decode/admission authority boundary.
var allowedOperationIDAuthorityFunctions = map[string]string{ //nolint:gochecknoglobals // Exact function bodies are the trust boundary.
	"*exactNumber.normalize":               "19357129c4b11b66b4e8522eb54ebba95f68a3d0898cedacb568d01dd56a91d0",
	"*strictJSONParser.consume":            "003140aa4037ef3d709936b551c5082425a14c0e7a08a4a74c48a5e96a148994",
	"*strictJSONParser.parseEscape":        "32d85ea9b2fd7559b650d94cdb38bae16db9e6956d5dcca86da88a95ce104514",
	"*strictJSONParser.parseHexQuad":       "855c34c87ad74c193133e5a2b04d74aa922154af53b2c1e3ab9ae3da1314f1e7",
	"*strictJSONParser.parseLiteral":       "5e7d7e8fc683f5d317bb1a785dab87bfde43606c9ba16da97d115f32f4c0a4db",
	"*strictJSONParser.parseNumber":        "78836d080ab42d678b771af2b5cb6d51c9475a2460f281d1e1843f9863f41ae6",
	"*strictJSONParser.parseString":        "68f4a15b1cf18129ac7472bf485d4f1cb372e56ca0c8048a650a4f007caac4cb",
	"*strictJSONParser.parseSurrogatePair": "de63c49c241dfdd2897a0136ce6edc74f418c76eb707316a25fad3ec47e3d155",
	"*strictJSONParser.parseUnicodeEscape": "964666883330cbf912f1d663b771d97346226cf21b10e725d4507180d992f355",
	"*strictJSONParser.parseValueToken":    "8435dbbbf40570b0dd95b401c4f903fc37d9eebc7b98e82f353b2ec901d8ac16",
	"*strictJSONParser.skipWhitespace":     "8fb0f19f051e430a1322f0e6b8869c6aff48c7311db02607540f6f89b215b8db",
	"*yamlJSONDecoder.assign":              "4fa7a7f6ed15a5025e4b23d7eabcaf26d7c5df1a45f8b6955058826483b8cd29",
	"*yamlJSONDecoder.assignDecoded":       "b85c0df90442935a9fed918a9632bb4d065681e166d937bcb615008d1390a24d",
	"*yamlJSONDecoder.decode":              "5be59f10d3745cc0d9b02ec69cf13d904eaa36b4e018d13d7b6f29d1954cd088",
	"*yamlJSONDecoder.pushAliasTarget":     "69b9c16c10eb481cf75bf1777807e131e94325d1364ba6ee7899ded0fb9d6a93",
	"Build":                                "c3e6bbe16481defdbf8f012f91a15453b036e67fbf28816b6c5b636d095dbc37",
	"decimalPower":                         "8db9cc08e2e01c85304f35c735977051b1ef93755d6ba345a2994aee596b68ac",
	"decimalTrailingZeros":                 "32210fe96d02e60462a5901509f52aabfa9a28bf202dd5c2b778c1183310016e",
	"decodeOpenAPIDocument":                "3435208ffd0cb11d1413e42a885a1d7269b2025968b904fee219e3f495c2bc36",
	"decodeYAMLBoolean":                    "4be8024e6d209c8b40afb2df960d27d92481979cd4ad3b4a3d46b78d3935e024",
	"decodeYAMLNumber":                     "21b9afa50993d539dacd9fe94eac911883347705c9760ffccad8f44159b8eef0",
	"decodeYAMLString":                     "e665ae4d01d170d2b92da715239ca6bce8fe1fe3fb73b349ed5d1ede46e67eb4",
	"decodeYAMLTag":                        "9a7405383fd541b69c1cb5e9585b038c6ca3755758f7189dc29c2969aed04673",
	"divisibleByPower":                     "3b0cdb5b7131276e4bdb58d1237a99cb905af1f6fca34609d894bfe2df9dab75",
	"escapePointerToken":                   "791fe23391c2a9ec60a34a89833b3735ce5478bfdb1952a4ae4a075fca8ab246",
	"factorMultiplicity":                   "4ce2088122274a973a99d7aa0744f1c5883c460d250ff99497a7a8286d9c1417",
	"hexadecimalValue":                     "cb1f3839813f16e08722d856b1c1037be2312b5791b81d6b5a188cbb07c5dee9",
	"integerPower":                         "040f2dac4f882629f735a602eb86d8744b4128c0a46ba99c2d1d745c1a973139",
	"isDecimalDigit":                       "9b0ce8f8689750b440370af5cac654e3a69bf584e519bff0ce048cd9e6e141a5",
	"isHexDigit":                           "b0d9364980441c6c5839db825ee85f9310b82aaa2c9a478361f36a5d52dede33",
	"isURIASCIIAlpha":                      "834b8598b42754e60ed58a12d48ec36600b3ad26f2d0e190d32d15ad3ba85091",
	"isURIASCIIAlphaNumeric":               "3e1f2de74c69d7b90254d01d10e2d12fc4079e8c7ac83401238c4bd385517850",
	"newExactNumber":                       "ecb1f6071b669028a339fca619eea472b5011f0287b0fb31b4d158d258b9998e",
	"parseExactNumber":                     "542f6ab64f4a93d65d58c1fe87d4deb00e033e85bc4baf5c280c01e5afdae64f",
	"parseInput":                           "15a92f3885739d451b3b8a777d808a8a0e8668998f47d952245a73269c4b3262",
	"parseLocalReferenceFragment":          "4d6302ea361e0cbbad041ca829cf2bde217b166bae3393410dba6416e63542c8",
	"parseStrictJSON":                      "a9943458caeef195e37d418c001b240164e359182f7202106703bdff8bf5313e",
	"pointerArrayIndex":                    "7d0aa0223071c85620c09eb26d0f8a39fa189016f25ea771579dffd866f19aa8",
	"requestSchema":                        "4f692cb9c9a318dd5676c8a5ae37741c096d29750f8f2262f264caa1b686d285",
	"requireJSONObject":                    "33b87a164b5ace757458cf124b397821ca5655b95846db27d47229e4f8a68e3b",
	"resolveLocalReference":                "6752beeb48061bc8f84eed170986eb2d5eb8a1fcb1679be17fab1388b27143cd",
	"resolvePathItemReference":             "eb9ff248043b93f1cd44bbf8edf99086a8609a5f0790659ab408dfb12abb5504",
	"resolveReferenceChain":                "b3b43c225abc489718fce8a30c58b800b1195bef9340a8a4820ec0c0ff75850e",
	"scanExactExponent":                    "f5432bb1c66532cc4548e0d19f498e0ba694b997d0a0a6c5614707935366465c",
	"scanExactFraction":                    "41bedb658ef8b1a8762127dae698e5493748f72deef49803a4685849bfdf12bb",
	"scanExactInteger":                     "bfa051fb7678e21aabc4d8ed73cb42240054ae7f7bd58212f4610fade7c49118",
	"scanExactNumber":                      "b7b4f5df1c518e8b6701e3b15396598617b166638fe71f5ad0ffadaba2b24bb2",
	"scanExactSign":                        "16bde429c989f524764d90690ec7dbc8950b050a303eaeda915195cef3055a73",
	"scanExponentSign":                     "3dbf965899f5fd7208581b3365a1c2884851d440c5b866ed75160d8ecd480edd",
	"selectJSONMediaType":                  "422e7ff2b61213d4eb27306be1a97e555812a8efe9050b38cbd9badadfd7907c",
	"selectRequestSchema":                  "1b6bc0e0ea2deda88e6717f1feb7c1c60c76f48911ef54eb9afd75598345d65b",
	"sortedObjectNames":                    "f6c61ede89ee13e3f20864d3559f4a087a36bf54623ff3802005295385212332",
	"templatedPathIdentity":                "abb8306553ac526c86dc0ebe1181ecb0cb46112b53e56f7a01972cad4815aece",
	"unescapePointerToken":                 "f0d731eaed75fc233b186228c6449b5f4138b72d777414832ac27d592f7744b6",
	"validMediaTypeOrRange":                "c571d083b104b865234143daabaeb0e529e767fa37c6d371a09e18296b741483",
	"validURIScheme":                       "169fc6462ef250ea52aa6f3bec45c7048bba3eeb73a1f88056a72fc78ff89be5",
	"validateExampleObject":                "0bda1aa143599f1a2c10405de6597c4a759bb4761b0a2da169055be802064525",
	"validateIPLiteral":                    "024f1eefbf8918b0584110bdc93a8674992207489ea1a47e0b866ac8af075b18",
	"validateIPvFuture":                    "02f8b0c0e21f51cf9817540e5697458f3bb7520ef722e190f9ebf3f3ec57565d",
	"validateMediaTypeExample":             "ac6c7327f27c0d0daae2f4fa13c771d0e609e48b4a66a9fbaeb142c581b5bc0c",
	"validateMediaTypeExamples":            "4f261cfdbc7477e40183efb5d5c2ec7236b809109315e2a098fa21180e645dfb",
	"validateMediaTypeFields":              "e6596f4fba135d80381bcebb951ac01b0e440be23778a5093bf7b145a7cecf8c",
	"validateRequestBodyFields":            "2ec3b3e44accc18812662e6315c5bf88592195318ebe7637cf9070e67bc3234c",
	"validateURIAuthority":                 "6ef97e51560202d7573fafb4b8a18361a506632ccba93b24ae1a4b0392fae09a",
	"validateURIComponent":                 "cd638ae951e2060c1c260711f315a340065b1ead6e3c90351b42725eaaed2897",
	"validateURIFragment":                  "7571063cd1833073d7a58211fdc28317921fce86dce623a803c3cfb06bc2915c",
	"validateURIPortSuffix":                "653fde8e3e5b6ffdf36b983ea00f438aa45dddf6d3ee02a7521353d4bc2281f7",
	"validateURIReference":                 "fb72b6715fc011fe6c65259c62b4c2aa3aad4676e9b0cce3dffb6040b7d2b334",
	"yamlContextError":                     "2bc5b10337c870a9e9bd4fc149697337d73a571d6ee20fb7af0eaea2610ed293",
	"yamlMappingKey":                       "785bf32f43a712ec0da0e0cf32cb4cfbc8ba050d8fc4b995e7271e917f229e93",
	"yamlString":                           "6dad290019e81b3e0dda198a81b073c1e9e003c061dc672ed3161cf64300aff5",
}

func operationIDAuthorityFunctions(functions map[*ssa.Function]bool) map[*ssa.Function]bool {
	authorities := make(map[*ssa.Function]bool)

	for function := range functions {
		switch function.Name() {
		case "Build", "parseInput":
			authorities[function] = true
		case "decodeOpenAPIDocument", "selectRequestSchema":
			for reachable := range operationIDRuntimeFunctions(function) {
				authorities[reachable] = true
			}
		}
	}

	return authorities
}

func operationIDAuthorityViolations(
	functions map[*ssa.Function]bool,
	guardPackage *sourceGuardPackage,
) []string {
	var violations []string

	for function := range functions {
		if operationIDTrustedFunction(function, guardPackage) {
			continue
		}

		key := function.String()
		if object, ok := function.Object().(*types.Func); ok {
			key = semanticFunctionKey(object, guardPackage.pkg)
		}

		hash, _ := semanticFunctionBodyHash(function, guardPackage)
		violations = append(violations, key+": untrusted OperationID authority function "+hash)
	}

	return violations
}

func operationIDTrustedFunction(function *ssa.Function, guardPackage *sourceGuardPackage) bool {
	object, ok := function.Object().(*types.Func)
	if !ok {
		return false
	}

	expected := allowedOperationIDAuthorityFunctions[semanticFunctionKey(object, guardPackage.pkg)]
	actual, hashOK := semanticFunctionBodyHash(function, guardPackage)

	return expected != "" && hashOK && actual == expected
}

func semanticFunctionBodyHash(function *ssa.Function, guardPackage *sourceGuardPackage) (string, bool) {
	object, ok := function.Object().(*types.Func)
	if !ok {
		return "", false
	}

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			declared, declarationOK := declaration.(*ast.FuncDecl)
			if declarationOK && guardPackage.info.Defs[declared.Name] == object {
				return semanticNodeHash(declared.Body, guardPackage)
			}
		}
	}

	return "", false
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
	ssaPackage := buildGuardSSA(guardPackage)

	build := ssaPackage.Func("Build")
	if build == nil {
		return nil
	}

	var violations []string

	for function := range operationIDRuntimeFunctions(build) {
		object, ok := function.Object().(*types.Func)
		if !ok || !functionHasLocalSemanticCollection(function) ||
			allowedLocalSemanticSpecification(object, guardPackage) {
			continue
		}

		hash, _ := semanticFunctionBodyHash(function, guardPackage)
		violations = append(violations, semanticFunctionKey(object, guardPackage.pkg)+
			": unapproved local semantic collection "+hash)
	}

	return violations
}

func functionHasLocalSemanticCollection(function *ssa.Function) bool {
	if signature := function.Signature; signature != nil {
		for index := range signature.Results().Len() {
			if semanticCollectionType(signature.Results().At(index).Type()) {
				return true
			}
		}
	}

	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			switch typed := instruction.(type) {
			case *ssa.MakeMap, *ssa.MakeSlice:
				return true
			case *ssa.Alloc:
				if pointer, ok := types.Unalias(typed.Type()).Underlying().(*types.Pointer); ok &&
					semanticCollectionType(pointer.Elem()) {
					return true
				}
			case *ssa.MapUpdate:
				return true
			case *ssa.Store:
				if _, ok := typed.Addr.(*ssa.IndexAddr); ok {
					return true
				}
			case *ssa.Call:
				if builtin, ok := typed.Common().Value.(*ssa.Builtin); ok &&
					slices.Contains([]string{"append", "clear", "copy", "delete"}, builtin.Name()) {
					return true
				}
			}
		}
	}

	return false
}

func semanticCollectionType(valueType types.Type) bool {
	switch types.Unalias(valueType).Underlying().(type) {
	case *types.Array, *types.Map, *types.Slice:
		return true
	default:
		return false
	}
}

func allowedLocalSemanticSpecification(function *types.Func, guardPackage *sourceGuardPackage) bool {
	key := semanticFunctionKey(function, guardPackage.pkg)
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
	}[key]

	for _, file := range guardPackage.files {
		for _, declaration := range file.Decls {
			functionDeclaration, ok := declaration.(*ast.FuncDecl)
			if !ok || guardPackage.info.Defs[functionDeclaration.Name] != function {
				continue
			}

			hash, hashOK := semanticNodeHash(functionDeclaration.Body, guardPackage)
			if !hashOK {
				return false
			}

			if expected != "" {
				return hash == expected
			}

			return strings.Contains(reviewedLocalSemanticFunctionHashes, "\n"+key+" "+hash+"\n")
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
