#!/usr/bin/env bash
set -euo pipefail

pr_url=${1%/}
output_path=$2
pr_path=${pr_url#https://github.com/}
owner=${pr_path%%/*}
pr_path=${pr_path#*/}
repo=${pr_path%%/*}
pr_number=${pr_url##*/}

issue_comments=$(gh api --paginate --slurp \
  "repos/$owner/$repo/issues/$pr_number/comments?per_page=100" |
  jq -c '[.[][] | {
    id,
    kind: "issue-comment",
    author: (.user.login // null),
    body: (.body // ""),
    url: (.html_url // .url // null),
    created_at: (.created_at // null),
    updated_at: (.updated_at // null),
    reply_to_id: (.in_reply_to_id // null),
    node_id: (.node_id // null)
  }]')
formal_reviews=$(gh api --paginate --slurp \
  "repos/$owner/$repo/pulls/$pr_number/reviews?per_page=100" |
  jq -c '[.[][] | {
    id,
    kind: "formal-review",
    author: (.user.login // null),
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
  }]')
inline_comments=$(gh api --paginate --slurp \
  "repos/$owner/$repo/pulls/$pr_number/comments?per_page=100" |
  jq -c '[.[][] | {
    id,
    kind: "inline-review-comment",
    author: (.user.login // null),
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
  }]')

jq -n \
  --argjson issue_comments "$issue_comments" \
  --argjson formal_reviews "$formal_reviews" \
  --argjson inline_comments "$inline_comments" \
  '$issue_comments + $formal_reviews + $inline_comments' >"$output_path"
