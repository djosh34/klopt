#!/usr/bin/env bash
set -euo pipefail

if (($# != 6)); then
  printf 'usage: %s ROOT_ISSUE_URL PARENT_ISSUE_URL ISSUE_GRAPH_PATH PR_URL FIXED_REVIEW_BASE_SHA CYCLE\n' "$0" >&2
  exit 2
fi
for issue_url in "$1" "$2"; do
  [[ ${issue_url%/} =~ ^https://github\.com/[^/]+/[^/]+/issues/[0-9]+$ ]] || {
    printf 'invalid GitHub issue URL: %s\n' "$issue_url" >&2
    exit 2
  }
done
[[ ${4%/} =~ ^https://github\.com/[^/]+/[^/]+/pull/[0-9]+$ ]] || {
  printf 'invalid GitHub pull-request URL: %s\n' "$4" >&2
  exit 2
}
[[ -f $3 ]] || {
  printf 'issue graph file does not exist: %s\n' "$3" >&2
  exit 2
}

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../.." && pwd)
root_issue_url=${1%/}
parent_issue_url=${2%/}
issue_graph_path=$3
pr_url=${4%/}
review_base_sha=$5
cycle=$6
root=${root_issue_url##*/}
parent=${parent_issue_url##*/}

prompt=$(<"$script_dir/architecture.txt")
prompt+=$'\n\nRUNTIME VALUES\n'
prompt+="ROOT_ISSUE_URL=$root_issue_url"$'\n'
prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
prompt+="PARENT_ISSUE_NUMBER=$parent"$'\n'
prompt+="ISSUE_GRAPH_PATH=$issue_graph_path"$'\n'
prompt+="REVIEWER_MARKER=code-review-architecture"$'\n'
prompt+="PR_URL=$pr_url"$'\n'
prompt+="FIXED_REVIEW_BASE_SHA=$review_base_sha"$'\n'
prompt+="CYCLE=$cycle"$'\n'
prompt+="SKILL_DIR=$skill_dir"$'\n'

agent_id=$(paseo -q run -d \
  --provider pi/openai-codex/gpt-5.6-sol \
  --thinking high \
  --title "Issue $root->$parent: Architecture Review" \
  "$prompt")

printf 'ARCHITECTURE_REVIEWER_MARKER=code-review-architecture\n'
printf 'ARCHITECTURE_REVIEWER_AGENT_ID=%s\n' "$agent_id"
