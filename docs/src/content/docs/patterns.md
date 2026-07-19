---
title: Patterns
description: Pattern dialects, matching semantics, resource limits, and generated-value boundaries.
---

Klopt's default mode implements a closed, portable subset of the ECMAScript 5.1 regular-expression dialect named by OpenAPI 3.0.x. Pattern matching uses search semantics: `cat` matches `scatter`; use `^cat$` when the whole string must match.

## Modes

Default mode supports literals, alternation, groups, character classes, ES5.1 escapes, anchors, word boundaries, greedy and lazy quantifiers, and leading positive or negative lookaheads after `^`. The closed grammar makes accepted syntax portable and translates ES string behavior, including UTF-16 code units, into validated Go regex checks.

`patternvalidator.UseRE2` instead passes the source to Go's `regexp` engine with raw Go syntax and semantics. It is useful when the contract is intentionally Go-specific, but it is not an ECMAScript portability mode. For example, Go flags can be accepted while ECMAScript-only lookaheads are not available.

`patternvalidator.RejectNonASCII` can be composed with either mode when the application wants an explicit ASCII-only pattern and subject policy. Pass options through `validation.PatternOptions(...)` to runtime parsing or generation.

## Limits

Both modes cap each pattern source at 64 KiB. `UseRE2` bypasses the ECMAScript parser and its default-only limits, but it does not bypass that source cap.

Default mode additionally limits nesting to 100 levels, the syntax tree to 10,000 nodes, leading lookaheads to 64, counted-repeat endpoints to 1,000, the cumulative nested-repeat product to 1,000, and each translated Go regexp check to 1 MiB. These bounds turn hostile or accidentally explosive patterns into parsing errors rather than unbounded work.

## Generated values and trusted evidence

Generated request-body tests construct ASCII-only string values. This is narrower than production validation. A default pattern such as `^é$` can validate Unicode correctly at runtime yet have a valid non-ASCII-only language that the constructor cannot populate. Trusted examples cannot rescue that language because generated values remain ASCII-only.

That boundary differs from a contradictory empty language. Two requirements such as `^a$` and `^b$` can leave no accepted value at all. In either case, source generation can succeed and the later generated `TestValidations` can report the construction problem when `go test` runs.

`x-valid-examples` and `x-invalid-examples` are trusted evidence attached to a Schema Object that directly declares `pattern` or `format`. Evidence describes the whole schema occurrence, not isolated keyword blame. Finite enums or compatible local valid evidence can provide accepted values where the constructor otherwise lacks one.

Declared string formats are opaque to generated request-body value construction even when production validation enforces them. A finite enum or usable local trusted evidence may provide values for `byte`, `date`, `date-time`, or `email`; missing or unusable evidence can surface as a generated-test failure without making `GenerateInMemory` fail.

## Rejections and limits

- Default mode rejects malformed ES5.1 syntax, backreferences, lookbehind, named groups, inline modes, foreign dialect constructs, and lookaheads outside the supported leading position during `validation.Parse`. Rewrite into the portable subset or deliberately select `UseRE2` when Go `regexp` supports the needed behavior.
- `UseRE2` rejects syntax unsupported by Go `regexp`; selecting it is not a way to enable backreferences or lookarounds.
- Either mode rejects invalid UTF-8, nil options, or pattern source beyond 64 KiB. Default mode also rejects its complexity limits; simplify or split the schema constraints.
- Generated tests can fail for unsupported construction capability, an empty or non-ASCII-only accepted language, opaque formats without usable values, or construction budgets. This happens when generated tests run, not during production validation or source rendering.

See [Architecture](/klopt/architecture/) for evidence provenance and exact failure timing.
