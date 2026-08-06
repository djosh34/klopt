#!/usr/bin/env bash
set -euo pipefail

if (($# != 4)); then
  printf 'usage: %s ROOT_ISSUE_URL PARENT_ISSUE_URL WORK_ISSUE_URL SHARED_BRANCH\n' "$0" >&2
  exit 2
fi
for issue_url in "$1" "$2" "$3"; do
  [[ ${issue_url%/} =~ ^https://github\.com/[^/]+/[^/]+/issues/[0-9]+$ ]] || {
    printf 'invalid GitHub issue URL: %s\n' "$issue_url" >&2
    exit 2
  }
done

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../.." && pwd)
root_issue_url=${1%/}
parent_issue_url=${2%/}
work_issue_url=${3%/}
shared_branch=$4
root=${root_issue_url##*/}
parent=${parent_issue_url##*/}
work=${work_issue_url##*/}

workspace_json=$(paseo workspace create --isolation local --title "Issue $root->$parent->$work" --json)
workspace_id=$(printf '%s\n' "$workspace_json" | jq -er '.workspaceId')
prompt=$(<"$script_dir/prompt.txt")
prompt+=$'\n\nRUNTIME VALUES\n'
prompt+="ROOT_ISSUE_URL=$root_issue_url"$'\n'
prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
prompt+="PARENT_ISSUE_NUMBER=$parent"$'\n'
prompt+="WORK_ISSUE_URL=$work_issue_url"$'\n'
prompt+="WORK_ISSUE_NUMBER=$work"$'\n'
prompt+="SHARED_BRANCH=$shared_branch"$'\n'
prompt+="SKILL_DIR=$skill_dir"$'\n'
prompt+="WORKSPACE_ID=$workspace_id"$'\n'

agent_id=$(paseo -q run -d \
  --provider pi/openrouter/deepseek/deepseek-v4-flash-0731 \
  --thinking high \
  --title "Issue $root->$parent->$work: Implementation Manager" \
  --workspace "$workspace_id" \
  "$prompt")

printf 'WORKSPACE_ID=%s\n' "$workspace_id"
printf 'AGENT_ID=%s\n' "$agent_id"
