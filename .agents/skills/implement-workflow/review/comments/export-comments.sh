#!/usr/bin/env bash
set -euo pipefail

pr_url=${1%/}
output_path=$2
pr_path=${pr_url#https://github.com/}
owner=${pr_path%%/*}
pr_path=${pr_path#*/}
repo=${pr_path%%/*}
pr_number=${pr_url##*/}

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

gh api --paginate --slurp \
  "repos/$owner/$repo/issues/$pr_number/comments?per_page=100" |
  jq '[.[][] | {
    id,
    kind: "issue-comment",
    author: (.user.login // null),
    author_type: (.user.type // null),
    author_association: (.author_association // null),
    body: (.body // ""),
    url: (.html_url // .url // null),
    created_at: (.created_at // null),
    updated_at: (.updated_at // null),
    reply_to_id: (.in_reply_to_id // null),
    node_id: (.node_id // null)
  }]' >"$tmp_dir/issue-comments.json"

gh api --paginate --slurp \
  "repos/$owner/$repo/pulls/$pr_number/reviews?per_page=100" |
  jq '[.[][] | {
    id,
    kind: "formal-review",
    author: (.user.login // null),
    author_type: (.user.type // null),
    author_association: (.author_association // null),
    body: (.body // ""),
    url: (.html_url // .url // null),
    created_at: (.created_at // null),
    updated_at: (.updated_at // null),
    submitted_at: (.submitted_at // null),
    reply_to_id: null,
    review_id: .id,
    state: (.state // null),
    commit_id: (.commit_id // null),
    node_id: (.node_id // null)
  }]' >"$tmp_dir/formal-reviews.json"

gh api --paginate --slurp \
  "repos/$owner/$repo/pulls/$pr_number/comments?per_page=100" |
  jq '[.[][] | {
    id,
    kind: "inline-review-comment",
    author: (.user.login // null),
    author_type: (.user.type // null),
    author_association: (.author_association // null),
    body: (.body // ""),
    url: (.html_url // .url // null),
    created_at: (.created_at // null),
    updated_at: (.updated_at // null),
    reply_to_id: (.in_reply_to_id // null),
    review_id: (.pull_request_review_id // null),
    path: (.path // null),
    line: (.line // null),
    start_line: (.start_line // null),
    side: (.side // null),
    start_side: (.start_side // null),
    original_line: (.original_line // null),
    original_start_line: (.original_start_line // null),
    commit_id: (.commit_id // null),
    diff_hunk: (.diff_hunk // null),
    node_id: (.node_id // null)
  }]' >"$tmp_dir/inline-comments.json"

jq -s 'add' \
  "$tmp_dir/issue-comments.json" \
  "$tmp_dir/formal-reviews.json" \
  "$tmp_dir/inline-comments.json" >"$output_path"
