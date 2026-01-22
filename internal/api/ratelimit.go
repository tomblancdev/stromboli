package api

import (
	"fmt"
	"net"
	"net/http"
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

	// Calculate rate per second
	ratePerSecond := rate.Limit(float64(config.Rate) / config.Period.Seconds())
	limiter := newIPRateLimiter(ratePerSecond, config.Burst, cleanupInterval)

	return func(c *gin.Context) {
		// Extract IP from request
		ip := getClientIP(c)

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

		// Calculate remaining requests
		remaining := l.Burst() - int(l.Tokens())
		if remaining < 0 {
			remaining = 0
		}

		// Set rate limit headers
		setRateLimitHeaders(c, config.Rate, remaining, time.Now().Add(config.Period))

		c.Next()
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(c *gin.Context) string {
	// Try to get IP from X-Forwarded-For header
	forwarded := c.GetHeader("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP in the list
		if ip, _, err := net.SplitHostPort(forwarded); err == nil {
			return ip
		}
		return forwarded
	}

	// Try to get IP from X-Real-IP header
	realIP := c.GetHeader("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return ip
}

// setRateLimitHeaders sets rate limit response headers
func setRateLimitHeaders(c *gin.Context, limit int, remaining int, reset time.Time) {
	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", reset.Unix()))
}
