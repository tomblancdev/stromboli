package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"stromboli/internal/job"
)

// JobResult represents the result of a job to be sent to webhook
type JobResult struct {
	JobID     string     `json:"job_id"`
	Status    string     `json:"status"`
	Output    string     `json:"output,omitempty"`
	Error     string     `json:"error,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
	Usage     *job.Usage `json:"usage,omitempty"`
}

// Header names for webhook signature verification. Receivers should compute
// HMAC-SHA256 over `timestamp + "." + body` using the shared secret and
// constant-time-compare against the signature header. The timestamp guards
// against replay attacks.
const (
	HeaderSignature = "X-Stromboli-Signature"
	HeaderTimestamp = "X-Stromboli-Timestamp"
	signaturePrefix = "sha256="
)

// Notifier sends webhook notifications.
//
// When constructed with NewSignedNotifier, every outgoing POST carries an
// X-Stromboli-Signature header so receivers can verify the request actually
// came from Stromboli. Without signing, receivers have no way to distinguish
// genuine notifications from forged ones — so production deployments should
// always set webhook.signing_secret.
type Notifier struct {
	client *http.Client
	secret []byte // nil = unsigned (legacy / dev)
}

// NewNotifier creates an unsigned notifier. Prefer NewSignedNotifier in
// production — unsigned webhooks can be forged by anyone who can reach the
// receiver.
func NewNotifier() *Notifier {
	return &Notifier{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// NewSignedNotifier creates a notifier that HMAC-SHA256-signs every payload
// with the provided secret. An empty secret is equivalent to NewNotifier
// (unsigned) — the caller is responsible for not promoting an empty secret
// to production.
func NewSignedNotifier(secret string) *Notifier {
	n := NewNotifier()
	if secret != "" {
		n.secret = []byte(secret)
	}
	return n
}

// Notify sends a webhook notification with retry logic.
func (n *Notifier) Notify(url string, payload JobResult) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Capture a single timestamp per Notify call so a retry signs the same
	// (timestamp, body) pair; the receiver's replay window evaluates the
	// initial send time, not the retry time.
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := ""
	if n.secret != nil {
		signature = sign(n.secret, timestamp, data)
	}

	// Try twice: initial attempt + one retry
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(data))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if signature != "" {
			req.Header.Set(HeaderTimestamp, timestamp)
			req.Header.Set(HeaderSignature, signaturePrefix+signature)
		}

		resp, err := n.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send webhook: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return lastErr
}

// sign computes HMAC-SHA256 over `timestamp + "." + body` and returns the
// hex-encoded digest. The timestamp prefix prevents an attacker from
// replaying an old (still-valid) signature against a different request body.
func sign(secret []byte, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks an incoming webhook signature. Receivers can use this to
// confirm a payload originated from Stromboli without rolling their own HMAC
// implementation. Returns nil on a valid signature.
//
// `maxAge` rejects timestamps older than the given duration to limit replay
// windows; pass 0 to skip the freshness check (not recommended for prod).
func Verify(secret []byte, timestamp, signatureHeader string, body []byte, maxAge time.Duration) error {
	if len(secret) == 0 {
		return fmt.Errorf("webhook secret is empty")
	}
	if timestamp == "" || signatureHeader == "" {
		return fmt.Errorf("missing signature headers")
	}

	tsInt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if maxAge > 0 {
		if age := time.Since(time.Unix(tsInt, 0)); age > maxAge || age < -maxAge {
			return fmt.Errorf("timestamp outside freshness window (%s)", age)
		}
	}

	expected := sign(secret, timestamp, body)
	provided := signatureHeader
	if len(provided) > len(signaturePrefix) && provided[:len(signaturePrefix)] == signaturePrefix {
		provided = provided[len(signaturePrefix):]
	}

	// Constant-time compare so an attacker can't time the comparison to
	// recover the signature byte-by-byte.
	if !hmac.Equal([]byte(expected), []byte(provided)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
