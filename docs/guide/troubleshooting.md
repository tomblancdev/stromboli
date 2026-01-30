# Troubleshooting Guide

Solutions for common issues when running Stromboli.

## Quick Diagnostics

Run these commands to diagnose issues:

```bash
# Check Stromboli health
curl http://localhost:8080/health

# Check Claude credentials status
curl http://localhost:8080/claude/status

# Check Podman socket
podman info

# Check running containers
podman ps -a

# View Stromboli logs
podman logs stromboli
# or
docker logs stromboli
```

---

## Error Reference

### HTTP Status Codes

| Code | Meaning | Common Causes |
|------|---------|---------------|
| `400` | Bad Request | Invalid JSON, missing required fields, invalid parameters |
| `401` | Unauthorized | Missing/invalid token, expired JWT |
| `403` | Forbidden | Image not allowed, workspace not in allowlist |
| `404` | Not Found | Invalid endpoint, job/session not found |
| `408` | Request Timeout | Container execution exceeded timeout |
| `429` | Too Many Requests | Rate limit exceeded |
| `500` | Internal Server Error | Server error, Podman failure |
| `502` | Bad Gateway | Container startup failure |
| `503` | Service Unavailable | Podman not available, Claude not configured |

### API Error Responses

All errors return JSON with details:

```json
{
  "error": "brief error message",
  "details": "detailed explanation (when available)",
  "code": "ERROR_CODE"
}
```

### Common Error Codes

#### `INVALID_REQUEST`

**Error:** `invalid request body`

**Causes:**
- Malformed JSON
- Missing required `prompt` field
- Invalid field types

**Solutions:**
```bash
# Ensure valid JSON
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello"}'  # Note: proper JSON with double quotes
```

---

#### `UNAUTHORIZED`

**Error:** `authentication required` or `invalid token`

**Causes:**
- Missing Authorization header
- Expired JWT token
- Invalid API token

**Solutions:**
```bash
# Get fresh token
TOKEN=$(curl -s -X POST http://localhost:8080/auth/token \
  -H "X-API-Token: your-api-token" | jq -r '.access_token')

# Use token
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/run ...
```

---

#### `IMAGE_NOT_ALLOWED`

**Error:** `image "xxx" not in allowed patterns`

**Causes:**
- Requested image doesn't match `allowed_image_patterns`
- Alpine/musl image requested (incompatible)

**Solutions:**
```yaml
# Add image pattern to config
agent:
  allowed_image_patterns:
    - "python:*"
    - "node:*"
    - "your-image:*"
```

```bash
# Or use environment variable
STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*"
```

---

#### `WORKSPACE_NOT_ALLOWED`

**Error:** `workspace "/path" not in allowed directories`

**Causes:**
- Workspace path not in allowlist
- Path traversal attempt blocked

**Solutions:**
```yaml
# Add directory to allowlist
agent:
  allowed_workspaces:
    - "/home/user/projects"
    - "/data/workspaces"
```

---

#### `CONTAINER_TIMEOUT`

**Error:** `container execution timed out after Xm`

**Causes:**
- Task took longer than configured timeout
- Infinite loop in Claude agent
- Heavy computation

**Solutions:**
```bash
# Increase timeout for specific request
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "...",
    "podman": {"timeout": "1h"}
  }'
```

```yaml
# Or increase default timeout
resources:
  timeout: "1h"
```

---

#### `CONTAINER_OOM`

**Error:** `container killed: out of memory`

**Causes:**
- Memory limit exceeded
- Large file processing
- Memory leak in agent task

**Solutions:**
```bash
# Increase memory limit
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "...",
    "podman": {"memory": "2g"}
  }'
```

---

#### `CLAUDE_NOT_CONFIGURED`

**Error:** `Claude credentials not found`

**Causes:**
- Missing credentials file
- Invalid credentials path
- Credentials file not mounted in container

**Solutions:**
```bash
# Check credentials exist
cat ~/.claude/.credentials.json

# If missing, authenticate with Claude CLI
claude

# Verify mount in compose file
volumes:
  - ${HOME}/.claude:/home/stromboli/.claude:ro
```

---

#### `PODMAN_ERROR`

**Error:** `podman: ...` or `failed to create container`

**Causes:**
- Podman socket not available
- Permission issues
- Resource exhaustion

**Solutions:**
```bash
# Start Podman socket
systemctl --user start podman.socket

# Check socket exists
ls -la ${XDG_RUNTIME_DIR}/podman/podman.sock

# Verify socket is mounted
podman run --rm -v ${XDG_RUNTIME_DIR}/podman/podman.sock:/run/podman/podman.sock \
  alpine ls -la /run/podman/
```

---

#### `RATE_LIMITED`

**Error:** `rate limit exceeded`

**Causes:**
- Too many requests from same IP/user
- Burst limit exceeded

**Solutions:**
```bash
# Check Retry-After header
curl -i http://localhost:8080/run ...
# HTTP/1.1 429 Too Many Requests
# Retry-After: 2

# Wait and retry
sleep 2
curl ...
```

---

## Common Issues

### Stromboli Won't Start

**Symptoms:** Server exits immediately or fails to bind port

**Check:**
```bash
# Port already in use?
lsof -i :8080

# Check logs
podman logs stromboli

# Verify config
cat stromboli.yaml
```

**Solutions:**
```bash
# Use different port
STROMBOLI_SERVER_ADDRESS=":8081" ./stromboli

# Kill existing process
kill $(lsof -t -i :8080)
```

---

### "Podman socket not found"

**Symptoms:** `dial unix /run/podman/podman.sock: connect: no such file or directory`

**Check:**
```bash
# Is socket enabled?
systemctl --user status podman.socket

# Where is socket?
echo $XDG_RUNTIME_DIR
ls -la ${XDG_RUNTIME_DIR}/podman/
```

**Solutions:**
```bash
# Enable and start socket
systemctl --user enable --now podman.socket

# Verify it's running
podman info

# Fix compose mount path
volumes:
  - ${XDG_RUNTIME_DIR}/podman/podman.sock:/run/podman/podman.sock
```

---

### "Image not compatible" (Alpine)

**Symptoms:** `image "xxx-alpine" is incompatible with Claude CLI mount`

**Cause:** Alpine uses musl libc, Claude CLI requires glibc

**Solutions:**
```bash
# Use Debian-based variants instead
# Instead of:
python:3.12-alpine  ❌

# Use:
python:3.12         ✅
python:3.12-slim    ✅
```

---

### Containers Keep Running After Timeout

**Symptoms:** Old containers accumulate, not cleaned up

**Check:**
```bash
# List stromboli containers
podman ps -a --filter "label=managed-by=stromboli"
```

**Solutions:**
```bash
# Manual cleanup
podman rm -f $(podman ps -aq --filter "label=managed-by=stromboli")

# Stromboli auto-cleans on startup - restart server
podman restart stromboli
```

---

### Session Not Found

**Symptoms:** `session "xxx" not found` when resuming

**Causes:**
- Session expired/cleaned up
- Session ID typo
- Session storage not persisted

**Check:**
```bash
# List sessions
curl http://localhost:8080/sessions

# Check session storage
ls -la /app/sessions/  # or configured path
```

**Solutions:**
```yaml
# Ensure session persistence in compose
volumes:
  - stromboli-sessions:/app/sessions

volumes:
  stromboli-sessions:
```

---

### Slow Container Startup

**Symptoms:** First request takes very long (30s+)

**Causes:**
- Image not cached locally
- Large image being pulled
- Slow network

**Solutions:**
```bash
# Pre-pull images
podman pull ghcr.io/tomblancdev/stromboli-agent:latest
podman pull python:3.12

# Use local images
podman build -t my-agent ./Dockerfile
```

---

### "Permission denied" Mounting Workspace

**Symptoms:** Container can't read/write workspace files

**Causes:**
- SELinux blocking access
- User namespace mapping issues
- File permission mismatch

**Solutions:**
```bash
# Option 1: Add :Z suffix for SELinux (Fedora/RHEL)
volumes:
  - /home/user/project:/workspace:Z

# Option 2: Fix permissions
chmod -R 755 /home/user/project

# Option 3: Use keep-id with rootless Podman (docker-compose)
userns_mode: keep-id
```

---

### High Memory Usage

**Symptoms:** Stromboli or containers use excessive memory

**Check:**
```bash
# Container memory usage
podman stats

# Host memory
free -h
```

**Solutions:**
```yaml
# Set memory limits
resources:
  memory: "512m"

# Enable cleanup
jobs:
  cleanup_ttl: "30m"
  cleanup_interval: "5m"
```

---

## Debugging Tips

### Enable Debug Logging

```bash
# Set log level (if supported)
STROMBOLI_LOG_LEVEL=debug ./stromboli
```

### Inspect Container State

```bash
# See what containers are running
podman ps -a --filter "label=managed-by=stromboli"

# Inspect specific container
podman inspect <container-id>

# View container logs
podman logs <container-id>

# Execute into running container (for debugging)
podman exec -it <container-id> /bin/bash
```

### Test Podman Directly

```bash
# Test container creation
podman run --rm -it \
  --memory 512m \
  --cpus 1 \
  python:3.12 \
  python -c "print('Hello from container')"

# Test with Claude CLI mount
podman run --rm -it \
  --mount type=image,src=ghcr.io/tomblancdev/stromboli-agent:latest,dst=/opt/claude,ro \
  python:3.12 \
  /opt/claude/bin/claude --version
```

### Network Debugging

```bash
# Test from inside Stromboli container
podman exec stromboli curl -v http://localhost:8080/health

# Check DNS resolution
podman exec stromboli nslookup ghcr.io

# Test external connectivity
podman exec stromboli curl -I https://api.anthropic.com
```

---

## Getting Help

### Information to Include

When reporting issues, include:

1. **Stromboli version**: `./stromboli --version`
2. **Podman version**: `podman version`
3. **OS/Distribution**: `cat /etc/os-release`
4. **Error message**: Full error text
5. **Request that failed**: (redact sensitive data)
6. **Logs**: `podman logs stromboli`
7. **Health check output**: `curl http://localhost:8080/health`

### Resources

- [GitHub Issues](https://github.com/tomblancdev/stromboli/issues) - Bug reports
- [Documentation](https://tomblancdev.github.io/stromboli) - Full docs
- [API Reference](../api/overview.md) - Endpoint details

---

## FAQ

### Can I use Docker instead of Podman?

Yes! Mount the Docker socket instead:

```yaml
volumes:
  - /var/run/docker.sock:/run/podman/podman.sock
```

Remove `userns_mode: keep-id` from compose file.

### How do I update Stromboli?

```bash
# Pull latest image
podman pull ghcr.io/tomblancdev/stromboli:latest

# Restart container
podman-compose down && podman-compose up -d
```

### How do I backup sessions?

```bash
# Sessions are stored in configured directory
cp -r /path/to/sessions /backup/

# Or use volume backup
podman volume export stromboli-sessions > sessions-backup.tar
```

### Can multiple users share one instance?

Yes, with authentication enabled. Each user gets separate sessions. Use JWT tokens for user isolation.

### How do I limit API costs?

```bash
# Set budget per request
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "...",
    "claude": {"max_budget_usd": 0.10}
  }'
```

### Why are my containers not cleaning up?

Check cleanup configuration:

```yaml
jobs:
  cleanup_ttl: "1h"      # Delete completed jobs after 1 hour
  cleanup_interval: "5m"  # Check every 5 minutes
```

Containers are cleaned when jobs are cleaned.
