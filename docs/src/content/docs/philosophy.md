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

That is why `Validation.Validate` reads the original JSON. It checks presence, nullability, exact numbers, duplicate names, and other schema rules before unmarshalling can discard information. See the standard library's [`json.Unmarshal` rules](https://pkg.go.dev/encoding/json#Unmarshal).

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
