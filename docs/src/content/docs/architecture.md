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
2. It compiles every request schema in the document into one immutable graph. `AND` nodes preserve schema siblings and `allOf`; `OR` nodes represent `anyOf`; small leaf rules handle strings, numbers, arrays, and objects.
3. A signed root goal asks that graph for either a valid or an invalid value. The graph is not copied or complemented ahead of time: the requested true or false result is pushed through its nodes while decoding.
4. A lazy productivity check removes choices that cannot finish. It examines only states reached by the current decode and memoizes equivalent states instead of listing complete assignments or paths.
5. `Program.Decode` maps arbitrary native Go fuzz bytes to the remaining choices. Local weights favor useful deep and near-invalid paths, but every supported productive choice keeps a positive chance of selection.
6. The generated fuzz target runs the selected operation's validator and checks its result against the verdict returned with the generated JSON.

There is no precomputed list of cases or schema-specific seed corpus. The generated fuzz target adds only the empty byte slice as its ordinary-test baseline; Go's fuzz engine supplies and saves other byte tapes. Decoding is deterministic and resource-bounded even for empty, short, or exhausted input because missing tape bytes read as zero.

For the object schema above, a valid walk emits an object with a valid `name`. An invalid walk usually selects one private fault, such as omitting `name`, giving it the wrong JSON kind, or adding a forbidden property, while keeping the rest of the object valid whenever that is possible. Lower-probability choices can exercise broader or coordinated failures.

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

1. Compile the schema and both `allOf` branches under one `AND` node.
2. For a valid root goal, push `true` to every child of that node.
3. The number leaf receives all active number rules together and emits a value satisfying them. Here it can emit `6` or `9`.
4. For an invalid root goal, first try choices that push `false` to one child and `true` to its siblings. This can produce focused failures such as `3`, `12`, `4`, or `10`.
5. Keep only choices that the lazy productivity check proves can finish. Broader and multi-rule failures remain available at lower positive weights.

No complete cross-product is built. The signed rules meet only in the graph state reached by the current tape, and the primitive leaf resolves that small set of rules when it emits the value.
