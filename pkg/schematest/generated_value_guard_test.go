//nolint:cyclop,gocognit,godoclint,lll // The test defines a whole-program taint lattice and readable source fixtures.
package schematest

import (
	"fmt"
	"go/types"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"

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
	values  map[ssa.Value]bool
	returns map[*ssa.Function]bool
}

func generatedValueEscapeViolations(guardPackage *sourceGuardPackage) []string {
	ssaPackage := buildGuardSSA(guardPackage)

	build := ssaPackage.Func("Build")
	if build == nil {
		return nil
	}

	functions := generatedRuntimeFunctions(build)
	taint := generatedValueTaint{
		values:  make(map[ssa.Value]bool),
		returns: make(map[*ssa.Function]bool),
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
				}
			}
		}
	}

	violations := generatedValueSinks(functions, taint.values, guardPackage.pkg)
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

				callee := call.Common().StaticCallee()
				if callee == nil || callee.Pkg != build.Pkg || callee.Name() == "parseInput" || callee.Name() == "makePlan" {
					continue
				}

				pending = append(pending, callee)
			}
		}
	}

	return functions
}

func propagateGeneratedValueTaint(
	instruction ssa.Instruction,
	functions map[*ssa.Function]bool,
	taint *generatedValueTaint,
	currentPackage *types.Package,
) bool {
	changed := false

	if stored, ok := instruction.(*ssa.Store); ok && taint.values[stored.Val] && !taint.values[stored.Addr] {
		taint.values[stored.Addr] = true
		changed = true
	}

	call, isCall := instruction.(ssa.CallInstruction)
	if isCall {
		common := call.Common()

		callee := common.StaticCallee()
		if callee != nil && functions[callee] {
			for index, argument := range common.Args {
				if index < len(callee.Params) && taint.values[argument] && !taint.values[callee.Params[index]] {
					taint.values[callee.Params[index]] = true
					changed = true
				}
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
	if !hasValue || taint.values[value] || !generatedValueCarrierType(value.Type(), currentPackage) {
		return changed
	}

	if isCall {
		callee := call.Common().StaticCallee()
		if callee != nil && taint.returns[callee] {
			taint.values[value] = true

			return true
		}

		if builtin, ok := call.Common().Value.(*ssa.Builtin); !ok || builtin.Name() != "append" {
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

//nolint:cyclop // The sink forms are the ownership boundary covered by the ticket.
func generatedValueSinks(
	functions map[*ssa.Function]bool,
	tainted map[ssa.Value]bool,
	currentPackage *types.Package,
) []string {
	var violations []string

	callbackOwners := make(map[*ssa.Function]bool)

	for function := range functions {
		for current := function; current != nil; current = current.Parent() {
			if generatedFunctionHasCallback(current, currentPackage) {
				callbackOwners[function] = true

				break
			}
		}
	}

	for function := range functions {
		for _, block := range function.Blocks {
			callbackSeen := false
			for _, instruction := range block.Instrs {
				if callbackSeen {
					for _, operand := range instruction.Operands(nil) {
						if operand != nil && tainted[*operand] && generatedValueCarrierType((*operand).Type(), currentPackage) {
							violations = append(violations, generatedValuePosition(instruction)+": callback alias used after callback return")

							break
						}
					}
				}

				switch typed := instruction.(type) {
				case *ssa.Store:
					if tainted[typed.Val] && generatedStoreEscapes(typed.Addr) &&
						(generatedGlobalAddress(typed.Addr) || callbackOwners[function]) {
						violations = append(violations, generatedValuePosition(instruction)+": generated value stored in owner state")
					}
				case *ssa.MapUpdate:
					if (tainted[typed.Key] || tainted[typed.Value]) && !tainted[typed.Map] && callbackOwners[function] {
						violations = append(violations, generatedValuePosition(instruction)+": generated value stored in map")
					}
				case *ssa.Send:
					if tainted[typed.X] {
						violations = append(violations, generatedValuePosition(instruction)+": generated value sent to surviving channel")
					}
				}

				call, ok := instruction.(ssa.CallInstruction)
				if ok {
					if builtin, builtinOK := call.Common().Value.(*ssa.Builtin); builtinOK && builtin.Name() == "append" &&
						callbackOwners[function] && generatedCallHasTaintedArgument(call.Common(), tainted) {
						violations = append(violations, generatedValuePosition(instruction)+": generated value appended to callback-lived collection")
					}

					if generatedExternalCallback(call.Common(), currentPackage) {
						callbackSeen = true
					}
				}
			}
		}
	}

	return violations
}

func generatedFunctionHasCallback(function *ssa.Function, currentPackage *types.Package) bool {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			call, ok := instruction.(ssa.CallInstruction)
			if ok && generatedExternalCallback(call.Common(), currentPackage) {
				return true
			}
		}
	}

	return false
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

func generatedStoreEscapes(address ssa.Value) bool {
	switch typed := address.(type) {
	case *ssa.Global:
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

func generatedExternalCallback(common *ssa.CallCommon, currentPackage *types.Package) bool {
	if common.StaticCallee() != nil || common.Signature() == nil || len(common.Args) == 0 {
		return false
	}

	caseType := packageObjectTypeFromTypes(currentPackage, "Case")
	for _, argument := range common.Args {
		if sameGuardType(argument.Type(), caseType) {
			return true
		}
	}

	return false
}

func packageObjectTypeFromTypes(currentPackage *types.Package, name string) types.Type {
	object := currentPackage.Scope().Lookup(name)
	if object == nil {
		return nil
	}

	return object.Type()
}

func generatedValueRootType(valueType types.Type, currentPackage *types.Package) bool {
	valueType = types.Unalias(valueType)
	if pointer, ok := valueType.(*types.Pointer); ok {
		valueType = types.Unalias(pointer.Elem())
	}

	named, ok := valueType.(*types.Named)

	return ok && named.Obj().Pkg() == currentPackage && (named.Obj().Name() == "jsonValue" || named.Obj().Name() == "Case")
}

func generatedValueCarrierType(valueType types.Type, currentPackage *types.Package) bool {
	return generatedValueCarrierTypeSeen(valueType, currentPackage, make(map[types.Type]bool))
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
			if generatedValueCarrierTypeSeen(typed.Field(index).Type(), currentPackage, seen) {
				return true
			}
		}
	}

	return false
}

func generatedValuePosition(instruction ssa.Instruction) string {
	position := instruction.Parent().Prog.Fset.Position(instruction.Pos())
	if !position.IsValid() {
		return instruction.Parent().String()
	}

	return fmt.Sprintf("%s:%d", strings.TrimPrefix(position.Filename, "./"), position.Line)
}
