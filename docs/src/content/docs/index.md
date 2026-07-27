---
title: Getting started
description: Validate OpenAPI 3.0.x JSON request bodies and decode path and query parameters into JSON.
---

:::caution[Work in progress]
These docs are a work in progress and may not yet be complete or fully up to date with the code.
:::

`pkg/validation` compiles an OpenAPI 3.0.x document once. Use the result to validate raw JSON request bodies and decode path and query parameters into validated JSON.

## Install

```sh
go get github.com/djosh34/klopt/pkg/validation
```

## Parse once

```go
spec, err := os.ReadFile("openapi.yaml")
if err != nil {
	return err
}

requestValidations, err := validation.Parse(spec)
if err != nil {
	return err
}
```

The map is keyed by exact, case-sensitive OpenAPI `operationId`. Every operation has one `RequestValidation` containing its optional `Body`, `Query`, and `Path` components. Parse at startup, then reuse the compiled values. Do not mutate them after parsing.

## Validate a request body

```go
func validateCreateThing(r *http.Request, requestValidation *validation.Validation) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return errors.Join(requestValidation.Validate(body)...)
}
```

Call it with `requestValidations["createThing"].Body`. Empty bytes mean the body is absent. JSON `null` is a present body and follows the schema's `type` and `nullable` rules.

## Decode a path

```go
type GetThingPath struct {
	ThingID int `json:"thingID"`
}

func decodeGetThingPath(r *http.Request, decoder *validation.PathDecoder) (GetThingPath, error) {
	raw, err := decoder.DecodePathParams(r.URL)
	if err != nil {
		return GetThingPath{}, err
	}

	var path GetThingPath
	if err := json.Unmarshal(raw, &path); err != nil {
		return GetThingPath{}, err
	}

	return path, nil
}
```

Call it with `requestValidations["getThing"].Path`. The URL must be operation-relative: remove any effective OpenAPI server URL path prefix before calling the decoder. The decoder matches the selected operation's exact path template; it does not route requests or resolve servers.

## Decode a query

```go
type ListThingsQuery struct {
	Tags   []string `json:"tags"`
	Limit  int      `json:"limit"`
}

func decodeListThings(r *http.Request, decoder *validation.QueryDecoder) (ListThingsQuery, error) {
	raw, err := decoder.Decode(r.URL)
	if err != nil {
		return ListThingsQuery{}, err
	}

	var query ListThingsQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return ListThingsQuery{}, err
	}

	return query, nil
}
```

Call it with `requestValidations["listThings"].Query`. The decoder handles the OpenAPI wire format and returns ordinary JSON, leaving the final Go type under your control.

Next: [why validation happens before unmarshalling](/klopt/philosophy/), [how path decoding works](/klopt/path-decoding/), [how query decoding works](/klopt/query-decoding/), and [the architecture](/klopt/architecture/).
