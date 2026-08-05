#!/usr/bin/env bash
set -euo pipefail

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../.." && pwd)
root=$1
parent_issue_url=${2%/}
pr_url=${3%/}
review_base_sha=$4
cycle=$5

reviewer_1_prompt=$(cat "$script_dir/reviewer-1.txt")
reviewer_1_prompt+=$'\n\nRUNTIME VALUES\n'
reviewer_1_prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
reviewer_1_prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
reviewer_1_prompt+="PR_URL=$pr_url"$'\n'
reviewer_1_prompt+="FIXED_REVIEW_BASE_SHA=$review_base_sha"$'\n'
reviewer_1_prompt+="CYCLE=$cycle"$'\n'
reviewer_1_prompt+="SKILL_DIR=$skill_dir"$'\n'

reviewer_2_prompt=$(cat "$script_dir/reviewer-2.txt")
reviewer_2_prompt+=$'\n\nRUNTIME VALUES\n'
reviewer_2_prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
reviewer_2_prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
reviewer_2_prompt+="PR_URL=$pr_url"$'\n'
reviewer_2_prompt+="FIXED_REVIEW_BASE_SHA=$review_base_sha"$'\n'
reviewer_2_prompt+="CYCLE=$cycle"$'\n'
reviewer_2_prompt+="SKILL_DIR=$skill_dir"$'\n'

reviewer_1_id=$(paseo -q run -d \
  --provider pi/openai-codex/gpt-5.6-sol \
  --thinking high \
  --title "Larger Issue $root: Code-review-1" \
  "$reviewer_1_prompt")
reviewer_2_id=$(paseo -q run -d \
  --provider pi/openai-codex/gpt-5.6-sol \
  --thinking high \
  --title "Larger Issue $root: Code-review-2" \
  "$reviewer_2_prompt")

printf 'REVIEWER_1_AGENT_ID=%s\n' "$reviewer_1_id"
printf 'REVIEWER_2_AGENT_ID=%s\n' "$reviewer_2_id"
