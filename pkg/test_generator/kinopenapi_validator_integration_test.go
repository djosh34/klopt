package testgenerator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// kinopenapiValidatorName identifies the pinned kin-openapi validator adapter.
const kinopenapiValidatorName = "kin-openapi v0.140.0"

// kinopenapiRequestBodyValidator adapts one loaded kin-openapi router to the private test seam.
type kinopenapiRequestBodyValidator struct {
	router routers.Router
}

// newKinopenapiRequestBodyValidator loads and validates one document once for one corpus schema.
func newKinopenapiRequestBodyValidator(spec []byte) (validatorAdapter, error) {
	loader := openapi3.NewLoader()

	document, err := loader.LoadFromData(spec)
	if err != nil {
		return validatorAdapter{}, fmt.Errorf("load OpenAPI document: %w", err)
	}

	router, err := legacy.NewRouter(document)
	if err != nil {
		return validatorAdapter{}, fmt.Errorf("build legacy router: %w", err)
	}

	return validatorAdapter{
		name:      kinopenapiValidatorName,
		validator: kinopenapiRequestBodyValidator{router: router},
	}, nil
}

// Validate builds a fresh request because kin-openapi consumes its request body.
func (adapter kinopenapiRequestBodyValidator) Validate(body []byte) error {
	request, err := http.NewRequest(http.MethodPost, "/things", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("construct integration request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	route, pathParams, err := adapter.router.FindRoute(request)
	if err != nil {
		return fmt.Errorf("find integration route: %w", err)
	}

	input := &openapi3filter.RequestValidationInput{
		Request: request, PathParams: pathParams, Route: route,
	}
	if err := openapi3filter.ValidateRequest(context.Background(), input); err != nil {
		var requestErr *openapi3filter.RequestError
		if errors.As(err, &requestErr) && requestErr.RequestBody != nil {
			return bodyRejectionError{err: err}
		}

		return err
	}

	return nil
}

// TestKinopenapiValidatorCharacterizations pins known v0.140.0 behavior outside the consensus corpus.
func TestKinopenapiValidatorCharacterizations(t *testing.T) {
	t.Parallel()

	runValidatorCharacterizations(t, newKinopenapiRequestBodyValidator, kinopenapiValidatorName)
}
