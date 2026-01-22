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
	router       *gin.Engine
	runner       runner.Runner
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

	// Session management
	s.router.GET("/sessions", s.listSessions)
	s.router.DELETE("/sessions/:id", s.destroySession)
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
// @Summary Health check
// @Description Returns the health status of the API
// @Tags system
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status: "ok",
		Name:   "stromboli",
	})
}

// claudeStatus checks if Claude credentials are configured
// @Summary Claude status
// @Description Checks if Claude credentials are configured
// @Tags system
// @Produce json
// @Success 200 {object} ClaudeStatusResponse
// @Router /claude/status [get]
func (s *Server) claudeStatus(c *gin.Context) {
	configured := s.claudeClient.IsConfigured()
	if configured {
		c.JSON(http.StatusOK, ClaudeStatusResponse{
			Configured: true,
			Message:    "Claude is configured",
		})
		return
	}
	c.JSON(http.StatusOK, ClaudeStatusResponse{
		Configured: false,
		Message:    "Run 'make claude-setup' to configure",
	})
}

// runClaude executes Claude in an isolated container
// @Summary Run Claude
// @Description Executes Claude Code in an isolated Podman container
// @Tags execution
// @Accept json
// @Produce json
// @Param request body RunRequest true "Run request"
// @Success 200 {object} RunResponse
// @Failure 400 {object} RunResponse "Invalid request"
// @Failure 500 {object} RunResponse "Execution failed"
// @Failure 503 {object} RunResponse "Claude not configured"
// @Router /run [post]
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

	// Map API types to runner types
	runnerReq := runner.Request{
		Prompt:    req.Prompt,
		Workspace: req.Workspace,
		Claude:    mapClaudeOptions(req.Claude),
		Podman:    mapPodmanOptions(req.Podman),
	}

	// Run Claude
	result, err := s.runner.Run(c.Request.Context(), runnerReq)
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

// mapClaudeOptions converts API ClaudeOptions to runner ClaudeOptions
func mapClaudeOptions(opts ClaudeOptions) runner.ClaudeOptions {
	return runner.ClaudeOptions{
		// Session management
		SessionID:     opts.SessionID,
		Resume:        opts.Resume,
		Continue:      opts.Continue,
		ForkSession:   opts.ForkSession,
		NoPersistence: opts.NoPersistence,

		// Model
		Model:         opts.Model,
		FallbackModel: opts.FallbackModel,

		// System prompt
		SystemPrompt:       opts.SystemPrompt,
		AppendSystemPrompt: opts.AppendSystemPrompt,

		// Tools
		Tools:           opts.Tools,
		AllowedTools:    opts.AllowedTools,
		DisallowedTools: opts.DisallowedTools,

		// Permissions
		PermissionMode:                  opts.PermissionMode,
		DangerouslySkipPermissions:      opts.DangerouslySkipPermissions,
		AllowDangerouslySkipPermissions: opts.AllowDangerouslySkipPermissions,

		// I/O format
		InputFormat:            opts.InputFormat,
		OutputFormat:           opts.OutputFormat,
		IncludePartialMessages: opts.IncludePartialMessages,
		ReplayUserMessages:     opts.ReplayUserMessages,

		// Structured output
		JSONSchema: opts.JSONSchema,

		// Budget
		MaxBudgetUSD: opts.MaxBudgetUSD,

		// MCP
		MCPConfigs:      opts.MCPConfigs,
		StrictMCPConfig: opts.StrictMCPConfig,

		// Agents
		Agent:  opts.Agent,
		Agents: opts.Agents,

		// Resources
		AddDirs:    opts.AddDirs,
		PluginDirs: opts.PluginDirs,
		Files:      opts.Files,

		// Settings
		Settings:       opts.Settings,
		SettingSources: opts.SettingSources,

		// Beta
		Betas: opts.Betas,

		// Misc
		Verbose:              opts.Verbose,
		Debug:                opts.Debug,
		DisableSlashCommands: opts.DisableSlashCommands,
	}
}

// mapPodmanOptions converts API PodmanOptions to runner PodmanOptions
func mapPodmanOptions(opts PodmanOptions) runner.PodmanOptions {
	return runner.PodmanOptions{
		Volumes: opts.Volumes,
	}
}

// listSessions returns all existing sessions
// @Summary List sessions
// @Description Returns all existing session IDs
// @Tags sessions
// @Produce json
// @Success 200 {object} SessionListResponse
// @Failure 500 {object} SessionListResponse
// @Router /sessions [get]
func (s *Server) listSessions(c *gin.Context) {
	sessions, err := s.runner.ListSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, SessionListResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SessionListResponse{
		Sessions: sessions,
	})
}

// destroySession removes a session and all its data
// @Summary Destroy session
// @Description Removes a session and all its stored data
// @Tags sessions
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} SessionDestroyResponse
// @Failure 400 {object} SessionDestroyResponse
// @Failure 404 {object} SessionDestroyResponse
// @Router /sessions/{id} [delete]
func (s *Server) destroySession(c *gin.Context) {
	sessionID := c.Param("id")

	if err := s.runner.DestroySession(sessionID); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "session ID is required" || err.Error() == "invalid session ID" {
			status = http.StatusBadRequest
		} else if err.Error() == "session not found: "+sessionID {
			status = http.StatusNotFound
		}

		c.JSON(status, SessionDestroyResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SessionDestroyResponse{
		Success:   true,
		SessionID: sessionID,
	})
}
