# Running Agents

Learn how to run Claude agents with Stromboli.

## Basic Request

The simplest way to run an agent:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello, Claude!"
  }'
```

## With a Workspace

Mount a directory for the agent to work with:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze the code in this project",
    "workspace": "/home/user/myproject"
  }'
```

The workspace is mounted at `/workspace` inside the container.

## Claude Options

Configure Claude's behavior:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Review this code",
    "workspace": "/home/user/myproject",
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
    "workspace": "/home/user/myproject",
    "podman": {
      "memory": "2g",
      "cpus": "2",
      "timeout": "30m",
      "image": "python:3.12"
    }
  }'
```

### Container Options

| Option | Type | Description |
|--------|------|-------------|
| `memory` | string | Memory limit (e.g., `512m`, `2g`) |
| `cpus` | string | CPU limit (e.g., `0.5`, `2`) |
| `timeout` | string | Max runtime (e.g., `5m`, `1h`) |
| `image` | string | Custom container image |
| `volumes` | []string | Additional volume mounts |
| `secrets_env` | map | Secrets as env vars |

## Async Execution

Run agents asynchronously for long tasks:

```bash
# Start async job
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Refactor the entire codebase",
    "workspace": "/home/user/myproject"
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
curl -N "http://localhost:8080/run/stream?prompt=Hello&workspace=/home/user/project"
```

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

## Custom Images

Use different container images (must be in allowlist):

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
```

!!! note "Image Allowlist"
    Configure `STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS` to allow custom images.
