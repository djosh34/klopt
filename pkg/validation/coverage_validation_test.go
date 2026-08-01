//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package validation

import (
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage" //nolint:depguard // Malformed compiled-state coverage needs the concrete internal type.
	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
	"github.com/stretchr/testify/require"
)

func exactNumber(t *testing.T, value string) jsonvalue.Number {
	t.Helper()

	number, err := jsonvalue.ParseNumber(value)
	require.NoError(t, err)

	return number
}

func exactValue(t *testing.T, value string) jsonvalue.Value {
	t.Helper()

	parsed, err := jsonvalue.Parse([]byte(value))
	require.NoError(t, err)

	return parsed
}

func TestCompiledValidationStateMatrix(t *testing.T) {
	t.Parallel()

	malformed := []*Validation{
		{EnumValidation: EnumValidation{
			Values:      []json.RawMessage{json.RawMessage(`null`)},
			ExactValues: []jsonvalue.Value{{Kind: jsonvalue.Kind(255)}},
		}},
		{EnumValidation: EnumValidation{
			Values:      []json.RawMessage{json.RawMessage(`invalid`)},
			ExactValues: []jsonvalue.Value{jsonvalue.Null()},
		}},
		{EnumValidation: EnumValidation{
			Values:      []json.RawMessage{json.RawMessage(`true`)},
			ExactValues: []jsonvalue.Value{jsonvalue.Null()},
		}},
		{StringValidation: StringValidation{CompiledFormat: new(stringlanguage.Language)}},
		{StringValidation: StringValidation{Format: "email"}},
	}
	for _, validation := range malformed {
		require.Error(t, validateLocalCompiledState(validation))
	}

	shared := new(Validation)
	require.NoError(t, validateCompiledState(&Validation{
		ArrayValidation: ArrayValidation{Items: shared},
		ObjectValidation: ObjectValidation{
			Properties: []PropertyValidation{{Name: "shared", Validation: shared}},
		},
	}))
}

func TestCompiledNumberAndCountStateMatrix(t *testing.T) {
	t.Parallel()

	one := exactNumber(t, "1")
	two := exactNumber(t, "2")
	zero := exactNumber(t, "0")
	invalid := jsonvalue.Number{Lexeme: "invalid"}

	for _, number := range []NumberValidation{
		{Minimum: &NumberBound{Value: "invalid", ExactValue: one}},
		{Minimum: &NumberBound{Value: "1", ExactValue: invalid}},
		{Minimum: &NumberBound{Value: "1", ExactValue: two}},
		{MultipleOf: "1"},
		{MultipleOf: "invalid", ExactMultipleOf: &one},
		{MultipleOf: "1", ExactMultipleOf: &invalid},
		{MultipleOf: "1", ExactMultipleOf: &two},
		{MultipleOf: "0", ExactMultipleOf: &zero},
		{MultipleOf: "-1", ExactMultipleOf: new(jsonvalue.Number)},
	} {
		if number.MultipleOf == "-1" {
			*number.ExactMultipleOf = exactNumber(t, "-1")
		}

		require.Error(t, validateNumberCompiledState(number))
	}

	require.Error(t, validatePositiveNumber(invalid))
	require.Error(t, validatePositiveNumber(zero))
	require.Error(t, validateNonNegativeInteger(invalid))

	for _, bound := range []*CountBound{
		{Value: "invalid", ExactValue: one},
		{Value: "1", ExactValue: invalid},
		{Value: "1", ExactValue: two},
		{Value: "1.5", ExactValue: exactNumber(t, "1.5")},
		{Value: "-1", ExactValue: exactNumber(t, "-1")},
	} {
		require.Error(t, validateCountBound(bound))
	}
}

func TestDecodeInstanceMalformedBranches(t *testing.T) {
	t.Parallel()

	for _, raw := range []json.RawMessage{
		nil,
		json.RawMessage(`"`),
		json.RawMessage(`[`),
		json.RawMessage(`{`),
		json.RawMessage(`invalid`),
	} {
		_, err := decodeInstance(raw)
		require.Error(t, err, string(raw))
	}

	_, err := decodeObjectMembersFrom(&objectDecoderSequence{tokens: []json.Token{json.Delim('{'), 1}})
	require.Error(t, err)

	for _, raw := range [][]byte{
		{},
		[]byte(`[`),
		[]byte(`{"a":`),
		[]byte(`{"a":1`),
		[]byte(`{"a":1,true:2}`),
		[]byte(`{"a":1} true`),
		[]byte(`{"a":1} ?`),
	} {
		_, err := decodeObjectMembers(raw)
		require.Error(t, err, string(raw))
	}
}

type objectDecoderSequence struct {
	tokens []json.Token
	index  int
}

func (sequence *objectDecoderSequence) Token() (json.Token, error) {
	if sequence.index >= len(sequence.tokens) {
		return nil, errors.New("end")
	}

	token := sequence.tokens[sequence.index]
	sequence.index++

	return token, nil
}

func (sequence *objectDecoderSequence) More() bool {
	return sequence.index < len(sequence.tokens)
}

func (sequence *objectDecoderSequence) Decode(any) error {
	return errors.New("decode")
}

func TestValidationKeywordErrorBranches(t *testing.T) {
	t.Parallel()

	invalidNumber := jsonvalue.Number{Lexeme: "invalid"}

	numberValue := instance{raw: json.RawMessage(`1`), kind: jsonvalue.KindNumber, number: exactNumber(t, "1")}
	for _, number := range []NumberValidation{
		{Minimum: &NumberBound{Value: "invalid", ExactValue: invalidNumber}},
		{Maximum: &NumberBound{Value: "invalid", ExactValue: invalidNumber}},
		{MultipleOf: "invalid", ExactMultipleOf: &invalidNumber},
		{Format: "invalid"},
	} {
		require.NotEmpty(t, number.validate(new(Validation), numberValue, "#"))
	}

	for _, format := range []string{"int32", "int64", "float", "double", "invalid"} {
		require.Error(t, validateNumberFormat(invalidNumber, format))
	}

	for _, test := range []struct {
		value   jsonvalue.Number
		minimum jsonvalue.Number
		maximum jsonvalue.Number
	}{
		{value: exactNumber(t, "1.5"), minimum: int64Minimum, maximum: int64Maximum},
		{value: exactNumber(t, "9223372036854775808"), minimum: int64Minimum, maximum: int64Maximum},
		{value: invalidNumber, minimum: int64Minimum, maximum: int64Maximum},
		{value: exactNumber(t, "1"), minimum: invalidNumber, maximum: int64Maximum},
		{value: exactNumber(t, "1"), minimum: int64Minimum, maximum: invalidNumber},
	} {
		require.Error(t, validateSignedInteger(test.value, test.minimum, test.maximum, "int64"))
	}

	require.NotEmpty(t, validateLocalAndAllOf(new(Validation), json.RawMessage(`invalid`), "#"))

	badBound := &CountBound{Value: "invalid", ExactValue: invalidNumber}
	text := instance{kind: jsonvalue.KindString, string: "x"}
	array := instance{kind: jsonvalue.KindArray}
	object := instance{kind: jsonvalue.KindObject}

	require.NotEmpty(t, (StringValidation{MinLength: badBound, MaxLength: badBound}).validate(new(Validation), text, "#"))
	require.NotEmpty(t, (ArrayValidation{MinItems: badBound, MaxItems: badBound}).validate(new(Validation), array, "#"))
	require.NotEmpty(t, (ObjectValidation{MinProperties: badBound, MaxProperties: badBound}).validate(new(Validation), object, "#"))

	format := new(stringlanguage.Language)
	require.NotEmpty(t, (StringValidation{CompiledFormat: format}).validate(new(Validation), text, "#"))

	pattern, err := patternvalidator.Parse("^a$")
	require.NoError(t, err)
	require.NotEmpty(t, (StringValidation{Pattern: "^a$", CompiledPattern: pattern}).validate(
		new(Validation), text, "#",
	))

	malformedInteger := instance{kind: jsonvalue.KindNumber, number: invalidNumber}
	require.NotEmpty(t, (KindValidation{Type: "integer"}).validate(new(Validation), malformedInteger, "#"))

	malformedEnum := instance{raw: json.RawMessage(`invalid`)}
	require.NotEmpty(t, (EnumValidation{ExactValues: []jsonvalue.Value{exactValue(t, "null")}}).validate(
		new(Validation), malformedEnum, "#",
	))
}

func TestValidationSmallHelpers(t *testing.T) {
	t.Parallel()

	for _, kind := range []jsonvalue.Kind{jsonvalue.KindArray, jsonvalue.KindObject, jsonvalue.Kind(255)} {
		require.NotEmpty(t, kindName(kind))
	}

	bound := &CountBound{
		Value:      "1",
		ExactValue: jsonvalue.Number{Lexeme: "1", Rational: big.NewRat(1, 1)},
	}
	comparison, err := compareCount(1, bound)
	require.NoError(t, err)
	require.Zero(t, comparison)
}
