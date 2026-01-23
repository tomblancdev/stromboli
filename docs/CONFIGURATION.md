# Configuration Guide

Stromboli uses [Viper](https://github.com/spf13/viper) for configuration management, supporting multiple configuration sources with the following priority:

1. **Environment variables** (highest priority)
2. **Configuration file** (YAML)
3. **Default values** (lowest priority)

## Quick Start

### Using Environment Variables

The simplest way to configure Stromboli is via environment variables:

```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_API_TOKENS="token1,token2"
export STROMBOLI_DEFAULT_MEMORY="1g"
export STROMBOLI_DEFAULT_CPUS="2"

./stromboli
```

### Using Configuration File

Create a `stromboli.yaml` file in one of these locations:
- Current directory: `./stromboli.yaml`
- User home: `~/.stromboli/stromboli.yaml`
- System-wide: `/etc/stromboli/stromboli.yaml`

Example configuration:

```yaml
server:
  address: ":8080"

agent:
  image: "stromboli-agent:latest"
  secrets_file: ".claude-secrets"
  sessions_dir: ".stromboli/sessions"

resources:
  memory: "1g"
  cpus: "2"
  timeout: "1h"

auth:
  enabled: true
  valid_tokens:
    - "your-secure-token-here"

rate_limit:
  enabled: true
  rate: 50
  burst: 100

jwt:
  secret: "your-jwt-secret-key"
  access_expiry: "24h"
  refresh_expiry: "168h"

jobs:
  cleanup_ttl: "2h"
  cleanup_interval: "10m"
```

## Configuration Reference

### Server Configuration

HTTP server settings.

| Setting | Environment Variable | YAML Path | Default | Description |
|---------|---------------------|-----------|---------|-------------|
| Address | `STROMBOLI_SERVER_ADDRESS` | `server.address` | `:8080` | Server listen address (host:port) |

**Example:**
```yaml
server:
  address: ":9090"  # Listen on port 9090
```

```bash
export STROMBOLI_SERVER_ADDRESS=":9090"
```

### Agent Configuration

Container agent settings for Claude Code execution.

| Setting | Environment Variable | YAML Path | Default | Description |
|---------|---------------------|-----------|---------|-------------|
| Image | `STROMBOLI_AGENT_IMAGE` | `agent.image` | `stromboli-agent:latest` | Container image name for Claude Code |
| Image Tag | `STROMBOLI_AGENT_IMAGE_TAG` | `agent.image_tag` | `latest` | Container image tag (pin to specific version for production) |
| Secrets File | `STROMBOLI_AGENT_SECRETS_FILE` | `agent.secrets_file` | `.claude-secrets` | Path to Claude API token file |
| Sessions Dir | `STROMBOLI_AGENT_SESSIONS_DIR` | `agent.sessions_dir` | `.stromboli/sessions` | Directory for session data |

**Example:**
```yaml
agent:
  image: "ghcr.io/your-org/claude-agent"
  image_tag: "v1.0.0"
  secrets_file: "/secrets/claude-token"
  sessions_dir: "/data/sessions"
```

```bash
export STROMBOLI_AGENT_IMAGE="ghcr.io/your-org/claude-agent"
export STROMBOLI_AGENT_IMAGE_TAG="v1.0.0"
export STROMBOLI_AGENT_SECRETS_FILE="/secrets/claude-token"
export STROMBOLI_AGENT_SESSIONS_DIR="/data/sessions"
```

### Resource Configuration

Default resource limits for spawned containers. Users can override these per request.

| Setting | Environment Variable | YAML Path | Default | Description |
|---------|---------------------|-----------|---------|-------------|
| Memory | `STROMBOLI_DEFAULT_MEMORY` | `resources.memory` | `512m` | Default memory limit (e.g., `512m`, `1g`, `2g`) |
| CPUs | `STROMBOLI_DEFAULT_CPUS` | `resources.cpus` | `1` | Default CPU limit (e.g., `1`, `2`, `4`) |
| Timeout | `STROMBOLI_DEFAULT_TIMEOUT` | `resources.timeout` | `30m` | Default execution timeout (e.g., `30m`, `1h`, `2h`) |

**Example:**
```yaml
resources:
  memory: "2g"    # 2 GB RAM
  cpus: "4"       # 4 CPU cores
  timeout: "2h"   # 2 hour timeout
```

```bash
export STROMBOLI_DEFAULT_MEMORY="2g"
export STROMBOLI_DEFAULT_CPUS="4"
export STROMBOLI_DEFAULT_TIMEOUT="2h"
```

### Authentication Configuration

Control access to the API using tokens or JWT.

| Setting | Environment Variable | YAML Path | Default | Description |
|---------|---------------------|-----------|---------|-------------|
| Enabled | `STROMBOLI_AUTH_ENABLED` | `auth.enabled` | `false` | Enable authentication |
| Valid Tokens | `STROMBOLI_API_TOKENS` | `auth.valid_tokens` | `[]` | Comma-separated list of valid Bearer tokens (legacy) |

**Example:**
```yaml
auth:
  enabled: true
  valid_tokens:
    - "prod-token-1"
    - "prod-token-2"
```

```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_API_TOKENS="prod-token-1,prod-token-2"
```

**Usage:**
```bash
curl -H "Authorization: Bearer prod-token-1" \
  http://localhost:8080/api/v1/spawn
```

### Rate Limiting Configuration

Protect the API from abuse with rate limiting.

| Setting | Environment Variable | YAML Path | Default | Description |
|---------|---------------------|-----------|---------|-------------|
| Enabled | `STROMBOLI_RATE_LIMIT_ENABLED` | `rate_limit.enabled` | `false` | Enable rate limiting |
| Rate | `STROMBOLI_RATE_LIMIT_RPS` | `rate_limit.rate` | `10` | Requests per second allowed |
| Burst | `STROMBOLI_RATE_LIMIT_BURST` | `rate_limit.burst` | `20` | Maximum burst size |

**Example:**
```yaml
rate_limit:
  enabled: true
  rate: 50     # 50 requests per second
  burst: 100   # Allow bursts up to 100
```

```bash
export STROMBOLI_RATE_LIMIT_ENABLED=true
export STROMBOLI_RATE_LIMIT_RPS=50
export STROMBOLI_RATE_LIMIT_BURST=100
```

### JWT Configuration

Modern JWT-based authentication (recommended over legacy tokens).

| Setting | Environment Variable | YAML Path | Default | Description |
|---------|---------------------|-----------|---------|-------------|
| Secret | `STROMBOLI_JWT_SECRET` | `jwt.secret` | `""` | Secret key for signing JWT tokens |
| Access Expiry | `STROMBOLI_JWT_EXPIRY` | `jwt.access_expiry` | `24h` | Access token lifetime |
| Refresh Expiry | `STROMBOLI_JWT_REFRESH_EXPIRY` | `jwt.refresh_expiry` | `168h` | Refresh token lifetime (7 days) |

**Example:**
```yaml
jwt:
  secret: "your-256-bit-secret-key-here"
  access_expiry: "12h"
  refresh_expiry: "72h"
```

```bash
export STROMBOLI_JWT_SECRET="your-256-bit-secret-key-here"
export STROMBOLI_JWT_EXPIRY="12h"
export STROMBOLI_JWT_REFRESH_EXPIRY="72h"
```

**Note:** JWT authentication is automatically enabled when `jwt.secret` is set. For production use, prefer JWT over legacy tokens.

See [AUTHENTICATION.md](./AUTHENTICATION.md) for complete JWT setup and usage.

### Job Configuration

Async job management settings.

| Setting | Environment Variable | YAML Path | Default | Description |
|---------|---------------------|-----------|---------|-------------|
| Cleanup TTL | `STROMBOLI_JOBS_CLEANUP_TTL` | `jobs.cleanup_ttl` | `1h` | How long to keep completed jobs |
| Cleanup Interval | `STROMBOLI_JOBS_CLEANUP_INTERVAL` | `jobs.cleanup_interval` | `5m` | How often to run cleanup |

**Example:**
```yaml
jobs:
  cleanup_ttl: "2h"      # Keep jobs for 2 hours
  cleanup_interval: "10m" # Clean up every 10 minutes
```

```bash
export STROMBOLI_JOBS_CLEANUP_TTL="2h"
export STROMBOLI_JOBS_CLEANUP_INTERVAL="10m"
```

### Tracing Configuration (v1.4+)

OpenTelemetry distributed tracing for observability.

| Setting | Environment Variable | YAML Path | Default | Description |
|---------|---------------------|-----------|---------|-------------|
| Enabled | `STROMBOLI_TRACING_ENABLED` | `tracing.enabled` | `false` | Enable OpenTelemetry tracing |
| Service Name | `STROMBOLI_TRACING_SERVICE_NAME` | `tracing.service_name` | `stromboli` | Service name in traces |
| Endpoint | `STROMBOLI_TRACING_ENDPOINT` | `tracing.endpoint` | `localhost:4317` | OTLP collector endpoint (gRPC) |
| Insecure | `STROMBOLI_TRACING_INSECURE` | `tracing.insecure` | `true` | Use insecure connection (no TLS) |

**Example:**
```yaml
tracing:
  enabled: true
  service_name: "stromboli-prod"
  endpoint: "jaeger:4317"
  insecure: true  # Set to false for production with TLS
```

```bash
export STROMBOLI_TRACING_ENABLED=true
export STROMBOLI_TRACING_SERVICE_NAME="stromboli-prod"
export STROMBOLI_TRACING_ENDPOINT="jaeger:4317"
export STROMBOLI_TRACING_INSECURE=true
```

**Compatible collectors:**
- Jaeger (with OTLP receiver)
- Grafana Tempo
- OpenTelemetry Collector
- Any OTLP-compatible backend

## Configuration Examples

### Development Configuration

Minimal setup for local development:

```yaml
# stromboli.yaml
server:
  address: ":8080"

agent:
  image: "stromboli-agent:latest"
  secrets_file: ".claude-secrets"
  sessions_dir: ".stromboli/sessions"

auth:
  enabled: false

rate_limit:
  enabled: false
```

Or with environment variables only:
```bash
./stromboli  # Uses all defaults
```

### Production Configuration

Secure production setup with authentication and rate limiting:

```yaml
# /etc/stromboli/stromboli.yaml
server:
  address: ":8080"

agent:
  image: "ghcr.io/your-org/stromboli-agent"
  image_tag: "v1.0.0"
  secrets_file: "/run/secrets/claude-token"
  sessions_dir: "/data/stromboli/sessions"

resources:
  memory: "2g"
  cpus: "2"
  timeout: "1h"

auth:
  enabled: true
  valid_tokens: []  # Use JWT instead

rate_limit:
  enabled: true
  rate: 100
  burst: 200

jwt:
  secret: "${STROMBOLI_JWT_SECRET}"  # Set via env var
  access_expiry: "24h"
  refresh_expiry: "168h"

jobs:
  cleanup_ttl: "2h"
  cleanup_interval: "10m"
```

With secrets in environment:
```bash
export STROMBOLI_JWT_SECRET="$(openssl rand -base64 32)"
./stromboli
```

### High-Performance Configuration

Optimized for high throughput:

```yaml
server:
  address: ":8080"

resources:
  memory: "4g"
  cpus: "4"
  timeout: "2h"

rate_limit:
  enabled: true
  rate: 500
  burst: 1000

jobs:
  cleanup_ttl: "30m"
  cleanup_interval: "5m"
```

### Container/Kubernetes Configuration

Using environment variables (12-factor app style):

```bash
# All config via environment - no config file needed
export STROMBOLI_SERVER_ADDRESS=":8080"
export STROMBOLI_AGENT_IMAGE="ghcr.io/your-org/stromboli-agent"
export STROMBOLI_AGENT_IMAGE_TAG="v1.0.0"
export STROMBOLI_AGENT_SECRETS_FILE="/secrets/claude-token"
export STROMBOLI_AGENT_SESSIONS_DIR="/data/sessions"
export STROMBOLI_DEFAULT_MEMORY="2g"
export STROMBOLI_DEFAULT_CPUS="2"
export STROMBOLI_DEFAULT_TIMEOUT="1h"
export STROMBOLI_AUTH_ENABLED="true"
export STROMBOLI_JWT_SECRET="${JWT_SECRET_FROM_VAULT}"
export STROMBOLI_RATE_LIMIT_ENABLED="true"
export STROMBOLI_RATE_LIMIT_RPS="100"
export STROMBOLI_RATE_LIMIT_BURST="200"

./stromboli
```

Kubernetes example:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: stromboli-config
data:
  STROMBOLI_SERVER_ADDRESS: ":8080"
  STROMBOLI_AGENT_IMAGE: "ghcr.io/your-org/stromboli-agent"
  STROMBOLI_AGENT_IMAGE_TAG: "v1.0.0"
  STROMBOLI_DEFAULT_MEMORY: "2g"
  STROMBOLI_DEFAULT_CPUS: "2"
  STROMBOLI_AUTH_ENABLED: "true"
  STROMBOLI_RATE_LIMIT_ENABLED: "true"
  STROMBOLI_RATE_LIMIT_RPS: "100"
  STROMBOLI_RATE_LIMIT_BURST: "200"

---
apiVersion: v1
kind: Secret
metadata:
  name: stromboli-secrets
stringData:
  STROMBOLI_JWT_SECRET: "your-secret-here"
  claude-token: "your-claude-token"
```

## Backward Compatibility

All existing environment variables continue to work:

| Legacy Variable | Still Supported |
|----------------|-----------------|
| `STROMBOLI_AUTH_ENABLED` | ✅ Yes |
| `STROMBOLI_API_TOKENS` | ✅ Yes |
| `STROMBOLI_RATE_LIMIT_ENABLED` | ✅ Yes |
| `STROMBOLI_RATE_LIMIT_RPS` | ✅ Yes |
| `STROMBOLI_RATE_LIMIT_BURST` | ✅ Yes |
| `STROMBOLI_DEFAULT_MEMORY` | ✅ Yes |
| `STROMBOLI_DEFAULT_CPUS` | ✅ Yes |
| `STROMBOLI_DEFAULT_TIMEOUT` | ✅ Yes |
| `STROMBOLI_JWT_SECRET` | ✅ Yes |
| `STROMBOLI_JWT_EXPIRY` | ✅ Yes |
| `STROMBOLI_JWT_REFRESH_EXPIRY` | ✅ Yes |

No existing deployments need to be changed. The config file is purely additive.

## Configuration Priority

When the same setting is defined in multiple places, the priority is:

1. **Environment variables** - Highest priority
2. **Configuration file** - Middle priority
3. **Defaults** - Lowest priority

Example:
```yaml
# stromboli.yaml
server:
  address: ":7070"
```

```bash
export STROMBOLI_SERVER_ADDRESS=":8080"
./stromboli
```

Result: Server listens on `:8080` (environment variable wins)

## Validation

Stromboli validates configuration on startup and exits with an error if invalid:

```bash
$ export STROMBOLI_RATE_LIMIT_RPS=-5
$ ./stromboli
Error: invalid configuration: rate limit RPS must be positive, got -5
```

Common validation errors:
- Rate limit values must be positive
- JWT expiry durations must be positive
- Duration strings must be valid Go durations (e.g., `30m`, `1h`, `2h30m`)

## Getting Configuration File

Copy the example configuration:

```bash
cp stromboli.example.yaml stromboli.yaml
# Edit as needed
vim stromboli.yaml
```

Or download from repository:
```bash
curl -o stromboli.yaml \
  https://raw.githubusercontent.com/tomblanc/stromboli/main/stromboli.example.yaml
```

## Best Practices

1. **Use config files for static settings** (server address, resource defaults)
2. **Use environment variables for secrets** (JWT secret, API tokens)
3. **Enable authentication in production** (`auth.enabled: true`)
4. **Enable rate limiting in production** (`rate_limit.enabled: true`)
5. **Use JWT over legacy tokens** for better security
6. **Set appropriate resource limits** based on workload
7. **Monitor job cleanup** to prevent disk space issues
8. **Use explicit config file path** with `-config` flag if needed (future feature)

## Troubleshooting

### Configuration Not Loading

Check that your config file is in a supported location:
```bash
# Check current directory
ls -la stromboli.yaml

# Check user home
ls -la ~/.stromboli/stromboli.yaml

# Check system-wide
ls -la /etc/stromboli/stromboli.yaml
```

### Environment Variables Not Working

Ensure the `STROMBOLI_` prefix is present:
```bash
# Wrong - won't work
export AUTH_ENABLED=true

# Correct
export STROMBOLI_AUTH_ENABLED=true
```

### Duration Parse Errors

Use valid Go duration strings:
```bash
# Valid
export STROMBOLI_JWT_EXPIRY="24h"
export STROMBOLI_JWT_EXPIRY="1h30m"
export STROMBOLI_JWT_EXPIRY="90m"

# Invalid
export STROMBOLI_JWT_EXPIRY="1 hour"
export STROMBOLI_JWT_EXPIRY="24"
```

### Validation Errors

Check the startup logs for detailed error messages:
```bash
./stromboli
# Look for: Error: invalid configuration: ...
```

## Migration from Previous Versions

If you're upgrading from a version without Viper:

1. **No changes needed** - All existing env vars work
2. **Optionally** create a config file for static settings
3. **Consider** moving secrets to environment (best practice)

Example migration:
```bash
# Before (all env vars)
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_API_TOKENS="token1,token2"
export STROMBOLI_DEFAULT_MEMORY="1g"
export STROMBOLI_RATE_LIMIT_ENABLED=true
./stromboli

# After (config file + env for secrets)
# stromboli.yaml:
#   auth:
#     enabled: true
#   resources:
#     memory: "1g"
#   rate_limit:
#     enabled: true

export STROMBOLI_API_TOKENS="token1,token2"  # Secrets stay in env
./stromboli
```

## See Also

- [AUTHENTICATION.md](./AUTHENTICATION.md) - Authentication setup
- [RATE_LIMITING.md](./RATE_LIMITING.md) - Rate limiting details
- [RESOURCE_LIMITS.md](./RESOURCE_LIMITS.md) - Resource management
- [stromboli.example.yaml](../stromboli.example.yaml) - Full example config
