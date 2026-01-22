# Stromboli v1.0 Roadmap 🌋

> Last Updated: January 22, 2026
> Current Score: 8.5/10

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
- **Status**: ❌ NOT STARTED
- **Priority**: HIGH
- **Effort**: ~30 minutes
- **Location**: `internal/container/container_test.go`, other integration tests

**Why Important**: Tests fail in CI/CD without Podman installed.

**Implementation**:
```go
//go:build integration
// +build integration

package container
```

**Tasks**:
- [ ] Add build tags to container tests
- [ ] Add build tags to secrets tests (if needed)
- [ ] Update Makefile: `make test-integration`
- [ ] Update CI/CD documentation

**Completion Date**: ___________

---

## 🟠 MEDIUM Priority (Code Quality)

### 5. Extract Executor Interface
- **Status**: ❌ NOT STARTED
- **Priority**: MEDIUM
- **Effort**: ~3-4 hours
- **Location**: `internal/runner/`

**Why Important**: Can't unit test runner without Podman installed. Execution logic mixed with orchestration.

**Tasks**:
- [ ] Create `internal/runner/executor.go`
- [ ] Define `Executor` interface with `Run()` and `RunStream()`
- [ ] Implement `ShellExecutor`
- [ ] Inject executor into `PodmanRunner`
- [ ] Create `MockExecutor` for tests
- [ ] Update existing tests
- [ ] Add new unit tests for runner logic

**Completion Date**: ___________

---

### 6. Refactor Long API Handlers
- **Status**: ❌ NOT STARTED
- **Priority**: MEDIUM
- **Effort**: ~2 hours
- **Location**: `internal/api/stream.go`, `internal/api/async.go`

**Why Important**: Some handlers are 100+ lines, harder to maintain.

**Tasks**:
- [ ] Create `internal/api/helpers.go`
- [ ] Extract `validateClaudeConfigured()` helper
- [ ] Extract `buildRunnerRequest()` helper
- [ ] Extract `handleRunError()` helper
- [ ] Refactor `runStream` handler
- [ ] Refactor `runAsync` handler
- [ ] Ensure tests still pass

**Completion Date**: ___________

---

### 7. Add E2E Test Suite
- **Status**: ❌ NOT STARTED
- **Priority**: MEDIUM
- **Effort**: ~6-8 hours
- **Location**: `tests/e2e/`

**Why Important**: No end-to-end coverage testing full API flows.

**Tasks**:
- [ ] Create `tests/e2e/` directory
- [ ] Add testcontainers-go dependency
- [ ] Create E2E test setup/teardown
- [ ] Test: Health endpoint
- [ ] Test: Sync execution flow
- [ ] Test: Async execution with polling
- [ ] Test: Streaming output
- [ ] Test: Session lifecycle
- [ ] Test: Job cancellation
- [ ] Add to CI/CD pipeline

**Completion Date**: ___________

---

## 🟢 LOW Priority (Nice to Have)

### 8. Decide Container Package Fate
- **Status**: ❌ NOT STARTED
- **Priority**: LOW
- **Effort**: ~30 minutes
- **Location**: `internal/container/`

**Current State**: Well-implemented but completely unused.

**Options**:
- [ ] **Option A**: Integrate with runner (4-6h additional work)
- [ ] **Option B**: Document as future work (30m)
- [ ] **Option C**: Remove entirely (5m)

**Decision**: ___________
**Completion Date**: ___________

---

### 9. Enhanced JWT Authentication
- **Status**: ❌ NOT STARTED
- **Priority**: LOW
- **Effort**: ~4 hours
- **Location**: `internal/auth/`

**Current State**: Simple token comparison works but no expiration.

**Tasks**:
- [ ] Add `github.com/golang-jwt/jwt` dependency
- [ ] Implement JWT generation endpoint
- [ ] Add token expiration
- [ ] Add refresh token support
- [ ] Update documentation

**Completion Date**: ___________

---

### 10. Viper Configuration Management
- **Status**: ❌ NOT STARTED
- **Priority**: LOW
- **Effort**: ~3 hours
- **Location**: `internal/config/`, `cmd/stromboli/main.go`

**Current State**: Hardcoded constants in main.go.

**Tasks**:
- [ ] Implement `internal/config/config.go`
- [ ] Support config file loading
- [ ] Support environment variable override
- [ ] Support command-line flags
- [ ] Migrate all constants
- [ ] Add tests
- [ ] Document configuration options

**Completion Date**: ___________

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
| 🟡 High | 3 | ~5.5h | ✅ 2/3 complete |
| 🟠 Medium | 3 | ~12h | 0/3 complete |
| 🟢 Low | 4 | ~9.5h | 0/4 complete |
| **Total** | **11** | **~29h** | **3/11 complete** |

---

## 🎯 Milestones

### v1.0-alpha (Critical Complete) ✅
- [x] Item #1: Session tests (Jan 22, 2026)

### v1.0-beta (Production Ready)
- [x] Item #1: Session tests (Jan 22, 2026)
- [x] Item #2: Rate limiting (Jan 22, 2026)
- [x] Item #3: Default resource limits (Jan 22, 2026)
- [ ] Item #4: Integration test build tags

### v1.0-rc (Release Candidate)
- [ ] Items #1-7: All critical, high, and medium

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
| Jan 22, 2026 | #3 Default Resource Limits | ✅ Completed | Memory: 512m, CPUs: 1, Timeout: 30m - configurable via env vars |
| Jan 22, 2026 | #2 Rate Limiting | ✅ Completed | Per-IP limiting, configurable via env vars |
| Jan 22, 2026 | #1 Session Tests | ✅ Completed | 21 tests, all passing |
| Jan 22, 2026 | Roadmap | Created | Initial planning document |

