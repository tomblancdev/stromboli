package api

import (
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
