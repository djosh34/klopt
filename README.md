# klopt

Klopt is a Go library and code generator that decodes and validates HTTP requests.

> [!NOTE]
> “Klopt” is Dutch for “is correct,” reflecting the library's focus on validation. The name is inspired by the naming of Google's code search engine zoekt, Dutch for “searches.”

Read the [documentation](https://djosh34.github.io/klopt/) for the model, query decoding, and design rationale.

## Getting started

Install the runtime package:

```sh
go get github.com/djosh34/klopt/pkg/validation
```

Given an OpenAPI operation like this:

```yaml
post:
  operationId: createThing
  requestBody:
    required: true
    # ...
  parameters:
    - name: filter
      in: query
      # ...
```

The `operationId` connects the compiled validation and query decoder to your handler. With an object schema for `filter`, this URL:

```text
/things?status=active
```

is decoded and validated as:

```json
{"filter":{"status":"active"}}
```

Keep request data in your own Go types. The query result is ordinary JSON, so an inline nested struct and JSON tags work as expected:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/djosh34/klopt/pkg/generate"
	"github.com/djosh34/klopt/pkg/validation"
)

type CreateThing struct {
	Name string `json:"name"`
}

type CreateThingQuery struct {
	Filter struct {
		Status string `json:"status"`
	} `json:"filter"`
}
```

Parse the OpenAPI document once at startup, then reuse the matching request validation and query decoder for every request. Validate the raw body before unmarshalling it. The query decoder interprets the OpenAPI wire format and returns JSON only after validation succeeds.

```go
func newCreateThingDecoder() (func(*http.Request) (CreateThing, CreateThingQuery, error), error) {
	spec, err := os.ReadFile("openapi.yaml")
	if err != nil {
		return nil, err
	}

	// Parse once at startup.
	validations, queryDecoders, err := validation.Parse(spec)
	if err != nil {
		return nil, err
	}

	requestValidation, ok := validations["createThing"]
	if !ok {
		return nil, fmt.Errorf("missing createThing validation")
	}
	queryDecoder, ok := queryDecoders["createThing"]
	if !ok {
		return nil, fmt.Errorf("missing createThing query decoder")
	}

	return func(r *http.Request) (CreateThing, CreateThingQuery, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return CreateThing{}, CreateThingQuery{}, err
		}

		// Validate the raw body first.
		if err := errors.Join(requestValidation.Validate(body)...); err != nil {
			return CreateThing{}, CreateThingQuery{}, err
		}

		var input CreateThing
		if err := json.Unmarshal(body, &input); err != nil {
			return CreateThing{}, CreateThingQuery{}, err
		}

		// Decode query syntax and validate its JSON.
		rawQuery, err := queryDecoder.Decode(r.URL)
		if err != nil {
			return CreateThing{}, CreateThingQuery{}, err
		}

		var query CreateThingQuery
		if err := json.Unmarshal(rawQuery, &query); err != nil {
			return CreateThing{}, CreateThingQuery{}, err
		}

		return input, query, nil
	}, nil
}
```

## Generate compiled data

Runtime parsing is useful while developing. When you want to parse the specification ahead of time, use `GenerateInMemory`:

```go
generatedFiles, err := generate.GenerateInMemory("openapivalidation", spec, validation.PatternOptions())
if err != nil {
	return err
}
```

The returned map contains all needed generated files. The source is caller-owned, generated maps are package-private, and generated tests cover JSON request bodies only.

## Test generation

Klopt undergoes extensive fuzz testing using its own [JSON test generator](https://djosh34.github.io/klopt/architecture/#test-generation).

## Roadmap

- [ ] Add proper format support for Int32 (`int32`), Int64 (`int64`), `float`, `double`, UUID, CIDR, IPv4, and possibly more, including the required test-generation additions.
- [ ] Add path decoding analogous to query decoding for both the Go standard library (`net/http`) and Gin.
- [ ] Continue improving test generation.
- [ ] Broaden OpenAPI support with `anyOf` and perhaps `oneOf` and `not`.

# Contributing

klopt is currently a greenfield project, and contributions are not yet accepted. Creating issues is welcome.

# License

All rights reserved.
