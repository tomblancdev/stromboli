# Docker Deployment

Run Stromboli in a Docker container.

## Prerequisites

- Docker or Podman installed
- Claude CLI authenticated on the host
- Podman socket accessible

## Quick Start

```bash
docker run -d \
  --name stromboli \
  -p 8080:8080 \
  -v /run/podman/podman.sock:/run/podman/podman.sock \
  -v ~/.claude:/app/.claude:ro \
  -v ./sessions:/app/sessions \
  ghcr.io/tomblancdev/stromboli:latest
```

## Build from Source

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli

# Build the image
make build-server-image

# Run
docker run -d \
  --name stromboli \
  -p 8080:8080 \
  -v /run/podman/podman.sock:/run/podman/podman.sock \
  -v ~/.claude:/app/.claude:ro \
  stromboli:latest
```

## Volume Mounts

| Host Path | Container Path | Purpose |
|-----------|----------------|---------|
| `/run/podman/podman.sock` | `/run/podman/podman.sock` | Podman API socket |
| `~/.claude` | `/app/.claude` | Claude credentials |
| `./sessions` | `/app/sessions` | Session storage |

## Environment Variables

```bash
docker run -d \
  --name stromboli \
  -p 8080:8080 \
  -e STROMBOLI_AGENT_ALLOWED_WORKSPACES=/workspace \
  -e STROMBOLI_AGENT_DEFAULT_MEMORY=512m \
  -e STROMBOLI_AGENT_DEFAULT_TIMEOUT=5m \
  -v /run/podman/podman.sock:/run/podman/podman.sock \
  -v ~/.claude:/app/.claude:ro \
  -v /home/user/projects:/workspace:ro \
  stromboli:latest
```

## Health Check

The container includes a health check:

```bash
docker inspect stromboli --format='{{.State.Health.Status}}'
```

## Logs

```bash
# Follow logs
docker logs -f stromboli

# Last 100 lines
docker logs --tail 100 stromboli
```

## Updating

```bash
# Pull latest
docker pull ghcr.io/tomblancdev/stromboli:latest

# Restart
docker stop stromboli
docker rm stromboli
docker run -d ... # same command as before
```

## Rootless Podman

For rootless Podman, mount the user socket:

```bash
docker run -d \
  --name stromboli \
  -p 8080:8080 \
  -v /run/user/1000/podman/podman.sock:/run/podman/podman.sock \
  -v ~/.claude:/app/.claude:ro \
  stromboli:latest
```

## Troubleshooting

### "Cannot connect to Podman"

Check the socket is mounted and accessible:
```bash
docker exec stromboli ls -la /run/podman/podman.sock
```

### "Claude not configured"

Ensure credentials are mounted:
```bash
docker exec stromboli ls -la /app/.claude/
```

### Permission Issues

The container runs as non-root (UID 1000). Ensure mounted directories have correct permissions:
```bash
chmod 755 ./sessions
```
