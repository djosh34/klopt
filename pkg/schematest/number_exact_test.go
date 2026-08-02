package schematest

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExactNumberComparison verifies decimal and exponent forms compare mathematically.
func TestExactNumberComparison(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{name: "equivalent scale", left: "1.00", right: "1", want: 0},
		{name: "equivalent exponent", left: "0.10e1", right: "1", want: 0},
		{name: "beyond binary float integer", left: "9007199254740993", right: "9007199254740992", want: 1},
		{name: "beyond binary float fraction", left: "0.10000000000000000001", right: "0.1", want: 1},
		{name: "huge exponent", left: "1e100000000000000000000", right: "9e99999999999999999999", want: 1},
		{name: "negative", left: "-2.5", right: "-2.4", want: -1},
		{name: "opposite signs", left: "-1", right: "1", want: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			left := requireExactNumber(t, test.left)
			right := requireExactNumber(t, test.right)
			comparison, err := left.compare(right)
			require.NoError(t, err)
			require.Equal(t, test.want, comparison)
		})
	}
}

// TestExactNumberIntegrality verifies integer classification is mathematical, not lexical.
func TestExactNumberIntegrality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		lexeme string
		want   bool
	}{
		{lexeme: "1", want: true},
		{lexeme: "1.0", want: true},
		{lexeme: "100e-2", want: true},
		{lexeme: "1e100000000000000000000", want: true},
		{lexeme: "0.1", want: false},
		{lexeme: "100e-3", want: false},
	}

	for _, test := range tests {
		t.Run(test.lexeme, func(t *testing.T) {
			t.Parallel()

			integral, err := requireExactNumber(t, test.lexeme).isInteger()
			require.NoError(t, err)
			require.Equal(t, test.want, integral)
		})
	}
}

// TestExactNumberMultipleOf verifies exact divisibility across authored scales.
func TestExactNumberMultipleOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		divisor string
		want    bool
	}{
		{name: "decimal multiple", value: "0.3", divisor: "0.1", want: true},
		{name: "decimal nonmultiple", value: "0.3000000000000000000000000001", divisor: "0.1", want: false},
		{name: "small exponents", value: "1e-100", divisor: "1e-101", want: true},
		{name: "large exact integer", value: "9007199254740993", divisor: "3", want: true},
		{name: "fractional quotient", value: "1", divisor: "8", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			multiple, err := requireExactNumber(t, test.value).isMultipleOf(requireExactNumber(t, test.divisor))
			require.NoError(t, err)
			require.Equal(t, test.want, multiple)
		})
	}

	_, err := requireExactNumber(t, "1").isMultipleOf(requireExactNumber(t, "0"))
	require.ErrorContains(t, err, "zero")
}

// TestExactQuantumUsesAuthoredScale verifies boundary quanta preserve authored decimal precision.
func TestExactQuantumUsesAuthoredScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		lexeme string
		scale  string
	}{
		{lexeme: "1.2300", scale: "4"},
		{lexeme: "1.2e-3", scale: "4"},
		{lexeme: "1e2", scale: "0"},
		{lexeme: "1.20e2", scale: "0"},
		{lexeme: "1e-2", scale: "2"},
	}

	numbers := make([]*exactNumber, 0, len(tests))
	for _, test := range tests {
		number := requireExactNumber(t, test.lexeme)
		require.Equal(t, test.scale, number.scale.String())
		numbers = append(numbers, number)
	}

	quantum, err := exactQuantum(numbers...)
	require.NoError(t, err)
	lexeme, err := quantum.canonicalDecimal()
	require.NoError(t, err)
	require.Equal(t, "1e-4", lexeme)
}

// TestExactRationalSerialization verifies finite rationals become JSON decimals and others error.
func TestExactRationalSerialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		numerator   int64
		denominator int64
		want        string
		wantErr     string
	}{
		{name: "half", numerator: 1, denominator: 2, want: "0.5"},
		{name: "eighth", numerator: 1, denominator: 8, want: "0.125"},
		{name: "negative", numerator: -5, denominator: 2, want: "-2.5"},
		{name: "third", numerator: 1, denominator: 3, wantErr: "finite decimal"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			number, err := newExactRational(big.NewInt(test.numerator), big.NewInt(test.denominator))
			require.NoError(t, err)

			encoded, err := marshalStrict(&jsonValue{kind: jsonNumber, number: number})
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, encoded)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.want, string(encoded))
		})
	}
}

// TestEquivalentDecimalProperty checks equivalent powers without relying on floating point.
func TestEquivalentDecimalProperty(t *testing.T) {
	t.Parallel()

	for exponent := -100; exponent <= 100; exponent++ {
		left := requireExactNumber(t, "1e"+strconv.Itoa(exponent))
		right := requireExactNumber(t, "10e"+strconv.Itoa(exponent-1))
		comparison, err := left.compare(right)
		require.NoError(t, err)
		require.Zero(t, comparison)
	}
}

// requireExactNumber parses a test number or fails the test.
func requireExactNumber(t *testing.T, lexeme string) *exactNumber {
	t.Helper()

	number, err := parseExactNumber(lexeme)
	require.NoError(t, err)

	return number
}
