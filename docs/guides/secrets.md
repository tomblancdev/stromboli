# Secrets

Inject tokens and API keys into agent containers using Podman's native secret store.

!!! info "Single-tenant design"
    Stromboli is designed for single-tenant deployments. All API users have access to all Podman secrets on the host. For multi-tenant environments, deploy separate instances per tenant.

## How it works

```
Host: podman secret create github-token ~/.gh/token
         ↓ stored securely in Podman
Podman Secrets Store
         ↓ API request: secrets_env
Agent Container: GH_TOKEN=<secret value>
```

1. You create secrets using `podman secret create`
2. API requests reference secrets by name via `secrets_env`
3. Stromboli injects them as environment variables inside the container

## Creating secrets

```bash
# GitHub token
echo "ghp_xxxxxxxxxxxx" | podman secret create github-token -

# From gh CLI config (requires yq)
yq -r '.["github.com"].oauth_token' ~/.config/gh/hosts.yml | podman secret create github-token -

# GitLab token
echo "$GITLAB_TOKEN" | podman secret create gitlab-token -

# Any API key
echo "sk-xxxxxxxxxxxx" | podman secret create openai-key -
```

## Using secrets

### List available secrets

```bash
curl localhost:8080/secrets
```

```json
{"secrets": ["claude-credentials", "github-token", "gitlab-token"]}
```

### Inject into an agent

Map Podman secret names to environment variables:

```bash
curl -X POST localhost:8080/run \
  -d '{
    "prompt": "Review the open PRs using gh CLI",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myrepo:/workspace"],
      "secrets_env": {
        "GH_TOKEN": "github-token",
        "GITLAB_TOKEN": "gitlab-token"
      }
    }
  }'
```

Format: `{"ENV_VAR_NAME": "podman_secret_name"}`

## Validation rules

| Rule | Detail |
|---|---|
| Env var format | Must start with letter or underscore, contain only `[a-zA-Z0-9_]` |
| Blocked vars | `LD_PRELOAD`, `LD_LIBRARY_PATH` are rejected |
| Secret name | Non-empty, max 253 characters |
| Max per request | 50 secrets |

## Managing secrets

```bash
# List
podman secret ls

# Inspect metadata (not the value)
podman secret inspect github-token

# Update (remove + recreate)
podman secret rm github-token
echo "new-token" | podman secret create github-token -

# Delete
podman secret rm github-token
```

## Security guidelines

**Do:**

- Use Podman secrets — never pass tokens directly in API requests
- Use minimal permissions (read-only tokens when possible)
- Rotate secrets regularly
- Use descriptive names (`github-token`, `gitlab-readonly`)

**Don't:**

- Hardcode tokens in code or configs
- Use admin/full-access tokens when read-only works
- Share secrets across environments

### Recommended token permissions

| Service | Use case | Permissions |
|---|---|---|
| GitHub | PR review | `repo:read`, `pull_request:read` |
| GitHub | PR actions | `repo:write`, `pull_request:write` |
| GitLab | Read repos | `read_repository` |
| GitLab | CI/CD | `read_repository`, `read_api` |

## Troubleshooting

**Secret not found** — Create it first: `echo "$TOKEN" | podman secret create name -`

**Permission denied** — Check the secret exists (`podman secret ls`) and the name in the request matches exactly.

**Token expired** — Update the secret: `podman secret rm name && echo "$NEW_TOKEN" | podman secret create name -`
