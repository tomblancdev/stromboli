package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tomblanc/stromboli/internal/claude"
	"github.com/tomblanc/stromboli/internal/runner"
)

// Server represents the HTTP API server
type Server struct {
	router      *gin.Engine
	runner      runner.Runner
	claudeClient *claude.Client
}

// NewServer creates a new API server
func NewServer(r runner.Runner, claudeClient *claude.Client) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware())

	s := &Server{
		router:       router,
		runner:       r,
		claudeClient: claudeClient,
	}
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
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/claude/status", s.claudeStatus)
	s.router.POST("/run", s.runClaude)
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
	configured := s.claudeClient.IsConfigured()
	if configured {
		c.JSON(http.StatusOK, gin.H{
			"configured": true,
			"message":    "Claude is configured",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"configured": false,
		"message":    "Run 'make claude-setup' to configure",
	})
}

// runClaude executes Claude in an isolated container
func (s *Server) runClaude(c *gin.Context) {
	var req RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, RunResponse{
			Status: "error",
			Error:  "Invalid request: " + err.Error(),
		})
		return
	}

	// Check if configured
	if !s.claudeClient.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, RunResponse{
			Status: "error",
			Error:  "Claude not configured. Run 'make claude-setup' first",
		})
		return
	}

	// Run Claude
	result, err := s.runner.Run(c.Request.Context(), runner.Request{
		Prompt:    req.Prompt,
		Workspace: req.Workspace,
		Model:     req.Model,
		SessionID: req.SessionID,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, RunResponse{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, RunResponse{
		ID:        result.ID,
		Status:    "completed",
		Output:    result.Output,
		SessionID: result.SessionID,
	})
}
