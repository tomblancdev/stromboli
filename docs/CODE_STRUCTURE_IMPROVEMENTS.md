# Stromboli Code Readability & Structure Improvements

## Overview

This document provides specific, actionable recommendations to enhance code readability and structure in the Stromboli project. Each recommendation includes the specific location, rationale, and code examples.

---

## 1. Consolidate Duplicate Type Definitions

### What to Change

**Files**: `internal/api/run.go` and `internal/runner/runner.go`

Currently, `ClaudeOptions` and related types are defined twice with nearly identical fields - once in the API layer and once in the runner layer.

### Why It Improves Readability

- Single source of truth eliminates confusion about which type to use
- Reduces maintenance burden when adding new options
- Makes the relationship between API and runner explicit

### Code Example

**Before** (current state):
```go
// internal/api/run.go
type ClaudeOptions struct {
    SessionID string `json:"session_id,omitempty"`
    Resume    bool   `json:"resume,omitempty"`
    // ... 30+ fields
}

// internal/runner/runner.go  
type ClaudeOptions struct {
    SessionID string
    Resume    bool
    // ... same 30+ fields duplicated
}
```

**After** (recommended):

Create a new shared types package:

```go
// internal/types/claude.go
package types

// ClaudeOptions contains all Claude CLI headless mode options.
// Used by both API layer (with JSON tags) and runner layer.
type ClaudeOptions struct {
    // Session Management
    SessionID string `json:"session_id,omitempty"`
    Resume    bool   `json:"resume,omitempty"`
    Continue  bool   `json:"continue,omitempty"`
    // ... all fields with JSON tags
}

// PodmanOptions contains Podman container configuration.
type PodmanOptions struct {
    Volumes []string `json:"volumes,omitempty"`
}
```

Then update both packages to use it:

```go
// internal/api/run.go
import "stromboli/internal/types"

type RunRequest struct {
    Prompt    string              `json:"prompt" binding:"required"`
    Workspace string              `json:"workspace,omitempty"`
    Claude    types.ClaudeOptions `json:"claude,omitempty"`
    Podman    types.PodmanOptions `json:"podman,omitempty"`
}

// internal/runner/runner.go
import "stromboli/internal/types"

type Request struct {
    Prompt    string
    Workspace string
    Claude    types.ClaudeOptions
    Podman    types.PodmanOptions
}
```

---

## 2. Extract Session Management into Dedicated Package

### What to Change

**File**: `internal/runner/runner.go` lines 120-260

The `PodmanRunner` struct handles both container orchestration AND session management (directory creation, listing, destruction). These are separate concerns.

### Why It Improves Readability

- Single responsibility principle - each package does one thing
- Session logic becomes independently testable
- Clearer mental model of what each package does

### Code Example

**Before** (in runner.go):
```go
type PodmanRunner struct {
    image       string
    secretsFile string
    sessionsDir string  // Session concern mixed with runner
    mu          sync.Mutex
}

func (r *PodmanRunner) DestroySession(sessionID string) error {
    // 20 lines of session directory management
}

func (r *PodmanRunner) ListSessions() ([]string, error) {
    // 15 lines of directory reading
}
```

**After** (new session package):

```go
// internal/session/session.go
package session

import (
    "errors"
    "os"
    "path/filepath"
    "strings"
)

var (
    ErrInvalidSessionID = errors.New("invalid session ID")
    ErrSessionNotFound  = errors.New("session not found")
)

// Manager handles session storage and lifecycle.
type Manager struct {
    baseDir string
}

// NewManager creates a session manager with the given base directory.
func NewManager(baseDir string) *Manager {
    return &Manager{baseDir: baseDir}
}

// Create creates a new session directory with secure permissions.
func (m *Manager) Create(sessionID string) (string, error) {
    if err := m.validateID(sessionID); err != nil {
        return "", err
    }
    
    path := filepath.Join(m.baseDir, sessionID)
    if err := os.MkdirAll(path, 0700); err != nil {
        return "", fmt.Errorf("create session directory: %w", err)
    }
    return path, nil
}

// Destroy removes a session directory.
func (m *Manager) Destroy(sessionID string) error {
    if err := m.validateID(sessionID); err != nil {
        return err
    }
    
    path := filepath.Join(m.baseDir, sessionID)
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return ErrSessionNotFound
    }
    return os.RemoveAll(path)
}

// List returns all session IDs.
func (m *Manager) List() ([]string, error) {
    entries, err := os.ReadDir(m.baseDir)
    if err != nil {
        if os.IsNotExist(err) {
            return []string{}, nil
        }
        return nil, fmt.Errorf("read sessions directory: %w", err)
    }
    
    sessions := make([]string, 0, len(entries))
    for _, entry := range entries {
        if entry.IsDir() {
            sessions = append(sessions, entry.Name())
        }
    }
    return sessions, nil
}

// Path returns the filesystem path for a session.
func (m *Manager) Path(sessionID string) string {
    return filepath.Join(m.baseDir, sessionID)
}

// validateID checks for path traversal attempts.
func (m *Manager) validateID(sessionID string) error {
    if strings.Contains(sessionID, "/") || strings.Contains(sessionID, "..") {
        return ErrInvalidSessionID
    }
    return nil
}
```

Updated runner:

```go
// internal/runner/runner.go
type PodmanRunner struct {
    image       string
    secretsFile string
    sessions    *session.Manager  // Composed, not embedded
}

func NewPodmanRunner(image, secretsFile, sessionsDir string) *PodmanRunner {
    return &PodmanRunner{
        image:       image,
        secretsFile: secretsFile,
        sessions:    session.NewManager(sessionsDir),
    }
}

// Session operations now delegate cleanly
func (r *PodmanRunner) DestroySession(sessionID string) error {
    return r.sessions.Destroy(sessionID)
}

func (r *PodmanRunner) ListSessions() ([]string, error) {
    return r.sessions.List()
}
```

---

## 3. Break Up `applyClaudeOptions` Function

### What to Change

**File**: `internal/runner/runner.go` lines 262-390

This 125-line function applies all Claude options to the command builder. While straightforward, its length makes it hard to scan.

### Why It Improves Readability

- Grouped options are easier to find and modify
- Reduces cognitive load when reading
- Each group can be independently tested

### Code Example

**Before**:
```go
func applyClaudeOptions(builder *claude.CommandBuilder, opts ClaudeOptions) {
    // 125 lines of if statements covering:
    // - Session management (10 options)
    // - Model configuration (2 options)
    // - System prompts (2 options)
    // - Tools (3 options)
    // - Permissions (3 options)
    // - I/O format (5 options)
    // - MCP config (2 options)
    // - Resources (4 options)
    // - Settings (2 options)
    // - Debug (3 options)
}
```

**After**:
```go
func applyClaudeOptions(builder *claude.CommandBuilder, opts types.ClaudeOptions) {
    applySessionOptions(builder, opts)
    applyModelOptions(builder, opts)
    applyPromptOptions(builder, opts)
    applyToolOptions(builder, opts)
    applyPermissionOptions(builder, opts)
    applyIOOptions(builder, opts)
    applyMCPOptions(builder, opts)
    applyResourceOptions(builder, opts)
    applySettingsOptions(builder, opts)
    applyDebugOptions(builder, opts)
}

func applySessionOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
    if opts.SessionID != "" {
        if opts.Resume {
            b.Resume(opts.SessionID)
        } else {
            b.SessionID(opts.SessionID)
        }
    }
    if opts.Continue {
        b.Continue()
    }
    if opts.ForkSession {
        b.ForkSession()
    }
    if opts.NoPersistence {
        b.NoPersistence()
    }
}

func applyModelOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
    if opts.Model != "" {
        b.Model(opts.Model)
    }
    if opts.FallbackModel != "" {
        b.FallbackModel(opts.FallbackModel)
    }
}

func applyToolOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
    for _, tool := range opts.Tools {
        b.Tool(tool)
    }
    for _, tool := range opts.AllowedTools {
        b.AllowedTool(tool)
    }
    for _, tool := range opts.DisallowedTools {
        b.DisallowedTool(tool)
    }
}

// ... similar small functions for each group
```

---

## 4. Improve Error Handling Consistency

### What to Change

**Files**: `internal/runner/runner.go`, `internal/api/server.go`

Currently errors are handled inconsistently:
- Some use sentinel errors (`ErrTokenNotFound`)
- Some use function-returned errors (`ErrSessionNotFound()`)
- API layer uses string matching

### Why It Improves Readability

- Consistent error patterns are easier to follow
- `errors.Is()` checks are cleaner than string matching
- Custom error types enable richer error information

### Code Example

**Before** (runner.go):
```go
// Inconsistent: function vs variable
var ErrTokenNotFound = errors.New("token not found")

func ErrSessionNotFound(id string) error {
    return fmt.Errorf("session not found: %s", id)
}
```

**Before** (server.go):
```go
// Fragile string matching
if strings.Contains(err.Error(), "not found") {
    c.JSON(http.StatusNotFound, ...)
}
```

**After** (new errors package):
```go
// internal/errors/errors.go
package errors

import "fmt"

// Sentinel errors for common conditions
var (
    ErrTokenNotFound   = &Error{Code: "TOKEN_NOT_FOUND", Message: "token not found"}
    ErrSessionNotFound = &Error{Code: "SESSION_NOT_FOUND", Message: "session not found"}
    ErrInvalidSession  = &Error{Code: "INVALID_SESSION", Message: "invalid session ID"}
    ErrNotConfigured   = &Error{Code: "NOT_CONFIGURED", Message: "claude not configured"}
)

// Error is a domain error with a code for programmatic handling.
type Error struct {
    Code    string
    Message string
    Cause   error
}

func (e *Error) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Cause)
    }
    return e.Message
}

func (e *Error) Unwrap() error {
    return e.Cause
}

// Is implements errors.Is matching by code.
func (e *Error) Is(target error) bool {
    if t, ok := target.(*Error); ok {
        return e.Code == t.Code
    }
    return false
}

// WithCause returns a copy of the error with a cause attached.
func (e *Error) WithCause(cause error) *Error {
    return &Error{
        Code:    e.Code,
        Message: e.Message,
        Cause:   cause,
    }
}

// WithMessage returns a copy with additional context.
func (e *Error) WithMessage(format string, args ...any) *Error {
    return &Error{
        Code:    e.Code,
        Message: fmt.Sprintf(format, args...),
        Cause:   e.Cause,
    }
}
```

**After** (server.go):
```go
import apperrors "stromboli/internal/errors"

func (s *Server) handleRunRequest(c *gin.Context) {
    result, err := s.runner.Run(c.Request.Context(), runnerReq)
    if err != nil {
        switch {
        case errors.Is(err, apperrors.ErrNotConfigured):
            c.JSON(http.StatusServiceUnavailable, RunResponse{
                Status: "error",
                Error:  err.Error(),
            })
        case errors.Is(err, apperrors.ErrSessionNotFound):
            c.JSON(http.StatusNotFound, RunResponse{
                Status: "error", 
                Error:  err.Error(),
            })
        default:
            c.JSON(http.StatusInternalServerError, RunResponse{
                Status: "error",
                Error:  err.Error(),
            })
        }
        return
    }
    // success handling
}
```

---

## 5. Simplify API Handler with Request Mapper

### What to Change

**File**: `internal/api/server.go` lines 120-180

The `handleRunRequest` function mixes HTTP concerns with business logic mapping.

### Why It Improves Readability

- HTTP handler focuses only on HTTP concerns
- Mapping logic is testable in isolation
- Cleaner separation of concerns

### Code Example

**Before**:
```go
func (s *Server) handleRunRequest(c *gin.Context) {
    var req RunRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, RunResponse{...})
        return
    }

    // 50+ lines of manual field mapping
    runnerReq := runner.Request{
        Prompt:    req.Prompt,
        Workspace: req.Workspace,
        Claude: runner.ClaudeOptions{
            SessionID: req.Claude.SessionID,
            Resume:    req.Claude.Resume,
            // ... many more fields
        },
    }

    result, err := s.runner.Run(c.Request.Context(), runnerReq)
    // error handling...
}
```

**After** (with shared types from recommendation #1):
```go
// internal/api/mapper.go
package api

import (
    "stromboli/internal/runner"
    "stromboli/internal/types"
)

// toRunnerRequest converts an API request to a runner request.
// With shared types, this becomes trivial.
func toRunnerRequest(req RunRequest) runner.Request {
    return runner.Request{
        Prompt:    req.Prompt,
        Workspace: req.Workspace,
        Claude:    req.Claude,  // Same type, direct assignment
        Podman:    req.Podman,  // Same type, direct assignment
    }
}
```

```go
// internal/api/server.go
func (s *Server) handleRunRequest(c *gin.Context) {
    var req RunRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        s.respondError(c, http.StatusBadRequest, "invalid request", err)
        return
    }

    result, err := s.runner.Run(c.Request.Context(), toRunnerRequest(req))
    if err != nil {
        s.handleRunError(c, err)
        return
    }

    c.JSON(http.StatusOK, toRunResponse(result))
}

// handleRunError maps runner errors to HTTP responses.
func (s *Server) handleRunError(c *gin.Context, err error) {
    switch {
    case errors.Is(err, apperrors.ErrNotConfigured):
        s.respondError(c, http.StatusServiceUnavailable, "claude not configured", err)
    case errors.Is(err, apperrors.ErrSessionNotFound):
        s.respondError(c, http.StatusNotFound, "session not found", err)
    default:
        s.respondError(c, http.StatusInternalServerError, "execution failed", err)
    }
}

// respondError sends a standardized error response.
func (s *Server) respondError(c *gin.Context, status int, message string, err error) {
    slog.Error(message, "error", err, "status", status)
    c.JSON(status, RunResponse{
        Status: "error",
        Error:  message,
    })
}

func toRunResponse(result *runner.Result) RunResponse {
    return RunResponse{
        ID:        result.ID,
        Status:    result.Status,
        Output:    result.Output,
        SessionID: result.SessionID,
    }
}
```

---

## 6. Add Package Documentation

### What to Change

**Files**: All package directories need a `doc.go` file

Currently no packages have package-level documentation.

### Why It Improves Readability

- `go doc` output becomes useful
- New developers understand package purpose immediately
- Forces clear thinking about package boundaries

### Code Example

```go
// internal/runner/doc.go
/*
Package runner provides the core orchestration logic for executing
Claude Code in Podman containers.

The primary type is PodmanRunner, which implements the Runner interface
and handles:
  - Container spawning with proper isolation
  - Session management (create, list, destroy)
  - Claude CLI command construction
  - Execution and output capture

Basic usage:

    runner := runner.NewPodmanRunner(
        "stromboli-agent:latest",
        ".claude-secrets",
        ".stromboli/sessions",
    )

    result, err := runner.Run(ctx, runner.Request{
        Prompt: "Analyze this code",
        Workspace: "/path/to/project",
    })

The runner is designed to be stateless - all session state is stored
on the filesystem under the configured sessions directory.
*/
package runner
```

```go
// internal/claude/doc.go
/*
Package claude provides Claude Code CLI integration.

It contains two main components:

Client handles OAuth token management:

    client := claude.NewClient(".claude-secrets")
    if client.IsConfigured() {
        token, _ := client.GetToken()
    }

CommandBuilder constructs Claude CLI arguments using a fluent API:

    args := claude.NewCommand().
        WithPrompt("Hello").
        Model("sonnet").
        OutputFormat("json").
        Build()
    // Returns: ["-p", "Hello", "--model", "sonnet", "--output-format", "json"]

The builder pattern ensures type-safe command construction and makes
the resulting commands easy to read and debug.
*/
package claude
```

```go
// internal/api/doc.go
/*
Package api provides the HTTP REST interface for Stromboli.

It uses the Gin framework and exposes the following endpoints:

    GET  /health           - Health check
    GET  /claude/status    - Claude configuration status
    POST /run              - Execute Claude in a container
    GET  /sessions         - List active sessions
    DELETE /sessions/:id   - Destroy a session

The Server type wraps a Runner implementation and a Claude Client,
translating HTTP requests into runner operations and formatting
responses.

All endpoints return JSON. Error responses include a status field
set to "error" and an error message.
*/
package api
```

---

## 7. Extract Command Execution Logic

### What to Change

**File**: `internal/runner/runner.go` lines 195-210

Command execution is buried in the Run method. Extracting it improves testability and reuse.

### Why It Improves Readability

- `Run()` method becomes a high-level orchestration flow
- Execution logic is isolated and testable
- Easier to add execution variants (async, streaming)

### Code Example

**Before**:
```go
func (r *PodmanRunner) Run(ctx context.Context, req Request) (*Result, error) {
    // ... 60 lines of setup ...
    
    // Execution buried in the middle
    fullCmd := podmanBuilder.Build()
    cmd := exec.CommandContext(ctx, fullCmd[0], fullCmd[1:]...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("execution failed: %w, output: %s", err, string(output))
    }
    
    // ... result construction ...
}
```

**After**:
```go
// internal/runner/executor.go
package runner

import (
    "context"
    "fmt"
    "os/exec"
)

// CommandResult holds the output of a command execution.
type CommandResult struct {
    Output   []byte
    ExitCode int
}

// Executor runs shell commands.
type Executor interface {
    Run(ctx context.Context, args []string) (*CommandResult, error)
}

// ShellExecutor executes commands via os/exec.
type ShellExecutor struct{}

func (e *ShellExecutor) Run(ctx context.Context, args []string) (*CommandResult, error) {
    if len(args) == 0 {
        return nil, fmt.Errorf("empty command")
    }

    cmd := exec.CommandContext(ctx, args[0], args[1:]...)
    output, err := cmd.CombinedOutput()

    result := &CommandResult{
        Output:   output,
        ExitCode: 0,
    }

    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            result.ExitCode = exitErr.ExitCode()
        }
        return result, fmt.Errorf("command failed: %w", err)
    }

    return result, nil
}
```

```go
// internal/runner/runner.go
type PodmanRunner struct {
    image       string
    secretsFile string
    sessions    *session.Manager
    executor    Executor  // Injected, mockable
}

func NewPodmanRunner(image, secretsFile, sessionsDir string) *PodmanRunner {
    return &PodmanRunner{
        image:       image,
        secretsFile: secretsFile,
        sessions:    session.NewManager(sessionsDir),
        executor:    &ShellExecutor{},
    }
}

// WithExecutor allows injecting a custom executor (for testing).
func (r *PodmanRunner) WithExecutor(e Executor) *PodmanRunner {
    r.executor = e
    return r
}

func (r *PodmanRunner) Run(ctx context.Context, req Request) (*Result, error) {
    // Setup phase
    token, err := r.getToken()
    if err != nil {
        return nil, err
    }

    sessionID, sessionPath, err := r.prepareSession(req)
    if err != nil {
        return nil, err
    }

    // Build command
    cmd := r.buildCommand(req, token, sessionPath)

    // Execute
    cmdResult, err := r.executor.Run(ctx, cmd)
    if err != nil {
        return nil, fmt.Errorf("execution failed: %w, output: %s", err, cmdResult.Output)
    }

    // Return result
    return &Result{
        ID:        generateRunID(),
        Status:    "completed",
        Output:    string(cmdResult.Output),
        SessionID: sessionID,
    }, nil
}
```

---

## 8. Remove Unused Code

### What to Change

**File**: `internal/runner/runner.go` line 121

The `mu sync.Mutex` field is declared but never used.

**File**: `internal/container/container.go`

This entire package appears to be scaffolding that's not integrated with the runner.

### Why It Improves Readability

- Dead code creates confusion ("Is this used somewhere I can't see?")
- Reduces cognitive load
- Keeps codebase focused

### Code Example

**Before**:
```go
type PodmanRunner struct {
    image       string
    secretsFile string
    sessionsDir string
    mu          sync.Mutex  // Never locked or unlocked
}
```

**After**:
```go
type PodmanRunner struct {
    image       string
    secretsFile string
    sessionsDir string
}
```

For `container/` package: Either integrate it with the runner or remove it entirely. If keeping for future use, add a clear TODO comment:

```go
// Package container provides Podman container lifecycle management.
//
// TODO: This package is not yet integrated. It's intended to replace
// the exec.Command approach in runner with proper Podman API bindings.
// See: https://github.com/containers/podman/blob/main/pkg/bindings
package container
```

---

## 9. Consolidate Mock Implementations

### What to Change

**Files**: `internal/runner/runner_test.go` and `internal/api/server_test.go`

Both files define their own `MockRunner` implementation.

### Why It Improves Readability

- Single mock to understand and maintain
- Consistent behavior across all tests
- Easier to add mock features

### Code Example

**Before** (duplicated in two files):
```go
// runner_test.go
type MockRunner struct {
    RunFunc            func(ctx context.Context, req Request) (*Result, error)
    DestroySessionFunc func(sessionID string) error
    ListSessionsFunc   func() ([]string, error)
}

// server_test.go  
type MockRunner struct {
    RunFunc            func(ctx context.Context, req runner.Request) (*runner.Result, error)
    DestroySessionFunc func(sessionID string) error
    ListSessionsFunc   func() ([]string, error)
}
```

**After**:
```go
// internal/runner/mock.go
package runner

import "context"

// MockRunner is a test double for the Runner interface.
type MockRunner struct {
    RunFunc            func(ctx context.Context, req Request) (*Result, error)
    DestroySessionFunc func(sessionID string) error
    ListSessionsFunc   func() ([]string, error)
}

func (m *MockRunner) Run(ctx context.Context, req Request) (*Result, error) {
    if m.RunFunc != nil {
        return m.RunFunc(ctx, req)
    }
    return &Result{Status: "completed"}, nil
}

func (m *MockRunner) DestroySession(sessionID string) error {
    if m.DestroySessionFunc != nil {
        return m.DestroySessionFunc(sessionID)
    }
    return nil
}

func (m *MockRunner) ListSessions() ([]string, error) {
    if m.ListSessionsFunc != nil {
        return m.ListSessionsFunc()
    }
    return []string{}, nil
}

// NewMockRunner creates a MockRunner with sensible defaults.
func NewMockRunner() *MockRunner {
    return &MockRunner{}
}

// WithRun configures the Run behavior.
func (m *MockRunner) WithRun(fn func(ctx context.Context, req Request) (*Result, error)) *MockRunner {
    m.RunFunc = fn
    return m
}

// WithRunResult configures Run to return a fixed result.
func (m *MockRunner) WithRunResult(result *Result) *MockRunner {
    m.RunFunc = func(ctx context.Context, req Request) (*Result, error) {
        return result, nil
    }
    return m
}

// WithRunError configures Run to return an error.
func (m *MockRunner) WithRunError(err error) *MockRunner {
    m.RunFunc = func(ctx context.Context, req Request) (*Result, error) {
        return nil, err
    }
    return m
}
```

Usage in tests:
```go
// api/server_test.go
mock := runner.NewMockRunner().WithRunResult(&runner.Result{
    ID:        "test-123",
    Status:    "completed",
    Output:    "test output",
    SessionID: "session-456",
})
server := NewServer(mock, claudeClient)
```

---

## Summary: Recommended File Structure After Changes

```
internal/
├── api/
│   ├── doc.go           # Package documentation
│   ├── server.go        # HTTP handlers (simplified)
│   ├── server_test.go   # Tests
│   ├── mapper.go        # Request/response mapping (NEW)
│   └── run.go           # DTOs (uses shared types)
├── claude/
│   ├── doc.go           # Package documentation
│   ├── client.go        # Token operations (renamed from claude.go)
│   ├── client_test.go
│   ├── builder.go       # Command builder
│   └── builder_test.go
├── errors/              # NEW
│   └── errors.go        # Domain error types
├── podman/
│   ├── doc.go           # Package documentation
│   ├── builder.go
│   └── builder_test.go
├── runner/
│   ├── doc.go           # Package documentation
│   ├── runner.go        # Core orchestration (simplified)
│   ├── runner_test.go
│   ├── executor.go      # Command execution (NEW)
│   ├── options.go       # applyXxxOptions functions (NEW)
│   └── mock.go          # Test mock (consolidated)
├── session/             # NEW
│   ├── session.go       # Session management
│   └── session_test.go
├── types/               # NEW
│   └── claude.go        # Shared ClaudeOptions, PodmanOptions
└── config/              # Empty (existing placeholder)
```

---

## Implementation Order

1. **Create `internal/types/`** - Enables all other refactoring
2. **Create `internal/errors/`** - Improves error handling throughout
3. **Create `internal/session/`** - Extracts session logic from runner
4. **Add `doc.go` files** - Quick win for documentation
5. **Consolidate mock** - Reduces test maintenance
6. **Break up `applyClaudeOptions`** - Improves runner readability
7. **Extract executor** - Improves testability
8. **Remove unused code** - Cleanup

Each change can be done incrementally with tests passing at each step.
