---
title: Architecture
description: How OpenAPI schemas become runtime validations and generated Go data.
---

## Validation

`validation.Parse` selects JSON request bodies and path/query parameters by `operationId`, resolves reachable local references, and compiles each schema into a `Validation` tree.

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

For runtime validation, `allOf` stays as separate child validations. Each branch checks the same raw value, matching OpenAPI's rule that its schemas are [validated independently but together](https://spec.openapis.org/oas/v3.0.4.html#composition-and-inheritance-polymorphism). Request-body `anyOf` keeps authored alternatives as child validations and accepts when at least one succeeds, after local and `allOf` constraints pass.

Path and query decoders support the current direct/root primitive schema-style `anyOf` subset. They try alternatives in source order, validate before selecting one, and retain parent and `allOf` constraints. Nested, object, array, and content-based parameter `anyOf` remain unsupported.

## Generated artifacts

Generation renders the same compiled validation trees used at runtime. `Generate` and `GenerateInMemory` produce only `validate.go`; they do not embed the source document or generate tests. The committed example keeps `validate_test.go` under human ownership, with deterministic operation and body tables.
