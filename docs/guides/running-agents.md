# Running Agents

Everything you need to spawn Claude agents through Stromboli's API.

## Basic usage

Send a prompt, get a response:

```bash
curl -X POST localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello, Claude!"}'
```

```json
{
  "id": "run-abc123",
  "status": "completed",
  "output": "Hello! How can I help?",
  "session_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## Mount a project directory

Give Claude access to your code:

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Analyze the code structure",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"]
    }
  }'
```

- `workdir` — working directory inside the container
- `volumes` — mount host paths into the container (`host:container[:options]`)

Use `:ro` for read-only access when the agent only needs to analyze, not modify:

```json
{"podman": {"volumes": ["/home/user/code:/workspace:ro"]}}
```

!!! warning "Volume allowlist"
    In production, only paths listed in `STROMBOLI_AGENT_ALLOWED_VOLUMES` can be mounted. See [security overview](../security/overview.md).

## Claude options

Control Claude's behavior per-request:

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Review this code",
    "claude": {
      "model": "sonnet",
      "system_prompt": "You are a senior Go developer",
      "max_budget_usd": 1.00,
      "allowed_tools": ["Read", "Grep", "Glob"],
      "output_format": "json"
    }
  }'
```

| Option | Type | Description |
|---|---|---|
| `model` | string | `sonnet`, `opus`, `haiku` |
| `system_prompt` | string | Replace the default system prompt |
| `append_system_prompt` | string | Append to the system prompt |
| `max_budget_usd` | float | Max API cost for this run |
| `allowed_tools` | []string | Whitelist specific tools |
| `disallowed_tools` | []string | Blacklist specific tools |
| `output_format` | string | `text`, `json`, `stream-json` |
| `dangerously_skip_permissions` | bool | Skip permission prompts |

## Container options

Set resource limits and timeouts:

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Run heavy analysis",
    "podman": {
      "memory": "2g",
      "cpus": "2",
      "timeout": "30m"
    }
  }'
```

| Option | Type | Description |
|---|---|---|
| `memory` | string | Memory limit (`512m`, `2g`) |
| `cpus` | string | CPU limit (`0.5`, `2`) |
| `timeout` | string | Max runtime (`5m`, `1h`) |
| `image` | string | [Custom container image](custom-images.md) |
| `volumes` | []string | Volume mounts (`host:container[:options]`) |
| `secrets_env` | map | [Secrets](secrets.md) as env vars |

## Async execution

For long-running tasks, use async mode to avoid HTTP timeouts:

```bash
# Start the job
curl -X POST localhost:8080/run/async \
  -d '{"prompt": "Refactor the entire codebase", "workdir": "/workspace"}'
# {"job_id": "job-abc123", "status": "pending"}

# Check status
curl localhost:8080/jobs/job-abc123
```

Add a `webhook_url` to get notified when the job completes:

```bash
curl -X POST localhost:8080/run/async \
  -d '{
    "prompt": "Long running task",
    "webhook_url": "https://your-server.com/webhook"
  }'
```

## Streaming output

Get real-time output via Server-Sent Events:

```bash
curl -N "localhost:8080/run/stream?prompt=Hello&workdir=/workspace"
```

```
data: {"type":"output","content":"Hello! I'm analyzing..."}
data: {"type":"output","content":"Found 3 files..."}
data: {"type":"done","session_id":"..."}
```

## Working directory

The `workdir` parameter sets where Claude starts inside the container. If the path doesn't exist, Stromboli creates it automatically (configurable via `STROMBOLI_AGENT_WORKDIR_AUTO_CREATE`).

Tips:

- Use absolute paths (`/workspace`, not `./workspace`)
- Make sure `workdir` matches where your volume is mounted
- Mount specific project directories, not entire home folders

## What's next

- [Sessions](sessions.md) — Resume conversations across requests
- [Secrets](secrets.md) — Inject tokens for GitHub, GitLab, etc.
- [Custom images](custom-images.md) — Use Python, Node, Go, or any glibc image
- [Lifecycle hooks](lifecycle-hooks.md) — Install deps, start services before Claude runs
- [Compose environments](compose-environments.md) — Multi-service stacks with databases
