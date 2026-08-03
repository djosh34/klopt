#!/usr/bin/env bash
set -euo pipefail

IMPLEMENTER_MODEL=${IMPLEMENTER_MODEL:-pi/openai-codex/gpt-5.6-luna}
IMPLEMENTER_REASONING=${IMPLEMENTER_REASONING:-max}
FIXER_MODEL=${FIXER_MODEL:-pi/openai-codex/gpt-5.6-luna}
FIXER_REASONING=${FIXER_REASONING:-max}
CODE_REVIEWER_1_MODEL=${CODE_REVIEWER_1_MODEL:-pi/openai-codex/gpt-5.6-luna}
CODE_REVIEWER_1_REASONING=${CODE_REVIEWER_1_REASONING:-max}
CODE_REVIEWER_2_MODEL=${CODE_REVIEWER_2_MODEL:-pi/openai-codex/gpt-5.6-sol}
CODE_REVIEWER_2_REASONING=${CODE_REVIEWER_2_REASONING:-high}
PR_REVIEWER_MODEL=${PR_REVIEWER_MODEL:-pi/openai-codex/gpt-5.6-sol}
PR_REVIEWER_REASONING=${PR_REVIEWER_REASONING:-high}

usage() {
  echo "usage: $0 <implementation-ticket-links-file> <wayfinder-issue-link>" >&2
  exit 2
}

[[ $# -eq 2 ]] || usage

tickets_file=$1
wayfinder_issue=${2%/}

[[ -f "$tickets_file" ]] || {
  echo "ticket links file not found: $tickets_file" >&2
  exit 2
}

issue_url_pattern='^https://github\.com/[^/]+/[^/]+/issues/[0-9]+$'
[[ "$wayfinder_issue" =~ $issue_url_pattern ]] || {
  echo "invalid wayfinder issue link: $wayfinder_issue" >&2
  exit 2
}

for required_command in git gh jq paseo; do
  command -v "$required_command" >/dev/null || {
    echo "$required_command is required" >&2
    exit 127
  }
done

mapfile -t tickets < <(grep -Ev '^[[:space:]]*(#|$)' "$tickets_file" | sed 's/\r$//')
[[ ${#tickets[@]} -gt 0 ]] || {
  echo "ticket links file contains no links: $tickets_file" >&2
  exit 2
}

for ticket_index in "${!tickets[@]}"; do
  ticket=${tickets[$ticket_index]%/}
  [[ "$ticket" =~ $issue_url_pattern ]] || {
    echo "invalid implementation ticket link: ${tickets[$ticket_index]}" >&2
    exit 2
  }
  tickets[ticket_index]=$ticket
done

assert_clean_worktree() {
  local worktree_status
  worktree_status=$(git status --porcelain --untracked-files=all)
  [[ -z "$worktree_status" ]] || {
    echo "worktree must be clean before starting an agent:" >&2
    printf '%s\n' "$worktree_status" >&2
    return 1
  }
}

wait_and_assert_agent() {
  local agent_id=$1
  local agent_state

  paseo wait "$agent_id"
  agent_state=$(paseo inspect --json "$agent_id")
  jq -e '.Status == "idle" and ((.PendingPermissions // []) | length == 0)' \
    <<<"$agent_state" >/dev/null || {
    echo "agent $agent_id did not finish cleanly:" >&2
    jq '{Id, Status, PendingPermissions}' <<<"$agent_state" >&2
    return 1
  }
}

assert_clean_worktree

default_branch=$(gh repo view --json defaultBranchRef --jq '.defaultBranchRef.name')
[[ -n "$default_branch" ]] || {
  echo "could not determine the default branch" >&2
  exit 1
}

for ticket in "${tickets[@]}"; do
  echo "=== Syncing $default_branch before $ticket ==="
  git fetch origin "$default_branch"
  git switch "$default_branch"
  git pull --ff-only origin "$default_branch"
  default_branch_commit=$(git rev-parse HEAD)

  branches_before=$(git for-each-ref --format='%(refname:short)' refs/heads)
  prs_before=$(gh pr list --state all --limit 1000 --json url --jq '.[].url')

  echo "=== Implementing $ticket ==="
  assert_clean_worktree

  implementer_id=$(paseo --quiet run --background \
    --provider "$IMPLEMENTER_MODEL" \
    --thinking "$IMPLEMENTER_REASONING" \
    --title "Implement ${ticket##*/}" \
    "$(cat <<EOF
$ticket

Implement the work described by the user in the spec or tickets.

Use /tdd where possible, at pre-agreed seams.

Run single test files regularly, and the full test suite once at the end.

Do this on new branch, push it and make pr when done

Leave the repository checked out on that new pull-request branch.

Do not create subagents.
EOF
)" )
  wait_and_assert_agent "$implementer_id"

  implementation_branch=$(git branch --show-current)
  [[ -n "$implementation_branch" && "$implementation_branch" != "$default_branch" ]] || {
    echo "implementation did not leave a new branch checked out for $ticket" >&2
    exit 1
  }
  if grep -Fxq "$implementation_branch" <<<"$branches_before"; then
    echo "implementation branch is not new: $implementation_branch" >&2
    exit 1
  fi

  pr_url=$(gh pr view "$implementation_branch" --json url --jq '.url')
  pr_head=$(gh pr view "$pr_url" --json headRefName --jq '.headRefName')
  pr_state=$(gh pr view "$pr_url" --json state --jq '.state')
  [[ -n "$pr_url" && "$pr_head" == "$implementation_branch" && "$pr_state" == "OPEN" ]] || {
    echo "could not identify an open pull request for new branch $implementation_branch" >&2
    exit 1
  }
  if grep -Fxq "$pr_url" <<<"$prs_before"; then
    echo "implementation pull request is not new: $pr_url" >&2
    exit 1
  fi

  echo "=== Identified branch $implementation_branch and PR $pr_url ==="
  assert_clean_worktree

  fixer_id=$(paseo --quiet run --background \
    --provider "$FIXER_MODEL" \
    --thinking "$FIXER_REASONING" \
    --title "Fix review ${ticket##*/}" \
    "$(cat <<EOF
You are the code-review fixer for:
- implementation ticket: $ticket
- original wayfinder issue: $wayfinder_issue
- pull request: $pr_url

Await instruction. Do not inspect or change code yet. Reviewers will send findings to you. Before receiving an explicit GO message, retain and acknowledge findings without editing. When GO arrives, pragmatically verify all accumulated findings against the ticket and $wayfinder_issue, fix the valid implementation problems on the branch for $pr_url, run focused tests and the full suite once at the end, then commit and push to the existing pull-request branch. Ignore or explain findings that do not identify implementation problems under the specification. On later instructions from the PR-comment agent, apply the same process and push the fixes to $pr_url.

Never ignore errors. Do not create subagents.
EOF
)" )
  wait_and_assert_agent "$fixer_id"
  assert_clean_worktree

  reviewer_1_id=$(paseo --quiet run --background \
    --provider "$CODE_REVIEWER_1_MODEL" \
    --thinking "$CODE_REVIEWER_1_REASONING" \
    --title "Review ${ticket##*/} reviewer 1" \
    "$(cat <<EOF
Review the current implementation and pull request for:
- implementation ticket: $ticket
- original wayfinder issue: $wayfinder_issue
- pull request: $pr_url
- fixed point on the default branch: $default_branch_commit
- fixer agent ID to retain for a later turn: $fixer_id

Read and follow the /code-review skill, but perform its Standards and Spec review axes yourself without creating its subagents. Review exactly the changes since $default_branch_commit. Be pragmatic and follow the specification first. Flag only actual implementation problems; do not flag issue wording, process, style preference, or speculative improvements. Review the completed implementation at $pr_url, not unrelated pre-existing code.

Complete the review now and retain the complete findings, explicitly including a no-findings result. Do not send anything to the fixer during this turn; a later sequential turn will tell you to send the retained review.

Do not create subagents.
EOF
)" )
  assert_clean_worktree

  reviewer_2_id=$(paseo --quiet run --background \
    --provider "$CODE_REVIEWER_2_MODEL" \
    --thinking "$CODE_REVIEWER_2_REASONING" \
    --title "Review ${ticket##*/} reviewer 2" \
    "$(cat <<EOF
Review the current implementation and pull request for:
- implementation ticket: $ticket
- original wayfinder issue: $wayfinder_issue
- pull request: $pr_url
- fixed point on the default branch: $default_branch_commit
- fixer agent ID to retain for a later turn: $fixer_id

Read and follow the /code-review skill, but perform its Standards and Spec review axes yourself without creating its subagents. Review exactly the changes since $default_branch_commit. Be pragmatic and follow the specification first. Flag only actual implementation problems; do not flag issue wording, process, style preference, or speculative improvements. Review the completed implementation at $pr_url, not unrelated pre-existing code.

Complete the review now and retain the complete findings, explicitly including a no-findings result. Do not send anything to the fixer during this turn; a later sequential turn will tell you to send the retained review.

Do not create subagents.
EOF
)" )

  wait_and_assert_agent "$reviewer_1_id"
  wait_and_assert_agent "$reviewer_2_id"

  paseo send --no-wait "$reviewer_1_id" "Send your retained complete review now by running exactly one blocking fixer turn with 'paseo send $fixer_id \"<your retained complete findings, explicitly including no findings>\"'. You must send the message yourself to the retained fixer agent ID. Do not review again or change code. Do not create subagents."
  wait_and_assert_agent "$reviewer_1_id"
  wait_and_assert_agent "$fixer_id"

  paseo send --no-wait "$reviewer_2_id" "Send your retained complete review now by running exactly one blocking fixer turn with 'paseo send $fixer_id \"<your retained complete findings, explicitly including no findings>\"'. You must send the message yourself to the retained fixer agent ID. Do not review again or change code. Do not create subagents."
  wait_and_assert_agent "$reviewer_2_id"
  wait_and_assert_agent "$fixer_id"

  assert_clean_worktree
  paseo send "$fixer_id" "GO. Reconcile both pragmatic review messages, fix every valid implementation problem for $ticket under $wayfinder_issue on $pr_url, run focused tests and the full test suite once at the end, then commit and push the pull-request branch. Do not create subagents."
  wait_and_assert_agent "$fixer_id"
  assert_clean_worktree

  pr_agent_id=$(paseo --quiet run --background \
    --provider "$PR_REVIEWER_MODEL" \
    --thinking "$PR_REVIEWER_REASONING" \
    --title "Triage and merge PR ${ticket##*/}" \
    "$(cat <<EOF
Triage and finish this exact pull request:
- implementation ticket (leaf): $ticket
- original wayfinder issue (issue 55): $wayfinder_issue
- pull request: $pr_url
- fixer agent ID: $fixer_id

Use gh to read all review comments on $pr_url. Perform a pragmatic, specification-first review of the comments themselves. Only implementation problems count.

For each incorrect or out-of-scope comment, reply on GitHub with a concise explanation grounded in the ticket/specification. In each round, consolidate all valid implementation problems into one complete fixer message, reply that they will be fixed, and run exactly one blocking fixer turn with 'paseo send $fixer_id "<complete actionable findings for $pr_url>"'. Then inspect the fixer with 'paseo inspect --json $fixer_id'. Reject the turn unless jq confirms that Status is exactly idle and PendingPermissions is empty; never trust the exit status from paseo send alone. Only after that assertion may you verify the pushed changes on $pr_url. Re-read the review state and repeat at most once, for a maximum of two PR-comment review/fix rounds total. Do not make code changes yourself.

After those rounds, wait for every required check on $pr_url to complete successfully. Never ignore a failed check: if it identifies an implementation problem, run one precise blocking fixer turn with 'paseo send $fixer_id "<required-check failure to fix on $pr_url>"', then run the same inspect/jq idle-and-no-pending-permissions assertion before waiting for the required checks again. Merge $pr_url using a repository-permitted merge method only after all required checks pass, and verify its state is MERGED.

Ensure the leaf implementation ticket $ticket is CLOSED after the merge. If the merge did not close it automatically, first verify the merged work satisfies it, then close it as completed.

Finally, inspect the live GitHub state of the hierarchy, sub-issues/task lists, acceptance tickets, and blockers rooted at $wayfinder_issue. Close only parent or acceptance tickets whose wayfinder closure criteria are now actually met. Do not infer closure from this prompt, do not close tickets still blocked by open work, and do not close the wayfinder/root unless all of its live closure criteria are met.

Never ignore errors. Do not create subagents.
EOF
)" )
  wait_and_assert_agent "$pr_agent_id"
  wait_and_assert_agent "$fixer_id"

  pr_state=$(gh pr view "$pr_url" --json state --jq '.state')
  [[ "$pr_state" == "MERGED" ]] || {
    echo "pull request did not reach MERGED state: $pr_url ($pr_state)" >&2
    exit 1
  }

  leaf_state=$(gh issue view "$ticket" --json state --jq '.state')
  [[ "$leaf_state" == "CLOSED" ]] || {
    echo "implementation leaf did not reach CLOSED state: $ticket ($leaf_state)" >&2
    exit 1
  }

  echo "=== Finished $ticket: $pr_url merged and leaf closed ==="
done
