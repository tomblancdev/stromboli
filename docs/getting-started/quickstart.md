# Quick Start

Get Stromboli running and spawn your first agent in 5 minutes.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/) with Compose support
- Claude CLI authenticated (`claude` command works in your terminal)

## 1. Install & start Stromboli

=== "Install Script"

    ```bash
    curl -sL https://raw.githubusercontent.com/tomblancdev/stromboli/main/install.sh | bash
    cd stromboli
    docker compose up -d
    ```

=== "Docker Compose"

    ```bash
    git clone https://github.com/tomblancdev/stromboli
    cd stromboli
    docker compose up -d
    ```

See [installation](installation.md) for all options.

## 2. Check health

```bash
curl -s localhost:8080/health | jq
```

```json
{
  "status": "ok",
  "name": "stromboli",
  "version": "0.3.0-alpha",
  "components": [
    {"name": "podman", "status": "ok"},
    {"name": "claude-credentials-file", "status": "ok"},
    {"name": "claude-credentials-secret", "status": "ok"}
  ]
}
```

## 3. Run your first agent

```bash
curl -X POST localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"prompt": "What is 2 + 2? Reply with just the number."}'
```

```json
{
  "id": "run-abc123def456",
  "status": "completed",
  "output": "4",
  "session_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## 4. Mount a project directory

Give the agent access to your code:

```bash
curl -X POST localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "List the files in this directory and describe what they do",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"]
    }
  }'
```

## 5. Continue the conversation

Use the `session_id` from the previous response to resume:

```bash
curl -X POST localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Now explain the main entry point in detail",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"]
    },
    "claude": {
      "session_id": "550e8400-e29b-41d4-a716-446655440000",
      "resume": true
    }
  }'
```

## What's next

- [Configuration](configuration.md) — Customize resource limits, auth, and more
- [Running agents](../guides/running-agents.md) — All the options for spawning agents
- [Secrets](../guides/secrets.md) — Inject GitHub/GitLab tokens
- [API reference](../api/overview.md) — Full endpoint documentation

## Troubleshooting

**"Claude not configured"** — Run `claude` in your terminal to authenticate, then restart Stromboli.

**Container fails to start** — Check Podman is running: `podman version && podman ps`

**Files not visible** — Make sure the volume mount path is correct: `"volumes": ["/your/actual/path:/workspace"]`
