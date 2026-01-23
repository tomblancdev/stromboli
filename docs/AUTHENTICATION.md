# Authentication Guide

Stromboli supports two authentication methods:

1. **Legacy Token Authentication** - Simple bearer tokens (backward compatible)
2. **JWT Authentication** - Token-based authentication with expiration and refresh capabilities

## Legacy Token Authentication

The simplest form of authentication using static API tokens.

### Configuration

Set environment variables:

```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_API_TOKENS="token1,token2,token3"
```

### Usage

Include the token in the Authorization header:

```bash
curl -H "Authorization: Bearer token1" \
  http://localhost:8080/run \
  -d '{"prompt": "hello"}'
```

### Limitations

- No expiration
- No refresh mechanism
- Tokens are long-lived and must be manually rotated

## JWT Authentication

Modern token-based authentication with automatic expiration and refresh capabilities.

### Enabling JWT

JWT authentication is enabled when you set the JWT secret:

```bash
export STROMBOLI_JWT_SECRET="your-secret-key-here"
export STROMBOLI_JWT_EXPIRY="24h"           # Optional, defaults to 24h
export STROMBOLI_JWT_REFRESH_EXPIRY="168h" # Optional, defaults to 7 days
```

**Security Note**: The JWT secret should be a long, random string. Generate one with:

```bash
openssl rand -base64 32
```

### Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `STROMBOLI_JWT_SECRET` | Secret key for signing tokens (required for JWT) | - |
| `STROMBOLI_JWT_EXPIRY` | Access token lifetime | `24h` |
| `STROMBOLI_JWT_REFRESH_EXPIRY` | Refresh token lifetime | `168h` (7 days) |

Time format examples: `1h`, `24h`, `7d`, `168h`

### Obtaining JWT Tokens

#### Step 1: Generate Tokens

Use your API token to generate JWT tokens:

```bash
curl -X POST http://localhost:8080/auth/token \
  -H "Authorization: Bearer your-api-token" \
  -H "Content-Type: application/json" \
  -d '{"client_id": "my-client"}'
```

Response:

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "bearer",
  "expires_in": 86400
}
```

#### Step 2: Use Access Token

Use the access token for API requests:

```bash
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  http://localhost:8080/run \
  -d '{"prompt": "hello"}'
```

### Token Refresh Flow

When your access token expires, use the refresh token to get a new one:

```bash
curl -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}'
```

Response:

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "bearer",
  "expires_in": 86400
}
```

### Token Validation

Validate a token and inspect its claims:

```bash
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  http://localhost:8080/auth/validate
```

Response:

```json
{
  "valid": true,
  "subject": "my-client",
  "expires_at": 1735689600
}
```

## API Endpoints

### POST /auth/token

Generate new JWT access and refresh tokens.

**Authentication**: Requires valid API token

**Request Body**:
```json
{
  "client_id": "string"
}
```

**Response**:
```json
{
  "access_token": "string",
  "refresh_token": "string",
  "token_type": "bearer",
  "expires_in": 86400
}
```

### POST /auth/refresh

Refresh an expiring access token.

**Authentication**: None (validates refresh token from request body)

**Request Body**:
```json
{
  "refresh_token": "string"
}
```

**Response**:
```json
{
  "access_token": "string",
  "refresh_token": "string",
  "token_type": "bearer",
  "expires_in": 86400
}
```

### GET /auth/validate

Validate a token and return its claims.

**Authentication**: Validates token from Authorization header

**Response**:
```json
{
  "valid": true,
  "subject": "string",
  "expires_at": 1735689600
}
```

### POST /auth/logout (v1.3+)

Invalidate a token (logout). The token is added to a blacklist and will be rejected on subsequent requests until it naturally expires.

**Authentication**: Requires valid JWT token

**Request**: No body required

**Response**:
```json
{
  "success": true,
  "message": "token invalidated"
}
```

**Usage:**
```bash
curl -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Note:** The blacklist is kept in memory and automatically cleans up expired tokens every hour. Blacklisted tokens are identified by their unique JTI (JWT ID) claim.

## Migration from Legacy to JWT

JWT authentication is fully backward compatible. You can enable JWT while keeping legacy tokens active:

```bash
# Keep existing API tokens
export STROMBOLI_API_TOKENS="legacy-token-1,legacy-token-2"

# Enable JWT
export STROMBOLI_JWT_SECRET="your-secret-key"
```

**Migration Strategy**:

1. **Phase 1**: Enable JWT alongside legacy tokens
2. **Phase 2**: Issue JWT tokens to all clients
3. **Phase 3**: Clients migrate to using JWT tokens
4. **Phase 4**: Remove legacy tokens from configuration

## How It Works

### Token Types

1. **Access Token**: Short-lived token (default 24h) used for API requests
2. **Refresh Token**: Longer-lived token (default 7 days) used to obtain new access tokens

### Token Claims

JWT tokens include these claims:

- `sub` (subject): Client/user identifier
- `iat` (issued at): Token creation timestamp
- `exp` (expires at): Token expiration timestamp
- `is_refresh`: Boolean flag indicating if this is a refresh token

### Authentication Middleware

The middleware validates tokens in this order:

1. Check if auth is enabled
2. Extract Bearer token from Authorization header
3. Try validating as legacy token first (backward compatibility)
4. If JWT is enabled and token isn't legacy, validate as JWT
5. Reject request if both validations fail

### Security Best Practices

1. **Secret Management**
   - Use a strong, randomly generated secret
   - Never commit secrets to version control
   - Rotate secrets periodically
   - Use different secrets for dev/staging/prod

2. **Token Expiry**
   - Keep access tokens short-lived (hours, not days)
   - Make refresh tokens longer but not infinite
   - Force re-authentication after refresh token expires

3. **Transport Security**
   - Always use HTTPS in production
   - Never log tokens
   - Never include tokens in URLs

4. **Token Storage (Client-Side)**
   - Store access tokens in memory when possible
   - Store refresh tokens in secure storage (e.g., httpOnly cookies)
   - Clear tokens on logout

## Troubleshooting

### "JWT authentication not configured"

**Cause**: JWT secret is not set

**Solution**: Set `STROMBOLI_JWT_SECRET` environment variable

### "invalid or expired token"

**Causes**:
- Token has expired
- Token was signed with a different secret
- Token is malformed

**Solutions**:
- Use refresh token to get a new access token
- Re-generate tokens if secret was rotated
- Check token format (should be three base64 strings separated by dots)

### "refresh token cannot be used for access"

**Cause**: Trying to use a refresh token for API requests

**Solution**: Use refresh tokens only with `/auth/refresh` endpoint. Use access tokens for API requests.

### "access token cannot be used for refresh"

**Cause**: Trying to refresh using an access token

**Solution**: Use the refresh token (not access token) with `/auth/refresh` endpoint.

## Example Integration

### Python Client

```python
import requests
import time

class StromboliClient:
    def __init__(self, base_url, api_token):
        self.base_url = base_url
        self.api_token = api_token
        self.access_token = None
        self.refresh_token = None
        self.token_expiry = 0

    def authenticate(self):
        """Generate JWT tokens using API token"""
        response = requests.post(
            f"{self.base_url}/auth/token",
            headers={"Authorization": f"Bearer {self.api_token}"},
            json={"client_id": "my-client"}
        )
        response.raise_for_status()
        data = response.json()

        self.access_token = data["access_token"]
        self.refresh_token = data["refresh_token"]
        self.token_expiry = time.time() + data["expires_in"]

    def refresh_access_token(self):
        """Refresh the access token"""
        response = requests.post(
            f"{self.base_url}/auth/refresh",
            json={"refresh_token": self.refresh_token}
        )
        response.raise_for_status()
        data = response.json()

        self.access_token = data["access_token"]
        self.refresh_token = data["refresh_token"]
        self.token_expiry = time.time() + data["expires_in"]

    def ensure_token(self):
        """Ensure we have a valid access token"""
        if not self.access_token or time.time() >= self.token_expiry - 300:
            if not self.refresh_token:
                self.authenticate()
            else:
                try:
                    self.refresh_access_token()
                except:
                    self.authenticate()

    def run(self, prompt):
        """Execute a Claude Code request"""
        self.ensure_token()

        response = requests.post(
            f"{self.base_url}/run",
            headers={"Authorization": f"Bearer {self.access_token}"},
            json={"prompt": prompt}
        )
        response.raise_for_status()
        return response.json()

# Usage
client = StromboliClient("http://localhost:8080", "your-api-token")
result = client.run("hello world")
print(result)
```

### Shell Script

```bash
#!/bin/bash

BASE_URL="http://localhost:8080"
API_TOKEN="your-api-token"
TOKEN_FILE="$HOME/.stromboli-tokens"

# Generate tokens
generate_tokens() {
    response=$(curl -s -X POST "$BASE_URL/auth/token" \
        -H "Authorization: Bearer $API_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"client_id": "shell-client"}')

    echo "$response" > "$TOKEN_FILE"
    chmod 600 "$TOKEN_FILE"
}

# Get access token
get_access_token() {
    if [ ! -f "$TOKEN_FILE" ]; then
        generate_tokens
    fi

    jq -r '.access_token' "$TOKEN_FILE"
}

# Refresh tokens if needed
refresh_if_needed() {
    # Simple implementation - always refresh
    refresh_token=$(jq -r '.refresh_token' "$TOKEN_FILE")

    response=$(curl -s -X POST "$BASE_URL/auth/refresh" \
        -H "Content-Type: application/json" \
        -d "{\"refresh_token\": \"$refresh_token\"}")

    if [ $? -eq 0 ]; then
        echo "$response" > "$TOKEN_FILE"
    else
        generate_tokens
    fi
}

# Make API request
ACCESS_TOKEN=$(get_access_token)

curl -X POST "$BASE_URL/run" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"prompt": "hello world"}'
```

## References

- [JWT RFC 7519](https://tools.ietf.org/html/rfc7519)
- [OAuth 2.0 Refresh Tokens](https://tools.ietf.org/html/rfc6749#section-6)
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
