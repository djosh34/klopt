package testgenerator

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	//nolint:depguard // This test-only dependency loads one independent integration SUT.
	"github.com/pb33f/libopenapi"
	//nolint:depguard // This test-only dependency is one independent integration SUT.
	validator "github.com/pb33f/libopenapi-validator"
	//nolint:depguard // Structured validator errors distinguish schema verdicts from infrastructure failures.
	validatorErrors "github.com/pb33f/libopenapi-validator/errors"
	//nolint:depguard // Validator error constants identify request-body schema verdicts.
	validatorHelpers "github.com/pb33f/libopenapi-validator/helpers"
)

// libopenapiValidatorName identifies the pinned libopenapi validator adapter.
const libopenapiValidatorName = "libopenapi-validator v0.13.13"

// libopenapiRequestBodyValidator adapts one loaded libopenapi validator to the private test seam.
type libopenapiRequestBodyValidator struct {
	validator validator.Validator
}

// newLibopenapiRequestBodyValidator loads the document and validator once for one corpus schema.
func newLibopenapiRequestBodyValidator(spec []byte) (validatorAdapter, error) {
	document, err := libopenapi.NewDocument(spec)
	if err != nil {
		return validatorAdapter{}, fmt.Errorf("load OpenAPI document: %w", err)
	}

	requestValidator, buildErrs := validator.NewValidator(document)
	if err := validatorSetupError(buildErrs); err != nil {
		if requestValidator != nil {
			requestValidator.Release()
		}

		document.Release()

		return validatorAdapter{}, err
	}

	if requestValidator == nil {
		document.Release()

		return validatorAdapter{}, errors.New("libopenapi-validator returned a nil validator")
	}

	cleanup := func() {
		requestValidator.Release()
		document.Release()
	}

	validDocument, documentErrs := requestValidator.ValidateDocument()
	if err := libopenapiDocumentValidationError(validDocument, documentErrs); err != nil {
		cleanup()

		return validatorAdapter{}, err
	}

	adapter := validatorAdapter{
		name:      libopenapiValidatorName,
		validator: libopenapiRequestBodyValidator{validator: requestValidator},
		cleanup:   cleanup,
	}

	// NewValidator's cache warmer discards schema compilation errors. A fixed probe forces compilation here;
	// either acceptance or an actual schema rejection proves setup succeeded.
	probeErr := adapter.validator.Validate([]byte(`null`))
	if probeErr != nil && !isBodyRejection(probeErr) {
		cleanup()

		return validatorAdapter{}, fmt.Errorf("compile request body schema: %w", probeErr)
	}

	return adapter, nil
}

// Validate builds a fresh request because request validation consumes its body.
func (adapter libopenapiRequestBodyValidator) Validate(body []byte) error {
	request, err := http.NewRequest(http.MethodPost, "/things", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("construct integration request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	valid, validationErrs := adapter.validator.ValidateHttpRequestSync(request)
	if valid && len(validationErrs) != 0 {
		return errors.New("libopenapi-validator returned valid with validation errors")
	}

	if valid {
		return nil
	}

	if len(validationErrs) == 0 {
		return errors.New("libopenapi-validator rejected request without an error")
	}

	messages := make([]string, len(validationErrs))
	for index, validationErr := range validationErrs {
		if validationErr == nil {
			return errors.New("libopenapi-validator returned a nil validation error")
		}

		messages[index] = validationErr.Error()
	}

	validationErr := errors.New(strings.Join(messages, "; "))
	if libopenapiBodyRejected(validationErrs) {
		return bodyRejectionError{err: validationErr}
	}

	return validationErr
}

// libopenapiBodyRejected distinguishes instance/schema failures from compilation, path, and adapter errors.
func libopenapiBodyRejected(validationErrs []*validatorErrors.ValidationError) bool {
	for _, validationErr := range validationErrs {
		if validationErr == nil ||
			validationErr.ValidationType != validatorHelpers.RequestBodyValidation ||
			validationErr.ValidationSubType != validatorHelpers.Schema {
			return false
		}

		if len(validationErr.SchemaValidationErrors) == 0 &&
			!strings.Contains(validationErr.Reason, "request body cannot be decoded") {
			return false
		}
	}

	return len(validationErrs) != 0
}

// libopenapiDocumentValidationError requires strict document validation before body checking.
func libopenapiDocumentValidationError(valid bool, validationErrs []*validatorErrors.ValidationError) error {
	if valid && len(validationErrs) == 0 {
		return nil
	}

	if valid {
		return errors.New("libopenapi-validator returned a valid document with validation errors")
	}

	if len(validationErrs) == 0 {
		return errors.New("libopenapi-validator rejected document without an error")
	}

	errs := make([]error, 0, len(validationErrs))
	for index, validationErr := range validationErrs {
		if validationErr == nil {
			errs = append(errs, fmt.Errorf("nil document validation error at index %d", index))

			continue
		}

		errs = append(errs, validationErr)
	}

	return fmt.Errorf("validate OpenAPI document: %w", errors.Join(errs...))
}

// validatorSetupError preserves every constructor error instead of treating setup failure as a body verdict.
func validatorSetupError(buildErrs []error) error {
	if len(buildErrs) == 0 {
		return nil
	}

	errs := make([]error, 0, len(buildErrs))
	for index, buildErr := range buildErrs {
		if buildErr == nil {
			errs = append(errs, fmt.Errorf("libopenapi-validator returned nil setup error at index %d", index))

			continue
		}

		errs = append(errs, buildErr)
	}

	return fmt.Errorf("build libopenapi-validator: %w", errors.Join(errs...))
}

// TestLibopenapiValidatorCharacterizations pins known v0.13.13 behavior outside the consensus corpus.
func TestLibopenapiValidatorCharacterizations(t *testing.T) {
	t.Parallel()

	runValidatorCharacterizations(t, newLibopenapiRequestBodyValidator, libopenapiValidatorName)
}
