# Troubleshooting

Quick fixes for common Stromboli issues.

## Quick diagnostics

```bash
curl localhost:8080/health           # API health
curl localhost:8080/claude/status    # Claude credentials
podman info                          # Podman status
podman ps -a                         # Running containers
```

## Error codes

| HTTP Status | Meaning | Common fix |
|---|---|---|
| `400` | Bad request | Check JSON syntax, required fields |
| `401` | Unauthorized | Get a fresh JWT token |
| `403` | Forbidden | Image or volume not in allowlist |
| `408` | Timeout | Increase `podman.timeout` in request |
| `429` | Rate limited | Wait for `Retry-After` header value |
| `500` | Server error | Check Stromboli logs |
| `502` | Container failed | Check image compatibility |
| `503` | Service unavailable | Claude not configured or Podman down |

## Common issues

### Stromboli won't start

```bash
# Port already in use?
lsof -i :8080

# Use different port
STROMBOLI_SERVER_ADDRESS=":8081" ./stromboli
```

### "Podman socket not found"

```bash
# Enable and start socket
systemctl --user enable --now podman.socket

# Check it exists
ls -la ${XDG_RUNTIME_DIR}/podman/podman.sock
```

### "Claude not configured"

Run `claude` in your terminal to authenticate, then restart Stromboli. Check credentials exist:

```bash
ls ~/.claude/.credentials.json
```

### "Image not allowed"

Add the image pattern to your config:

```bash
STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*,your-image:*"
```

### "Workspace not allowed"

Add the directory to your volume allowlist:

```bash
STROMBOLI_AGENT_ALLOWED_VOLUMES="/home/user/projects,/data/workspaces"
```

### Alpine images don't work

Claude CLI requires glibc. Alpine uses musl. Use Debian-based variants:

```
python:3.12       # works
python:3.12-slim  # works
python:3.12-alpine  # doesn't work
```

### Container timeout

Increase the timeout per-request or in defaults:

```bash
# Per-request
{"podman": {"timeout": "1h"}}

# Default
STROMBOLI_RESOURCES_TIMEOUT=1h
```

### Out of memory

Increase the memory limit:

```bash
# Per-request
{"podman": {"memory": "2g"}}

# Default
STROMBOLI_RESOURCES_MEMORY=2g
```

### Session not found

Sessions may have been cleaned up or the storage path changed. Check:

```bash
curl localhost:8080/sessions    # list existing sessions
ls /path/to/sessions/           # check storage directory
```

### Slow first request

The first request pulls images if they're not cached locally. Pre-pull to avoid this:

```bash
podman pull ghcr.io/tomblancdev/stromboli-agent:latest
podman pull python:3.12
```

### Permission denied on volumes

For SELinux systems (Fedora/RHEL):

```bash
# Add :Z suffix to volumes
{"podman": {"volumes": ["/home/user/project:/workspace:Z"]}}
```

For general permission issues:

```bash
chmod -R 755 /home/user/project
```

### Containers not cleaning up

```bash
# List Stromboli containers
podman ps -a --filter "label=managed-by=stromboli"

# Manual cleanup
podman rm -f $(podman ps -aq --filter "label=managed-by=stromboli")
```

Check cleanup config: `STROMBOLI_JOBS_CLEANUP_TTL` and `STROMBOLI_JOBS_CLEANUP_INTERVAL`.

## Debugging

### Enable debug logging

```bash
STROMBOLI_LOG_LEVEL=debug ./stromboli
```

### Inspect a container

```bash
podman ps -a --filter "label=managed-by=stromboli"
podman logs <container-id>
podman inspect <container-id>
```

### Test Podman directly

```bash
# Test container creation
podman run --rm python:3.12 python -c "print('works')"

# Test Claude CLI mount
podman run --rm \
  --mount type=image,src=ghcr.io/tomblancdev/stromboli-agent:latest,dst=/opt/claude,ro \
  python:3.12 /opt/claude/bin/claude --version
```

## FAQ

**Can I use Docker instead of Podman?** — Yes, mount the Docker socket: `-v /var/run/docker.sock:/run/podman/podman.sock`. Remove `userns_mode: keep-id` from compose.

**How do I update?** — `docker compose pull && docker compose up -d`

**Can multiple users share one instance?** — Yes, with authentication enabled. Each user gets separate sessions.

**How do I limit API costs?** — Set `max_budget_usd` per request: `{"claude": {"max_budget_usd": 0.10}}`

## Getting help

When reporting issues, include:

1. Stromboli version: `./stromboli --version`
2. Podman version: `podman version`
3. OS: `cat /etc/os-release`
4. Error message and request (redact secrets)
5. Logs: `podman logs stromboli`

[GitHub Issues](https://github.com/tomblancdev/stromboli/issues)
