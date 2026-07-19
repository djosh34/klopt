---
title: Query Decoding
description: Supported OpenAPI query styles, canonical wire forms, ownership policies, and failure boundaries.
---

## Decoding model

`QueryDecoder.Decode` reads `URL.RawQuery`, claims each wire name, percent-decodes exactly once, converts declared scalar text, emits one JSON object, and runs final schema validation. The returned bytes are validated JSON; callers can unmarshal them into any Go type.

This ordering preserves encoded data that would be lost by decoding into `url.Values` first. For example, the raw comma in `?ids=a%2Cb,c` separates items while `%2C` belongs to the first string, producing `{"ids":["a,b","c"]}`.

## Style matrix

Style-based parameters require a direct root type. Scalars support `form`; arrays and objects support the cells below. Array items and declared object properties must have direct primitive types, except for the documented `deepObject` array extension.

| Value | Style | Explode | Wire ownership and delimiter |
|---|---|---:|---|
| boolean, integer, number, string | `form` | either/default | One occurrence named by the parameter; duplicate scalars reject. |
| array | `form` | `true` | Repeated parameter names, one item per occurrence. |
| array | `form` | `false` | One named occurrence with raw comma delimiters. |
| array | `spaceDelimited` | `false` | One named occurrence with decoded space delimiters. |
| array | `pipeDelimited` | `false` | One named occurrence with canonical `%7C` delimiters. |
| object | `form` | `true` | Declared property names, or one open dynamic bare-key namespace. |
| object | `form` | `false` | One named occurrence containing alternating comma-separated names and values. |
| object | `spaceDelimited` | `false` | One named occurrence containing alternating space-separated names and values. |
| object | `pipeDelimited` | `false` | One named occurrence containing alternating canonical `%7C`-separated names and values. |
| object | `deepObject` | `true` | One-level canonical `name%5Bchild%5D` occurrences. |

## Dynamic object ownership

Style-based object schemas can allow additional scalar values. Omitted `additionalProperties`, `true`, `{}`, and schemas with no explicit root/`allOf` type restriction decode dynamic wire values as JSON strings. This is Klopt's deterministic fallback, not an OpenAPI type default: text such as `true` or `1` remains `"true"` or `"1"`. An explicit compatible `boolean`, `integer`, `number`, or `string` intersection selects that conversion, including through local references and nested `allOf`; final validation still runs.

Ownership is deterministic:

1. Exact parameter names and declared exploded properties win.
2. A known `deepObject` base receives canonical one-level undeclared children.
3. The sole open exploded `form` map receives otherwise unclaimed bare names.
4. Other unknown names are ignored.

Only one open exploded `form` map is allowed per operation because two maps cannot invert the same bare wire key. Declared properties use their declared decoder; only undeclared properties reach the dynamic fallback. Satisfiable dynamic array or object value schemas are outside style decoding; use JSON content for heterogeneous or nested values.

## Canonical wire examples

Every supported style cell has a representative wire-to-JSON form here:

| Shape | Wire | JSON |
|---|---|---|
| `form` string | `?q=red%20shoes` | `{"q":"red shoes"}` |
| exploded `form` array | `?tags=go&tags=red%2Cblue` | `{"tags":["go","red,blue"]}` |
| compact `form` array | `?ids=a%2Cb,c` | `{"ids":["a,b","c"]}` |
| `spaceDelimited` array | `?ids=10%2020%2030` | `{"ids":[10,20,30]}` |
| `pipeDelimited` array | `?flags=true%7Cfalse` | `{"flags":[true,false]}` |
| compact `form` object | `?point=lat,52.1,long,4.3` | `{"point":{"lat":52.1,"long":4.3}}` |
| exploded `form` object | `?lat=52.1&long=4.3` | `{"point":{"lat":52.1,"long":4.3}}` |
| `spaceDelimited` object | `?point=lat%2052.1%20long%204.3` | `{"point":{"lat":52.1,"long":4.3}}` |
| `pipeDelimited` object | `?point=lat%7C52.1%7Clong%7C4.3` | `{"point":{"lat":52.1,"long":4.3}}` |
| `deepObject` | `?filter%5Brole%5D=admin&filter%5Bactive%5D=true` | `{"filter":{"active":true,"role":"admin"}}` |

Comma delimiters are found before percent decoding, while pipe and space delimiters are found after one decode. Following [RFC 3986's single-pass decoding rule](https://www.rfc-editor.org/rfc/rfc3986.html#section-2.4), `%257C` therefore represents the literal text `%7C`, not another delimiter. Literal pipes in a `pipeDelimited` wire value and raw deep-object brackets are noncanonical and reject.

## Content parameters

Parameter `content` supports exactly one parsed `application/json` media type. Case variants and well-formed parameters such as `Application/JSON; charset=utf-8` are accepted; other media types, suffixes, ranges, and wildcards are outside this decoder.

The Media Type Object's `schema` is optional. Missing `schema` and explicit `schema: {}` both accept one strict JSON value of any kind:

```text
?q=null                         → {"q":null}
?q=true                         → {"q":true}
?q=1.25                         → {"q":1.25}
?q=%22value%22                  → {"q":"value"}
?q=%5B1%2Ctrue%5D               → {"q":[1,true]}
?q=%7B%22x%22%3A1%7D            → {"q":{"x":1}}
```

An absent optional parameter is omitted from the output object; `q=null` is present with the JSON null value. Exactly one occurrence and one complete JSON value are required. An explicit schema, including a local reference or `allOf`-only schema, runs ordinary final validation. A root or resolved explicit default can supply an absent optional query parameter and is then validated.

A Parameter must use exactly one of `schema` and `content`. With `content`, serialization fields `style`, `explode`, and `allowReserved` are forbidden, and parameter-level `example` or `examples` must move into the Media Type Object.

## Extensions and interoperability

OpenAPI 3.0.x leaves `deepObject` nesting and arrays undefined. Klopt supports exactly one object level and extends declared array properties by collecting repeated canonical child names:

```text
?filter%5Btag%5D=go&filter%5Btag%5D=api
→ {"filter":{"tag":["go","api"]}}
```

That repeated-array rule is a Klopt extension; clients must agree on it. Exact-owner precedence, the string fallback for untyped dynamic values, and the sole-open-form rule are also Klopt interoperability policies rather than OpenAPI inverse-decoding rules.

For schema/style query parameters, boolean `allowReserved` is accepted but does not change this consumer's decoding. URI-safe reserved characters can arrive literally and percent-encoded conflicting characters decode normally. Producers remain responsible for correct serialization.

## Rejections and errors

- **Parse:** unsupported style/type/explode cells, nested style-based values, or missing direct style types cannot be decoded deterministically. Use a supported matrix cell or `content: application/json`.
- **Parse:** two open exploded `form` maps share one bare-key namespace. Use namespaced `deepObject` parameters or JSON content.
- **Parse:** `schema`/`content` conflicts, forbidden content fields, unsupported content media, and explicit owner collisions have no unambiguous contract. Move media examples into the Media Type Object and select one supported representation.
- **Parse:** bracket-containing `deepObject` bases or declared children are not reversibly owned. Rename them or use JSON content.
- **Runtime:** malformed percent encoding or UTF-8, raw brackets or pipes, nested deep names, duplicate scalar/object/JSON occurrences, odd object tuples, invalid scalar text, absent required values, and failed final validation reject the request. Send the canonical wire form and a schema-valid value.

See the concise [Compatibility rejections](/klopt/openapi-compatibility/#rejections) and the official [OpenAPI 3.0.3 Parameter Object](https://spec.openapis.org/oas/v3.0.3.html#parameter-object) for the surrounding standard.
