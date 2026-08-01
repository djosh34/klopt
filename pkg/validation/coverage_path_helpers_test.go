//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package validation

import (
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

func TestCompiledPathTypeInferenceMatrix(t *testing.T) {
	t.Parallel()

	defaultType, err := compiledPathScalarType()
	require.NoError(t, err)
	require.Equal(t, styleScalarType("string"), defaultType)
	defaultType, err = compiledPathScalarType(
		&Validation{KindValidation: KindValidation{Type: "string"}},
		&Validation{KindValidation: KindValidation{Type: "number"}},
	)
	require.NoError(t, err)
	require.Equal(t, styleScalarType("string"), defaultType)

	invalidNumber := jsonvalue.Value{
		Kind:   jsonvalue.KindNumber,
		Number: jsonvalue.Number{Lexeme: "invalid"},
	}
	for _, validations := range [][]*Validation{
		{{KindValidation: KindValidation{Type: "object"}}},
		{{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{invalidNumber}}}},
	} {
		_, err = compiledPathScalarType(validations...)
		require.Error(t, err)
	}

	arrayValues := []struct {
		values []jsonvalue.Value
		valid  bool
	}{
		{values: nil, valid: true},
		{values: []jsonvalue.Value{exactValue(t, `1`), exactValue(t, `2.5`)}, valid: true},
		{values: []jsonvalue.Value{exactValue(t, `{}`)}},
		{values: []jsonvalue.Value{invalidNumber}},
	}
	for _, test := range arrayValues {
		validation := &Validation{EnumValidation: EnumValidation{
			ExactValues: []jsonvalue.Value{jsonvalue.Array(test.values)},
		}}

		_, err = compiledArrayScalarType(validation)
		if test.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

	_, err = compiledPathPropertyScalarType(nil, []jsonvalue.Value{invalidNumber})
	require.Error(t, err)
	_, err = compiledPathPropertyScalarType(
		[]*Validation{{KindValidation: KindValidation{Type: "object"}}},
		nil,
	)
	require.Error(t, err)
	_, err = compiledPathPropertyScalarType(
		[]*Validation{{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{invalidNumber}}}},
		nil,
	)
	require.Error(t, err)

	var types []string
	require.Error(t, collectCompiledValidationTypes(&Validation{
		AllOfValidations: []*Validation{{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{invalidNumber}}}},
	}, &types))

	object, err := jsonvalue.Object([]jsonvalue.Member{{Name: "", Value: jsonvalue.String("x")}})
	require.NoError(t, err)
	_, _, err = compiledPathProperties(&Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{object}}})
	require.Error(t, err)
}

func TestPrimitiveAnyOfCompilationFailureMatrix(t *testing.T) {
	t.Parallel()

	nested := &Validation{SchemaPointer: "#/nested", AnyOfValidations: []*Validation{new(Validation)}}

	invalidEnum := &Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{{
		Kind:   jsonvalue.KindNumber,
		Number: jsonvalue.Number{Lexeme: "invalid"},
	}}}}
	for _, validation := range []*Validation{
		{SchemaPointer: "#/root", ArrayValidation: ArrayValidation{Items: nested}, AnyOfValidations: []*Validation{new(Validation)}},
		{SchemaPointer: "#/root", AnyOfValidations: []*Validation{nested}},
		{SchemaPointer: "#/root", AnyOfValidations: []*Validation{invalidEnum}},
		{SchemaPointer: "#/root", AnyOfValidations: []*Validation{{KindValidation: KindValidation{Type: "object"}}}},
	} {
		_, err := compilePrimitiveAnyOf(validation)
		require.Error(t, err)
	}

	parentInvalidEnum := &Validation{
		EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{{
			Kind:   jsonvalue.KindNumber,
			Number: jsonvalue.Number{Lexeme: "invalid"},
		}}},
		AnyOfValidations: []*Validation{new(Validation)},
	}
	_, err := compilePrimitiveAnyOf(parentInvalidEnum)
	require.Error(t, err)

	inheritParent := &Validation{
		KindValidation:   KindValidation{Type: "string"},
		AnyOfValidations: []*Validation{new(Validation)},
	}
	candidates, err := compilePrimitiveAnyOf(inheritParent)
	require.NoError(t, err)
	require.Len(t, candidates, 1)

	_, err = compilePathAnyOfCandidates(pathParameter{
		wire:       pathWireSimpleArray,
		validation: &Validation{SchemaPointer: "#/root", AnyOfValidations: []*Validation{new(Validation)}},
	})
	require.Error(t, err)
}

func TestStyleAnyOfPointerRecursionMatrix(t *testing.T) {
	t.Parallel()

	nested := &Validation{SchemaPointer: "#/nested", AnyOfValidations: []*Validation{new(Validation)}}
	for _, validation := range []*Validation{
		{ObjectValidation: ObjectValidation{Properties: []PropertyValidation{{Name: "p", Validation: nested}}}},
		{ObjectValidation: ObjectValidation{AdditionalPropertiesValidation: nested}},
		{AllOfValidations: []*Validation{nested}},
	} {
		require.NotEmpty(t, styleParameterAnyOfPointer(validation))
	}
}

func TestEnumTypeMatrix(t *testing.T) {
	t.Parallel()

	allOfArray := &Validation{AllOfValidations: []*Validation{{
		EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{jsonvalue.Array([]jsonvalue.Value{exactValue(t, `1`)})}},
	}}}

	var items []jsonvalue.Value
	collectEnumArrayItems(allOfArray, &items)
	require.NotEmpty(t, items)

	allOfObject := &Validation{AllOfValidations: []*Validation{{
		EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{exactValue(t, `{"p":1}`)}},
	}}}
	valuesByName := make(map[string][]jsonvalue.Value)
	collectEnumObjectProperties(allOfObject, valuesByName)
	require.Contains(t, valuesByName, "p")
	collectEnumObjectProperties(&Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{exactValue(t, `1`)}}}, valuesByName)

	_, err := homogeneousEnumType([]jsonvalue.Value{
		exactValue(t, `1`),
		{Kind: jsonvalue.KindNumber, Number: jsonvalue.Number{Lexeme: "invalid"}},
	})
	require.Error(t, err)

	for _, values := range [][]jsonvalue.Value{
		{exactValue(t, `1`), exactValue(t, `2.5`)},
		{exactValue(t, `2.5`), exactValue(t, `1`)},
		{exactValue(t, `true`), exactValue(t, `"x"`)},
		{exactValue(t, `null`)},
		{{Kind: jsonvalue.Kind(255)}},
	} {
		_, err = homogeneousEnumType(values)
		require.NoError(t, err)
	}

	for _, value := range []jsonvalue.Value{
		exactValue(t, `[]`), exactValue(t, `{}`), exactValue(t, `null`), {Kind: jsonvalue.Kind(255)},
	} {
		_, err = enumValueType(value)
		require.NoError(t, err)
	}

	_, err = enumValueType(jsonvalue.Value{
		Kind:   jsonvalue.KindNumber,
		Number: jsonvalue.Number{Lexeme: "invalid"},
	})
	require.Error(t, err)
}
