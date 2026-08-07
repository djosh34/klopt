package schematest

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
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

// TestOnlyLockedDeclarationsArePublic keeps the deep public module interface unchanged.
func TestOnlyLockedDeclarationsArePublic(t *testing.T) {
	t.Parallel()

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
	for _, file := range productionGoFiles(t) {
		got = append(got, topLevelExportedNames(parseGoFile(t, file))...)
	}

	slices.Sort(got)
	require.Equal(t, want, got)
}

// TestProductionSourceDoesNotEmbedFixtureAnswers rejects source-specific oracle paths.
func TestProductionSourceDoesNotEmbedFixtureAnswers(t *testing.T) {
	t.Parallel()

	fixtureOperationIDs := fixtureOperationIDs(t)

	for _, file := range productionGoFiles(t) {
		parsed := parseGoFile(t, file)
		for _, literal := range stringLiteralValues(t, parsed) {
			require.NotContainsf(t, literal, "testdata/", "%s refers to schematest fixtures", file)
			require.NotContainsf(t, literal, "alpha_zeta", "%s refers to the alpha/zeta fixture", file)
			require.NotContainsf(t, literal, "request_bodies", "%s refers to the request-body fixture", file)
			require.Falsef(
				t, fixtureOperationIDs[literal],
				"%s contains fixture operationId %q; production must select generically", file, literal,
			)
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if !ok {
				return true
			}

			require.NotContainsf(
				t, map[string]bool{"generatedValidations": true, "RequestValidations": true}, identifier.Name,
				"%s contains generated implementation identifier %s", file, identifier.Name,
			)

			return true
		})

		source, err := os.ReadFile(file)
		require.NoError(t, err)
		require.NotContainsf(t, string(source), "Code generated", "%s copies generated implementation", file)
		require.NotContainsf(t, string(source), "//go:generate", "%s introduces generated semantic code", file)
	}
}

// TestBuildStateRetainsOnlyModelPlanCoverageAndActiveSearch locks the callback-lifetime state boundary.
func TestBuildStateRetainsOnlyModelPlanCoverageAndActiveSearch(t *testing.T) {
	t.Parallel()

	requireStructFields(t, reflect.TypeFor[schemaModel](), map[string]reflect.Type{
		"root": reflect.TypeFor[*schemaNode](),
	})
	requireStructFields(t, reflect.TypeFor[searchPlan](), map[string]reflect.Type{
		"validTargets": reflect.TypeFor[[]validTarget](),
		"faultTargets": reflect.TypeFor[[]faultTarget](),
		"obligations":  reflect.TypeFor[[]obligation](),
	})
	requireStructFields(t, reflect.TypeFor[search](), map[string]reflect.Type{
		"model":    reflect.TypeFor[*schemaModel](),
		"maxSteps": reflect.TypeFor[uint64](),
		"steps":    reflect.TypeFor[uint64](),
	})

	for _, file := range productionGoFiles(t) {
		for _, declaration := range parseGoFile(t, file).Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}

			for _, specification := range general.Specs {
				value, valueOK := specification.(*ast.ValueSpec)
				if !valueOK {
					continue
				}

				for _, name := range value.Names {
					require.Truef(
						t, allowedPackageVariable(name.Name),
						"%s declares package-lifetime state %s", file, name.Name,
					)
				}
			}
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

// fixtureOperationIDs reads test-only selector values that production must never special-case.
func fixtureOperationIDs(t *testing.T) map[string]bool {
	t.Helper()

	_, current, _, ok := runtime.Caller(0)
	require.True(t, ok)

	files, err := filepath.Glob(filepath.Join(filepath.Dir(current), "testdata", "*.yaml"))
	require.NoError(t, err)

	identifiers := make(map[string]bool)

	for _, file := range files {
		source, openErr := os.Open(file)
		require.NoError(t, openErr)

		scanner := bufio.NewScanner(source)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if value, found := strings.CutPrefix(line, "operationId:"); found {
				identifiers[strings.TrimSpace(value)] = true
			}
		}

		require.NoError(t, scanner.Err())
		require.NoError(t, source.Close())
	}

	return identifiers
}

// stringLiteralValues returns decoded source strings for fixture-coupling checks.
func stringLiteralValues(t *testing.T, file *ast.File) []string {
	t.Helper()

	var values []string

	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}

		value, err := strconv.Unquote(literal.Value)
		require.NoError(t, err)

		values = append(values, value)

		return true
	})

	return values
}

// requireStructFields pins the complete long-lived execution state of one private type.
func requireStructFields(t *testing.T, structure reflect.Type, want map[string]reflect.Type) {
	t.Helper()

	require.Equal(t, len(want), structure.NumField())

	for name, fieldType := range want {
		field, found := structure.FieldByName(name)
		require.Truef(t, found, "%s is missing state field %s", structure, name)
		require.Equal(t, fieldType, field.Type)
	}
}

// allowedPackageVariable identifies fixed admission vocabulary and immutable error sentinels.
func allowedPackageVariable(name string) bool {
	switch name {
	case "operationMethods", "schemaFormats", "schemaKeywords", "schemaKinds",
		"errBuildNotImplemented", "errCompositionEditInapplicable", "errFaultNotFound",
		"errMaxSteps", "errNumberEdgeStop":
		return true
	default:
		return false
	}
}
