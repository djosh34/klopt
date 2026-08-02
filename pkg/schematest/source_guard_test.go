package schematest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// modulePath is the local module import prefix checked by source guards.
const modulePath = "github.com/djosh34/klopt"

// TestProductionImportsStayCleanRoom forbids semantic production dependencies in non-test sources.
func TestProductionImportsStayCleanRoom(t *testing.T) {
	t.Parallel()

	forbidden := []string{
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

// TestOnlyLockedDeclarationsArePublic keeps the deep public module interface unchanged.
func TestOnlyLockedDeclarationsArePublic(t *testing.T) {
	t.Parallel()

	allowed := map[string]bool{
		"Build":           true,
		"Case":            true,
		"Input":           true,
		"MaxStepsReached": true,
		"Report":          true,
		"SpaceExhausted":  true,
		"StopReason":      true,
	}

	for _, file := range productionGoFiles(t) {
		for _, name := range topLevelExportedNames(parseGoFile(t, file)) {
			require.Truef(t, allowed[name], "unexpected public declaration %s in %s", name, file)
		}
	}
}

// topLevelExportedNames returns exported package-scope declarations.
func topLevelExportedNames(file *ast.File) []string {
	var names []string

	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			if typed.Recv == nil && ast.IsExported(typed.Name.Name) {
				names = append(names, typed.Name.Name)
			}
		case *ast.GenDecl:
			names = append(names, exportedGeneralDeclarationNames(typed)...)
		}
	}

	return names
}

// exportedGeneralDeclarationNames returns exported names from one const, type, or var declaration.
func exportedGeneralDeclarationNames(declaration *ast.GenDecl) []string {
	var names []string

	for _, specification := range declaration.Specs {
		switch spec := specification.(type) {
		case *ast.TypeSpec:
			if ast.IsExported(spec.Name.Name) {
				names = append(names, spec.Name.Name)
			}
		case *ast.ValueSpec:
			names = append(names, exportedValueNames(spec)...)
		}
	}

	return names
}

// exportedValueNames returns exported names from one const or var specification.
func exportedValueNames(specification *ast.ValueSpec) []string {
	var names []string

	for _, name := range specification.Names {
		if ast.IsExported(name.Name) {
			names = append(names, name.Name)
		}
	}

	return names
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

// parseGoFile parses one package source file for guard checks.
func parseGoFile(t *testing.T, file string) *ast.File {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	require.NoError(t, err)

	return parsed
}
