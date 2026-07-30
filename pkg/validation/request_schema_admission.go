//nolint:godoclint // Private admission traversal stays behind AdmitRequestSchema.
package validation

import (
	"fmt"

	"github.com/djosh34/klopt/pkg/internal/oas"
	"github.com/djosh34/klopt/pkg/patternvalidator"
)

const directSchemaChildFamilies = 2

// RequestSchemaUse identifies one consumer of an admitted request schema.
type RequestSchemaUse uint8

const (
	// UseRuntimeValidation admits features implemented by the runtime validator.
	UseRuntimeValidation RequestSchemaUse = iota
	// UseGeneratedValidation admits features implemented by generated validators.
	UseGeneratedValidation
	// UseRequestGeneration admits only features with exact generation semantics.
	UseRequestGeneration
)

// AdmittedRequestSchema is a completely checked request-schema use.
type AdmittedRequestSchema struct {
	Source     oas.Source
	Root       oas.LocatedSchema
	Validation *Validation
}

// AdmissionError reports a source-located capability rejection.
type AdmissionError struct {
	Pointer string
	Feature string
	Use     RequestSchemaUse
	Reason  string
}

// Error formats the rejected feature and exact schema pointer.
func (admissionError *AdmissionError) Error() string {
	return fmt.Sprintf(
		"admit request schema at %s: feature %s is %s",
		admissionError.Pointer,
		admissionError.Feature,
		admissionError.Reason,
	)
}

// AdmitRequestSchema validates one complete schema use against the shared capability matrix.
func AdmitRequestSchema(
	source oas.Source,
	schema oas.LocatedSchema,
	use RequestSchemaUse,
	patternOptions ...patternvalidator.Option,
) (*AdmittedRequestSchema, error) {
	if use > UseRequestGeneration {
		return nil, fmt.Errorf("unknown request schema use %d", use)
	}

	compiler := schemaCompiler{
		source: source, bySchema: make(map[string]*Validation), active: make(map[string]struct{}),
		patternOptions: patternOptions,
	}

	return admitRequestSchemaWithCompiler(source, schema, use, &compiler)
}

func admitRequestSchemaWithCompiler(
	source oas.Source,
	schema oas.LocatedSchema,
	use RequestSchemaUse,
	compiler *schemaCompiler,
) (*AdmittedRequestSchema, error) {
	if compiler == nil {
		return nil, fmt.Errorf("schema compiler must not be nil")
	}

	compiled, err := compiler.compile(schema)
	if err != nil {
		return nil, err
	}

	if use == UseRequestGeneration {
		if err := admitExactGeneration(compiled, make(map[*Validation]struct{})); err != nil {
			return nil, err
		}
	}

	return &AdmittedRequestSchema{Source: source, Root: schema, Validation: compiled}, nil
}

//nolint:cyclop // The exact-generation capability matrix checks each final schema family explicitly.
func admitExactGeneration(validation *Validation, visited map[*Validation]struct{}) error {
	if _, ok := visited[validation]; ok {
		return nil
	}

	visited[validation] = struct{}{}

	if validation.ArrayValidation.UniqueItems {
		return &AdmissionError{
			Pointer: validation.SchemaPointer + "/uniqueItems",
			Feature: "uniqueItems: true",
			Use:     UseRequestGeneration,
			Reason:  "unsupported for exact request generation",
		}
	}

	for _, item := range []struct {
		keyword string
		bound   *CountBound
	}{
		{keyword: "minLength", bound: validation.StringValidation.MinLength},
		{keyword: "maxLength", bound: validation.StringValidation.MaxLength},
	} {
		keyword := item.keyword

		bound := item.bound
		if bound == nil {
			continue
		}

		exact := bound.ExactValue
		if exact.Rational == nil || !exact.Rational.IsInt() || !exact.Rational.Num().IsInt64() {
			return &AdmissionError{
				Pointer: validation.SchemaPointer + "/" + keyword,
				Feature: keyword + ": " + bound.Value,
				Use:     UseRequestGeneration,
				Reason:  "not representable by the exact string-language compiler",
			}
		}

		parsed := exact.Rational.Num().Int64()
		if parsed < 0 || int64(int(parsed)) != parsed {
			return &AdmissionError{
				Pointer: validation.SchemaPointer + "/" + keyword,
				Feature: keyword + ": " + bound.Value,
				Use:     UseRequestGeneration,
				Reason:  "not representable by the exact string-language compiler",
			}
		}
	}

	children := make(
		[]*Validation,
		0,
		len(validation.AllOfValidations)+len(validation.AnyOfValidations)+directSchemaChildFamilies,
	)
	children = append(children, validation.AllOfValidations...)
	children = append(children, validation.AnyOfValidations...)

	if validation.ArrayValidation.Items != nil {
		children = append(children, validation.ArrayValidation.Items)
	}

	for _, property := range validation.ObjectValidation.Properties {
		children = append(children, property.Validation)
	}

	if validation.ObjectValidation.AdditionalPropertiesValidation != nil {
		children = append(children, validation.ObjectValidation.AdditionalPropertiesValidation)
	}

	for _, child := range children {
		if err := admitExactGeneration(child, visited); err != nil {
			return err
		}
	}

	return nil
}
