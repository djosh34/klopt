---
title: Roadmap
description: Remaining validation, composition, generation, and portability work.
---

## Validation

- Expand the supported Schema Object keyword set only where runtime behavior and generated-test evidence can stay aligned.
- Add enforcement for more useful string formats where the semantics are stable enough to document and test.
- Broaden request media handling beyond compile-time JSON selection without making runtime content selection ambiguous.

## Composition and references

- Add `oneOf`, `anyOf`, and `not` with validation and useful generated cases, rather than validation-only partial support.
- Explore safe recursive-schema and external-reference support with bounded compilation and clear source identity.

## Query decoding

- Evaluate additional query serialization shapes and media types where inverse ownership and canonical wire forms can be defined.
- Improve interoperability research around behaviors OpenAPI leaves undefined, especially nested objects and heterogeneous maps.

## Generation

- Extend string construction beyond ASCII and cover more raw Go regexp capabilities without changing production matching semantics.
- Construct accepted values for enforced formats without requiring finite enums or trusted local evidence.
- Improve contradictory-language diagnostics, case quality, shrinking, and bounded construction for complex composed schemas.

## Portability

- Continue cross-validator comparisons as the supported subset grows, recording deliberate policy where OpenAPI or JSON Schema permits divergent implementations.
