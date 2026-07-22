# klopt

Klopt is a Go library and code generator that decodes and validates HTTP requests according to an OpenAPI 3.0.x document.

“Klopt” is Dutch for “is correct,” reflecting the library's focus on validation. The name is inspired by the naming of Google's code search engine Zoekt, Dutch for “searches.”

Read the [documentation](https://djosh34.github.io/klopt/) for the model, query decoding, and design rationale.

```sh
go get github.com/djosh34/klopt/pkg/validation
```

## Getting started

See this operation snippet:

```yaml
post:
  operationId: createThing
  requestBody:
    required: true
    content:
      application/json:
        schema:
          type: object
          required: [name]
          properties:
            name:
              type: string
            # ...
  parameters:
    - name: filter
      in: query
      required: true
      style: deepObject
      explode: true
      schema:
        type: object
        required: [status]
        additionalProperties: false
        properties:
          status:
            type: string
          limit:
            type: integer
          # ...
```

Use your own Go types for the request data:

```go
type CreateThing struct {
	Name string `json:"name"`
}

type CreateThingQuery struct {
	Filter struct {
		Status string `json:"status"`
		Limit  int    `json:"limit"`
	} `json:"filter"`
}
```

Parse the OpenAPI document once at startup, then use the matching request validation and query decoder for each request. The decoder interprets the OpenAPI query serialization, validates the resulting JSON, and returns it only after validation succeeds.

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/djosh34/klopt/pkg/validation"
)

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

Use `GenerateInMemory` when you want to parse the specification ahead of time:

```go
generatedFiles, err := generate.GenerateInMemory("openapivalidation", spec, validation.PatternOptions())
if err != nil {
	return err
}
// generatedFiles is a map containing all needed generated files.
```

The generated source is caller-owned. Generated maps are package-private. Generated tests cover JSON request bodies only.

## Contributions and license

Contributions are not accepted. This repository has no open-source license; all rights are reserved.
