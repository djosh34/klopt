package domain

import (
	"encoding/json"
	"testing"

	"decode_and_validate_generator/pkg/test_generator/types" //nolint:depguard // Tests assert the shared enum contract.

	"github.com/stretchr/testify/require"
)

// TestCanonicalEnumsSortsAndDeduplicates verifies enum constraints use set semantics.
func TestCanonicalEnumsSortsAndDeduplicates(t *testing.T) {
	t.Parallel()

	got, err := canonicalEnums([]types.Enum{
		types.Enum(`{"b":2,"a":1}`),
		types.Enum(`"b"`),
		types.Enum(`1.0`),
		types.Enum(`"a"`),
		types.Enum(`1e0`),
		types.Enum(`{"a":1e0,"b":2.0}`),
	})
	require.NoError(t, err)
	require.Equal(t, []types.Enum{
		types.Enum(`"a"`),
		types.Enum(`"b"`),
		types.Enum(`1`),
		types.Enum(`{"a":1,"b":2}`),
	}, got)
}

// TestMergeEnumsIsDeterministic verifies intersection is independent of operand order.
func TestMergeEnumsIsDeterministic(t *testing.T) {
	t.Parallel()

	left := []types.Enum{types.Enum(`"b"`), types.Enum(`1`), types.Enum(`"a"`)}
	right := []types.Enum{types.Enum(`1.0`), types.Enum(`"c"`), types.Enum(`"b"`)}
	want := []types.Enum{types.Enum(`"b"`), types.Enum(`1`)}

	leftRight, err := mergeEnums(left, right)
	require.NoError(t, err)
	require.Equal(t, want, leftRight)

	rightLeft, err := mergeEnums(right, left)
	require.NoError(t, err)
	require.Equal(t, want, rightLeft)
}

// TestEnumConstraintsRejectEmptyAndNilValues verifies invalid programmatic constraints.
func TestEnumConstraintsRejectEmptyAndNilValues(t *testing.T) {
	t.Parallel()

	_, err := mergeEnums([]types.Enum{}, nil)
	require.ErrorContains(t, err, "enum cannot be empty")

	_, err = mergeEnums([]types.Enum{nil}, nil)
	require.ErrorContains(t, err, "enum raw value cannot be nil")

	_, err = (&BoolDomain{Enum: []types.Enum{nil}}).GenerateHash()
	require.ErrorContains(t, err, "enum raw value cannot be nil")
}

// TestParseEnumsReturnsCanonicalSet verifies raw schema enums are sorted and deduplicated.
func TestParseEnumsReturnsCanonicalSet(t *testing.T) {
	t.Parallel()

	got, present, err := parseEnums(JSONKV{
		"enum": json.RawMessage(`[2,1.0,1,{"b":2,"a":1}]`),
	})
	require.NoError(t, err)
	require.True(t, present)
	require.Equal(t, []types.Enum{
		types.Enum(`1`),
		types.Enum(`2`),
		types.Enum(`{"a":1,"b":2}`),
	}, got)
}

// TestFilterEnumsByType verifies type, nullable, and integer filtering.
func TestFilterEnumsByType(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		enums      []types.Enum
		schemaType string
		nullable   bool
		want       []types.Enum
		wantError  string
	}{
		"unconstrained": {
			schemaType: "string",
		},
		"boolean": {
			enums:      []types.Enum{types.Enum(`true`), types.Enum(`"true"`), types.Enum(`null`), types.Enum(`false`)},
			schemaType: "boolean",
			want:       []types.Enum{types.Enum(`false`), types.Enum(`true`)},
		},
		"nullable boolean": {
			enums:      []types.Enum{types.Enum(`true`), types.Enum(`null`)},
			schemaType: "boolean",
			nullable:   true,
			want:       []types.Enum{types.Enum(`null`), types.Enum(`true`)},
		},
		"string": {
			enums:      []types.Enum{types.Enum(`1`), types.Enum(`"a"`)},
			schemaType: "string",
			want:       []types.Enum{types.Enum(`"a"`)},
		},
		"array": {
			enums:      []types.Enum{types.Enum(`{"a":1}`), types.Enum(`[1]`)},
			schemaType: "array",
			want:       []types.Enum{types.Enum(`[1]`)},
		},
		"object": {
			enums:      []types.Enum{types.Enum(`[1]`), types.Enum(`{"a":1}`)},
			schemaType: "object",
			want:       []types.Enum{types.Enum(`{"a":1}`)},
		},
		"number": {
			enums:      []types.Enum{types.Enum(`"1"`), types.Enum(`1.5`), types.Enum(`1`)},
			schemaType: "number",
			want:       []types.Enum{types.Enum(`1`), types.Enum(`1.5`)},
		},
		"integer": {
			enums:      []types.Enum{types.Enum(`1e-3`), types.Enum(`1.5`), types.Enum(`1e3`), types.Enum(`1`)},
			schemaType: "integer",
			want:       []types.Enum{types.Enum(`1`), types.Enum(`1e3`)},
		},
		"only wrong type": {
			enums:      []types.Enum{types.Enum(`null`), types.Enum(`"true"`)},
			schemaType: "boolean",
			wantError:  "enum has no values compatible with boolean schema",
		},
		"unknown type": {
			enums:      []types.Enum{types.Enum(`true`)},
			schemaType: "unknown",
			wantError:  `unsupported enum schema type "unknown"`,
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := filterEnumsByType(testCase.enums, testCase.schemaType, testCase.nullable)
			if testCase.wantError != "" {
				require.EqualError(t, err, testCase.wantError)
				require.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.Equal(t, testCase.want, got)
		})
	}
}

// TestEnumAllowsNull verifies the combined nullable and enum constraint.
func TestEnumAllowsNull(t *testing.T) {
	t.Parallel()

	allowed, err := enumAllowsNull(true, nil)
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = enumAllowsNull(true, []types.Enum{types.Enum(`null`), types.Enum(`"x"`)})
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = enumAllowsNull(true, []types.Enum{types.Enum(`"x"`)})
	require.NoError(t, err)
	require.False(t, allowed)

	allowed, err = enumAllowsNull(false, []types.Enum{types.Enum(`null`)})
	require.NoError(t, err)
	require.False(t, allowed)

	_, err = enumAllowsNull(false, []types.Enum{nil})
	require.Error(t, err)
}

// TestPrimitiveDomainHashesCanonicalizeEnumSets verifies enum order does not affect hashes.
func TestPrimitiveDomainHashesCanonicalizeEnumSets(t *testing.T) {
	t.Parallel()

	left := &BoolDomain{Enum: []types.Enum{types.Enum(`true`), types.Enum(`false`), types.Enum(`true`)}}
	right := &BoolDomain{Enum: []types.Enum{types.Enum(`false`), types.Enum(`true`)}}

	leftHash, err := left.GenerateHash()
	require.NoError(t, err)
	rightHash, err := right.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, leftHash, rightHash)
}

// TestObjectDomainHashCanonicalizesEnumSets verifies object hashes use the shared enum invariant.
func TestObjectDomainHashCanonicalizesEnumSets(t *testing.T) {
	t.Parallel()

	left := &ObjectDomain{Enum: []types.Enum{
		types.Enum(`{"b":2}`), types.Enum(`{"a":1}`), types.Enum(`{"b":2}`),
	}}
	right := &ObjectDomain{Enum: []types.Enum{
		types.Enum(`{"a":1}`), types.Enum(`{"b":2}`),
	}}

	leftHash, err := left.GenerateHash()
	require.NoError(t, err)
	rightHash, err := right.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, leftHash, rightHash)

	_, err = (&ObjectDomain{Enum: []types.Enum{nil}}).GenerateHash()
	require.ErrorContains(t, err, "enum raw value cannot be nil")
}
