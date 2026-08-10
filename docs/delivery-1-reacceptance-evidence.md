# Delivery 1 reacceptance evidence

This document records the D1R9 acceptance audit and the exact focused campaigns run for #206. The root contract is #164 (ultimately #55); the repaired parents are #165, #169, #173, #178, #183, #188, #193, and #200.

## D1R1–D1R8 architecture audit

| Repair | Inspected private seam | Governing invariant and forbidden shapes | Result / localized correction |
|---|---|---|---|
| #165 (D1R1) | `documentProfileValidator`, clean OAS admission, and both pattern parsers | Complete document-wide exclusions precede compilation; reachable ordinary checks retain their scope; pattern nullability and one authored-pattern size accumulator agree; no post-compilation exclusion scan or shared production semantics | **Pass.** Exact admission/pointer conformance and clean-room guards passed; no correction required. |
| #169 (D1R2) | iterative YAML/strict-value frames, `cloneJSONValue`, exact-number lexical normalization, and shared URI syntax | No recursive admitted-value walk or depth cap; no one-zero-at-a-time arithmetic normalization; one URI/IP-literal scanner; occurrence-owned clones | **Pass.** Deep-value, number, URI, and ownership evidence passed; no correction required. |
| #173 (D1R3) | `evaluationRecords`, evaluation cache rebasing, enum-member metadata, and record consumers | One heterogeneous ordered record sequence is authoritative; no parallel semantic slices or string-projected identities; cached and uncached complete identities agree | **Pass.** Oracle identity/failure-closure evidence passed; no correction required. |
| #178 (D1R4) | `planBuilder`, requirement sets, valid schedule, fault descriptors, and order keys | Planning is declarative; no candidate evaluation, witness cap/corpus, fixed-width mask, or materialized candidate product; report and execution orders remain separate | **Pass.** Plan and additive-schedule evidence passed; no correction required. |
| #183 (D1R5) | `rowProjectionCursor`, array/object structural frontiers, and active projected members/items | One same-instance projection interpretation; no eager branch/size product or authored-capacity allocation; each structural assignment is charged immediately | **Pass after localized correction.** The beyond-`uint64` array charge-only loop became one transient `beyondArrayRow` advanced once between shared-frontier alternatives. It emits no partial row and cannot starve a later viable projection. AST and public-Build regressions lock the seam. |
| #188 (D1R6) | compact pattern machines, `basicStringProduct`, declarative format programs, and directed objective schedule | No counted-machine expansion, sample/post-filter approximation, hidden search cap, visited witness corpus, or coupling to production validation | **Pass.** Exact string-product/format evidence passed; no correction required. |
| #193 (D1R7) | fault product continuation, parent replay, array/object/scalar mutation machines, and closure comparison | One fair continuation; no first-parent commitment, rank-prefix replay, delayed batch charging, alias-preserving copies, or non-identity closure comparison | **Pass after localized correction.** Ranked array insertion now directly addresses its tuple. Array derivatives mutate one independent clone incrementally, charging retry, length, index, and value mutations at their concrete boundaries; `arrayEditCharges` and complete replacement recipes were removed. |
| #200 (D1R8) | callback lifetime, transitive dependency guard, ownership allowlist, generated-value and source-provenance analyses | Callback bytes are borrowed; local dependency closure stays clean; ownership is role-specific; no generated corpus or source-specific semantic switch survives | **Pass.** Callback, clean-room, ownership/no-corpus, and retention campaigns passed; the new sole beyond-count current-row role is explicitly call-lifetime bounded. |

No blocking grand architectural failure was found in this audit.

## D1R9.2 integrated public-seam evidence

`pkg/schematest/build_integration_acceptance_test.go` records exact ordered `[]Case` and complete `Report` values for repeated-reference/request-direction, anyOf closure, genuinely open structural and fault alternatives, beyond-`uint64` structural fairness, billion-count cutoffs, mutation-name collision, and a public 65-branch cutoff. `pkg/schematest/source_guard_test.go` supplements behavior with AST guards against charge-only and rank-prefix loops; it is not the behavioral proof.

## D1R9.3 campaigns

All commands below were run from the repository root on 2026-08-10.

| Campaign | Exact command | Outcome |
|---|---|---|
| Runtime corpus / production verdict consumer | `go test ./pkg/schematest -run '^TestCorpusRuntimeVerdictsMatchBuild$' -count=1` | PASS, 11.708s; unchanged callback bytes agreed by verdict. |
| Generated alpha/zeta consumer | `go test ./pkg/generate -run '^TestGenerateWritesCompiledValidation$' -count=1` | PASS, 7.698s; generated alpha/zeta validators agreed by verdict. |
| Callback error and lifetime | `go test ./pkg/schematest -run '^(TestBuildCallbackBytesRequireCallerCopyAndAreNotRetained|TestBuildReturnsCallbackErrors)$' -count=1` | PASS. |
| Transitive clean room and copied/source-specific guards | `go test ./pkg/schematest -run '^(TestProductionImportsStayCleanRoom|TestCleanRoomDependencyGuardRejectsLocalBridge|TestGeneratedImportGuardRejectsCheckedInConsumer)$' -count=1` | PASS. |
| Ownership and no-corpus dataflow | `go test ./pkg/schematest -run '^(TestExactLongLivedOwnershipShapes|TestGeneratedValuesDoNotEscapeBuildRuntime|TestStructuralFrontierRetainsNoProjectionCorpusOrLocalProduct)$' -count=1` | PASS, 34.293s. |
| Retention | `go test ./pkg/schematest -run '^TestBuildRetainedMemoryIsFlatWithEmittedCount$' -count=1` | PASS, 0.093s. |
| Deterministic complete stream | `go test ./pkg/schematest -run '^TestBuildFullStreamDeterminism$' -count=1` | PASS, 0.157s. |
| R1 admission/pointers | `go test ./pkg/schematest -run '^(TestAdmissionConformanceMatrix|TestParseInputAdmitsRepresentativeCorpus)$' -count=1` | PASS. |
| R2 strict values/numbers/annotations | `go test ./pkg/schematest -run '^(TestDeepStrictJSONOperationsAreIterative|TestExactNumber|TestURI)' -count=1` | PASS. |
| R3 authoritative oracle identities | `go test ./pkg/schematest -run '^(TestEvaluateReferenceCacheRebasesCompleteRecords|TestCleanOracleFailureIdentitiesDistinguishRuleAndBranchOccurrences)$' -count=1` | PASS. |
| R4 declarative obligations/additive schedule | `go test ./pkg/schematest -run '^(TestPlan|TestBuildStreamsDeterministicValidPrimitiveRows)$' -count=1` | PASS. |
| R5 lazy structural search | `go test ./pkg/schematest -run '^(TestBuildBeyondUint64ArrayYieldsToLaterProjection|TestSharedArrayFrontierBuildsAllOfItems)$' -count=1` | PASS. |
| R6 compact exact string search | `go test ./pkg/schematest -run '^(TestSimpleStringFormatWitnessesAreCanonicalAndDeterministic|TestBasicStringProductTransitionsAreDeterministic)$' -count=1` | PASS. |
| R7 isolated replay/closures | `go test ./pkg/schematest -run '^(TestNonCompositionFaultFamiliesHaveExactClosures|TestArrayCountFaultCutoffPrecedesEachAtomicAssignment|TestArrayInsertionRanksAddressWithoutPrefixReplay)$' -count=1` | PASS after correcting the direct tuple cumulative-count decoder. |
| R8 callback/clean-room/ownership | `go test ./pkg/schematest -run '^(TestBuildCallbackBytesRequireCallerCopyAndAreNotRetained|TestProductionImportsStayCleanRoom|TestExactLongLivedOwnershipShapes)$' -count=1` | PASS, 31.398s. |

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

The completion phase first recorded formatter/linter findings together with the provenance-hash regression, corrected them as one batch, and reran the complete boundary. Final outcomes:

| Command | Outcome |
|---|---|
| `make fmt` | PASS; `gofumpt -w .` plus `golangci-lint run --fix`, 0 issues. |
| `make lint` | PASS; 0 issues. |
| `make test` | PASS; `go test ./...`, including `pkg/schematest` in 136.048s. |
| `git diff --check` | PASS. |

`.golangci.yml` is unchanged.
