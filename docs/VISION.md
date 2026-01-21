# Stromboli Vision & Architecture

## Core Principle

**Stromboli = Simple API layer for isolated AI execution**

Keep it dumb. Keep it secure. Let others handle orchestration.

## What Stromboli IS

- Secure container execution for Claude Code
- Token/credential management
- Workspace isolation
- Simple REST API
- OAuth-protected endpoints

## What Stromboli IS NOT

- Workflow engine (use n8n)
- State machine (use n8n)
- Notification system (use n8n webhooks)
- UI framework (separate apps)

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        CONSUMERS                                     │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐               │
│  │   n8n   │  │ Web UI  │  │  Slide  │  │  CLI    │               │
│  │Workflows│  │  Chat   │  │ Builder │  │  Tools  │               │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘               │
└───────┼────────────┼────────────┼────────────┼───────────────────────┘
        │            │            │            │
        └────────────┴─────┬──────┴────────────┘
                           │ OAuth2 Bearer Token
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        STROMBOLI API                                 │
│                                                                      │
│  Authentication: OAuth2 (Authentik)                                 │
│                                                                      │
│  Endpoints:                                                          │
│    POST /run              - Execute Claude (sync)                   │
│    POST /runs             - Execute Claude (async)                  │
│    GET  /runs/{id}        - Get run status/output                   │
│    GET  /runs             - List runs                               │
│    GET  /health           - Health check                            │
│    GET  /claude/status    - Check Claude config                     │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     PODMAN CONTAINERS                                │
│                                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │
│  │ Claude Code │  │ Claude Code │  │ Claude Code │                 │
│  │ + gh CLI    │  │ + custom    │  │ + tools     │                 │
│  │ + git       │  │   tools     │  │             │                 │
│  └─────────────┘  └─────────────┘  └─────────────┘                 │
│                                                                      │
│  Security:                                                           │
│  - Resource limits (CPU, memory)                                    │
│  - Network isolation                                                │
│  - Read-only root filesystem                                        │
│  - Dropped capabilities                                             │
│  - Workspace path validation                                        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Integration with n8n

n8n handles:
- Workflow orchestration
- Parallel execution
- Human-in-the-loop (wait for input)
- Notifications (Slack, Discord)
- GitHub webhooks
- Retry logic
- State persistence

Stromboli handles:
- Secure Claude execution
- Container isolation
- Token management
- Workspace mounting

### Custom n8n Node

Create `n8n-nodes-stromboli` package for seamless integration:

```
n8n-nodes-stromboli/
├── nodes/Stromboli/
│   ├── Stromboli.node.ts
│   └── stromboli.svg
├── credentials/
│   └── StromboliApi.credentials.ts
└── package.json
```

## Roadmap

### Phase 1: Secure MVP (Current)
- [x] Basic /run endpoint
- [x] Claude command builder
- [x] Podman command builder
- [ ] Fix security issues (path validation, thread-safe IDs)
- [ ] Add OAuth2 authentication
- [ ] Add resource limits to containers

### Phase 2: Async Execution
- [ ] POST /runs (async)
- [ ] GET /runs/{id} (status + output)
- [ ] GET /runs (list)
- [ ] Background job processing

### Phase 3: n8n Integration
- [ ] Create n8n custom node
- [ ] Docker compose setup
- [ ] First workflow: PR review automation

### Phase 4: Multi-App Support
- [ ] Web UI chat interface
- [ ] Slide builder integration
- [ ] Other AI-powered apps

## Security Requirements

### Authentication
- OAuth2 via Authentik
- Bearer token validation
- Scope-based permissions

### Container Security
```go
args := []string{
    "podman", "run", "--rm",
    "--memory=2g",
    "--cpus=2",
    "--cap-drop=ALL",
    "--security-opt=no-new-privileges",
    "--read-only",
    "--tmpfs=/tmp:rw,noexec,nosuid,size=100m",
    "--network=none",  // or restricted network
}
```

### Workspace Validation
- Allowlist of permitted paths
- Symlink resolution
- Path traversal prevention

## Example Workflow: PR Review

```
n8n Workflow:
1. [GitHub Webhook] → PR opened
2. [Stromboli] → Analyze code changes
3. [IF] Issues found?
   ├─ Yes → [Stromboli] → Create review comments
   │        [Slack] → Notify developer
   │        [Wait] → Human fixes
   │        Loop back to step 2
   └─ No  → [Stromboli] → Approve PR
            [GitHub] → Auto-merge
```

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.24+ |
| Web Framework | Gin |
| Containers | Podman |
| Auth | OAuth2 (Authentik) |
| Orchestration | n8n (external) |
| Database | SQLite → PostgreSQL |

## Key Decisions

1. **Keep Stromboli simple** - It's just an execution layer
2. **OAuth is mandatory** - No unauthenticated access
3. **n8n for orchestration** - Don't reinvent workflow engine
4. **Custom n8n node** - First-class integration
5. **Multi-app future** - API designed for various consumers
