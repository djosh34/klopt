---
title: Path decoding
description: Convert one selected OpenAPI 3.0 path template into validated JSON.
---

`PathDecoder` decodes path parameters after your router has selected an exact `operationId`. It is not a router.

Given this operation:

```yaml
/files/{name}.{extension}:
  get:
    operationId: getFile
    parameters:
      - {name: name, in: path, required: true, schema: {type: string}}
      - {name: extension, in: path, required: true, schema: {type: string}}
```

this operation-relative URL:

```text
/files/report%2F2026.json
```

decodes to:

```json
{"name":"report/2026","extension":"json"}
```

Use the decoder from the operation's request validation:

```go
raw, err := requestValidations["getFile"].Path.DecodePathParams(operationURL)
if err != nil {
	return err
}
```

The decoder matches `URL.EscapedPath()` segment by segment. Raw `/` separates segments; percent-encoded `%2F` remains parameter data. Values are percent-decoded exactly once, converted according to the parameter schema or JSON content definition, assembled into one JSON object, and validated.

## Server path prefixes

Pass an operation-relative URL. If the effective OpenAPI server URL path is `/api/v1`, the Paths key is `/pets/{id}`, and the request path is `/api/v1/pets/42`, remove `/api/v1` before calling `DecodePathParams`. Server selection, variable expansion, and prefix removal belong to the caller or router.

Path parameters support OpenAPI `simple`, `label`, and `matrix` styles plus one `application/json` content entry. Unsupported declarations fail while parsing the OpenAPI document, before request handling begins.
