---
description: Walk open Dependabot PRs, check CI, and merge green ones (with confirmation)
---

Sweep open Dependabot PRs in this repository.

Steps:

1. List open PRs from `dependabot[bot]`:
   ```
   gh pr list --author 'app/dependabot' --state open --json number,title,headRefName,createdAt
   ```
   (Fall back to `--author dependabot[bot]` if the first form returns empty.)

2. For each PR, fetch CI status:
   ```
   gh pr checks <num> --json name,state,conclusion
   ```

3. Bucket the PRs:
   - **🟢 GREEN** — every required check `SUCCESS`
   - **🟡 PENDING** — any check `IN_PROGRESS` / `QUEUED`
   - **🔴 RED** — any check `FAILURE` / `TIMED_OUT`

4. Print a compact table: `#num  title  bucket  notes`.

5. For the GREEN bucket, ask me (use AskUserQuestion with multiSelect) which PRs to merge. Then run:
   ```
   gh pr merge --squash --auto <num>
   ```
   for each selected PR.

6. For RED PRs, paste the failing check name and the first failing log line so I can decide whether to investigate.

7. For PENDING PRs, just list them — do nothing.

**Hard rules:**
- Never merge without my explicit confirmation per PR.
- Never close or rebase PRs without asking.
- If a PR title contains `major` version bumps, flag it as 🔶 RISKY even if green and require an extra confirmation step.
