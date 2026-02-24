# Why Stromboli?

Stromboli turns Claude Code into an API-driven service. Instead of running Claude in your terminal, you spawn agents over HTTP — each one isolated in its own container with its own environment, resources, and secrets.

## Full isolation

Every agent runs in its own Podman container. Separate filesystem, network namespace, process tree, resource limits. If an agent runs `rm -rf /`, it only affects its container — your host is untouched.

```
┌─ Agent 1 ──────────────┐  ┌─ Agent 2 ──────────────┐
│ python:3.12             │  │ node:20                 │
│ /workspace → your-repo  │  │ /workspace → other-repo │
│ 512MB RAM, 1 CPU        │  │ 1GB RAM, 2 CPUs         │
│ GH_TOKEN secret         │  │ NPM_TOKEN secret        │
└─────────────────────────┘  └─────────────────────────┘
         ▲                            ▲
         └──── Podman (rootless) ─────┘
```

No shared state. No port conflicts. No "it worked on my machine."

## Any runtime, any stack

Use any Docker image as the agent's base environment. Python, Node, Go, Rust — if there's a Docker image for it, Stromboli can run Claude in it.

Need a database alongside your agent? Use a [compose environment](../guides/compose-environments.md) to spin up PostgreSQL, Redis, or any multi-service stack.

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Run the test suite",
    "podman": {
      "image": "python:3.12",
      "lifecycle": {"on_create_command": ["pip install -r requirements.txt"]}
    }
  }'
```

## API-first automation

Stromboli exposes a REST API. Spawn agents from CI/CD pipelines, webhooks, Slack bots, cron jobs, or any HTTP client. Three execution modes fit different workflows:

| Mode | Endpoint | Use case |
|------|----------|----------|
| **Sync** | `POST /run` | Short tasks — get the result directly |
| **Async** | `POST /run/async` | Long tasks — poll for results via job ID |
| **Streaming** | `GET /run/stream` | Real-time output via Server-Sent Events |

## Secrets without the mess

Inject tokens (GitHub, GitLab, npm, etc.) via Podman's native secret store. Each agent only sees the secrets you explicitly pass — no leaked environment variables, no shared credentials.

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Push the fix to GitHub",
    "podman": {"secrets_env": {"GH_TOKEN": "github-token"}}
  }'
```

## Resource control

Set memory limits, CPU limits, and execution timeouts per agent. A runaway agent gets killed — not your machine.

| Setting | Default | Per-request override |
|---------|---------|---------------------|
| Memory | 512MB | `podman.memory` |
| CPUs | 1 | `podman.cpus` |
| Timeout | 30 min | `podman.timeout` |

## Session persistence

Conversations survive across requests. Every run returns a `session_id` — pass it back to continue where you left off, even in a completely fresh container.

```bash
# First request
curl -X POST localhost:8080/run -d '{"prompt": "Analyze this code"}'
# → {"session_id": "abc-123", ...}

# Resume later
curl -X POST localhost:8080/run -d '{
  "prompt": "Now fix the bug you found",
  "claude": {"session_id": "abc-123", "resume": true}
}'
```

## Security by default

Multiple validation layers protect every request:

- **Rootless containers** — even a container escape gives unprivileged access
- **Volume allowlists** — host paths must be explicitly allowed
- **Image allowlists** — only approved images can be used
- **Path blocklists** — can't mount over `/etc`, `~/.ssh`, or other sensitive paths
- **Rate limiting** and **JWT authentication** when you need them

See [security overview](../security/overview.md) for the full picture.

## How does this compare to worktrees?

Claude Code has built-in git worktree support for local isolation. Worktrees are great for quick single-agent work on your laptop — zero setup, instant startup.

Stromboli is for when you need more:

| Need | Worktrees | Stromboli |
|------|-----------|-----------|
| Run agents from CI/CD or scripts | No (CLI only) | Yes (REST API) |
| Custom runtimes (Python, Node, etc.) | No (host only) | Any Docker image |
| Databases and services alongside agents | No | Yes (Compose) |
| Per-agent secrets and resource limits | No | Yes |
| OS-level isolation | No (filesystem only) | Yes (full container) |
| Parallel agents without conflicts | Limited | Natural |

You can use both — worktrees for local prototyping, Stromboli for automation and production.

## What's next

- [How it works](how-it-works.md) — Architecture and request flow
- [Quick start](../getting-started/quickstart.md) — Get running in 5 minutes
- [Running agents](../guides/running-agents.md) — All the options for spawning agents
