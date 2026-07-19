---
title: OpenAPI Compatibility
description: The observable OpenAPI 3.0.x subset Klopt supports, annotates, extends, and rejects.
---

Klopt implements a request-focused subset of OpenAPI. This page describes observable behavior, not complete conformance to every OpenAPI feature. The [OpenAPI 3.0.3 specification](https://spec.openapis.org/oas/v3.0.3.html) and its extended subset of [JSON Schema Wright Draft 00](https://datatracker.ietf.org/doc/html/draft-wright-json-schema-00#section-4.2) define the surrounding standards.

## Version handling

| Support | Boundary | Canonical detail |
|---|---|---|
| OpenAPI `3.0.x` feature-set versions | The entire value must be strict [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html) with major `3` and minor `0`. Other feature sets and malformed values reject during Parse. | Patch numbers, prereleases, and build metadata are accepted without widening Klopt's feature subset. This follows OpenAPI's rule that `major.minor` identifies the feature set. |
| Request-focused subset | A valid `openapi` version does not imply that every OpenAPI construct is supported. | Schema, body, query, pattern, and generation boundaries below remain authoritative. |

## Schema Objects

| Support | Boundary | Canonical detail |
|---|---|---|
| JSON kinds and validation | `boolean`, `integer`, `number`, `string`, `array`, and `object` types are supported; `nullable: true` adds null only beside a same-object `type`. | Exact `enum`; exact numeric bounds, exclusive flags, and `multipleOf`; string lengths and patterns; array bounds, `items`, and semantic uniqueness; object bounds, `required`, `properties`, and boolean/schema `additionalProperties` are enforced on applicable instances. |
| Composition and references | Non-empty `allOf` and reachable local JSON Pointer references are supported. `oneOf`, `anyOf`, `not`, recursive schemas, external references, and unknown non-extension Schema Object keywords reject during Parse. | Every `allOf` branch validates the same instance. A Reference Object follows `$ref` and ignores siblings. |
| Runtime string formats | Runtime-enforced formats are `byte`, `date`, `date-time`, and `email`. | `format` must be a string and affects only string instances. Production validation is separate from generated request-body value construction. |
| Annotation-only formats | `binary`, `password`, `int32`, `int64`, `float`, `double`, unknown/vendor/case variants, and an empty format add no runtime rule. | They impose no width, range, precision, or additional format validation. Numeric formats remain annotations even on numeric schemas. |
| Defaults | `default` is behavior-bearing but Parse checks it only against same-object `type` and `nullable`. | An absent optional query parameter may emit a root or resolved default and then run final validation. Request defaults never synthesize an absent body. |
| Request direction | `readOnly` and `writeOnly` are boolean-shape-checked everywhere. On a direct property or resolved local target, `readOnly: true` removes request requiredness; a supplied value still validates. `writeOnly` adds no request restriction. | Both true reject only in property context. Direction does not propagate or aggregate across `allOf` branches. |
| Discriminator hint | A non-null object with string `propertyName` and optional string-valued `mapping` is shape-checked and otherwise inert. | It adds no requiredness, mapping resolution, variant selection, or validation. Accepting a bare discriminator without a composition keyword is an intentional permissive placement deviation. |
| Documentation annotations | Well-formed `title`, `description`, `deprecated`, `xml`, and `externalDocs` are shape-checked and non-validating. `example` and ordinary `x-*` are inert. | `x-valid-examples` and `x-invalid-examples` are the documented generated-test evidence exception. Accepting `xml` at a root or another non-property placement is an intentional permissive deviation. |

## Request bodies

| Support | Boundary | Canonical detail |
|---|---|---|
| Compile-time JSON media selection | Klopt selects the most specific matching entry in order: exact `application/json`, `application/*`, then `*/*`, including well-formed parameterized spellings. Other content is not compiled as a request validation. | Selection happens during Parse. There is no runtime `Content-Type` dispatch. |
| Schema-less JSON media | A selected Media Type Object may omit `schema`; omission and explicit `{}` impose no schema constraints. Null or non-object media/schema values reject during Parse at their source location. | The synthetic empty schema accepts every strict JSON kind. An explicit schema uses ordinary compilation and validation. |
| Presence | Nil or zero-length bytes mean an absent body; JSON `null` is a present value. Optional absence passes and required absence rejects. | Schema defaults do not create request bodies. `readOnly`/`writeOnly` request semantics apply within supplied bodies. |
| Strict JSON | One complete JSON value with surrounding whitespace is accepted before schema validation. | Empty whitespace, malformed JSON, duplicate object names, trailing input, or multiple values reject at runtime. |

## Query decoding

| Support | Boundary | Canonical detail |
|---|---|---|
| Style matrix | Scalars use `form`; arrays use exploded or compact `form`, compact `spaceDelimited`, or compact `pipeDelimited`; objects use exploded or compact `form`, compact space/pipe, or exploded one-level `deepObject`. | Direct primitive typing is required for style conversion. Canonical `%7C` pipes and `%5B`/`%5D` deep brackets preserve inverse decoding. See [Query Decoding](/klopt/query-decoding/). |
| Dynamic object properties | Untyped style-based additional properties decode as strings; compatible explicit scalar intersections choose boolean, integer, number, or string conversion. | Exact owners win, then known deep namespaces, then at most one open exploded-form fallback. Final schema validation always runs. |
| JSON content | Exactly one parsed `application/json` media entry is supported, including case variants and parameters. Its schema may be absent; one occurrence supplies one strict JSON value. | Parameter `schema` and `content` are exclusive. Content cannot carry parameter-level style, explode, allowReserved, example, or examples fields. |
| Presence and annotations | Optional absence omits the property; JSON `null` remains present. Explicit defaults may be emitted then validated. Boolean `allowReserved` is accepted for schema/style parameters without changing consumer decoding. | Unknown unclaimed names are ignored. Media Type examples and schema/style Parameter examples are inert at this boundary. |
| Documented policies/extensions | Repeated declared `deepObject` array children are supported; exact-owner precedence, string fallback, and sole-open-form ownership are deterministic Klopt policies. | These inverse-decoding choices are explicitly not portable OpenAPI guarantees; clients and servers must agree. |

## Patterns

| Support | Boundary | Canonical detail |
|---|---|---|
| Default mode | Closed portable ECMAScript 5.1 subset with search semantics and supported leading lookaheads. | Pattern source is capped at 64 KiB; default-only limits are 100 nesting levels, 10,000 AST nodes, 64 leading lookaheads, a 1,000 counted-repeat endpoint, a 1,000 cumulative nested-repeat product, and 1 MiB translated regexp per check. See [Patterns](/klopt/patterns/). |
| `UseRE2` | Raw Go `regexp` syntax and semantics for deliberately non-portable contracts. | It bypasses the ES parser and default-only limits, not the 64 KiB source cap. |
| Generated values | Production matching and generated construction are distinct. Generated request-body strings are ASCII-only and formats are opaque construction constraints. | A runtime-valid pattern, unusable evidence, empty language, or constructor budget can fail later in generated tests. |

## Generation

| Support | Boundary | Canonical detail |
|---|---|---|
| Runtime Parse | `validation.Parse` returns request-body validations and query decoders keyed by `operationId`. | It compiles the document for runtime reuse and does not generate files. |
| In-memory generation | `generate.GenerateInMemory(packageName, spec, validation.PatternOptions(...))` parses internally and returns caller-owned source bytes. | Success contains exactly `validate.go` and `validate_test.go`. Every error returns a nil map. |
| Generated-test boundary | Parsing, operation-ID safety, rendering, syntax, and formatting complete before generation succeeds. Test-suite construction is deliberately deferred. | Successful generation does not guarantee generated `TestValidations` passes; construction capability, contradictory/no-constructible languages, formats, evidence, or budgets can fail when `go test` runs without changing production validation. |

## Rejections

- **Parse:** unsupported Schema Object behavior, reference forms, malformed fixed fields, or unsupported composition is rejected rather than ignored. Use the supported subset, split the schema, or remove behavior Klopt cannot enforce.
- **Parse:** unsupported or ambiguous query serialization is rejected when the description cannot establish one inverse mapping. Choose a supported style, namespaced `deepObject`, or JSON content; see [Query decoding rejections](/klopt/query-decoding/#rejections-and-errors).
- **Parse:** malformed or over-limit default patterns and invalid raw Go patterns reject before requests. Rewrite or choose the intended mode; see [Pattern rejections](/klopt/patterns/#rejections-and-limits).
- **Runtime:** absent required input, malformed strict JSON/query wire data, conversion failures, and final schema violations reject the individual request. Send canonical wire data satisfying the compiled contract.
- **Generated `go test`:** construction capability, empty-language, evidence, and budget failures belong to generated tests. Simplify the construction problem or supply usable finite/local evidence without weakening production validation.
