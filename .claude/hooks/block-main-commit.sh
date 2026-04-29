#!/usr/bin/env bash
# Block direct commits to main/master. Force feature branch + PR flow.
# Exit code 2 cancels the tool call and feeds the message back to Claude.
set -euo pipefail

input=$(cat)
cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // empty')

[[ -n "$cmd" ]] || exit 0

# Match `git commit ...` but not `git commit-tree`, `git diff --commit`, etc.
if ! printf '%s' "$cmd" | grep -qE '(^|[[:space:]]|;|&&|\|\|)git[[:space:]]+commit([[:space:]]|$)'; then
  exit 0
fi

# Skip if user is just asking for help or doing a dry-run.
if printf '%s' "$cmd" | grep -qE -- '--dry-run|--help|-h(\b|$)'; then
  exit 0
fi

branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
case "$branch" in
  main|master)
    cat >&2 <<EOF
BLOCKED: direct commit to '$branch' is not allowed.

Create a feature branch and open a PR instead:
  git checkout -b <type>/<short-description>
  # ...stage and commit there...
  gh pr create

If this is intentional (e.g. emergency hotfix), bypass by committing from a different branch and merging via PR, or temporarily disable this hook.
EOF
    exit 2
    ;;
esac
