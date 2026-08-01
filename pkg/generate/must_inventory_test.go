//nolint:godoclint // Private AST audit helpers keep the Must boundary inventory executable.
package generate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:cyclop,gocognit,paralleltest // The filesystem audit must not race generated fixture tests.
func TestProductionMustInventory(t *testing.T) {
	root := repoRoot(t)
	allowed := map[string]string{
		"pkg/patternvalidator/pattern_validation.go:MustParse": "Parse",
		"pkg/validation/parse.go:MustCompileStringFormat":      "compileStringFormat",
		"pkg/decode/example/validate.go:mustQueryDecoder":      "validation.NewQueryDecoderFromGenerated",
		"pkg/decode/example/validate.go:mustPathDecoder":       "validation.NewPathDecoderFromGenerated",
	}
	foundDeclarations := make(map[string]bool, len(allowed))
	foundCalls := make(map[string]bool, len(allowed))

	err := filepath.WalkDir(filepath.Join(root, "pkg"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		relative = filepath.ToSlash(relative)

		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}

		for _, declaration := range parsed.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Body == nil {
					continue
				}

				key := relative + ":" + declaration.Name.Name

				mustFunction := strings.Contains(strings.ToLower(declaration.Name.Name), "must")
				if mustFunction {
					expectedCallee, permitted := allowed[key]
					require.True(t, permitted, "unexpected production Must function %s", key)
					assertMustWrapper(t, declaration, expectedCallee)

					foundDeclarations[key] = true
				}

				ast.Inspect(declaration.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}

					name := calledName(call.Fun)
					if name == "panic" {
						require.True(t, mustFunction && allowed[key] != "", "panic outside Must wrapper in %s", key)
					}

					require.NotContains(
						t, strings.ToLower(name), "must",
						"production function %s calls Must boundary %s", key, name,
					)

					return true
				})
			case *ast.GenDecl:
				ast.Inspect(declaration, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}

					name := calledName(call.Fun)
					require.NotEqual(t, "panic", name, "panic in package initializer %s", relative)

					if !strings.Contains(strings.ToLower(name), "must") {
						return true
					}

					calleeKey := productionMustKey(relative, name)
					_, permitted := allowed[calleeKey]
					require.True(t, permitted, "unexpected production Must call %s in %s", name, relative)
					require.Equal(t, "pkg/decode/example/validate.go", relative)

					foundCalls[calleeKey] = true

					return true
				})
			}
		}

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, allowedKeys(allowed), foundDeclarations)
	require.Equal(t, allowedKeys(allowed), foundCalls)
}

func assertMustWrapper(t *testing.T, function *ast.FuncDecl, expectedCallee string) {
	t.Helper()
	require.Len(t, function.Body.List, 3, "%s must only call, panic on error, and return", function.Name.Name)

	assignment, ok := function.Body.List[0].(*ast.AssignStmt)
	require.True(t, ok)
	require.Equal(t, token.DEFINE, assignment.Tok)
	require.Len(t, assignment.Lhs, 2)
	require.Len(t, assignment.Rhs, 1)

	resultIdentifier, ok := assignment.Lhs[0].(*ast.Ident)
	require.True(t, ok)
	errorIdentifier, ok := assignment.Lhs[1].(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "err", errorIdentifier.Name)

	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	require.True(t, ok)
	require.Equal(t, expectedCallee, calledName(call.Fun))
	require.NotContains(t, strings.ToLower(expectedCallee), "must")

	condition, ok := function.Body.List[1].(*ast.IfStmt)
	require.True(t, ok)
	comparison, ok := condition.Cond.(*ast.BinaryExpr)
	require.True(t, ok)
	require.Equal(t, token.NEQ, comparison.Op)
	conditionError, ok := comparison.X.(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "err", conditionError.Name)

	conditionNil, ok := comparison.Y.(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "nil", conditionNil.Name)
	require.Nil(t, condition.Else)
	require.Len(t, condition.Body.List, 1)
	panicStatement, ok := condition.Body.List[0].(*ast.ExprStmt)
	require.True(t, ok)
	panicCall, ok := panicStatement.X.(*ast.CallExpr)
	require.True(t, ok)
	require.Equal(t, "panic", calledName(panicCall.Fun))
	require.Len(t, panicCall.Args, 1)
	panicArgument, ok := panicCall.Args[0].(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "err", panicArgument.Name)

	returned, ok := function.Body.List[2].(*ast.ReturnStmt)
	require.True(t, ok)
	require.Len(t, returned.Results, 1)
	returnedIdentifier, ok := returned.Results[0].(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, resultIdentifier.Name, returnedIdentifier.Name)
}

func calledName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		return calledName(expression.X) + "." + expression.Sel.Name
	default:
		return ""
	}
}

func productionMustKey(file string, name string) string {
	name = name[strings.LastIndex(name, ".")+1:]
	switch name {
	case "MustParse":
		return "pkg/patternvalidator/pattern_validation.go:" + name
	case "MustCompileStringFormat":
		return "pkg/validation/parse.go:" + name
	default:
		return file + ":" + name
	}
}

func allowedKeys(allowed map[string]string) map[string]bool {
	keys := make(map[string]bool, len(allowed))
	for key := range allowed {
		keys[key] = true
	}

	return keys
}
