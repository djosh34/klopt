package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseStrictJSON verifies the private strict-JSON ingress seam used by model literals.
func TestParseStrictJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		wantErr string
	}{
		{name: "null", input: []byte(" \nnull\t")},
		{name: "paired surrogate", input: []byte(`"\uD834\uDD1E"`)},
		{name: "all kinds", input: []byte(`{"object":{"array":[null,false,1.25,"text"]}}`)},
		{name: "empty", input: nil, wantErr: "expected JSON value"},
		{name: "whitespace", input: []byte(" \n\t"), wantErr: "expected JSON value"},
		{name: "malformed UTF-8", input: []byte{'"', 0xff, '"'}, wantErr: "UTF-8"},
		{name: "duplicate name", input: []byte(`{"a":1,"a":2}`), wantErr: "duplicate object member"},
		{name: "duplicate decoded name", input: []byte(`{"a":1,"\u0061":2}`), wantErr: "duplicate object member"},
		{name: "trailing value", input: []byte(`null false`), wantErr: "trailing data"},
		{name: "unpaired high surrogate", input: []byte(`"\uD834"`), wantErr: "surrogate"},
		{name: "unpaired low surrogate", input: []byte(`"\uDD1E"`), wantErr: "surrogate"},
		{name: "high surrogate followed by non-low", input: []byte(`"\uD834\u0061"`), wantErr: "surrogate"},
		{name: "raw control", input: []byte("\"line\nfeed\""), wantErr: "control"},
		{name: "bad escape", input: []byte(`"\x20"`), wantErr: "escape"},
		{name: "leading zero", input: []byte(`01`), wantErr: "trailing data"},
		{name: "non-finite", input: []byte(`NaN`), wantErr: "expected JSON value"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseStrictJSON(test.input)
			if test.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

// TestJSONSemanticEquality verifies JSON Schema equality through the private value seam.
func TestJSONSemanticEquality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{name: "equivalent decimal", left: `1`, right: `1.0`, want: true},
		{name: "equivalent exponent", left: `0.10e1`, right: `1`, want: true},
		{name: "decoded string", left: `"a\\b"`, right: `"a\u005Cb"`, want: true},
		{name: "ordered array", left: `[1,2]`, right: `[1.0,2e0]`, want: true},
		{name: "array order differs", left: `[1,2]`, right: `[2,1]`, want: false},
		{name: "object order ignored", left: `{"a":1,"b":[true,null]}`, right: `{"b":[true,null],"a":1.0}`, want: true},
		{name: "object member differs", left: `{"a":1}`, right: `{"a":2}`, want: false},
		{name: "kind differs", left: `false`, right: `0`, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			left, err := parseStrictJSON([]byte(test.left))
			require.NoError(t, err)
			right, err := parseStrictJSON([]byte(test.right))
			require.NoError(t, err)

			equal, err := jsonSemanticEqual(left, right)
			require.NoError(t, err)
			require.Equal(t, test.want, equal)
		})
	}
}

// TestMarshalStrict verifies fixed escaping, key order, and canonical exact numbers.
func TestMarshalStrict(t *testing.T) {
	t.Parallel()

	value, err := parseStrictJSON([]byte(`{"é":"\u0000\n\t\"\\/","a":0.50}`))
	require.NoError(t, err)

	encoded, err := marshalStrict(value)
	require.NoError(t, err)
	require.Equal(t, []byte(`{"a":0.5,"é":"\u0000\n\t\"\\/"}`), encoded)
}

// TestMarshalStrictRejectsMalformedState verifies invariant errors are returned without panics.
func TestMarshalStrictRejectsMalformedState(t *testing.T) {
	t.Parallel()

	cycle := &jsonValue{kind: jsonArray, array: make([]*jsonValue, 1)}
	cycle.array[0] = cycle

	tests := []struct {
		name  string
		value *jsonValue
	}{
		{name: "nil value", value: nil},
		{name: "unknown kind", value: &jsonValue{kind: jsonKind(99)}},
		{name: "nil number", value: &jsonValue{kind: jsonNumber}},
		{name: "nil array", value: &jsonValue{kind: jsonArray}},
		{name: "nil object", value: &jsonValue{kind: jsonObject}},
		{name: "invalid string UTF-8", value: &jsonValue{kind: jsonString, text: string([]byte{0xff})}},
		{name: "nil array member", value: &jsonValue{kind: jsonArray, array: []*jsonValue{nil}}},
		{name: "cycle", value: cycle},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := marshalStrict(test.value)
			require.Error(t, err)
			require.Nil(t, encoded)
		})
	}
}

// TestStrictJSONRoundTripProperty verifies repeated bytes and semantic round trips across representative values.
func TestStrictJSONRoundTripProperty(t *testing.T) {
	t.Parallel()

	sources := []string{
		`null`,
		`true`,
		`-0.000`,
		`12.3400e-2`,
		`"plain"`,
		`"\uD834\uDD1E"`,
		`[1,1.0,1e0,{"z":false,"a":null}]`,
		`{"é":[],"a":{"b":"\u0062"}}`,
	}

	for _, source := range sources {
		value, err := parseStrictJSON([]byte(source))
		require.NoError(t, err)

		first, err := marshalStrict(value)
		require.NoError(t, err)

		for range 10 {
			repeated, repeatErr := marshalStrict(value)
			require.NoError(t, repeatErr)
			require.Equal(t, first, repeated)
		}

		roundTrip, err := parseStrictJSON(first)
		require.NoError(t, err)
		equal, err := jsonSemanticEqual(value, roundTrip)
		require.NoError(t, err)
		require.True(t, equal)
	}
}
