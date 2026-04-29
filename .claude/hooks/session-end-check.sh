#!/usr/bin/env bash
# Stop hook: when the session ends with modified Go files, run `go vet` on the
# affected packages and remind to run tests. Surfaces issues once at the end
# instead of slowing every edit with a containerised vet.
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

# Files changed since last commit (staged + unstaged + untracked).
# Exclude .stromboli-worktrees/ — those are agent-spawned worktrees, not real edits.
changed_go=$(
  {
    git diff --name-only HEAD 2>/dev/null
    git ls-files --others --exclude-standard 2>/dev/null
  } | grep -E '\.go$' | grep -vE '^\.stromboli-worktrees/' | sort -u || true
)

[[ -n "$changed_go" ]] || exit 0

# Reduce to unique package directories.
pkgs=$(printf '%s\n' "$changed_go" | xargs -n1 dirname | sort -u | sed 's|^|./|')

echo "" >&2
echo "📝 Go files changed this session:" >&2
printf '%s\n' "$changed_go" | sed 's/^/   - /' >&2
echo "" >&2

if command -v podman >/dev/null 2>&1 && podman image exists golang:1.24 2>/dev/null; then
  echo "🔍 Running go vet on changed packages..." >&2
  vet_out=$(timeout 60 podman run --rm --userns=keep-id \
    -v "$repo_root:/app" -w /app \
    golang:1.24 go vet $pkgs 2>&1) || true
  if [[ -n "$vet_out" ]]; then
    echo "$vet_out" | sed 's/^/   /' >&2
  else
    echo "   ✅ no vet issues" >&2
  fi
else
  echo "🔍 Skipping go vet (golang:1.24 image not pulled)." >&2
fi

echo "" >&2
echo "💡 Reminder: run 'make test' before pushing." >&2
