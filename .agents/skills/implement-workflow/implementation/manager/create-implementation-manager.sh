#!/usr/bin/env bash
set -euo pipefail

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../.." && pwd)
root=$1
parent_issue_url=${2%/}
work_issue_url=${3%/}
shared_branch=$4
work=${work_issue_url##*/}

workspace_json=$(paseo workspace create --isolation local --title "Issue $root->$work" --json)
workspace_id=$(printf '%s\n' "$workspace_json" | jq -er '.workspaceId')
prompt=$(cat "$script_dir/prompt.txt")
prompt+=$'\n\nRUNTIME VALUES\n'
prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
prompt+="WORK_ISSUE_URL=$work_issue_url"$'\n'
prompt+="WORK_ISSUE_NUMBER=$work"$'\n'
prompt+="SHARED_BRANCH=$shared_branch"$'\n'
prompt+="SKILL_DIR=$skill_dir"$'\n'
prompt+="WORKSPACE_ID=$workspace_id"$'\n'

agent_id=$(paseo -q run -d \
  --provider openrouter/deepseek/deepseek-v4-flash-0731 \
  --thinking high \
  --title "Issue $root->$work: Implementation Manager" \
  --workspace "$workspace_id" \
  "$prompt")

printf 'WORKSPACE_ID=%s\n' "$workspace_id"
printf 'AGENT_ID=%s\n' "$agent_id"
