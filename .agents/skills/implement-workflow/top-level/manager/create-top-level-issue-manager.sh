#!/usr/bin/env bash
set -euo pipefail

if (($# != 2)); then
  printf 'usage: %s ROOT_ISSUE_URL PARENT_ISSUE_URL\n' "$0" >&2
  exit 2
fi

script_dir=$(builtin cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(builtin cd -- "$script_dir/../.." && pwd)
root_issue_url=${1%/}
parent_issue_url=${2%/}

parse_issue_url() {
  local url=$1
  if [[ $url =~ ^https://github\.com/([^/]+)/([^/]+)/issues/([0-9]+)$ ]]; then
    printf '%s\n%s\n%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
    return
  fi
  printf 'invalid GitHub issue URL: %s\n' "$url" >&2
  return 1
}

require_issue() {
  local owner=$1
  local repo=$2
  local number=$3
  local issue_json
  issue_json=$(gh api "repos/$owner/$repo/issues/$number")
  if ! jq -e 'has("pull_request") | not' <<<"$issue_json" >/dev/null; then
    printf 'URL identifies a pull request, not an issue: https://github.com/%s/%s/issues/%s\n' \
      "$owner" "$repo" "$number" >&2
    return 1
  fi
}

root_parts_text=$(parse_issue_url "$root_issue_url")
parent_parts_text=$(parse_issue_url "$parent_issue_url")
mapfile -t root_parts <<<"$root_parts_text"
mapfile -t parent_parts <<<"$parent_parts_text"
root_owner=${root_parts[0]}
root_repo=${root_parts[1]}
root=${root_parts[2]}
parent_owner=${parent_parts[0]}
parent_repo=${parent_parts[1]}
parent=${parent_parts[2]}

if [[ $root_owner/$root_repo != "$parent_owner/$parent_repo" ]]; then
  printf 'root and parent issues must belong to the same repository\n' >&2
  exit 1
fi

require_issue "$root_owner" "$root_repo" "$root"
require_issue "$parent_owner" "$parent_repo" "$parent"

if [[ $root != "$parent" ]]; then
  declare -a queue=("$root")
  declare -A seen=()
  found=false

  while ((${#queue[@]} > 0)); do
    current=${queue[0]}
    queue=("${queue[@]:1}")
    [[ ${seen[$current]+set} ]] && continue
    seen[$current]=1

    children_json=$(gh api --paginate --slurp \
      "repos/$root_owner/$root_repo/issues/$current/sub_issues?per_page=100")
    children_text=$(jq -r '.[][] | .number' <<<"$children_json")

    while IFS= read -r child; do
      [[ -z $child ]] && continue
      if [[ $child == "$parent" ]]; then
        found=true
        break 2
      fi
      queue+=("$child")
    done <<<"$children_text"
  done

  if [[ $found != true ]]; then
    printf 'parent issue %s is not the root or a transitive subissue of %s\n' \
      "$parent_issue_url" "$root_issue_url" >&2
    exit 1
  fi
fi

workspace_json=$(paseo workspace create --isolation local --title "Issue $root->$parent" --json)
workspace_id=$(printf '%s\n' "$workspace_json" | jq -er '.workspaceId')
prompt=$(<"$script_dir/prompt.txt")
prompt+=$'\n\nRUNTIME VALUES\n'
prompt+="ROOT_ISSUE_URL=$root_issue_url"$'\n'
prompt+="ROOT_ISSUE_NUMBER=$root"$'\n'
prompt+="PARENT_ISSUE_URL=$parent_issue_url"$'\n'
prompt+="PARENT_ISSUE_NUMBER=$parent"$'\n'
prompt+="SKILL_DIR=$skill_dir"$'\n'
prompt+="TOP_WORKSPACE_ID=$workspace_id"$'\n'

agent_id=$(paseo -q run -d \
  --provider pi/openrouter/deepseek/deepseek-v4-flash-0731 \
  --thinking high \
  --title "Issue $root->$parent" \
  --workspace "$workspace_id" \
  "$prompt")

printf 'WORKSPACE_ID=%s\n' "$workspace_id"
printf 'AGENT_ID=%s\n' "$agent_id"
