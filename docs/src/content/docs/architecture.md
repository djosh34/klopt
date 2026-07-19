---
title: Architecture
description: The boundaries between OpenAPI compilation, request handling, generated source, and generated evidence.
---

## Request pipeline boundaries

Klopt keeps five boundaries separate:

1. **OpenAPI parsing and compilation.** The document version, reachable local references, selected request media, query parameter serialization, and supported Schema Objects are checked. Unsupported behavior fails here instead of reaching a request.
2. **Request-body validation.** A selected JSON body remains raw while strict JSON parsing, presence, type, exact-number, collection, object, composition, pattern, format, and request-direction rules run.
3. **Query wire decoding.** `URL.RawQuery` is claimed by compiled parameters before percent decoding can erase delimiter information. Declared values become JSON scalars, arrays, or objects.
4. **Final query schema validation.** The whole decoded query object is validated, including defaults and dynamic properties. Successful decoding therefore returns ordinary validated JSON.
5. **Generated source and tests.** Generation renders the same compiled runtime model plus a `TestValidations` suite. Generated production source contains validation data and uses the same validator rather than reimplementing every rule.

The [Query Decoding](/klopt/query-decoding/) and [Patterns](/klopt/patterns/) guides describe the two boundaries with additional policies.

## Runtime Parse and source generation

`validation.Parse` is the runtime constructor. It returns request-body validations and query decoders keyed by `operationId`; applications can compile once at startup and reuse them.

`generate.GenerateInMemory` is an alternative deployment boundary. It invokes parsing internally, verifies that request-body operation IDs can name generated Go state, renders and formats source, then returns `validate.go` and `validate_test.go` as bytes. The generated validation definitions restore the same runtime behavior without parsing the OpenAPI document in production. The caller owns publication of the returned bytes.

## Generated-test evidence

Generated tests construct accepted and rejected JSON around the effective schema. The optional `x-valid-examples` and `x-invalid-examples` extensions provide trusted local evidence for pattern or format occurrences that construction cannot otherwise prove. Evidence belongs to the whole Schema Object occurrence. An invalid example proves that the occurrence rejects that value; it does not prove that one isolated keyword alone caused the rejection.

This provenance matters across `allOf`. Production validation applies every branch to the same value, while test generation intersects their constructive domains and retains the source of each obligation. Finite enums and locally usable evidence can provide concrete values for otherwise opaque string constraints.

## Generation success is not test success

Successful source generation means parsing and rendering succeeded. It does not guarantee that the generated `TestValidations` will pass when `go test` runs.

Test construction has capabilities and budgets of its own. A valid production language can be outside the ASCII-only constructor, a raw Go regexp can use syntax the constructor cannot model, or construction can exceed its graph and search budgets. Contradictory constraints can describe an empty language with no accepted value. Declared string formats are opaque construction constraints: a finite enum or usable local trusted evidence may supply an accepted value, but unusable evidence can still produce an actionable generated-test failure.

These are generated-test execution failures, not changes to production validation and not reasons for `GenerateInMemory` to return an error. Keeping that boundary explicit avoids turning a constructor limitation into a false OpenAPI rejection.
