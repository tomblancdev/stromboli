package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
	Enabled         bool          // Whether rate limiting is enabled
	Rate            int           // Requests per period
	Period          time.Duration // Time period (e.g., time.Second)
	Burst           int           // Maximum burst size
	CleanupInterval time.Duration // How often to clean up stale IP entries (default: 10 minutes)
	// TrustedProxies is the parsed CIDR list whose X-Forwarded-For /
	// X-Real-IP headers will be honored. When empty, forwarding headers
	// are ignored and the immediate peer's address is always used —
	// without this allowlist, anyone on the internet could spoof their
	// rate-limit identity simply by setting X-Forwarded-For.
	TrustedProxies []string
}

// ipLimiterEntry holds a rate limiter and its last access time
type ipLimiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// ipRateLimiter tracks rate limiters per IP address
type ipRateLimiter struct {
	limiters        map[string]*ipLimiterEntry
	mu              sync.RWMutex
	rate            rate.Limit
	burst           int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// newIPRateLimiter creates a new IP-based rate limiter
func newIPRateLimiter(r rate.Limit, b int, cleanupInterval time.Duration) *ipRateLimiter {
	i := &ipRateLimiter{
		limiters:        make(map[string]*ipLimiterEntry),
		rate:            r,
		burst:           b,
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go i.cleanupStaleEntries()

	return i
}

// getLimiter returns the rate limiter for the given IP address
func (i *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	entry, exists := i.limiters[ip]
	if !exists {
		entry = &ipLimiterEntry{
			limiter:    rate.NewLimiter(i.rate, i.burst),
			lastAccess: time.Now(),
		}
		i.limiters[ip] = entry
	} else {
		entry.lastAccess = time.Now()
	}

	return entry.limiter
}

// cleanupStaleEntries periodically removes entries not accessed in cleanupInterval
func (i *ipRateLimiter) cleanupStaleEntries() {
	ticker := time.NewTicker(i.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			i.mu.Lock()
			now := time.Now()
			for ip, entry := range i.limiters {
				if now.Sub(entry.lastAccess) > i.cleanupInterval {
					delete(i.limiters, ip)
				}
			}
			i.mu.Unlock()
		case <-i.stopCleanup:
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (i *ipRateLimiter) Stop() {
	close(i.stopCleanup)
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(config RateLimitConfig) gin.HandlerFunc {
	// If rate limiting is disabled, return a no-op middleware
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Set default cleanup interval if not configured
	cleanupInterval := config.CleanupInterval
	if cleanupInterval == 0 {
		cleanupInterval = 10 * time.Minute
	}

	// Pre-parse trusted-proxy CIDRs once so the per-request path is just a
	// linear scan over already-parsed nets. Malformed entries are dropped
	// silently here — config validation surfaces them at startup.
	trustedNets := parseTrustedProxies(config.TrustedProxies)

	// Calculate rate per second
	ratePerSecond := rate.Limit(float64(config.Rate) / config.Period.Seconds())
	limiter := newIPRateLimiter(ratePerSecond, config.Burst, cleanupInterval)

	return func(c *gin.Context) {
		// Extract IP from request
		ip := getClientIP(c, trustedNets)

		// Get limiter for this IP
		l := limiter.getLimiter(ip)

		// Check if request is allowed
		if !l.Allow() {
			// Set rate limit headers
			setRateLimitHeaders(c, config.Rate, 0, time.Now().Add(config.Period))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		// Calculate remaining requests as the number of tokens currently in
		// the bucket, floored at zero. The previous formula `Burst()-Tokens()`
		// reported the *consumed* count, which is the inverse of what
		// X-RateLimit-Remaining is supposed to convey to the client.
		remaining := int(l.Tokens())
		if remaining < 0 {
			remaining = 0
		}

		// Set rate limit headers
		setRateLimitHeaders(c, config.Rate, remaining, time.Now().Add(config.Period))

		c.Next()
	}
}

// getClientIP extracts the client IP from the request.
//
// When trustedNets is non-empty AND the immediate peer (RemoteAddr) is inside
// one of those CIDRs, we honor X-Forwarded-For / X-Real-IP. Otherwise we
// always use RemoteAddr — this prevents an internet-facing client from
// spoofing its rate-limit identity by setting X-Forwarded-For: <victim>.
func getClientIP(c *gin.Context, trustedNets []*net.IPNet) string {
	peer := remoteAddrIP(c.Request.RemoteAddr)
	if !peerIsTrusted(peer, trustedNets) {
		return peer
	}

	// Behind a trusted proxy: forwarding headers are reliable.
	if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For is a comma-separated chain (`client, proxy1, proxy2`).
		// The leftmost entry is the original client; later entries are the
		// proxy chain we already trust.
		first := forwarded
		if idx := strings.IndexByte(forwarded, ','); idx >= 0 {
			first = strings.TrimSpace(forwarded[:idx])
		}
		// Strip a possible :port suffix.
		if host, _, err := net.SplitHostPort(first); err == nil {
			return host
		}
		return first
	}
	if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
		return realIP
	}
	return peer
}

// remoteAddrIP strips the port from a net.Conn-formatted RemoteAddr. Falls
// back to the raw value when SplitHostPort can't parse it (unix sockets,
// in-process httptest.NewRequest etc.).
func remoteAddrIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// parseTrustedProxies parses the operator-supplied CIDR list. Empty / invalid
// entries are dropped — the resulting slice is what runtime path matches
// against, so we keep it safe-by-construction.
func parseTrustedProxies(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		// Accept a bare IP by promoting it to /32 (v4) or /128 (v6).
		if !strings.Contains(raw, "/") {
			if ip := net.ParseIP(raw); ip != nil {
				if ip.To4() != nil {
					raw += "/32"
				} else {
					raw += "/128"
				}
			}
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			continue
		}
		out = append(out, network)
	}
	return out
}

func peerIsTrusted(ip string, trustedNets []*net.IPNet) bool {
	if len(trustedNets) == 0 || ip == "" {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range trustedNets {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

// setRateLimitHeaders sets rate limit response headers
func setRateLimitHeaders(c *gin.Context, limit int, remaining int, reset time.Time) {
	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", reset.Unix()))
}
