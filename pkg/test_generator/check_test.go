//nolint:godoclint // Package-private tests document the public Check contract.
package testgenerator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckReportsRuntimeAndGeneratedDisagreementsIndependently(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"string"}`)

	valid := Sample{OperationID: "request", Body: []byte(`""`), ExpectValid: true}
	require.NoError(t, generator.Check(valid, func(string, []byte) (bool, error) {
		return true, nil
	}))

	runtimeMismatch := valid
	runtimeMismatch.ExpectValid = false
	err := generator.Check(runtimeMismatch, func(string, []byte) (bool, error) {
		return true, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "runtime validator verdict")
	require.Contains(t, err.Error(), "generated validator verdict")
}

func TestCheckWrapsGeneratedValidatorExecutionErrors(t *testing.T) {
	t.Parallel()

	generator := mustCompileGenerator(t, `{"type":"string"}`)
	want := errors.New("validator failed")

	err := generator.Check(
		Sample{OperationID: "request", Body: []byte(`""`), ExpectValid: true},
		func(string, []byte) (bool, error) { return false, want },
	)
	require.Error(t, err)
	require.ErrorIs(t, err, want)
	require.Contains(t, err.Error(), "generated validator")
}
