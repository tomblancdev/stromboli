# Docker / Docker Compose

How to run Stromboli in a container using Docker or Podman Compose.

## Docker Compose (recommended)

The fastest way to get Stromboli running:

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli
docker compose up -d
```

### Full compose file

```yaml
services:
  stromboli:
    image: ghcr.io/tomblancdev/stromboli:latest
    ports:
      - "8080:8080"
    volumes:
      # Podman socket (required)
      - ${XDG_RUNTIME_DIR:-/run/user/${UID:-1000}}/podman/podman.sock:/run/podman/podman.sock
      # Claude credentials (read-only)
      - ${HOME}/.claude:/home/stromboli/.claude:ro
      # Session persistence
      - ${HOME}/.stromboli/sessions:/app/sessions
    environment:
      CONTAINER_HOST: "unix:///run/podman/podman.sock"
      STROMBOLI_SERVER_ADDRESS: ":8080"
      STROMBOLI_AGENT_IMAGE: "ghcr.io/tomblancdev/stromboli-agent"
      STROMBOLI_AGENT_MOUNT_CLAUDE_CLI: "true"
      STROMBOLI_AGENT_AUTO_PULL_CLI: "true"
      STROMBOLI_AGENT_CREDENTIALS_FILE: "/home/stromboli/.claude/.credentials.json"
      STROMBOLI_AGENT_SESSIONS_DIR: "/app/sessions"
      STROMBOLI_AGENT_SESSIONS_HOST_DIR: "${HOME}/.stromboli/sessions"
      STROMBOLI_AGENT_ALLOWED_VOLUMES: "${HOME}/projects,${HOME}/.stromboli/sessions"
    userns_mode: keep-id
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    restart: unless-stopped
```

!!! info "Two session paths"
    When Stromboli runs in a container, you need both `SESSIONS_DIR` (path inside the Stromboli container) and `SESSIONS_HOST_DIR` (path on the host) pointing to the same data via bind mount. This lets agent containers mount session data from the host.

### Volume mounts explained

| Host path | Container path | Purpose |
|---|---|---|
| Podman socket | `/run/podman/podman.sock` | Container orchestration API |
| `~/.claude` | `/home/stromboli/.claude` | Claude credentials (read-only) |
| `~/.stromboli/sessions` | `/app/sessions` | Session persistence |

## Plain Docker

If you prefer a single `docker run` command:

```bash
docker run -d \
  --name stromboli \
  -p 8080:8080 \
  -v ${XDG_RUNTIME_DIR}/podman/podman.sock:/run/podman/podman.sock \
  -v ~/.claude:/home/stromboli/.claude:ro \
  -v ~/.stromboli/sessions:/app/sessions \
  ghcr.io/tomblancdev/stromboli:latest
```

## Build from source

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli
make build-server-image
```

## Production compose

Add authentication, TLS, and observability for production. See [production hardening](../security/production.md) for the full checklist.

```yaml
services:
  stromboli:
    image: ghcr.io/tomblancdev/stromboli:v0.3.0-alpha  # pin version
    env_file: .env
    environment:
      STROMBOLI_AUTH_ENABLED: "true"
      STROMBOLI_RATE_LIMIT_ENABLED: "true"
      STROMBOLI_TRACING_ENABLED: "true"
      STROMBOLI_TRACING_ENDPOINT: "jaeger:4317"
    # ... volumes and other config same as above
    depends_on:
      - jaeger

  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "4317:4317"
```

Store secrets in a `.env` file (never commit to git):

```bash
# .env
STROMBOLI_JWT_SECRET=your-256-bit-secret
STROMBOLI_API_TOKENS=your-api-token
```

## Common commands

```bash
# Start
docker compose up -d

# View logs
docker compose logs -f stromboli

# Stop
docker compose down

# Update
docker compose pull && docker compose up -d

# Check health
curl localhost:8080/health
```

## Using Docker instead of Podman

Mount the Docker socket instead:

```yaml
volumes:
  - /var/run/docker.sock:/run/podman/podman.sock
```

Remove `userns_mode: keep-id` from the compose file.

## Troubleshooting

### "Cannot connect to Podman"

Check the socket is mounted:
```bash
docker exec stromboli ls -la /run/podman/podman.sock
```

### "Claude not configured"

Check credentials are mounted:
```bash
docker exec stromboli ls -la /home/stromboli/.claude/
```

### Permission issues

The container runs as non-root (UID 1000). Make sure mounted directories are readable:
```bash
chmod 755 ~/.stromboli/sessions
```

For SELinux systems (Fedora/RHEL), add `:Z` to volume mounts.
