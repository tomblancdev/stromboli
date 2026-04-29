---
description: Run Stromboli tests via Make (unit by default; pass `integration`, `e2e`, `all`, or `coverage`)
---

Run the requested Stromboli test suite and report results concisely.

Argument handling (`$ARGUMENTS`):
- empty or `unit` → `make test`
- `integration` → `make test-integration`
- `e2e` → `make test-e2e`
- `all` → `make test-all`
- `coverage` → `make test-coverage` (then mention the path to `coverage.html`)

After the command finishes:
1. Report pass/fail counts and the elapsed time.
2. On failure, list the failing tests with `package.TestName` and the first error line for each. Show file:line.
3. Do **not** dump the full test output unless I ask — keep the summary tight.
4. If failures look like flakes (timeouts, podman-related, network), call that out and suggest re-running.
