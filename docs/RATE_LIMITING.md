# Rate Limiting

Stromboli includes rate limiting middleware to protect the API from abuse and DoS attacks.

## Configuration

Rate limiting is **disabled by default** for backward compatibility. Enable it using environment variables:

### Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `STROMBOLI_RATE_LIMIT_ENABLED` | Enable/disable rate limiting | `false` | `true` |
| `STROMBOLI_RATE_LIMIT_RPS` | Requests per second allowed | `10` | `20` |
| `STROMBOLI_RATE_LIMIT_BURST` | Maximum burst size | `20` | `50` |

### Example Configuration

```bash
# Enable rate limiting with 10 requests/second, burst of 20
export STROMBOLI_RATE_LIMIT_ENABLED=true
export STROMBOLI_RATE_LIMIT_RPS=10
export STROMBOLI_RATE_LIMIT_BURST=20

# Start server
./bin/stromboli
```

## How It Works

- **Per-IP Limiting**: Rate limits are applied per client IP address
- **Token Bucket Algorithm**: Uses `golang.org/x/time/rate` for efficient rate limiting
- **Automatic Headers**: Response includes rate limit information headers
- **Graceful Degradation**: When disabled, middleware is a no-op with zero overhead

### Rate Limit Headers

All protected endpoints include these headers when rate limiting is enabled:

- `X-RateLimit-Limit`: Maximum requests allowed per period
- `X-RateLimit-Remaining`: Remaining requests in current period
- `X-RateLimit-Reset`: Unix timestamp when the limit resets

### Example Response

**Within Rate Limit:**
```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 5
X-RateLimit-Reset: 1706140800
```

**Rate Limit Exceeded:**
```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1706140801

{
  "error": "rate limit exceeded"
}
```

## Protected Endpoints

Rate limiting is applied to all protected routes:

- `GET /claude/status`
- `POST /run`
- `GET /run/stream`
- `POST /run/async`
- `GET /jobs`
- `GET /jobs/:id`
- `DELETE /jobs/:id`
- `GET /sessions`
- `DELETE /sessions/:id`

### Excluded Endpoints

These endpoints are **not** rate limited:

- `GET /health` - Health checks should always succeed
- `GET /metrics` - Prometheus metrics scraping

## Client IP Detection

The rate limiter extracts the client IP from the request in this order:

1. `X-Forwarded-For` header (first IP)
2. `X-Real-IP` header
3. `RemoteAddr` from the request

This works correctly behind reverse proxies like nginx or Traefik.

## Testing

Run rate limiting tests:

```bash
# Run rate limit tests only
go test -v ./internal/api/ratelimit_test.go ./internal/api/ratelimit.go

# Run all API tests
go test -v ./internal/api/...

# Or use the test script
./test-ratelimit.sh
```

## Production Recommendations

For production deployments:

1. **Enable rate limiting** to prevent abuse
2. **Adjust limits** based on your expected traffic:
   - Low traffic API: 5-10 RPS
   - Medium traffic API: 20-50 RPS
   - High traffic API: 100+ RPS
3. **Monitor metrics** to identify rate limit hits
4. **Consider per-user limits** if authentication is enabled (future enhancement)

### Example Production Config

```bash
# Conservative limits for production
STROMBOLI_RATE_LIMIT_ENABLED=true
STROMBOLI_RATE_LIMIT_RPS=20
STROMBOLI_RATE_LIMIT_BURST=40
```

## Architecture

The rate limiter uses:

- **`golang.org/x/time/rate`**: Standard library rate limiting
- **Per-IP tracking**: Separate limiter instance for each unique IP
- **Token bucket algorithm**: Allows burst traffic while maintaining average rate
- **Zero-copy middleware**: No overhead when disabled

## Future Enhancements

Potential improvements:

- [ ] Per-user rate limits (when auth is enabled)
- [ ] Redis-backed distributed rate limiting
- [ ] Rate limit exemptions for trusted IPs
- [ ] Configurable rate limits per endpoint
- [ ] Rate limit metrics and alerting
