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
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	"stromboli/internal/types"
	"stromboli/internal/version"
)

// defaultHealthTimeout is the timeout for each health check component
const defaultHealthTimeout = 5 * time.Second

// buildAgentArgv returns an agent.ArgvBuilder closure that knows the
// container image, credentials path, and sessions root for this deployment.
// The closure is invoked once per /agents create and produces the podman +
// claude argv that the agent.Spawner will exec.
//
// The argv layout is shared with the regular /run runner via
// claude.ApplyOptions and claude.EnvVars — when a caller sends `claude.model`
// or `claude.allowed_tools` to /agents, those translate to the same flags
// /run honors. Three things are pinned and not user-overridable: input/output
// format (stream-json), include_partial_messages, and verbose — together
// they're the contract the agent's stdout-line dispatcher relies on.
func buildAgentArgv(agentImage, credentialsFile, sessionsDir string) agent.ArgvBuilder {
	return func(req agent.CreateRequest, agentID, sessionID string) ([]string, error) {
		// Pre-create the per-session bind-mount source. Podman's `-v` requires
		// the host path to exist before mount — without this, the spawner
		// dies with `statfs ...: no such file or directory` and the agent
		// transitions straight to StatusExited with `signal: killed` from
		// cmd.Wait, which gives operators almost nothing to debug from.
		// Mode 0700: only the runtime user reads sessions; matches the
		// existing /run runner's session-dir creation.
		sessionDir := filepath.Join(sessionsDir, sessionID)
		if err := os.MkdirAll(sessionDir, 0o700); err != nil {
			return nil, fmt.Errorf("create session dir %q: %w", sessionDir, err)
		}

		// Container name doubles as the bookkeeping handle so a kill -9 of
		// stromboli leaves a discoverable orphan that cleanup can reap.
		containerName := "stromboli-" + agentID

		// Map the host user into the container and preserve their UID. Without
		// this the container runs as root, and claude refuses
		// --dangerously-skip-permissions when running as root. The regular
		// /run runner does the same dance via podman.CommandBuilder.WithKeepID.
		hostUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())

		// ----- podman flags -----
		podmanArgs := []string{
			"podman", "run",
			"--rm",
			"--interactive", // keep stdin open for stream-json input
			"--name", containerName,
			"--userns=keep-id", // map host UID/GID into container
			"--user", hostUser, // run claude as the host user
			"-e", "HOME=/home/user",
			"-v", credentialsFile + ":/home/user/.claude/.credentials.json:ro",
			"-v", sessionDir + ":/home/user",
		}
		// Per-request env vars (prompt cache TTL, Bedrock tier, PowerShell tool).
		podmanArgs = append(podmanArgs, claude.EnvVars(opts(req))...)
		if req.Workdir != "" {
			podmanArgs = append(podmanArgs, "-w", req.Workdir)
		}
		podmanArgs = append(podmanArgs, agentImage)

		// ----- claude flags -----
		// The agent image's entrypoint (docker-entrypoint.sh) execs its first
		// positional arg as the binary; we name "claude" explicitly so the
		// rest of the flags reach the Claude CLI rather than node (the image
		// default). Same prepend pattern the regular /run runner uses.
		//
		// Build via claude.NewCommandBuilder so user-supplied options
		// (model, effort, system_prompt, allowed_tools, etc.) are honored
		// the same way they are for /run.
		cb := claude.NewCommandBuilder().
			WithSessionID(sessionID).
			WithInputFormat("stream-json").
			WithOutputFormat("stream-json").
			WithIncludePartialMessages().
			WithVerbose() // required by claude when output-format=stream-json
		claude.ApplyOptions(cb, opts(req))

		// ApplyOptions may have flipped these from user input — restore the
		// agent-required values. Stream-json on stdin/stdout is how the
		// dispatcher reads agent events; flipping output to "text" would
		// silently break every subscriber.
		cb.WithInputFormat("stream-json")
		cb.WithOutputFormat("stream-json")

		argv := append(podmanArgs, "claude")
		argv = append(argv, cb.Build()...)
		return argv, nil
	}
}

// opts unpacks the optional Claude options on a CreateRequest into a value
// safe for ApplyOptions / EnvVars to consume. Callers pass `req.Claude` as a
// pointer so the JSON shape is `claude: { ... }` rather than a freeform
// object — but the helper APIs want the value directly.
func opts(req agent.CreateRequest) types.ClaudeOptions {
	if req.Claude == nil {
		return types.ClaudeOptions{}
	}
	return *req.Claude
}

// buildBlacklist constructs the configured Blacklist backend, starts its
// cleanup goroutine, and returns a closer that the shutdown path calls to
// flush + release any underlying resources (e.g. the bolt DB file lock).
//
// Default: in-memory. Operators opt into durability with
// STROMBOLI_AUTH_BLACKLIST_BACKEND=bolt.
func buildBlacklist(cfg config.BlacklistConfig) (auth.Blacklist, func() error, error) {
	switch cfg.Backend {
	case "", "memory":
		bl := auth.NewMemoryBlacklist()
		bl.StartCleanup(cfg.CleanupInterval)
		return bl, bl.Close, nil
	case "bolt":
		bl, err := auth.NewBoltBlacklist(cfg.BoltPath)
		if err != nil {
			return nil, nil, fmt.Errorf("bolt blacklist: %w", err)
		}
		bl.StartCleanup(cfg.CleanupInterval)
		return bl, bl.Close, nil
	default:
		// Unreachable — config.Validate catches unknown backends — but keep
		// a defensive branch so a future config bug doesn't silently fall
		// back to memory.
		return nil, nil, fmt.Errorf("unknown blacklist backend %q", cfg.Backend)
	}
}

// blacklistBackendName returns the resolved backend label for logging
// (treats the empty string as the default "memory").
func blacklistBackendName(cfg config.BlacklistConfig) string {
	if cfg.Backend == "" {
		return "memory"
	}
	return cfg.Backend
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
		Enabled:        cfg.RateLimit.Enabled,
		Rate:           cfg.RateLimit.Rate,
		Period:         cfg.RateLimit.Period,
		Burst:          cfg.RateLimit.Burst,
		TrustedProxies: cfg.RateLimit.TrustedProxies,
	}

	if rateLimitConfig.Enabled {
		slog.Info("Rate limiting enabled",
			"rate", rateLimitConfig.Rate,
			"burst", rateLimitConfig.Burst,
			"trusted_proxies", len(rateLimitConfig.TrustedProxies))
	} else {
		slog.Info("Rate limiting disabled")
	}

	// Webhook signing — when no secret is configured, outgoing async-job
	// webhooks ship unsigned. Loud about it so an operator can spot the
	// missing setting before someone forges a callback.
	if cfg.Webhook.SigningSecret == "" {
		slog.Warn("Webhook signing disabled: STROMBOLI_WEBHOOK_SIGNING_SECRET is empty — outgoing job webhooks will be unsigned and unverifiable")
	} else {
		slog.Info("Webhook signing enabled (HMAC-SHA256)")
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

	// Create token blacklist for logout support. Backend selection is
	// config-driven; in-memory is the default (zero deps, lost on restart),
	// bolt persists across restarts via a single-file store.
	blacklist, blacklistCloser, err := buildBlacklist(cfg.Auth.Blacklist)
	if err != nil {
		slog.Error("Failed to initialize token blacklist", "error", err)
		os.Exit(1)
	}
	slog.Info("Token blacklist started",
		"backend", blacklistBackendName(cfg.Auth.Blacklist),
		"cleanup_interval", cfg.Auth.Blacklist.CleanupInterval)

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
	server := api.NewServer(podmanRunner, claudeClient, authConfig, rateLimitConfig, jobMgr, healthChecker, blacklist, cfg.Tracing.Enabled, cfg.Agent.SessionsDir, secretsRegistry, imagesRegistry, agentsHandler, cfg.Webhook.SigningSecret)

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

	// Stop blacklist cleanup and (for the bolt backend) close the underlying
	// DB so the file lock is released.
	if err := blacklistCloser(); err != nil {
		slog.Warn("Token blacklist close returned error", "error", err)
	}
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
