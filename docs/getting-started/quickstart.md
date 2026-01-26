# Quick Start

Get Stromboli running in 5 minutes.

## Prerequisites

- [Podman](https://podman.io/) v4.0+ installed
- Claude CLI authenticated (`claude` command works)
- Docker Compose (optional, for easy deployment)

## Step 1: Clone & Start

=== "Docker Compose"

    ```bash
    git clone https://github.com/tomblancdev/stromboli
    cd stromboli

    # Start Stromboli
    docker compose up -d

    # Check health
    curl http://localhost:8080/health
    ```

=== "From Source"

    ```bash
    git clone https://github.com/tomblancdev/stromboli
    cd stromboli

    # Build
    make build

    # Run
    ./stromboli --port 8080
    ```

## Step 2: Verify Installation

```bash
curl -s http://localhost:8080/health | jq
```

Expected output:
```json
{
  "status": "ok",
  "name": "stromboli",
  "version": "0.1.5-alpha",
  "components": [
    {"name": "podman", "status": "ok"},
    {"name": "claude-credentials-file", "status": "ok"},
    {"name": "claude-credentials-secret", "status": "ok"}
  ]
}
```

## Step 3: Run Your First Agent

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is 2 + 2? Reply with just the number."
  }'
```

Response:
```json
{
  "id": "run-abc123def456",
  "status": "completed",
  "output": "4",
  "session_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## Step 4: Run with a Workspace

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "List the files in this directory",
    "workspace": "/home/user/myproject"
  }'
```

!!! warning "Workspace Allowlist"
    By default, workspaces must be in the allowlist. Configure `STROMBOLI_AGENT_ALLOWED_WORKSPACES` to allow your directories.

## Step 5: Continue a Conversation

Use the `session_id` from the previous response:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Now explain what each file does",
    "workspace": "/home/user/myproject",
    "claude": {
      "session_id": "550e8400-e29b-41d4-a716-446655440000",
      "resume": true
    }
  }'
```

## Next Steps

- [Configuration](configuration.md) - Customize settings
- [Secrets Guide](../guide/secrets.md) - Inject GitHub/GitLab tokens
- [API Reference](../api/overview.md) - Full API documentation

## Troubleshooting

### "Claude not configured"

Run `claude` in your terminal to authenticate, then restart Stromboli.

### "Workspace not allowed"

Add your workspace to the allowlist:
```bash
export STROMBOLI_AGENT_ALLOWED_WORKSPACES="/home/user/projects,/var/data"
```

### Container fails to start

Check Podman is running:
```bash
podman version
podman ps
```
