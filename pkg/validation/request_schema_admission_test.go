//nolint:godoclint // Behavior test names are the admission specifications.
package validation

import (
	"testing"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/stretchr/testify/require"
)

func TestAdmitRequestSchemaKeepsRuntimeUniqueItemsAndRejectsGeneration(t *testing.T) {
	t.Parallel()

	sources, err := oas.Parse(openAPISpec(`{
		"type":"object",
		"properties":{"values":{"type":"array","items":{},"uniqueItems":true}}
	}`, "", false))
	require.NoError(t, err)

	source := sources["checkThing"]
	runtime, err := AdmitRequestSchema(source, source.RequestSchema, UseRuntimeValidation)
	require.NoError(t, err)
	require.NotNil(t, runtime)

	generated, err := AdmitRequestSchema(source, source.RequestSchema, UseGeneratedValidation)
	require.NoError(t, err)
	require.NotNil(t, generated)

	request, err := AdmitRequestSchema(source, source.RequestSchema, UseRequestGeneration)
	require.Nil(t, request)
	require.ErrorContains(t, err, source.RequestSchema.Pointer+"/properties/values/uniqueItems")
	require.ErrorContains(t, err, "unsupported for exact request generation")
}

func TestAdmitRequestSchemaRejectsUnsupportedSyntaxBeforeReturningAResult(t *testing.T) {
	t.Parallel()

	sources, err := oas.Parse(openAPISpec(`{
		"properties":{"value":{"oneOf":[{"type":"string"}]}}
	}`, "", false))
	require.NoError(t, err)

	source := sources["checkThing"]
	admitted, err := AdmitRequestSchema(source, source.RequestSchema, UseRequestGeneration)
	require.Nil(t, admitted)
	require.ErrorContains(t, err, source.RequestSchema.Pointer+"/properties/value/oneOf")
}

func TestAdmitRequestSchemaRejectsUnrepresentableGenerationLengthAtExactPointer(t *testing.T) {
	t.Parallel()

	sources, err := oas.Parse(openAPISpec(
		`{"type":"string","minLength":9223372036854775808}`,
		"",
		false,
	))
	require.NoError(t, err)

	source := sources["checkThing"]
	admitted, err := AdmitRequestSchema(source, source.RequestSchema, UseRequestGeneration)
	require.Nil(t, admitted)
	require.ErrorContains(t, err, source.RequestSchema.Pointer+"/minLength")
	require.ErrorContains(t, err, "not representable by the exact string-language compiler")
}
