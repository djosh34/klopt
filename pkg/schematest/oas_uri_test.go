//nolint:godoclint,lll // URI fixtures keep exact RFC forms visible beside their expectations.
package schematest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateURIReferenceAcceptsRFC3986IPLiteralForms(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"https://[::]/",
		"https://[::1]/",
		"https://[1::]/",
		"https://[1:2:3:4:5:6:7:8]/",
		"https://[1:2:3:4:5:6::8]/",
		"https://[1:2:3:4:5::8]/",
		"https://[1:2:3:4::8]/",
		"https://[1:2:3::8]/",
		"https://[1:2::8]/",
		"https://[1::8]/",
		"https://[::ffff:192.0.2.128]/",
		"https://[1:2:3:4:5:6:192.0.2.1]:443/path?query#fragment",
		"https://[v1.a]/",
		"https://[VAF.a:b!$&'()*+,;=-._~]:443/",
	} {
		require.NoError(t, validateURIReference(value), value)
	}
}

func TestValidateURIReferenceRejectsMalformedIPLiteralForms(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"https://[]/",
		"https://[not-ip]/",
		"https://[1:2:3:4:5:6:7:8:9]/",
		"https://[2001:db8:::1]/",
		"https://[::ffff:192.0.2.999]/",
		"https://[fe80::1%25eth0]/",
		"https://[::1%eth0]/",
		"https://[v.a]/",
		"https://[v1.]/",
		"https://[v1.a%20b]/",
		"https://[v1.a/b]/",
		"https://[::1]extra/",
		"https://[::1]:/",
		"https://[::1]:port/",
	} {
		require.Error(t, validateURIReference(value), value)
	}
}

func TestURIComponentAndFragmentScanningShareExactPercentTriplets(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"a%20b", "%00", "%ff"} {
		require.NoError(t, validateURIComponent(value, ""), value)
		require.NoError(t, validateURIFragment(value), value)
	}

	for _, value := range []string{"%", "%0", "%0g", "a%2"} {
		require.Error(t, validateURIComponent(value, ""), value)
		require.Error(t, validateURIFragment(value), value)
	}
}

func TestParseInputAnnotationIPLiteralParity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		jsonSchema  string
		yamlSchema  string
		wantPointer string
	}{
		{
			name:       "external documentation IPvFuture",
			jsonSchema: `{"externalDocs":{"url":"https://[v1.docs]:443/path"}}`,
			yamlSchema: `externalDocs: {url: "https://[v1.docs]:443/path"}`,
		},
		{
			name:       "XML namespace IPv6",
			jsonSchema: `{"xml":{"namespace":"urn://[2001:db8::1]/name"}}`,
			yamlSchema: `xml: {namespace: "urn://[2001:db8::1]/name"}`,
		},
		{
			name:        "external documentation malformed bracket host",
			jsonSchema:  `{"externalDocs":{"url":"https://[not-ip]/"}}`,
			yamlSchema:  `externalDocs: {url: "https://[not-ip]/"}`,
			wantPointer: "/externalDocs/url",
		},
		{
			name:        "XML namespace empty bracket host",
			jsonSchema:  `{"xml":{"namespace":"https://[]/"}}`,
			yamlSchema:  `xml: {namespace: "https://[]/"}`,
			wantPointer: "/xml/namespace",
		},
		{
			name:        "external documentation overlong IPv6",
			jsonSchema:  `{"externalDocs":{"url":"https://[1:2:3:4:5:6:7:8:9]/"}}`,
			yamlSchema:  `externalDocs: {url: "https://[1:2:3:4:5:6:7:8:9]/"}`,
			wantPointer: "/externalDocs/url",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for encoding, document := range map[string]string{
				"json": documentWithJSONSchema(test.jsonSchema),
				"yaml": documentWithYAMLSchema(test.yamlSchema),
			} {
				t.Run(encoding, func(t *testing.T) {
					t.Parallel()

					_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
					if test.wantPointer == "" {
						require.NoError(t, err)

						return
					}

					require.ErrorContains(t, err, test.wantPointer)
				})
			}
		})
	}
}

func TestParseInputExampleURLAndLocalReferenceFragmentParity(t *testing.T) {
	t.Parallel()

	for encoding, document := range map[string]string{
		"json": `{"openapi":"3.0.4","components":{"examples":{"a b":{"value":1}}},"paths":{"/":{"post":{"operationId":"selected","requestBody":{"content":{"application/json":{"schema":{},"examples":{"url":{"externalValue":"https://[::1]/example%20one"},"ref":{"$ref":"#/components/examples/a%20b"}}}}}}}}}`,
		"yaml": `openapi: 3.0.4
components:
  examples:
    a b: {value: 1}
paths:
  /:
    post:
      operationId: selected
      requestBody:
        content:
          application/json:
            schema: {}
            examples:
              url: {externalValue: "https://[::1]/example%20one"}
              ref: {$ref: "#/components/examples/a%20b"}
`,
	} {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			_, err := parseInput(Input{OpenAPI: []byte(document), OperationID: "selected"})
			require.NoError(t, err)
		})
	}
}
