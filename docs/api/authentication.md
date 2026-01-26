# Authentication

Stromboli supports JWT-based authentication for securing the API.

## Enabling Authentication

Set the required environment variables:

```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_AUTH_JWT_SECRET="your-secure-256-bit-secret"
export STROMBOLI_AUTH_API_TOKEN="your-api-token-for-generating-jwts"
```

## Authentication Flow

```mermaid
sequenceDiagram
    participant Client
    participant Stromboli

    Client->>Stromboli: POST /auth/token (API Token)
    Stromboli->>Client: {access_token, refresh_token}

    Client->>Stromboli: POST /run (Bearer access_token)
    Stromboli->>Client: {output: "..."}

    Note over Client,Stromboli: When access token expires...

    Client->>Stromboli: POST /auth/refresh (refresh_token)
    Stromboli->>Client: {new access_token, new refresh_token}
```

## Generating Tokens

### POST /auth/token

Exchange your API token for JWT tokens.

**Request:**
```bash
curl -X POST http://localhost:8080/auth/token \
  -H "Authorization: Bearer your-api-token" \
  -H "Content-Type: application/json" \
  -d '{"client_id": "my-app"}'
```

**Response:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

## Using Tokens

Include the access token in the `Authorization` header:

```bash
curl -X POST http://localhost:8080/run \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello"}'
```

## Refreshing Tokens

### POST /auth/refresh

Get new tokens using a refresh token.

**Request:**
```bash
curl -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "eyJhbGciOiJIUzI1NiIs..."}'
```

**Response:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

## Validating Tokens

### POST /auth/validate

Check if a token is valid.

**Request:**
```bash
curl -X POST http://localhost:8080/auth/validate \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

**Response:**
```json
{
  "valid": true,
  "claims": {
    "sub": "my-app",
    "exp": 1640000000,
    "iat": 1639996400
  }
}
```

## Logging Out

### POST /auth/logout

Invalidate a token (adds to blacklist).

**Request:**
```bash
curl -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

**Response:**
```json
{
  "success": true,
  "message": "Token invalidated"
}
```

## Token Lifetimes

| Token Type | Lifetime | Configurable |
|------------|----------|--------------|
| Access Token | 1 hour | Not yet |
| Refresh Token | 7 days | Not yet |

## Security Best Practices

### 1. Use Strong Secrets

```bash
# Generate a secure secret
openssl rand -base64 32
```

### 2. Store Tokens Securely

- Never log tokens
- Don't store in localStorage for web apps
- Use secure, HTTP-only cookies when possible

### 3. Rotate Secrets Periodically

Change `STROMBOLI_AUTH_JWT_SECRET` periodically:
1. Add new secret
2. Accept both old and new
3. Remove old secret

### 4. Use HTTPS in Production

Always use TLS/SSL to protect tokens in transit.

## Public Endpoints

These endpoints don't require authentication:

- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics
- `POST /auth/token` - Token generation (requires API token)
- `POST /auth/refresh` - Token refresh

## Error Responses

| Status | Error | Description |
|--------|-------|-------------|
| 401 | `token required` | No token provided |
| 401 | `invalid token` | Token is malformed or invalid |
| 401 | `token expired` | Token has expired |
| 401 | `token blacklisted` | Token was invalidated |
