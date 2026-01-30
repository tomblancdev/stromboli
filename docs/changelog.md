# Changelog

All notable changes to Stromboli will be documented here.

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
