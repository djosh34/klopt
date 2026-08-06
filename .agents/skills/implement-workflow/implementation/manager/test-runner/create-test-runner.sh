#!/usr/bin/env bash
set -euo pipefail

if (($# != 5)); then
  printf 'usage: %s ROOT_ISSUE_URL PARENT_ISSUE_URL CONTEXT_ISSUE_URL SHARED_BRANCH TITLE\n' "$0" >&2
  exit 2
fi
for issue_url in "$1" "$2" "$3"; do
  [[ ${issue_url%/} =~ ^https://github\.com/[^/]+/[^/]+/issues/[0-9]+$ ]] || {
    printf 'invalid GitHub issue URL: %s\n' "$issue_url" >&2
    exit 2
  }
done

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../../../" && pwd)
root_issue_url=${1%/}
parent_issue_url=${2%/}
context_issue_url=${3%/}
shared_branch=$4
title=$5
root=${root_issue_url##*/}
parent=${parent_issue_url##*/}

prompt=$(<"$script_dir/prompt.txt")
prompt+=$'\n\nRUNTIME VALUES\n'
prompt+="ROOT_ISSUE_URL=$root_issue_url"$'\n'
prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
prompt+="PARENT_ISSUE_NUMBER=$parent"$'\n'
prompt+="CONTEXT_ISSUE_URL=$context_issue_url"$'\n'
prompt+="SHARED_BRANCH=$shared_branch"$'\n'
prompt+="SKILL_DIR=$skill_dir"$'\n'

agent_id=$(paseo -q run -d \
  --provider pi/openai-codex/gpt-5.6-luna \
  --thinking high \
  --title "$title" \
  "$prompt")

printf 'TEST_RUNNER_AGENT_ID=%s\n' "$agent_id"
