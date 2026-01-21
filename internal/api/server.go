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

	// Agent endpoints
	agents := s.router.Group("/agents")
	{
		agents.POST("", s.spawnAgent)
		agents.GET("/:id/logs", s.getAgentLogs)
	}

	// Auth status (CLI handles setup)
	s.router.GET("/auth/status", s.authStatus)
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

// authStatus checks if Claude auth is configured
func (s *Server) authStatus(c *gin.Context) {
	// TODO: Check if claude-auth volume exists and has credentials
	c.JSON(http.StatusOK, gin.H{
		"configured": false,
		"message":    "Run 'stromboli auth init' to configure",
	})
}

// SpawnRequest represents a request to spawn a new agent
type SpawnRequest struct {
	Task   string       `json:"task" binding:"required"`
	Mounts []MountSpec  `json:"mounts,omitempty"`
}

// MountSpec represents a mount configuration
type MountSpec struct {
	Source string `json:"source" binding:"required"`
	Target string `json:"target" binding:"required"`
	Mode   string `json:"mode,omitempty"` // "ro" or "rw", default "ro"
}

// spawnAgent creates and starts a new Claude agent container
func (s *Server) spawnAgent(c *gin.Context) {
	var req SpawnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement Podman container spawning
	slog.Info("Spawn request received", "task", req.Task, "mounts", len(req.Mounts))

	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Container spawning not yet implemented",
	})
}

// getAgentLogs retrieves logs from an agent container
func (s *Server) getAgentLogs(c *gin.Context) {
	agentID := c.Param("id")

	// TODO: Implement log retrieval from Podman
	slog.Info("Logs request", "agent_id", agentID)

	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Log retrieval not yet implemented",
	})
}
