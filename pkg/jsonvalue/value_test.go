//nolint:godoclint // Internal white-box tests name exact malformed-state coverage matrices.
package jsonvalue

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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

	comparison, err := parse("1e100001").Compare(parse("9e100000"))
	require.NoError(t, err)
	require.Equal(t, 1, comparison)
	comparison, err = parse("-1e100001").Compare(parse("-9e100000"))
	require.NoError(t, err)
	require.Equal(t, -1, comparison)
	comparison, err = parse("1.0").Compare(parse("1e0"))
	require.NoError(t, err)
	require.Zero(t, comparison)

	integer, err := parse("1e100001").IsInteger()
	require.NoError(t, err)
	require.True(t, integer)
	integer, err = parse("1e-100001").IsInteger()
	require.NoError(t, err)
	require.False(t, integer)

	for _, test := range []struct {
		value   string
		divisor string
		want    bool
	}{
		{value: "1e100001", divisor: "2e100000", want: true},
		{value: "1e100001", divisor: "3e100000", want: false},
		{value: "1e-100001", divisor: "5e-100002", want: true},
		{value: "1e-100001", divisor: "3e-100002", want: false},
		{value: "0", divisor: "1e100001", want: true},
	} {
		multiple, multipleErr := parse(test.value).IsMultipleOf(parse(test.divisor))
		require.NoError(t, multipleErr)
		require.Equal(t, test.want, multiple)
	}
}

// TestCompiledNumberOperationsReuseCheckedOperandsAndRejectZeroState covers the prepared-number API.
func TestCompiledNumberOperationsReuseCheckedOperandsAndRejectZeroState(t *testing.T) {
	t.Parallel()

	one, err := ParseNumber("1")
	require.NoError(t, err)
	two, err := ParseNumber("2")
	require.NoError(t, err)

	compiledOne, err := one.Compile()
	require.NoError(t, err)
	comparison, err := two.CompareCompiled(compiledOne)
	require.NoError(t, err)
	require.Equal(t, 1, comparison)

	multiple, err := two.IsMultipleOfCompiled(compiledOne)
	require.NoError(t, err)
	require.True(t, multiple)

	_, err = two.CompareCompiled(CompiledNumber{})
	require.Error(t, err)
	_, err = two.IsMultipleOfCompiled(CompiledNumber{})
	require.Error(t, err)
}

// TestMalformedNumberOperationsReturnErrors verifies every exact-number API rejects invalid public state.
func TestMalformedNumberOperationsReturnErrors(t *testing.T) {
	t.Parallel()

	valid, err := ParseNumber("1")
	require.NoError(t, err)

	malformed := []Number{
		{},
		{Lexeme: "1e+"},
		{Lexeme: "1.0"},
		{Lexeme: "1", Rational: big.NewRat(2, 1)},
	}
	for _, number := range malformed {
		_, compareErr := number.Compare(valid)
		require.Error(t, compareErr)
		_, compareErr = valid.Compare(number)
		require.Error(t, compareErr)

		_, integerErr := number.IsInteger()
		require.Error(t, integerErr)

		_, multipleErr := number.IsMultipleOf(valid)
		require.Error(t, multipleErr)
		_, multipleErr = valid.IsMultipleOf(number)
		require.Error(t, multipleErr)
	}
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

// TestSemanticEqualityMatrix checks all JSON kinds without property-test machinery.
func TestSemanticEqualityMatrix(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		left  string
		right string
	}{
		{name: "null", left: `null`, right: `null`},
		{name: "boolean", left: `true`, right: `true`},
		{name: "numeric aliases", left: `1.0`, right: `1e0`},
		{name: "string escaping", left: `"a"`, right: `"\u0061"`},
		{name: "nested arrays", left: `[1.0,[true,null]]`, right: `[1e0,[true,null]]`},
		{
			name:  "reordered objects",
			left:  `{"n":42.0,"items":[42,true],"nested":{"z":null,"a":"x"}}`,
			right: `{"nested":{"a":"x","z":null},"items":[42e0,true],"n":42}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			left, err := Parse([]byte(test.left))
			require.NoError(t, err)
			right, err := Parse([]byte(test.right))
			require.NoError(t, err)
			encoded, err := right.MarshalJSON()
			require.NoError(t, err)
			restored, err := Parse(encoded)
			require.NoError(t, err)

			require.True(t, left.Equal(left), "equality must be reflexive")
			require.Equal(t, left.Equal(right), right.Equal(left), "equality must be symmetric")
			require.True(t, left.Equal(right))
			require.True(t, right.Equal(restored))
			require.True(t, left.Equal(restored), "equality must be transitive")
		})
	}
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

func TestNumberOperationBranches(t *testing.T) {
	t.Parallel()

	parse := func(lexeme string) Number {
		t.Helper()

		number, err := ParseNumber(lexeme)
		require.NoError(t, err)

		return number
	}

	for _, test := range []struct {
		left  string
		right string
		want  int
	}{
		{left: "0", right: "-1e100001", want: 1},
		{left: "0", right: "1e100001", want: -1},
		{left: "-1e100001", right: "0", want: -1},
		{left: "1e100001", right: "0", want: 1},
		{left: "-1e100001", right: "1e100001", want: -1},
		{left: "1e100001", right: "-1e100001", want: 1},
		{left: "12e100001", right: "11e100001", want: 1},
		{left: "11e100001", right: "12e100001", want: -1},
		{left: "1e100002", right: "10e100001", want: 0},
	} {
		comparison, err := parse(test.left).Compare(parse(test.right))
		require.NoError(t, err)
		require.Equal(t, test.want, comparison)
	}

	integer, err := parse("2").IsInteger()
	require.NoError(t, err)
	require.True(t, integer)

	multiple, err := parse("2").IsMultipleOf(parse("0"))
	require.NoError(t, err)
	require.False(t, multiple)

	require.Zero(t, compareZero(false, "0", false, "0"))
}

func TestMalformedAndDistinctValuesReturnExpectedVerdicts(t *testing.T) {
	t.Parallel()

	invalidKind := Value{Kind: Kind(255)}
	require.False(t, invalidKind.Equal(invalidKind))
	require.False(t, Null().Equal(Bool(false)))

	_, err := invalidKind.Hash()
	require.Error(t, err)

	malformedNumber := Value{Kind: KindNumber, Number: Number{Lexeme: "invalid"}}
	_, err = malformedNumber.MarshalJSON()
	require.Error(t, err)
	_, err = Array([]Value{malformedNumber}).MarshalJSON()
	require.Error(t, err)
	_, err = (Value{Kind: KindObject, Object: []Member{{Name: "value", Value: malformedNumber}}}).MarshalJSON()
	require.Error(t, err)
	_, err = (Value{Kind: KindObject, Object: []Member{{Name: "same"}, {Name: "same"}}}).MarshalJSON()
	require.Error(t, err)
	_, err = (Value{Kind: KindObject, Object: []Member{{Name: string([]byte{0xff})}}}).MarshalJSON()
	require.Error(t, err)

	require.False(t, Array([]Value{Null()}).Equal(Array(nil)))
	require.False(t, Array([]Value{Null()}).Equal(Array([]Value{Bool(false)})))
	require.False(t, (Value{Kind: KindObject, Object: []Member{{Name: "a"}}}).Equal(Value{Kind: KindObject}))
	require.False(t, (Value{Kind: KindObject, Object: []Member{{Name: "a"}, {Name: "a"}}}).Equal(
		Value{Kind: KindObject, Object: []Member{{Name: "a"}, {Name: "b"}}},
	))
	require.False(t, (Value{Kind: KindObject, Object: []Member{{Name: "a"}, {Name: "b"}}}).Equal(
		Value{Kind: KindObject, Object: []Member{{Name: "a"}, {Name: "a"}}},
	))
	require.False(t, (Value{Kind: KindObject, Object: []Member{{Name: "a"}}}).Equal(
		Value{Kind: KindObject, Object: []Member{{Name: "b"}}},
	))
	require.Nil(t, membersByName([]Member{{Name: "a"}, {Name: "a"}}))
}

func TestMalformedJSONDecoderPathsReturnErrors(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"", "[", "[tru]", "[1", `{"a"`, `{"a":}`, `{"a":1`, `{"a":1,true:2}`, "true ?", "true false",
	} {
		_, err := Parse([]byte(input))
		require.Error(t, err, input)
	}

	_, err := ParseNumber("true")
	require.Error(t, err)
	_, err = ParseNumber("1 2")
	require.Error(t, err)

	_, err = decodeDelimitedValue(json.NewDecoder(strings.NewReader("")), ']')
	require.Error(t, err)
	_, err = decodeScalarValue(json.Number("invalid"))
	require.Error(t, err)
	_, err = decodeScalarValue(1)
	require.Error(t, err)
	_, err = decodeObject(json.NewDecoder(strings.NewReader("[1]")))
	require.Error(t, err)
}

func TestDecimalIntegerParsesLongCoefficientExactly(t *testing.T) {
	t.Parallel()

	digits := strings.Repeat("1234567890", 10_000)
	require.Equal(t, digits, decimalInteger(digits).String())
}

func BenchmarkDecimalIntegerLongCoefficient(b *testing.B) {
	digits := strings.Repeat("1234567890", 10_000)

	for b.Loop() {
		decimalInteger(digits)
	}
}

func TestNumberFormattingAndEscapeHelpers(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		digits        string
		exponent      int64
		maximumLength int
		want          string
		ok            bool
	}{
		{digits: "1", exponent: 2, maximumLength: 3, want: "100", ok: true},
		{digits: "1", exponent: 2, maximumLength: 2, ok: false},
		{digits: "1", exponent: -2, maximumLength: 4, want: "0.01", ok: true},
		{digits: "12", exponent: -1, maximumLength: 3, want: "1.2", ok: true},
		{digits: "1", exponent: -10, maximumLength: 3, ok: false},
	} {
		got, ok := formatPlainNumber(test.digits, big.NewInt(test.exponent), test.maximumLength)
		require.Equal(t, test.ok, ok)
		require.Equal(t, test.want, got)
	}

	for _, test := range []struct {
		input []byte
		valid bool
	}{
		{input: []byte(`"\\"`), valid: true},
		{input: []byte(`"\u"`), valid: true},
		{input: []byte(`"\u12"`), valid: true},
		{input: []byte(`"\u12xz"`), valid: true},
		{input: []byte(`"\udc00"`)},
		{input: []byte(`"\ud800\u0041"`)},
		{input: []byte(`"\ud800\uzzzz"`)},
		{input: []byte(`"\ud800\udc00"`), valid: true},
	} {
		err := validateJSONStringEscapes(test.input)
		if test.valid {
			require.NoError(t, err, string(test.input))
		} else {
			require.Error(t, err, string(test.input))
		}
	}

	_, ok := decodeHexQuad([]byte("123"))
	require.False(t, ok)
	decoded, ok := decodeHexQuad([]byte("aF09"))
	require.True(t, ok)
	require.Equal(t, uint16(0xaf09), decoded)

	_, ok = decodeHexQuad([]byte("12x4"))
	require.False(t, ok)
}
