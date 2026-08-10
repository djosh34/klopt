# Delivery 1 reacceptance evidence

This document records the D1R9 acceptance audit and the exact focused campaigns run for #206. The root contract is #164 (ultimately #55); the repaired parents are #165, #169, #173, #178, #183, #188, #193, and #200.

## D1R1–D1R8 architecture audit

| Repair | Inspected private seam | Governing invariant and forbidden shapes | Result / localized correction |
|---|---|---|---|
| #165 (D1R1) | `documentProfileValidator`, clean OAS admission, and both pattern parsers | Complete document-wide exclusions precede compilation; reachable ordinary checks retain their scope; pattern nullability and one authored-pattern size accumulator agree; no post-compilation exclusion scan or shared production semantics | **Pass.** Exact admission/pointer conformance and clean-room guards passed; no correction required. |
| #169 (D1R2) | iterative YAML/strict-value frames, `cloneJSONValue`, exact-number lexical normalization, and shared URI syntax | No recursive admitted-value walk or depth cap; no one-zero-at-a-time arithmetic normalization; one URI/IP-literal scanner; occurrence-owned clones | **Pass.** Deep-value, number, URI, and ownership evidence passed; no correction required. |
| #173 (D1R3) | `evaluationRecords`, evaluation cache rebasing, enum-member metadata, and record consumers | One heterogeneous ordered record sequence is authoritative; no parallel semantic slices or string-projected identities; cached and uncached complete identities agree | **Pass.** Oracle identity/failure-closure evidence passed; no correction required. |
| #178 (D1R4) | `planBuilder`, requirement sets, valid schedule, fault descriptors, and order keys | Planning is declarative; no candidate evaluation, witness cap/corpus, fixed-width mask, or materialized candidate product; report and execution orders remain separate | **Pass.** Plan and additive-schedule evidence passed; no correction required. |
| #183 (D1R5) | `rowProjectionCursor`, array/object structural frontiers, and active projected members/items | One same-instance projection interpretation; no eager branch/size product or authored-capacity allocation; each structural assignment is charged immediately | **Pass after localized correction.** The beyond-`uint64` branch retains only a scalar `beyondArrayCursor`. It charges length once, advances child ranks to the genuine complete multi-source conjunction endpoint, and discards each attempt-local child before yielding; only natural exhaustion marks the projection unavailable. Retention, AST, and exact public-Build regressions lock the seam. |
| #188 (D1R6) | compact pattern machines, `basicStringProduct`, declarative format programs, and directed objective schedule | No counted-machine expansion, sample/post-filter approximation, hidden search cap, visited witness corpus, or coupling to production validation | **Pass.** Exact string-product/format evidence passed; no correction required. |
| #193 (D1R7) | fault product continuation, parent replay, array/object/scalar mutation machines, and closure comparison | One fair continuation; no first-parent commitment, rank-prefix replay, delayed batch charging, alias-preserving copies, or non-identity closure comparison | **Pass after localized correction.** Ranked array insertion is directly addressed, and each ranked combination is decoded once through an attempt-local scalar cursor with one monotonic advance per charged coordinate. Array derivatives grow one coordinate only after its charge. Oversized object/array attempts perform one ranked edit before yielding to the outer fault product. |
| #200 (D1R8) | callback lifetime, transitive dependency guard, ownership allowlist, generated-value and source-provenance analyses | Callback bytes are borrowed; local dependency closure stays clean; ownership is role-specific; no generated corpus or source-specific semantic switch survives | **Pass.** Callback, clean-room, ownership/no-corpus, and retention campaigns passed; no beyond-count generated row survives an attempt boundary. |

No blocking grand architectural failure was found in this audit.

## D1R9.2 integrated public-seam evidence

`pkg/schematest/build_integration_acceptance_test.go` records exact ordered `[]Case` and complete `Report` values for one unchanged crossed document combining repeated references, request direction, allOf composition, an exact numeric bound, directed pattern/format/length strings, and array/object child search. Its repeated sufficient run reaches `space_exhausted` with an empty uncovered set at exactly 700,917 steps; a separate 100-step run against the same document pins the exact cutoff stream and report. The file also covers genuinely open alternatives, beyond-`uint64` fairness and exhaustion, billion-count cutoffs, mutation-name collision, and supplementary public 65-branch **allOf** cutoff evidence. The arbitrary-width **anyOf** mask proof is the source/unit audit sanctioned by #208: `parentReplayMaskAtOrdinal` and `parentReplayMaskCursor` cover bit 64 and an ordinal beyond `uint64`, while source assertions forbid fixed-width narrowing. Private helpers are the intended behavioral seam because a public 65-branch anyOf exact report would require `2^65-1` canonical mask obligations.

## D1R9.3 campaigns

All commands below were run literally from the repository root on 2026-08-10. Multi-test regular expressions are fenced so their unescaped `|` operators remain executable alternation. The first pass exposed stale ownership allowlist rows in the ownership and R8 campaigns; those rows were corrected and both displayed commands were rerun literally to the final PASS outcomes recorded below. A `go test -list` audit also confirmed every displayed command selects at least one named test.

#### Runtime corpus / production verdict consumer

```sh
go test ./pkg/schematest -run '^TestCorpusRuntimeVerdictsMatchBuild$' -count=1
```

**Outcome:** PASS.

#### Generated alpha/zeta consumer

```sh
go test ./pkg/generate -run '^TestGenerateWritesCompiledValidation$' -count=1
```

**Outcome:** PASS.

#### Callback error and lifetime

```sh
go test ./pkg/schematest -run '^(TestBuildCallbackBytesRequireCallerCopyAndAreNotRetained|TestBuildReturnsCallbackErrors)$' -count=1
```

**Outcome:** PASS.

#### Transitive clean room and copied/source-specific guards

```sh
go test ./pkg/schematest -run '^(TestProductionImportsStayCleanRoom|TestCleanRoomDependencyGuardRejectsLocalBridge|TestGeneratedImportGuardRejectsCheckedInConsumer|TestCopiedAnswerGuardRejectsConcreteBypasses|TestCopiedSemanticGuard.*|TestOperationIDFlowGuardRejectsSourceSpecificBranching)$' -count=1
```

**Outcome:** PASS.

#### Ownership and no-corpus dataflow

```sh
go test ./pkg/schematest -run '^(TestExactLongLivedOwnershipShapes|TestGeneratedValuesDoNotEscapeBuildRuntime|TestStructuralFrontierRetainsNoProjectionCorpusOrLocalProduct)$' -count=1
```

**Outcome:** PASS.

#### Retention

```sh
go test ./pkg/schematest -run '^(TestBuildRetainedMemoryIsFlatWithEmittedCount|TestBeyondCountCursorRetainsNoGeneratedRowAcrossBudgets)$' -count=1
```

**Outcome:** PASS.

#### Deterministic complete stream

```sh
go test ./pkg/schematest -run '^TestBuildFullStreamDeterminism$' -count=1
```

**Outcome:** PASS.

#### Arbitrary-width anyOf source/unit audit

```sh
go test ./pkg/schematest -run '^(TestAnyOfReplayMaskUnitAuditReachesBeyondUint64|TestAnyOfReplayMasksHaveNoFixedWidthNarrowing|TestBuildSixtyFiveAllOfBranchesHasExactCutoffReport)$' -count=1
```

**Outcome:** PASS.

#### R1 admission/pointers

```sh
go test ./pkg/schematest -run '^(TestPublicAdmissionConformance|TestPublicDocumentProfileTraversalConformance|TestPublicStructuralAdmissionConformance|TestParseInputOpenAPIVersionProfile|TestParseInputRejectsExcludedSchemaCapabilitiesAtAuthoredPointers|TestMakePlanAcceptsSchemaNamesThatMatchCompositionKeywords)$' -count=1
```

**Outcome:** PASS.

#### R2 strict values/numbers/annotations

```sh
go test ./pkg/schematest -run '^(TestDeepJSONValueOperationsUseHeapBackedTraversal|TestDeepJSONBuildFinishesWithoutProcessStackFailure|TestDeepYAMLBuildFinishesWithoutProcessStackFailure|TestExactNumber.*|TestValidateURIReference.*|TestURIComponentAndFragmentScanningShareExactPercentTriplets|TestParseInputAcceptsAndDiscardsWellFormedInertMetadata|TestParseInputShapeChecksInertMetadata|TestInertSchemaMetadataDoesNotChangeAuthoritativeRecords)$' -count=1
```

**Outcome:** PASS.

#### R3 authoritative oracle identities

```sh
go test ./pkg/schematest -run '^(TestEvaluationCache.*|TestEvaluationRecordsCacheHitKeepsEveryFact|TestEvaluateReferenceCacheRebasesCompleteRecords|TestCleanOracleFailureIdentitiesDistinguishRuleAndBranchOccurrences)$' -count=1
```

**Outcome:** PASS.

#### R4 declarative obligations/additive schedule

```sh
go test ./pkg/schematest -run '^(TestMakePlan.*|TestPlannerCallGraphIsDeclarative|TestBuildStreamsDeterministicValidPrimitiveRows)$' -count=1
```

**Outcome:** PASS.

#### R5 lazy structural search

```sh
go test ./pkg/schematest -run '^(TestBuildBeyondUint64ArrayYieldsToLaterProjection|TestBuildBeyondCountSkipsUnusableFirstChildRank|TestBuildUnavailableBeyondCountChildYieldsNormally|TestBeyondArrayCursorChargesLengthBeforeChildren|TestBeyondArrayCursorAdvancesPastUnusableFirstChildRank|TestSharedArrayFrontierBuildsAllOfItems|TestArrayFrontierHasNoSyntheticCutoffLoop)$' -count=1
```

**Outcome:** PASS.

#### R6 compact exact string search

```sh
go test ./pkg/schematest -run '^(TestSimpleStringFormatWitnessesAreCanonicalAndDeterministic|TestBasicStringProductTransitionsAreDeterministic)$' -count=1
```

**Outcome:** PASS.

#### R7 isolated replay/closures

```sh
go test ./pkg/schematest -run '^(TestNonCompositionFaultFamiliesHaveExactClosures|TestArrayCountFaultCutoffPrecedesEachAtomicAssignment|TestArrayInsertionFaultCutoffPrecedesEveryCoordinateMutation|TestArrayInsertionRanksAddressWithoutPrefixReplay|TestArrayCombinationCoordinatesMatchCanonicalTuples|TestOversizedArrayFaultRanksYieldDeterministically|TestOversizedObjectFaultRanksYieldDeterministically)$' -count=1
```

**Outcome:** PASS.

#### R8 callback/clean-room/ownership

```sh
go test ./pkg/schematest -run '^(TestBuildCallbackBytesRequireCallerCopyAndAreNotRetained|TestProductionImportsStayCleanRoom|TestExactLongLivedOwnershipShapes|TestCopiedAnswerGuardRejectsConcreteBypasses|TestCopiedSemanticGuard.*|TestOperationIDFlowGuardRejectsSourceSpecificBranching)$' -count=1
```

**Outcome:** PASS.

#### Integrated crossed public Build document

```sh
go test ./pkg/schematest -run '^TestBuildGenuinelyCompoundMatrixIsExact$' -count=1
```

**Outcome:** PASS.

### Real-Build stress

Command:

```text
go test ./pkg/schematest -run '^$' -bench '^BenchmarkBuildStress$' -benchtime=1x -count=1 -benchmem
```

Measured outcomes (descriptive evidence only; no timing SLA):

| Benchmark | Cases | Steps | Stop | Go alloc bytes/op | Allocations/op | Measured allocation | Retained |
|---|---:|---:|---|---:|---:|---:|---:|
| `deep_wide` | 2 | 100,000 | `max_steps_reached` | 3,317,119,448 | 70,168,927 | 3,317,135,592 B | 8,602,968 B |
| `composition_15` | 16 | 100,000 | `max_steps_reached` | 179,803,724,856 | 3,664,793,993 | 179,803,781,336 B | 3,615,645,360 B |

The benchmark command passed.

## Final repository commands

The resolution completion phase recorded exact-stream line-length and source-guard findings together, corrected them as one batch, and reran the complete boundary. The final boundary passed:

| Command | Outcome |
|---|---|
| `make fmt` | PASS; `gofumpt -w .` plus `golangci-lint run --fix`, 0 issues. |
| `make lint` | PASS; 0 issues. |
| `make test` | PASS; `go test ./...`, including `pkg/schematest` in 145.601s. |
| `git diff --check` | PASS. |

`.golangci.yml` is unchanged.
