# Stromboli Security Documentation

> **Version:** 1.0
> **Last Updated:** February 2026
> **Scope:** Complete security architecture and controls for Stromboli container orchestration API

## Overview

Stromboli is a container orchestration API designed to spawn and manage isolated Claude Code agents in Podman containers. The security architecture follows defense-in-depth principles with multiple layers of protection for:

- **Container isolation** - Each agent runs in an isolated Podman container
- **Session separation** - Sessions are isolated with separate filesystems
- **Input validation** - All user inputs are validated before processing
- **Authentication** - JWT-based auth with token blacklist support
- **Resource limits** - Memory, CPU, and timeout constraints prevent resource abuse
- **Secret management** - Credentials are handled via Podman's native secret store

## Threat Model

### Assets to Protect

| Asset | Sensitivity | Description |
|-------|-------------|-------------|
| Claude API credentials | **Critical** | OAuth tokens stored in Podman secrets |
| User-provided secrets | **High** | API keys, tokens injected into containers |
| Session data | **Medium** | Conversation history, code artifacts |
| Host filesystem | **High** | Must prevent container escape/traversal |
| API tokens | **High** | JWT tokens and legacy API tokens |
| Container runtime | **High** | Podman socket access |

### Trust Boundaries

```
+------------------+     +-------------------+     +------------------+
|  External Users  | --> |   Stromboli API   | --> |  Podman Runtime  |
|   (Untrusted)    |     | (Partially Trusted)|    |    (Trusted)     |
+------------------+     +-------------------+     +------------------+
                                |
                                v
                         +-------------------+
                         | Agent Containers  |
                         |  (Semi-Trusted)   |
                         +-------------------+
```

### Threat Actors

1. **Malicious API users** - Authenticated users attempting privilege escalation
2. **Network attackers** - Attempting to exploit unauthenticated endpoints
3. **Compromised agents** - Container attempting to escape isolation
4. **Supply chain attacks** - Malicious container images

### Attack Surfaces

| Surface | Risk Level | Mitigations |
|---------|------------|-------------|
| REST API | High | Auth, rate limiting, input validation |
| Container runtime | High | Image allowlist, resource limits, rootless Podman |
| Volume mounts | High | Allowlist, path validation, blocked paths |
| Compose files | Medium | Security validation, blocked configurations |
| Session filesystem | Medium | Path traversal prevention, isolation |

## Security Controls

### Authentication & Authorization

#### JWT Authentication

Stromboli uses HS256-signed JWT tokens with the following security measures:

```go
// Token structure includes JTI for blacklist support
type Claims struct {
    jwt.RegisteredClaims
    IsRefresh bool `json:"is_refresh,omitempty"`
}
```

**Security features:**
- **Signing method validation**: Only HMAC (HS256) is accepted, preventing algorithm confusion attacks
- **Refresh/access token separation**: Refresh tokens cannot be used for API access
- **Token blacklist**: Supports logout via JTI-based blacklist with automatic expiry cleanup
- **Configurable expiry**: Default 24h for access, 7 days for refresh

**Configuration:**
```yaml
jwt:
  secret: "your-secret-key"  # MUST be cryptographically random, >=32 bytes
  access_expiry: "24h"
  refresh_expiry: "168h"
```

**Recommendations:**
- Always set `auth.enabled: true` in production
- Use `openssl rand -base64 32` to generate JWT secret
- Rotate JWT secrets periodically
- Never log or expose JWT secrets

#### Legacy Token Support

For backward compatibility, Stromboli supports static API tokens. This is **not recommended** for production.

### Rate Limiting

IP-based rate limiting protects against abuse:

```go
type RateLimitConfig struct {
    Enabled         bool
    Rate            int           // Requests per period
    Period          time.Duration // Time period
    Burst           int           // Maximum burst
    CleanupInterval time.Duration // Stale IP cleanup interval
}
```

**Security considerations:**
- Rate limiters are per-IP, using `X-Forwarded-For` or `X-Real-IP` headers
- **Warning**: IP spoofing possible if reverse proxy not properly configured
- Stale entries are automatically cleaned up to prevent memory exhaustion

**Recommended production settings:**
```yaml
rate_limit:
  enabled: true
  rate: 50     # Requests per second
  burst: 100   # Allow short bursts
```

### Container Isolation

#### Rootless Podman

All containers run as the host user via `--userns=keep-id`:

```go
podmanBuilder.WithKeepID().WithUser(currentUser)
```

**Security benefits:**
- No root privileges inside containers
- UID/GID mapping preserves file ownership
- Reduces container escape impact

#### Resource Limits

Containers have configurable resource constraints:

```go
type ResourceDefaults struct {
    Memory  string // e.g., "512m"
    CPUs    string // e.g., "1"
    Timeout string // e.g., "30m"
}
```

**Default limits:**
- Memory: 512MB
- CPUs: 1 core
- Timeout: 30 minutes

**Recommendations:**
- Set appropriate limits based on workload
- Monitor for containers hitting limits
- Use `cpu_shares` for relative prioritization

#### Image Validation

Images are validated against configurable allowlists:

```go
type ImageValidator struct {
    patterns       []string
    defaultImage   string
    checkCompat    bool
}
```

**Validation checks:**
1. Pattern matching with glob support (`python:*`, `ghcr.io/org/*`)
2. Compatibility checking (blocks Alpine/musl images when mounting Claude CLI)
3. Name format validation

**Configuration:**
```yaml
agent:
  image: "stromboli-agent"
  allowed_image_patterns:
    - "python:*"
    - "node:*"
    - "ghcr.io/your-org/*"
```

**Note:** If `allowed_image_patterns` is empty, **all images are allowed**. Always configure an allowlist in production.

### Input Validation

#### Workdir Validation

Working directories are validated for shell safety:

```go
var validWorkdirPattern = regexp.MustCompile(`^[a-zA-Z0-9/_.\-]+$`)

func validateWorkdir(workdir string) error {
    // Must be absolute path
    // Max 4096 characters (PATH_MAX)
    // No path traversal (..)
    // Only safe characters allowed
}
```

**Blocked patterns:**
- Relative paths
- Path traversal (`..`)
- Shell metacharacters (`;`, `|`, `&`, `$`, etc.)
- Null bytes

#### Volume Mount Validation

Volumes undergo multi-layer validation:

```go
func (r *PodmanRunner) validateVolumes(volumes []string) error {
    // 1. Format validation (host:container[:options])
    // 2. Host path against allowlist
    // 3. Container path against blocklist
    // 4. Mount options against allowlist
}
```

**Blocked container paths:**
```go
var blockedContainerPaths = map[string]bool{
    "/home/user/.claude":     true, // Claude credentials
    "/home/user/.ssh":        true, // SSH keys
    "/home/user/.gnupg":      true, // GPG keys
    "/home/user/.aws":        true, // AWS credentials
    "/etc":                   true, // System config
    "/root":                  true, // Root home
    "/bin":                   true, // System binaries
    // ... and more
}
```

**Allowed mount options:**
```go
var allowedMountOptions = map[string]bool{
    "ro":       true, // Read-only
    "rw":       true, // Read-write
    "z":        true, // SELinux shared
    "Z":        true, // SELinux private
    "noexec":   true, // No execution
    "nosuid":   true, // No setuid
    "nodev":    true, // No device files
    // ... propagation modes
}
```

**Configuration:**
```yaml
agent:
  allowed_volumes:
    - "/home/user/projects"
    - "/data/shared"
  allow_all_volumes: false  # NEVER set to true in production
```

#### Lifecycle Hooks Validation

Hooks are validated for security and sanity:

```go
const (
    MaxHookCommandArgs = 100    // Max args per command
    MaxHookArgLength   = 4096   // Max single arg length
    MaxTotalHookLength = 65536  // Max total hook size
    MaxTotalHookArgs   = 200    // Max total args across hooks
    MaxHooksTimeout    = 1h     // Max timeout duration
)
```

**Validation checks:**
- No empty commands or arguments
- No null bytes (would be truncated by shell)
- Length limits enforced
- Timeout duration validated

**Shell escaping:**
```go
func EscapeShellArg(arg string) string {
    return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}
```

This single-quote escaping prevents:
- Variable expansion (`$VAR`)
- Command substitution (`` `cmd` ``)
- Glob expansion (`*`, `?`)
- Special characters (`;`, `|`, `&`)

### Secret Management

#### Claude Credentials

Credentials are stored using Podman's native secret store:

```go
func (m *Manager) CreateSecret(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "podman", "secret", "create",
        m.secretName, m.credentialsFile)
    // ...
}
```

**Security features:**
- Secrets are encrypted at rest by Podman
- Mounted as files inside containers (not env vars by default)
- Hash-based change detection for credential rotation

#### User Secrets API

The secrets registry validates all inputs:

```go
var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const (
    MaxSecretNameLength = 253
    MaxSecretValueSize  = 1 * 1024 * 1024 // 1MB
)
```

**Security measures:**
- Values passed via stdin (never in process arguments)
- Name validation prevents command injection
- Size limits prevent DoS
- Secret values never returned by API (only metadata)

**Dangerous environment variables blocked:**
```go
var dangerousEnvVars = map[string]bool{
    "LD_PRELOAD":      true,
    "LD_LIBRARY_PATH": true,
}
```

### Compose Security

#### File Validation

Compose files undergo extensive security validation:

```go
type FileValidator struct {
    config Config
}
```

**Path validation:**
- Must be absolute path
- Must be `.yml` or `.yaml` extension
- No path traversal sequences
- Symlinks resolved and validated
- TOCTOU protection (file handle kept open during parsing)

**Size limit:**
```go
const maxComposeFileSize = 1 * 1024 * 1024 // 1MB
```

#### Blocked Configurations

By default, the following are blocked:

| Configuration | Risk | Default |
|---------------|------|---------|
| `privileged: true` | Container escape | Blocked |
| `cap_add: ALL` | Full capabilities | Blocked |
| `cap_add: SYS_ADMIN` | Near-root access | Blocked |
| `network_mode: host` | Network namespace escape | Blocked |
| `ipc: host` | IPC namespace sharing | Blocked |
| `pid: host` | PID namespace sharing | Blocked |
| `userns_mode: host` | User namespace escape | Blocked |
| `devices: [...]` | Device access | Blocked |
| `security_opt: seccomp:unconfined` | Disabled seccomp | Blocked |
| `security_opt: apparmor:unconfined` | Disabled AppArmor | Blocked |
| Host volume mounts | Filesystem access | Blocked |
| Dangerous sysctls (`kernel.*`, `net.*`, etc.) | Kernel tampering | Blocked |

**Configuration to allow (NOT RECOMMENDED):**
```yaml
compose:
  allow_privileged: false   # Keep false in production
  allow_host_network: false # Keep false in production
  allow_host_volumes: false # Keep false in production
```

#### Service Isolation

Compose exec operations validate service access:

```go
func (m *Manager) Exec(ctx context.Context, sessionID, service string, cmd []string) {
    // Validate service matches stack's configured service
    if service != stack.Service {
        return nil, fmt.Errorf("service %q not allowed", service)
    }
}
```

### Session Security

#### Session ID Validation

Session IDs are validated to prevent path traversal:

```go
func validateSessionID(sessionID string) error {
    if sessionID == "" {
        return ErrSessionIDRequired
    }
    if strings.Contains(sessionID, "/") ||
       strings.Contains(sessionID, "..") ||
       strings.ContainsAny(sessionID, "\x00") {
        return ErrInvalidSessionID
    }
    return nil
}
```

#### Session Isolation

Each session gets isolated storage:

```
/sessions/{sessionID}/
  ├── .claude/                 # Claude config directory
  │   └── .credentials.json    # Mounted via Podman secret
  ├── .stromboli-initialized   # Init marker file
  └── .stromboli-init.lock     # flock-based init lock
```

**Security features:**
- Directory permissions: 0700 (owner only)
- Marker file permissions: 0600
- flock-based mutual exclusion for initialization
- Session data destroyed with `DestroySession()`

#### History Access

Session history reader validates session IDs before file access:

```go
func (r *Reader) findSessionFile(sessionID string) (string, error) {
    // Fixed path structure prevents traversal
    sessionPath := filepath.Join(r.sessionsDir, sessionID,
        ".claude", "projects", "-workspace", sessionID+".jsonl")
    // ...
}
```

## Security Configuration

### Recommended Production Settings

```yaml
server:
  address: ":8080"

agent:
  image: "your-registry/stromboli-agent"
  image_tag: "v1.0.0"  # Pin to specific version
  allowed_image_patterns:
    - "your-registry/*"
  allowed_volumes:
    - "/data/projects"
  allow_all_volumes: false  # NEVER true in production
  workdir_auto_create: true
  volume_auto_create: true

resources:
  memory: "1g"
  cpus: "2"
  timeout: "1h"

auth:
  enabled: true
  valid_tokens: []  # Use JWT instead

rate_limit:
  enabled: true
  rate: 50
  burst: 100

jwt:
  secret: "${STROMBOLI_JWT_SECRET}"  # From environment
  access_expiry: "24h"
  refresh_expiry: "168h"

compose:
  allow_privileged: false
  allow_host_network: false
  allow_host_volumes: false
  build_timeout: "10m"
  health_timeout: "2m"

tracing:
  enabled: true
  service_name: "stromboli-prod"
  endpoint: "jaeger:4317"
  insecure: false  # Use TLS
```

### Security Headers

While Stromboli uses Gin's default settings, consider adding these headers via reverse proxy:

```nginx
# Nginx example
add_header X-Content-Type-Options "nosniff" always;
add_header X-Frame-Options "DENY" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
add_header Content-Security-Policy "default-src 'none'" always;
```

### Environment Variables

Sensitive configuration via environment:

```bash
export STROMBOLI_JWT_SECRET="$(openssl rand -base64 32)"
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_RATE_LIMIT_ENABLED=true
```

## Known Limitations

### Current Limitations

1. **IP-based rate limiting**: Can be bypassed with IP rotation; consider API key-based rate limiting for high-security environments.

2. **No request body size limit in code**: Gin's default is 32MB. Consider setting explicit limits via middleware or reverse proxy.

3. **Metrics endpoint unauthenticated**: `/metrics` is public. Restrict access via network controls or reverse proxy.

4. **Health endpoint unauthenticated**: `/health` is public by design. May leak version information.

5. **In-memory token blacklist**: Blacklist is not persisted. Tokens may become valid again after server restart within their expiry window.

6. **No CORS configuration**: Configure CORS via reverse proxy if needed for browser-based clients.

7. **Error messages**: Some error messages may reveal internal paths. Consider sanitizing in production.

8. **Volume allowlist bypass**: If `allow_all_volumes: true`, no volume restrictions apply. Never use in production.

9. **Container image pull**: The `/images/pull` endpoint can pull arbitrary images (subject to name validation). Consider additional restrictions.

10. **Webhook security**: Async job webhooks don't support authentication. Use internal networks or add HMAC signing.

### Platform Limitations

- **Unix-only session locking**: `flock(2)` is used for init synchronization, only available on Unix-like systems
- **Rootless Podman required**: Security model depends on rootless container execution
- **Claude CLI compatibility**: Some base images (Alpine, distroless) are incompatible when mounting Claude CLI

## Security Checklist for Operators

### Deployment Checklist

- [ ] Enable authentication (`auth.enabled: true`)
- [ ] Set strong JWT secret (>=32 bytes, random)
- [ ] Configure volume allowlist (non-empty `allowed_volumes`)
- [ ] Ensure `allow_all_volumes: false`
- [ ] Configure image allowlist (`allowed_image_patterns`)
- [ ] Enable rate limiting
- [ ] Use TLS termination (reverse proxy)
- [ ] Set appropriate resource limits
- [ ] Configure compose security (all `allow_*: false`)
- [ ] Review network access to `/metrics` and `/health`
- [ ] Monitor logs for security events
- [ ] Enable tracing for audit trail

### Operational Security

- [ ] Rotate JWT secrets periodically
- [ ] Rotate Claude API credentials as needed
- [ ] Monitor for failed authentication attempts
- [ ] Review container images for vulnerabilities
- [ ] Keep Podman and host OS updated
- [ ] Back up session data securely
- [ ] Implement log retention policy
- [ ] Set up alerting for anomalous activity

### Network Security

- [ ] Use TLS for all external connections
- [ ] Restrict API access to authorized networks
- [ ] Use network policies in Kubernetes/Podman
- [ ] Configure firewall rules appropriately
- [ ] Consider API gateway for additional controls

## Reporting Security Issues

If you discover a security vulnerability in Stromboli, please report it responsibly:

1. **Do not** create a public GitHub issue
2. Email security concerns to the maintainers
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

We aim to acknowledge reports within 48 hours and provide a fix timeline within 7 days.

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | Feb 2026 | Initial security documentation |
