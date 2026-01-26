# Changelog

All notable changes to Stromboli will be documented here.

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
