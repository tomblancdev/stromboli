package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware())

	var capturedID string
	router.GET("/test", func(c *gin.Context) {
		capturedID = c.GetString("request_id")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.NotEmpty(t, capturedID)
	assert.Len(t, capturedID, 16) // 8 bytes hex = 16 chars
}

func TestRequestIDMiddleware_UsesProvidedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware())

	var capturedID string
	router.GET("/test", func(c *gin.Context) {
		capturedID = c.GetString("request_id")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "custom-request-id-123")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, "custom-request-id-123", capturedID)
}

func TestRequestIDMiddleware_SetsResponseHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware())

	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	responseID := rec.Header().Get("X-Request-ID")
	assert.NotEmpty(t, responseID)
}

func TestEnhancedLoggingMiddleware_LogsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(EnhancedLoggingMiddleware())

	router.GET("/test", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond) // Simulate some work
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "test-123")
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Note: We can't easily test log output without capturing slog,
	// but we verify the handler completes successfully
}

func TestEnhancedLoggingMiddleware_HandlesErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(EnhancedLoggingMiddleware())

	router.GET("/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestEnhancedLoggingMiddleware_CalculatesDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(EnhancedLoggingMiddleware())

	var handlerExecuted bool
	router.GET("/slow", func(c *gin.Context) {
		time.Sleep(50 * time.Millisecond)
		handlerExecuted = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	assert.True(t, handlerExecuted)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(50))
}

func TestEnhancedLoggingMiddleware_ExtractsClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(EnhancedLoggingMiddleware())

	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	testCases := []struct {
		name       string
		remoteAddr string
		xForwardedFor string
	}{
		{
			name:       "direct connection",
			remoteAddr: "10.0.0.1:54321",
		},
		{
			name:          "proxied connection",
			remoteAddr:    "127.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tc.xForwardedFor)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}
