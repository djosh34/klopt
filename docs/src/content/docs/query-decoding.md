---
title: Query decoding
description: Convert OpenAPI 3.0 query parameter encodings into validated JSON.
---

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
raw, err := requestValidations["listThings"].Query.Decode(r.URL)
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
