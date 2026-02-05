# Running Agents

Learn how to run Claude agents with Stromboli.

## How It Works

Stromboli runs Claude Code inside isolated containers:

```
┌─────────────────────────────────────────────────────────────┐
│  Stromboli Server                                           │
│                                                             │
│  1. Receives API request                                    │
│  2. Creates Podman container                                │
│  3. Mounts Claude CLI into container                        │
│  4. Runs Claude with your prompt                            │
│  5. Returns output                                          │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  Agent Container                                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Base Image (debian, python, node, etc.)                    │
│       +                                                     │
│  Claude CLI (mounted from CLI image)                        │
│       +                                                     │
│  Your workspace (mounted read/write)                        │
│       +                                                     │
│  Credentials (mounted read-only)                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Image Architecture

Stromboli uses two types of images:

### 1. CLI Image (Claude CLI source)

Contains the Claude CLI binary. This is mounted into agent containers.

```yaml
# Configuration
agent:
  cli_image: "ghcr.io/tomblancdev/stromboli-agent"
  cli_image_tag: "latest"
  auto_pull_cli: true  # Pull automatically if missing
```

**On startup**, Stromboli:

1. Checks if CLI image exists locally
2. If missing and `auto_pull_cli: true` → pulls from registry
3. If missing and `auto_pull_cli: false` → logs warning

### 2. Base Image (agent environment)

The container where Claude actually runs. Can be customized per request.

```yaml
# Default base image
agent:
  image: "ghcr.io/tomblancdev/stromboli-agent"
  image_tag: "latest"
```

## Basic Usage

### Simple Request

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello, Claude!"
  }'
```

### With a Project Directory

Mount a directory for the agent to work with:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze the code in this project",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"]
    }
  }'
```

- `workdir`: Sets the working directory inside the container
- `volumes`: Mounts host paths into the container (`host:container` format)

## Claude Options

Configure Claude's behavior:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Review this code",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"]
    },
    "claude": {
      "model": "sonnet",
      "system_prompt": "You are a senior Go developer",
      "max_budget_usd": 1.00,
      "allowed_tools": ["Read", "Grep", "Glob"],
      "output_format": "json"
    }
  }'
```

### Available Options

| Option | Type | Description |
|--------|------|-------------|
| `model` | string | Model: `sonnet`, `opus`, `haiku` |
| `system_prompt` | string | Replace default system prompt |
| `append_system_prompt` | string | Append to system prompt |
| `max_budget_usd` | float | Maximum API cost |
| `allowed_tools` | []string | Whitelist tools |
| `disallowed_tools` | []string | Blacklist tools |
| `output_format` | string | `text`, `json`, `stream-json` |
| `dangerously_skip_permissions` | bool | Skip permission prompts |

## Container Options

Configure the Podman container:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Run heavy analysis",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"],
      "memory": "2g",
      "cpus": "2",
      "timeout": "30m"
    }
  }'
```

### Available Options

| Option | Type | Description |
|--------|------|-------------|
| `memory` | string | Memory limit (e.g., `512m`, `2g`) |
| `cpus` | string | CPU limit (e.g., `0.5`, `2`) |
| `timeout` | string | Max runtime (e.g., `5m`, `1h`) |
| `image` | string | Custom container image |
| `volumes` | []string | Volume mounts (`host:container[:options]`) |
| `secrets_env` | map | Secrets as env vars |

## Dynamic Images

Use different container images based on your needs:

```bash
# Python environment
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "Run Python analysis",
    "podman": {"image": "python:3.12"}
  }'

# Go environment
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "Build and test",
    "podman": {"image": "golang:1.22"}
  }'

# Node.js environment
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "Run npm tests",
    "podman": {"image": "node:20"}
  }'
```

### How Dynamic Images Work

```
┌───────────────────────────────────────────────────────────┐
│  Request: {"podman": {"image": "python:3.12"}}            │
└─────────────────────────┬─────────────────────────────────┘
                          │
                          ▼
┌───────────────────────────────────────────────────────────┐
│  Stromboli creates container:                             │
│                                                           │
│  podman run python:3.12                                   │
│    --mount type=image,src=stromboli-agent,dst=/opt/claude │
│    --volume /workspace:/workspace                         │
│    ...                                                    │
│    /opt/claude/bin/claude "your prompt"                   │
└───────────────────────────────────────────────────────────┘
```

The Claude CLI is **mounted from the CLI image** into any base container, so it works with any glibc-based image.

!!! warning "Image Compatibility"
    Alpine Linux and musl-based images are **not supported**. Use glibc-based images:

    - ✅ `python:3.12` (Debian-based)
    - ✅ `node:20` (Debian-based)
    - ✅ `golang:1.22` (Debian-based)
    - ✅ `ubuntu:22.04`
    - ✅ `debian:bookworm`
    - ❌ `python:3.12-alpine`
    - ❌ `node:20-alpine`

### Configure Allowed Images

For security, configure which images are allowed:

```yaml
agent:
  allowed_image_patterns:
    - "python:*"
    - "node:*"
    - "golang:*"
    - "ubuntu:*"
    - "debian:*"
```

Or via environment variable:

```bash
STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*,golang:*"
```

## Async Execution

Run agents asynchronously for long tasks:

```bash
# Start async job
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Refactor the entire codebase",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"]
    }
  }'
```

Response:
```json
{
  "job_id": "job-abc123",
  "status": "pending"
}
```

Check job status:
```bash
curl http://localhost:8080/jobs/job-abc123
```

## Streaming Output

Get real-time output via Server-Sent Events:

```bash
curl -N "http://localhost:8080/run/stream?prompt=Hello&workdir=/workspace"
```

!!! note "Streaming with Volumes"
    For streaming with mounted directories, use the POST endpoint instead.

Output:
```
data: {"type":"output","content":"Hello! I'm analyzing..."}
data: {"type":"output","content":"Found 3 files..."}
data: {"type":"done","session_id":"..."}
```

## Webhooks

Get notified when async jobs complete:

```bash
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Long running task",
    "webhook_url": "https://your-server.com/webhook"
  }'
```

Webhook payload:
```json
{
  "job_id": "job-abc123",
  "status": "completed",
  "output": "Task completed...",
  "session_id": "..."
}
```

## Secrets Injection

Inject Podman secrets as environment variables:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Deploy to production",
    "podman": {
      "secrets_env": {
        "AWS_ACCESS_KEY_ID": "aws-key-secret",
        "AWS_SECRET_ACCESS_KEY": "aws-secret-secret"
      }
    }
  }'
```

See [Secrets Guide](secrets.md) for more details.

## Lifecycle Hooks

Run commands at specific container lifecycle stages:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Run the tests",
    "workdir": "/workspace",
    "podman": {
      "image": "python:3.12",
      "volumes": ["/home/user/myproject:/workspace"],
      "lifecycle": {
        "on_create_command": ["pip install -r requirements.txt"],
        "post_start": ["redis-server --daemonize yes"]
      }
    }
  }'
```

See [Lifecycle Hooks](lifecycle-hooks.md) for more details.

## Compose Environments

Run Claude in multi-service environments with databases, caches, and other services:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Set up the database schema",
    "workdir": "/workspace",
    "podman": {
      "environment": {
        "type": "compose",
        "path": "/home/user/myproject/docker-compose.yml",
        "service": "dev"
      }
    }
  }'
```

See [Compose Environments](compose-environments.md) for more details.
