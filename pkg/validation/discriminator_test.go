//nolint:godoclint,lll // Focused JSON and YAML discriminator fixtures keep authored pointers visible.
package validation_test

import (
	"testing"

	"github.com/djosh34/klopt/pkg/validation"

	"github.com/stretchr/testify/require"
)

func TestParseRejectsAuthoredDiscriminatorAtEveryCompositionShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		yaml    string
		pointer string
	}{
		{
			name:    "direct anyOf",
			json:    `{"anyOf":[{"type":"object"}],"discriminator":{"propertyName":"kind"}}`,
			yaml:    "anyOf: [{type: object}]\ndiscriminator: {propertyName: kind}",
			pointer: "/discriminator",
		},
		{
			name:    "direct allOf",
			json:    `{"allOf":[{"type":"object"}],"discriminator":{"propertyName":"kind"}}`,
			yaml:    "allOf: [{type: object}]\ndiscriminator: {propertyName: kind}",
			pointer: "/discriminator",
		},
		{
			name:    "inbound allOf child",
			json:    `{"allOf":[{"type":"object","discriminator":{"propertyName":"kind"}}]}`,
			yaml:    "allOf: [{type: object, discriminator: {propertyName: kind}}]",
			pointer: "/allOf/0/discriminator",
		},
		{
			name:    "referenced component parent",
			json:    `{"allOf":[{"$ref":"#/components/schemas/Parent"}]}`,
			yaml:    "allOf: [{$ref: '#/components/schemas/Parent'}]",
			pointer: "#/components/schemas/Parent/discriminator",
		},
		{
			name:    "standalone",
			json:    `{"discriminator":{"propertyName":"kind"}}`,
			yaml:    "discriminator: {propertyName: kind}",
			pointer: "/discriminator",
		},
		{
			name:    "populated mapping",
			json:    `{"discriminator":{"propertyName":"kind","mapping":{"cat":"#/components/schemas/Cat"}}}`,
			yaml:    "discriminator:\n  propertyName: kind\n  mapping:\n    cat: '#/components/schemas/Cat'",
			pointer: "/discriminator",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for encoding, document := range map[string][]byte{
				"JSON": []byte(documentWithSchema(test.json, test.name == "referenced component parent")),
				"YAML": []byte(documentWithYAMLSchema(test.yaml, test.name == "referenced component parent")),
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					parsed, err := validation.Parse(document)
					require.Nil(t, parsed)
					require.Error(t, err)
					require.ErrorContains(t, err, "compile schema at "+schemaPointer(test.pointer))
					require.ErrorContains(t, err, "authored discriminator is outside the Klopt profile")
				})
			}
		})
	}
}

func TestParseRejectsAuthoredDiscriminatorOutsideSelectedRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		yaml    string
		pointer string
	}{
		{
			name:    "unused component schema",
			json:    `{"openapi":"3.0.3","paths":{},"components":{"schemas":{"Unused":{"type":"object","discriminator":{"propertyName":"kind"}}}}}`,
			yaml:    "openapi: 3.0.3\npaths: {}\ncomponents:\n  schemas:\n    Unused:\n      type: object\n      discriminator:\n        propertyName: kind\n",
			pointer: "#/components/schemas/Unused/discriminator",
		},
		{
			name:    "response schema",
			json:    `{"openapi":"3.0.3","paths":{"/things":{"get":{"operationId":"things","responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","discriminator":{"propertyName":"kind"}}}}}}}}}}`,
			yaml:    "openapi: 3.0.3\npaths:\n  /things:\n    get:\n      operationId: things\n      responses:\n        \"200\":\n          description: ok\n          content:\n            application/json:\n              schema:\n                type: object\n                discriminator:\n                  propertyName: kind\n",
			pointer: "#/paths/~1things/get/responses/200/content/application~1json/schema/discriminator",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for encoding, document := range map[string]string{
				"JSON": test.json,
				"YAML": test.yaml,
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					parsed, err := validation.Parse([]byte(document))
					require.Nil(t, parsed)
					require.ErrorContains(t, err, "compile schema at "+test.pointer)
					require.ErrorContains(t, err, "authored discriminator is outside the Klopt profile")
				})
			}
		})
	}
}

func TestParseKeepsOneOfRejectionAtItsOwnPointer(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		`{"oneOf":[{}]}`,
		`{"oneOf":[{}],"discriminator":{"propertyName":"kind"}}`,
	} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()

			parsed, err := validation.Parse([]byte(documentWithSchema(schema, false)))
			require.Nil(t, parsed)
			require.ErrorContains(t, err, "compile schema at "+schemaPointer("/oneOf"))
			require.ErrorContains(t, err, "unsupported keyword")
			require.NotContains(t, err.Error(), "/discriminator")
		})
	}
}

func documentWithSchema(schema string, referencedParent bool) string {
	components := ""
	if referencedParent {
		components = `,"components":{"schemas":{"Parent":{"type":"object","discriminator":{"propertyName":"kind"}}}}`
	}

	return `{"openapi":"3.0.3","paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":` + schema + `}}}}}}` + components + `}`
}

func documentWithYAMLSchema(schema string, referencedParent bool) string {
	components := ""
	if referencedParent {
		components = "components:\n  schemas:\n    Parent:\n      type: object\n      discriminator: {propertyName: kind}\n"
	}

	return "openapi: 3.0.3\npaths:\n  /things:\n    post:\n      operationId: selected\n      requestBody:\n        content:\n          application/json:\n            schema:\n" + indentSchema(schema) + components
}

func schemaPointer(pointer string) string {
	if len(pointer) > 0 && pointer[0] == '#' {
		return pointer
	}

	return "#/paths/~1things/post/requestBody/content/application~1json/schema" + pointer
}

func indentSchema(schema string) string {
	result := ""
	for _, line := range splitLines(schema) {
		result += "              " + line + "\n"
	}

	return result
}

func splitLines(value string) []string {
	lines := []string{}
	start := 0

	for index, character := range value {
		if character != '\n' {
			continue
		}

		lines = append(lines, value[start:index])
		start = index + 1
	}

	return append(lines, value[start:])
}
