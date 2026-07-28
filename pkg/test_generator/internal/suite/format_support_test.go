//nolint:godoclint // Public-seam test names are intentionally descriptive.
package suite

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Required shared module; the plan forbids changing lint config.
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

func TestSuiteCompileAcceptsEveryLegalFormatTypePair(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		typeName string
		format   string
	}{
		{typeName: "integer", format: "int32"},
		{typeName: "integer", format: "int64"},
		{typeName: "number", format: "int32"},
		{typeName: "number", format: "int64"},
		{typeName: "number", format: "float"},
		{typeName: "number", format: "double"},
		{typeName: "string", format: "uuid"},
		{typeName: "string", format: "uuidv4"},
		{typeName: "string", format: "uuid-v4"},
		{typeName: "string", format: "ipv4"},
		{typeName: "string", format: "cidr"},
		{typeName: "string", format: "ipv4-cidr"},
		{typeName: "string", format: "email"},
		{typeName: "string", format: "byte"},
		{typeName: "string", format: "date"},
		{typeName: "string", format: "date-time"},
		{typeName: "string", format: "password"},
	} {
		t.Run(test.typeName+"/"+test.format, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, fmt.Sprintf(
				"type: %s\nformat: %s", test.typeName, test.format,
			), "", "create"))
			_, err := compiler.CompileSuite()
			require.NoError(t, err)
		})
	}
}

func TestSuiteGeneratesStrictByteValidAndIsolatedInvalidCases(t *testing.T) {
	t.Parallel()

	compiler := NewCompiler(parseSchemaSource(t, "type: string\nformat: byte", "", "create"))
	compiled, err := compiler.CompileSuite()
	require.NoError(t, err)
	language, err := stringlanguage.Format("byte")
	require.NoError(t, err)
	set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
		Language: language, WantMatch: true,
	}}, stringlanguage.Length{})
	require.NoError(t, err)

	seenAccepted := false
	seenFormatRejected := false

	for _, plannedCase := range compiled.Cases {
		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "value")

			if plannedCase.Expect == ExpectAccepted {
				seenAccepted = true

				require.Equal(rt, jsonvalue.KindString, value.Kind)
				_, decodeErr := base64.StdEncoding.Strict().DecodeString(value.String)
				require.NoError(rt, decodeErr)

				return
			}

			if plannedCase.Source.Keyword == "format" {
				seenFormatRejected = true

				require.Equal(rt, jsonvalue.KindString, value.Kind)
				require.False(rt, set.Matches(value.String))
			}
		})
	}

	require.True(t, seenAccepted)
	require.True(t, seenFormatRejected)
}

func TestSuiteGeneratesNativeDateAndDateTimeSignedCases(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		format  string
		pattern string
	}{
		{name: "date", format: "date", pattern: `^2024-`},
		{name: "date-time", format: "date-time", pattern: `Z$`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiler := NewCompiler(parseSchemaSource(t, fmt.Sprintf(
				"type: string\nformat: %s\npattern: '%s'", test.format, test.pattern,
			), "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			language, err := stringlanguage.Format(test.format)
			require.NoError(t, err)
			pattern, err := stringlanguage.Pattern(test.pattern)
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

					set, compileErr := stringlanguage.Compile([]stringlanguage.Requirement{
						{Language: language, WantMatch: plannedCase.Expect == ExpectAccepted || plannedCase.Source.Keyword != "format"},
						{Language: pattern, WantMatch: plannedCase.Expect == ExpectAccepted || plannedCase.Source.Keyword != "pattern"},
					}, stringlanguage.Length{})
					require.NoError(rt, compileErr)
					require.True(rt, set.Matches(value.String))
				})

				seen[plannedCase.Source.Keyword] = true
			}

			require.True(t, seen[""])
			require.True(t, seen["format"])
			require.True(t, seen["pattern"])
		})
	}

	leapCompiler := NewCompiler(parseSchemaSource(t, `type: string
format: date
pattern: '^2024-02-29$'`, "", "create"))
	leapSuite, err := leapCompiler.CompileSuite()
	require.NoError(t, err)

	seenLeap := false

	for _, plannedCase := range leapSuite.Cases {
		if plannedCase.Expect != ExpectAccepted {
			continue
		}

		seenLeap = true

		rapid.Check(t, func(rt *rapid.T) {
			value := plannedCase.Generator.Draw(rt, "leap date")
			require.Equal(rt, jsonvalue.String("2024-02-29"), value)
		})
	}

	require.True(t, seenLeap)
}

func TestSuiteFloatFormatsUseFiniteNativeWidthsAndRejectEmptyRanges(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		format   string
		exponent int
		bitSize  int
	}{
		{format: "float", exponent: 39, bitSize: 32},
		{format: "double", exponent: 309, bitSize: 64},
	} {
		t.Run(test.format, func(t *testing.T) {
			t.Parallel()

			emptyCompiler := NewCompiler(parseJSONSchemaSource(t, fmt.Sprintf(
				`{"type":"number","format":%q,"minimum":%s}`,
				test.format,
				"1"+strings.Repeat("0", test.exponent),
			)))
			_, err := emptyCompiler.CompileSuite()
			require.ErrorContains(t, err, "accepts no JSON value")

			compiler := NewCompiler(parseSchemaSource(t, "type: number\nformat: "+test.format, "", "create"))
			compiled, err := compiler.CompileSuite()
			require.NoError(t, err)

			seenAccepted := false

			for _, plannedCase := range compiled.Cases {
				if plannedCase.Expect != ExpectAccepted {
					continue
				}

				seenAccepted = true

				rapid.Check(t, func(rt *rapid.T) {
					value := plannedCase.Generator.Draw(rt, "value")
					require.Equal(rt, jsonvalue.KindNumber, value.Kind)
					parsed, parseErr := strconv.ParseFloat(value.Number.Lexeme, test.bitSize)
					require.NoError(rt, parseErr)
					require.False(rt, parsed != parsed)
					native, nativeErr := jsonvalue.ParseNumber(
						strconv.FormatFloat(parsed, 'g', -1, test.bitSize),
					)
					require.NoError(rt, nativeErr)
					require.Equal(rt, native.Lexeme, value.Number.Lexeme)
				})
			}

			require.True(t, seenAccepted)
		})
	}
}

func parseJSONSchemaSource(t *testing.T, schema string) oas.Source {
	t.Helper()

	sources, err := oas.Parse([]byte(fmt.Sprintf(
		`{"openapi":"3.0.3","paths":{"/things":{"post":{`+
			`"operationId":"create","requestBody":{"content":{`+
			`"application/json":{"schema":%s}}}}}}}`,
		schema,
	)))
	require.NoError(t, err)

	return sources["create"]
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
