// Stromboli - Claude Code Container Orchestration API
//
// Stromboli is a simple API layer for executing Claude Code in isolated Podman containers.
// It provides secure, sandboxed execution with full Claude CLI options exposed.
//
// @title Stromboli API
// @version 1.0
// @description Claude Code container orchestration API - secure, isolated AI execution
// @termsOfService https://github.com/tomblanc/stromboli
//
// @contact.name API Support
// @contact.url https://stromboli/issues
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host localhost:8080
// @BasePath /
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description OAuth2 Bearer token (future)
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"stromboli/internal/agent"
	"stromboli/internal/api"
	"stromboli/internal/auth"
	"stromboli/internal/claude"
	"stromboli/internal/config"
	"stromboli/internal/images"
	"stromboli/internal/job"
	"stromboli/internal/runner"
	"stromboli/internal/secrets"
	"stromboli/internal/tracing"
	"stromboli/internal/version"
)

// defaultHealthTimeout is the timeout for each health check component
const defaultHealthTimeout = 5 * time.Second

// buildAgentArgv returns an agent.ArgvBuilder closure that knows the
// container image, credentials path, and sessions root for this deployment.
// The closure is invoked once per /agents create and produces the podman +
// claude argv that the agent.Spawner will exec.
//
// MVP scope: single image, claude with stream-json I/O, sessions mounted at
// /app/sessions, credentials at /home/stromboli/.claude/.credentials.json
// (matching the production compose template). Custom images, allowed-volumes,
// etc. land in a follow-up — see PR description.
func buildAgentArgv(agentImage, credentialsFile, sessionsDir string) agent.ArgvBuilder {
	return func(req agent.CreateRequest, agentID, sessionID string) ([]string, error) {
		// Container name doubles as the bookkeeping handle so a kill -9 of
		// stromboli leaves a discoverable orphan that cleanup can reap.
		containerName := "stromboli-" + agentID

		argv := []string{
			"podman", "run",
			"--rm",
			"--interactive",                   // keep stdin open for stream-json input
			"--name", containerName,
			"-e", "HOME=/home/user",
			"-v", credentialsFile + ":/home/user/.claude/.credentials.json:ro",
			"-v", sessionsDir + "/" + sessionID + ":/home/user",
			agentImage,
			"--input-format", "stream-json",
			"--output-format", "stream-json",
			"--include-partial-messages",
			"--session-id", sessionID,
		}
		if req.Workdir != "" {
			// Insert -w just before the image arg.
			imgIdx := len(argv) - 7 // 7 args after the image
			argv = append(argv[:imgIdx], append([]string{"-w", req.Workdir}, argv[imgIdx:]...)...)
		}
		return argv, nil
	}
}

// parseLogLevel maps a STROMBOLI_LOG_LEVEL value to a slog.Level. Unknown
// values fall back to Info; the caller is expected to emit a warning so the
// typo is visible in the log stream.
func parseLogLevel(s string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, true
	case "debug":
		return slog.LevelDebug, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

func main() {
	// Setup structured logging.
	// JSON is the default so logs slot straight into Loki/CloudWatch/Datadog
	// without fragile regex parsers. Operators can opt back into the human-
	// readable text format with STROMBOLI_LOG_FORMAT=text (useful for local dev).
	// Log level is configurable via STROMBOLI_LOG_LEVEL (debug|info|warn|error).
	rawLevel := os.Getenv("STROMBOLI_LOG_LEVEL")
	logLevel, levelOK := parseLogLevel(rawLevel)
	handlerOpts := &slog.HandlerOptions{Level: logLevel}
	var handler slog.Handler
	if os.Getenv("STROMBOLI_LOG_FORMAT") == "text" {
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	if !levelOK {
		slog.Warn("Unknown STROMBOLI_LOG_LEVEL — falling back to info",
			"received", rawLevel,
			"valid_values", "debug|info|warn|error")
	}

	slog.Info("Starting Stromboli 🌋", "version", version.String())

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Combine image and tag for container operations
	fullImage := cfg.Agent.Image + ":" + cfg.Agent.ImageTag

	slog.Info("Configuration loaded",
		"server_address", cfg.Server.Address,
		"agent_image", fullImage)

	// Ensure Claude CLI image is available (for dynamic image mounting)
	if cfg.Agent.MountClaudeCLI {
		cliImageConfig := runner.CLIImageConfig{
			Image:    cfg.Agent.CLIImage,
			Tag:      cfg.Agent.CLIImageTag,
			AutoPull: cfg.Agent.AutoPullCLI,
		}
		cliImageMgr := runner.NewCLIImageManager(cliImageConfig, runner.NewShellExecutor())

		// Set the global CLI image name for the runner
		runner.ClaudeCLIImageName = cliImageMgr.GetCLIImageName()

		if err := cliImageMgr.EnsureImage(context.Background()); err != nil {
			slog.Error("Failed to ensure CLI image", "error", err)
			os.Exit(1)
		}
	}

	// Initialize tracing
	tracingCfg := tracing.Config{
		Enabled:     cfg.Tracing.Enabled,
		ServiceName: cfg.Tracing.ServiceName,
		Endpoint:    cfg.Tracing.Endpoint,
		Insecure:    cfg.Tracing.Insecure,
	}

	shutdownTracing, err := tracing.Init(context.Background(), tracingCfg)
	if err != nil {
		slog.Error("Failed to initialize tracing", "error", err)
		os.Exit(1)
	}

	if cfg.Tracing.Enabled {
		slog.Info("Tracing enabled",
			"service_name", cfg.Tracing.ServiceName,
			"endpoint", cfg.Tracing.Endpoint)
	} else {
		slog.Info("Tracing disabled")
	}

	// Create Claude client for credentials validation
	claudeClient := claude.NewClient(cfg.Agent.CredentialsFile)

	if !claudeClient.IsConfigured() {
		slog.Warn("Claude credentials not found - run 'claude' to authenticate",
			"expected_path", claudeClient.CredentialsFile())
	} else {
		slog.Info("Claude credentials found", "path", claudeClient.CredentialsFile())
	}

	resourceDefaults := runner.ResourceDefaults{
		Memory:  cfg.Resources.Memory,
		CPUs:    cfg.Resources.CPUs,
		Timeout: cfg.Resources.Timeout,
	}

	imageConfig := runner.ImageConfig{
		AllowedPatterns: cfg.Agent.AllowedImagePatterns,
		MountClaudeCLI:  cfg.Agent.MountClaudeCLI,
	}

	volumeConfig := runner.VolumeConfig{
		AllowedVolumes:    cfg.Agent.AllowedVolumes,
		AllowAllVolumes:   cfg.Agent.AllowAllVolumes,
		WorkdirAutoCreate: cfg.Agent.WorkdirAutoCreate,
		VolumeAutoCreate:  cfg.Agent.VolumeAutoCreate,
	}

	podmanRunner, err := runner.NewPodmanRunnerFull(
		fullImage,
		cfg.Agent.CredentialsFile,
		cfg.Agent.SessionsDir,
		cfg.Agent.SessionsHostDir,
		resourceDefaults,
		imageConfig,
		volumeConfig,
		runner.NewShellExecutor(),
	)
	if err != nil {
		slog.Error("Failed to create runner", "error", err)
		os.Exit(1)
	}

	slog.Info("Resource defaults configured",
		"memory", cfg.Resources.Memory,
		"cpus", cfg.Resources.CPUs,
		"timeout", cfg.Resources.Timeout)

	// Configure compose support if needed
	if cfg.Compose.BuildTimeout > 0 {
		podmanRunner.SetComposeConfig(runner.ComposeConfig{
			AllowPrivileged:  cfg.Compose.AllowPrivileged,
			AllowHostNetwork: cfg.Compose.AllowHostNetwork,
			AllowHostVolumes: cfg.Compose.AllowHostVolumes,
			BuildTimeout:     cfg.Compose.BuildTimeout,
			HealthTimeout:    cfg.Compose.HealthTimeout,
			StackTTL:         cfg.Compose.StackTTL,
		})
		slog.Info("Compose environment support enabled",
			"allow_privileged", cfg.Compose.AllowPrivileged,
			"allow_host_network", cfg.Compose.AllowHostNetwork,
			"allow_host_volumes", cfg.Compose.AllowHostVolumes,
			"build_timeout", cfg.Compose.BuildTimeout,
			"health_timeout", cfg.Compose.HealthTimeout,
			"stack_ttl", cfg.Compose.StackTTL)
	}

	if len(cfg.Agent.AllowedImagePatterns) > 0 {
		slog.Info("Dynamic images enabled",
			"allowed_patterns", cfg.Agent.AllowedImagePatterns,
			"mount_claude_cli", cfg.Agent.MountClaudeCLI)
	}

	if len(cfg.Agent.AllowedVolumes) > 0 {
		slog.Info("Volume allowlist configured",
			"allowed_volumes", cfg.Agent.AllowedVolumes)
	} else if cfg.Agent.AllowAllVolumes {
		slog.Warn("⚠️  SECURITY WARNING: allow_all_volumes is enabled - all host paths can be mounted!")
	} else {
		slog.Info("Volume mounts disabled (no allowlist configured)")
	}

	slog.Info("Workdir auto-creation", "enabled", cfg.Agent.WorkdirAutoCreate)

	// Cleanup orphaned containers from previous runs
	slog.Info("Cleaning up orphaned containers...")
	if err := runner.CleanupOrphanedContainers(context.Background()); err != nil {
		slog.Warn("Failed to cleanup orphaned containers", "error", err)
		// Non-fatal, continue starting server
	}

	// Build auth config for middleware
	authConfig := auth.Config{
		Enabled:     cfg.Auth.Enabled,
		ValidTokens: cfg.Auth.ValidTokens,
		JWTConfig: auth.JWTConfig{
			Secret:        cfg.JWT.Secret,
			AccessExpiry:  cfg.JWT.AccessExpiry,
			RefreshExpiry: cfg.JWT.RefreshExpiry,
		},
	}

	if authConfig.Enabled {
		slog.Info("Authentication enabled", "tokens", len(authConfig.ValidTokens))
		if cfg.JWT.Secret != "" {
			slog.Info("JWT authentication enabled",
				"access_expiry", cfg.JWT.AccessExpiry,
				"refresh_expiry", cfg.JWT.RefreshExpiry)
		}
	} else {
		slog.Info("Authentication disabled")
	}

	// Build rate limit config
	rateLimitConfig := api.RateLimitConfig{
		Enabled: cfg.RateLimit.Enabled,
		Rate:    cfg.RateLimit.Rate,
		Period:  cfg.RateLimit.Period,
		Burst:   cfg.RateLimit.Burst,
	}

	if rateLimitConfig.Enabled {
		slog.Info("Rate limiting enabled", "rate", rateLimitConfig.Rate, "burst", rateLimitConfig.Burst)
	} else {
		slog.Info("Rate limiting disabled")
	}

	// Create job manager for async execution
	jobMgr := job.NewManager()

	// Start job cleanup with configured TTL and interval
	jobMgr.StartCleanup(cfg.Jobs.CleanupTTL, cfg.Jobs.CleanupInterval)
	slog.Info("Job cleanup started",
		"ttl", cfg.Jobs.CleanupTTL,
		"interval", cfg.Jobs.CleanupInterval)

	// Start compose stack cleanup if compose is configured
	var composeCleanupStop chan struct{}
	if cfg.Compose.BuildTimeout > 0 {
		composeCleanupStop = make(chan struct{})
		go func() {
			ticker := time.NewTicker(cfg.Jobs.CleanupInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := podmanRunner.CleanupComposeStacks(context.Background(), cfg.Compose.StackTTL); err != nil {
						slog.Warn("Failed to cleanup compose stacks", "error", err)
					}
				case <-composeCleanupStop:
					return
				}
			}
		}()
		slog.Info("Compose stack cleanup started", "ttl", cfg.Compose.StackTTL)
	}

	// Create token blacklist for logout support
	blacklist := auth.NewTokenBlacklist()

	// Start blacklist cleanup (cleanup every hour - expired tokens are removed)
	blacklist.StartCleanup(time.Hour)
	slog.Info("Token blacklist started", "cleanup_interval", time.Hour)

	// Create health checker with credentials file and secret check
	healthConfig := api.HealthConfig{
		Timeout:         defaultHealthTimeout,
		CredentialsFile: claudeClient.CredentialsFile(),
		SecretName:      secrets.DefaultSecretName,
	}
	healthChecker := api.NewHealthChecker(runner.NewShellExecutor(), healthConfig)
	slog.Info("Health checker configured",
		"timeout", healthConfig.Timeout,
		"credentials_file", healthConfig.CredentialsFile,
		"secret_name", healthConfig.SecretName)

	// Create secrets registry for secret management API
	secretsRegistry := secrets.NewRegistry(runner.NewShellExecutor())

	// Create images registry for image discovery API
	imagesRegistry := images.NewRegistry(runner.NewShellExecutor())

	// Setup signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Persistent agents (long-lived stream-json Claude sessions). One Manager
	// for the whole server; the handler is wired into the API and shut down
	// alongside the rest of the goroutines on SIGTERM.
	agentManager := agent.NewManager(agent.NewProcessSpawner())
	agentArgvBuilder := buildAgentArgv(fullImage, cfg.Agent.CredentialsFile, cfg.Agent.SessionsDir)
	agentsHandler := api.NewAgentsHandler(agentManager, agentArgvBuilder)

	// Create the API server
	server := api.NewServer(podmanRunner, claudeClient, authConfig, rateLimitConfig, jobMgr, healthChecker, blacklist, cfg.Tracing.Enabled, cfg.Agent.SessionsDir, secretsRegistry, imagesRegistry, agentsHandler)

	srv := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("Server starting", "addr", cfg.Server.Address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Start the metrics endpoint on a separate listener. The default address
	// (127.0.0.1:9090) keeps Prometheus scrape data off any public interface;
	// operators that need to expose it move it onto an internal-only network
	// via STROMBOLI_METRICS_ADDRESS.
	var metricsSrv *http.Server
	if cfg.Metrics.Enabled {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsSrv = &http.Server{
			Addr:              cfg.Metrics.Address,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("Metrics endpoint starting", "addr", cfg.Metrics.Address)
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("Metrics endpoint error", "error", err)
				// Non-fatal: app keeps serving requests even if metrics listener fails.
			}
		}()
	} else {
		slog.Info("Metrics endpoint disabled")
	}

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("Shutdown signal received, gracefully shutting down...")

	// Stop job cleanup
	jobMgr.StopCleanup()
	slog.Info("Job cleanup stopped")

	// Stop compose stack cleanup
	if composeCleanupStop != nil {
		close(composeCleanupStop)
		slog.Info("Compose stack cleanup stopped")
	}

	// Stop blacklist cleanup
	blacklist.StopCleanup()
	slog.Info("Token blacklist cleanup stopped")

	// Tear every persistent agent down so we don't leak podman containers
	// after the API stops accepting traffic.
	agentsHandler.StopAll(context.Background())
	slog.Info("Persistent agents stopped")

	// Shutdown tracing
	if err := shutdownTracing(context.Background()); err != nil {
		slog.Error("Failed to shutdown tracing", "error", err)
	}
	slog.Info("Tracing shutdown completed")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Metrics endpoint forced to shutdown", "error", err)
		}
	}

	slog.Info("Server stopped")
}
