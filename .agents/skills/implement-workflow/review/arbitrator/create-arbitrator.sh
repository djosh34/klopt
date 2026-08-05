#!/usr/bin/env bash
set -euo pipefail

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../.." && pwd)
root=$1
parent_issue_url=${2%/}
pr_url=${3%/}
review_base_sha=$4
artifact_dir=$5
shared_branch=$6

prompt=$(cat "$script_dir/prompt.txt")
prompt+=$'\n\nRUNTIME VALUES\n'
prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
prompt+="PR_URL=$pr_url"$'\n'
prompt+="FIXED_REVIEW_BASE_SHA=$review_base_sha"$'\n'
prompt+="ARTIFACT_DIR=$artifact_dir"$'\n'
prompt+="SHARED_BRANCH=$shared_branch"$'\n'
prompt+="SKILL_DIR=$skill_dir"$'\n'

agent_id=$(paseo -q run -d \
  --provider pi/openai-codex/gpt-5.6-sol \
  --thinking high \
  --title "Larger Issue $root: Review Arbitrator" \
  "$prompt")

printf 'AGENT_ID=%s\n' "$agent_id"
