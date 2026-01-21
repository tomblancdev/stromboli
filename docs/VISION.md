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
| Database | PostgreSQL + ClickHouse |

## Key Decisions

1. **Keep Stromboli simple** - It's just an execution layer
2. **OAuth is mandatory** - No unauthenticated access
3. **n8n for orchestration** - Don't reinvent workflow engine
4. **Custom n8n node** - First-class integration
5. **Multi-app future** - API designed for various consumers

---

## Future Considerations: Multi-Node & Analytics

### Multi-Machine Architecture

When scaling Stromboli across multiple machines:

```
                         Load Balancer
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
 ┌─────────────┐       ┌─────────────┐       ┌─────────────┐
 │ Stromboli 1 │       │ Stromboli 2 │       │ Stromboli 3 │
 │ (Stateless) │       │ (Stateless) │       │ (Stateless) │
 └──────┬──────┘       └──────┬──────┘       └──────┬──────┘
        │                     │                     │
        └─────────────────────┼─────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │ PostgreSQL│   │ClickHouse │   │   Redis   │
       │  (state)  │   │ (analytics)│   │  (queue)  │
       └───────────┘   └───────────┘   └───────────┘
```

**Key Principle:** Each Stromboli node must be **stateless** for horizontal scaling.

### Data Layer Split

| Database | Purpose | Data |
|----------|---------|------|
| **PostgreSQL** | Operational | Run state, users, configs, CRUD |
| **ClickHouse** | Analytics | Events, logs, metrics, time-series |
| **Redis** | Coordination | Job queue, pub/sub, cache |

### Why ClickHouse for Logs/Analytics

- Column-oriented = fast aggregations
- 10-100x compression vs row databases
- Optimized for time-series queries
- Handles high-volume event ingestion
- Materialized views for real-time metrics

### ClickHouse Schema (Draft)

```sql
-- Run events (immutable log)
CREATE TABLE run_events (
    event_id UUID,
    run_id UUID,
    event_type Enum8('started'=1, 'tool_call'=2, 'completed'=3, 'failed'=4),
    node_id String,
    user_id UUID,
    timestamp DateTime64(3),
    data String,  -- JSON payload
    model LowCardinality(String),
    duration_ms UInt32,
    tokens_used UInt32,
    cost_usd Decimal64(4),
    date Date DEFAULT toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (user_id, run_id, timestamp);

-- Tool usage analytics
CREATE TABLE tool_calls (
    run_id UUID,
    tool_name LowCardinality(String),
    duration_ms UInt32,
    success UInt8,
    timestamp DateTime64(3),
    date Date DEFAULT toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (tool_name, timestamp);
```

### Example Analytics Queries

```sql
-- Cost per user last 24h
SELECT user_id, sum(cost_usd) as total_cost, count() as runs
FROM run_events
WHERE timestamp > now() - INTERVAL 24 HOUR
GROUP BY user_id ORDER BY total_cost DESC;

-- Most used tools
SELECT tool_name, count() as calls, avg(duration_ms) as avg_duration
FROM tool_calls WHERE date = today()
GROUP BY tool_name ORDER BY calls DESC;

-- Failure rate by model
SELECT model,
    countIf(event_type = 'completed') as success,
    countIf(event_type = 'failed') as failed
FROM run_events
WHERE date >= today() - 7
GROUP BY model;
```

### What to Capture

```
📝 Execution Logs         📊 Metrics              🔍 Audit Trail
─────────────────         ────────                ─────────────
• AI responses            • Execution time        • Who ran what
• Tool calls              • Token usage           • When
• Claude's reasoning      • Container resources   • From where
• Errors/failures         • Success/failure rate  • Input params
• Workspace changes       • Queue depth           • OAuth user
```

### Implementation Order

1. Start single-node with PostgreSQL only
2. Add Redis when job queuing needed
3. Add ClickHouse when analytics/logging at scale needed
4. Scale to multi-node when single machine isn't enough
