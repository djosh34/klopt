---
title: Architecture
description: How OpenAPI schemas become runtime validations and targeted random JSON tests.
---

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

1. It admits the request schema through the same strict capability gate as validation.
2. It lowers the schema once into an occurrence tree and canonical accepted sets built from union, intersection, and complement.
3. It plans aggregate-valid, boundary, and focused-invalid cases with explicit expected validator verdicts.
4. It compiles each non-empty case into an immutable, certified `Program`.
5. Native Go fuzzing mutates byte tapes. `Program.Decode` deterministically constructs JSON, and the runner checks the validator result against the case verdict.

Every case includes canonical replay seeds, and ordinary tests execute all of them. Native fuzzing provides variation around those seeds while the sealed program keeps decoding deterministic and resource-bounded. For the object schema above, planned cases cover valid objects, a missing required `name`, a wrong type for `name`, and an unknown property.

Schema parsing is fuzzed independently. That separate OpenAPI-schema generator still uses Rapid to build supported documents and independently mutated invalid copies; request-body program execution has no Rapid dependency.

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
3. Intersect both accepted sets. The numeric result is integers from `4` through `10` that are multiples of `3`.
4. Compile the aggregate valid case from that merged set. It can emit `6` or `9`.
5. Build isolated rejected cases from the same context. For example, `3` fails only the minimum, `12` fails only the maximum, and values such as `4` or `10` fail only `multipleOf`.

The merge happens before program compilation. This keeps every accepted output inside all branches, while rejected cases target a specific rule without accidentally dropping unrelated siblings.
