package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Tests exercise shared domain contracts.

	"github.com/stretchr/testify/require"
)

// TestAllOfDomainRemainingBranches exercises error paths shared by allOf merging and hashing.
func TestAllOfDomainRemainingBranches(t *testing.T) {
	t.Run("typed nil allOf", func(t *testing.T) {
		t.Parallel()

		var other *AllOfDomain

		got, err := (&AllOfDomain{}).AllOfMerge(other)
		require.Error(t, err)
		require.Nil(t, got)
	})

	t.Run("merged-only allOf right", func(t *testing.T) {
		t.Parallel()

		left := &AllOfDomain{MergedDomain: &StringDomain{MinLength: 1}}
		right := &AllOfDomain{MergedDomain: &StringDomain{MaxLength: new(5)}}

		got, err := left.AllOfMerge(right)
		require.NoError(t, err)
		require.Equal(t, &AllOfDomain{
			Domains:      []types.Domain{&StringDomain{MaxLength: new(5)}},
			MergedDomain: &StringDomain{MinLength: 1, MaxLength: new(5)},
		}, got)
	})

	t.Run("merged-only allOf right merge error", func(t *testing.T) {
		t.Parallel()

		left := &AllOfDomain{MergedDomain: &StringDomain{}}
		right := &AllOfDomain{MergedDomain: &BoolDomain{}}

		got, err := left.AllOfMerge(right)
		require.Error(t, err)
		require.Nil(t, got)
	})

	t.Run("nil child domain", func(t *testing.T) {
		t.Parallel()

		got, err := (&AllOfDomain{}).AllOfMerge(&AllOfDomain{Domains: []types.Domain{nil}})
		require.Error(t, err)
		require.Nil(t, got)
	})

	t.Run("domain generate hash error", func(t *testing.T) {
		t.Parallel()

		_, err := (&AllOfDomain{Domains: []types.Domain{failingGenerateHashDomain{}}}).GenerateHash()
		require.Error(t, err)
	})

	t.Run("merged domain generate hash error", func(t *testing.T) {
		t.Parallel()

		_, err := (&AllOfDomain{MergedDomain: failingGenerateHashDomain{}}).GenerateHash()
		require.Error(t, err)
	})
}

// TestParseAllOfRemainingBranches exercises allOf parser validation and rollback paths.
func TestParseAllOfRemainingBranches(t *testing.T) {
	t.Run("nil node", func(t *testing.T) {
		t.Parallel()

		allOfDomain, err := (&Context{}).ParseAllOf(nil)
		require.Error(t, err)
		require.Empty(t, allOfDomain)
	})

	t.Run("node must be object", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`null`)
		allOfDomain, err := (&Context{}).ParseAllOf(&raw)
		require.Error(t, err)
		require.Empty(t, allOfDomain)
	})

	t.Run("nullable must be boolean", func(t *testing.T) {
		t.Parallel()

		node := rawObjectFromYAML(t, `
nullable: nope
allOf:
  - type: string
`)
		allOfDomain, err := (&Context{}).ParseAllOf(node)
		require.Error(t, err)
		require.Empty(t, allOfDomain)
	})

	for name, raw := range map[string]string{
		"number":  `{"nullable":true,"allOf":[{"type":"number"}]}`,
		"boolean": `{"nullable":true,"allOf":[{"type":"boolean"}]}`,
		"array":   `{"nullable":true,"allOf":[{"type":"array","items":{}}]}`,
		"object":  `{"nullable":true,"allOf":[{"type":"object"}]}`,
	} {
		t.Run("nullable sibling for "+name, func(t *testing.T) {
			t.Parallel()

			rawMessage := json.RawMessage(raw)
			domain, err := (&Context{}).Parse(&rawMessage)
			require.NoError(t, err)
			require.IsType(t, new(AllOfDomain), domain)
		})
	}

	for name, tt := range map[string]struct {
		secondDomain types.Domain
		secondErr    error
	}{
		"sibling parse error": {secondErr: errors.New("sibling parse failed")},
		"sibling parse nil":   {},
		"sibling merge error": {secondDomain: &BoolDomain{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			parseCall := 0
			dc := Context{parse: func(_ *json.RawMessage) (types.Domain, error) {
				parseCall++
				if parseCall == 1 {
					return &StringDomain{}, nil
				}

				return tt.secondDomain, tt.secondErr
			}}
			node := rawObjectFromYAML(t, `
allOf:
  - type: string
maxLength: 5
`)

			allOfDomain, err := dc.ParseAllOf(node)
			require.Error(t, err)
			require.Empty(t, allOfDomain)
			require.Equal(t, 2, parseCall)
		})
	}
}

// TestParseDomainKindsRemainingBranches covers validation shared by primitive domain parsers.
func TestParseDomainKindsRemainingBranches(t *testing.T) {
	t.Run("required type null", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":null}`)
		_, err := (&Context{}).ParseString(&raw)
		require.Error(t, err)
	})

	for name, parse := range map[string]func(*json.RawMessage) error{
		"array": func(node *json.RawMessage) error {
			_, err := (&Context{}).ParseArray(node)

			return err
		},
		"bool": func(node *json.RawMessage) error {
			_, err := (&Context{}).ParseBool(node)

			return err
		},
		"number": func(node *json.RawMessage) error {
			_, err := (&Context{}).ParseNumber(node)

			return err
		},
		"string": func(node *json.RawMessage) error {
			_, err := (&Context{}).ParseString(node)

			return err
		},
	} {
		t.Run(name+" nil node", func(t *testing.T) {
			t.Parallel()

			err := parse(nil)
			require.Error(t, err)
		})

		t.Run(name+" invalid json", func(t *testing.T) {
			t.Parallel()

			raw := json.RawMessage(`{`)
			err := parse(&raw)
			require.Error(t, err)
		})
	}

	for name, raw := range map[string]string{
		"array nullable null":  `{"type":"array","nullable":null,"items":{}}`,
		"bool nullable null":   `{"type":"boolean","nullable":null}`,
		"number nullable null": `{"type":"number","nullable":null}`,
		"string nullable null": `{"type":"string","nullable":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rawMessage := json.RawMessage(raw)
			domain, err := (&Context{}).Parse(&rawMessage)
			require.Error(t, err)
			require.Nil(t, domain)
		})
	}

	t.Run("array items must be object", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":"array","items":"nope"}`)
		_, err := (&Context{}).ParseArray(&raw)
		require.Error(t, err)
	})

	for name, raw := range map[string]string{
		"exclusiveMinimum null":    `{"type":"number","exclusiveMinimum":null}`,
		"exclusiveMaximum null":    `{"type":"number","exclusiveMaximum":null}`,
		"format null":              `{"type":"number","format":null}`,
		"maximum huge exponent":    `{"type":"number","maximum":1e999999999}`,
		"multipleOf huge exponent": `{"type":"number","multipleOf":1e999999999}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rawMessage := json.RawMessage(raw)
			_, err := (&Context{}).ParseNumber(&rawMessage)
			require.Error(t, err)
		})
	}

	for name, raw := range map[string]string{
		"pattern null":            `{"type":"string","pattern":null}`,
		"format null":             `{"type":"string","format":null}`,
		"x-valid-examples null":   `{"type":"string","pattern":"x","x-valid-examples":null,"x-invalid-examples":["y"]}`,
		"x-invalid-examples null": `{"type":"string","pattern":"x","x-valid-examples":["x"],"x-invalid-examples":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rawMessage := json.RawMessage(raw)
			_, err := (&Context{}).ParseString(&rawMessage)
			require.Error(t, err)
		})
	}

	for _, key := range []string{"title", "description"} {
		for name, value := range map[string]string{"number": "1", "null": "null"} {
			t.Run(key+" rejects "+name, func(t *testing.T) {
				t.Parallel()

				raw := json.RawMessage(fmt.Sprintf(`{"type":"string",%q:%s}`, key, value))
				_, err := (&Context{}).ParseString(&raw)
				require.ErrorContains(t, err, key+" must be string")
			})
		}
	}
}

// TestContextParseDefaultRemainingBranches exercises dispatcher errors and typed parser propagation.
func TestContextParseDefaultRemainingBranches(t *testing.T) {
	t.Run("nil node", func(t *testing.T) {
		t.Parallel()

		domain, err := (&Context{}).Parse(nil)
		require.Error(t, err)
		require.Nil(t, domain)
	})

	t.Run("malformed json", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{`)
		domain, err := (&Context{}).Parse(&raw)
		require.ErrorContains(t, err, "decode schema JSON")
		require.Nil(t, domain)
	})

	t.Run("object-shaped schema without type is rejected", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"properties":{"name":{"type":"string"}}}`)
		domain, err := (&Context{}).Parse(&raw)
		require.Error(t, err)
		require.Nil(t, domain)
	})

	t.Run("null schema", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`null`)
		domain, err := (&Context{}).Parse(&raw)
		require.ErrorContains(t, err, "schema node must be object")
		require.Nil(t, domain)
	})

	for name, raw := range map[string]string{
		"object error": `{"type":"object","minProperties":-1}`,
		"array error":  `{"type":"array","items":null}`,
		"string error": `{"type":"string","minLength":-1}`,
		"number error": `{"type":"number","multipleOf":0}`,
		"bool error":   `{"type":"boolean","enum":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rawMessage := json.RawMessage(raw)
			domain, err := (&Context{}).Parse(&rawMessage)
			require.Error(t, err)
			require.Nil(t, domain)
		})
	}
}

// TestEnumRemainingBranches verifies invalid raw enum values are rejected.
func TestEnumRemainingBranches(t *testing.T) {
	t.Parallel()

	_, err := mergeEnums([]types.Enum{types.Enum(`"ok"`)}, []types.Enum{nil})
	require.Error(t, err)

	_, err = mergeEnums([]types.Enum{types.Enum(`not-json`)}, nil)
	require.Error(t, err)
}

// TestNumberRemainingBranches exercises invalid programmatic number domains and rational formatting.
func TestNumberRemainingBranches(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		left  *NumberDomain
		right *NumberDomain
	}{
		"right minimum invalid": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{Type: "number", Minimum: new(Number("bad"))},
		},
		"left minimum invalid": {
			left:  &NumberDomain{Type: "number", Minimum: new(Number("bad"))},
			right: &NumberDomain{Type: "number"},
		},
		"right maximum invalid": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{Type: "number", Maximum: new(Number("bad"))},
		},
		"left maximum invalid": {
			left:  &NumberDomain{Type: "number", Maximum: new(Number("bad"))},
			right: &NumberDomain{Type: "number"},
		},
		"right multiple invalid": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("bad"))},
		},
		"right multiple not positive": {
			left:  &NumberDomain{Type: "number"},
			right: &NumberDomain{Type: "number", MultipleOf: new(Number("0"))},
		},
		"left multiple invalid": {
			left:  &NumberDomain{Type: "number", MultipleOf: new(Number("bad"))},
			right: &NumberDomain{Type: "number"},
		},
		"left multiple not positive": {
			left:  &NumberDomain{Type: "number", MultipleOf: new(Number("0"))},
			right: &NumberDomain{Type: "number"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.left.AllOfMerge(tt.right)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}

	_, err := compareNumbers(Number("1"), Number("bad"))
	require.Error(t, err)

	_, err = mergeMultipleOf(Number("1"), Number("bad"))
	require.Error(t, err)

	_, err = mergeMultipleOf(Number("1"), Number("0"))
	require.Error(t, err)

	require.Equal(t, "0.2", formatRatDecimal(big.NewRat(1, 5)))
	require.Equal(t, "1/3", formatRatDecimal(big.NewRat(1, 3)))
}

// TestArrayRemainingBranches verifies that item hash errors propagate.
func TestArrayRemainingBranches(t *testing.T) {
	t.Parallel()

	_, err := (&ArrayDomain{Items: failingGenerateHashDomain{}}).GenerateHash()
	require.Error(t, err)
}

// TestObjectRemainingBranches exercises object merge, hash, and parser error paths.
func TestObjectRemainingBranches(t *testing.T) {
	t.Parallel()

	_, keep, err := mergePropertyWithAdditional(
		Property{},
		&ObjectDomain{AdditionalPropertyKind: AdditionalSchema},
	)
	require.Error(t, err)
	require.False(t, keep)

	_, keep, err = mergePropertyWithAdditional(
		Property{},
		&ObjectDomain{AdditionalPropertyKind: AdditionalPropertyKind(99)},
	)
	require.Error(t, err)
	require.False(t, keep)

	_, err = (&ObjectDomain{Properties: []Property{{Domain: failingGenerateHashDomain{}}}}).GenerateHash()
	require.Error(t, err)

	raw := json.RawMessage(`{"type":"string"}`)
	_, err = (&Context{}).ParseObject(&raw)
	require.Error(t, err)

	raw = json.RawMessage(`{"type":"object","enum":[]}`)
	_, err = (&Context{}).ParseObject(&raw)
	require.Error(t, err)
}

// TestStringRemainingBranches exercises invalid enum examples and malformed JSON string values.
func TestStringRemainingBranches(t *testing.T) {
	t.Parallel()

	got, err := (&StringDomain{
		Enum: []types.Enum{types.Enum(`null`), types.Enum(`123`)},
	}).AllOfMerge(&StringDomain{XValidExamples: []string{"123"}})
	require.Error(t, err)
	require.Nil(t, got)

	stringValue, err := unmarshalJSONString(types.Enum(`"bad`))
	require.Error(t, err)
	require.Empty(t, stringValue)
}
