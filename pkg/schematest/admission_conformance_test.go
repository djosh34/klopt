//nolint:godoclint,lll // Full authored documents and exact cross-seam diagnostics are intentionally visible.
package schematest_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/schematest" //nolint:depguard // The conformance matrix must exercise the public Build seam.
	"github.com/djosh34/klopt/pkg/validation"

	"github.com/stretchr/testify/require"
)

type admissionClass string

const (
	admissionAccepted        admissionClass = "accepted"
	admissionInvalidOAS      admissionClass = "invalid_oas"
	admissionProfileExcluded admissionClass = "Klopt_profile_excluded"
)

type admissionDiagnostic struct {
	class      admissionClass
	pointer    string
	production string
	clean      string
}

func TestPublicAdmissionConformance(t *testing.T) {
	t.Parallel()

	const schemaPointer = "#/paths/~1things/post/requestBody/content/application~1json/schema"

	tests := []struct {
		name       string
		document   string
		diagnostic admissionDiagnostic
	}{
		{
			name: "OAS 3.0 patch and Semantic Version metadata",
			document: `{
				"openapi":"3.0.999-alpha.1+build.9",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}}}
			}`,
			diagnostic: admissionDiagnostic{class: admissionAccepted},
		},
		{
			name: "typed nullable false and matching default",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
					"type":"string","nullable":false,"default":"authored"
				}}}}}}}
			}`,
			diagnostic: admissionDiagnostic{class: admissionAccepted},
		},
		{
			name: "typeless nullable and exact semantic enum",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
					"nullable":true,"enum":[null,1,1.0,{"a":1,"b":[true]},{"b":[true],"a":1.0}]
				}}}}}}}
			}`,
			diagnostic: admissionDiagnostic{class: admissionAccepted},
		},
		{
			name: "complete admitted type and format pairs",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
					"type":"object","properties":{
						"int32":{"type":"integer","format":"int32"},
						"int64":{"type":"integer","format":"int64"},
						"numberInt32":{"type":"number","format":"int32"},
						"numberInt64":{"type":"number","format":"int64"},
						"float":{"type":"number","format":"float"},
						"double":{"type":"number","format":"double"},
						"byte":{"type":"string","format":"byte"},
						"date":{"type":"string","format":"date"},
						"dateTime":{"type":"string","format":"date-time"},
						"email":{"type":"string","format":"email"},
						"ipv4":{"type":"string","format":"ipv4"},
						"uuid":{"type":"string","format":"uuid"},
						"uuidv4":{"type":"string","format":"uuidv4"},
						"uuidDashV4":{"type":"string","format":"uuid-v4"},
						"cidr":{"type":"string","format":"cidr"},
						"ipv4CIDR":{"type":"string","format":"ipv4-cidr"},
						"password":{"type":"string","format":"password"}
					}
				}}}}}}}
			}`,
			diagnostic: admissionDiagnostic{class: admissionAccepted},
		},
		{
			name: "request direction",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
					"type":"object","required":["output","input"],"properties":{
						"output":{"type":"string","readOnly":true},
						"input":{"type":"string","writeOnly":true}
					}
				}}}}}}}
			}`,
			diagnostic: admissionDiagnostic{class: admissionAccepted},
		},
		{
			name: "leading assertion and ASCII punctuation identity escape",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
					"type":"string","allOf":[{"pattern":"^(?=a)a$"},{"pattern":"^\\.$"}]
				}}}}}}}
			}`,
			diagnostic: admissionDiagnostic{class: admissionAccepted},
		},
		{
			name: "legal inert metadata",
			document: `{
				"openapi":"3.0.4",
				"info":{"title":"Admission","version":"1"},
				"externalDocs":{"description":"API docs","url":"https://example.test/api"},
				"paths":{"/things":{"post":{"operationId":"selected","description":"operation docs","requestBody":{"description":"body docs","content":{"application/json":{"schema":{
					"type":"object","title":"Payload","description":"schema docs","default":{},"example":{"ignored":true},"deprecated":true,
					"externalDocs":{"description":"schema docs","url":"https://example.test/schema"},
					"xml":{"name":"payload","namespace":"https://example.test/xml","prefix":"x","attribute":false,"wrapped":true},
					"x-note":{"legal":"extension"}
				}}}}}}}
			}`,
			diagnostic: admissionDiagnostic{class: admissionAccepted},
		},
		{
			name:     "malformed OAS version",
			document: `{"openapi":"3.0","paths":{}}`,
			diagnostic: admissionDiagnostic{
				class:      admissionInvalidOAS,
				pointer:    "#/openapi",
				production: "#/openapi: OpenAPI document version must be a Semantic Versioning 2.0.0 version",
				clean:      "#/openapi: must be a valid Semantic Version: version core must have major, minor, and patch",
			},
		},
		{
			name:     "unsupported OAS major and minor feature set",
			document: `{"openapi":"3.1.0","paths":{}}`,
			diagnostic: admissionDiagnostic{
				class:      admissionProfileExcluded,
				pointer:    "#/openapi",
				production: "#/openapi: OpenAPI document feature set 3.1 is outside the Klopt 3.0 profile",
				clean:      "#/openapi: feature set 3.1 is outside the Klopt 3.0 profile",
			},
		},
		{
			name: "nullable must be boolean",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"string","nullable":null}}}}}}}
			}`,
			diagnostic: selectedSchemaDiagnostic(
				admissionInvalidOAS,
				schemaPointer+"/nullable",
				"nullable must be a boolean",
				"must be a boolean",
			),
		},
		{
			name: "default must match its explicit type",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"string","default":1}}}}}}}
			}`,
			diagnostic: selectedSchemaDiagnostic(
				admissionInvalidOAS,
				schemaPointer+"/default",
				`must conform to type "string"`,
				"must conform to the explicit type in the same Schema Object",
			),
		},
		{
			name: "empty enum profile exclusion",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"enum":[]}}}}}}}
			}`,
			diagnostic: selectedSchemaDiagnostic(
				admissionProfileExcluded,
				schemaPointer+"/enum",
				"empty enum is outside the Klopt profile",
				"empty enum is outside the Klopt profile",
			),
		},
		{
			name: "unknown legal format profile exclusion",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"string","format":"hostname"}}}}}}}
			}`,
			diagnostic: selectedSchemaDiagnostic(
				admissionProfileExcluded,
				schemaPointer+"/format",
				`format "hostname" is legal OAS but outside the Klopt profile`,
				`format "hostname" is legal OAS but outside the Klopt profile`,
			),
		},
		{
			name: "incompatible type and format profile exclusion",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"string","format":"int32"}}}}}}}
			}`,
			diagnostic: selectedSchemaDiagnostic(
				admissionProfileExcluded,
				schemaPointer+"/format",
				`type/format pair "string"/"int32" is legal OAS but outside the Klopt profile`,
				`type/format pair "string"/"int32" is legal OAS but outside the Klopt profile`,
			),
		},
		{
			name: "request direction conflict",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
					"type":"object","properties":{"value":{"type":"string","readOnly":true,"writeOnly":true}}
				}}}}}}}
			}`,
			diagnostic: selectedSchemaDiagnostic(
				admissionProfileExcluded,
				schemaPointer+"/properties/value/writeOnly",
				"readOnly and writeOnly cannot both be true in the Klopt profile",
				"readOnly and writeOnly cannot both be true in the Klopt profile",
			),
		},
		{
			name: "nullable leading assertion remainder",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"string","pattern":"^(?=a)$"}}}}}}}
			}`,
			diagnostic: selectedSchemaDiagnostic(
				admissionProfileExcluded,
				schemaPointer+"/pattern",
				"authored pattern is outside the Klopt profile: pattern at byte 1: valid ECMAScript 5.1 syntax is unsupported: pattern syntax at byte 1: leading assertions require a consuming remainder",
				"authored pattern is outside the Klopt profile: leading assertions require a consuming remainder",
			),
		},
		{
			name: "non-ASCII identity escape",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"string","pattern":"^\\é$"}}}}}}}
			}`,
			diagnostic: selectedSchemaDiagnostic(
				admissionProfileExcluded,
				schemaPointer+"/pattern",
				"authored pattern is outside the Klopt profile: pattern at byte 1: invalid syntax: pattern syntax at byte 1: unknown or forbidden identity escape",
				"authored pattern is outside the Klopt profile: identity escape \\é is outside the pattern profile",
			),
		},
		{
			name: "document discriminator wins over selected compiler failure",
			document: `{
				"openapi":"3.0.4",
				"components":{"schemas":{"Unused":{"discriminator":{"propertyName":"kind"}}}},
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":1}}}}}}}
			}`,
			diagnostic: documentProfileDiagnostic(
				"#/components/schemas/Unused/discriminator",
				"authored discriminator is outside the Klopt profile",
			),
		},
		{
			name: "adjacent oneOf wins over discriminator",
			document: `{
				"openapi":"3.0.4",
				"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{
					"oneOf":[{}],"discriminator":{"propertyName":"kind"}
				}}}}}}}
			}`,
			diagnostic: documentProfileDiagnostic(
				schemaPointer+"/oneOf",
				"authored oneOf is outside the Klopt profile",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertProductionAdmission(t, []byte(test.document), test.diagnostic)
			assertCleanAdmission(t, []byte(test.document), test.diagnostic)
		})
	}
}

func TestPublicDocumentProfileTraversalConformance(t *testing.T) {
	t.Parallel()

	const selected = `"paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}}}`

	tests := []struct {
		name       string
		document   string
		diagnostic admissionDiagnostic
	}{
		{
			name:     "orphan nested schema self cycle",
			document: `{"openapi":"3.0.4",` + selected + `,"components":{"schemas":{"Loop":{"properties":{"self":{"$ref":"#/components/schemas/Loop"}}}}}}`,
			diagnostic: referenceProfileDiagnostic(
				"#/components/schemas/Loop/properties/self/$ref",
				"recursive schema graph reaching #/components/schemas/Loop is outside the Klopt profile",
			),
		},
		{
			name: "orphan nested schema mutual composition cycle",
			document: `{"openapi":"3.0.4",` + selected + `,"components":{"schemas":{` +
				`"A":{"allOf":[{"$ref":"#/components/schemas/B"}]},` +
				`"B":{"properties":{"a":{"$ref":"#/components/schemas/A"}}}}}}`,
			diagnostic: referenceProfileDiagnostic(
				"#/components/schemas/B/properties/a/$ref",
				"recursive schema graph reaching #/components/schemas/A is outside the Klopt profile",
			),
		},
		{
			name: "orphan nested callback cycle",
			document: `{"openapi":"3.0.4",` + selected + `,"components":{"callbacks":{"Loop":{` +
				`"/again":{"post":{"callbacks":{"back":{"$ref":"#/components/callbacks/Loop"}}}}}}}}`,
			diagnostic: referenceProfileDiagnostic(
				"#/components/callbacks/Loop/~1again/post/callbacks/back/$ref",
				"recursive callback reference reaching #/components/callbacks/Loop is outside the Klopt profile",
			),
		},
		{
			name: "cyclic example aliases",
			document: `{"openapi":"3.0.4",` + selected + `,"components":{"examples":{` +
				`"A":{"$ref":"#/components/examples/B"},"B":{"$ref":"#/components/examples/A"}}}}`,
			diagnostic: referenceProfileDiagnostic(
				"#/components/examples/A/$ref",
				"recursive example reference reaching #/components/examples/B is outside the Klopt profile",
			),
		},
		{
			name:     "missing local example target remains invalid OAS",
			document: `{"openapi":"3.0.4",` + selected + `,"components":{"examples":{"Bad":{"$ref":"#/components/examples/Missing"}}}}`,
			diagnostic: admissionDiagnostic{
				class:      admissionInvalidOAS,
				pointer:    "#/components/examples/Bad/$ref",
				production: `resolve example reference at #/components/examples/Bad/$ref: resolve reference "#/components/examples/Missing" from #/components/examples/Bad through #/components/examples/Missing: pointer #/components/examples token "Missing": member "Missing" does not exist`,
				clean:      `#/components/examples/Bad/$ref: local reference target #/components/examples/Missing does not exist`,
			},
		},
		{
			name:     "malformed local example reference remains invalid OAS",
			document: `{"openapi":"3.0.4",` + selected + `,"components":{"examples":{"Bad":{"$ref":"#/bad~2"}}}}`,
			diagnostic: admissionDiagnostic{
				class:      admissionInvalidOAS,
				pointer:    "#/components/examples/Bad/$ref",
				production: `resolve example reference at #/components/examples/Bad/$ref: resolve reference "#/bad~2" from #/components/examples/Bad through #/bad~2: reference "#/bad~2" token "bad~2": ~2 is invalid`,
				clean:      `#/components/examples/Bad/$ref: malformed JSON Pointer token "bad~2": unknown escape ~2`,
			},
		},
		{
			name:     "external path item before request collection",
			document: `{"openapi":"3.0.4","paths":{"/external":{"$ref":"other.yaml#/Path"},"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}}}}`,
			diagnostic: admissionDiagnostic{
				class:      admissionProfileExcluded,
				pointer:    "#/paths/~1external/$ref",
				production: `compile schema at #/paths/~1external/$ref: external reference "other.yaml#/Path" is outside the Klopt profile`,
				clean:      `#/paths/~1external/$ref: external reference "other.yaml#/Path" is outside the Klopt profile`,
			},
		},
		{
			name: "cyclic path items before request collection",
			document: `{"openapi":"3.0.4","paths":{` +
				`"/a":{"$ref":"#/paths/~1b"},"/b":{"$ref":"#/paths/~1a"},` +
				`"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{}}}}}}}}`,
			diagnostic: referenceProfileDiagnostic(
				"#/paths/~1a/$ref",
				"recursive path item reference reaching #/paths/~1b is outside the Klopt profile",
			),
		},
		{
			name: "external request body before acquisition",
			document: `{"openapi":"3.0.4","paths":{"/things":{"post":{` +
				`"operationId":"selected","requestBody":{"$ref":"other.yaml#/Body"}}}}}`,
			diagnostic: admissionDiagnostic{
				class:      admissionProfileExcluded,
				pointer:    "#/paths/~1things/post/requestBody/$ref",
				production: `compile schema at #/paths/~1things/post/requestBody/$ref: external reference "other.yaml#/Body" is outside the Klopt profile`,
				clean:      `#/paths/~1things/post/requestBody/$ref: external reference "other.yaml#/Body" is outside the Klopt profile`,
			},
		},
		{
			name: "cyclic request bodies before acquisition",
			document: `{"openapi":"3.0.4","paths":{"/things":{"post":{` +
				`"operationId":"selected","requestBody":{"$ref":"#/components/requestBodies/A"}}}},` +
				`"components":{"requestBodies":{` +
				`"A":{"$ref":"#/components/requestBodies/B"},"B":{"$ref":"#/components/requestBodies/A"}}}}`,
			diagnostic: referenceProfileDiagnostic(
				"#/components/requestBodies/B/$ref",
				"recursive request body reference reaching #/components/requestBodies/A is outside the Klopt profile",
			),
		},
		{
			name: "orphan discriminator before missing operation ID",
			document: `{"openapi":"3.0.4","paths":{"/things":{"post":{` +
				`"requestBody":{"content":{"application/json":{"schema":{}}}}}}},` +
				`"components":{"schemas":{"Unused":{"discriminator":{"propertyName":"kind"}}}}}`,
			diagnostic: documentProfileDiagnostic(
				"#/components/schemas/Unused/discriminator",
				"authored discriminator is outside the Klopt profile",
			),
		},
		{
			name: "orphan discriminator before empty operation ID",
			document: `{"openapi":"3.0.4","paths":{"/things":{"post":{` +
				`"operationId":"","requestBody":{"content":{"application/json":{"schema":{}}}}}}},` +
				`"components":{"schemas":{"Unused":{"discriminator":{"propertyName":"kind"}}}}}`,
			diagnostic: documentProfileDiagnostic(
				"#/components/schemas/Unused/discriminator",
				"authored discriminator is outside the Klopt profile",
			),
		},
		{
			name: "orphan discriminator before private-invalid operation ID",
			document: `{"openapi":"3.0.4","paths":{"/things":{"post":{` +
				`"operationId":"not valid","requestBody":{"content":{"application/json":{"schema":{}}}}}}},` +
				`"components":{"schemas":{"Unused":{"discriminator":{"propertyName":"kind"}}}}}`,
			diagnostic: documentProfileDiagnostic(
				"#/components/schemas/Unused/discriminator",
				"authored discriminator is outside the Klopt profile",
			),
		},
		{
			name: "canonical delete before get",
			document: `{"openapi":"3.0.4","paths":{"/things":{` +
				`"delete":{"operationId":"deleted","requestBody":{"content":{"application/json":{"schema":{"uniqueItems":false}}}}},` +
				`"get":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"discriminator":{"propertyName":"kind"}}}}}}}}}`,
			diagnostic: documentProfileDiagnostic(
				"#/paths/~1things/delete/requestBody/content/application~1json/schema/uniqueItems",
				"authored uniqueItems is outside the Klopt profile",
			),
		},
	}

	externalSlots := []struct {
		name     string
		fragment string
		pointer  string
	}{
		{name: "component example", fragment: `"components":{"examples":{"External":{"$ref":"other.yaml#/Example"}}}`, pointer: "#/components/examples/External/$ref"},
		{name: "component security scheme", fragment: `"components":{"securitySchemes":{"External":{"$ref":"other.yaml#/Security"}}}`, pointer: "#/components/securitySchemes/External/$ref"},
		{name: "component link", fragment: `"components":{"links":{"External":{"$ref":"other.yaml#/Link"}}}`, pointer: "#/components/links/External/$ref"},
		{name: "parameter example", fragment: `"components":{"parameters":{"P":{"name":"p","in":"query","schema":{},"examples":{"External":{"$ref":"other.yaml#/Example"}}}}}`, pointer: "#/components/parameters/P/examples/External/$ref"},
		{name: "header example", fragment: `"components":{"headers":{"H":{"schema":{},"examples":{"External":{"$ref":"other.yaml#/Example"}}}}}`, pointer: "#/components/headers/H/examples/External/$ref"},
		{name: "media example", fragment: `"components":{"requestBodies":{"B":{"content":{"application/json":{"schema":{},"examples":{"External":{"$ref":"other.yaml#/Example"}}}}}}}`, pointer: "#/components/requestBodies/B/content/application~1json/examples/External/$ref"},
		{name: "response link", fragment: `"components":{"responses":{"R":{"description":"response","links":{"External":{"$ref":"other.yaml#/Link"}}}}}`, pointer: "#/components/responses/R/links/External/$ref"},
	}
	for _, slot := range externalSlots {
		tests = append(tests, struct {
			name       string
			document   string
			diagnostic admissionDiagnostic
		}{
			name:     "external " + slot.name,
			document: `{"openapi":"3.0.4",` + selected + `,` + slot.fragment + `}`,
			diagnostic: admissionDiagnostic{
				class:      admissionProfileExcluded,
				pointer:    slot.pointer,
				production: `compile schema at ` + slot.pointer + `: external reference "other.yaml#/` + externalTarget(slot.name) + `" is outside the Klopt profile`,
				clean:      slot.pointer + `: external reference "other.yaml#/` + externalTarget(slot.name) + `" is outside the Klopt profile`,
			},
		})
	}

	targets := []struct {
		name  string
		value string
	}{
		{name: "scalar", value: "1"},
		{name: "null", value: "null"},
		{name: "array", value: "[]"},
	}

	referenceKinds := []struct {
		name       string
		kind       string
		components string
	}{
		{name: "schema", kind: "schema", components: `"schemas":{"Bad":{"$ref":"#/x-target"}}`},
		{name: "example", kind: "example", components: `"examples":{"Bad":{"$ref":"#/x-target"}}`},
		{name: "link", kind: "link", components: `"links":{"Bad":{"$ref":"#/x-target"}}`},
		{name: "security scheme", kind: "security scheme", components: `"securitySchemes":{"Bad":{"$ref":"#/x-target"}}`},
		{name: "response", kind: "response", components: `"responses":{"Bad":{"$ref":"#/x-target"}}`},
	}
	for _, target := range targets {
		for _, referenceKind := range referenceKinds {
			production := "parse " + referenceKind.kind + " at #/x-target: referenced " +
				referenceKind.kind + " must be an object"
			if referenceKind.kind == "schema" {
				production = "parse schema at #/x-target: Schema Object must be an object"
			}

			tests = append(tests, struct {
				name       string
				document   string
				diagnostic admissionDiagnostic
			}{
				name: "non-object " + target.name + " " + referenceKind.name + " target",
				document: `{"openapi":"3.0.4",` + selected + `,"x-target":` + target.value +
					`,"components":{` + referenceKind.components + `}}`,
				diagnostic: admissionDiagnostic{
					class:      admissionInvalidOAS,
					pointer:    "#/x-target",
					production: production,
					clean:      "#/x-target: must be an object",
				},
			})
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertProductionAdmission(t, []byte(test.document), test.diagnostic)
			assertCleanAdmission(t, []byte(test.document), test.diagnostic)
		})
	}

	t.Run("example payload references stay inert", func(t *testing.T) {
		document := []byte(`{"openapi":"3.0.4",` + selected + `,"components":{"examples":{"Payload":{"value":{"$ref":"other.yaml#/Payload"}}}}}`)
		diagnostic := admissionDiagnostic{class: admissionAccepted}
		assertProductionAdmission(t, document, diagnostic)
		assertCleanAdmission(t, document, diagnostic)
	})
}

func TestPublicDiscriminatorShapeConformance(t *testing.T) {
	t.Parallel()

	const schemaPointer = "#/paths/~1things/post/requestBody/content/application~1json/schema"

	tests := []struct {
		name    string
		schema  string
		extra   string
		pointer string
	}{
		{name: "direct", schema: `{"discriminator":{"propertyName":"kind"}}`, pointer: schemaPointer + "/discriminator"},
		{name: "composed", schema: `{"allOf":[{"discriminator":{"propertyName":"kind"}}]}`, pointer: schemaPointer + "/allOf/0/discriminator"},
		{name: "referenced", schema: `{"$ref":"#/components/schemas/Target"}`, extra: `,"components":{"schemas":{"Target":{"discriminator":{"propertyName":"kind"}}}}`, pointer: "#/components/schemas/Target/discriminator"},
		{name: "mapped", schema: `{"discriminator":{"propertyName":"kind","mapping":{"a":"other.yaml#/A"}}}`, pointer: schemaPointer + "/discriminator"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document := []byte(`{"openapi":"3.0.4","paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":` + test.schema + `}}}}}}` + test.extra + `}`)
			diagnostic := documentProfileDiagnostic(test.pointer, "authored discriminator is outside the Klopt profile")
			assertProductionAdmission(t, document, diagnostic)
			assertCleanAdmission(t, document, diagnostic)
		})
	}
}

//nolint:paralleltest // Large three-point limit fixtures intentionally run sequentially within the test.
func TestPublicPatternLimitConformance(t *testing.T) {
	t.Parallel()

	matcherBelow := strings.Repeat(`\S`, 5_835) + strings.Repeat("a", 3_657) + strings.Repeat(".", 493)
	matcherExact := strings.Repeat(`\S`, 5_835) + strings.Repeat("a", 3_661) + strings.Repeat(".", 492)
	matcherOver := strings.Repeat(`\S`, 5_835) + strings.Repeat("a", 3_665) + strings.Repeat(".", 491)
	leadingBelow := "^(?=" + strings.Repeat(`\S`, 2_942) + ")(?=" +
		strings.Repeat(`\S`, 2_943) + strings.Repeat("a", 4_073) + strings.Repeat(".", 5) + ")a"
	leadingExact := "^(?=" + strings.Repeat(`\S`, 2_942) + ")(?=" +
		strings.Repeat(`\S`, 2_943) + strings.Repeat("a", 4_077) + strings.Repeat(".", 4) + ")a"
	leadingOver := "^(?=" + strings.Repeat(`\S`, 2_942) + ")(?=" +
		strings.Repeat(`\S`, 2_943) + strings.Repeat("a", 4_081) + strings.Repeat(".", 3) + ")a"

	tests := []struct {
		name               string
		below, exact, over string
	}{
		{name: "source bytes", below: "[" + strings.Repeat("a", 65_533) + "]", exact: "[" + strings.Repeat("a", 65_534) + "]", over: "[" + strings.Repeat("a", 65_535) + "]"},
		{name: "nesting", below: strings.Repeat("(", 99) + "a" + strings.Repeat(")", 99), exact: strings.Repeat("(", 100) + "a" + strings.Repeat(")", 100), over: strings.Repeat("(", 101) + "a" + strings.Repeat(")", 101)},
		{name: "AST nodes", below: strings.Repeat("a", 9_997), exact: strings.Repeat("a", 9_998), over: strings.Repeat("a", 9_999)},
		{name: "leading assertions", below: "^" + strings.Repeat("(?=a)", 63) + "a", exact: "^" + strings.Repeat("(?=a)", 64) + "a", over: "^" + strings.Repeat("(?=a)", 65) + "a"},
		{name: "counted endpoint", below: "a{999}", exact: "a{1000}", over: "a{1001}"},
		{
			name:  "cumulative repeat product",
			below: "(?:(?:(?:a{3}){3}){3}){37}",
			exact: "(?:(?:a{10}){10}){10}",
			over:  "(?:(?:a{7}){11}){13}",
		},
		{name: "translated matcher source", below: matcherBelow, exact: matcherExact, over: matcherOver},
		{name: "cumulative leading translated source", below: leadingBelow, exact: leadingExact, over: leadingOver},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, admitted := range []struct {
				name, pattern string
			}{{name: "boundary minus one", pattern: test.below}, {name: "exact boundary", pattern: test.exact}} {
				t.Run(admitted.name, func(t *testing.T) {
					assertPublicPatternAdmission(t, admitted.pattern, true)
				})
			}

			t.Run("first overflow", func(t *testing.T) {
				assertPublicPatternAdmission(t, test.over, false)
			})
		})
	}
}

func TestPublicAstralCharacterClassConformance(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		pattern  string
		admitted bool
	}{
		{name: "astral class atoms", pattern: "[😀]", admitted: true},
		{name: "astral class range", pattern: "[😀-😁]", admitted: false},
		{name: "range into surrogate block", pattern: "[a-😀]", admitted: true},
		{name: "range out of surrogate block", pattern: "[😀-\\uFFFF]", admitted: true},
		{name: "range across surrogate block", pattern: "[\ud7ff-\ue000]", admitted: true},
		{
			name: "surrogate split translated source overflow",
			pattern: strings.Repeat(`\S`, 5_835) + strings.Repeat("a", 3_661) +
				strings.Repeat(".", 491) + "[\ud7ff-\ue000]",
			admitted: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertPublicPatternAdmission(t, test.pattern, test.admitted)
		})
	}
}

func assertPublicPatternAdmission(t *testing.T, pattern string, admitted bool) {
	t.Helper()

	document := []byte(fmt.Sprintf(`{"openapi":"3.0.4","paths":{"/things":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{"type":"string","pattern":%s}}}}}}}}`, strconv.Quote(pattern)))

	const pointer = "#/paths/~1things/post/requestBody/content/application~1json/schema/pattern"

	parsed, productionErr := validation.Parse(document)
	if admitted {
		require.NoError(t, productionErr)
		require.Contains(t, parsed, "selected")
	} else {
		require.Nil(t, parsed)
		require.Equal(t, admissionProfileExcluded, classifyAdmission(productionErr))
		require.Equal(t, pointer, admissionPointer(productionErr))
	}

	report, cleanErr := schematest.Build(schematest.Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 0,
	}, func(schematest.Case) error { return nil })
	if admitted {
		require.NoError(t, cleanErr)
		require.Equal(t, schematest.MaxStepsReached, report.Stop)
	} else {
		require.Equal(t, schematest.Report{}, report)
		require.Equal(t, admissionProfileExcluded, classifyAdmission(cleanErr))
		require.Equal(t, pointer, admissionPointer(cleanErr))
	}
}

func classifyAdmission(err error) admissionClass {
	if err == nil {
		return admissionAccepted
	}

	if strings.Contains(err.Error(), "Klopt profile") || strings.Contains(err.Error(), "Klopt 3.0 profile") {
		return admissionProfileExcluded
	}

	return admissionInvalidOAS
}

func admissionPointer(err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()

	start := strings.IndexByte(message, '#')
	if start == -1 {
		return ""
	}

	end := len(message)
	for index := start; index < len(message); index++ {
		if message[index] == ':' || message[index] == ' ' || message[index] == '"' {
			end = index

			break
		}
	}

	return message[start:end]
}

func referenceProfileDiagnostic(pointer, detail string) admissionDiagnostic {
	return admissionDiagnostic{
		class:      admissionProfileExcluded,
		pointer:    pointer,
		production: "compile schema at " + pointer + ": " + detail,
		clean:      pointer + ": " + detail,
	}
}

func externalTarget(name string) string {
	if strings.Contains(name, "link") {
		return "Link"
	}

	if strings.Contains(name, "security") {
		return "Security"
	}

	return "Example"
}

func selectedSchemaDiagnostic(class admissionClass, pointer, productionDetail, cleanDetail string) admissionDiagnostic {
	return admissionDiagnostic{
		class:      class,
		pointer:    pointer,
		production: `compile operationId "selected": compile schema at ` + pointer + ": " + productionDetail,
		clean:      pointer + ": " + cleanDetail,
	}
}

func documentProfileDiagnostic(pointer, detail string) admissionDiagnostic {
	return admissionDiagnostic{
		class:      admissionProfileExcluded,
		pointer:    pointer,
		production: "compile schema at " + pointer + ": " + detail,
		clean:      pointer + ": " + detail,
	}
}

func assertProductionAdmission(t *testing.T, document []byte, diagnostic admissionDiagnostic) {
	t.Helper()

	parsed, err := validation.Parse(document)
	if diagnostic.class == admissionAccepted {
		require.NoError(t, err)
		require.Contains(t, parsed, "selected")

		return
	}

	require.Nil(t, parsed)
	require.EqualError(t, err, diagnostic.production)
	require.Equal(t, diagnostic.class, classifyAdmission(err))
	require.Equal(t, diagnostic.pointer, admissionPointer(err))
}

func assertCleanAdmission(t *testing.T, document []byte, diagnostic admissionDiagnostic) {
	t.Helper()

	called := false
	report, err := schematest.Build(schematest.Input{
		OpenAPI: document, OperationID: "selected", MaxSteps: 0,
	}, func(schematest.Case) error {
		called = true

		return nil
	})

	if diagnostic.class == admissionAccepted {
		require.NoError(t, err)
		require.Equal(t, schematest.MaxStepsReached, report.Stop)
		require.Zero(t, report.Steps)
		require.False(t, called)

		return
	}

	require.Equal(t, schematest.Report{}, report)
	require.EqualError(t, err, diagnostic.clean)
	require.Equal(t, diagnostic.class, classifyAdmission(err))
	require.Equal(t, diagnostic.pointer, admissionPointer(err))
	require.False(t, called)
}
