---
title: Getting started
description: Validate OpenAPI 3.0.3 JSON request bodies and decode query parameters into JSON.
---

`pkg/validation` compiles an OpenAPI 3.0.3 document once. Use the result to validate raw JSON request bodies and decode query parameters into validated JSON.

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

validations, queryDecoders, err := validation.Parse(spec)
if err != nil {
	return err
}
```

Both maps are keyed by OpenAPI `operationId`. Parse at startup, then reuse the compiled values. Do not mutate them after parsing.

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

Call it with `validations["createThing"]`. Empty bytes mean the body is absent. JSON `null` is a present body and follows the schema's `type` and `nullable` rules.

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

Call it with `queryDecoders["listThings"]`. The decoder handles the OpenAPI wire format and returns ordinary JSON, leaving the final Go type under your control.

Next: [why validation happens before unmarshalling](/klopt/philosophy/), [how query decoding works](/klopt/query-decoding/), and [the architecture](/klopt/architecture/).
