# Installation

## One-line install (recommended)

The install script detects Docker or Podman, downloads the compose file and config, and gets you ready to start:

```bash
curl -sL https://raw.githubusercontent.com/tomblancdev/stromboli/main/install.sh | bash
```

Then start Stromboli:

```bash
cd stromboli
docker compose up -d    # or: podman-compose up -d
```

The script creates a `stromboli/` directory with:

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Pre-configured compose stack |
| `stromboli.example.yaml` | Full config file with all options documented |

## Docker Compose (manual)

If you prefer to set things up yourself:

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli
docker compose up -d
```

This starts Stromboli on port 8080 with automatic credential sync and persistent session storage. See [Docker deployment](../self-hosting/docker.md) for the full compose file breakdown.

## Docker image

Pull and run the image directly:

```bash
docker pull ghcr.io/tomblancdev/stromboli:latest

docker run -d \
  -p 8080:8080 \
  -v ${XDG_RUNTIME_DIR}/podman/podman.sock:/run/podman/podman.sock \
  -v ~/.claude:/home/stromboli/.claude:ro \
  -v ~/.stromboli/sessions:/app/sessions \
  ghcr.io/tomblancdev/stromboli:latest
```

See [Docker deployment](../self-hosting/docker.md) for volume mounts, environment variables, and production setup.

## Requirements

| Component | Version | Notes |
|---|---|---|
| Docker or Podman | 4.0+ | Container runtime — the install script detects which you have |
| Claude CLI | Latest | Authenticated — run `claude` in your terminal first |

!!! tip "Don't have Claude CLI?"
    ```bash
    npm install -g @anthropic-ai/claude-code
    claude    # authenticate
    ```
    Credentials are stored at `~/.claude/.credentials.json`. Stromboli mounts this file read-only into containers.

## Verify

```bash
curl localhost:8080/health
curl localhost:8080/claude/status
```

## Advanced: build from source

If you want to build the binary or image yourself:

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli
make build          # binary at ./stromboli
make build-server-image  # container image
```

Requires Go 1.24+.

## What's next

- [Quick start](quickstart.md) — Run your first agent
- [Configuration](configuration.md) — Customize Stromboli
