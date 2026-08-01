//nolint:godoclint,lll // Internal white-box coverage matrices use compact malformed-state literals.
package validation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneratedQueryDefinitionFailureMatrix(t *testing.T) {
	t.Parallel()

	valid := new(Validation)

	malformed := &Validation{KindValidation: KindValidation{Type: "bad"}}
	for _, definition := range []QueryDecoderDefinition{
		{OperationID: "bad.operation", Parameters: []QueryParameterDefinition{{Name: "q", Validation: valid}}},
		{OperationID: "query", Parameters: []QueryParameterDefinition{{Name: "", Validation: valid}}},
		{OperationID: "query", Parameters: []QueryParameterDefinition{{Name: "q", Validation: malformed}}},
		{OperationID: "query", Parameters: []QueryParameterDefinition{{Name: "q", Validation: valid, Wire: 255}}},
		{OperationID: "query", Parameters: []QueryParameterDefinition{{Name: "q", Validation: valid, Properties: []QueryPropertyDefinition{{Name: ""}}}}},
		{OperationID: "query", Parameters: []QueryParameterDefinition{{Name: "q", Validation: valid, Properties: []QueryPropertyDefinition{{Name: "a"}, {Name: "a"}}}}},
		{OperationID: "query", Parameters: []QueryParameterDefinition{{Name: "q", Validation: valid, DefaultValue: json.RawMessage(`invalid`)}}},
	} {
		_, err := NewQueryDecoderFromGenerated(definition)
		require.Error(t, err)
	}
}

func TestGeneratedAnyOfDefinitionsRestore(t *testing.T) {
	t.Parallel()

	anyOf := &Validation{
		SchemaPointer: "#/schema",
		AnyOfValidations: []*Validation{{
			SchemaPointer:  "#/schema/anyOf/0",
			KindValidation: KindValidation{Type: "string"},
		}},
	}
	query, err := NewQueryDecoderFromGenerated(QueryDecoderDefinition{
		OperationID: "query",
		Parameters: []QueryParameterDefinition{{
			Name: "q", Validation: anyOf, Wire: uint8(wirePrimitive), ScalarType: "string",
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, query)

	path, err := NewPathDecoderFromGenerated(PathDecoderDefinition{
		OperationID: "path", PathTemplate: "/{p}",
		Parameters: []PathParameterDefinition{{
			Name: "p", Validation: anyOf, Wire: uint8(pathWireSimplePrimitive), ScalarType: "string",
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, path)

	objectValidation := &Validation{KindValidation: KindValidation{Type: "object"}}
	path, err = NewPathDecoderFromGenerated(PathDecoderDefinition{
		OperationID: "objectPath", PathTemplate: "/{p}",
		Parameters: []PathParameterDefinition{{
			Name: "p", Validation: objectValidation, Wire: uint8(pathWireSimpleObject),
			Properties: []PathPropertyDefinition{{Name: "value", ScalarType: "string"}},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, path)

	unsupported := &Validation{
		SchemaPointer: "#/schema",
		AnyOfValidations: []*Validation{{
			SchemaPointer:  "#/schema/anyOf/0",
			KindValidation: KindValidation{Type: "object"},
		}},
	}
	_, err = NewQueryDecoderFromGenerated(QueryDecoderDefinition{
		OperationID: "query",
		Parameters: []QueryParameterDefinition{{
			Name: "q", Validation: unsupported, Wire: uint8(wirePrimitive), ScalarType: "string",
		}},
	})
	require.Error(t, err)
	_, err = NewPathDecoderFromGenerated(PathDecoderDefinition{
		OperationID: "path", PathTemplate: "/{p}",
		Parameters: []PathParameterDefinition{{
			Name: "p", Validation: unsupported, Wire: uint8(pathWireSimplePrimitive), ScalarType: "string",
		}},
	})
	require.Error(t, err)
}

func TestQueryMetadataAndOwnershipFailureMatrix(t *testing.T) {
	t.Parallel()

	valid := new(Validation)

	for _, parameter := range []queryParameter{
		{wire: wirePrimitive},
		{wire: wireDelimitedArray, scalarType: "string", separator: ";"},
		{wire: wireFormObjectNamed, scalarType: "string", separator: ","},
		{wire: wireFormObjectExploded, scalarType: "string"},
		{wire: wireDelimitedObject, separator: ","},
		{wire: wireJSONContent, dynamicType: "string"},
		{wire: wireKind(255)},
		{wire: wireFormObjectNamed, separator: ",", dynamicType: "bad"},
		{wire: wireFormObjectNamed, separator: ",", properties: []queryProperty{{name: "p", scalarType: "bad"}}},
		{wire: wireFormObjectNamed, separator: ",", properties: []queryProperty{{name: "p", scalarType: "string", array: true}}},
	} {
		require.Error(t, validateQueryParameterMetadata(parameter))
	}

	_, err := newQueryDecoder("query", []queryParameter{
		{name: "a", wire: wireFormObjectExploded, dynamicType: "string", validation: valid},
		{name: "b", wire: wireFormObjectExploded, dynamicType: "string", validation: valid},
	})
	require.Error(t, err)
	_, err = newQueryDecoder("query", []queryParameter{
		{name: "a", wire: wirePrimitive, scalarType: "string", validation: valid},
		{name: "a", wire: wirePrimitive, scalarType: "string", validation: valid},
	})
	require.Error(t, err)
	_, err = newQueryDecoder("query", []queryParameter{
		{name: "deep", wire: wireDeepObject, properties: []queryProperty{{name: "x", scalarType: "string"}}, validation: valid},
		{name: "deep[x]", wire: wirePrimitive, scalarType: "string", validation: valid},
	})
	require.Error(t, err)
}

func TestGeneratedPathDefinitionFailureMatrix(t *testing.T) {
	t.Parallel()

	valid := &Validation{KindValidation: KindValidation{Type: "string"}}

	malformed := &Validation{KindValidation: KindValidation{Type: "bad"}}
	for _, definition := range []PathDecoderDefinition{
		{OperationID: "bad.operation", PathTemplate: "/{p}", Parameters: []PathParameterDefinition{{Name: "p", Validation: valid, ScalarType: "string"}}},
		{OperationID: "path", PathTemplate: "/{p}", Parameters: []PathParameterDefinition{{Name: "a/b", Validation: valid, ScalarType: "string"}}},
		{OperationID: "path", PathTemplate: "/{p}", Parameters: []PathParameterDefinition{{Name: "p", Validation: malformed, ScalarType: "string"}}},
		{OperationID: "path", PathTemplate: "/{p}", Parameters: []PathParameterDefinition{{Name: "p", Validation: valid, Wire: 255, ScalarType: "string"}}},
		{OperationID: "path", PathTemplate: "/{p}", Parameters: []PathParameterDefinition{{Name: "p", Validation: valid, Wire: uint8(pathWireSimpleObject), Properties: []PathPropertyDefinition{{Name: "a", ScalarType: "string"}, {Name: "a", ScalarType: "string"}}}}},
	} {
		_, err := NewPathDecoderFromGenerated(definition)
		require.Error(t, err)
	}
}

func TestPathMetadataAndConstructorFailureMatrix(t *testing.T) {
	t.Parallel()

	valid := &Validation{KindValidation: KindValidation{Type: "string"}}
	for _, parameter := range []pathParameter{
		{},
		{name: "p", validation: valid, wire: 255},
		{name: "p", validation: valid, wire: pathWireJSONContent, explode: true},
		{name: "p", validation: valid, wire: pathWireSimplePrimitive},
		{name: "p", validation: valid, wire: pathWireSimpleObject, scalarType: "string"},
		{name: "p", validation: valid, wire: pathWireSimpleObject, dynamicType: "bad"},
		{name: "p", validation: valid, wire: pathWireSimpleObject, properties: []pathProperty{{name: "", scalarType: "string"}}, propertyByName: map[string]int{"": 0}},
		{name: "p", validation: valid, wire: pathWireSimpleObject, properties: []pathProperty{{name: "a", scalarType: "string"}, {name: "a", scalarType: "string"}}, propertyByName: map[string]int{"a": 0}},
		{name: "p", validation: valid, wire: pathWireSimpleObject, properties: []pathProperty{{name: "a", scalarType: "string"}}, propertyByName: map[string]int{"a": 1}},
		{name: "p", validation: valid, wire: pathWireSimpleObject, properties: []pathProperty{{name: "a", scalarType: "string"}}, propertyByName: map[string]int{"a": 0, "b": 1}},
		{name: "p", validation: &Validation{KindValidation: KindValidation{Nullable: true}}, wire: pathWireSimplePrimitive, scalarType: "string"},
	} {
		require.Error(t, validatePathParameterMetadata(parameter))
	}

	base := pathParameter{name: "p", validation: valid, wire: pathWireSimplePrimitive, scalarType: "string"}
	for _, test := range []struct {
		operation string
		template  string
		params    []pathParameter
	}{
		{operation: "bad.operation", template: "/{p}", params: []pathParameter{base}},
		{operation: "path", template: "/{p}", params: []pathParameter{base, base}},
		{operation: "path", template: "invalid", params: []pathParameter{base}},
		{operation: "path", template: "/{missing}", params: []pathParameter{base}},
		{operation: "path", template: "/fixed", params: []pathParameter{base}},
	} {
		_, err := newPathDecoder(test.operation, test.template, test.params)
		require.Error(t, err)
	}
}
