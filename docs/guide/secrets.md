# Secrets Management

Stromboli allows you to securely inject secrets into agent containers as environment variables. This enables agents to access external services like GitHub, GitLab, cloud providers, etc.

> **⚠️ Security Model**: Stromboli is designed for **single-tenant deployments**. All API users have access to all Podman secrets on the host. For multi-tenant environments, deploy separate Stromboli instances per tenant.

## How It Works

```
┌─────────────────────────────────────────────────────┐
│              Host Machine                            │
│  podman secret create github-token ~/.gh/token      │
└─────────────────────┬───────────────────────────────┘
                      │ stored securely
                      ▼
┌─────────────────────────────────────────────────────┐
│              Podman Secrets Store                    │
│  - claude-credentials (managed by stromboli)        │
│  - github-token (user-created)                      │
│  - gitlab-token (user-created)                      │
└─────────────────────┬───────────────────────────────┘
                      │ API request: secrets_env
                      ▼
┌─────────────────────────────────────────────────────┐
│              Agent Container                         │
│  GH_TOKEN=<secret value>                            │
│  GITLAB_TOKEN=<secret value>                        │
└─────────────────────────────────────────────────────┘
```

## Creating Secrets

### GitHub CLI Token

```bash
# Option 1: Extract OAuth token from gh CLI config (requires yq)
yq -r '.["github.com"].oauth_token' ~/.config/gh/hosts.yml | podman secret create github-token -

# Option 2: Create from a Personal Access Token
echo "ghp_xxxxxxxxxxxx" | podman secret create github-token -

# Option 3: From environment variable
echo "$GITHUB_TOKEN" | podman secret create github-token -

# Option 4: Create interactively (token won't appear in shell history)
podman secret create github-token -
# Then paste the token and press Ctrl+D
```

### GitLab Token

```bash
echo "$GITLAB_TOKEN" | podman secret create gitlab-token -
```

### Generic API Keys

```bash
echo "sk-xxxxxxxxxxxx" | podman secret create openai-key -
echo "xoxb-xxxxxxxxxxxx" | podman secret create slack-token -
```

## Using Secrets in API Requests

### List Available Secrets

```bash
curl -s http://localhost:8080/secrets | jq
```

Response:
```json
{
  "secrets": ["claude-credentials", "github-token", "gitlab-token"]
}
```

### Inject Secrets into Agent

Use the `secrets_env` field in your request to map Podman secrets to environment variables:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Review the open PRs on this repo using gh CLI",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"],
      "secrets_env": {
        "GH_TOKEN": "github-token",
        "GITLAB_TOKEN": "gitlab-token"
      }
    }
  }'
```

The format is:
```json
{
  "secrets_env": {
    "<ENV_VAR_NAME>": "<podman_secret_name>"
  }
}
```

### Validation Rules

Stromboli validates `secrets_env` before creating containers:

| Rule | Description |
|------|-------------|
| **Env var format** | Must start with letter or underscore, contain only `[a-zA-Z0-9_]` |
| **Blocked vars** | `LD_PRELOAD`, `LD_LIBRARY_PATH` are blocked for security |
| **Secret name** | Cannot be empty, max 253 characters |
| **Max secrets** | Maximum 50 secrets per request |

Invalid requests return `400 Bad Request` with details:
```json
{
  "error": "secrets validation failed: invalid environment variable name \"MY-VAR\": must start with letter or underscore, contain only alphanumeric and underscore"
}
```

## Security Guidelines

### DO

- **Use Podman secrets** - Never pass tokens directly in API requests or environment variables
- **Use minimal permissions** - Create tokens with only the permissions needed
- **Rotate regularly** - Update secrets periodically
- **Name secrets clearly** - Use descriptive names like `github-token`, `gitlab-readonly`
- **List before using** - Check `/secrets` endpoint to see available secrets

### DON'T

- **Don't hardcode tokens** - Never put tokens in code, configs, or API requests
- **Don't share secrets** - Each environment should have its own secrets
- **Don't use overly permissive tokens** - Avoid admin/full-access tokens when read-only works
- **Don't log secrets** - Stromboli never logs secret values

### Token Permissions

Create tokens with minimal required permissions:

| Service | Use Case | Recommended Permissions |
|---------|----------|------------------------|
| GitHub | PR review | `repo:read`, `pull_request:read` |
| GitHub | PR actions | `repo:write`, `pull_request:write` |
| GitLab | Read repos | `read_repository` |
| GitLab | CI/CD | `read_repository`, `read_api` |

## Managing Secrets

### List Secrets

```bash
podman secret ls
```

### Inspect Secret (metadata only)

```bash
podman secret inspect github-token
```

### Update a Secret

```bash
# Remove old secret
podman secret rm github-token

# Create new secret with updated value
echo "new-token-value" | podman secret create github-token -
```

### Delete a Secret

```bash
podman secret rm github-token
```

## Troubleshooting

### Secret Not Found

```
Error: secret "github-token" not found
```

**Solution**: Create the secret first:
```bash
echo "$TOKEN" | podman secret create github-token -
```

### Permission Denied

If the agent can't use the secret, ensure:
1. The secret exists: `podman secret ls`
2. The secret name in the request matches exactly
3. The Podman socket is accessible

### Token Expired

If operations fail with auth errors, the token may be expired:
```bash
# Update the secret
podman secret rm github-token
echo "$NEW_TOKEN" | podman secret create github-token -
```

## Examples

### GitHub PR Review Agent

```bash
# Setup
echo "$GITHUB_TOKEN" | podman secret create github-token -

# Run
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "List open PRs and summarize the changes in PR #42",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myrepo:/workspace"],
      "secrets_env": {
        "GH_TOKEN": "github-token"
      }
    }
  }'
```

### Multi-Service Agent

```bash
# Setup multiple secrets
echo "$GITHUB_TOKEN" | podman secret create github-token -
echo "$SLACK_TOKEN" | podman secret create slack-token -
echo "$OPENAI_KEY" | podman secret create openai-key -

# Run with multiple secrets
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze the latest PR, generate a summary, and post it to #dev-updates",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myrepo:/workspace"],
      "secrets_env": {
        "GH_TOKEN": "github-token",
        "SLACK_BOT_TOKEN": "slack-token",
        "OPENAI_API_KEY": "openai-key"
      }
    }
  }'
```
