// Package validation compiles OpenAPI 3.0.x request-body validations and query decoders.
//
// Parse is the OpenAPI constructor. Callers may also construct a compiled graph
// directly by populating every exported textual and exact field consistently.
// Invalid field combinations, mutation after construction, and mutation
// concurrent with Validate have undefined behavior.
package validation

import (
	"encoding/json"

	"github.com/djosh34/klopt/pkg/jsonvalue"
	"github.com/djosh34/klopt/pkg/patternvalidator"
)

// Validation is one compiled OpenAPI Schema Object.
type Validation struct {
	SchemaPointer string
	BodyRequired  bool

	KindValidation   KindValidation
	EnumValidation   EnumValidation
	NumberValidation NumberValidation
	StringValidation StringValidation
	ArrayValidation  ArrayValidation
	ObjectValidation ObjectValidation

	AllOfValidations []*Validation
}

// KindValidation constrains the JSON kind. An empty Type accepts every kind.
type KindValidation struct {
	Type     string
	Nullable bool
}

// EnumValidation constrains a value to exact semantic JSON members.
type EnumValidation struct {
	Values []json.RawMessage

	// ExactValues is the compiled semantic form of Values used by Validate.
	ExactValues []jsonvalue.Value
}

// NumberBound is one exact inclusive or exclusive decimal bound.
type NumberBound struct {
	Value     string
	Exclusive bool

	// ExactValue is the compiled numeric form of Value used by Validate.
	ExactValue jsonvalue.Number
}

// NumberValidation holds exact numeric constraints.
type NumberValidation struct {
	Minimum    *NumberBound
	Maximum    *NumberBound
	MultipleOf string

	// ExactMultipleOf is the compiled numeric form of MultipleOf used by Validate.
	ExactMultipleOf *jsonvalue.Number
}

// CountBound is one exact non-negative integer bound for a collection or string length.
type CountBound struct {
	Value string

	// ExactValue is the compiled numeric form of Value used by Validate.
	ExactValue jsonvalue.Number
}

// StringValidation holds string-specific constraints.
type StringValidation struct {
	MinLength *CountBound
	MaxLength *CountBound
	Pattern   string
	Format    string

	// CompiledPattern is the compiled form of Pattern used by Validate.
	CompiledPattern *patternvalidator.PatternValidation
}

// ArrayValidation holds array-specific constraints.
type ArrayValidation struct {
	MinItems    *CountBound
	MaxItems    *CountBound
	Items       *Validation
	UniqueItems bool
}

// PropertyValidation pairs one lexical object property name with its schema.
type PropertyValidation struct {
	Name       string
	Validation *Validation
}

// ObjectValidation holds object-specific constraints.
type ObjectValidation struct {
	MinProperties                  *CountBound
	MaxProperties                  *CountBound
	Required                       []string
	Properties                     []PropertyValidation
	AdditionalPropertiesAllowed    bool
	AdditionalPropertiesValidation *Validation
}
