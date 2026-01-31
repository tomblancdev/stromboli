# Sessions

Sessions allow conversations to persist across multiple requests.

## How Sessions Work

```mermaid
sequenceDiagram
    participant User
    participant Stromboli
    participant Container
    participant Storage

    User->>Stromboli: POST /run (prompt)
    Stromboli->>Container: Run Claude
    Container->>Storage: Save session
    Stromboli->>User: {session_id: "abc123"}

    User->>Stromboli: POST /run (resume: true)
    Stromboli->>Storage: Load session
    Stromboli->>Container: Continue conversation
    Container->>User: Response with context
```

## Creating a Session

Every request creates a new session by default:

```bash
curl -X POST http://localhost:8080/run \
  -d '{"prompt": "Hello, remember my name is Tom"}'
```

Response:
```json
{
  "output": "Hello Tom! Nice to meet you...",
  "session_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## Resuming a Session

Use the session ID to continue the conversation:

```bash
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "What is my name?",
    "claude": {
      "session_id": "550e8400-e29b-41d4-a716-446655440000",
      "resume": true
    }
  }'
```

Response:
```json
{
  "output": "Your name is Tom!",
  "session_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## Session Options

### Resume vs Continue

| Option | Description |
|--------|-------------|
| `resume: true` | Continue specific session by ID |
| `continue: true` | Continue most recent session in workspace |

```bash
# Resume specific session
{"claude": {"session_id": "abc123", "resume": true}}

# Continue most recent
{"claude": {"continue": true}}
```

### Fork a Session

Create a new session from an existing one:

```bash
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "Try a different approach",
    "claude": {
      "session_id": "abc123",
      "fork_session": true
    }
  }'
```

This creates a new session with the conversation history but a new ID.

### No Persistence

Run without saving the session:

```bash
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "One-off question",
    "claude": {
      "no_persistence": true
    }
  }'
```

## Managing Sessions

### List Sessions

```bash
curl http://localhost:8080/sessions
```

Response:
```json
{
  "sessions": [
    "550e8400-e29b-41d4-a716-446655440000",
    "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
  ]
}
```

### Get Session Messages

```bash
curl http://localhost:8080/sessions/550e8400.../messages
```

Response:
```json
{
  "messages": [
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi there!"}
  ]
}
```

### Delete a Session

```bash
curl -X DELETE http://localhost:8080/sessions/550e8400...
```

Response:
```json
{
  "success": true,
  "session_id": "550e8400..."
}
```

## Session Storage

Sessions are stored in the directory configured by `STROMBOLI_AGENT_SESSIONS_DIR` (default: `./sessions`).

```
sessions/
├── 550e8400-e29b-41d4-a716-446655440000/
│   ├── .claude/
│   │   ├── projects/
│   │   ├── debug/
│   │   └── todos/
│   └── .claude.json
└── 6ba7b810-9dad-11d1-80b4-00c04fd430c8/
    └── ...
```

!!! tip "Cleanup"
    Periodically delete old sessions to free disk space:
    ```bash
    # Delete sessions older than 7 days
    find ./sessions -type d -mtime +7 -exec rm -rf {} \;
    ```

## Containerized Deployment

When running Stromboli in a container, session directories need special handling because:

1. Stromboli creates session directories inside its container
2. Agent containers need to mount these directories from the **host**

### Two-Path Configuration

```bash
# Path inside Stromboli container (where sessions are created)
STROMBOLI_AGENT_SESSIONS_DIR="/app/sessions"

# Path on the host (for mounting into agent containers)
STROMBOLI_AGENT_SESSIONS_HOST_DIR="/home/user/.stromboli/sessions"
```

### Docker Compose Setup

```yaml
services:
  stromboli:
    volumes:
      # Mount host path at the internal path
      - /home/user/.stromboli/sessions:/app/sessions
    environment:
      STROMBOLI_AGENT_SESSIONS_DIR: "/app/sessions"
      STROMBOLI_AGENT_SESSIONS_HOST_DIR: "/home/user/.stromboli/sessions"
      # Include sessions in allowed volumes
      STROMBOLI_AGENT_ALLOWED_VOLUMES: "/home/user/projects,/home/user/.stromboli/sessions"
```

```mermaid
graph TB
    subgraph Host
        HS["/home/user/.stromboli/sessions"]
    end
    subgraph Stromboli Container
        SC["/app/sessions"]
    end
    subgraph Agent Container
        AC["/home/user"]
    end
    HS -->|"bind mount"| SC
    HS -->|"volume mount"| AC
```

!!! warning "Both Paths Required"
    If `SESSIONS_HOST_DIR` is not set, it defaults to `SESSIONS_DIR`.
    This only works when running Stromboli directly on the host (not in a container).
