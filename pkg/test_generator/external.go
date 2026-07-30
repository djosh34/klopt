//nolint:cyclop,godoclint // Adapter setup validates every third-party construction result.
package testgenerator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pb33f/libopenapi"                              //nolint:depguard // Independent request-body parity validator required by the generation plan.
	libvalidator "github.com/pb33f/libopenapi-validator"       //nolint:depguard // Independent request-body parity validator required by the generation plan.
	highbase "github.com/pb33f/libopenapi/datamodel/high/base" //nolint:depguard // Detect one narrowly characterized renderer limitation.
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"     //nolint:depguard // Path items feed the body-only validator seam.
)

var errExternalUnsupported = errors.New("external validator does not support this schema")

type verdictValidator interface {
	validate(operationID string, body []byte) (bool, string, error)
}

type kinValidator struct {
	byOperation map[string]*openapi3.Schema
	unsupported string
}

func newKinValidator(document []byte) (*kinValidator, error) {
	loaded, err := openapi3.NewLoader().LoadFromData(document)
	if err != nil {
		if kinFloatRangeError(err) {
			return &kinValidator{
				unsupported: "kin-openapi stores numeric Schema Object bounds as float64",
			}, nil
		}

		return nil, fmt.Errorf("load kin-openapi document: %w", err)
	}

	result := &kinValidator{byOperation: make(map[string]*openapi3.Schema)}

	for _, path := range loaded.Paths.Keys() {
		pathItem := loaded.Paths.Value(path)
		for _, operation := range pathItem.Operations() {
			if operation.RequestBody == nil || operation.RequestBody.Value == nil {
				continue
			}

			mediaType := operation.RequestBody.Value.GetMediaType("application/json")
			if mediaType == nil {
				continue
			}

			if mediaType.Schema == nil {
				result.byOperation[operation.OperationID] = nil
			} else {
				result.byOperation[operation.OperationID] = mediaType.Schema.Value
			}
		}
	}

	return result, nil
}

func (validator *kinValidator) validate(operationID string, body []byte) (bool, string, error) {
	if validator.unsupported != "" {
		return false, validator.unsupported, errExternalUnsupported
	}

	schema, ok := validator.byOperation[operationID]
	if !ok {
		return false, "", fmt.Errorf("kin-openapi has no JSON request schema for %q", operationID)
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return false, "", fmt.Errorf("decode body for kin-openapi: %w", err)
	}

	if schema == nil {
		return true, "", nil
	}

	err := schema.VisitJSON(
		value,
		openapi3.VisitAsRequest(),
		openapi3.EnableFormatValidation(),
	)
	if err != nil {
		return false, err.Error(), nil //nolint:nilerr // Schema rejection is a verdict, not an adapter failure.
	}

	return true, "", nil
}

func kinFloatRangeError(err error) bool {
	message := err.Error()

	return strings.Contains(message, "cannot unmarshal number") &&
		strings.Contains(message, "type float64")
}

type libOperation struct {
	method      string
	path        string
	pathItem    *highv3.PathItem
	unsupported string
}

type libValidator struct {
	document   libopenapi.Document
	validator  libvalidator.Validator
	operations map[string]libOperation
}

func newLibValidator(document []byte) (*libValidator, error) {
	loaded, err := libopenapi.NewDocument(document)
	if err != nil {
		return nil, fmt.Errorf("load libopenapi document: %w", err)
	}

	model, err := loaded.BuildV3Model()
	if err != nil {
		loaded.Release()

		return nil, fmt.Errorf("build libopenapi model: %w", err)
	}

	compiled, buildErrs := libvalidator.NewValidator(loaded)
	if err := joinedErrors("build libopenapi validator", buildErrs); err != nil {
		if compiled != nil {
			compiled.Release()
		}

		loaded.Release()

		return nil, err
	}

	if compiled == nil {
		loaded.Release()

		return nil, errors.New("libopenapi returned a nil validator")
	}

	result := &libValidator{
		document: loaded, validator: compiled, operations: make(map[string]libOperation),
	}

	if model.Model.Paths != nil && model.Model.Paths.PathItems != nil {
		for pathPair := model.Model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
			path := pathPair.Key()

			pathItem := pathPair.Value()
			for operationPair := pathItem.GetOperations().First(); operationPair != nil; operationPair = operationPair.Next() {
				operation := operationPair.Value()
				if operation == nil || operation.RequestBody == nil {
					continue
				}

				operationRecord := libOperation{
					method: operationPair.Key(), path: path, pathItem: pathItem,
				}

				mediaType := operation.RequestBody.Content.GetOrZero("application/json")
				if mediaType != nil && mediaType.Schema != nil &&
					libSchemaHasEmptyPropertyName(mediaType.Schema.Schema(), make(map[*highbase.Schema]struct{})) {
					operationRecord.unsupported = "libopenapi cannot render a Schema Object property named with the empty string"
				}

				result.operations[operation.OperationId] = operationRecord
			}
		}
	}

	return result, nil
}

func (validator *libValidator) validate(operationID string, body []byte) (bool, string, error) {
	operation, ok := validator.operations[operationID]
	if !ok {
		return false, "", fmt.Errorf("libopenapi has no JSON request schema for %q", operationID)
	}

	if operation.unsupported != "" {
		return false, operation.unsupported, errExternalUnsupported
	}

	request, err := http.NewRequest(
		strings.ToUpper(operation.method),
		operation.path,
		bytes.NewReader(body),
	)
	if err != nil {
		return false, "", fmt.Errorf("construct libopenapi request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	valid, validationErrs := validator.validator.GetRequestBodyValidator().
		ValidateRequestBodyWithPathItem(request, operation.pathItem, operation.path)
	if valid && len(validationErrs) != 0 {
		return false, "", errors.New("libopenapi returned valid with validation errors")
	}

	if valid {
		return true, "", nil
	}

	if len(validationErrs) == 0 {
		return false, "", errors.New("libopenapi rejected without a validation error")
	}

	reasons := make([]string, len(validationErrs))
	for index, validationErr := range validationErrs {
		if validationErr == nil {
			return false, "", fmt.Errorf("libopenapi returned nil validation error %d", index)
		}

		reasons[index] = validationErr.Error()
	}

	return false, strings.Join(reasons, "; "), nil
}

func libSchemaHasEmptyPropertyName(
	schema *highbase.Schema,
	seen map[*highbase.Schema]struct{},
) bool {
	if schema == nil {
		return false
	}

	if _, ok := seen[schema]; ok {
		return false
	}

	seen[schema] = struct{}{}

	if schema.Properties != nil {
		for property := schema.Properties.First(); property != nil; property = property.Next() {
			if property.Key() == "" || libSchemaProxyHasEmptyPropertyName(property.Value(), seen) {
				return true
			}
		}
	}

	for _, alternatives := range [][]*highbase.SchemaProxy{schema.AllOf, schema.AnyOf} {
		for _, alternative := range alternatives {
			if libSchemaProxyHasEmptyPropertyName(alternative, seen) {
				return true
			}
		}
	}

	if schema.Items != nil && schema.Items.IsA() &&
		libSchemaProxyHasEmptyPropertyName(schema.Items.A, seen) {
		return true
	}

	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() &&
		libSchemaProxyHasEmptyPropertyName(schema.AdditionalProperties.A, seen) {
		return true
	}

	return false
}

func libSchemaProxyHasEmptyPropertyName(
	proxy *highbase.SchemaProxy,
	seen map[*highbase.Schema]struct{},
) bool {
	return proxy != nil && libSchemaHasEmptyPropertyName(proxy.Schema(), seen)
}

func (validator *libValidator) close() {
	if validator == nil {
		return
	}

	if validator.validator != nil {
		validator.validator.Release()
	}

	if validator.document != nil {
		validator.document.Release()
	}
}

func joinedErrors(operation string, source []error) error {
	if len(source) == 0 {
		return nil
	}

	errs := make([]error, 0, len(source))
	for index, err := range source {
		if err == nil {
			errs = append(errs, fmt.Errorf("nil error %d", index))
		} else {
			errs = append(errs, err)
		}
	}

	return fmt.Errorf("%s: %w", operation, errors.Join(errs...))
}

func externalMismatch(
	name string,
	validator verdictValidator,
	sample Sample,
) error {
	accepted, reason, err := validator.validate(sample.OperationID, sample.Body)
	if errors.Is(err, errExternalUnsupported) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("%s adapter: %w", name, err)
	}

	if accepted == sample.ExpectValid {
		return nil
	}

	return fmt.Errorf(
		"%s validator for %q returned valid=%t, want %t for %s: %s",
		name,
		sample.OperationID,
		accepted,
		sample.ExpectValid,
		sample.Body,
		reason,
	)
}
