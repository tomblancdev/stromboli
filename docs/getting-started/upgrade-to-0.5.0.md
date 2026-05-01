# Upgrading to v0.5.0-alpha

v0.5.0-alpha is the largest release since v0.3.0-alpha. It ships substantial new functionality (persistent agents, webhook signing, pluggable token blacklist) plus four operationally-breaking default changes that existing deployments must address before restarting.

This page is the migration playbook. Read the [changelog](../changelog.md) for the full feature list.

## TL;DR checklist

If you're upgrading an existing deployment, work through this list in order:

- [ ] **Set a real JWT secret** — auth is now on by default, server fails fast without one
- [ ] **Update Prometheus scrape config** — `/metrics` moved to `127.0.0.1:9090` on a separate listener
- [ ] **Configure tracing TLS** — `insecure: false` is now the default; either provide TLS to your collector or set `STROMBOLI_TRACING_INSECURE=true` for localhost
- [ ] **Verify log parsing** — JSON is now the default log format; `STROMBOLI_LOG_FORMAT=text` if your aggregator expects plain text
- [ ] **Drop Windows binaries from your fleet** — they're no longer published; run via WSL2 or a Linux container
- [ ] **(Recommended) Sign outgoing webhooks** — set `STROMBOLI_WEBHOOK_SIGNING_SECRET` and verify on receivers
- [ ] **(Recommended) Pick a blacklist backend** — `memory` is fine for single-instance; `bolt` for durability across restart

Each item below explains what changed, why, how to verify, and how to opt out if you have to.

---

## Auth on by default

**What changed.** `STROMBOLI_AUTH_ENABLED` defaults to `true`. The server refuses to start if `STROMBOLI_JWT_SECRET` is empty, shorter than 32 characters, or matches a known placeholder string (`change-me`, `your-jwt-secret`, etc.).

**Why.** Defaults that ship insecure get pushed to production. The old `false` default meant operators had to remember to flip the switch — now production fails loud at boot rather than silently exposing an unauthenticated API.

**Migration.**

```bash
export STROMBOLI_JWT_SECRET="$(openssl rand -base64 32)"
./bin/stromboli
```

You should see in the startup logs:

```
INFO Authentication enabled tokens=0
INFO JWT authentication enabled access_expiry=24h refresh_expiry=168h
```

**Opt-out (dev only).** If you genuinely need an unauthenticated server (local dev, ephemeral CI runner, etc.):

```bash
export STROMBOLI_AUTH_ENABLED=false
```

The startup log will say `INFO Authentication disabled`. **Do not deploy this way.**

**Failure mode.** If you upgrade without setting a secret, you'll see:

```
ERROR Failed to load configuration error="invalid configuration: auth is enabled but STROMBOLI_JWT_SECRET is empty — generate one with: openssl rand -base64 32"
```

The process exits with code 1.

---

## Metrics on a separate localhost listener

**What changed.** `/metrics` is no longer mounted on the main API router. It's served from its own `http.Server` bound to `127.0.0.1:9090` by default. The main API port (`:8080` by default) no longer exposes Prometheus metrics at all.

**Why.** Co-locating `/metrics` on the public port meant a forgotten ingress rule could leak operational data (job counts, cardinality of session IDs, error rate signals) to the internet. A separate listener forces a deliberate decision to expose it.

**Migration.**

If your Prometheus runs on the same host:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'stromboli'
    static_configs:
      - targets: ['127.0.0.1:9090']  # was 'stromboli:8080'
```

If your Prometheus is in another pod / namespace, you have two options:

1. **Sidecar pattern (recommended).** Run a Prometheus scrape agent in the same pod; it talks to `localhost:9090` and forwards. The cluster-internal traffic crosses no untrusted boundaries.
2. **Bind on a private interface.** Override the address:
   ```bash
   export STROMBOLI_METRICS_ADDRESS="10.0.5.7:9090"  # the pod's private interface
   ```
   And add a NetworkPolicy that restricts ingress on `9090` to the Prometheus pod only.

!!! danger "Don't bind metrics to 0.0.0.0"
    The Kubernetes manifests we ship default to `127.0.0.1:9090` for a reason. If you override to `0.0.0.0:9090` to satisfy a Prometheus that doesn't support sidecars, pair it with a NetworkPolicy. An exposed metrics endpoint leaks more than people expect — request rates, session enumeration, error spikes correlating with deploy times.

**Verify.**

```bash
curl http://127.0.0.1:9090/metrics | head -3       # should return Prometheus text
curl http://localhost:8080/metrics                 # should 404
```

---

## Tracing TLS by default

**What changed.** `STROMBOLI_TRACING_INSECURE` flipped from `true` to `false`. The OpenTelemetry exporter now uses TLS by default and expects the OTLP collector to present a valid certificate.

**Why.** Plaintext OTLP leaks request URLs, session IDs, and internal routing details to anyone on the network path. The default belongs on the secure side.

**Migration.**

If your collector is on `localhost` or a trusted private network:

```bash
export STROMBOLI_TRACING_INSECURE=true
```

If your collector is reachable over the public network:

- Make sure it presents a TLS certificate (most managed collectors do)
- Make sure the certificate's subject matches the hostname in `STROMBOLI_TRACING_ENDPOINT`
- Use the system root CA store (Stromboli relies on it)

**Failure mode.** If you upgrade without flipping the env var and your collector is plaintext, traces silently stop arriving (the gRPC connect fails). You'll see periodic `ERROR` logs from the OTLP exporter; metrics-based "is tracing flowing" alerting will go red.

---

## Logs JSON by default

**What changed.** `STROMBOLI_LOG_FORMAT` defaults to `json`. Each log line is a single JSON object instead of human-readable text.

**Why.** Most log aggregators (Loki, CloudWatch Logs, Datadog) parse JSON natively. Plain text required fragile regex parsers that broke when message format changed. JSON is the right default for operators; humans get the opt-out.

**Migration.**

If you tail logs by hand and want them readable:

```bash
export STROMBOLI_LOG_FORMAT=text
```

If your aggregator was already auto-detecting / regex-parsing the old format, switch its parser to JSON. This is usually a one-line config change.

**Bonus: log level is now configurable.** `STROMBOLI_LOG_LEVEL` accepts `debug` / `info` / `warn` / `error`. Defaults to `info`.

```bash
export STROMBOLI_LOG_LEVEL=debug   # see every podman invocation, every config decision
```

---

## Windows binaries dropped

**What changed.** `v0.5.0-alpha` no longer publishes a `stromboli-windows-amd64.exe` artifact in the GitHub release. The matrix is now `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.

**Why.** Persistent agents (the marquee feature this release) use Unix-only syscalls — process groups via `SysProcAttr.Setpgid`, signal delivery via `syscall.Kill`. There's no clean Windows equivalent without job-object plumbing nobody asked for.

Stromboli is also fundamentally Linux-bound at the orchestration layer: it speaks to a Podman Unix socket. Even if the binary linked, you couldn't usefully run it as a Windows host.

**Migration.** Run Stromboli inside a Linux container (the published `ghcr.io/tomblancdev/stromboli` image), or via WSL2 on a Windows workstation.

---

## New env vars to be aware of

These are not breaking — defaults preserve existing behavior — but they unlock features you may want to enable.

| Env var | Default | Purpose |
|---|---|---|
| `STROMBOLI_WEBHOOK_SIGNING_SECRET` | (none) | Sign outgoing async-job webhooks with HMAC-SHA256 so receivers can verify authenticity. See [webhook security](../guides/webhook-security.md). |
| `STROMBOLI_RATE_LIMIT_TRUSTED_PROXIES` | (none) | CIDRs whose `X-Forwarded-For` header should be honored for rate-limit identity. Required if you're behind a reverse proxy. |
| `STROMBOLI_AUTH_BLACKLIST_BACKEND` | `memory` | `memory` (default) or `bolt` for token blacklist storage. `bolt` survives restarts. |
| `STROMBOLI_AUTH_BLACKLIST_BOLT_PATH` | `.stromboli/blacklist.db` | File path when `BACKEND=bolt`. |
| `STROMBOLI_AUTH_BLACKLIST_CLEANUP_INTERVAL` | `1h` | How often expired entries are reaped. |

For each, the [configuration reference](configuration.md) has the full table.

## Sanity check after upgrading

After restarting the server, hit a few endpoints to confirm:

```bash
# Health (should always work, no auth required)
curl http://localhost:8080/health

# Get a token (proves auth is wired correctly)
curl -X POST http://localhost:8080/auth/token \
  -H "Authorization: Bearer $STROMBOLI_API_TOKEN" \
  -d '{"client_id": "smoke-test"}'

# Metrics (on the new port)
curl http://127.0.0.1:9090/metrics | head -3

# Persistent agent (new in 0.5.0)
curl -X POST http://localhost:8080/agents \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "hello", "claude": {"model": "sonnet"}}'
```

If all four return what you expect, you're good. If anything's off, the [troubleshooting guide](../self-hosting/troubleshooting.md) has common upgrade snags.

## See also

- [Full v0.5.0-alpha changelog](../changelog.md#050-alpha---2026-05-01)
- [Configuration reference](configuration.md) — every env var, including the new ones
- [Production hardening checklist](../security/production.md) — has been updated for v0.5.0
- [Persistent agents guide](../guides/persistent-agents.md) — the headline new feature
- [Webhook security](../guides/webhook-security.md) — how to verify signed callbacks
