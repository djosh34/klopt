# Delivery 1 reacceptance evidence

This document records the D1R9 acceptance audit and the exact focused campaigns run for #206. The root contract is #164 (ultimately #55); the repaired parents are #165, #169, #173, #178, #183, #188, #193, and #200.

## D1R1–D1R8 architecture audit

| Repair | Inspected private seam | Governing invariant and forbidden shapes | Result / localized correction |
|---|---|---|---|
| #165 (D1R1) | `documentProfileValidator`, clean OAS admission, and both pattern parsers | Complete document-wide exclusions precede compilation; reachable ordinary checks retain their scope; pattern nullability and one authored-pattern size accumulator agree; no post-compilation exclusion scan or shared production semantics | **Pass.** Exact admission/pointer conformance and clean-room guards passed; no correction required. |
| #169 (D1R2) | iterative YAML/strict-value frames, `cloneJSONValue`, exact-number lexical normalization, and shared URI syntax | No recursive admitted-value walk or depth cap; no one-zero-at-a-time arithmetic normalization; one URI/IP-literal scanner; occurrence-owned clones | **Pass.** Deep-value, number, URI, and ownership evidence passed; no correction required. |
| #173 (D1R3) | `evaluationRecords`, evaluation cache rebasing, enum-member metadata, and record consumers | One heterogeneous ordered record sequence is authoritative; no parallel semantic slices or string-projected identities; cached and uncached complete identities agree | **Pass.** Oracle identity/failure-closure evidence passed; no correction required. |
| #178 (D1R4) | `planBuilder`, requirement sets, valid schedule, fault descriptors, and order keys | Planning is declarative; no candidate evaluation, witness cap/corpus, fixed-width mask, or materialized candidate product; report and execution orders remain separate | **Pass.** Plan and additive-schedule evidence passed; no correction required. |
| #183 (D1R5) | `rowProjectionCursor`, array/object structural frontiers, and active projected members/items | One same-instance projection interpretation; no eager branch/size product or authored-capacity allocation; each structural assignment is charged immediately | **Pass after localized correction.** The beyond-`uint64` branch retains only a scalar `beyondArrayCursor`. Each advance charges and selects one attempt-local child, discards it before yielding, and treats unavailable children as ordinary exhaustion. Retention, AST, and exact public-Build regressions lock the seam. |
| #188 (D1R6) | compact pattern machines, `basicStringProduct`, declarative format programs, and directed objective schedule | No counted-machine expansion, sample/post-filter approximation, hidden search cap, visited witness corpus, or coupling to production validation | **Pass.** Exact string-product/format evidence passed; no correction required. |
| #193 (D1R7) | fault product continuation, parent replay, array/object/scalar mutation machines, and closure comparison | One fair continuation; no first-parent commitment, rank-prefix replay, delayed batch charging, alias-preserving copies, or non-identity closure comparison | **Pass after localized correction.** Ranked array insertion and combination coordinates are directly addressed without complete tuple recipes. Array derivatives grow one coordinate only after its charge. Oversized object/array attempts perform one ranked edit before yielding to the outer fault product. |
| #200 (D1R8) | callback lifetime, transitive dependency guard, ownership allowlist, generated-value and source-provenance analyses | Callback bytes are borrowed; local dependency closure stays clean; ownership is role-specific; no generated corpus or source-specific semantic switch survives | **Pass.** Callback, clean-room, ownership/no-corpus, and retention campaigns passed; no beyond-count generated row survives an attempt boundary. |

No blocking grand architectural failure was found in this audit.

## D1R9.2 integrated public-seam evidence

`pkg/schematest/build_integration_acceptance_test.go` records exact ordered `[]Case` and complete `Report` values for a genuinely compound repeated-reference/request-direction/composition/scalar/container document, anyOf closure, genuinely open alternatives, beyond-`uint64` fairness and exhaustion, billion-count cutoffs, mutation-name collision, and supplementary public 65-branch **allOf** cutoff evidence. The arbitrary-width **anyOf** mask proof is the source/unit audit sanctioned by #208: `parentReplayMaskAtOrdinal` and `parentReplayMaskCursor` cover bit 64 and an ordinal beyond `uint64`, while source assertions forbid fixed-width narrowing. Private helpers are the intended behavioral seam because a public 65-branch anyOf exact report would require `2^65-1` canonical mask obligations.

## D1R9.3 campaigns

All commands below were run from the repository root on 2026-08-10.

| Campaign | Exact command | Outcome |
|---|---|---|
| Runtime corpus / production verdict consumer | `go test ./pkg/schematest -run '^TestCorpusRuntimeVerdictsMatchBuild$' -count=1` | PASS, 11.853s; unchanged callback bytes agreed by verdict. |
| Generated alpha/zeta consumer | `go test ./pkg/generate -run '^TestGenerateWritesCompiledValidation$' -count=1` | PASS, 8.047s; generated alpha/zeta validators agreed by verdict. |
| Callback error and lifetime | `go test ./pkg/schematest -run '^(TestBuildCallbackBytesRequireCallerCopyAndAreNotRetained\|TestBuildReturnsCallbackErrors)$' -count=1` | PASS. |
| Transitive clean room and copied/source-specific guards | `go test ./pkg/schematest -run '^(TestProductionImportsStayCleanRoom\|TestCleanRoomDependencyGuardRejectsLocalBridge\|TestGeneratedImportGuardRejectsCheckedInConsumer\|TestCopiedAnswerGuardRejectsConcreteBypasses\|TestCopiedSemanticGuard.*\|TestOperationIDFlowGuardRejectsSourceSpecificBranching)$' -count=1` | PASS. |
| Ownership and no-corpus dataflow | `go test ./pkg/schematest -run '^(TestExactLongLivedOwnershipShapes\|TestGeneratedValuesDoNotEscapeBuildRuntime\|TestStructuralFrontierRetainsNoProjectionCorpusOrLocalProduct)$' -count=1` | PASS, 34.797s after removing stale cursor allowlist rows. |
| Retention | `go test ./pkg/schematest -run '^(TestBuildRetainedMemoryIsFlatWithEmittedCount\|TestBeyondCountCursorRetainsNoGeneratedRowAcrossBudgets)$' -count=1` | PASS, 0.210s. |
| Deterministic complete stream | `go test ./pkg/schematest -run '^TestBuildFullStreamDeterminism$' -count=1` | PASS, 0.159s. |
| Arbitrary-width anyOf source/unit audit | `go test ./pkg/schematest -run '^(TestAnyOfReplayMaskUnitAuditReachesBeyondUint64\|TestAnyOfReplayMasksHaveNoFixedWidthNarrowing\|TestBuildSixtyFiveAllOfBranchesHasExactCutoffReport)$' -count=1` | PASS; bit 64, a beyond-`uint64` ordinal, the wide cursor, fixed-width source exclusions, and separately labeled 65-allOf public cutoff evidence all ran. |
| R1 admission/pointers | `go test ./pkg/schematest -run '^(TestPublicAdmissionConformance\|TestPublicDocumentProfileTraversalConformance\|TestPublicStructuralAdmissionConformance\|TestParseInputOpenAPIVersionProfile\|TestParseInputRejectsExcludedSchemaCapabilitiesAtAuthoredPointers\|TestMakePlanAcceptsSchemaNamesThatMatchCompositionKeywords)$' -count=1` | PASS, 3.997s; admission matrix, profile traversal, pointer failures, and MakePlan keyword-name admission all matched. |
| R2 strict values/numbers/annotations | `go test ./pkg/schematest -run '^(TestDeepJSONValueOperationsUseHeapBackedTraversal\|TestDeepJSONBuildFinishesWithoutProcessStackFailure\|TestDeepYAMLBuildFinishesWithoutProcessStackFailure\|TestExactNumber.*\|TestValidateURIReference.*\|TestURIComponentAndFragmentScanningShareExactPercentTriplets\|TestParseInputAcceptsAndDiscardsWellFormedInertMetadata\|TestParseInputShapeChecksInertMetadata\|TestInertSchemaMetadataDoesNotChangeAuthoritativeRecords)$' -count=1` | PASS, 0.490s; deep JSON/YAML, exact numbers, URI scanning, and inert annotations all ran. |
| R3 authoritative oracle identities | `go test ./pkg/schematest -run '^(TestEvaluationCache.*\|TestEvaluationRecordsCacheHitKeepsEveryFact\|TestEvaluateReferenceCacheRebasesCompleteRecords\|TestCleanOracleFailureIdentitiesDistinguishRuleAndBranchOccurrences)$' -count=1` | PASS; cache and complete-identity campaigns ran. |
| R4 declarative obligations/additive schedule | `go test ./pkg/schematest -run '^(TestMakePlan.*\|TestPlannerCallGraphIsDeclarative\|TestBuildStreamsDeterministicValidPrimitiveRows)$' -count=1` | PASS, 0.575s; MakePlan and declarative call-graph campaigns ran. |
| R5 lazy structural search | `go test ./pkg/schematest -run '^(TestBuildBeyondUint64ArrayYieldsToLaterProjection\|TestBuildUnavailableBeyondCountChildYieldsNormally\|TestSharedArrayFrontierBuildsAllOfItems\|TestArrayFrontierHasNoSyntheticCutoffLoop)$' -count=1` | PASS. |
| R6 compact exact string search | `go test ./pkg/schematest -run '^(TestSimpleStringFormatWitnessesAreCanonicalAndDeterministic\|TestBasicStringProductTransitionsAreDeterministic)$' -count=1` | PASS. |
| R7 isolated replay/closures | `go test ./pkg/schematest -run '^(TestNonCompositionFaultFamiliesHaveExactClosures\|TestArrayCountFaultCutoffPrecedesEachAtomicAssignment\|TestArrayInsertionFaultCutoffPrecedesEveryCoordinateMutation\|TestArrayInsertionRanksAddressWithoutPrefixReplay\|TestOversizedArrayFaultRanksYieldDeterministically\|TestOversizedObjectFaultRanksYieldDeterministically)$' -count=1` | PASS; direct coordinates, per-charge cutoffs, and outer-product yielding all ran. |
| R8 callback/clean-room/ownership | `go test ./pkg/schematest -run '^(TestBuildCallbackBytesRequireCallerCopyAndAreNotRetained\|TestProductionImportsStayCleanRoom\|TestExactLongLivedOwnershipShapes\|TestCopiedAnswerGuardRejectsConcreteBypasses\|TestCopiedSemanticGuard.*\|TestOperationIDFlowGuardRejectsSourceSpecificBranching)$' -count=1` | PASS, 34.047s; copied/source-specific guards were included. |

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

The Cycle 2 completion phase recorded formatter/linter findings and formatter-induced provenance-hash changes together, corrected them as a batch, then consolidated duplicate cutoff-test setup reported by the second boundary. The complete third boundary passed:

| Command | Outcome |
|---|---|
| `make fmt` | PASS; `gofumpt -w .` plus `golangci-lint run --fix`, 0 issues. |
| `make lint` | PASS; 0 issues. |
| `make test` | PASS; `go test ./...`, including `pkg/schematest` in 127.905s. |
| `git diff --check` | PASS. |

`.golangci.yml` is unchanged.
