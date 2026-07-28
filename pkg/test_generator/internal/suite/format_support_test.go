//nolint:godoclint // Public-seam test names are intentionally descriptive.
package suite

import (
	"encoding/json"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage"
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestSuiteCompileEnforcesTheClosedFormatTypeContract(t *testing.T) {
	t.Parallel()

	const unsupported = "legal OpenAPI but unsupported by this tool"

	for _, test := range []struct {
		name     string
		schema   string
		contains string
	}{
		{name: "unsupported", schema: "type: string\nformat: hostname", contains: unsupported},
		{name: "binary", schema: "type: string\nformat: binary", contains: unsupported},
		{name: "wrong pair", schema: "type: string\nformat: int32", contains: "invalid type/format pair"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			compiler := NewCompiler(parseSchemaSource(t, test.schema, "", "create"))
			_, err := compiler.CompileSuite()
			require.Error(t, err)
			require.ErrorContains(t, err, "/format")
			require.ErrorContains(t, err, test.contains)
		})
	}
}

func TestSuiteGeneratesExactSignedUUIDPatternCases(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
format: uuid
pattern: ^a`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	format, err := stringlanguage.Format("uuid")
	require.NoError(t, err)
	pattern, err := stringlanguage.Pattern(`^a`)
	require.NoError(t, err)

	seen := map[string]bool{}

	for _, plannedCase := range compiled.Cases {
		if plannedCase.Expect == ExpectRejected &&
			plannedCase.Source.Keyword != "format" && plannedCase.Source.Keyword != "pattern" {
			continue
		}

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")
			require.Equal(rt, jsonvalue.KindString, value.Kind)

			wantFormat := plannedCase.Expect == ExpectAccepted || plannedCase.Source.Keyword != "format"
			wantPattern := plannedCase.Expect == ExpectAccepted || plannedCase.Source.Keyword != "pattern"
			set, compileErr := stringlanguage.Compile([]stringlanguage.Requirement{
				{Language: format, WantMatch: wantFormat},
				{Language: pattern, WantMatch: wantPattern},
			}, stringlanguage.Length{})
			require.NoError(rt, compileErr)
			require.True(rt, set.Matches(value.String))
		})

		seen[plannedCase.Source.Keyword] = true
	}

	require.True(t, seen[""])
	require.True(t, seen["format"])
	require.True(t, seen["pattern"])
}

func TestSuiteMarksIdenticalAliasesDominated(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
allOf:
  - format: uuid
  - format: uuidv4
  - format: uuid-v4`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	formatPlans := 0

	for _, constraint := range compiled.Constraints {
		if constraint.Source.Keyword == "format" {
			formatPlans++

			require.Equal(t, ObligationDominated, constraint.Outcome)
		}
	}

	require.Equal(t, 3, formatPlans)
}

func TestSuitePasswordIsNoOpWithOrdinarySiblingConstraints(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, `type: string
format: password
pattern: ^a$
minLength: 1
maxLength: 1
enum: [a]`, "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)

	for _, constraint := range compiled.Constraints {
		require.NotEqual(t, "format", constraint.Source.Keyword)
	}
}

func TestSuiteNumericFormatsConstrainDomainsAndGeneration(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		schema  string
		formats []string
	}{
		{
			name: "int32 narrows int64", schema: "type: number\nformat: int64\nallOf:\n  - format: int32",
			formats: []string{"int32"},
		},
		{
			name: "float narrows double", schema: "type: number\nformat: double\nallOf:\n  - format: float",
			formats: []string{"float"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			compiler := NewCompiler(parseSchemaSource(t, test.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)
			domain := mustDomain(t, compiled.Domains, compiled.Root)
			require.Equal(t, test.formats, domain.Number.Formats)

			for _, plannedCase := range compiled.Cases {
				if plannedCase.Expect != ExpectAccepted {
					continue
				}

				rapid.Check(t, func(rt *rapid.T) {
					value := plannedCase.Generator.Draw(rt, "value")
					matches, fitErr := numberFits(value.Number, domain.Number)
					require.NoError(rt, fitErr)
					require.True(rt, matches)
				})
			}
		})
	}
}

func TestSuitePlansOnlyConstructibleNumericFormatFailures(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		inner  NumberConstraints
		outer  NumberConstraints
	}{
		{
			name:   "int32 inside int64",
			schema: "type: number\nformat: int64\nallOf:\n  - format: int32",
			inner:  numericFormatConstraintsForTest(t, "int32"),
			outer:  numericFormatConstraintsForTest(t, "int64"),
		},
		{
			name:   "float inside double",
			schema: "type: number\nformat: double\nallOf:\n  - format: float",
			inner:  numericFormatConstraintsForTest(t, "float"),
			outer:  numericFormatConstraintsForTest(t, "double"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, test.schema, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			outcomes := make(map[ObligationOutcome]int)

			for _, constraint := range compiled.Constraints {
				if constraint.Source.Keyword == "format" {
					outcomes[constraint.Outcome]++
				}
			}

			require.Equal(t, 1, outcomes[ObligationPlanned])
			require.Equal(t, 1, outcomes[ObligationDominated])

			foundFailure := false

			for _, plannedCase := range compiled.Cases {
				if plannedCase.Expect != ExpectRejected || plannedCase.Source.Keyword != "format" {
					continue
				}

				foundFailure = true

				rapid.Check(t, func(rt *rapid.T) {
					value := plannedCase.Generator.Draw(rt, "value")
					require.Equal(rt, jsonvalue.KindNumber, value.Kind)

					matchesInner, fitErr := numberFits(value.Number, test.inner)
					require.NoError(rt, fitErr)
					require.False(rt, matchesInner)

					matchesOuter, fitErr := numberFits(value.Number, test.outer)
					require.NoError(rt, fitErr)
					require.True(rt, matchesOuter)
				})
			}

			require.True(t, foundFailure)
		})
	}
}

func numericFormatConstraintsForTest(t *testing.T, format string) NumberConstraints {
	t.Helper()

	number := NumberConstraints{State: KindUnrestricted}
	err := compileNumberFormat(&number, map[string]json.RawMessage{"format": json.RawMessage(`"` + format + `"`)})
	require.NoError(t, err)

	return number
}
