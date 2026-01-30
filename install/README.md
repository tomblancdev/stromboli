# Stromboli Installation

Container orchestration API for Claude Code agents.

## Quick Start

### One-Line Installer (Recommended)

```bash
curl -sL https://raw.githubusercontent.com/tomblancdev/stromboli/main/install.sh | bash
```

Works with both **Docker** and **Podman**.

### Manual Setup

```bash
# Download files
curl -O https://raw.githubusercontent.com/tomblancdev/stromboli/main/install/docker-compose.yml
curl -O https://raw.githubusercontent.com/tomblancdev/stromboli/main/install/stromboli.example.yaml

# Start
docker compose up -d   # or: podman-compose up -d

# Check health
curl http://localhost:8080/health
```

## Prerequisites

1. **Docker or Podman** with Compose support

2. **Podman socket** (if using Podman):
   ```bash
   systemctl --user enable --now podman.socket
   ```

3. **Claude credentials** at `~/.claude/.credentials.json`

## Configuration

Edit `stromboli.example.yaml` or use environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_SERVER_ADDRESS` | `:8080` | Listen address |
| `STROMBOLI_RESOURCES_MEMORY` | `512m` | Memory per agent |
| `STROMBOLI_RESOURCES_TIMEOUT` | `30m` | Execution timeout |
| `STROMBOLI_AUTH_ENABLED` | `false` | Enable auth |

## API Usage

```bash
# Run Claude
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello!"}'

# Async execution
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Write a poem"}'
```

## Documentation

https://tomblancdev.github.io/stromboli
