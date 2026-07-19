---
title: Philosophy
description: Why Klopt keeps request evidence intact and makes its supported OpenAPI boundary explicit.
---

## Why another validation library?

Klopt serves Go applications that want one OpenAPI request contract to drive runtime validation, query decoding, generated validation data, and generated contract tests. Its differentiator is not the size of its keyword catalog. It is the decision to preserve request evidence until the contract has been checked, and to reject unsupported behavior loudly instead of approximating it.

The result is deliberately aimed at teams that value explainable failures and a narrow, testable contract more than broad best-effort acceptance. [Architecture](/klopt/architecture/) shows where those boundaries sit; [OpenAPI Compatibility](/klopt/openapi-compatibility/) records the exact supported subset.

## Raw JSON before Go values

The request bodies `{}` and `{"name":null}` are observably different to OpenAPI: one omits `name`, while the other supplies it as `null`. A Go field such as `Name *string` becomes nil in both cases after ordinary unmarshalling. Requiredness and nullability can no longer be decided from that field alone.

Numbers have a similar problem. Generic decoding commonly turns JSON numbers into `float64`, which cannot preserve every valid JSON integer exactly. Klopt validates `json.RawMessage` instead. Presence, `null`, duplicate object names, and exact decimal values remain observable while schema rules run, before the application chooses a Go representation.

## Query wire data before Go types

Query strings are not typed JSON. OpenAPI styles assign meaning to repeated names, commas, spaces, pipes, bracketed names, and percent encoding. Decoding into a convenient generic form too early can erase the distinction between a delimiter and encoded data.

Klopt reads the raw query, applies the documented OpenAPI style and Klopt ownership policies, converts declared scalar types, builds JSON, and validates the completed object. Only then does the caller choose a struct or another Go type. This introduces an extra JSON encode/decode step for callers that want structs, but it keeps one familiar output boundary and avoids spreading serialization rules through application models.

## Deliberate scope, dependable behavior

OpenAPI is larger than Klopt's request-focused contract. Unsupported behavior-bearing Schema Object or query serialization constructs fail during parsing with their source context. That is a reliability choice: accepting a description while silently dropping a rule would make successful parsing misleading.

This does not mean every accepted field changes validation. OpenAPI contains annotations, and Klopt also documents a small number of permissive deviations, deterministic query policies, and interoperability extensions. Those are accepted only when their inert or extended behavior is explicit in [OpenAPI Compatibility](/klopt/openapi-compatibility/). The promise is observable behavior, not a claim that every valid OpenAPI document is accepted.
