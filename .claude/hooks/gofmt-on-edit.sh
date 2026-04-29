#!/usr/bin/env bash
# Auto-format Go files after Edit/Write/MultiEdit.
# Runs gofmt inside the same golang container the Makefile uses.
# Silent on success; logs to stderr on real failures.
set -euo pipefail

input=$(cat)
file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')

[[ -n "$file" ]] || exit 0
[[ "$file" == *.go ]] || exit 0
[[ -f "$file" ]] || exit 0
# Skip agent-spawned worktrees — they're disposable copies, not real edits.
case "$file" in
  */.stromboli-worktrees/*) exit 0 ;;
esac

# Skip generated files (gofmt would touch them and create churn).
case "$(basename "$file")" in
  *_gen.go|zz_generated_*.go|*.pb.go) exit 0 ;;
esac

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
rel=$(realpath --relative-to="$repo_root" "$file")

if ! command -v podman >/dev/null 2>&1; then
  exit 0
fi

# If golang:1.24 isn't pulled yet, do nothing rather than block on a 500MB pull.
if ! podman image exists golang:1.24 2>/dev/null; then
  echo "[gofmt-hook] golang:1.24 not pulled; skipping. Run 'podman pull golang:1.24' to enable." >&2
  exit 0
fi

# 10s ceiling so a stuck container can't hang the edit pipeline.
timeout 10 podman run --rm --userns=keep-id \
  -v "$repo_root:/app" -w /app \
  golang:1.24 gofmt -w "$rel" 2>/dev/null || {
    echo "[gofmt-hook] gofmt failed or timed out for $rel" >&2
    exit 0
  }
