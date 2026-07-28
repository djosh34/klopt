---
title: Roadmap
description: Known hard problems and likely next steps for validation test generation.
---

## Test generation

- Correctly intersect string patterns across `allOf`.

## Composition

- Potentially support `anyOf`, `oneOf`, and `not`.
- Validation is easier than generating useful random JSON for these branches. Test generation is the limiting problem.

## Ongoing quality

- Improve JSON value generation and shrinking.
- Expand schema mutation and cross-validator coverage as new behavior lands.
