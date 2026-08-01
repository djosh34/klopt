//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package validation

import (
	"testing"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/stretchr/testify/require"
)

func TestQueryTypeInferenceFailureMatrix(t *testing.T) {
	t.Parallel()

	invalidNumber := jsonvalue.Value{
		Kind:   jsonvalue.KindNumber,
		Number: jsonvalue.Number{Lexeme: "invalid"},
	}
	_, err := compiledQueryScalarType(&Validation{KindValidation: KindValidation{Type: "object"}})
	require.Error(t, err)
	_, err = compiledQueryScalarType(&Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{invalidNumber}}})
	require.Error(t, err)

	for _, test := range []struct {
		validation *Validation
		valid      bool
	}{
		{validation: &Validation{ObjectValidation: ObjectValidation{AdditionalPropertiesAllowed: false}}, valid: true},
		{validation: &Validation{ObjectValidation: ObjectValidation{AdditionalPropertiesAllowed: true}}, valid: true},
		{validation: &Validation{ObjectValidation: ObjectValidation{
			AdditionalPropertiesAllowed:    true,
			AdditionalPropertiesValidation: &Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{invalidNumber}}},
		}}},
		{validation: &Validation{ObjectValidation: ObjectValidation{
			AdditionalPropertiesAllowed:    true,
			AdditionalPropertiesValidation: &Validation{KindValidation: KindValidation{Type: "object"}},
		}}},
	} {
		_, err = queryAdditionalPropertiesType(test.validation)
		if test.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

	require.False(t, compiledAdditionalPropertiesAllowed(&Validation{
		ObjectValidation: ObjectValidation{AdditionalPropertiesAllowed: true},
		AllOfValidations: []*Validation{{ObjectValidation: ObjectValidation{AdditionalPropertiesAllowed: false}}},
	}))

	for _, types := range [][]string{
		nil,
		{"string", "string"},
		{"number", "integer"},
		{"integer", "number"},
		{"string", "number"},
	} {
		require.NotNil(t, intersectQuerySchemaTypes(types))
	}
}

func TestQueryPropertyCompilationFailureMatrix(t *testing.T) {
	t.Parallel()

	_, _, err := compileQueryProperties(nil, false)
	require.Error(t, err)

	invalidEnum := &Validation{EnumValidation: EnumValidation{ExactValues: []jsonvalue.Value{{
		Kind:   jsonvalue.KindNumber,
		Number: jsonvalue.Number{Lexeme: "invalid"},
	}}}}
	object := &Validation{KindValidation: KindValidation{Type: "object"}}

	array := &Validation{
		KindValidation:  KindValidation{Type: "array"},
		ArrayValidation: ArrayValidation{Items: object},
	}
	for _, test := range []struct {
		validation *Validation
		arrays     bool
	}{
		{validation: &Validation{ObjectValidation: ObjectValidation{Properties: []PropertyValidation{{Name: "a[b]", Validation: new(Validation)}}}}, arrays: true},
		{validation: &Validation{ObjectValidation: ObjectValidation{Properties: []PropertyValidation{{Name: "p", Validation: invalidEnum}}}}},
		{validation: &Validation{ObjectValidation: ObjectValidation{Properties: []PropertyValidation{{Name: "p", Validation: object}}}}},
		{validation: &Validation{ObjectValidation: ObjectValidation{Properties: []PropertyValidation{{Name: "p", Validation: array}}}}, arrays: true},
	} {
		_, _, err = compileQueryProperties(test.validation, test.arrays)
		require.Error(t, err)
	}
}

func TestQueryAnyOfAndMediaHelpers(t *testing.T) {
	t.Parallel()

	_, err := compileQueryAnyOfCandidates(queryParameter{
		wire:       wireDelimitedArray,
		validation: &Validation{SchemaPointer: "#/schema", AnyOfValidations: []*Validation{new(Validation)}},
	})
	require.Error(t, err)

	for _, mediaType := range []string{
		`application/json; charset`,
		`application/json; charset=`,
		`application/json; charset =utf-8`,
		`application/json; charset= utf-8`,
		`application/json; charset=utf-8; CHARSET=ascii`,
		`application/json; note="a\\\";b"; charset=utf-8`,
	} {
		_ = strictMediaTypeParameters(mediaType)
		_ = mediaTypeParameterSegments(mediaType)
	}
}
