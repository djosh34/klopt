package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCanonicalEnumNormalizesExactNumbers verifies equivalent numeric spellings.
func TestCanonicalEnumNormalizesExactNumbers(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"integer":                   "1",
		"decimal integer":           "1",
		"positive exponent":         "1",
		"fraction and exponent":     "1",
		"fraction":                  "2.5",
		"shifted fraction":          "1.23",
		"shorter positive exponent": "1e3",
		"plain exponent tie":        "0.01",
		"shorter negative exponent": "1e-3",
		"negative":                  "-12e-4",
		"negative zero":             "0",
	}

	inputs := map[string]string{
		"integer":                   "1",
		"decimal integer":           "1.0",
		"positive exponent":         "10e-1",
		"fraction and exponent":     "0.01e2",
		"fraction":                  "2.5000",
		"shifted fraction":          "123e-2",
		"shorter positive exponent": "1000",
		"plain exponent tie":        "0.01",
		"shorter negative exponent": "0.001",
		"negative":                  "-0.00120",
		"negative zero":             "-0e999999999999999999999",
	}

	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			canonical, err := CanonicalEnum(json.RawMessage(input))
			require.NoError(t, err)
			require.Equal(t, tests[name], string(canonical))
			require.True(t, json.Valid(canonical))
		})
	}
}

// TestCanonicalEnumKeepsHugeExponentsBounded verifies exponent magnitude cannot expand output.
func TestCanonicalEnumKeepsHugeExponentsBounded(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"1e1000000",
		"1e-1000000",
		"10e999999999",
		"10e-1000000000",
	} {
		canonical, err := CanonicalEnum(json.RawMessage(input))
		require.NoError(t, err)
		require.LessOrEqual(t, len(canonical), len(input)+1)
		require.True(t, json.Valid(canonical))
	}

	positive, err := CanonicalEnum(json.RawMessage("10e999999999"))
	require.NoError(t, err)
	require.Equal(t, "1e1000000000", string(positive))

	negative, err := CanonicalEnum(json.RawMessage("10e-1000000000"))
	require.NoError(t, err)
	require.Equal(t, "1e-999999999", string(negative))
}

// TestCanonicalEnumNormalizesNestedJSON verifies object ordering and array ordering.
func TestCanonicalEnumNormalizesNestedJSON(t *testing.T) {
	t.Parallel()

	canonical, err := CanonicalEnum(json.RawMessage(`{"b":[1000,{"z":1.0,"a":0.001}],"a":"x"}`))
	require.NoError(t, err)
	require.Equal(t, `{"a":"x","b":[1e3,{"a":1e-3,"z":1}]}`, string(canonical))
}

// TestCanonicalEnumRejectsInvalidOrAmbiguousJSON verifies strict JSON handling.
func TestCanonicalEnumRejectsInvalidOrAmbiguousJSON(t *testing.T) {
	t.Parallel()

	invalidUTF8 := json.RawMessage{'"', 0xff, '"'}
	tests := map[string]json.RawMessage{
		"nil":                    nil,
		"invalid syntax":         json.RawMessage(`true false`),
		"invalid utf8":           invalidUTF8,
		"duplicate name":         json.RawMessage(`{"a":1,"a":2}`),
		"escaped duplicate name": json.RawMessage(`{"a":1,"\u0061":2}`),
		"nested duplicate name":  json.RawMessage(`{"nested":{"a":1,"a":2}}`),
		"lone high surrogate":    json.RawMessage(`"\ud800"`),
		"lone low surrogate":     json.RawMessage(`"\udc00"`),
		"high then non-low":      json.RawMessage(`"\ud800\u0041"`),
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := CanonicalEnum(input)
			require.Error(t, err)
		})
	}
}

// TestCanonicalEnumAcceptsValidSurrogatesAndNestedNames verifies strict checks do not overreject.
func TestCanonicalEnumAcceptsValidSurrogatesAndNestedNames(t *testing.T) {
	t.Parallel()

	canonical, err := CanonicalEnum(json.RawMessage(`{"a":1,"nested":{"a":2},"emoji":"\ud83d\ude00"}`))
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(canonical, &decoded))
	require.Equal(t, "\U0001F600", decoded["emoji"])
}

// TestEnumMarshalAndHashRejectNil verifies nil cannot masquerade as JSON null.
func TestEnumMarshalAndHashRejectNil(t *testing.T) {
	t.Parallel()

	_, err := json.Marshal(Enum(nil))
	require.Error(t, err)

	_, err = Enum(nil).GenerateHash()
	require.Error(t, err)
}

// TestEnumHashUsesCanonicalSemanticValue verifies equivalent JSON values hash equally.
func TestEnumHashUsesCanonicalSemanticValue(t *testing.T) {
	t.Parallel()

	left := Enum(`{"b":1000,"a":[1.0,"x"]}`)
	right := Enum(`{"a":[1e0,"x"],"b":1e3}`)

	leftHash, err := left.GenerateHash()
	require.NoError(t, err)
	rightHash, err := right.GenerateHash()
	require.NoError(t, err)
	require.Equal(t, leftHash, rightHash)
}
