# Klopt
Klopt is a Go library that validates raw JSON requests and decodes OpenAPI 3.0.x query parameters into validated JSON, with optional generated Go validation source and contract tests.

## Why Klopt

- **One contract.** Klopt validates requests at runtime and generates Go validation code plus contract tests from the same OpenAPI document.
- **Pragmatic scope.** Klopt implements a deliberate, useful OpenAPI subset instead of pretending to support everything.
- **Loud boundaries.** Unsupported or ambiguous behavior-bearing validation and serialization constructs return an actionable `validation.Parse` error instead of being guessed or silently ignored.

For example, these two otherwise valid parameters share one ambiguous bare-key namespace:

```yaml
parameters:
  - name: filter
    in: query
    style: form
    explode: true
    schema:
      type: object
      additionalProperties:
        type: string
  - name: options
    in: query
    style: form
    explode: true
    schema:
      type: object
      additionalProperties:
        type: string
```

For `?status=open`, both open maps can claim `status`. Klopt rejects the operation during `validation.Parse` instead of guessing; namespaced `deepObject` parameters or JSON `content` are unambiguous alternatives.

Klopt's differentiator is its boundary: request bodies are validated as raw JSON before Go unmarshalling can hide presence, `null`, or exact-number information, while query wire formats are decoded into validated JSON before callers choose Go types. Documented annotations can be intentionally inert, and documented permissive deviations, policies, and extensions are accepted only where explicitly labeled. Read the [Philosophy](https://djosh34.github.io/klopt/philosophy/) and [OpenAPI Compatibility](https://djosh34.github.io/klopt/openapi-compatibility/) guides for the full rationale and boundaries.

## Getting Started

Install the runtime package:

```sh
go get github.com/djosh34/klopt/pkg/validation
```

Runtime validation is one independent way to use Klopt:

```go
package example

import (
	"errors"
	"os"

	"github.com/djosh34/klopt/pkg/validation"
)

func validateCreateThing(body []byte) error {
	spec, err := os.ReadFile("openapi.yaml")
	if err != nil {
		return err
	}

	validations, _, err := validation.Parse(spec)
	if err != nil {
		return err
	}

	return errors.Join(validations["createThing"].Validate(body)...)
}
```

In-memory test-source generation is a separate alternative:

```go
package example

import (
	"github.com/djosh34/klopt/pkg/generate"
	"github.com/djosh34/klopt/pkg/validation"
)

func generateValidationSources(spec []byte) (map[string][]byte, error) {
	files, err := generate.GenerateInMemory(
		"generated",
		spec,
		validation.PatternOptions(),
	)
	if err != nil {
		return nil, err
	}

	return files, nil
}
```

`files` is a caller-owned `map[string][]byte` containing exactly `validate.go` and `validate_test.go`; each value is generated Go source. On error, the map is nil. See the documentation [Getting Started](https://djosh34.github.io/klopt/) guide for query decoding, [Architecture](https://djosh34.github.io/klopt/architecture/) for fuller source-byte handling, [Patterns](https://djosh34.github.io/klopt/patterns/) for pattern options and generated-test limits, and [OpenAPI Compatibility](https://djosh34.github.io/klopt/openapi-compatibility/) for supported boundaries.

## OpenAPI compatibility

Klopt accepts the OpenAPI 3.0 feature set and implements a focused subset of Schema Objects, JSON request bodies, query serialization, patterns, and local references. It does not claim complete OpenAPI support: unsupported composition, recursive or external references, unsupported query styles and ambiguous ownership, and unknown behavior-bearing Schema Object keywords fail during parsing. See the [compatibility reference](https://djosh34.github.io/klopt/openapi-compatibility/) for the observable contract.

## Contributing

Klopt is currently a greenfield project, and contributions are not yet accepted. Creating issues is welcome.

## License

All rights reserved.
