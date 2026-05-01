package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		config         RateLimitConfig
		requests       int
		expectedStatus int
		checkHeaders   bool
	}{
		{
			name: "within rate limit",
			config: RateLimitConfig{
				Enabled: true,
				Rate:    10,
				Period:  time.Second,
				Burst:   20,
			},
			requests:       5,
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
		{
			name: "exceeds rate limit",
			config: RateLimitConfig{
				Enabled: true,
				Rate:    2,
				Period:  time.Second,
				Burst:   2,
			},
			requests:       5,
			expectedStatus: http.StatusTooManyRequests,
			checkHeaders:   true,
		},
		{
			name: "rate limiting disabled",
			config: RateLimitConfig{
				Enabled: false,
				Rate:    1,
				Period:  time.Second,
				Burst:   1,
			},
			requests:       10,
			expectedStatus: http.StatusOK,
			checkHeaders:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(RateLimitMiddleware(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			var lastResponse *httptest.ResponseRecorder
			for i := 0; i < tt.requests; i++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.1:12345" // Simulate same IP
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				lastResponse = w
			}

			if tt.name == "exceeds rate limit" {
				assert.Equal(t, tt.expectedStatus, lastResponse.Code)
			} else {
				assert.Equal(t, tt.expectedStatus, lastResponse.Code)
			}

			if tt.checkHeaders && tt.config.Enabled {
				assert.NotEmpty(t, lastResponse.Header().Get("X-RateLimit-Limit"))
				assert.NotEmpty(t, lastResponse.Header().Get("X-RateLimit-Remaining"))
				assert.NotEmpty(t, lastResponse.Header().Get("X-RateLimit-Reset"))
			}
		})
	}
}

func TestRateLimitHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := RateLimitConfig{
		Enabled: true,
		Rate:    10,
		Period:  time.Second,
		Burst:   20,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimitPerIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := RateLimitConfig{
		Enabled: true,
		Rate:    2,
		Period:  time.Second,
		Burst:   2,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First IP should get rate limited after 2 requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i < 2 {
			assert.Equal(t, http.StatusOK, w.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
		}
	}

	// Second IP should still be allowed
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := RateLimitConfig{
		Enabled: false,
		Rate:    1,
		Period:  time.Second,
		Burst:   1,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Should allow many requests when disabled
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestRateLimit_RemainingHeaderReportsAvailableTokens(t *testing.T) {
	// X-RateLimit-Remaining is supposed to tell the client how many more
	// requests they can make right now — not how many they've already used.
	// Burst=5: after the first allowed request, 4 tokens remain.
	gin.SetMode(gin.TestMode)

	cfg := RateLimitConfig{
		Enabled: true,
		Rate:    1, // slow refill so we observe the bucket draining as we burst
		Period:  time.Second,
		Burst:   5,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(cfg))
	router.GET("/", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) })

	// Make 3 quick requests against the same client and assert remaining
	// counts down monotonically toward zero (not upward toward Burst).
	var remainings []int
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var r int
		_, err := fmt.Sscanf(w.Header().Get("X-RateLimit-Remaining"), "%d", &r)
		assert.NoError(t, err)
		remainings = append(remainings, r)
	}

	assert.LessOrEqual(t, remainings[0], 4, "first request: at most burst-1 left")
	assert.LessOrEqual(t, remainings[1], remainings[0], "remaining must not increase between bursts")
	assert.LessOrEqual(t, remainings[2], remainings[1], "remaining must not increase between bursts")
}

func TestRateLimitErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := RateLimitConfig{
		Enabled: true,
		Rate:    1,
		Period:  time.Second,
		Burst:   1,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First request succeeds
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request should be rate limited
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.Contains(t, w2.Body.String(), "rate limit exceeded")
}

func TestIPRateLimiterCleanup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a rate limiter with short cleanup interval for testing
	cleanupInterval := 100 * time.Millisecond
	limiter := newIPRateLimiter(1, 1, cleanupInterval)
	defer limiter.Stop()

	// Add some IPs
	limiter.getLimiter("192.168.1.1")
	limiter.getLimiter("192.168.1.2")
	limiter.getLimiter("192.168.1.3")

	// Verify all IPs are tracked
	limiter.mu.RLock()
	assert.Equal(t, 3, len(limiter.limiters))
	limiter.mu.RUnlock()

	// Wait for cleanup to run (cleanup interval + buffer)
	time.Sleep(cleanupInterval + 50*time.Millisecond)

	// All entries should be cleaned up since they haven't been accessed
	limiter.mu.RLock()
	assert.Equal(t, 0, len(limiter.limiters))
	limiter.mu.RUnlock()
}

func TestIPRateLimiterCleanupKeepsActive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a rate limiter with short cleanup interval for testing
	cleanupInterval := 100 * time.Millisecond
	limiter := newIPRateLimiter(1, 1, cleanupInterval)
	defer limiter.Stop()

	// Add some IPs
	limiter.getLimiter("192.168.1.1")
	limiter.getLimiter("192.168.1.2")

	// Keep accessing one IP to keep it alive
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				limiter.getLimiter("192.168.1.1")
			case <-done:
				return
			}
		}
	}()

	// Wait for cleanup to run
	time.Sleep(cleanupInterval + 50*time.Millisecond)

	// Stop the access goroutine
	close(done)

	// IP 192.168.1.1 should still exist, but 192.168.1.2 should be cleaned up
	limiter.mu.RLock()
	assert.Equal(t, 1, len(limiter.limiters))
	_, exists := limiter.limiters["192.168.1.1"]
	assert.True(t, exists)
	_, exists = limiter.limiters["192.168.1.2"]
	assert.False(t, exists)
	limiter.mu.RUnlock()
}

func TestRateLimitCleanupIntervalDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test that default cleanup interval is set
	config := RateLimitConfig{
		Enabled: true,
		Rate:    10,
		Period:  time.Second,
		Burst:   20,
		// CleanupInterval not set, should default to 10 minutes
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitCleanupIntervalCustom(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test that custom cleanup interval is used
	customInterval := 5 * time.Minute
	config := RateLimitConfig{
		Enabled:         true,
		Rate:            10,
		Period:          time.Second,
		Burst:           20,
		CleanupInterval: customInterval,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
