# Stromboli v1.0 Roadmap 🌋

> Last Updated: January 22, 2026
> Current Score: 9/11

## Progress Tracker

Use this checklist to track implementation progress. Update status as items are completed.

---

## 🔴 CRITICAL (Blocks v1.0 Release)

### 1. Add Session Package Tests
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: CRITICAL
- **Effort**: ~2 hours
- **Location**: `internal/session/session_test.go`

**Why Critical**: Session management is core functionality. Path traversal prevention, UUID generation, and file operations are completely untested.

**Test Cases Implemented** (21 tests):
- [x] `TestGenerateID_Format` - UUID v4 format validation
- [x] `TestGenerateID_Uniqueness` - No duplicate IDs in 1000 generations
- [x] `TestManager_Create_*` - Session directory creation (4 tests)
- [x] `TestManager_Destroy_*` - Session cleanup (3 tests)
- [x] `TestManager_List_*` - List all sessions (4 tests)
- [x] `TestManager_PathTraversal_*` - Block `../`, `/`, `./` attacks (4 tests)
- [x] `TestManager_Concurrent*` - Thread safety (2 tests)

**Completion Date**: January 22, 2026

---

## 🟡 HIGH Priority (Production Readiness)

### 2. Implement Rate Limiting
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: HIGH
- **Effort**: ~4 hours
- **Location**: `internal/api/ratelimit.go`

**Why Important**: API currently has no protection against abuse or DoS attacks.

**Implementation**:
```go
import "golang.org/x/time/rate"
// Per-IP rate limiting with token bucket algorithm
```

**Tasks Completed**:
- [x] Add `golang.org/x/time/rate` dependency
- [x] Create rate limit middleware (`internal/api/ratelimit.go`)
- [x] Apply to protected routes (excludes /health and /metrics)
- [x] Add configuration options (env vars: STROMBOLI_RATE_LIMIT_ENABLED, _RPS, _BURST)
- [x] Add tests (`internal/api/ratelimit_test.go`)
- [x] Update documentation (`docs/RATE_LIMITING.md`)

**Completion Date**: January 22, 2026

---

### 3. Set Default Resource Limits
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: HIGH
- **Effort**: ~1 hour
- **Location**: `cmd/stromboli/main.go`, `internal/runner/runner.go`

**Why Important**: Without defaults, containers can exhaust host resources.

**Implementation**:
```go
const (
    defaultMemory   = "512m"
    defaultCPUs     = "1"
    defaultTimeout  = "30m"
)
```

**Tasks Completed**:
- [x] Add default constants to main.go
- [x] Create `ResourceDefaults` struct in runner package
- [x] Add `NewPodmanRunnerWithDefaults()` function
- [x] Maintain backward compatibility with `NewPodmanRunner()`
- [x] Apply defaults in `runner.Run()` and `runner.RunStream()` methods
- [x] Allow override via environment variables (STROMBOLI_DEFAULT_MEMORY, _CPUS, _TIMEOUT)
- [x] Add comprehensive unit tests (4 tests covering defaults, overrides, partial overrides, streaming)
- [x] Document default limits in `docs/RESOURCE_LIMITS.md`

**Completion Date**: January 22, 2026

---

### 4. Add Integration Test Build Tags
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: HIGH
- **Effort**: ~30 minutes
- **Location**: `internal/container/container_test.go`, other integration tests

**Why Important**: Tests fail in CI/CD without Podman installed.

**Implementation**:
```go
//go:build integration

package container
```

**Tasks Completed**:
- [x] Add build tags to container tests
- [x] Add build tags to secrets tests
- [x] Add build tags to runner tests
- [x] Extract unit tests to separate files (manager_test.go, id_test.go)
- [x] Update Makefile: `make test`, `make test-integration`, `make test-all`
- [x] Create comprehensive documentation (`docs/TESTING.md`)

**Completion Date**: January 22, 2026

---

## 🟠 MEDIUM Priority (Code Quality)

### 5. Extract Executor Interface
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: MEDIUM
- **Effort**: ~3-4 hours
- **Location**: `internal/runner/`

**Why Important**: Can't unit test runner without Podman installed. Execution logic mixed with orchestration.

**Tasks Completed**:
- [x] Create `internal/runner/executor.go` (interface definition)
- [x] Define `Executor` interface with `Run()` and `RunStream()`
- [x] Implement `ShellExecutor` for production use
- [x] Inject executor into `PodmanRunner` via dependency injection
- [x] Create `MockExecutor` for tests with configurable behavior
- [x] Add comprehensive unit tests (`executor_test.go`)
- [x] Add runner unit tests using MockExecutor (`runner_unit_test.go`)
- [x] Maintain backward compatibility (existing constructors unchanged)
- [x] Create documentation (`docs/EXECUTOR_INTERFACE.md`)

**Impact**:
- ✅ Unit tests run without Podman (10ms vs 5-10s per test)
- ✅ 17 new unit tests for executor implementations
- ✅ 11 new unit tests for runner logic
- ✅ Better separation of concerns (orchestration vs execution)
- ✅ Full backward compatibility maintained

**Completion Date**: January 22, 2026

---

### 6. Refactor Long API Handlers
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: MEDIUM
- **Effort**: ~2 hours
- **Location**: `internal/api/stream.go`, `internal/api/async.go`, `internal/api/server.go`

**Why Important**: Some handlers are 100+ lines, harder to maintain.

**Tasks Completed**:
- [x] Create `internal/api/helpers.go` with extracted functions
- [x] Extract `validateClaudeConfigured()` helper (validates Claude configuration)
- [x] Extract `buildRunnerRequest()` helper (converts API request to runner request)
- [x] Extract `handleRunError()` helper (standardized error responses)
- [x] Extract `handleBadRequest()` helper (standardized bad request responses)
- [x] Refactor `runClaude` handler (server.go) - reduced from 43 to 25 lines
- [x] Refactor `runStream` handler (stream.go) - simplified request building
- [x] Refactor `runAsync` handler (async.go) - reduced from 28 to 18 lines
- [x] Add comprehensive unit tests (`internal/api/helpers_test.go`)

**Impact**:
- ✅ Reduced code duplication across handlers
- ✅ Standardized error response handling
- ✅ Improved maintainability and readability
- ✅ All handlers now focus on HTTP concerns
- ✅ Business logic extracted to reusable helpers

**Completion Date**: January 22, 2026

---

### 7. Add E2E Test Suite
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: MEDIUM
- **Effort**: ~6-8 hours
- **Location**: `tests/e2e/`

**Why Important**: No end-to-end coverage testing full API flows.

**Tasks Completed**:
- [x] Create `tests/e2e/` directory structure
- [x] Create E2E test setup/teardown infrastructure (`e2e_test.go`)
- [x] Create test helpers and utilities (`helpers.go`)
- [x] Test: Health and status endpoints (`health_test.go`)
- [x] Test: Sync execution flow (`run_test.go`)
- [x] Test: Async execution with job management (`async_test.go`)
- [x] Test: Streaming output and SSE format (`stream_test.go`)
- [x] Test: Session lifecycle (create, resume, continue, fork, destroy) (`session_test.go`)
- [x] Test: Webhook notifications
- [x] Test: Error handling and request validation
- [x] Add Makefile targets (`make test-e2e`, `make test-all`)
- [x] Update documentation (`docs/TESTING.md`)

**Implementation Details**:
- E2E tests start a real Stromboli server on random port
- Tests gracefully skip Claude-dependent tests when `ANTHROPIC_API_KEY` not set
- All tests use `//go:build e2e` build tag
- Comprehensive coverage of all API endpoints and flows
- Tests verify HTTP status codes, response formats, and API contracts

**Completion Date**: January 22, 2026

---

## 🟢 LOW Priority (Nice to Have)

### 8. Decide Container Package Fate
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: LOW
- **Effort**: ~5 minutes
- **Location**: `internal/container/` (removed)

**Current State**: Was well-implemented but completely unused.

**Options Considered**:
- [ ] **Option A**: Integrate with runner (4-6h additional work)
- [ ] **Option B**: Document as future work (30m)
- [x] **Option C**: Remove entirely (5m) ← **SELECTED**

**Decision**: Removed the package entirely. The runner uses the podman package directly for command building, and the container package added maintenance burden without providing value. Can be recreated if needed in the future.

**Completion Date**: January 22, 2026

---

### 9. Enhanced JWT Authentication
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: LOW
- **Effort**: ~4 hours
- **Location**: `internal/auth/`

**Current State**: JWT authentication with token expiration and refresh capabilities is fully implemented.

**Tasks Completed**:
- [x] Add `github.com/golang-jwt/jwt/v5` dependency
- [x] Implement JWT token generation and validation (`internal/auth/token.go`)
- [x] Add token expiration support (configurable, default: 24h access, 7d refresh)
- [x] Add refresh token support
- [x] Implement three auth endpoints:
  - `POST /auth/token` - Generate JWT tokens (requires API token)
  - `POST /auth/refresh` - Refresh access token using refresh token
  - `GET /auth/validate` - Validate token and return claims
- [x] Update auth middleware to support both legacy tokens and JWT
- [x] Add comprehensive unit tests (`internal/auth/token_test.go`)
- [x] Add API endpoint tests (`internal/api/auth_test.go`)
- [x] Create comprehensive documentation (`docs/AUTHENTICATION.md`)
- [x] Maintain full backward compatibility with existing simple token auth

**Implementation Details**:
- JWT tokens use HS256 signing algorithm
- Configurable via environment variables:
  - `STROMBOLI_JWT_SECRET` - Secret for signing tokens
  - `STROMBOLI_JWT_EXPIRY` - Access token expiry (default: 24h)
  - `STROMBOLI_JWT_REFRESH_EXPIRY` - Refresh token expiry (default: 168h)
- JWT is opt-in: enabled when JWT_SECRET is set
- Backward compatible: legacy tokens continue to work alongside JWT
- Tokens include claims: subject, issued at, expiration, is_refresh flag
- Middleware validates legacy tokens first, then JWT tokens

**Completion Date**: January 22, 2026

---

### 10. Viper Configuration Management
- **Status**: ✅ COMPLETED (Jan 22, 2026)
- **Priority**: LOW
- **Effort**: ~3 hours
- **Location**: `internal/config/`, `cmd/stromboli/main.go`

**Current State**: Centralized configuration management using Viper is fully implemented.

**Tasks Completed**:
- [x] Add `github.com/spf13/viper` dependency to go.mod
- [x] Implement `internal/config/config.go` with Config struct and Load function
- [x] Support config file loading from multiple locations (., ~/.stromboli, /etc/stromboli)
- [x] Support environment variable override (all existing env vars maintained)
- [x] Migrate all hardcoded constants from main.go
- [x] Add comprehensive unit tests with TDD approach
- [x] Create `stromboli.example.yaml` example configuration
- [x] Document all configuration options in `docs/CONFIGURATION.md`
- [x] Maintain full backward compatibility with existing environment variables

**Implementation Details**:
- Configuration priority: Environment variables > Config file > Defaults
- Config struct includes: Server, Agent, Resources, Auth, RateLimit, JWT, Jobs
- All existing env vars work unchanged (STROMBOLI_* prefix)
- YAML config file support with automatic discovery
- Comprehensive validation on startup
- Config file is optional - can use env vars or defaults only
- Backward compatible: no changes required for existing deployments

**Completion Date**: January 22, 2026

---

### 11. Token Caching
- **Status**: ❌ NOT STARTED
- **Priority**: LOW
- **Effort**: ~2 hours
- **Location**: `internal/claude/client.go`

**Current State**: File read on every request.

**Tasks**:
- [ ] Add token cache with TTL
- [ ] Invalidate on file change (fsnotify)
- [ ] Add cache hit/miss metrics
- [ ] Add tests

**Completion Date**: ___________

---

## 📊 Summary

| Priority | Items | Total Effort | Status |
|----------|-------|--------------|--------|
| 🔴 Critical | 1 | ~2h | ✅ 1/1 complete |
| 🟡 High | 3 | ~5.5h | ✅ 3/3 complete |
| 🟠 Medium | 3 | ~12h | ✅ 3/3 complete |
| 🟢 Low | 4 | ~9.5h | ✅ 3/4 complete |
| **Total** | **11** | **~29h** | **10/11 complete** |

---

## 🎯 Milestones

### v1.0-alpha (Critical Complete) ✅
- [x] Item #1: Session tests (Jan 22, 2026)

### v1.0-beta (Production Ready) ✅
- [x] Item #1: Session tests (Jan 22, 2026)
- [x] Item #2: Rate limiting (Jan 22, 2026)
- [x] Item #3: Default resource limits (Jan 22, 2026)
- [x] Item #4: Integration test build tags (Jan 22, 2026)

### v1.0-rc (Release Candidate) ✅
- [x] Items #1-7: All critical, high, and medium (Jan 22, 2026)

### v1.0 (Stable Release)
- [ ] All items complete
- [ ] Full documentation
- [ ] Performance benchmarks

---

## 📝 Notes

_Add notes here as you work through the roadmap..._

---

## 🔄 Changelog

| Date | Item | Action | Notes |
|------|------|--------|-------|
| Jan 22, 2026 | #9 Enhanced JWT Authentication | ✅ Completed | JWT with expiration & refresh, backward compatible with legacy tokens |
| Jan 22, 2026 | #8 Container Package Fate | ✅ Completed | Removed unused package entirely |
| Jan 22, 2026 | #7 E2E Test Suite | ✅ Completed | Comprehensive E2E tests for all API endpoints, graceful Claude skipping |
| Jan 22, 2026 | #6 Refactor API Handlers | ✅ Completed | Extracted helpers, reduced duplication, standardized error handling |
| Jan 22, 2026 | #5 Extract Executor Interface | ✅ Completed | Enables unit testing without Podman, 28 new tests, full backward compatibility |
| Jan 22, 2026 | #4 Integration Test Build Tags | ✅ Completed | Separated unit/integration tests with build tags, added docs/TESTING.md |
| Jan 22, 2026 | #3 Default Resource Limits | ✅ Completed | Memory: 512m, CPUs: 1, Timeout: 30m - configurable via env vars |
| Jan 22, 2026 | #2 Rate Limiting | ✅ Completed | Per-IP limiting, configurable via env vars |
| Jan 22, 2026 | #1 Session Tests | ✅ Completed | 21 tests, all passing |
| Jan 22, 2026 | Roadmap | Created | Initial planning document |

