package jsonvalue

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestParseCanonicalizesSemanticJSON verifies exact numbers, member ordering, and array order.
func TestParseCanonicalizesSemanticJSON(t *testing.T) {
	t.Parallel()

	left, err := Parse([]byte(`{"z":[1.0,0.001],"a":{"b":true,"a":null}}`))
	require.NoError(t, err)
	right, err := Parse([]byte(`{"a":{"a":null,"b":true},"z":[1e0,1e-3]}`))
	require.NoError(t, err)

	require.True(t, left.Equal(right))

	leftHash, err := left.Hash()
	require.NoError(t, err)
	rightHash, err := right.Hash()
	require.NoError(t, err)
	require.Equal(t, leftHash, rightHash)

	encoded, err := left.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, `{"a":{"a":null,"b":true},"z":[1,1e-3]}`, string(encoded))
	require.True(t, json.Valid(encoded))
}

// TestParseNumberRetainsExactRational verifies decimal arithmetic never passes through float64.
func TestParseNumberRetainsExactRational(t *testing.T) {
	t.Parallel()

	number, err := ParseNumber("12345678901234567890.1250")
	require.NoError(t, err)
	require.Equal(t, "12345678901234567890.125", number.Lexeme)

	wantRational, ok := new(big.Rat).SetString("12345678901234567890.125")
	require.True(t, ok)
	require.Equal(t, wantRational, number.Rational)

	huge, err := ParseNumber("10e999999999")
	require.NoError(t, err)
	require.Equal(t, "1e1000000000", huge.Lexeme)
	require.Nil(t, huge.Rational)
}

// TestNumberExactOperationsWithArbitraryExponents verifies symbolic decimal arithmetic.
func TestNumberExactOperationsWithArbitraryExponents(t *testing.T) {
	t.Parallel()

	parse := func(lexeme string) Number {
		t.Helper()

		number, err := ParseNumber(lexeme)
		require.NoError(t, err)

		return number
	}

	require.Equal(t, 1, parse("1e100001").Compare(parse("9e100000")))
	require.Equal(t, -1, parse("-1e100001").Compare(parse("-9e100000")))
	require.Zero(t, parse("1.0").Compare(parse("1e0")))
	require.True(t, parse("1e100001").IsInteger())
	require.False(t, parse("1e-100001").IsInteger())
	require.True(t, parse("1e100001").IsMultipleOf(parse("2e100000")))
	require.False(t, parse("1e100001").IsMultipleOf(parse("3e100000")))
	require.True(t, parse("1e-100001").IsMultipleOf(parse("5e-100002")))
	require.False(t, parse("1e-100001").IsMultipleOf(parse("3e-100002")))
	require.True(t, parse("0").IsMultipleOf(parse("1e100001")))
}

// TestExactNumbersBeyondFloat64RemainDistinct verifies equality never rounds through binary floats.
func TestExactNumbersBeyondFloat64RemainDistinct(t *testing.T) {
	t.Parallel()

	left, err := Parse([]byte("9007199254740992"))
	require.NoError(t, err)
	right, err := Parse([]byte("9007199254740993"))
	require.NoError(t, err)
	equivalent, err := Parse([]byte("90071992547409920e-1"))
	require.NoError(t, err)

	require.False(t, left.Equal(right))
	require.True(t, left.Equal(equivalent))

	leftHash, err := left.Hash()
	require.NoError(t, err)
	rightHash, err := right.Hash()
	require.NoError(t, err)
	require.NotEqual(t, leftHash, rightHash)
}

// TestSemanticEqualityLaws checks equality and canonical encoding over generated trees.
func TestSemanticEqualityLaws(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		integer := rapid.Int64().Draw(rt, "integer")
		left, err := Parse(fmt.Appendf(nil, `{"n":%d.0,"items":[%d,true]}`, integer, integer))
		require.NoError(rt, err)
		right, err := Parse(fmt.Appendf(nil, `{"items":[%de0,true],"n":%d}`, integer, integer))
		require.NoError(rt, err)
		thirdJSON, err := right.MarshalJSON()
		require.NoError(rt, err)
		third, err := Parse(thirdJSON)
		require.NoError(rt, err)

		require.True(rt, left.Equal(left), "equality must be reflexive")
		require.Equal(rt, left.Equal(right), right.Equal(left), "equality must be symmetric")
		require.True(rt, left.Equal(right))
		require.True(rt, right.Equal(third))
		require.True(rt, left.Equal(third), "equality must be transitive")
	})
}

// TestParseRejectsAmbiguousOrInvalidJSON verifies strict semantic decoding.
func TestParseRejectsAmbiguousOrInvalidJSON(t *testing.T) {
	t.Parallel()

	invalidUTF8 := []byte{'"', 0xff, '"'}
	tests := map[string][]byte{
		"nil":                    nil,
		"trailing value":         []byte(`true false`),
		"duplicate name":         []byte(`{"a":1,"a":2}`),
		"escaped duplicate name": []byte(`{"a":1,"\u0061":2}`),
		"invalid utf8":           invalidUTF8,
		"unpaired surrogate":     []byte(`"\ud800"`),
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(input)
			require.Error(t, err)
		})
	}
}

// TestConstructorsDeepCopyNestedValues verifies callers cannot mutate constructed values through aliases.
func TestConstructorsDeepCopyNestedValues(t *testing.T) {
	t.Parallel()

	nested := []Value{String("before")}
	array := Array([]Value{Array(nested)})
	object, err := Object([]Member{{Name: "nested", Value: Array(nested)}})
	require.NoError(t, err)

	nested[0] = String("after")

	arrayJSON, err := array.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `[["before"]]`, string(arrayJSON))

	objectJSON, err := object.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"nested":["before"]}`, string(objectJSON))
}

// TestConstructedValuesEncodeDeterministically verifies constructors copy and sort their input.
func TestConstructedValuesEncodeDeterministically(t *testing.T) {
	t.Parallel()

	members := []Member{{Name: "z", Value: Bool(false)}, {Name: "a", Value: String("λ")}}
	value, err := Object(members)
	require.NoError(t, err)

	members[0].Value = Bool(true)

	encoded, err := value.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, `{"a":"λ","z":false}`, string(encoded))

	_, err = Object([]Member{{Name: "same"}, {Name: "same"}})
	require.ErrorContains(t, err, "duplicate")

	invalidString := String(string([]byte{0xff}))
	_, err = invalidString.MarshalJSON()
	require.ErrorContains(t, err, "valid UTF-8")

	_, err = Object([]Member{{Name: string([]byte{0xff}), Value: Null()}})
	require.ErrorContains(t, err, "valid UTF-8")
}
