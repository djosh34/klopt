//nolint:cyclop,gocognit,godoclint,lll // The test defines a whole-program taint lattice and readable source fixtures.
package schematest

import (
	"fmt"
	"go/types"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa" //nolint:depguard // SSA is required for the source-local whole-program guard.

	"github.com/stretchr/testify/require"
)

// TestGeneratedValuesDoNotEscapeBuildRuntime enforces callback-lifetime ownership through
// locals, representations, helper calls, and closure captures. Shape checks remain separate:
// this pass follows values rather than authorizing a variable or owner by its name or type.
func TestGeneratedValuesDoNotEscapeBuildRuntime(t *testing.T) {
	t.Parallel()

	require.Empty(t, generatedValueEscapeViolations(productionGuardPackage(t)))
}

func TestGeneratedValueGuardRejectsEscapesThroughRepresentations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
	}{
		{
			name: "package field",
			source: `package schematest
				type Case struct { JSON []byte }; var saved []byte
				func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; saved = current.JSON; yield(current) }`,
		},
		{
			name: "pointer field",
			source: `package schematest
				type Case struct { JSON []byte }; type owner struct { body []byte }
				func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; state := new(owner); state.body = current.JSON; yield(current) }`,
		},
		{
			name: "map",
			source: `package schematest
				type Case struct { JSON []byte }
				func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; saved := make(map[int]string); saved[0] = string(current.JSON); yield(current) }`,
		},
		{
			name: "interface",
			source: `package schematest
				type Case struct { JSON []byte }; var saved any
				func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; saved = any(current); yield(current) }`,
		},
		{
			name: "local archive",
			source: `package schematest
				type Case struct { JSON []byte }
				func Build(yield func(Case)) { saved := make([]string, 0); for range 2 { current := Case{JSON: []byte("null")}; saved = append(saved, string(current.JSON)); yield(current) }; _ = saved }`,
		},
		{
			name: "captured slice",
			source: `package schematest
				type Case struct { JSON []byte }
				func Build(yield func(Case)) { saved := make([][]byte, 0); keep := func(current Case) { saved = append(saved, current.JSON) }; current := Case{JSON: []byte("null")}; keep(current); yield(current); _ = saved }`,
		},
		{
			name: "channel",
			source: `package schematest
				type Case struct { JSON []byte }
				func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; saved := make(chan []byte, 1); saved <- current.JSON; yield(current) }`,
		},
		{
			name: "callback alias",
			source: `package schematest
				type Case struct { JSON []byte }
				func consume([]byte) {}
				func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; alias := current.JSON; yield(current); consume(alias) }`,
		},
		{
			name: "returned captured alias",
			source: `package schematest
				type Case struct { JSON []byte }
				func capture(current Case) func() int { alias := current.JSON; return func() int { return len(alias) } }
				func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; retained := capture(current); yield(current); _ = retained() }`,
		},
		{
			name: "interprocedural return",
			source: `package schematest
				type Case struct { JSON []byte }; var saved string
				func encode(current Case) string { return string(current.JSON) }
				func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; saved = encode(current); yield(current) }`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			violations := generatedValueEscapeViolations(parseGuardPackage(t, map[string]string{"guard.go": test.source}))
			require.NotEmpty(t, violations)
		})
	}
}

func TestGeneratedValueGuardRejectsDerivedAndInterproceduralRetention(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"external derived bytes": `package schematest
			import "bytes"
			type Case struct { JSON []byte }; var saved []byte
			func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; saved = bytes.Clone(current.JSON); yield(current) }`,
		"external derived string": `package schematest
			import "strings"
			type Case struct { JSON []byte }; var saved string
			func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; saved = strings.TrimSpace(string(current.JSON)); yield(current) }`,
		"branch successor": `package schematest
			type Case struct { JSON []byte }
			func consume([]byte) {}
			func Build(yield func(Case) error) error { current := Case{JSON: []byte("null")}; if err := yield(current); err != nil { return err }; consume(current.JSON); return nil }`,
		"indirect local helper": `package schematest
			type Case struct { JSON []byte }; var saved []byte
			func keep(current Case) { saved = current.JSON }
			func Build(yield func(Case)) { helper := keep; current := Case{JSON: []byte("null")}; helper(current); yield(current) }`,
		"pointer helper write": `package schematest
			type Case struct { JSON []byte }; type owner struct { saved []byte }
			func keep(target *owner, value []byte) { target.saved = value }
			func Build(yield func(Case)) { target := new(owner); current := Case{JSON: []byte("null")}; keep(target, current.JSON); yield(current) }`,
		"map helper write": `package schematest
			type Case struct { JSON []byte }
			func keep(target map[int][]byte, value []byte) { target[0] = value }
			func Build(yield func(Case)) { target := make(map[int][]byte); current := Case{JSON: []byte("null")}; keep(target, current.JSON); yield(current) }`,
		"slice helper write": `package schematest
			type Case struct { JSON []byte }
			func keep(target [][]byte, value []byte) { target[0] = value }
			func Build(yield func(Case)) { target := make([][]byte, 1); current := Case{JSON: []byte("null")}; keep(target, current.JSON); yield(current) }`,
		"loop continuation": `package schematest
			type Case struct { JSON []byte }
			func consume([]byte) {}
			func Build(yield func(Case)) { var previous []byte; for range 2 { if previous != nil { consume(previous) }; current := Case{JSON: []byte("null")}; previous = current.JSON; yield(current) } }`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, generatedValueEscapeViolations(parseGuardPackage(t, map[string]string{"guard.go": source})))
		})
	}
}

func TestGeneratedValueGuardRejectsOpaqueCarrierAndCallbackEffects(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"nested carrier result": `package schematest
			import "bytes"
			type Case struct { JSON []byte }; var saved [][]byte
			func Build(yield func(Case)) { current := Case{JSON: []byte("a b")}; saved = bytes.Fields(current.JSON); yield(current) }`,
		"external writable output": `package schematest
			import "encoding/json"
			type Case struct { JSON []byte }
			func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; var output any; _ = json.Unmarshal(current.JSON, &output); yield(current) }`,
		"external retained receiver": `package schematest
			import "bytes"
			type Case struct { JSON []byte }
			func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; var output bytes.Buffer; _, _ = output.Write(current.JSON); yield(current) }`,
		"helper invokes callback": `package schematest
			type Case struct { JSON []byte }
			func invoke(callback func(Case), current Case) { callback(current) }
			func consume([]byte) {}
			func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; alias := current.JSON; invoke(yield, current); consume(alias) }`,
		"callback phi keeps taint": `package schematest
			type Case struct { JSON []byte }
			func consume([]byte) {}
			func Build(yield func(Case) []byte, choose bool) { current := Case{JSON: []byte("null")}; alias := current.JSON; replacement := yield(current); if choose { alias = replacement }; consume(alias) }`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, generatedValueEscapeViolations(parseGuardPackage(t, map[string]string{"guard.go": source})))
		})
	}
}

func TestGeneratedValueGuardRejectsRuntimeJSONAndLoopBackedgeRetention(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"unemitted runtime JSON archive": `package schematest
			type jsonValue struct { text string }; type Case struct{}
			func consume([]*jsonValue) {}
			func Build(yield func(Case)) { current := new(jsonValue); archive := []*jsonValue{current}; yield(Case{}); consume(archive) }`,
		"interface-held runtime JSON archive": `package schematest
			type jsonValue struct { text string }; type Case struct{}
			func consume([]any) {}
			func Build(yield func(Case)) { current := new(jsonValue); archive := []any{current}; yield(Case{}); consume(archive) }`,
		"nested runtime JSON archive": `package schematest
			type jsonValue struct { text string }; type Case struct{}
			func consume(map[string][]any) {}
			func Build(yield func(Case)) { current := new(jsonValue); archive := map[string][]any{"saved": {current}}; yield(Case{}); consume(archive) }`,
		"helper-returned runtime JSON archive": `package schematest
			type jsonValue struct { text string }; type Case struct{}
			func makeCandidate() *jsonValue { return new(jsonValue) }
			func consume([]any) {}
			func Build(yield func(Case)) { archive := []any{makeCandidate()}; yield(Case{}); consume(archive) }`,
		"helper-local runtime JSON": `package schematest
			type jsonValue struct { text string }; type Case struct{}
			func retain(yield func(Case)) { current := new(jsonValue); yield(Case{}); consume(current) }
			func consume(*jsonValue) {}
			func Build(yield func(Case)) { retain(yield) }`,
		"helper out parameter": `package schematest
			type jsonValue struct { text string }; type Case struct{}
			func fill(dst *[]any) { *dst = append(*dst, new(jsonValue)) }
			func consume([]any) {}
			func Build(yield func(Case)) { var archive []any; fill(&archive); yield(Case{}); consume(archive) }`,
		"later loop iteration": `package schematest
			type Case struct { JSON []byte }
			func consume([]byte) {}
			func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; alias := current.JSON; for range 2 { consume(alias); yield(current) } }`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, generatedValueEscapeViolations(parseGuardPackage(t, map[string]string{"guard.go": source})))
		})
	}
}

func TestGeneratedValueGuardAllowsAuthoredJSONMetadata(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type jsonValue struct { text string }; type schemaShape struct { enum []*jsonValue }; type Case struct{}
		func Build(shape *schemaShape, yield func(Case)) { _ = shape.enum; yield(Case{}) }
	`

	require.Empty(t, generatedValueEscapeViolations(parseGuardPackage(t, map[string]string{"guard.go": source})))
}

func TestGeneratedValueGuardAllowsImmediateConsumption(t *testing.T) {
	t.Parallel()

	source := `package schematest
		type Case struct { JSON []byte }
		func consume([]byte) {}
		func Build(yield func(Case)) { current := Case{JSON: []byte("null")}; consume(current.JSON); yield(current) }
	`

	require.Empty(t, generatedValueEscapeViolations(parseGuardPackage(t, map[string]string{"guard.go": source})))
}

// generatedValueTaint is a package-wide fixed point. SSA supplies explicit values for
// conversions, interfaces, aggregate addresses, helper results, and closure bindings.
type generatedValueTaint struct {
	values      map[ssa.Value]bool
	returns     map[*ssa.Function]bool
	backward    bool
	runtimeJSON bool
}

func generatedValueEscapeViolations(guardPackage *sourceGuardPackage) []string {
	ssaPackage := buildGuardSSA(guardPackage)

	build := ssaPackage.Func("Build")
	if build == nil {
		return nil
	}

	functions := generatedRuntimeFunctions(build)
	taint := generatedValueTaint{
		values:   make(map[ssa.Value]bool),
		returns:  make(map[*ssa.Function]bool),
		backward: true,
	}
	runtimeJSON := generatedValueTaint{
		values:      make(map[ssa.Value]bool),
		returns:     make(map[*ssa.Function]bool),
		runtimeJSON: true,
	}

	for function := range functions {
		for _, parameter := range function.Params {
			if generatedValueRootType(parameter.Type(), guardPackage.pkg) {
				taint.values[parameter] = true
			}
		}

		for _, freeVariable := range function.FreeVars {
			if generatedValueRootType(freeVariable.Type(), guardPackage.pkg) {
				taint.values[freeVariable] = true
			}
		}

		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				value, ok := instruction.(ssa.Value)
				if ok && generatedValueRootType(value.Type(), guardPackage.pkg) {
					taint.values[value] = true
				}

				if ok && generatedRuntimeJSONRoot(value, guardPackage.pkg) {
					runtimeJSON.values[value] = true
				}
			}
		}
	}

	for changed := true; changed; {
		changed = false

		for function := range functions {
			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					if propagateGeneratedValueTaint(instruction, functions, &taint, guardPackage.pkg) {
						changed = true
					}

					if propagateGeneratedValueTaint(instruction, functions, &runtimeJSON, guardPackage.pkg) {
						changed = true
					}
				}
			}
		}
	}

	violations := generatedValueSinks(functions, taint.values, guardPackage.pkg, true)
	violations = append(violations, generatedValueSinks(functions, runtimeJSON.values, guardPackage.pkg, false)...)
	slices.Sort(violations)

	return slices.Compact(violations)
}

func buildGuardSSA(guardPackage *sourceGuardPackage) *ssa.Package {
	program := ssa.NewProgram(guardPackage.fset, ssa.InstantiateGenerics)
	created := make(map[*types.Package]bool)

	var addImports func(*types.Package)

	addImports = func(current *types.Package) {
		if created[current] {
			return
		}

		created[current] = true
		for _, imported := range current.Imports() {
			addImports(imported)
		}

		if current != guardPackage.pkg {
			program.CreatePackage(current, nil, nil, true)
		}
	}
	addImports(guardPackage.pkg)

	ssaPackage := program.CreatePackage(guardPackage.pkg, guardPackage.files, guardPackage.info, true)
	ssaPackage.Build()

	return ssaPackage
}

func generatedRuntimeFunctions(build *ssa.Function) map[*ssa.Function]bool {
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

				for callee := range possibleLocalCallees(call.Common(), build.Pkg) {
					if callee.Name() != "parseInput" && callee.Name() != "makePlan" {
						pending = append(pending, callee)
					}
				}
			}
		}
	}

	return functions
}

func possibleLocalCallees(common *ssa.CallCommon, pkg *ssa.Package) map[*ssa.Function]bool {
	result := resolvedLocalCallees(common.Value, pkg, make(map[ssa.Value]bool))
	if callee := common.StaticCallee(); callee != nil {
		if localSSAPackage(callee) == pkg {
			result[callee] = true
		}

		return result
	}

	if pkg == nil || !common.IsInvoke() && !storedFunctionValue(common.Value) {
		return result
	}

	addMatchingPackageFunctions(result, common.Signature(), common.Method, pkg)

	return result
}

func storedFunctionValue(value ssa.Value) bool {
	switch typed := value.(type) {
	case *ssa.UnOp, *ssa.Field:
		return true
	case *ssa.Phi:
		for _, edge := range typed.Edges {
			if storedFunctionValue(edge) {
				return true
			}
		}
	case *ssa.ChangeType:
		return storedFunctionValue(typed.X)
	case *ssa.Convert:
		return storedFunctionValue(typed.X)
	case *ssa.Parameter, *ssa.FreeVar, *ssa.Function, *ssa.MakeClosure:
		return false
	default:
		return true
	}

	return false
}

func localSSAPackage(function *ssa.Function) *ssa.Package {
	for function != nil {
		if function.Pkg != nil {
			return function.Pkg
		}

		function = function.Parent()
	}

	return nil
}

func addMatchingPackageFunctions(
	result map[*ssa.Function]bool,
	signature *types.Signature,
	invokedMethod *types.Func,
	pkg *ssa.Package,
) {
	if invokedMethod == nil {
		for _, member := range pkg.Members {
			function, ok := member.(*ssa.Function)
			if ok && (types.Identical(function.Signature, signature) ||
				types.AssignableTo(function.Type(), signature)) {
				result[function] = true
			}
		}
	}

	scope := pkg.Pkg.Scope()
	for _, name := range scope.Names() {
		typeName, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}

		for _, receiver := range []types.Type{typeName.Type(), types.NewPointer(typeName.Type())} {
			methodSet := types.NewMethodSet(receiver)
			for index := range methodSet.Len() {
				function := pkg.Prog.MethodValue(methodSet.At(index))
				if function != nil && (types.Identical(function.Signature, signature) ||
					types.AssignableTo(function.Type(), signature)) &&
					(invokedMethod == nil || function.Name() == invokedMethod.Name()) {
					result[function] = true
				}
			}
		}
	}
}

func resolvedLocalCallees(value ssa.Value, pkg *ssa.Package, seen map[ssa.Value]bool) map[*ssa.Function]bool {
	result := make(map[*ssa.Function]bool)
	if value == nil || seen[value] {
		return result
	}

	seen[value] = true

	switch typed := value.(type) {
	case *ssa.Function:
		if typed.Pkg == pkg || typed.Parent() != nil && typed.Parent().Pkg == pkg {
			result[typed] = true
		}
	case *ssa.MakeClosure:
		if function, ok := typed.Fn.(*ssa.Function); ok {
			result[function] = true
		}
	case *ssa.Phi:
		for _, edge := range typed.Edges {
			for function := range resolvedLocalCallees(edge, pkg, seen) {
				result[function] = true
			}
		}
	case *ssa.ChangeType:
		for function := range resolvedLocalCallees(typed.X, pkg, seen) {
			result[function] = true
		}
	case *ssa.Convert:
		for function := range resolvedLocalCallees(typed.X, pkg, seen) {
			result[function] = true
		}
	case *ssa.MakeInterface:
		for function := range resolvedLocalCallees(typed.X, pkg, seen) {
			result[function] = true
		}
	case *ssa.UnOp:
		for function := range resolvedStoredAddressCallees(typed.X, pkg, seen) {
			result[function] = true
		}
	case *ssa.Field:
		for function := range resolvedLocalCallees(typed.X, pkg, seen) {
			result[function] = true
		}
	}

	return result
}

func resolvedStoredAddressCallees(
	address ssa.Value,
	pkg *ssa.Package,
	seen map[ssa.Value]bool,
) map[*ssa.Function]bool {
	result := make(map[*ssa.Function]bool)

	referrers := address.Referrers()
	if referrers == nil {
		return result
	}

	for _, instruction := range *referrers {
		store, ok := instruction.(*ssa.Store)
		if !ok || store.Addr != address {
			continue
		}

		for function := range resolvedLocalCallees(store.Val, pkg, seen) {
			result[function] = true
		}
	}

	return result
}

//nolint:gocyclo,maintidx,nestif // Forward and backward interprocedural propagation share one fixed-point transfer.
func propagateGeneratedValueTaint(
	instruction ssa.Instruction,
	functions map[*ssa.Function]bool,
	taint *generatedValueTaint,
	currentPackage *types.Package,
) bool {
	changed := false

	if updated, ok := instruction.(*ssa.MapUpdate); ok &&
		(taint.values[updated.Key] || taint.values[updated.Value]) && !taint.values[updated.Map] {
		taint.values[updated.Map] = true
		changed = true
	}

	if stored, ok := instruction.(*ssa.Store); ok {
		if taint.values[stored.Val] && !taint.values[stored.Addr] {
			taint.values[stored.Addr] = true
			changed = true
		}

		if taint.values[stored.Addr] && generatedTaintCarrierType(stored.Val.Type(), currentPackage, taint) &&
			!taint.values[stored.Val] {
			taint.values[stored.Val] = true
			changed = true
		}
	}

	call, isCall := instruction.(ssa.CallInstruction)
	if isCall {
		common := call.Common()
		for callee := range possibleLocalCallees(common, localSSAPackage(instruction.Parent())) {
			if !functions[callee] {
				continue
			}

			for index, argument := range common.Args {
				if index >= len(callee.Params) {
					continue
				}

				parameter := callee.Params[index]
				if taint.values[argument] && !taint.values[parameter] {
					taint.values[parameter] = true
					changed = true
				}

				if taint.values[parameter] && generatedTaintCarrierType(argument.Type(), currentPackage, taint) &&
					generatedWritableCarrier(argument.Type(), currentPackage) && !taint.values[argument] {
					taint.values[argument] = true
					changed = true
				}
			}
		}
	}

	if isCall && generatedCallHasTaintedArgument(call.Common(), taint.values) &&
		generatedUnanalyzedCall(call.Common(), localSSAPackage(instruction.Parent())) {
		for _, argument := range call.Common().Args {
			if generatedTaintCarrierType(argument.Type(), currentPackage, taint) &&
				generatedWritableCarrier(argument.Type(), currentPackage) && !taint.values[argument] {
				taint.values[argument] = true
				changed = true
			}
		}
	}

	if returned, ok := instruction.(*ssa.Return); ok {
		for _, result := range returned.Results {
			if taint.values[result] && !taint.returns[returned.Parent()] {
				taint.returns[returned.Parent()] = true
				changed = true
			}
		}
	}

	value, hasValue := instruction.(ssa.Value)
	if hasValue && taint.values[value] {
		if !taint.backward {
			switch instruction.(type) {
			case *ssa.FieldAddr, *ssa.IndexAddr:
				for _, operand := range instruction.Operands(nil) {
					if operand != nil && *operand != nil && generatedRuntimeJSONContainerType((*operand).Type(), currentPackage) &&
						!taint.values[*operand] {
						taint.values[*operand] = true
						changed = true
					}
				}
			}

			return changed
		}

		for _, operand := range instruction.Operands(nil) {
			if operand != nil && *operand != nil && generatedTaintCarrierType((*operand).Type(), currentPackage, taint) &&
				!taint.values[*operand] {
				taint.values[*operand] = true
				changed = true
			}
		}

		if callbackCall, ok := instruction.(ssa.CallInstruction); ok {
			for callee := range possibleLocalCallees(callbackCall.Common(), localSSAPackage(instruction.Parent())) {
				for _, block := range callee.Blocks {
					for _, candidate := range block.Instrs {
						returned, returnOK := candidate.(*ssa.Return)
						if !returnOK {
							continue
						}

						for _, result := range returned.Results {
							if generatedTaintCarrierType(result.Type(), currentPackage, taint) && !taint.values[result] {
								taint.values[result] = true
								changed = true
							}
						}
					}
				}
			}
		}

		return changed
	}

	if !hasValue || !generatedTaintCarrierType(value.Type(), currentPackage, taint) {
		return changed
	}

	if isCall {
		common := call.Common()
		for callee := range possibleLocalCallees(common, localSSAPackage(instruction.Parent())) {
			if taint.returns[callee] {
				taint.values[value] = true

				return true
			}
		}

		if generatedCallHasTaintedArgument(common, taint.values) &&
			!generatedCallbackType(common.Value.Type(), currentPackage) &&
			generatedTaintCarrierType(value.Type(), currentPackage, taint) {
			taint.values[value] = true

			return true
		}

		if builtin, ok := common.Value.(*ssa.Builtin); !ok || builtin.Name() != "append" {
			return changed
		}
	}

	var operands []*ssa.Value

	operands = instruction.Operands(operands)
	for _, operand := range operands {
		if operand != nil && taint.values[*operand] {
			taint.values[value] = true

			return true
		}
	}

	return changed
}

func generatedTaintCarrierType(
	valueType types.Type,
	currentPackage *types.Package,
	taint *generatedValueTaint,
) bool {
	if taint.runtimeJSON {
		return generatedRuntimeJSONContainerType(valueType, currentPackage)
	}

	return generatedValueCarrierType(valueType, currentPackage)
}

//nolint:cyclop,gocyclo // The sink forms and CFG lifetime transitions form one ownership boundary.
func generatedValueSinks(
	functions map[*ssa.Function]bool,
	tainted map[ssa.Value]bool,
	currentPackage *types.Package,
	ownerStorage bool,
) []string {
	var violations []string

	callbacks := generatedCallbackValues(functions, currentPackage)
	callbackOwners := generatedCallbackOwners(functions, callbacks)
	callbackInvokers := generatedCallbackInvokers(functions, callbacks)

	cleared := generatedCallbackResultValues(functions, callbacks)
	for function := range functions {
		callbackAtEntry := generatedCallbackEntryState(function, callbacks, callbackInvokers)
		for _, block := range function.Blocks {
			callbackFromEntry := callbackAtEntry[block]

			callbackSeen := callbackFromEntry
			for _, instruction := range block.Instrs {
				if callbackSeen {
					if _, phi := instruction.(*ssa.Phi); phi {
						continue
					}

					for _, operand := range instruction.Operands(nil) {
						if operand != nil && !cleared[*operand] && tainted[*operand] &&
							generatedValueCarrierType((*operand).Type(), currentPackage) &&
							(!callbackFromEntry || !generatedValueRefreshedBeforeUse(*operand, instruction)) {
							violations = append(violations, generatedValuePosition(instruction)+": callback alias used after callback return")

							break
						}
					}
				}

				switch typed := instruction.(type) {
				case *ssa.Store:
					if tainted[typed.Val] && generatedStoreEscapes(typed.Addr) &&
						(generatedGlobalAddress(typed.Addr) || ownerStorage &&
							(generatedParameterAddress(typed.Addr) || callbackOwners[function])) {
						violations = append(violations, generatedValuePosition(instruction)+": generated value stored in owner state")
					}
				case *ssa.MapUpdate:
					if (generatedGlobalAddress(typed.Map) || ownerStorage &&
						(generatedParameterAddress(typed.Map) || callbackOwners[function])) &&
						(tainted[typed.Key] || tainted[typed.Value]) {
						violations = append(violations, generatedValuePosition(instruction)+": generated value stored in map")
					}
				case *ssa.Send:
					if tainted[typed.X] {
						violations = append(violations, generatedValuePosition(instruction)+": generated value sent to surviving channel")
					}
				}

				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}

				common := call.Common()
				if builtin, builtinOK := common.Value.(*ssa.Builtin); ownerStorage && builtinOK && builtin.Name() == "append" &&
					callbackOwners[function] && generatedCallHasTaintedArgument(common, tainted) {
					violations = append(violations, generatedValuePosition(instruction)+": generated value appended to callback-lived collection")
				}

				if generatedCallInvokesCallback(common, callbacks, callbackInvokers, function.Pkg) {
					callbackSeen = true

					continue
				}

				if generatedCallHasTaintedArgument(common, tainted) && generatedUnanalyzedCall(common, localSSAPackage(function)) &&
					(ownerStorage || callbackSeen) {
					violations = append(violations, generatedValuePosition(instruction)+": generated value escapes through unknown call")
				}
			}
		}
	}

	return violations
}

func generatedValueRefreshedBeforeUse(value ssa.Value, use ssa.Instruction) bool {
	definition, ok := value.(ssa.Instruction)
	if !ok || definition.Block() == nil || use.Block() == nil {
		return false
	}

	if _, phi := definition.(*ssa.Phi); phi {
		return false
	}

	if definition.Block() != use.Block() {
		return definition.Block().Dominates(use.Block()) && generatedBlockCanReach(use.Block(), definition.Block())
	}

	if !generatedBlockCanReach(use.Block(), definition.Block()) {
		return false
	}

	for _, instruction := range use.Block().Instrs {
		if instruction == definition {
			return true
		}

		if instruction == use {
			return false
		}
	}

	return false
}

func generatedBlockCanReach(from, wanted *ssa.BasicBlock) bool {
	seen := map[*ssa.BasicBlock]bool{from: true}
	pending := slices.Clone(from.Succs)

	for len(pending) > 0 {
		block := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		if block == wanted {
			return true
		}

		if seen[block] {
			continue
		}

		seen[block] = true
		pending = append(pending, block.Succs...)
	}

	return false
}

func generatedCallbackValues(functions map[*ssa.Function]bool, currentPackage *types.Package) map[ssa.Value]bool {
	callbacks := make(map[ssa.Value]bool)

	for function := range functions {
		if function.Name() != "Build" {
			continue
		}

		for _, parameter := range function.Params {
			if generatedCallbackType(parameter.Type(), currentPackage) {
				callbacks[parameter] = true
			}
		}
	}

	for changed := true; changed; {
		changed = false

		for function := range functions {
			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					call, ok := instruction.(ssa.CallInstruction)
					if ok {
						for callee := range possibleLocalCallees(call.Common(), function.Pkg) {
							for index, argument := range call.Common().Args {
								if index < len(callee.Params) && callbacks[argument] && !callbacks[callee.Params[index]] {
									callbacks[callee.Params[index]] = true
									changed = true
								}
							}
						}
					}

					value, isValue := instruction.(ssa.Value)
					if !isValue || callbacks[value] {
						continue
					}

					for _, operand := range instruction.Operands(nil) {
						if operand != nil && callbacks[*operand] {
							callbacks[value] = true
							changed = true

							break
						}
					}
				}
			}
		}
	}

	return callbacks
}

func generatedCallbackResultValues(
	functions map[*ssa.Function]bool,
	callbacks map[ssa.Value]bool,
) map[ssa.Value]bool {
	cleared := make(map[ssa.Value]bool)

	for function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)

				value, isValue := instruction.(ssa.Value)
				if ok && isValue && generatedExternalCallback(call.Common(), callbacks) {
					cleared[value] = true
				}
			}
		}
	}

	for changed := true; changed; {
		changed = false

		for function := range functions {
			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					value, ok := instruction.(ssa.Value)
					if !ok || cleared[value] {
						continue
					}

					operands := instruction.Operands(nil)
					hasCleared := false
					allValuesCleared := true

					for _, operand := range operands {
						if operand == nil || *operand == nil {
							continue
						}

						hasCleared = hasCleared || cleared[*operand]
						allValuesCleared = allValuesCleared && cleared[*operand]
					}

					if hasCleared && allValuesCleared {
						cleared[value] = true
						changed = true
					}
				}
			}
		}
	}

	return cleared
}

func generatedCallbackOwners(functions map[*ssa.Function]bool, callbacks map[ssa.Value]bool) map[*ssa.Function]bool {
	owners := make(map[*ssa.Function]bool)

	for function := range functions {
		for _, parameter := range function.Params {
			owners[function] = owners[function] || callbacks[parameter]
		}

		for _, freeVariable := range function.FreeVars {
			owners[function] = owners[function] || callbacks[freeVariable]
		}
	}

	return owners
}

func generatedCallbackInvokers(
	functions map[*ssa.Function]bool,
	callbacks map[ssa.Value]bool,
) map[*ssa.Function]bool {
	invokers := make(map[*ssa.Function]bool)

	for changed := true; changed; {
		changed = false

		for function := range functions {
			if invokers[function] {
				continue
			}

			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					call, ok := instruction.(ssa.CallInstruction)
					if !ok {
						continue
					}

					if generatedExternalCallback(call.Common(), callbacks) {
						invokers[function] = true
						changed = true
					}

					for callee := range possibleLocalCallees(call.Common(), function.Pkg) {
						if invokers[callee] {
							invokers[function] = true
							changed = true
						}
					}
				}
			}
		}
	}

	return invokers
}

func generatedCallInvokesCallback(
	common *ssa.CallCommon,
	callbacks map[ssa.Value]bool,
	invokers map[*ssa.Function]bool,
	currentPackage *ssa.Package,
) bool {
	if generatedExternalCallback(common, callbacks) {
		return true
	}

	for callee := range possibleLocalCallees(common, currentPackage) {
		if invokers[callee] {
			return true
		}
	}

	return false
}

func generatedCallbackEntryState(
	function *ssa.Function,
	callbacks map[ssa.Value]bool,
	invokers map[*ssa.Function]bool,
) map[*ssa.BasicBlock]bool {
	entry := make(map[*ssa.BasicBlock]bool)

	for changed := true; changed; {
		changed = false

		for _, block := range function.Blocks {
			seen := entry[block]
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if ok && generatedCallInvokesCallback(call.Common(), callbacks, invokers, function.Pkg) {
					seen = true
				}
			}

			if !seen {
				continue
			}

			for _, successor := range block.Succs {
				if !entry[successor] {
					entry[successor] = true
					changed = true
				}
			}
		}
	}

	return entry
}

func generatedUnanalyzedCall(common *ssa.CallCommon, currentPackage *ssa.Package) bool {
	if _, builtin := common.Value.(*ssa.Builtin); builtin {
		return false
	}

	if currentPackage != nil && generatedCallbackType(common.Value.Type(), currentPackage.Pkg) {
		return false
	}

	callee := common.StaticCallee()
	if callee != nil {
		return localSSAPackage(callee) != currentPackage
	}

	return len(resolvedLocalCallees(common.Value, currentPackage, make(map[ssa.Value]bool))) == 0
}

func generatedWritableCarrier(valueType types.Type, currentPackage *types.Package) bool {
	switch types.Unalias(valueType).Underlying().(type) {
	case *types.Pointer, *types.Map, *types.Slice, *types.Interface:
		return generatedValueCarrierType(valueType, currentPackage)
	default:
		return false
	}
}

func generatedCallHasTaintedArgument(common *ssa.CallCommon, tainted map[ssa.Value]bool) bool {
	for _, argument := range common.Args {
		if tainted[argument] {
			return true
		}
	}

	return false
}

func generatedGlobalAddress(address ssa.Value) bool {
	switch typed := address.(type) {
	case *ssa.Global:
		return true
	case *ssa.FieldAddr:
		return generatedGlobalAddress(typed.X)
	case *ssa.IndexAddr:
		return generatedGlobalAddress(typed.X)
	}

	return false
}

func generatedParameterAddress(address ssa.Value) bool {
	switch typed := address.(type) {
	case *ssa.Parameter, *ssa.FreeVar:
		return true
	case *ssa.FieldAddr:
		return generatedParameterAddress(typed.X)
	case *ssa.IndexAddr:
		return generatedParameterAddress(typed.X)
	}

	return false
}

func generatedStoreEscapes(address ssa.Value) bool {
	switch typed := address.(type) {
	case *ssa.Global, *ssa.Parameter, *ssa.FreeVar:
		return true
	case *ssa.Alloc:
		return typed.Heap
	case *ssa.FieldAddr:
		return generatedStoreEscapes(typed.X)
	case *ssa.IndexAddr:
		return generatedStoreEscapes(typed.X)
	}

	return false
}

func generatedExternalCallback(common *ssa.CallCommon, callbacks map[ssa.Value]bool) bool {
	return common.StaticCallee() == nil && callbacks[common.Value]
}

func generatedCallbackType(valueType types.Type, currentPackage *types.Package) bool {
	signature, ok := types.Unalias(valueType).Underlying().(*types.Signature)
	if !ok || signature.Params().Len() != 1 {
		return false
	}

	parameterType := signature.Params().At(0).Type()
	if sameGuardType(parameterType, packageObjectTypeFromTypes(currentPackage, "Case")) {
		return true
	}

	jsonValueType := packageObjectTypeFromTypes(currentPackage, "jsonValue")

	return jsonValueType != nil && sameGuardType(parameterType, types.NewPointer(jsonValueType))
}

func packageObjectTypeFromTypes(currentPackage *types.Package, name string) types.Type {
	object := currentPackage.Scope().Lookup(name)
	if object == nil {
		return nil
	}

	return object.Type()
}

func generatedRuntimeJSONContainerType(valueType types.Type, currentPackage *types.Package) bool {
	valueType = types.Unalias(valueType)
	if pointer, ok := valueType.Underlying().(*types.Pointer); ok {
		valueType = types.Unalias(pointer.Elem())
	}

	if named, ok := valueType.(*types.Named); ok && named.Obj().Pkg() == currentPackage &&
		named.Obj().Name() == "jsonValue" {
		return true
	}

	switch valueType.Underlying().(type) {
	case *types.Array, *types.Interface, *types.Map, *types.Slice:
		return generatedValueCarrierType(valueType, currentPackage)
	default:
		return false
	}
}

func generatedRuntimeJSONRoot(value ssa.Value, currentPackage *types.Package) bool {
	allocation, ok := value.(*ssa.Alloc)
	if !ok {
		return false
	}

	pointer, ok := types.Unalias(allocation.Type()).Underlying().(*types.Pointer)

	return ok && sameGuardType(pointer.Elem(), packageObjectTypeFromTypes(currentPackage, "jsonValue"))
}

func generatedValueRootType(valueType types.Type, currentPackage *types.Package) bool {
	valueType = types.Unalias(valueType)
	if pointer, ok := valueType.(*types.Pointer); ok {
		valueType = types.Unalias(pointer.Elem())
	}

	named, ok := valueType.(*types.Named)

	return ok && named.Obj().Pkg() == currentPackage && named.Obj().Name() == "Case"
}

func generatedValueCarrierType(valueType types.Type, currentPackage *types.Package) bool {
	return valueType != nil && generatedValueCarrierTypeSeen(valueType, currentPackage, make(map[types.Type]bool))
}

func generatedValueCarrierTypeSeen(valueType types.Type, currentPackage *types.Package, seen map[types.Type]bool) bool {
	valueType = types.Unalias(valueType)
	if seen[valueType] {
		return false
	}

	seen[valueType] = true

	if generatedValueRootType(valueType, currentPackage) {
		return true
	}

	root := valueType
	if pointer, ok := root.(*types.Pointer); ok {
		root = types.Unalias(pointer.Elem())
	}

	if named, ok := root.(*types.Named); ok && named.Obj().Pkg() == currentPackage && named.Obj().Name() == "jsonValue" {
		return true
	}

	switch typed := valueType.Underlying().(type) {
	case *types.Basic:
		return typed.Kind() == types.String
	case *types.Interface, *types.Signature:
		return true
	case *types.Pointer:
		return generatedValueCarrierTypeSeen(typed.Elem(), currentPackage, seen)
	case *types.Slice:
		if basic, ok := types.Unalias(typed.Elem()).Underlying().(*types.Basic); ok && basic.Kind() == types.Byte {
			return true
		}

		return generatedValueCarrierTypeSeen(typed.Elem(), currentPackage, seen)
	case *types.Array:
		return generatedValueCarrierTypeSeen(typed.Elem(), currentPackage, seen)
	case *types.Map:
		return generatedValueCarrierTypeSeen(typed.Key(), currentPackage, seen) || generatedValueCarrierTypeSeen(typed.Elem(), currentPackage, seen)
	case *types.Chan:
		return generatedValueCarrierTypeSeen(typed.Elem(), currentPackage, seen)
	case *types.Struct:
		for index := range typed.NumFields() {
			if generatedWrapperFieldType(typed.Field(index).Type(), currentPackage, seen) {
				return true
			}
		}

		return false
	}

	return false
}

func generatedWrapperFieldType(
	valueType types.Type,
	currentPackage *types.Package,
	seen map[types.Type]bool,
) bool {
	valueType = types.Unalias(valueType)
	switch typed := valueType.Underlying().(type) {
	case *types.Interface:
		return true
	case *types.Slice:
		if basic, ok := types.Unalias(typed.Elem()).Underlying().(*types.Basic); ok {
			return basic.Kind() == types.Byte
		}

		return generatedWrapperFieldType(typed.Elem(), currentPackage, seen)
	case *types.Array:
		return generatedWrapperFieldType(typed.Elem(), currentPackage, seen)
	case *types.Map:
		return generatedWrapperFieldType(typed.Key(), currentPackage, seen) ||
			generatedWrapperFieldType(typed.Elem(), currentPackage, seen)
	case *types.Struct:
		return generatedValueCarrierTypeSeen(valueType, currentPackage, seen)
	default:
		return false
	}
}

func generatedValuePosition(instruction ssa.Instruction) string {
	position := instruction.Parent().Prog.Fset.Position(instruction.Pos())
	if !position.IsValid() {
		return instruction.Parent().String()
	}

	return fmt.Sprintf("%s:%d", strings.TrimPrefix(position.Filename, "./"), position.Line)
}
