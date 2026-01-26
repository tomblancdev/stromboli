# Workspaces

Workspaces allow Claude agents to access directories on the host system.

## How Workspaces Work

When you specify a workspace, Stromboli mounts it into the container:

```
Host: /home/user/myproject
  ↓
Container: /workspace
```

The agent can read and write files in this directory.

## Using Workspaces

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "List all Go files in this project",
    "workspace": "/home/user/myproject"
  }'
```

## Security: Workspace Allowlist

By default, Stromboli restricts which directories can be mounted.

### Configure Allowed Workspaces

```bash
export STROMBOLI_AGENT_ALLOWED_WORKSPACES="/home/user/projects,/data/workspaces"
```

Or in config file:
```yaml
agent:
  allowed_workspaces:
    - /home/user/projects
    - /data/workspaces
```

### Subdirectories

Subdirectories of allowed paths are automatically permitted:

```bash
# If /home/user/projects is allowed:
/home/user/projects           # ✅ Allowed
/home/user/projects/foo       # ✅ Allowed
/home/user/projects/foo/bar   # ✅ Allowed
/home/user/other              # ❌ Denied
```

### Path Traversal Protection

Stromboli blocks path traversal attempts:

```bash
# These are blocked:
/home/user/projects/../secrets  # ❌ Blocked
/home/user/projects/./../../    # ❌ Blocked
```

## Additional Volumes

Mount extra directories with the `volumes` option:

```bash
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "Process the data",
    "workspace": "/home/user/project",
    "podman": {
      "volumes": [
        "/data/input:/input:ro",
        "/data/output:/output"
      ]
    }
  }'
```

Volume format: `host_path:container_path[:options]`

Options:
- `ro` - Read-only
- `rw` - Read-write (default)

## Best Practices

### 1. Use Specific Paths

```bash
# Good: Specific project
"workspace": "/home/user/projects/myapp"

# Bad: Too broad
"workspace": "/home/user"
```

### 2. Limit Permissions

```bash
# Mount data as read-only when possible
"volumes": ["/data/config:/config:ro"]
```

### 3. Don't Mount Sensitive Directories

Never allow these paths:
- `/` (root)
- `/etc`
- `/home/user/.ssh`
- `/home/user/.aws`

### 4. Use Absolute Paths

```bash
# Good
"workspace": "/home/user/projects/myapp"

# Bad - relative paths may not work as expected
"workspace": "./myapp"
```

## Troubleshooting

### "Workspace not allowed"

Add the path to your allowlist:
```bash
export STROMBOLI_AGENT_ALLOWED_WORKSPACES="/your/path"
```

### "Path traversal detected"

Your path contains `..` - use an absolute path instead.

### Files not visible to agent

Check:
1. The path exists on the host
2. Stromboli has read permission
3. The path is in the allowlist
