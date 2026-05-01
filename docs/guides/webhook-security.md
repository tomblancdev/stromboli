# Webhook Security

Stromboli can POST to a callback URL when an async job completes (the `webhook_url` field on `/run/async`). By default those callbacks are unsigned — your receiver has no way to distinguish a genuine Stromboli notification from a forged one. This guide covers the signing flow and the related trusted-proxy setting that locks down rate-limit identity.

## Why sign

If your receiver acts on the callback (closes a ticket, deploys code, posts to Slack), an unsigned callback is a side-channel for anyone who can guess the URL. They send a forged "completed" event, you trigger a real action.

Signing turns the callback into something only a holder of the shared secret could produce. The receiver verifies before acting; forged calls get rejected.

## Server-side setup

Set a secret on the Stromboli process (32+ random bytes is fine):

```bash
export STROMBOLI_WEBHOOK_SIGNING_SECRET="$(openssl rand -base64 32)"
./bin/stromboli
```

On startup the server logs:

```
INFO Webhook signing enabled (HMAC-SHA256)
```

If you forget, you instead get a loud `WARN` that callbacks will ship unsigned. That's deliberate — silent unsigned sends are how production deployments end up with a forgery vulnerability nobody noticed.

Once enabled, every async job webhook carries two extra headers:

```
X-Stromboli-Timestamp: 1735726800
X-Stromboli-Signature: sha256=4f8c2a7b...
```

The signature is HMAC-SHA256 over `timestamp + "." + body`, hex-encoded.

## Receiver-side verification

### Go (using the helper Stromboli ships)

Stromboli's `webhook` package exports a `Verify` helper so receivers don't have to roll their own HMAC code:

```go
import "stromboli/internal/webhook"

func handle(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    err := webhook.Verify(
        []byte(os.Getenv("STROMBOLI_WEBHOOK_SIGNING_SECRET")),
        r.Header.Get(webhook.HeaderTimestamp),
        r.Header.Get(webhook.HeaderSignature),
        body,
        5*time.Minute, // freshness window
    )
    if err != nil {
        http.Error(w, "invalid signature", http.StatusUnauthorized)
        return
    }
    // ...act on the verified body...
}
```

The freshness window rejects timestamps older than the bound — a leaked signature can't be replayed indefinitely. 5 minutes is a reasonable starting point; tighten if your receiver is on a tightly-clocked network.

### Python

```python
import hmac, hashlib, time
from flask import Flask, request, abort

SECRET = os.environ["STROMBOLI_WEBHOOK_SIGNING_SECRET"].encode()
MAX_AGE = 300  # 5 min

@app.post("/hook")
def hook():
    ts = request.headers.get("X-Stromboli-Timestamp", "")
    sig = request.headers.get("X-Stromboli-Signature", "")
    body = request.get_data()

    if not ts or not sig:
        abort(401, "missing signature headers")

    try:
        ts_int = int(ts)
    except ValueError:
        abort(401, "invalid timestamp")
    if abs(time.time() - ts_int) > MAX_AGE:
        abort(401, "timestamp outside freshness window")

    expected = hmac.new(SECRET, f"{ts}.".encode() + body, hashlib.sha256).hexdigest()
    provided = sig.removeprefix("sha256=")
    if not hmac.compare_digest(expected, provided):
        abort(401, "signature mismatch")

    # ...act on the verified body...
    return "", 204
```

`hmac.compare_digest` is critical — a normal `==` leaks timing info that lets attackers recover the signature byte-by-byte.

### Node

```js
import crypto from "node:crypto";

function verify(req, secret, maxAgeSec = 300) {
  const ts = req.headers["x-stromboli-timestamp"];
  const sig = req.headers["x-stromboli-signature"];
  if (!ts || !sig) throw new Error("missing signature headers");

  const tsInt = parseInt(ts, 10);
  if (Math.abs(Date.now() / 1000 - tsInt) > maxAgeSec) {
    throw new Error("timestamp outside freshness window");
  }

  const expected = crypto
    .createHmac("sha256", secret)
    .update(`${ts}.`)
    .update(req.rawBody) // requires raw body — see your framework's docs
    .digest("hex");

  const provided = sig.replace(/^sha256=/, "");
  const a = Buffer.from(expected, "hex");
  const b = Buffer.from(provided, "hex");
  if (a.length !== b.length || !crypto.timingSafeEqual(a, b)) {
    throw new Error("signature mismatch");
  }
}
```

The receiver MUST hash the **raw bytes** of the request body, not a JSON-reparse. If your framework auto-parses JSON, configure it to keep the raw payload around (express: `express.raw({type: 'application/json'})` then re-parse later, fastify: `rawBody: true`, etc.).

## Retry semantics

Stromboli retries each webhook once after 100 ms. The retry **reuses the same timestamp and signature** as the original send. Your receiver's freshness check should evaluate the original send time, not the retry time — clients that recompute timestamps per retry will fail when the original had a few seconds of clock skew.

If your receiver returns a 2xx response, the webhook is considered delivered. Anything else triggers the one retry, then Stromboli logs and gives up. There is no exponential backoff or DLQ; if reliable delivery matters, hold the receiver behind a queue you control.

## Rotating the secret

There is no built-in rotation flow yet. To rotate without dropping in-flight callbacks:

1. Configure your receiver to accept signatures from **either** the old or new secret for an overlap window
2. Update `STROMBOLI_WEBHOOK_SIGNING_SECRET` and restart Stromboli
3. After the freshness window has passed (5 min is enough), drop the old secret on the receiver

## Trusted proxies for rate limiting

Related but distinct: rate limiting decides who's making a request, which can be spoofed via `X-Forwarded-For` if the server trusts that header blindly. Stromboli ignores forwarding headers by default and only honours them when the immediate peer is in a configured allowlist:

```bash
# Comma-separated CIDRs (or bare IPs) of proxies you trust.
export STROMBOLI_RATE_LIMIT_TRUSTED_PROXIES="10.0.0.0/8,172.16.0.0/12"
```

When set, requests arriving from those CIDRs may include `X-Forwarded-For`; the leftmost (original-client) entry becomes the rate-limit bucket key. From any other source, forwarding headers are ignored and the immediate peer is used.

Rule of thumb:

- **Direct exposure** (no proxy) → leave empty. Don't honor headers nobody is supposed to send.
- **Behind a single trusted reverse proxy** → set its private CIDR.
- **Behind a CDN with rotating egress IPs** → set the CDN's documented egress range. Don't list `0.0.0.0/0` — that defeats the purpose.

If you skip this and run behind a proxy, every authenticated client appears as the same IP (the proxy's), and your per-IP rate limits become per-cluster rate limits. If you skip it and run with direct exposure, anyone can spoof their bucket. Either way: configure the allowlist that matches your topology.

## See also

- [API endpoints reference](../api/endpoints.md) — `webhook_url` on `/run/async`
- [Configuration](../getting-started/configuration.md) — both env vars in the table
- [Production hardening](../security/production.md) — checklist that includes signing
