//nolint:godoclint,lll // Interface tables keep each OpenAPI wire case self-contained.
package validation_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/djosh34/klopt/pkg/validation"
	"github.com/stretchr/testify/require"
)

func TestParseBuildsBodyAndQueryMaps(t *testing.T) {
	t.Parallel()

	validations, decoders, err := validation.Parse([]byte(`openapi: 3.0.3
paths:
  /both:
    post:
      operationId: both
      parameters:
        - {name: q, in: query, schema: {type: string}}
      requestBody:
        content:
          application/json:
            schema: {type: object}
  /body:
    post:
      operationId: body
      requestBody:
        content:
          application/json:
            schema: {type: string}
  /query:
    get:
      operationId: query
      parameters:
        - {name: q, in: query, schema: {type: string}}
`))
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"body", "both"}, mapKeys(validations))
	require.ElementsMatch(t, []string{"both", "query"}, mapKeys(decoders))
}

func TestQueryDecoderStyleMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parameter string
		rawQuery  string
		expected  string
	}{
		{name: "primitive", parameter: `{name: q, in: query, schema: {type: string}}`, rawQuery: `q=red%20shoes`, expected: `{"q":"red shoes"}`},
		{name: "boolean", parameter: `{name: active, in: query, schema: {type: boolean}}`, rawQuery: `active=false`, expected: `{"active":false}`},
		{name: "integer", parameter: `{name: limit, in: query, schema: {type: integer}}`, rawQuery: `limit=1.0`, expected: `{"limit":1` + `}`},
		{name: "number", parameter: `{name: ratio, in: query, schema: {type: number}}`, rawQuery: `ratio=9007199254740993.25`, expected: `{"ratio":9007199254740993.25}`},
		{name: "form array exploded", parameter: `{name: tags, in: query, style: form, explode: true, schema: {type: array, items: {type: string}}}`, rawQuery: `tags=go&tags=red,blue`, expected: `{"tags":["go","red,blue"]}`},
		{name: "form array", parameter: `{name: ids, in: query, style: form, explode: false, schema: {type: array, items: {type: string}}}`, rawQuery: `ids=a%2Cb,c`, expected: `{"ids":["a,b","c"]}`},
		{name: "space array", parameter: `{name: ids, in: query, style: spaceDelimited, explode: false, schema: {type: array, items: {type: integer}}}`, rawQuery: `ids=10+20%2030`, expected: `{"ids":[10,20,30]}`},
		{name: "pipe array", parameter: `{name: flags, in: query, style: pipeDelimited, explode: false, schema: {type: array, items: {type: boolean}}}`, rawQuery: `flags=true%7Cfalse%7Ctrue`, expected: `{"flags":[true,false,true]}`},
		{name: "form object", parameter: objectParameter("form", false), rawQuery: `point=lat,52.1,long,4.3`, expected: `{"point":{"lat":52.1,"long":4.3}}`},
		{name: "form object exploded", parameter: objectParameter("form", true), rawQuery: `long=4.3&lat=52.1`, expected: `{"point":{"lat":52.1,"long":4.3}}`},
		{name: "space object", parameter: objectParameter("spaceDelimited", false), rawQuery: `point=lat%2052.1%20long%204.3`, expected: `{"point":{"lat":52.1,"long":4.3}}`},
		{name: "pipe object", parameter: objectParameter("pipeDelimited", false), rawQuery: `point=lat%7C52.1%7Clong%7C4.3`, expected: `{"point":{"lat":52.1,"long":4.3}}`},
		{name: "deep object", parameter: deepParameter(`role: {type: string}
          active: {type: boolean}`), rawQuery: `filter%5Brole%5D=admin&filter[active]=true`, expected: `{"filter":{"active":true,"role":"admin"}}`},
		{name: "JSON content", parameter: `name: filter
      in: query
      content:
        application/json:
          schema:
            type: object
            properties:
              tags: {type: array, items: {type: string}}
              user: {type: object, properties: {role: {type: string}}}`, rawQuery: `filter=%7B%22tags%22%3A%5B%22go%22%5D%2C%22user%22%3A%7B%22role%22%3A%22admin%22%7D%7D`, expected: `{"filter":{"tags":["go"],"user":{"role":"admin"}}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			decoder := parseQueryDecoder(t, test.parameter)
			actual, err := decoder.Decode(&url.URL{RawQuery: test.rawQuery})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestQueryDecoderRejectsRawPipeDelimitedArray(t *testing.T) {
	t.Parallel()

	decoder := parseQueryDecoder(t, `{name: flags, in: query, style: pipeDelimited, explode: false, schema: {type: array, items: {type: boolean}}}`)
	_, err := decoder.Decode(&url.URL{RawQuery: `flags=true|false`})
	require.ErrorContains(t, err, `pipeDelimited separator "|" must be percent-encoded as "%7C"`)
}

func TestQueryDecoderRejectsRawPipeDelimitedObject(t *testing.T) {
	t.Parallel()

	decoder := parseQueryDecoder(t, objectParameter("pipeDelimited", false))
	_, err := decoder.Decode(&url.URL{RawQuery: `point=lat|52.1|long|4.3`})
	require.ErrorContains(t, err, `pipeDelimited separator "|" must be percent-encoded as "%7C"`)
}

func TestQueryDecoderRejectsOddPipeDelimitedObjectTuple(t *testing.T) {
	t.Parallel()

	decoder := parseQueryDecoder(t, objectParameter("pipeDelimited", false))
	_, err := decoder.Decode(&url.URL{RawQuery: `point=lat%7C52.1%7Clong`})
	require.ErrorContains(t, err, "object serialization must contain name/value pairs")
}

func TestQueryDecoderUnescapesPipeDelimitedValueOnce(t *testing.T) {
	t.Parallel()

	decoder := parseQueryDecoder(t, `{name: q, in: query, style: pipeDelimited, explode: false, schema: {type: array, items: {type: string}}}`)
	actual, err := decoder.Decode(&url.URL{RawQuery: `q=left%257Cmiddle%7Cright`})
	require.NoError(t, err)
	require.JSONEq(t, `{"q":["left%7Cmiddle","right"]}`, string(actual))
}

func TestQueryDecoderAllowReservedIsInertForSchemaParameters(t *testing.T) {
	t.Parallel()

	for _, allowReserved := range []bool{false, true} {
		t.Run(fmt.Sprintf("allowReserved %t", allowReserved), func(t *testing.T) {
			t.Parallel()

			decoder := parseQueryDecoder(t, fmt.Sprintf(
				`{name: q, in: query, allowReserved: %t, schema: {type: string}}`,
				allowReserved,
			))

			for _, test := range []struct {
				rawQuery string
				expected string
			}{
				{rawQuery: `q=:/?@!$'()*,;`, expected: `{"q":":/?@!$'()*,;"}`},
				{rawQuery: `q=%23%5B%5D%26%3D%2B`, expected: `{"q":"#[]&=+"}`},
			} {
				actual, err := decoder.Decode(&url.URL{RawQuery: test.rawQuery})
				require.NoError(t, err)
				require.JSONEq(t, test.expected, string(actual))
			}
		})
	}
}

func TestQueryDecoderLiteralBracketNamesAcrossNonDeepStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parameter string
		rawQuery  string
		expected  string
	}{
		{name: "primitive", parameter: `{name: 'q[key]', in: query, schema: {type: string}}`, rawQuery: `q%5Bkey%5D=value`, expected: `{"q[key]":"value"}`},
		{name: "form array repeated", parameter: `{name: 'q[key]', in: query, schema: {type: array, items: {type: string}}}`, rawQuery: `q[key]=a&q%5Bkey%5D=b`, expected: `{"q[key]":["a","b"]}`},
		{name: "form array delimited", parameter: `{name: 'q[key]', in: query, explode: false, schema: {type: array, items: {type: string}}}`, rawQuery: `q[key]=a,b`, expected: `{"q[key]":["a","b"]}`},
		{name: "space array", parameter: `{name: 'q[key]', in: query, style: spaceDelimited, explode: false, schema: {type: array, items: {type: string}}}`, rawQuery: `q[key]=a+b`, expected: `{"q[key]":["a","b"]}`},
		{name: "pipe array", parameter: `{name: 'q[key]', in: query, style: pipeDelimited, explode: false, schema: {type: array, items: {type: string}}}`, rawQuery: `q%5Bkey%5D=a%7Cb`, expected: `{"q[key]":["a","b"]}`},
		{name: "form object named", parameter: bracketObjectParameter("q[key]", "form", false, "child"), rawQuery: `q[key]=child,value`, expected: `{"q[key]":{"child":"value"}}`},
		{name: "form object exploded", parameter: bracketObjectParameter("q", "form", true, "child[key]"), rawQuery: `child%5Bkey%5D=value`, expected: `{"q":{"child[key]":"value"}}`},
		{name: "space object", parameter: bracketObjectParameter("q[key]", "spaceDelimited", false, "child"), rawQuery: `q%5Bkey%5D=child+value`, expected: `{"q[key]":{"child":"value"}}`},
		{name: "pipe object", parameter: bracketObjectParameter("q[key]", "pipeDelimited", false, "child"), rawQuery: `q[key]=child%7Cvalue`, expected: `{"q[key]":{"child":"value"}}`},
		{name: "JSON content", parameter: `name: 'q[key]'
      in: query
      content: {application/json: {schema: {type: object}}}`, rawQuery: `q%5Bkey%5D=%7B%22child%22%3Atrue%7D`, expected: `{"q[key]":{"child":true}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decoder := parseQueryDecoder(t, test.parameter)
			actual, err := decoder.Decode(&url.URL{RawQuery: test.rawQuery})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(actual))
		})
	}
}

func TestQueryDecoderNamesAreCaseSensitive(t *testing.T) {
	t.Parallel()

	primitive := parseQueryDecoder(t, `{name: q, in: query, schema: {type: string}}`)
	actual, err := primitive.Decode(&url.URL{RawQuery: `Q=value`})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(actual))

	exploded := parseQueryDecoder(t, objectParameter("form", true))
	actual, err = exploded.Decode(&url.URL{RawQuery: `Lat=1&Long=2`})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(actual))

	deep := parseQueryDecoder(t, deepParameter(`role: {type: string}`))
	actual, err = deep.Decode(&url.URL{RawQuery: `Filter[role]=admin`})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(actual))

	_, err = deep.Decode(&url.URL{RawQuery: `filter[Role]=admin`})
	require.ErrorContains(t, err, "malformed or unknown deepObject child")
}

func TestQueryDecoderDeepObjectArrayExtension(t *testing.T) {
	t.Parallel()

	decoder := parseQueryDecoder(t, deepParameter(`key: {type: array, items: {type: string, minLength: 2}}
          key2: {type: array, items: {type: integer}}
          enabled: {type: array, items: {type: boolean}}
          ratio: {type: array, items: {type: number}}
          active: {type: boolean}`))

	actual, err := decoder.Decode(&url.URL{RawQuery: `filter[key]=a%2Cb&filter%5Bkey2%5D=1&filter[key]=cd&filter[enabled]=false&filter[key2]=2&filter[ratio]=1.25&filter[active]=true`})
	require.NoError(t, err)
	require.JSONEq(t, `{"filter":{"active":true,"enabled":[false],"key":["a,b","cd"],"key2":[1,2],"ratio":[1.25]}}`, string(actual))

	actual, err = decoder.Decode(&url.URL{RawQuery: `filter[key]=only`})
	require.NoError(t, err)
	require.JSONEq(t, `{"filter":{"key":["only"]}}`, string(actual))

	_, err = decoder.Decode(&url.URL{RawQuery: `filter[key]=x`})
	require.ErrorContains(t, err, "minLength")
	_, err = decoder.Decode(&url.URL{RawQuery: `filter[enabled]=1`})
	require.ErrorContains(t, err, "is not a boolean")
	_, err = decoder.Decode(&url.URL{RawQuery: `filter[ratio]=nope`})
	require.Error(t, err)
	_, err = decoder.Decode(&url.URL{RawQuery: `filter[active]=true&filter[active]=false`})
	require.ErrorContains(t, err, "duplicate scalar object property")
	_, err = decoder.Decode(&url.URL{RawQuery: `filter[unknown]=x`})
	require.ErrorContains(t, err, "malformed or unknown deepObject child")
	_, err = decoder.Decode(&url.URL{RawQuery: `filter[key][]=x`})
	require.ErrorContains(t, err, "malformed or unknown deepObject child")
	_, err = decoder.Decode(&url.URL{RawQuery: `filter[]=x`})
	require.ErrorContains(t, err, "malformed or unknown deepObject child")
	_, err = decoder.Decode(&url.URL{RawQuery: `filter[user][role]=x`})
	require.ErrorContains(t, err, "malformed or unknown deepObject child")
}

func TestQueryDecoderMissingEmptyUnknownDuplicateAndBinding(t *testing.T) {
	t.Parallel()

	decoder := parseQueryDecoder(t, `name: q
      in: query
      allowEmptyValue: true
      schema: {type: string, default: fallback}`)

	actual, err := decoder.Decode(&url.URL{RawQuery: `unknown=x`})
	require.NoError(t, err)
	require.JSONEq(t, `{"q":"fallback"}`, string(actual))
	actual, err = decoder.Decode(&url.URL{RawQuery: `q=`})
	require.NoError(t, err)
	require.JSONEq(t, `{"q":""}`, string(actual))

	_, err = decoder.Decode(&url.URL{RawQuery: `q=x&q=y`})
	require.ErrorContains(t, err, "duplicate scalar occurrence")

	type params struct {
		Query string `json:"q"`
	}

	var bound params
	require.NoError(t, json.Unmarshal(actual, &bound))
	require.Equal(t, "", bound.Query)

	required := parseQueryDecoder(t, `{name: q, in: query, required: true, schema: {type: string, default: fallback}}`)
	_, err = required.Decode(&url.URL{})
	require.ErrorContains(t, err, "required parameter is absent")

	notEmpty := parseQueryDecoder(t, `{name: tags, in: query, schema: {type: array, items: {type: string}}}`)
	_, err = notEmpty.Decode(&url.URL{RawQuery: `tags=`})
	require.ErrorContains(t, err, "empty value is not allowed")

	allowEmptyArray := parseQueryDecoder(t, `{name: tags, in: query, allowEmptyValue: true, schema: {type: array, items: {type: string}}}`)
	actual, err = allowEmptyArray.Decode(&url.URL{RawQuery: `tags=`})
	require.NoError(t, err)
	require.JSONEq(t, `{"tags":[""]}`, string(actual))
}

func TestQueryDecoderRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parameter string
		rawQuery  string
		contains  string
	}{
		{name: "bad name escape", parameter: `{name: q, in: query, schema: {type: string}}`, rawQuery: `%zz=x`, contains: "invalid URL escape"},
		{name: "bad value escape", parameter: `{name: q, in: query, schema: {type: string}}`, rawQuery: `q=%zz`, contains: "invalid URL escape"},
		{name: "invalid name UTF-8", parameter: `{name: q, in: query, schema: {type: string}}`, rawQuery: `%FF=x`, contains: "not valid UTF-8"},
		{name: "invalid UTF-8", parameter: `{name: q, in: query, schema: {type: string}}`, rawQuery: `q=%FF`, contains: "not valid UTF-8"},
		{name: "invalid boolean", parameter: `{name: q, in: query, schema: {type: boolean}}`, rawQuery: `q=1`, contains: "is not a boolean"},
		{name: "non-integer", parameter: `{name: q, in: query, schema: {type: integer}}`, rawQuery: `q=1.5`, contains: "is not an integer"},
		{name: "odd object", parameter: objectParameter("form", false), rawQuery: `point=lat,1,long`, contains: "name/value pairs"},
		{name: "duplicate non-exploded array", parameter: `{name: q, in: query, explode: false, schema: {type: array, items: {type: string}}}`, rawQuery: `q=a&q=b`, contains: "duplicate non-exploded array"},
		{name: "invalid array item", parameter: `{name: q, in: query, explode: false, schema: {type: array, items: {type: boolean}}}`, rawQuery: `q=true,nope`, contains: "is not a boolean"},
		{name: "invalid repeated array item", parameter: `{name: q, in: query, schema: {type: array, items: {type: boolean}}}`, rawQuery: `q=true&q=nope`, contains: "is not a boolean"},
		{name: "duplicate non-exploded object", parameter: objectParameter("form", false), rawQuery: `point=lat,1,long,2&point=lat,1,long,2`, contains: "duplicate non-exploded object"},
		{name: "unknown object property", parameter: objectParameter("form", false), rawQuery: `point=x,1`, contains: "unknown object property"},
		{name: "duplicate object property", parameter: objectParameter("form", false), rawQuery: `point=lat,1,lat,2`, contains: "duplicate object property"},
		{name: "invalid object property", parameter: objectParameter("form", false), rawQuery: `point=lat,nope,long,2`, contains: "property \"lat\""},
		{name: "invalid exploded property", parameter: objectParameter("form", true), rawQuery: `lat=nope&long=2`, contains: "property \"lat\""},
		{name: "empty object", parameter: objectParameter("form", false), rawQuery: `point=`, contains: "empty value is not allowed"},
		{name: "JSON trailing", parameter: `name: q
      in: query
      content:
        application/json:
          schema: {type: object}`, rawQuery: `q=%7B%7Dx`, contains: "invalid character"},
		{name: "duplicate JSON", parameter: `name: q
      in: query
      content:
        application/json:
          schema: {type: string}`, rawQuery: `q=%22a%22&q=%22b%22`, contains: "duplicate JSON content"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			decoder := parseQueryDecoder(t, test.parameter)
			actual, err := decoder.Decode(&url.URL{RawQuery: test.rawQuery})
			require.Nil(t, actual)
			require.ErrorContains(t, err, test.contains)
		})
	}

	decoder := parseQueryDecoder(t, `{name: q, in: query, schema: {type: string}}`)
	actual, err := decoder.Decode(nil)
	require.Nil(t, actual)
	require.ErrorContains(t, err, "URL is nil")
}

func TestQueryCompileRejectionsAndLiteralBracketOwnership(t *testing.T) {
	t.Parallel()

	invalid := []struct {
		name       string
		parameters string
		contains   string
	}{
		{name: "both schema and content", parameters: `- name: q
      in: query
      schema: {type: string}
      content: {application/json: {schema: {type: string}}}`, contains: "exactly one of schema or content"},
		{name: "no direct type", parameters: `- {name: q, in: query, schema: {allOf: [{type: string}]}}`, contains: "must have a direct type"},
		{name: "JSON no direct type", parameters: `- name: q
      in: query
      content: {application/json: {schema: {allOf: [{type: object}]}}}`, contains: "must have a direct type"},
		{name: "required shape", parameters: `- {name: q, in: query, required: nope, schema: {type: string}}`, contains: "required"},
		{name: "allow empty shape", parameters: `- {name: q, in: query, allowEmptyValue: nope, schema: {type: string}}`, contains: "allowEmptyValue"},
		{name: "allow reserved shape", parameters: `- {name: q, in: query, allowReserved: nope, schema: {type: string}}`, contains: "allowReserved must be a boolean"},
		{name: "content with allow reserved false", parameters: `- name: q
      in: query
      allowReserved: false
      content: {application/json: {schema: {type: string}}}`, contains: "content cannot be combined with allowReserved"},
		{name: "content with allow reserved true", parameters: `- name: q
      in: query
      allowReserved: true
      content: {application/json: {schema: {type: string}}}`, contains: "content cannot be combined with allowReserved"},
		{name: "content with style", parameters: `- name: q
      in: query
      style: form
      content: {application/json: {schema: {type: string}}}`, contains: "content cannot be combined with style"},
		{name: "content with explode", parameters: `- name: q
      in: query
      explode: true
      content: {application/json: {schema: {type: string}}}`, contains: "content cannot be combined with explode"},
		{name: "content count", parameters: `- {name: q, in: query, content: {}}`, contains: "exactly one media type"},
		{name: "content media type", parameters: `- {name: q, in: query, content: {text/plain: {schema: {type: string}}}}`, contains: "only application/json"},
		{name: "content schema missing", parameters: `- {name: q, in: query, content: {application/json: {}}}`, contains: "application/json schema"},
		{name: "content media object shape", parameters: `- {name: q, in: query, content: {application/json: 1}}`, contains: "application/json schema"},
		{name: "content schema unsupported", parameters: `- {name: q, in: query, content: {application/json: {schema: {type: string, oneOf: [{type: string}]}}}}`, contains: "unsupported keyword"},
		{name: "nested array", parameters: `- {name: q, in: query, schema: {type: array, items: {type: array, items: {type: string}}}}`, contains: "primitive type"},
		{name: "array items missing", parameters: `- {name: q, in: query, schema: {type: array}}`, contains: "array items"},
		{name: "array items typeless", parameters: `- {name: q, in: query, schema: {type: array, items: {allOf: [{type: string}]}}}`, contains: "primitive type"},
		{name: "nested deep child", parameters: `- name: filter
      in: query
      style: deepObject
      explode: true
      schema: {type: object, additionalProperties: false, properties: {x: {type: object}}}`, contains: "primitive type"},
		{name: "unsupported explode", parameters: `- {name: q, in: query, style: pipeDelimited, explode: true, schema: {type: array, items: {type: string}}}`, contains: "unsupported"},
		{name: "scalar style", parameters: `- {name: q, in: query, style: pipeDelimited, explode: false, schema: {type: string}}`, contains: "unsupported"},
		{name: "style shape", parameters: `- {name: q, in: query, style: 1, schema: {type: string}}`, contains: "style"},
		{name: "explode shape", parameters: `- {name: q, in: query, explode: nope, schema: {type: string}}`, contains: "explode"},
		{name: "form exploded open", parameters: `- {name: q, in: query, schema: {type: object, properties: {x: {type: string}}}}`, contains: "additionalProperties false"},
		{name: "form named open", parameters: `- {name: q, in: query, explode: false, schema: {type: object, properties: {x: {type: string}}}}`, contains: "additionalProperties false"},
		{name: "form named explicit open", parameters: `- {name: q, in: query, explode: false, schema: {type: object, additionalProperties: true, properties: {x: {type: string}}}}`, contains: "additionalProperties false"},
		{name: "space named open", parameters: `- {name: q, in: query, style: spaceDelimited, explode: false, schema: {type: object, properties: {x: {type: string}}}}`, contains: "additionalProperties false"},
		{name: "pipe named schema open", parameters: `- {name: q, in: query, style: pipeDelimited, explode: false, schema: {type: object, additionalProperties: {type: string}, properties: {x: {type: string}}}}`, contains: "additionalProperties false"},
		{name: "deep open", parameters: `- {name: q, in: query, style: deepObject, explode: true, schema: {type: object, properties: {x: {type: string}}}}`, contains: "additionalProperties false"},
		{name: "deep bracket base", parameters: `- {name: 'q[x]', in: query, style: deepObject, explode: true, schema: {type: object, additionalProperties: false, properties: {y: {type: string}}}}`, contains: "parameter name"},
		{name: "deep empty property", parameters: `- {name: q, in: query, style: deepObject, explode: true, schema: {type: object, additionalProperties: false, properties: {'': {type: string}}}}`, contains: "property name"},
		{name: "deep left bracket property", parameters: `- {name: q, in: query, style: deepObject, explode: true, schema: {type: object, additionalProperties: false, properties: {'x[y': {type: string}}}}`, contains: "property name"},
		{name: "deep right bracket property", parameters: `- {name: q, in: query, style: deepObject, explode: true, schema: {type: object, additionalProperties: false, properties: {'x]y': {type: string}}}}`, contains: "property name"},
		{name: "object style", parameters: `- {name: q, in: query, style: matrix, explode: false, schema: {type: object, additionalProperties: false, properties: {x: {type: string}}}}`, contains: "unsupported"},
		{name: "properties shape", parameters: `- {name: q, in: query, style: form, explode: false, schema: {type: object, properties: []}}`, contains: "properties"},
		{name: "property typeless", parameters: `- {name: q, in: query, style: form, explode: false, schema: {type: object, properties: {x: {allOf: [{type: string}]}}}}`, contains: "direct type"},
		{name: "deep array items missing", parameters: `- {name: q, in: query, style: deepObject, explode: true, schema: {type: object, additionalProperties: false, properties: {x: {type: array}}}}`, contains: "array property"},
		{name: "deep nested array items", parameters: `- {name: q, in: query, style: deepObject, explode: true, schema: {type: object, additionalProperties: false, properties: {x: {type: array, items: {type: object}}}}}`, contains: "primitive type"},
		{name: "bad schema reference", parameters: `- {name: q, in: query, schema: {$ref: '#/components/schemas/Missing'}}`, contains: "resolve schema"},
		{name: "schema not object", parameters: `- {name: q, in: query, schema: 1}`, contains: "Schema Object must be an object"},
		{name: "type shape", parameters: `- {name: q, in: query, schema: {type: 1}}`, contains: "type"},
		{name: "unsupported type", parameters: `- {name: q, in: query, schema: {type: 'null'}}`, contains: "unsupported direct type"},
		{name: "unsupported schema", parameters: `- {name: q, in: query, schema: {type: string, oneOf: [{type: string}]}}`, contains: "unsupported keyword"},
		{name: "ownership collision", parameters: `- {name: lat, in: query, schema: {type: string}}
    - ` + objectParameter("form", true), contains: "ownership"},
		{name: "bracket collision", parameters: `- {name: 'filter[key]', in: query, schema: {type: string}}
    - ` + deepParameter(`key: {type: string}`), contains: "ownership"},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := validation.Parse(querySpec(test.parameters))
			require.ErrorContains(t, err, test.contains)
		})
	}

	decoder := parseQueryDecoder(t, `{name: 'tags[key]', in: query, schema: {type: string}}`)
	for _, rawQuery := range []string{`tags[key]=raw`, `tags%5Bkey%5D=encoded`} {
		actual, err := decoder.Decode(&url.URL{RawQuery: rawQuery})
		require.NoError(t, err)
		require.JSONEq(t, `{"tags[key]":"`+map[string]string{`tags[key]=raw`: "raw", `tags%5Bkey%5D=encoded`: "encoded"}[rawQuery]+`"}`, string(actual))
	}

	normal := parseQueryDecoder(t, `{name: tags, in: query, schema: {type: string}}`)
	actual, err := normal.Decode(&url.URL{RawQuery: `tags[key]=ignored`})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(actual))
}

func TestQueryDecoderConstraintsDefaultsAndConcurrency(t *testing.T) {
	t.Parallel()

	decoder := parseQueryDecoder(t, `{name: n, in: query, schema: {type: integer, minimum: 1, maximum: 3, default: 0}}`)
	_, err := decoder.Decode(&url.URL{})
	require.ErrorContains(t, err, "minimum")
	_, err = decoder.Decode(&url.URL{RawQuery: `n=4`})
	require.ErrorContains(t, err, "maximum")
	actual, err := decoder.Decode(&url.URL{RawQuery: `n=2`})
	require.NoError(t, err)
	require.JSONEq(t, `{"n":2}`, string(actual))

	const goroutines = 32

	var wait sync.WaitGroup

	errs := make(chan error, goroutines)
	for range goroutines {
		wait.Add(1)
		go func() {
			defer wait.Done()

			result, decodeErr := decoder.Decode(&url.URL{RawQuery: `n=3`})
			if decodeErr != nil || string(result) != `{"n":3}` {
				errs <- fmt.Errorf("result %s: %w", result, decodeErr)
			}
		}()
	}

	wait.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func TestQueryDecoderPreservesParameterOrderAndSortsValidationLookup(t *testing.T) {
	t.Parallel()

	_, decoders, err := validation.Parse(querySpec(`- {name: z, in: query, schema: {type: string}}
    - {name: a, in: query, required: true, schema: {type: integer}}`))
	require.NoError(t, err)
	actual, err := decoders["query"].Decode(&url.URL{RawQuery: `a=1&z=last`})
	require.NoError(t, err)
	require.Equal(t, `{"z":"last","a":1}`, string(actual))
}

func FuzzQueryDecoder(f *testing.F) {
	decoder := parseQueryDecoder(f, `{name: q, in: query, allowEmptyValue: true, schema: {type: array, items: {type: string}, maxItems: 5}}`)
	for _, seed := range []string{"", "q=x", "q=%FF", "q=a&q=b", "%zz=x", "unknown=value"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, rawQuery string) {
		first, firstErr := decoder.Decode(&url.URL{RawQuery: rawQuery})
		second, secondErr := decoder.Decode(&url.URL{RawQuery: rawQuery})
		require.Equal(t, first, second)
		require.Equal(t, fmt.Sprint(firstErr), fmt.Sprint(secondErr))

		if firstErr != nil {
			require.Nil(t, first)

			return
		}

		require.True(t, json.Valid(first))
	})
}

func parseQueryDecoder(t testing.TB, parameter string) *validation.QueryDecoder {
	t.Helper()

	_, decoders, err := validation.Parse(querySpec("- " + parameter))
	require.NoError(t, err)
	require.Contains(t, decoders, "query")

	return decoders["query"]
}

func querySpec(parameters string) []byte {
	lines := strings.Split(parameters, "\n")
	for index := 1; index < len(lines); index++ {
		lines[index] = strings.TrimPrefix(lines[index], "    ")
	}

	return []byte("openapi: 3.0.3\npaths:\n  /items:\n    get:\n      operationId: query\n      parameters:\n        " + strings.Join(lines, "\n        ") + "\n")
}

func objectParameter(style string, explode bool) string {
	return fmt.Sprintf(`name: point
      in: query
      style: %s
      explode: %t
      schema:
        type: object
        additionalProperties: false
        required: [lat, long]
        properties:
          lat: {type: number}
          long: {type: number}`, style, explode)
}

func deepParameter(properties string) string {
	return `name: filter
      in: query
      style: deepObject
      explode: true
      schema:
        type: object
        additionalProperties: false
        properties:
          ` + properties
}

func bracketObjectParameter(name string, style string, explode bool, property string) string {
	return fmt.Sprintf(`name: %q
      in: query
      style: %s
      explode: %t
      schema:
        type: object
        additionalProperties: false
        properties:
          %q: {type: string}`, name, style, explode, property)
}

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	return keys
}
