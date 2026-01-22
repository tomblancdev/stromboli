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
	Enabled bool          // Whether rate limiting is enabled
	Rate    int           // Requests per period
	Period  time.Duration // Time period (e.g., time.Second)
	Burst   int           // Maximum burst size
}

// ipRateLimiter tracks rate limiters per IP address
type ipRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// newIPRateLimiter creates a new IP-based rate limiter
func newIPRateLimiter(r rate.Limit, b int) *ipRateLimiter {
	return &ipRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    b,
	}
}

// getLimiter returns the rate limiter for the given IP address
func (i *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(i.rate, i.burst)
		i.limiters[ip] = limiter
	}

	return limiter
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(config RateLimitConfig) gin.HandlerFunc {
	// If rate limiting is disabled, return a no-op middleware
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Calculate rate per second
	ratePerSecond := rate.Limit(float64(config.Rate) / config.Period.Seconds())
	limiter := newIPRateLimiter(ratePerSecond, config.Burst)

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
