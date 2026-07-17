---
title: Philosophy
description: Why klopt works on raw JSON and generates compiled data instead of validation code.
---

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
