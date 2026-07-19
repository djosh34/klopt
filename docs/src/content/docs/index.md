---
title: Getting Started
description: Install Klopt and choose runtime request handling or in-memory Go source generation.
---

Klopt requires Go 1.26.4 or newer and an OpenAPI 3.0.x document. Every operation Klopt compiles must have a non-empty `operationId`. Install the runtime package with:

```sh
go get github.com/djosh34/klopt/pkg/validation
```

Choose runtime handling or in-memory generation. They are independent alternatives, not stages in one required pipeline. The reasons for that split are covered in [Philosophy](/klopt/philosophy/) and the boundaries are listed in [OpenAPI Compatibility](/klopt/openapi-compatibility/).

## Runtime validation and query decoding

```go
package example

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"

	"github.com/djosh34/klopt/pkg/validation"
)

func useRuntime(body []byte, requestURL *url.URL) error {
	spec, err := os.ReadFile("openapi.yaml")
	if err != nil {
		return err
	}

	validations, queryDecoders, err := validation.Parse(spec)
	if err != nil {
		return err
	}

	if err := errors.Join(validations["createThing"].Validate(body)...); err != nil {
		return err
	}

	queryJSON, err := queryDecoders["listItems"].Decode(requestURL)
	if err != nil {
		return err
	}

	var query any
	return json.Unmarshal(queryJSON, &query)
}
```

Both maps are keyed by `operationId`; this example uses the body operation `createThing` and the query operation `listItems`. Parse once at startup, reuse the immutable compiled values, and do not mutate them. Empty body bytes mean absence; JSON `null` is present. Query decoding returns validated JSON so the caller retains control of its Go types. See [Query Decoding](/klopt/query-decoding/) for wire styles and [Patterns](/klopt/patterns/) before choosing pattern options.

## In-memory source generation

```go
package example

import (
	"github.com/djosh34/klopt/pkg/generate"
	"github.com/djosh34/klopt/pkg/validation"
)

func generateSources(spec []byte) (map[string][]byte, error) {
	files, err := generate.GenerateInMemory(
		"api", spec, validation.PatternOptions(),
	)
	if err != nil {
		return nil, err
	}

	return files, nil
}
```

Generation parses the OpenAPI document internally. On success it returns a caller-owned `map[string][]byte` with exactly `validate.go` and `validate_test.go`; each value is complete generated Go source, and the caller decides whether to write, inspect, transform, or embed it. On error the map is nil. Generation success and generated-test construction are separate boundaries; [Architecture](/klopt/architecture/) explains why a later generated `TestValidations` can still fail.
