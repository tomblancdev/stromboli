# Performance & Cost Tuning

Stromboli passes through a handful of Claude CLI levers that change how requests are billed and how fast they return. They live under `claude.*` on `RunRequest` and `agent.CreateRequest` and most map to environment variables the Claude CLI reads internally. This guide groups them by intent.

## Track what you're spending

Every `/run` response carries a `usage` block:

```json
{
  "id": "run-abc",
  "status": "completed",
  "output": "...",
  "session_id": "550e...",
  "usage": {
    "model": "claude-sonnet-4-6",
    "input_tokens": 1234,
    "output_tokens": 567,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 8901,
    "total_tokens": 10702,
    "estimated_cost_usd": 0.0142
  }
}
```

`estimated_cost_usd` is computed best-effort from the per-model price table. `cache_read_input_tokens` is what you saved by hitting Claude's prompt cache (priced at ~10% of fresh input).

Async jobs surface the same field on `GET /jobs/{id}.usage`, populated when the job completes. Persistent agents don't return per-turn usage on `/send` — read the session JSONL instead via `GET /sessions/{session_id}/messages`.

!!! tip "Hard budget caps"
    Set `claude.max_budget_usd` to short-circuit a run when projected cost exceeds the cap. Useful when you don't trust a recursive agent loop to terminate.

## Effort levels

`claude.effort` sets how aggressively the model thinks before responding. Valid values per the upstream CLI: `low`, `medium`, `high`, `xhigh`, `max`. The accepted subset depends on the model — Stromboli passes through; Claude rejects unsupported levels at runtime.

```json
{
  "prompt": "Refactor this distributed lock for correctness under network partitions",
  "claude": {"model": "sonnet", "effort": "high"}
}
```

Higher effort ⇒ more thinking tokens ⇒ slower + more expensive. For routine code edits, `medium` is plenty. Reserve `high`/`xhigh` for design-level reasoning, multi-step refactors, or correctness-critical work where you'd rather the model think twice.

## Prompt caching TTL

When the same context (system prompt, repo state, long instructions) gets re-used across many turns, Claude's prompt cache reads at ~10% the price of fresh input. The default cache TTL is short; for long-lived agents that re-use the same context for hours, extend it:

```json
{"claude": {"prompt_caching_ttl": "1h"}}
```

Mapping:

| Value | Effect | Env var set |
|---|---|---|
| `""` | Use Claude's default | _none_ |
| `"5m"` | Force the 5-minute cache | `FORCE_PROMPT_CACHING_5M=1` |
| `"1h"` | Enable the 1-hour cache | `ENABLE_PROMPT_CACHING_1H=1` |

Best fit:

- **Persistent agents** with a long system prompt that stays constant → `1h` is a no-brainer.
- **Synchronous `/run`** calls that share a system prompt across many requests in the same hour → `1h` and the second-onward call hits cache.
- **Single one-shot runs** → leave default; the cache won't pay off in a single turn.

## Bedrock service tier

When running against AWS Bedrock as the model backend, `claude.bedrock_service_tier` chooses the throughput tier:

| Value | Use case |
|---|---|
| `default` | Standard rate limits |
| `flex` | Lower priority, lower per-token rate |
| `priority` | Higher priority, premium rate |

```json
{"claude": {"bedrock_service_tier": "flex"}}
```

Maps to `ANTHROPIC_BEDROCK_SERVICE_TIER`. Ignored when not on Bedrock — safe to set unconditionally if your fleet is mixed.

## Native PowerShell on Windows agent hosts

If the agent container's host OS is Windows and you want Claude to use the native PowerShell tool instead of a POSIX shell:

```json
{"claude": {"enable_powershell_tool": true}}
```

Sets `CLAUDE_CODE_USE_POWERSHELL_TOOL=1`. No-op on Linux/macOS containers. Most Stromboli operators run Linux agent images; this matters mainly for Windows-targeted automation work.

## Combining for lowest cost

A "cheap-but-good-enough" persistent agent template:

```json
{
  "prompt": "...long stable system prompt...",
  "idle_timeout_seconds": 3600,
  "claude": {
    "model": "sonnet",
    "effort": "medium",
    "prompt_caching_ttl": "1h",
    "max_budget_usd": 5.00
  }
}
```

What this buys you:

- One spawn cost amortized over hours of activity (persistent agent)
- 1-hour prompt cache means the long system prompt costs full price once, then ~10% per turn
- `medium` effort keeps thinking tokens bounded
- `max_budget_usd: 5` is the safety net if a turn loops

For one-shot review jobs, keep it simple — just `model` + `effort: medium` and let the rest default.

## What to read next

- [API endpoints reference](../api/endpoints.md) — every `claude.*` field, all in one place
- [Persistent agents](persistent-agents.md) — when to keep an agent warm vs. one-shot
- [Configuration](../getting-started/configuration.md) — env vars that override per-request defaults at the deployment level
