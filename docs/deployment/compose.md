# Docker Compose Deployment

The recommended way to deploy Stromboli.

## Quick Start

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli
docker compose up -d
```

## docker-compose.yml

```yaml
version: '3.8'

services:
  stromboli:
    image: ghcr.io/tomblancdev/stromboli:latest
    # Or build from source:
    # build:
    #   context: .
    #   dockerfile: deployments/docker/Dockerfile.server
    ports:
      - "8080:8080"
    environment:
      # Server
      - STROMBOLI_PORT=8080

      # Agent
      - STROMBOLI_AGENT_ALLOWED_WORKSPACES=/workspace
      - STROMBOLI_AGENT_DEFAULT_MEMORY=512m
      - STROMBOLI_AGENT_DEFAULT_TIMEOUT=5m

      # Optional: Authentication
      # - STROMBOLI_AUTH_ENABLED=true
      # - STROMBOLI_AUTH_JWT_SECRET=${JWT_SECRET}
      # - STROMBOLI_AUTH_API_TOKEN=${API_TOKEN}

      # Optional: Rate Limiting
      # - STROMBOLI_RATELIMIT_ENABLED=true
      # - STROMBOLI_RATELIMIT_REQUESTS_PER_SECOND=10

    volumes:
      # Podman socket (required)
      - /run/podman/podman.sock:/run/podman/podman.sock

      # Claude credentials (required)
      - ~/.claude:/app/.claude:ro

      # Session storage (persistent)
      - stromboli-sessions:/app/sessions

      # Workspaces (mount your projects)
      - ~/projects:/workspace:ro

    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

    restart: unless-stopped

volumes:
  stromboli-sessions:
```

## Production Configuration

### With Authentication & TLS

```yaml
version: '3.8'

services:
  stromboli:
    image: ghcr.io/tomblancdev/stromboli:latest
    environment:
      - STROMBOLI_PORT=8080
      - STROMBOLI_AUTH_ENABLED=true
      - STROMBOLI_AUTH_JWT_SECRET=${JWT_SECRET}
      - STROMBOLI_AUTH_API_TOKEN=${API_TOKEN}
      - STROMBOLI_RATELIMIT_ENABLED=true
      - STROMBOLI_RATELIMIT_REQUESTS_PER_SECOND=5
      - STROMBOLI_AGENT_ALLOWED_WORKSPACES=/workspace
      - STROMBOLI_AGENT_DEFAULT_MEMORY=1g
      - STROMBOLI_AGENT_DEFAULT_TIMEOUT=10m
    volumes:
      - /run/podman/podman.sock:/run/podman/podman.sock
      - ~/.claude:/app/.claude:ro
      - stromboli-sessions:/app/sessions
      - /data/projects:/workspace:ro
    restart: unless-stopped
    networks:
      - traefik
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.stromboli.rule=Host(`stromboli.example.com`)"
      - "traefik.http.routers.stromboli.tls=true"
      - "traefik.http.routers.stromboli.tls.certresolver=letsencrypt"

  traefik:
    image: traefik:v2.10
    command:
      - "--providers.docker=true"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
      - "--certificatesresolvers.letsencrypt.acme.email=you@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
    ports:
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - traefik-certs:/letsencrypt
    networks:
      - traefik

volumes:
  stromboli-sessions:
  traefik-certs:

networks:
  traefik:
```

## With Observability

```yaml
version: '3.8'

services:
  stromboli:
    image: ghcr.io/tomblancdev/stromboli:latest
    environment:
      - STROMBOLI_TRACING_ENABLED=true
      - STROMBOLI_TRACING_ENDPOINT=jaeger:4317
      - STROMBOLI_TRACING_SERVICE_NAME=stromboli
    # ... other config
    depends_on:
      - jaeger

  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # UI
      - "4317:4317"    # OTLP gRPC

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    depends_on:
      - prometheus
```

## Commands

```bash
# Start
docker compose up -d

# View logs
docker compose logs -f stromboli

# Stop
docker compose down

# Update
docker compose pull
docker compose up -d

# Restart
docker compose restart stromboli
```

## Environment File

Create a `.env` file for secrets:

```bash
# .env
JWT_SECRET=your-256-bit-secret
API_TOKEN=your-api-token
```

Then reference in compose:
```yaml
services:
  stromboli:
    env_file:
      - .env
```

!!! danger "Security"
    Never commit `.env` files to git. Add to `.gitignore`.
