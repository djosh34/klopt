klopt is a Go library and code generator that validates OpenAPI 3.0.3 JSON request bodies and decodes query parameters into JSON.

“Klopt” is Dutch for “is correct,” reflecting the library's focus on validation. The name is inspired by the naming of Google's code search engine [zoekt](https://github.com/sourcegraph/zoekt), Dutch for “searches.”

Read the [published documentation](https://djosh34.github.io/klopt/).

# Getting started

`pkg/validation` compiles an OpenAPI 3.0.3 document once. Use the result to validate raw JSON request bodies and decode query parameters into validated JSON.

## Install

```sh
go get github.com/djosh34/klopt/pkg/validation
```

```go
import "github.com/djosh34/klopt/pkg/validation"
```

## Parse once

```go
spec, err := os.ReadFile("openapi.yaml")
if err != nil {
	return err
}

validations, queryDecoders, err := validation.Parse(spec)
if err != nil {
	return err
}
```

Both maps are keyed by OpenAPI `operationId`. Parse at startup, then reuse the compiled values. Do not mutate them after parsing.

## Validate a request body

```go
func validateCreateThing(r *http.Request, requestValidation *validation.Validation) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return errors.Join(requestValidation.Validate(body)...)
}
```

Call it with `validations["createThing"]`. Empty bytes mean the body is absent. JSON `null` is a present body and follows the schema's `type` and `nullable` rules.

## Decode a query

```go
type ListThingsQuery struct {
	Tags   []string `json:"tags"`
	Limit  int      `json:"limit"`
}

func decodeListThings(r *http.Request, decoder *validation.QueryDecoder) (ListThingsQuery, error) {
	raw, err := decoder.Decode(r.URL)
	if err != nil {
		return ListThingsQuery{}, err
	}

	var query ListThingsQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return ListThingsQuery{}, err
	}

	return query, nil
}
```

Call it with `queryDecoders["listThings"]`. The decoder handles the OpenAPI wire format and returns ordinary JSON, leaving the final Go type under your control.

# Philosophy

## Validate before unmarshalling

These request bodies mean different things:

```json
{}
```

```json
{"name": null}
```

An ordinary Go struct can lose that distinction:

```go
type Input struct {
	Name *string `json:"name"`
}
```

After `json.Unmarshal`, `Name` is `nil` in both cases. An omitted field keeps its zero value; JSON `null` sets a pointer to `nil`. OpenAPI can independently say that `name` is required and whether it is nullable.

Technically, unmarshalling into `map[string]any` preserves this particular distinction: a missing key is omitted from the map, while a key containing JSON `null` has a `nil` value. Some validation libraries use that representation. It works, and `json.Marshal` can encode the map again without custom marshaling code. The tradeoff appears when the application still wants `Input`: it must unmarshal the original body a second time, marshal and unmarshal the map, or walk nested maps with type assertions and conversions. The last option recreates parts of `encoding/json`'s typed decoding and becomes especially finicky around nested values, integer types, struct tags, and custom `UnmarshalJSON` methods. Custom presence wrappers have similar bookkeeping costs in every affected model.

Generic unmarshalling can also discard information needed for validation. Given:

```json
{"sequence": 9007199254740993}
```

`json.Unmarshal` stores the number in an `any` as a `float64`, which rounds it to `9007199254740992`. `Decoder.UseNumber` avoids that particular conversion, but requires the whole generic-decoding path to use and interpret `json.Number` correctly.

That is why `Validation.Validate` reads `json.RawMessage`. Raw decoding delays conversion and keeps the original JSON available while the schema is checked. The validator can check presence, nullability, exact numbers, duplicate names, and other schema rules before ordinary unmarshalling creates the application's typed value. See the standard library's [`json.Unmarshal` rules](https://pkg.go.dev/encoding/json#Unmarshal), [`Decoder.UseNumber`](https://pkg.go.dev/encoding/json#Decoder.UseNumber), and [`json.RawMessage`](https://pkg.go.dev/encoding/json#RawMessage).

## Dynamic validation, plain generated data

At runtime, the library works like a dynamic Go validator:

```go
validations, _, err := validation.Parse(spec)
if err != nil {
	return err
}

errs := validations["createThing"].Validate(body)
```

The compiled result is also plain Go data that can be generated ahead of time:

```go
var createThing = &validation.Validation{
	SchemaPointer: "#/paths/~1things/post/requestBody/content/application~1json/schema",
	BodyRequired:  true,
	KindValidation: validation.KindValidation{
		Type: "string",
	},
	ObjectValidation: validation.ObjectValidation{
		AdditionalPropertiesAllowed: true,
	},
}
```

Dynamic OpenAPI libraries normally parse a specification when the process starts. Generated literals let an application or test call `Validate` immediately. The behavior still lives in one runtime validator instead of being duplicated across generated validation functions.

Fully generated validation code is difficult to make bug-free and difficult to test exhaustively. Generating compiled data is the middle way: no runtime specification parsing when literals are used, but only one validation implementation to harden.

## A deliberate subset

OpenAPI 3.0.3 is large. This library rejects unsupported behavior during parsing instead of guessing. Current examples include `oneOf`, `anyOf`, `not`, and reference cycles.

This is intentional. Clear rejection is safer than accepting a document with partial semantics. The supported model can grow as its behavior becomes testable.

## Generative testing

Validation receives extensive [Rapid](https://pkg.go.dev/pgregory.net/rapid) property testing. The tests generate supported OpenAPI schemas, mutate copies into invalid schemas, and generate valid and invalid JSON around individual constraints.

Generating JSON that has a known relationship to a complex schema is hard. That part is not perfect and remains active work. The architecture is designed to make every improvement test the same runtime validator used by applications.

OpenAPI details in this documentation follow the [OpenAPI 3.0.3 Schema Object](https://spec.openapis.org/oas/v3.0.3.html#schema-object).

# Query decoding

In this library, decoding a query means converting its OpenAPI wire format into validated JSON.

OpenAPI query parameters combine styles such as `form`, `spaceDelimited`, `pipeDelimited`, and `deepObject` with `explode`, repeated names, scalar conversion, and bracketed names. The [OpenAPI 3.0.3 Parameter Object](https://spec.openapis.org/oas/v3.0.3.html#parameter-object) shows how much the wire shape changes between combinations.

## One output format

Given a `deepObject` parameter named `filter`:

```text
filter[role]=admin&filter[active]=true
```

`QueryDecoder.Decode` returns:

```json
{"filter":{"active":true,"role":"admin"}}
```

OpenAPI 3.0.3 does not define how `deepObject` serializes array-valued object properties. Klopt extends the OpenAPI behavior with repeated bracketed keys:

```yaml
- name: tags
  in: query
  style: deepObject
  explode: true
  schema:
    type: object
    properties:
      key:
        type: array
        items:
          type: string
```

```text
tags[key]=item1&tags[key]=item2
```

decodes to:

```json
{"tags":{"key":["item1","item2"]}}
```

Use that JSON with any struct:

```go
raw, err := queryDecoders["listThings"].Decode(r.URL)
if err != nil {
	return err
}

var query ListThingsQuery
if err := json.Unmarshal(raw, &query); err != nil {
	return err
}
```

Or pass it to `json.Decoder` after validation. Callers only need to handle JSON; the OpenAPI style rules stay inside the query decoder.

## Why not `url.Values`

OpenAPI delimiters must be read before ordinary percent-decoding:

```text
ids=a%2Cb,c   → ["a,b", "c"]
ids=a,b,c     → ["a", "b", "c"]
```

In the first query, the encoded comma belongs to the first string. The raw comma separates array items. After decoding into `url.Values`, both values look like `a,b,c`, so that information is gone.

`QueryDecoder` therefore reads `URL.RawQuery`, claims names against the compiled parameters, applies `style` and `explode`, converts scalar types, and validates the resulting JSON value.

## Trade-off

A caller may decode the returned JSON into a Go struct, which means creating JSON and decoding it again. That extra work is accepted deliberately. A single familiar format, user-chosen structs, and fewer wire-decoding bugs matter more here than avoiding one intermediate representation.

# Architecture

## Validation

`validation.Parse` selects JSON request bodies and query parameters by `operationId`, resolves reachable local references, and compiles each schema into a `Validation` tree.

For example:

```yaml
openapi: 3.0.3
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              additionalProperties: false
              properties:
                name:
                  type: string
```

Compiles to data shaped like this:

```go
var createThing = &validation.Validation{
	SchemaPointer: "#/paths/~1things/post/requestBody/content/application~1json/schema",
	BodyRequired:  true,
	KindValidation: validation.KindValidation{
		Type: "object",
	},
	ObjectValidation: validation.ObjectValidation{
		Required: []string{"name"},
		Properties: []validation.PropertyValidation{{
			Name: "name",
			Validation: &validation.Validation{
				SchemaPointer: "#/paths/~1things/post/requestBody/content/application~1json/schema/properties/name",
				KindValidation: validation.KindValidation{
					Type: "string",
				},
				ObjectValidation: validation.ObjectValidation{
					AdditionalPropertiesAllowed: true,
				},
			},
		}},
		AdditionalPropertiesAllowed: false,
	},
}
```

Runtime parsing and generated literals produce the same compiled model. Generated source contains data, not generated validation functions. `Validate` walks that model while retaining raw JSON at every nested value.

For runtime validation, `allOf` stays as separate child validations. Each branch checks the same raw value, matching OpenAPI's rule that its schemas are [validated independently but together](https://spec.openapis.org/oas/v3.0.3.html#composition-and-inheritance-polymorphism).

## Test generation

The test generator does more than draw arbitrary JSON:

1. It compiles a schema into a constructive **Domain**: reachable JSON kinds plus exact constraints for numbers, strings, arrays, objects, and enums.
2. It plans focused **cases** from that domain: an aggregate valid case, useful valid partitions such as boundaries, and rejected cases that fail one constraint while satisfying the others when possible.
3. It attaches a Rapid generator to each case. The generator recursively draws random JSON values from that case's domain.
4. Each case runs as a Rapid property. The value is marshalled as exact JSON and passed to the validator callback. The planned accepted or rejected result is checked against that callback.

The cases provide coverage intent; Rapid provides variation and shrinking inside each case. For the object schema above, planned cases cover valid objects, a missing required `name`, a wrong type for `name`, and an unknown property. Each property can produce many concrete JSON bodies rather than one fixed fixture.

Schema parsing is fuzzed too. A separate Rapid generator builds supported OpenAPI documents, then makes independently mutated invalid copies. This tests both successful compilation and precise rejection of malformed schemas.

### Why `allOf` must merge before generating

Consider a schema where neither branch describes the final valid values:

```yaml
allOf:
  - type: integer
    minimum: 4
  - maximum: 10
    multipleOf: 3
```

At the `allOf` level there is no ready-made value to draw. Choosing from either branch alone is unsafe: `4` satisfies the first branch but not `multipleOf: 3`; `3` satisfies the second branch but not `minimum: 4`.

The generator handles it step by step:

1. Compile the first branch as integers greater than or equal to `4`.
2. Compile the second branch as all JSON kinds, with numbers no greater than `10` and restricted to multiples of `3`.
3. Intersect both domains. The numeric result is integers from `4` through `10` that are multiples of `3`.
4. Build the aggregate valid generator from that merged domain. It can draw `6` or `9`.
5. Build isolated rejected cases from the same context. For example, `3` fails only the minimum, `12` fails only the maximum, and values such as `4` or `10` fail only `multipleOf`.

The merge happens before Rapid generation. This is what lets random values remain meaningful: every accepted draw satisfies all branches, while rejected draws target a specific rule without accidentally failing unrelated siblings.

# Roadmap

## Test generation

- Generate strings for `pattern` and `format` without depending on `x-valid-examples` and `x-invalid-examples`.
- Correctly intersect string patterns across `allOf`.

## Composition

- Potentially support `anyOf`, `oneOf`, and `not`.
- Validation is easier than generating useful random JSON for these branches. Test generation is the limiting problem.

## Ongoing quality

- Improve JSON value generation and shrinking.
- Expand schema mutation and cross-validator coverage as new behavior lands.

# Contributing

klopt is currently a greenfield project, and contributions are not yet accepted. Creating issues is welcome.

# License

All rights reserved.
