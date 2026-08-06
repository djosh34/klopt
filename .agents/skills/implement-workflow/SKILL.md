---
name: implement-workflow
description: Runs a reusable hierarchy workflow for implementing one parent GitHub issue under an explicitly supplied root issue through sequential implementation, review, testing, checks, merge, and evidence-based closure.
---

# Implement Workflow

This skill takes exactly two full GitHub issue URLs: the governing root issue and the bounded parent issue to implement. It is reusable for any repository and issue numbers.

## Skill tree

```text
SKILL.md
top-level/manager/create-top-level-issue-manager.sh
top-level/manager/prompt.txt
implementation/worker-rules.txt
implementation/manager/create-implementation-manager.sh
implementation/manager/prompt.txt
implementation/manager/implementer/create-implementer.sh
implementation/manager/implementer/prompt.txt
implementation/manager/implementer/fix-test-failures.txt
implementation/manager/test-runner/create-test-runner.sh
implementation/manager/test-runner/prompt.txt
review/reviewers/create-reviewers.sh
review/reviewers/create-architecture-reviewer.sh
review/reviewers/prompt.txt
review/reviewers/architecture.txt
review/reviewers/review-fixes.txt
review/reviewers/verify-final-fixes.txt
review/comments/export-comments.sh
review/arbitrator/create-arbitrator.sh
review/arbitrator/prompt.txt
review/arbitrator/next-cycle.txt
review/arbitrator/verify-final-fixes.txt
review/fixer/create-fixer.sh
review/fixer/prompt.txt
review/fixer/apply-next-fixes.txt
review/fixer/fix-check-failures.txt
```

Role-specific instructions live in adjacent text files. Implementer and fixer launchers/follow-ups concatenate `implementation/worker-rules.txt`, their thin role/operation prompt, and runtime values. General leaf reviewers share one prompt; the architecture reviewer has its own specialty prompt. Do not add prompt heredocs to scripts or messages.

## Start and validation

The already-running boss must explicitly supply `ROOT_ISSUE_URL` and `PARENT_ISSUE_URL` as full GitHub issue URLs. Call the top launcher once:

```bash
TOP_CREATE="$SKILL_DIR/top-level/manager/create-top-level-issue-manager.sh"
"$TOP_CREATE" "$ROOT_ISSUE_URL" "$PARENT_ISSUE_URL"
```

The launcher rejects malformed URLs, cross-repository pairs, pull-request URLs disguised as issues, missing issues, and a parent that is neither root nor a transitive root subissue. Validation finishes before workspace creation. Retain the exact output:

```text
WORKSPACE_ID=...
AGENT_ID=...
```

The top manager uses DeepSeek V4 Flash with high thinking. The boss supervises manually every five minutes:

```bash
sleep 300
paseo inspect --json "$TOP_AGENT_ID"
paseo logs --tail 20 "$TOP_AGENT_ID"
```

Do not write a polling loop, counter, recovery tree, or monitoring script. Compare the latest twenty entries mentally. Investigate only after twenty minutes without log movement or progress. Preserve work and intervene at the smallest safe boundary. Archive the retained top workspace only after merge and justified issue closure:

```bash
paseo workspace archive "$TOP_WORKSPACE_ID"
```

Do not run this workflow while implementing or validating the skill.

## Authority and work graph

`ROOT_ISSUE_URL` is always the highest specification authority. `PARENT_ISSUE_URL` selects the bounded hierarchy for this run. Parent, descendant, and blocker requirements are binding decomposition when compatible with root; root wins every conflict. Agents continue root-compatible work without rewriting conflicting issue text.

The top manager discovers the parent's ordered required subissues, linked work issues, and transitive blockers from live GitHub data, deduplicates canonical URLs, and reorders only for dependencies. No issue number or queue is hardcoded. It verifies closed-issue evidence before relying on it, uses one shared implementation branch, processes work sequentially, and opens one integration PR only after every implementation handoff.

Both root and parent URLs are passed to every manager, code writer, reviewer, arbitrator, fixer, and test runner.

## Launchers and models

- `create-top-level-issue-manager.sh ROOT_ISSUE_URL PARENT_ISSUE_URL`: workspace and manager `Issue ROOT->PARENT`; DeepSeek V4 Flash, high.
- `create-implementation-manager.sh ROOT_ISSUE_URL PARENT_ISSUE_URL WORK_ISSUE_URL SHARED_BRANCH`: child workspace and manager; DeepSeek V4 Flash, high.
- `create-implementer.sh ROOT_ISSUE_URL PARENT_ISSUE_URL WORK_ISSUE_URL SHARED_BRANCH`: implementer; **Sol, medium**.
- `create-test-runner.sh ROOT_ISSUE_URL PARENT_ISSUE_URL CONTEXT_ISSUE_URL SHARED_BRANCH TITLE`: dumb runner; Luna, high.
- `create-reviewers.sh ROOT_ISSUE_URL PARENT_ISSUE_URL FOCUS_ISSUE_URL ISSUE_GRAPH_PATH PR_URL FIXED_REVIEW_BASE_SHA CYCLE`: two general leaf reviewers; Sol, high.
- `create-architecture-reviewer.sh ROOT_ISSUE_URL PARENT_ISSUE_URL ISSUE_GRAPH_PATH PR_URL FIXED_REVIEW_BASE_SHA CYCLE`: one whole-PR architecture reviewer; Sol, high.
- `create-arbitrator.sh ROOT_ISSUE_URL PARENT_ISSUE_URL PR_URL FIXED_REVIEW_BASE_SHA ARTIFACT_DIR SHARED_BRANCH`: persistent arbitrator; Sol, high.
- `create-fixer.sh ROOT_ISSUE_URL PARENT_ISSUE_URL PR_URL SHARED_BRANCH ARTIFACT_DIR`: persistent fixer; **Sol, medium**.

Workspace launchers print `WORKSPACE_ID` and `AGENT_ID`. Agent-only launchers print explicit role labels such as `IMPLEMENTER_AGENT_ID`, `TEST_RUNNER_AGENT_ID`, `ARBITRATOR_AGENT_ID`, and `FIXER_AGENT_ID`; reviewer launchers also print markers. Every launcher validates exact arity and URL shapes before calling Paseo. Launchers create only their named workspace/agents and never wait, poll, retry, archive, or clean up.

Top and implementation managers inspect active children manually every 60 seconds and use the same twenty-minute stalled threshold. External review snapshots also remain manual.

## Shared code-writing contract

Every implementer and fixer turn receives the shared worker rules plus its specific operation. Focused tests are strongly advised during coding.

Before all requested code for the turn is complete, general formatting, linting, and full-test commands are explicitly **FORBIDDEN**. Never invoke a formatter directly. Do not run `make fmt`, `make lint`, or `make test` in the edit loop.

Once coding is complete, the worker enters one completion phase: run all three Make targets, collect failures, fix them together, and rerun the completion checks until green. Then commit the complete result. Implementers do not push; fixers push the existing PR branch. Independent dumb test runners remain as manager-owned handoff verification.

## Review topology

All reviews occur after every parent implementation cycle and after the single PR is open. Nothing reviews between implementation managers.

Before launch, the manager writes the complete canonical issue list, hierarchy, dependency order, links, blockers, and applicability notes to immutable `issue-graph.txt`. For each implemented leaf issue, create exactly two independent general reviewers from the shared `reviewers/prompt.txt`. Each reads that graph but focuses on whether the PR correctly implements its leaf. Create one additional whole-PR architecture/design reviewer from `architecture.txt`. All internal reviewers are Sol high, created in cycle 1, and reused in later cycles.

Codex and non-rate-limited CodeRabbit are still requested once per active cycle with exact standalone comments `@codex review` and `@coderabbitai review`. Internal reviewers post one top-level comment per turn with their unique `bot: code-review-*` marker and numeric cycle. They never run tests or edit code.

Reviewers in later cycles receive only the arbitrator dispositions for their own prior findings. They state whether accepted findings were correctly fixed, may challenge a rejection once with new concrete evidence, and independently inspect the new fixed-base diff under the current cycle bar. They never read other reviewers' findings.

## Arbitration and cycles

GitHub is the review inbox. `export-comments.sh` combines issue comments, formal reviews, and inline comments and includes `author_type` and `author_association`. Only `OWNER`, `MEMBER`, and `COLLABORATOR` comments are user-authoritative; other human comments are ordinary review input. Authority identifies the source, not a third unresolved disposition: every request is Accepted or Rejected, accepted work blocks merge until verified, and root-incompatible or scope-expanding requests are rejected with explanation.

The persistent arbitrator may inspect issues, diffs, source, and tests only to decide submitted evidence. It may run the exporter, read-only inspection commands, and required `gh` reply/resolution operations; it never executes repository code/tests, runs build/format/lint commands, originates findings, edits code, or acts as fixer. It uses the current reviewer markers supplied by the manager, replies to every exact finding, has final say when reviewers disagree, and writes:

- `comments.json`: complete refreshed review export;
- `to_fix.txt`: only current accepted implementation work;
- `dispositions.txt`: each internal reviewer's own accepted/rejected findings and reasons.

There are at most three finding-discovery cycles:

1. **Cycle 1:** concrete root-compatible correctness, performance, maintainability, refactoring, and especially design corrections are welcome. No finding or remedy may contradict root or expand parent scope, even when technically attractive.
2. **Cycle 2:** only demonstrated material problems are accepted; minor polish and alternate tastes are rejected.
3. **Cycle 3:** only major correctness/spec regressions or severe architecture/performance defects that should block merge are accepted.

A later cycle occurs only after accepted findings caused pushed code. Reviewers are reused with their own prior dispositions and asked whether they agree with the pushed changes. The arbitrator reconsiders one evidence-based challenge to a rejection and remains final authority.

Three cycles cap new-finding discovery, not correct completion of accepted work. After cycle-3 fixes, only originating internal reviewers perform resolution-only verification; they cannot add findings. The arbitrator verifies all frozen accepted items, including external-origin items. An incomplete accepted fix returns to the fixer and is reverified without adding scope or creating cycle 4. Non-convergence or ambiguity blocks and escalates; known-bad code is never merged.

## Final checks, merge, and closure

After review resolution, one final dumb runner executes `make fmt`, `make lint`, and `make test`. Exact operational failures go to the persistent fixer with the shared worker rules and operation prompt. Required checks must pass and the worktree must be clean.

Merge simply:

```bash
gh pr merge --squash --delete-branch
```

The PR references issues without automatic closing keywords. Verify the merge, then close completed issues manually bottom-up with evidence. After the supplied parent, evaluate each ancestor toward root. Close an ancestor only when all required subissues and blockers are complete and its own acceptance is satisfied. The root may close when this run completed its final outstanding hierarchy; completing one parent alone is never sufficient.

Review artifacts use `/tmp/implement-workflow-ROOT-PARENT/` and are deleted only after merge and every justified closure is verified. The boss, not a child, archives the top workspace.

## Follow-up prompt convention

For every direct follow-up, concatenate the applicable text files and append actual runtime values. Implementer and fixer code-writing turns always begin with `implementation/worker-rules.txt`. Reviewer follow-ups include only that reviewer's prior dispositions. Arbitrator turns include every current URL, SHA, cycle, and artifact path required by the operation. Never use prompt heredocs and never add another prompt file unless the operation is genuinely distinct.
