#!/usr/bin/env bash
set -euo pipefail

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../../../" && pwd)
root=$1
context_issue_url=${2%/}
shared_branch=$3
title=$4

prompt=$(cat "$script_dir/prompt.txt")
prompt+=$'\n\nRUNTIME VALUES\n'
prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
prompt+="CONTEXT_ISSUE_URL=$context_issue_url"$'\n'
prompt+="SHARED_BRANCH=$shared_branch"$'\n'
prompt+="SKILL_DIR=$skill_dir"$'\n'

agent_id=$(paseo -q run -d \
  --provider pi/openai-codex/gpt-5.6-luna \
  --thinking high \
  --title "$title" \
  "$prompt")

printf 'AGENT_ID=%s\n' "$agent_id"
