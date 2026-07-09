package domain

import (
	"encoding/json"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Internal domain contract.

	"github.com/stretchr/testify/require"
)

// TestParseArrayFiltersEnumsByItemDomain checks recursive item validation.
func TestParseArrayFiltersEnumsByItemDomain(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		schema string
		want   []types.Enum
	}{
		"numeric constraints": {
			schema: `
type: array
items:
  type: integer
  minimum: 2
  multipleOf: 2
enum:
  - [2]
  - [3]
  - [4]
  - [text]
`,
			want: []types.Enum{types.Enum(`[2]`), types.Enum(`[4]`)},
		},
		"allOf constraints": {
			schema: `
type: array
items:
  allOf:
    - type: string
      minLength: 2
    - type: string
      maxLength: 3
enum:
  - [a]
  - [ab]
  - [abcd]
`,
			want: []types.Enum{types.Enum(`["ab"]`)},
		},
		"nested enum and nullability": {
			schema: `
type: array
items:
  type: string
  nullable: true
  enum: [okay, null]
enum:
  - [bad]
  - [okay]
  - [null]
`,
			want: []types.Enum{types.Enum(`["okay"]`), types.Enum(`[null]`)},
		},
		"trusted pattern and format examples": {
			schema: `
type: array
items:
  type: string
  pattern: '^actual$'
  format: custom
  x-valid-examples: [trusted]
  x-invalid-examples: [actual]
enum:
  - [actual]
  - [trusted]
  - [unlisted]
`,
			want: []types.Enum{types.Enum(`["trusted"]`)},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			domain, err := (&Context{}).ParseArray(rawObjectFromYAML(t, tt.schema))
			require.NoError(t, err)
			require.Equal(t, tt.want, domain.Enum)
		})
	}
}

// TestParseObjectFiltersEnumsByPropertyDomains checks declared and additional values.
func TestParseObjectFiltersEnumsByPropertyDomains(t *testing.T) {
	t.Parallel()

	node := rawObjectFromYAML(t, `
type: object
required: [count]
properties:
  count:
    type: integer
    minimum: 2
    multipleOf: 2
  labels:
    type: array
    items:
      type: string
      minLength: 2
additionalProperties:
  type: boolean
enum:
  - count: 2
    labels: [okay]
    extra: true
  - count: 3
  - count: 2
    labels: [x]
  - count: 2
    extra: nope
  - count: 4
`)

	objectDomain, err := (&Context{}).ParseObject(node)
	require.NoError(t, err)
	require.Equal(t, []types.Enum{
		types.Enum(`{"count":2,"extra":true,"labels":["okay"]}`),
		types.Enum(`{"count":4}`),
	}, objectDomain.Enum)
}

// TestParseArrayRollsBackNestedEnumFailure checks child-domain store atomicity.
func TestParseArrayRollsBackNestedEnumFailure(t *testing.T) {
	t.Parallel()

	seed := &BoolDomain{}
	dc := Context{domainStore: domainStore{seed: struct{}{}}}
	node := rawObjectFromYAML(t, `
type: array
items:
  type: string
enum:
  - [1]
`)

	arrayDomain, err := dc.ParseArray(node)
	require.Error(t, err)
	require.Empty(t, arrayDomain)
	requireDomainStoreDomains(t, &dc, seed)
}

// TestParseArrayRejectsNilParsedItems checks a broken parser result and rollback.
func TestParseArrayRejectsNilParsedItems(t *testing.T) {
	t.Parallel()

	dc := Context{
		domainStore: domainStore{},
		parse: func(_ *json.RawMessage) (types.Domain, error) {
			return nil, nil //nolint:nilnil // Exercise a broken parser result.
		},
	}
	node := rawObjectFromYAML(t, `
type: array
items:
  type: string
`)

	arrayDomain, err := dc.ParseArray(node)
	require.Error(t, err)
	require.Empty(t, arrayDomain)
	require.Empty(t, dc.domainStore)
}

// TestDomainAllowsJSONValueUsesSemanticEnumMembership checks canonical equality.
func TestDomainAllowsJSONValueUsesSemanticEnumMembership(t *testing.T) {
	t.Parallel()

	domain := &NumberDomain{
		Type: "number",
		Enum: []types.Enum{types.Enum(`1e0`)},
	}

	allowed, err := domainAllowsJSONValue(domain, json.RawMessage(`1.0`))
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = domainAllowsJSONValue(domain, json.RawMessage(`2`))
	require.NoError(t, err)
	require.False(t, allowed)
}

// TestDomainAllowsJSONValueDoesNotMutateInputs checks validation is read-only.
func TestDomainAllowsJSONValueDoesNotMutateInputs(t *testing.T) {
	t.Parallel()

	domain := &ObjectDomain{
		Enum: []types.Enum{types.Enum(`{"value":1.0}`)},
		Properties: []Property{
			{Key: "z", Domain: &BoolDomain{}},
			{Key: "value", Domain: &NumberDomain{Type: "integer"}},
		},
	}
	domainBefore := *domain
	domainBefore.Enum = append([]types.Enum(nil), domain.Enum...)
	domainBefore.Properties = append([]Property(nil), domain.Properties...)
	value := json.RawMessage(`{"value":1}`)
	valueBefore := append(json.RawMessage(nil), value...)

	allowed, err := domainAllowsJSONValue(domain, value)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, domainBefore, *domain)
	require.Equal(t, valueBefore, value)
}
