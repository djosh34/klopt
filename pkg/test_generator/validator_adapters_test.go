//nolint:godoclint // Private adapter seams are intentionally concise.
package testgenerator

import (
	"errors"
	"fmt"

	"github.com/djosh34/klopt/pkg/test_generator/internal/suite"
	"pgregory.net/rapid"
)

type requestBodyValidator interface {
	Validate(body []byte) error
}

type validatorAdapter struct {
	name      string
	validator requestBodyValidator
	cleanup   func()
}

type bodyRejectionError struct {
	err error
}

func (rejection bodyRejectionError) Error() string {
	return rejection.err.Error()
}

func (rejection bodyRejectionError) Unwrap() error {
	return rejection.err
}

type validatorAdapterFactory func([]byte) (validatorAdapter, error)

func newExternalValidatorAdapters(spec []byte) ([]validatorAdapter, error) {
	return newValidatorAdaptersFromFactories(spec, []validatorAdapterFactory{
		newLibopenapiRequestBodyValidator,
		newKinopenapiRequestBodyValidator,
	})
}

func newValidatorAdaptersFromFactories(
	spec []byte,
	factories []validatorAdapterFactory,
) ([]validatorAdapter, error) {
	adapters := make([]validatorAdapter, 0, len(factories))
	for _, factory := range factories {
		adapter, err := factory(spec)
		if err != nil {
			cleanupValidatorAdapter(adapter)
			releaseValidatorAdapters(adapters)

			return nil, err
		}

		if adapter.name == "" || adapter.validator == nil {
			cleanupValidatorAdapter(adapter)
			releaseValidatorAdapters(adapters)

			return nil, errors.New("validator adapter is incomplete")
		}

		adapters = append(adapters, adapter)
	}

	return adapters, nil
}

func cleanupValidatorAdapter(adapter validatorAdapter) {
	if adapter.cleanup != nil {
		adapter.cleanup()
	}
}

func releaseValidatorAdapters(adapters []validatorAdapter) {
	for _, adapter := range adapters {
		cleanupValidatorAdapter(adapter)
	}
}

func drawPlannedBody(rt *rapid.T, plannedCase suite.CasePlan) ([]byte, error) {
	value := plannedCase.Generator.Draw(rt, "json value")

	body, err := value.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("encode generated JSON: %w", err)
	}

	return body, nil
}

func validatorVerdictMatches(expect suite.ExpectedResult, validationErr error) bool {
	switch expect {
	case suite.ExpectAccepted:
		return validationErr == nil
	case suite.ExpectRejected:
		return isBodyRejection(validationErr)
	default:
		return false
	}
}

func validatorChecksCase(adapter validatorAdapter, plannedCase suite.CasePlan) bool {
	return adapter.name == runtimeValidationName || plannedCase.Source.Keyword != "format"
}

func isBodyRejection(err error) bool {
	var rejection bodyRejectionError

	return errors.As(err, &rejection)
}
