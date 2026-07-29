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
    - name: thingID
      in: path
      required: true
      # ...
    - name: filter
      in: query
      # ...
```

The `operationId` connects one compiled request validation to your handler. With path parameter `thingID` and an object schema for `filter`, this URL:

```text
/things/42?status=active
```

is decoded and validated as:

```text
path:  {"thingID":42}
query: {"filter":{"status":"active"}}
```

Keep request data in your own Go types. Parameter results are ordinary JSON, so nested structs and JSON tags work as expected:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type CreateThingPath struct {
	ThingID int `json:"thingID"`
}
```

Parse the OpenAPI document once at startup, then reuse the matching body validation and parameter decoders for every request. Validate the raw body before unmarshalling it. Path and query decoders interpret the OpenAPI wire formats and return JSON only after validation succeeds.

```go
func newCreateThingDecoder() (
	func(*http.Request, *url.URL) (CreateThing, CreateThingQuery, CreateThingPath, error),
	error,
) {
	spec, err := os.ReadFile("openapi.yaml")
	if err != nil {
		return nil, err
	}

	// Parse once at startup.
	requestValidations, err := validation.Parse(spec)
	if err != nil {
		return nil, err
	}

	requestValidation, ok := requestValidations["createThing"]
	if !ok {
		return nil, fmt.Errorf("missing createThing validation")
	}
	if requestValidation.Body == nil || requestValidation.Query == nil || requestValidation.Path == nil {
		return nil, fmt.Errorf("incomplete createThing validation")
	}

	return func(r *http.Request, operationURL *url.URL) (CreateThing, CreateThingQuery, CreateThingPath, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return CreateThing{}, CreateThingQuery{}, CreateThingPath{}, err
		}

		// Validate the raw body first.
		if err := errors.Join(requestValidation.Body.Validate(body)...); err != nil {
			return CreateThing{}, CreateThingQuery{}, CreateThingPath{}, err
		}

		var input CreateThing
		if err := json.Unmarshal(body, &input); err != nil {
			return CreateThing{}, CreateThingQuery{}, CreateThingPath{}, err
		}

		// Decode query syntax and validate its JSON.
		rawQuery, err := requestValidation.Query.Decode(r.URL)
		if err != nil {
			return CreateThing{}, CreateThingQuery{}, CreateThingPath{}, err
		}

		var query CreateThingQuery
		if err := json.Unmarshal(rawQuery, &query); err != nil {
			return CreateThing{}, CreateThingQuery{}, CreateThingPath{}, err
		}

		// operationURL is supplied by the router after removing any effective server path prefix.
		rawPath, err := requestValidation.Path.DecodePathParams(operationURL)
		if err != nil {
			return CreateThing{}, CreateThingQuery{}, CreateThingPath{}, err
		}

		var path CreateThingPath
		if err := json.Unmarshal(rawPath, &path); err != nil {
			return CreateThing{}, CreateThingQuery{}, CreateThingPath{}, err
		}

		return input, query, path, nil
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

The returned map contains all needed generated files. The source is caller-owned, generated packages export one `RequestValidations` map, and generated tests cover JSON request bodies.

## Test generation

Klopt undergoes extensive fuzz testing using its own [JSON test generator](https://djosh34.github.io/klopt/architecture/#test-generation).

## Roadmap

- [ ] Add proper format support for Int32 (`int32`), Int64 (`int64`), `float`, `double`, UUID, CIDR, IPv4, and possibly more, including the required test-generation additions.
- [ ] Continue improving test generation.
- [ ] Broaden OpenAPI support with `oneOf` and `not`.

# Contributing

klopt is currently a greenfield project, and contributions are not yet accepted. Creating issues is welcome.

# License

All rights reserved.
