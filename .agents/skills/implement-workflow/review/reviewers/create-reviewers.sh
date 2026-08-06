#!/usr/bin/env bash
set -euo pipefail

if (($# != 7)); then
  printf 'usage: %s ROOT_ISSUE_URL PARENT_ISSUE_URL FOCUS_ISSUE_URL ISSUE_GRAPH_PATH PR_URL FIXED_REVIEW_BASE_SHA CYCLE\n' "$0" >&2
  exit 2
fi
for issue_url in "$1" "$2" "$3"; do
  [[ ${issue_url%/} =~ ^https://github\.com/[^/]+/[^/]+/issues/[0-9]+$ ]] || {
    printf 'invalid GitHub issue URL: %s\n' "$issue_url" >&2
    exit 2
  }
done
[[ ${5%/} =~ ^https://github\.com/[^/]+/[^/]+/pull/[0-9]+$ ]] || {
  printf 'invalid GitHub pull-request URL: %s\n' "$5" >&2
  exit 2
}
[[ -f $4 ]] || {
  printf 'issue graph file does not exist: %s\n' "$4" >&2
  exit 2
}

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../.." && pwd)
root_issue_url=${1%/}
parent_issue_url=${2%/}
focus_issue_url=${3%/}
issue_graph_path=$4
pr_url=${5%/}
review_base_sha=$6
cycle=$7
root=${root_issue_url##*/}
parent=${parent_issue_url##*/}
focus=${focus_issue_url##*/}

launch_reviewer() {
  local slot=$1
  local marker="code-review-$focus-$slot"
  local prompt
  prompt=$(<"$script_dir/prompt.txt")
  prompt+=$'\n\nRUNTIME VALUES\n'
  prompt+="ROOT_ISSUE_URL=$root_issue_url"$'\n'
  prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
  prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
  prompt+="PARENT_ISSUE_NUMBER=$parent"$'\n'
  prompt+="FOCUS_ISSUE_URL=$focus_issue_url"$'\n'
  prompt+="FOCUS_ISSUE_NUMBER=$focus"$'\n'
  prompt+="ISSUE_GRAPH_PATH=$issue_graph_path"$'\n'
  prompt+="REVIEWER_MARKER=$marker"$'\n'
  prompt+="PR_URL=$pr_url"$'\n'
  prompt+="FIXED_REVIEW_BASE_SHA=$review_base_sha"$'\n'
  prompt+="CYCLE=$cycle"$'\n'
  prompt+="SKILL_DIR=$skill_dir"$'\n'

  paseo -q run -d \
    --provider pi/openai-codex/gpt-5.6-sol \
    --thinking high \
    --title "Issue $root->$parent: Review $focus-$slot" \
    "$prompt"
}

reviewer_1_id=$(launch_reviewer 1)
reviewer_2_id=$(launch_reviewer 2)

printf 'FOCUS_ISSUE_URL=%s\n' "$focus_issue_url"
printf 'REVIEWER_1_MARKER=code-review-%s-1\n' "$focus"
printf 'REVIEWER_1_AGENT_ID=%s\n' "$reviewer_1_id"
printf 'REVIEWER_2_MARKER=code-review-%s-2\n' "$focus"
printf 'REVIEWER_2_AGENT_ID=%s\n' "$reviewer_2_id"
