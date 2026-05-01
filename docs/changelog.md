# Changelog

All notable changes to Stromboli will be documented here.

## [Unreleased]

### Added

#### Persistent Agents
- New `/agents/*` endpoint family for long-lived Claude processes with stream-json I/O — sub-second turn latency for event-driven workloads (sensor buses, on-call bots). See [API endpoints](api/endpoints.md#persistent-agents). (#60, #73)
- Per-session token usage and estimated USD cost surfaced on `RunResponse.usage` (read from session JSONL, best-effort). (#56, #72)
- `claude.effort` request field exposes the upstream CLI's thinking/agentic complexity level (`low`, `medium`, `high`, `xhigh`, `max` — accepted subset depends on model). (#74, #84)
- Env-var passthrough for runtime tuning: `claude.prompt_caching_ttl` (`5m`/`1h`), `claude.bedrock_service_tier`, `claude.enable_powershell_tool`. (#76, #77, #81, #85)
- Session titles surfaced from a `UserPromptSubmit` hook returning `hookSpecificOutput.sessionTitle`. `GET /sessions` now returns `{id, title}` records. (#83, #86)
- New `GET /sessions/:id/messages/:message_id` endpoint to fetch a single message without re-reading the whole transcript.

### Changed

- **BREAKING (operational): Auth enabled by default.** `STROMBOLI_AUTH_ENABLED` now defaults to `true`. The server fails fast at startup if `STROMBOLI_JWT_SECRET` is empty, shorter than 32 chars, or matches a known placeholder. Existing setups must either set a real JWT secret or explicitly opt out with `STROMBOLI_AUTH_ENABLED=false`. (#45, #64)
- **Logs are JSON by default** so log aggregators can parse them without preprocessing. Set `STROMBOLI_LOG_FORMAT=text` for human-friendly output during local dev. (#47, #65)
- **Log level configurable** via `STROMBOLI_LOG_LEVEL` (`debug`/`info`/`warn`/`error`). (#48, #67)
- **Tracing TLS by default**: `STROMBOLI_TRACING_INSECURE` now defaults to `false`. Plaintext OTLP only in dev. (#49, #70)
- **Metrics on a separate listener bound to localhost** (`127.0.0.1:9090` by default) — never co-located with the public API port. Run a Prometheus sidecar or override `STROMBOLI_METRICS_ADDRESS` if you need cross-pod scraping. (#46, #66)

### Fixed

- Dev compose now uses a named volume for sessions (was a bind mount that dropped state across `compose down`). (#50, #69)
- `.dockerignore` slims the build context. (#51, #68)

### Infra

- Kubernetes manifests under `deployments/kubernetes/` (namespace, ConfigMap, Secret example, Deployment, Service, Kustomization).
- Prometheus alert rules under `deployments/grafana/`.
- Dev image notes in `deployments/`. (#53, #54, #55, #71)
- CI pipeline for lint, tests, and Docker build. (#44, #63)

#### Lifecycle Hooks
- **OnCreateCommand**: Run commands once when session is first created (e.g., `pip install`)
- **PostCreate**: Run commands after OnCreateCommand completes (e.g., build steps)
- **PostStart**: Run commands on every container start (e.g., start background services)
- **Hooks Timeout**: Configurable timeout for hook execution (`hooks_timeout`)
- Hooks are chained with fail-fast behavior - if any hook fails, execution stops
- Shell escaping for all hook arguments to prevent injection attacks
- Documentation: [Lifecycle Hooks Guide](guides/lifecycle-hooks.md)

#### Compose Environments
- **Multi-service environments**: Run Claude agents in Docker/Podman Compose stacks
- **Service selection**: Specify which service Claude runs in via `environment.service`
- **Health check waiting**: Stromboli waits for all services to become healthy
- **Stack lifecycle management**: Automatic cleanup on session destroy or TTL expiry
- **Security validation**: Blocks privileged containers, host network, and dangerous configurations
- Configuration options: `allow_privileged`, `allow_host_network`, `allow_host_volumes`
- Timeout configuration: `build_timeout`, `health_timeout`, `stack_ttl`
- Documentation: [Compose Environments Guide](guides/compose-environments.md)

#### Image Discovery API
- **GET /images**: List all local images sorted by compatibility rank
- **GET /images/:name**: Inspect a specific image with detailed metadata
- **GET /images/search**: Search container registries (Docker Hub, etc.)
- **POST /images/pull**: Pull an image from a registry
- Compatibility ranking system (1-4) to identify Claude-compatible images

### Security

- Compose file validation with security checks for dangerous configurations
- Lifecycle hooks validation with length limits and shell escaping
- TOCTOU protection for compose file parsing

---

## [0.3.0-alpha] - 2026-01-31

### Changed

- **BREAKING: Renamed `workspace` to `workdir`**:
  - `workdir` sets the **working directory** inside the container (e.g., `/workspace`)
  - Use `podman.volumes` to mount host directories into the container
  - Example migration:
    ```json
    // Before (v0.2.0)
    {"workspace": "/home/user/project"}

    // After
    {
      "workdir": "/workspace",
      "podman": {"volumes": ["/home/user/project:/workspace"]}
    }
    ```

- **BREAKING: Default-deny volume security**: When `allowed_volumes` is empty, all volume mounts are now **DENIED** by default (was: allow all). Set `STROMBOLI_AGENT_ALLOW_ALL_VOLUMES=true` for development.

- **Agent entrypoint simplified**: Removed `claude` from entrypoint command. The runner now always prepends `claude` when `MOUNT_CLAUDE_CLI=true`.

### Added

- **Workdir auto-creation**: If `workdir` doesn't exist in the container, it's automatically created (configurable via `STROMBOLI_AGENT_WORKDIR_AUTO_CREATE`)
- **Volume validation**: Volume host paths are validated against `allowed_volumes` allowlist (`STROMBOLI_AGENT_ALLOWED_VOLUMES`)
- **Sessions host path**: New `STROMBOLI_AGENT_SESSIONS_HOST_DIR` config for containerized deployments where Stromboli runs inside a container
- **Symlink bypass prevention**: Host paths are resolved via `filepath.EvalSymlinks()` before validation
- **Container path blocklist**: Sensitive container paths are blocked (`/etc`, `~/.claude`, `~/.ssh`, `~/.aws`, etc.)
- **Mount options validation**: Only safe mount options allowed (`ro`, `rw`, `z`, `Z`, `noexec`, `nosuid`, `nodev`, etc.)
- **Workdir character validation**: Workdir paths validated for shell-safe characters only

### Security

- Defense-in-depth volume validation with multiple security layers
- Explicit error messages for security rejections (e.g., "Alpine/musl-based images not supported")

---

## [0.2.0-alpha] - 2026-01-30

### Added

#### Release & Distribution
- **Cross-platform binaries**: Linux, macOS, Windows (amd64 + arm64)
- **Docker images**: Published to `ghcr.io/tomblancdev/stromboli`
- **Agent image**: Separate `ghcr.io/tomblancdev/stromboli-agent` with Claude CLI
- **Native cross-compilation**: Fast ARM64 builds (no QEMU emulation)

#### Image Architecture
- **CLI image auto-pull**: Automatically pulls Claude CLI image on startup if missing
- **Dynamic image support**: Mount Claude CLI into any glibc-based container (Python, Node, Go, etc.)
- **Image compatibility checking**: Warns about incompatible Alpine/musl images

#### Documentation
- **Examples & Use Cases**: Multi-language API clients (Python, JavaScript, Go, curl)
- **CI/CD Integration**: Service container approach for full codebase access
- **Security Guide**: Threat model, TLS setup, audit logging, production checklist
- **Troubleshooting Guide**: Error reference, debugging tips, FAQ
- **OpenAPI Reference**: Interactive Swagger UI, ReDoc, downloadable specs
- **Contributing Guide**: Code architecture, request flow diagrams, testing patterns
- **Mermaid diagrams**: Visual architecture and flow diagrams

### Changed
- **Configuration**: All settings now documented with environment variables
- **Install files**: Comprehensive docker-compose.yml and stromboli.example.yaml

### CI/CD
- **Release workflow**: Automated binary + Docker builds on version tags
- **Agent image workflow**: Auto-builds when Dockerfile.claude-cli changes
- **OpenAPI validation**: Ensures specs are up-to-date
- **Versioned documentation**: Each release has frozen docs + OpenAPI specs

### Fixed
- Docker ARM64 build performance (was 15-20 min, now ~2 min)
- Documentation link validation

---

## [0.1.5-alpha] - 2025-01-26

### Added
- **Credentials Sync**: Automatic synchronization of Claude credentials with Podman secrets
- **Generic Secrets Injection**: Mount Podman secrets as environment variables via `secrets_env`
- **Input Validation**: Comprehensive validation for secrets environment variables
- **/secrets Endpoint**: List available Podman secrets via API

### Security
- Block dangerous environment variables (LD_PRELOAD, LD_LIBRARY_PATH)
- Environment variable name validation (must match `^[a-zA-Z_][a-zA-Z0-9_]*$`)
- Maximum 50 secrets per request

## [0.1.4-alpha] - 2025-01-25

### Added
- **Dynamic Container Images**: Support for multiple container images with pattern allowlist
- **Version Info**: `/version` endpoint and startup version logging
- **Container Naming**: Unique container names with `stromboli-` prefix
- **Orphan Cleanup**: Automatic cleanup of orphaned containers on startup

### Fixed
- Version injection into Docker server image during build

## [0.1.3-alpha] - 2025-01-24

### Added
- Initial public release
- Core API for running Claude Code agents
- Session management (create, resume, destroy)
- Async job execution with polling
- Workspace mounting with allowlist security
- JWT authentication support
- Rate limiting middleware
- Health check endpoint

### Security
- Container isolation via Podman
- Workspace allowlist validation
- Read-only credential mounting
