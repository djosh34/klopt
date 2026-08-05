---
name: implement-workflow
description: Runs a reusable hierarchy workflow for implementing one larger GitHub issue through sequential implementation, review, testing, checks, merge, and issue closure.
---

# Implement Workflow

This skill takes exactly one parent GitHub issue URL. It is reusable for any repository and issue number; it is not a ticket-68 workflow.

## Exact skill tree

```text
SKILL.md
top-level/manager/create-top-level-issue-manager.sh
top-level/manager/prompt.txt
implementation/manager/create-implementation-manager.sh
implementation/manager/prompt.txt
implementation/manager/implementer/create-implementer.sh
implementation/manager/implementer/prompt.txt
implementation/manager/implementer/fix-test-failures.txt
implementation/manager/test-runner/create-test-runner.sh
implementation/manager/test-runner/prompt.txt
review/reviewers/create-reviewers.sh
review/reviewers/reviewer-1.txt
review/reviewers/reviewer-2.txt
review/reviewers/review-fixes.txt
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

Role-specific instructions live in the adjacent `.txt` files. Do not copy them into a launcher or add prompt heredocs. Launchers read their neighboring prompt and append actual runtime URLs, paths, SHAs, and cycle values.

## Start and boss supervision

The already-running boss agent supplies one full URL such as:

```text
https://github.com/OWNER/REPO/issues/68
```

Resolve this file's directory as `SKILL_DIR`, then call the top launcher once:

```bash
TOP_CREATE="$SKILL_DIR/top-level/manager/create-top-level-issue-manager.sh"
"$TOP_CREATE" "$PARENT_ISSUE_URL"
```

Retain both printed values exactly:

```text
WORKSPACE_ID=...
AGENT_ID=...
```

The launcher creates the top workspace with runtime title `Larger Issue N` using `paseo workspace create --isolation local --title "Larger Issue N"`, then creates the detached top-level manager in that workspace. The top manager uses `openrouter/deepseek/deepseek-v4-flash-0731` with thinking `high`.

Paseo's normal caller-agent relationship supplies the hierarchy through the caller's `PASEO_AGENT_ID`. Do not add hierarchy environment variables. All launchers use detached `paseo ... run -d`; no launcher waits for an agent.

The boss does not continuously inspect GitHub or interfere with healthy work. Every twenty minutes, manually run:

```bash
sleep 1200
paseo inspect --json "$TOP_AGENT_ID"
paseo logs --tail 20 "$TOP_AGENT_ID"
```

Compare the last twenty log entries mentally with the previous check. Hands off while logs and progress move. Only after twenty minutes with no log movement or progress investigate the top manager, its child agents, the repository, and GitHub. If healthy, continue waiting. If genuinely stuck, preserve work and choose the smallest judgment-based intervention: reuse the existing agent, send a precise prompt, or replace/resume from the smallest safe boundary. Do not write a polling loop, counter file, recovery tree, or recovery script.

When the top manager has truly completed the merge and every required issue closure, the boss manually archives the retained top workspace:

```bash
paseo workspace archive "$TOP_WORKSPACE_ID"
```

No bundled script archives a workspace, and no child archives itself. Do not run this workflow while implementing or validating this skill.

## Launcher interfaces and exact role assignments

Call launchers from the agent that owns the intended Paseo hierarchy. Child agents inherit their caller naturally.

- `create-top-level-issue-manager.sh PARENT_ISSUE_URL`: workspace and detached manager both titled `Larger Issue N`; DeepSeek V4 Flash, high.
- `create-implementation-manager.sh ROOT PARENT_ISSUE_URL WORK_ISSUE_URL SHARED_BRANCH`: workspace `Issue ROOT->WORK`; detached manager titled `Issue ROOT->WORK: Implementation Manager`; DeepSeek V4 Flash, high.
- `create-implementer.sh ROOT PARENT_ISSUE_URL WORK_ISSUE_URL SHARED_BRANCH`: detached `Issue ROOT->WORK: Implementer`; Luna, max.
- `create-test-runner.sh ROOT CONTEXT_ISSUE_URL SHARED_BRANCH TITLE`: detached runner with the supplied title. Use `Issue ROOT->WORK: Test runner` for an issue and `Larger Issue ROOT: Test runner` for the final run; Luna, **high**.
- `create-reviewers.sh ROOT PARENT_ISSUE_URL PR_URL FIXED_REVIEW_BASE_SHA CYCLE`: detached Code-review-1 and Code-review-2; both Sol, high; prints `REVIEWER_1_AGENT_ID=...` and `REVIEWER_2_AGENT_ID=...`.
- `create-arbitrator.sh ROOT PARENT_ISSUE_URL PR_URL FIXED_REVIEW_BASE_SHA ARTIFACT_DIR SHARED_BRANCH`: detached `Larger Issue ROOT: Review Arbitrator`; Sol, high.
- `create-fixer.sh ROOT PARENT_ISSUE_URL PR_URL SHARED_BRANCH ARTIFACT_DIR`: detached `Larger Issue ROOT: Fixer`; Luna, max.

Workspace launchers print `WORKSPACE_ID=...` and `AGENT_ID=...`. Agent-only launchers print `AGENT_ID=...`, except the reviewer launcher prints both distinct reviewer labels. These scripts only create the named workspace/agent(s); they do not wait, poll, retry, archive, clean up, or check status. The exporter is the only non-launcher script.

## Workflow boundaries

The top manager's full sequence is in `top-level/manager/prompt.txt`. In summary, it dynamically reads the parent hierarchy, ordered required subissues, and transitive blockers; deduplicates them; processes all required work sequentially with dependency-only reordering; and closes every completed blocker/subissue before the parent. It creates one shared implementation branch and one final PR. It never edits implementation code, but may commit and push exact deterministic formatter output.

For each work issue it launches one implementation manager, retains both IDs, manually supervises that child every five minutes with `sleep 300`, status inspection, and `paseo logs --tail 20`, and investigates only after twenty minutes without log movement or progress. After a tested commit handoff it manually archives that issue workspace. The implementation manager's full contract is in `implementation/manager/prompt.txt`; its implementer and test-runner prompts are next to their launchers. The manager creates the test runner only after the implementer handoff, waits for the launcher's initial run, and resends the same runner prompt only after a correction. After every runner turn it inspects the worktree; it directly commits and pushes successful `make fmt` changes before rerunning the test runner, while concrete command failures go to the implementer.

An already-linked open PR is an unlikely edge case, not the normal flow. It may be adopted only after the local checkout is clean and exactly synchronized to the remote PR branch; the default branch is synchronized before the shared branch is created. The final PR closes/references the parent and every implemented required subissue and blocker.

Review artifacts live in `/tmp/implement-workflow-ROOT/`. The persistent arbitrator and fixer are each created once. The two internal reviewers are created together for cycle 1, then reused with `review-fixes.txt`. The arbitrator is reused with `next-cycle.txt` for cycles 1-3 and, after a cycle-3 fixer push, with `verify-final-fixes.txt` for post-cycle verification. That verification is not cycle 4: it starts no reviews, accepts no new findings, and sends no fixer task. The fixer is reused with `apply-next-fixes.txt` and `fix-check-failures.txt`. Follow-up sends read the relevant `.txt` file and append actual runtime values; they never use a prompt heredoc.

There are at most three review/adjudication/fix cycles. A later cycle is allowed only after the prior cycle's accepted findings caused pushed code. Cycle 3 is final; there is no cycle 4. External Codex and non-rate-limited CodeRabbit are requested once per active cycle with exact standalone comments `@codex review` and `@coderabbitai review`; CodeRabbit may be skipped only when rate limited. Internal reviewers independently cover Standards and Spec against the fixed review base, post exactly one top-level comment each, and never read GitHub comments/reviews/findings or run tests/fmt/lint. Their comments begin exactly with their `bot: code-review-N` line and `cycle: N` on line two.

All cycles accept only a direct applicable parent/required-issue contradiction, unimplemented required scope, a PR-introduced regression, or a concrete failing input, test, or required check. Speculation, hardening, refactors, design preferences, future work, unrelated pre-existing defects, optional improvements, and extra tests without acceptance evidence are rejected. Cycle 1 has the lower filter bar, cycle 2 requires substantially skeptical concrete merge-blocking evidence, and cycle 3 requires the highest, unequivocal direct evidence.

Each cycle's arbitrator runs `review/comments/export-comments.sh PR_URL ARTIFACT_DIR/comments.json`, overwriting the same file, reads all actors' comments/reviews/inline comments, retains context, detects doom loops/bad/noisy fixes, replies to every exact review finding, and overwrites the same `to_fix.txt` with only current accepted work. Arbitrator replies begin exactly `manager: review-arbitrator`; rejected threads are resolved immediately and accepted threads only after later verifying a fix. The arbitrator never runs tests/fmt/lint or edits code. The fixer implements only explicit accepted work or exact operational failures, runs focused tests while changing code, commits/pushes, and never adjudicates, comments, creates subagents, contacts agents, or runs Paseo.

After the final or otherwise relevant review cycles, one final dumb test runner is created only after the review/fixer handoff. Its launcher starts the initial run, which the manager waits for before any resend. It runs `make fmt`, `make lint`, and `make test`, reports exact results, and does nothing else. After every runner turn the top manager inspects the worktree. The top manager directly commits and pushes successful formatter changes; concrete command failures go to the existing fixer with exact evidence. The same runner is resent only after a correction. Proceed only when all commands pass and the worktree is clean with changes committed. Required check and concrete conflict failures go to that fixer as exact operational work. Merge simply with:

```bash
gh pr merge --squash --delete-branch
```

Do not add an OID, fallback merge method, or scripted retry tree. Verify the merged PR and every required issue closure, delete the artifact directory after merge, return completed to the boss, and let the boss archive the top workspace.

## Follow-up prompt convention

For a direct follow-up, the manager reads a nearby `.txt` and appends labels with actual values. For example:

```bash
paseo send "$ARBITRATOR_ID" "$(cat "$SKILL_DIR/review/arbitrator/next-cycle.txt"; printf '\n\nRUNTIME VALUES\nPR_URL=%s\nCURRENT_HEAD_SHA=%s\nCYCLE=%s\nCOMMENTS_JSON=%s\nTO_FIX=%s\n' "$PR_URL" "$HEAD_SHA" "$CYCLE" "$ARTIFACT_DIR/comments.json" "$ARTIFACT_DIR/to_fix.txt")"
```

Use the same convention for reviewer, arbitrator final-verification, and fixer follow-ups, appending every actual URL, path, SHA, cycle, accepted-item list, and exact failure output needed by that turn. Do not add another prompt file unless the send request is genuinely a distinct role or operation.

## Export format

`review/comments/export-comments.sh` takes exactly a PR URL and output path. It uses `gh api --paginate --slurp` and `jq` to combine all issue comments, formal reviews, and inline review comments from all actors into one valid JSON array. Entries include useful IDs, kind, author, body, URL, timestamps, node/review IDs, reply metadata, and inline source/commit details where available. It does not filter authors or interpret findings, create directories, or perform any other workflow action.
