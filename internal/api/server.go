package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"stromboli/internal/auth"
	"stromboli/internal/claude"
	strerrors "stromboli/internal/errors"
	"stromboli/internal/images"
	"stromboli/internal/job"
	"stromboli/internal/runner"
	"stromboli/internal/secrets"
	"stromboli/internal/version"
)

// Server represents the HTTP API server
type Server struct {
	router          *gin.Engine
	runner          runner.Runner
	claudeClient    *claude.Client
	authConfig      auth.Config
	rateLimitConfig RateLimitConfig
	jobMgr          *job.Manager
	healthChecker   *HealthChecker
	blacklist       *auth.TokenBlacklist
	tracingEnabled  bool
	historyHandler  *SessionHistoryHandler
	secretsHandler  *SecretsHandler
	imagesHandler   *ImagesHandler
}

// NewServer creates a new API server
func NewServer(r runner.Runner, claudeClient *claude.Client, authConfig auth.Config, rateLimitConfig RateLimitConfig, jobMgr *job.Manager, healthChecker *HealthChecker, blacklist *auth.TokenBlacklist, tracingEnabled bool, sessionsDir string, secretsRegistry *secrets.Registry, imagesRegistry *images.Registry) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(RequestIDMiddleware())
	router.Use(EnhancedLoggingMiddleware())

	// Add OpenTelemetry tracing middleware if enabled
	if tracingEnabled {
		router.Use(otelgin.Middleware("stromboli"))
	}

	// Wire blacklist into auth config if provided
	if blacklist != nil {
		authConfig.Blacklist = blacklist
	}

	s := &Server{
		router:          router,
		runner:          r,
		claudeClient:    claudeClient,
		authConfig:      authConfig,
		rateLimitConfig: rateLimitConfig,
		jobMgr:          jobMgr,
		healthChecker:   healthChecker,
		blacklist:       blacklist,
		tracingEnabled:  tracingEnabled,
		historyHandler:  NewSessionHistoryHandler(sessionsDir),
		secretsHandler:  NewSecretsHandler(secretsRegistry),
		imagesHandler:   NewImagesHandler(imagesRegistry),
	}
	s.setupRoutes()

	return s
}

// Run starts the server on the given address
func (s *Server) Run(addr string) error {
	slog.Info("API server listening", "addr", addr)
	return s.router.Run(addr)
}

// Handler returns the HTTP handler for use with http.Server
func (s *Server) Handler() http.Handler {
	return s.router
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check and metrics are public (no auth required, no rate limiting)
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Auth routes (JWT token generation, refresh, validation)
	s.setupAuthRoutes()

	// Protected routes (require auth when enabled, rate limited when enabled)
	protected := s.router.Group("/")
	protected.Use(RateLimitMiddleware(s.rateLimitConfig))
	protected.Use(auth.Middleware(s.authConfig))
	{
		protected.GET("/claude/status", s.claudeStatus)
		protected.POST("/run", s.runClaude)
		protected.GET("/run/stream", s.runStream)

		// Async execution
		protected.POST("/run/async", s.runAsync)
		protected.GET("/jobs", s.listJobs)
		protected.GET("/jobs/:id", s.getJob)
		protected.DELETE("/jobs/:id", s.cancelJob)

		// Session management
		protected.GET("/sessions", s.listSessions)
		protected.DELETE("/sessions/:id", s.destroySession)

		// Session history (message retrieval)
		protected.GET("/sessions/:id/messages", s.historyHandler.ListMessages)
		protected.GET("/sessions/:id/messages/:message_id", s.historyHandler.GetMessage)

		// Secrets management
		protected.GET("/secrets", s.secretsHandler.List)
		protected.POST("/secrets", s.secretsHandler.Create)
		protected.GET("/secrets/:name", s.secretsHandler.Get)
		protected.DELETE("/secrets/:name", s.secretsHandler.Delete)

		// Image discovery (static routes first, parameterized last)
		protected.GET("/images", s.imagesHandler.List)
		protected.GET("/images/search", s.imagesHandler.Search)
		protected.POST("/images/pull", s.imagesHandler.Pull)
		protected.GET("/images/:name", s.imagesHandler.Inspect)
	}
}

// healthCheck returns API health status
// @Summary Health check
// @Description Returns the health status of the API with component checks
// @Tags system
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (s *Server) healthCheck(c *gin.Context) {
	// If no health checker is configured, return basic health response
	if s.healthChecker == nil {
		c.JSON(http.StatusOK, HealthResponse{
			Status:  "ok",
			Name:    "stromboli",
			Version: version.Version,
		})
		return
	}

	// Perform detailed health check
	health := s.healthChecker.Check(c.Request.Context())
	c.JSON(http.StatusOK, HealthResponse{
		Status:     health.Status,
		Name:       health.Name,
		Version:    health.Version,
		Components: health.Components,
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
		handleBadRequest(c, err)
		return
	}

	if !validateClaudeConfigured(c, s.claudeClient) {
		return
	}

	runnerReq := buildRunnerRequest(req)

	result, err := s.runner.Run(c.Request.Context(), runnerReq)
	if err != nil {
		handleRunError(c, err)
		return
	}

	status := "completed"
	var crashInfo *job.CrashInfo
	if result.CrashInfo != nil {
		status = "crashed"
		crashInfo = result.CrashInfo
	}

	usage := s.historyHandler.SumUsage(result.SessionID)

	c.JSON(http.StatusOK, RunResponse{
		ID:               result.ID,
		Status:           status,
		Output:           result.Output,
		StructuredOutput: extractStructuredOutput(result.Output),
		SessionID:        result.SessionID,
		CrashInfo:        crashInfo,
		Usage:            usage,
	})
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
		if errors.Is(err, strerrors.ErrSessionIDRequired) || errors.Is(err, strerrors.ErrInvalidSessionID) {
			status = http.StatusBadRequest
		} else if errors.Is(err, strerrors.ErrSessionNotFound) {
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

