# Installation

## Requirements

| Component | Version | Notes |
|-----------|---------|-------|
| Podman | 4.0+ | Container runtime |
| Claude CLI | Latest | Authenticated with `claude` |
| Go | 1.22+ | Only for building from source |

## Installation Methods

### Docker Compose (Recommended)

The easiest way to run Stromboli:

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli
docker compose up -d
```

This starts:

- Stromboli API on port 8080
- Automatic credential sync
- Persistent session storage

### Pre-built Binary

Download from [GitHub Releases](https://github.com/tomblancdev/stromboli/releases):

```bash
# Linux amd64
curl -LO https://github.com/tomblancdev/stromboli/releases/latest/download/stromboli-linux-amd64
chmod +x stromboli-linux-amd64
./stromboli-linux-amd64
```

### From Source

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli

# Build
make build

# Or with Go directly
go build -o stromboli ./cmd/stromboli
```

### Docker Image

```bash
# Pull the image
docker pull ghcr.io/tomblancdev/stromboli:latest

# Run with Podman socket
docker run -d \
  -p 8080:8080 \
  -v /run/podman/podman.sock:/run/podman/podman.sock \
  -v ~/.claude:/app/.claude:ro \
  ghcr.io/tomblancdev/stromboli:latest
```

## Verify Installation

```bash
# Check health
curl http://localhost:8080/health

# Check Claude status
curl http://localhost:8080/claude/status
```

## Claude CLI Setup

Stromboli requires an authenticated Claude CLI:

```bash
# Install Claude CLI
npm install -g @anthropic-ai/claude-code

# Authenticate
claude

# Verify
claude --version
```

The credentials are stored in `~/.claude/.credentials.json`.

## Next Steps

- [Configuration](configuration.md) - Customize Stromboli
- [Quick Start](quickstart.md) - Run your first agent
