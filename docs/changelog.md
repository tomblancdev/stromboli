# Changelog

All notable changes to Stromboli will be documented here.

## [Unreleased]

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
