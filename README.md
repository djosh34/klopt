# klopt

Klopt is a Go library and code generator for validating OpenAPI 3.0.x JSON request bodies and decoding OpenAPI query parameters into JSON.

```sh
go get github.com/djosh34/klopt/pkg/validation
```

## Use at runtime

Parse the OpenAPI document once, normally when the process starts. The returned maps are keyed by `operationId`. Invalid documents and unsupported OpenAPI behavior return a parse error.

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

Validate the original request bytes before decoding them into an application type:

```go
body, err := io.ReadAll(r.Body)
if err != nil {
	return err
}

requestValidation, ok := validations["createThing"]
if !ok {
	return fmt.Errorf("missing createThing validation")
}
if err := errors.Join(requestValidation.Validate(body)...); err != nil {
	return err
}

var input CreateThing
if err := json.Unmarshal(body, &input); err != nil {
	return err
}
```

Use the decoder from that same parsed result for a query. It applies the OpenAPI query serialization rules, validates the result, and returns JSON for your own Go type:

```go
queryDecoder, ok := queryDecoders["listThings"]
if !ok {
	return fmt.Errorf("missing listThings query decoder")
}

rawQuery, err := queryDecoder.Decode(r.URL)
if err != nil {
	return err
}

var query ListThingsQuery
if err := json.Unmarshal(rawQuery, &query); err != nil {
	return err
}
```

## Generate compiled data

`pkg/generate` writes the parsed validation and query-decoder data as `validate.go`; it does not generate application structs or unmarshalling code. Use it when you want to avoid parsing the specification at runtime:

```go
err := generate.Generate(
	"internal/openapivalidation",
	"openapivalidation",
	spec,
	validation.PatternOptions(),
)
if err != nil {
	return err
}
```

`generate.GenerateInMemory` returns the same `validate.go` and `validate_test.go` files as `map[string][]byte` when the caller owns writing them.

The generated test file uses Rapid property tests to check JSON request-body validation against generated accepted and rejected cases. It does not generate query-decoder tests.

## Documentation

Read the [documentation](https://djosh34.github.io/klopt/) for query decoding, the supported model, and design rationale. Package APIs are on [pkg.go.dev](https://pkg.go.dev/github.com/djosh34/klopt/pkg/validation) and [the generator package](https://pkg.go.dev/github.com/djosh34/klopt/pkg/generate).

## Contributions and license

Contributions are not accepted. This repository has no open-source license; all rights are reserved.

## Name

“Klopt” is Dutch for “is correct.” The name is inspired by [Zoekt](https://github.com/sourcegraph/zoekt), Dutch for “searches.”
