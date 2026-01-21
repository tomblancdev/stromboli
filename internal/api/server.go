package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP API server
type Server struct {
	router *gin.Engine
}

// NewServer creates a new API server
func NewServer() *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware())

	s := &Server{router: router}
	s.setupRoutes()

	return s
}

// Run starts the server on the given address
func (s *Server) Run(addr string) error {
	slog.Info("API server listening", "addr", addr)
	return s.router.Run(addr)
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health endpoints
	s.router.GET("/health", s.healthCheck)

	// Claude credentials status (CLI handles setup)
	s.router.GET("/claude/status", s.claudeStatus)
}

// loggerMiddleware logs HTTP requests
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
		)
	}
}

// healthCheck returns API health status
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"name":   "stromboli",
	})
}

// claudeStatus checks if Claude credentials are configured
func (s *Server) claudeStatus(c *gin.Context) {
	// TODO: Check if claude-auth volume exists and has credentials
	c.JSON(http.StatusOK, gin.H{
		"configured": false,
		"message":    "Run 'stromboli claude init' to configure",
	})
}
