# Custom Images

Stromboli lets you run Claude in any glibc-based Docker image — Python, Node, Go, or anything else your agent needs.

## How it works

When you specify a custom image, Stromboli mounts the Claude CLI binary from a separate CLI image into your chosen base:

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Run the tests",
    "podman": {"image": "python:3.12"}
  }'
```

Behind the scenes:

```
podman run python:3.12 \
  --mount type=image,src=stromboli-agent,dst=/opt/claude \
  /opt/claude/bin/claude "Run the tests"
```

The CLI image is pulled once and reused for all agent runs.

## Supported images

Any image based on glibc works. Most official Docker images are Debian-based and compatible:

| Image | Works | Notes |
|---|---|---|
| `python:3.12` | Yes | Debian-based |
| `python:3.12-slim` | Yes | Smaller Debian variant |
| `node:20` | Yes | Debian-based |
| `golang:1.22` | Yes | Debian-based |
| `ubuntu:22.04` | Yes | |
| `debian:bookworm` | Yes | |
| `python:3.12-alpine` | **No** | musl-based — Claude CLI requires glibc |
| `node:20-alpine` | **No** | musl-based |

!!! warning "Alpine doesn't work"
    Alpine Linux uses musl instead of glibc. Claude CLI won't run on musl-based images. Use the standard (Debian-based) variants.

## Restricting allowed images

In production, set an allowlist of approved image patterns:

```yaml
agent:
  allowed_image_patterns:
    - "python:*"
    - "node:*"
    - "golang:*"
    - "ubuntu:*"
    - "ghcr.io/myorg/*"
```

Or via environment variable:

```bash
STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*,golang:*"
```

If `allowed_image_patterns` is empty, **all images are allowed**. Always set an allowlist in production.

## Combining with lifecycle hooks

Custom images pair well with [lifecycle hooks](lifecycle-hooks.md) for dependency installation:

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Run the test suite",
    "workdir": "/workspace",
    "podman": {
      "image": "python:3.12",
      "volumes": ["/home/user/myproject:/workspace"],
      "lifecycle": {
        "on_create_command": ["pip install -r requirements.txt"]
      }
    }
  }'
```

Dependencies install once when the session is first created. Subsequent `resume` requests skip `on_create_command` — only `post_start` hooks re-run.

## Combining with compose environments

For multi-service setups (database, cache, etc.), use [compose environments](compose-environments.md) instead of plain custom images:

```yaml
# docker-compose.yml
services:
  dev:
    image: python:3.12
    working_dir: /workspace
    volumes:
      - .:/workspace
    depends_on:
      - db

  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: devpass
```

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Run the integration tests",
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

## Image discovery API

Stromboli can list local images, inspect them, search registries, and pull new images:

```bash
# List local images (sorted by compatibility)
curl localhost:8080/images

# Search registries
curl "localhost:8080/images/search?q=python"

# Pull an image
curl -X POST localhost:8080/images/pull \
  -d '{"image": "python:3.12"}'
```

See the [API reference](../api/endpoints.md#get-images) for details.

## CLI image configuration

Configure the CLI image source in your config:

```yaml
agent:
  cli_image: "ghcr.io/tomblancdev/stromboli-agent"
  cli_image_tag: "latest"
  auto_pull_cli: true   # pull on startup if missing
  mount_claude_cli: true
```

On startup, Stromboli checks for the CLI image locally. If missing and `auto_pull_cli` is true, it pulls from the registry automatically.
