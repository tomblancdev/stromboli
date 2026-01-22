# Stromboli Code Review Report

## Executive Summary

**Project**: Stromboli - Claude Code Container Orchestration API  
**Language**: Go 1.24.2  
**Framework**: Gin (HTTP), Podman (Container Runtime)  
**Review Date**: January 2026  
**Lines of Code**: ~2,900 (excluding tests)  
**Test Files**: 6 test files with 77 test cases

**Overall Assessment**: The codebase demonstrates solid fundamentals with clean architecture, good separation of concerns, and thoughtful security considerations. The project is in an early-to-mid stage of development with core functionality implemented well, though several planned features (OAuth, persistence) remain unimplemented.

---

## 1. Architecture Overview

### Structure
```
stromboli/
├── cmd/stromboli/main.go          # Entry point (thin, clean)
├── internal/
│   ├── api/                       # HTTP layer (Gin handlers)
│   ├── claude/                    # Claude CLI token & command building
│   ├── podman/                    # Podman command construction
│   ├── runner/                    # Core orchestration logic
│   └── container/                 # Container lifecycle management
├── deployments/docker/            # Dockerfiles for agent/server
└── docs/                          # Architecture & API documentation
```

### Architectural Patterns

| Pattern | Implementation | Assessment |
|---------|----------------|------------|
| **Dependency Injection** | Constructor-based in `main.go` | ✅ Clean, testable |
| **Fluent Builder** | `claude.CommandBuilder`, `podman.CommandBuilder` | ✅ Excellent readability |
| **Interface Abstraction** | `runner.Runner` interface | ✅ Enables mocking |
| **Layer Separation** | API → Runner → Podman/Claude | ✅ Clear boundaries |

### Data Flow
```
HTTP Request → api.Server → runner.PodmanRunner → podman.CommandBuilder
                    ↓                                      ↓
             claude.Client                        claude.CommandBuilder
                    ↓                                      ↓
              Token read                          exec.CommandContext
```

**Strengths**:
- Clean separation between API layer (`api/`) and business logic (`runner/`)
- Builder pattern makes complex command construction readable and testable
- Internal packages prevent accidental external dependencies

**Concerns**:
- `container/` package is largely unused - appears to be scaffolding for future Podman bindings integration
- Duplication of type definitions between `api/run.go` and `runner/runner.go` (ClaudeOptions, PodmanOptions)

---

## 2. Code Quality

### Naming Conventions

| Element | Convention | Compliance |
|---------|------------|------------|
| Packages | lowercase, short | ✅ `api`, `claude`, `runner` |
| Exported Types | PascalCase | ✅ `CommandBuilder`, `PodmanRunner` |
| Unexported | camelCase | ✅ `generateSessionID` |
| Files | snake_case | ✅ `server_test.go` |
| Interfaces | `-er` suffix | ✅ `Runner` |

### Error Handling

**Good Practices Observed**:
```go
// runner.go:138 - Proper error wrapping with context
return nil, fmt.Errorf("failed to get token: %w", err)

// runner.go:206-208 - Error message includes output for debugging
return nil, fmt.Errorf("execution failed: %w, output: %s", err, string(output))
```

**Custom Error Types**:
```go
// claude.go:9 - Sentinel error for expected condition
var ErrTokenNotFound = errors.New("token not found")

// container.go:9-10 - Domain errors
var ErrContainerNotFound = errors.New("container not found")
var ErrCommandFailed = errors.New("podman command failed")
```

**Areas for Improvement**:
- `runner.go:111-113` defines `ErrSessionNotFound` as a function returning error, inconsistent with other sentinel errors
- Error handling in `api/server.go:267-279` uses string matching for error classification - fragile approach

### Code Organization

**Function Size**: Generally good (10-50 lines), with one exception:
- `runner.applyClaudeOptions()` at 125 lines - acceptable given it's a straightforward mapping function

**Cyclomatic Complexity**: Low throughout - simple, linear logic flows

**Comments Quality**:
```go
// Good - Explains WHY, not WHAT
// :U flag tells Podman to recursively chown the volume to match the container user
// This is essential for rootless containers writing to mounted volumes
func (b *CommandBuilder) WithVolumeChown(hostPath, containerPath string) *CommandBuilder
```

---

## 3. Security

### Strengths

| Security Control | Location | Assessment |
|------------------|----------|------------|
| **Path Traversal Prevention** | `runner.go:224-226` | ✅ Blocks `../` and `/` in session IDs |
| **Non-root Container User** | `Dockerfile.agent:26` | ✅ Runs as `claude` user |
| **Token File Permissions** | `Makefile:115` | ✅ `chmod 600` on secrets file |
| **Session Directory Permissions** | `runner.go:149` | ✅ `0700` permissions |
| **Volume Chown Flag** | `runner.go:186` | ✅ `:U` ensures container user ownership |

### Security Concerns

**HIGH: Token Exposure in Podman Command**
```go
// runner.go:177 - Token passed as environment variable
podmanBuilder := podman.NewCommand().
    WithEnv("CLAUDE_CODE_OAUTH_TOKEN", token)
```
- Environment variables may be visible in `ps` output, `/proc/<pid>/environ`, or Podman logs
- **Recommendation**: Use Podman secrets (`--secret`) or mount token as file

**MEDIUM: No Input Validation on Workspace Path**
```go
// runner.go:189-192 - Workspace directly mounted without validation
if req.Workspace != "" {
    podmanBuilder.WithVolume(req.Workspace, "/workspace")
}
```
- Arbitrary host paths can be mounted
- **Recommendation**: Validate workspace paths against allowed directories

**MEDIUM: Missing Authentication**
- No OAuth/JWT middleware implemented despite being documented in architecture
- API is completely open

**LOW: Session ID Validation Incomplete**
```go
// runner.go:224 - Only checks for "/" and ".."
if strings.Contains(sessionID, "/") || strings.Contains(sessionID, "..") {
```
- Could be bypassed with URL encoding or other tricks
- **Recommendation**: Use allowlist (UUID format validation) instead of blocklist

**LOW: No Rate Limiting**
- Container spawning has no rate limits
- **Recommendation**: Add rate limiting middleware

---

## 4. Testing

### Coverage Analysis

| Package | Test File | Test Cases | Coverage Focus |
|---------|-----------|------------|----------------|
| `api` | `server_test.go` | 11 | HTTP handlers, request/response validation |
| `claude` | `claude_test.go`, `builder_test.go` | 39 | Token handling, CLI command generation |
| `podman` | `builder_test.go` | 10 | Command building |
| `runner` | `runner_test.go` | 12 | Session management, path traversal |
| `container` | `container_test.go` | 5 | Container lifecycle (integration) |
| **Total** | | **77** | |

### Test Quality Assessment

**Strengths**:
- Comprehensive test coverage for builder patterns (31 tests for `claude.CommandBuilder`)
- Good use of table-driven tests
- Proper test isolation with `t.TempDir()`
- Mock implementations for interface testing

**Good Test Example**:
```go
// runner_test.go:202-212 - Validates UUID format with 100 iterations
func TestGenerateSessionID_IsUnique(t *testing.T) {
    ids := make(map[string]bool)
    for i := 0; i < 100; i++ {
        id := generateSessionID()
        assert.Regexp(t, `^[0-9a-f]{8}-...`, id)
        assert.False(t, ids[id], "ID should be unique")
        ids[id] = true
    }
}
```

**Areas for Improvement**:

1. **Integration Tests Require Real Podman**
   - `container_test.go` tests actually execute Podman commands
   - These will fail in CI without Podman available
   - Missing build tags to skip in unit test runs

2. **Missing E2E Tests**
   - `tests/e2e/` directory referenced in docs doesn't exist
   - No full API flow tests

3. **Runner Tests Skip Execution**
   ```go
   // runner_test.go:43-51 - Test expects failure, doesn't verify command construction
   _, err = runner.Run(context.Background(), Request{Prompt: "hello"})
   assert.Error(t, err)  // Always fails due to no Podman
   ```

4. **Duplicate Mock Implementations**
   - `MockRunner` defined in both `runner_test.go` and `api/server_test.go`

---

## 5. Performance

### Potential Bottlenecks

**1. Synchronous Container Execution**
```go
// runner.go:204-205 - Blocking call
cmd := exec.CommandContext(ctx, fullCmd[0], fullCmd[1:]...)
output, err := cmd.CombinedOutput()
```
- HTTP request blocks until container completes
- Long-running prompts will timeout HTTP connections
- **Recommendation**: Implement async execution with polling/WebSocket for status

**2. Token Read on Every Request**
```go
// runner.go:135-136 - New client and file read per execution
client := claude.NewClient(r.secretsFile)
token, err := client.GetToken()
```
- File I/O on every request
- **Recommendation**: Cache token with TTL, invalidate on file change

**3. Map Iteration Non-determinism**
```go
// podman/builder.go:109-111 - Map iteration order is random
for key, value := range b.env {
    args = append(args, "-e", key+"="+value)
}
```
- Makes testing harder (argument order varies)
- **Recommendation**: Sort keys for deterministic output

**4. Session Directory Scanning**
```go
// runner.go:245 - Full directory read for listing
entries, err := os.ReadDir(r.sessionsDir)
```
- O(n) for n sessions - acceptable for small scale
- Will need indexing/database for production scale

---

## 6. Best Practices

### Go Idioms Used Well

| Practice | Example | Assessment |
|----------|---------|------------|
| **Context propagation** | `Run(ctx context.Context, req Request)` | ✅ |
| **Functional options** | Builder pattern instead | ✅ Alternative approach, works well |
| **Interface in consumer** | `Runner` defined in `runner/` package | ✅ |
| **Error wrapping** | `fmt.Errorf("...: %w", err)` | ✅ |
| **Structured logging** | `slog` stdlib | ✅ |

### Go Idioms Missing/Improvable

**1. Context Not Passed Through Full Stack**
```go
// container.go:37 - No context parameter
func (m *Manager) Create(spec Spec) (string, error) {
    cmd := exec.Command("podman", args...)  // Should use exec.CommandContext
}
```

**2. Mutex Could Cause Contention**
```go
// runner.go:120-121 - Mutex defined but never used!
type PodmanRunner struct {
    mu sync.Mutex  // Declared but unused
}
```

**3. Missing `io.Reader`/`io.Writer` Interfaces**
- Output handling uses `[]byte` everywhere instead of streaming interfaces
- Limits ability to stream large outputs

**4. No Graceful Shutdown**
```go
// main.go:57 - No signal handling
if err := server.Run(defaultAddr); err != nil {
```
- **Recommendation**: Add `signal.NotifyContext` for graceful shutdown

### Configuration

**Current State**: Hardcoded constants in `main.go`
```go
const (
    defaultImage       = "stromboli-agent:latest"
    defaultSecretsFile = ".claude-secrets"
    defaultSessionsDir = ".stromboli/sessions"
    defaultAddr        = ":8080"
)
```

**Missing**:
- Environment variable override support
- Configuration file loading (Viper is in tech stack but not used)
- `internal/config/` package is empty

---

## 7. Documentation

### Code Documentation

| Type | Quality | Notes |
|------|---------|-------|
| **Package Comments** | ❌ Missing | No package-level documentation |
| **Exported Functions** | ✅ Good | Most have godoc comments |
| **Swagger Annotations** | ✅ Excellent | Full OpenAPI spec generation |
| **README/CLAUDE.md** | ✅ Comprehensive | Clear guidelines |
| **Architecture Docs** | ✅ Excellent | Detailed system design |

### Swagger Documentation
```go
// server.go:101-112 - Well-annotated endpoint
// @Summary Run Claude
// @Description Executes Claude Code in an isolated Podman container
// @Tags execution
// @Accept json
// @Produce json
// @Success 200 {object} RunResponse
```

### Missing Documentation
- No package-level godoc comments
- No API changelog/versioning documentation
- No deployment/operations runbook

---

## 8. Recommendations

### Priority 1 - Critical (Security)

1. **Implement Token Security**
   - Use Podman secrets or file-based token injection
   - Remove token from environment variables visible in process listing

2. **Add Workspace Path Validation**
   - Implement allowlist of permitted workspace directories
   - Prevent mounting sensitive host paths

3. **Implement Authentication Middleware**
   - Add JWT validation middleware to API routes
   - Integrate with Authentik as designed

### Priority 2 - High (Reliability)

4. **Add Graceful Shutdown**
   ```go
   ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
   defer stop()
   ```

5. **Implement Async Execution**
   - Return job ID immediately
   - Add status polling endpoint
   - Consider WebSocket for streaming output

6. **Use Context Throughout**
   - Pass context to `container.Manager` methods
   - Respect cancellation in long-running operations

### Priority 3 - Medium (Maintainability)

7. **Consolidate Type Definitions**
   - Remove duplication between `api.ClaudeOptions` and `runner.ClaudeOptions`
   - Use single source of truth or proper type embedding

8. **Extract Mock to Shared Package**
   - Move `MockRunner` to `internal/runner/mock.go`
   - Avoid duplication in test files

9. **Add Configuration Management**
   - Implement Viper-based configuration
   - Support environment variables and config files

10. **Implement Error Types**
    - Define custom error types with `errors.As()` support
    - Replace string-based error matching in API layer

### Priority 4 - Low (Polish)

11. **Add Rate Limiting**
    - Implement request rate limiting middleware
    - Prevent container spawn abuse

12. **Deterministic Command Building**
    - Sort environment variables in `podman.CommandBuilder`
    - Makes debugging and testing easier

13. **Complete Container Package**
    - Either integrate `container.Manager` or remove it
    - Currently unused scaffolding

14. **Add E2E Test Suite**
    - Create `tests/e2e/` directory with integration tests
    - Use testcontainers-go for portable testing

---

## Summary Metrics

| Metric | Value | Assessment |
|--------|-------|------------|
| **Code Quality** | 8/10 | Clean, readable, well-structured |
| **Test Coverage** | 7/10 | Good unit tests, missing integration/E2E |
| **Security** | 6/10 | Good foundations, missing auth, token exposure |
| **Documentation** | 8/10 | Excellent architecture docs, missing package docs |
| **Performance** | 7/10 | Acceptable for MVP, needs async for production |
| **Go Best Practices** | 8/10 | Follows most idioms, minor improvements needed |

**Overall**: This is a well-architected project in early-mid development stage. The core functionality is implemented cleanly with good testing of the builder patterns. The main gaps are in security (missing auth, token exposure) and production readiness (async execution, configuration management). The codebase follows the project's own guidelines (CLAUDE.md) reasonably well and demonstrates thoughtful design decisions.
