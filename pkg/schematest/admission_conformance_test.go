//nolint:godoclint,lll // Full authored documents and exact cross-seam diagnostics are intentionally visible.
package schematest_test

import (
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
	require.NotEmpty(t, diagnostic.pointer)
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
	require.False(t, called)
	require.NotEmpty(t, diagnostic.pointer)
}
