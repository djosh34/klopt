#!/usr/bin/env bash
set -euo pipefail

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../.." && pwd)
parent_issue_url=${1%/}
root=${parent_issue_url##*/}

workspace_json=$(paseo workspace create --isolation local --title "Larger Issue $root" --json)
workspace_id=$(printf '%s\n' "$workspace_json" | jq -er '.workspaceId')
prompt=$(cat "$script_dir/prompt.txt")
prompt+=$'\n\nRUNTIME VALUES\n'
prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
prompt+="SKILL_DIR=$skill_dir"$'\n'
prompt+="TOP_WORKSPACE_ID=$workspace_id"$'\n'

agent_id=$(paseo -q run -d \
  --provider pi/openrouter/deepseek/deepseek-v4-flash-0731 \
  --thinking high \
  --title "Larger Issue $root" \
  --workspace "$workspace_id" \
  "$prompt")

printf 'WORKSPACE_ID=%s\n' "$workspace_id"
printf 'AGENT_ID=%s\n' "$agent_id"
