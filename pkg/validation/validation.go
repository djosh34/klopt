// Package validation parses one OpenAPI 3.0.3 JSON request schema and validates
// raw JSON request bodies against an immutable compiled graph.
//
// Parse is the only supported constructor. Direct construction, invalid field
// combinations, mutation after parsing, and mutation concurrent with Validate
// have undefined behavior.
package validation

import (
	"encoding/json"
	"regexp"

	"decode_and_validate_generator/pkg/internal/jsonvalue"
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

	exactValues []jsonvalue.Value
}

// NumberBound is one exact inclusive or exclusive decimal bound.
type NumberBound struct {
	Value     string
	Exclusive bool

	exactValue jsonvalue.Number
}

// NumberValidation holds exact numeric constraints.
type NumberValidation struct {
	Integer    bool
	Minimum    *NumberBound
	Maximum    *NumberBound
	MultipleOf string

	exactMultipleOf *jsonvalue.Number
}

// StringValidation holds string-specific constraints.
type StringValidation struct {
	MinLength *int
	MaxLength *int
	Pattern   string
	Format    string

	compiledPattern *regexp.Regexp
}

// ArrayValidation holds array-specific constraints.
type ArrayValidation struct {
	MinItems    *int
	MaxItems    *int
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
	MinProperties                  *int
	MaxProperties                  *int
	Required                       []string
	Properties                     []PropertyValidation
	AdditionalPropertiesAllowed    bool
	AdditionalPropertiesValidation *Validation
}
