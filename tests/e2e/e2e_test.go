//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pinocchio338/stromboli/internal/api"
	"github.com/pinocchio338/stromboli/internal/claude"
	"github.com/pinocchio338/stromboli/internal/job"
	"github.com/pinocchio338/stromboli/internal/runner"
)

// testEnv holds the test environment state
type testEnv struct {
	Server      *api.Server
	BaseURL     string
	ClaudeToken string
	TempDir     string
	SessionsDir string
	HasClaude   bool
	Cleanup     func()
}

// setupE2EEnv creates a complete E2E test environment with a real server
func setupE2EEnv(t *testing.T) *testEnv {
	t.Helper()

	// Create temporary directory for test artifacts
	tempDir, err := os.MkdirTemp("", "stromboli-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create sessions directory
	sessionsDir := filepath.Join(tempDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("failed to create sessions dir: %v", err)
	}

	// Create secrets file path
	secretsFile := filepath.Join(tempDir, ".claude-secrets")

	// Check for Claude token in environment
	claudeToken := os.Getenv("ANTHROPIC_API_KEY")
	hasClaude := claudeToken != ""

	// Write secrets file if token available
	if hasClaude {
		if err := os.WriteFile(secretsFile, []byte(claudeToken), 0600); err != nil {
			t.Fatalf("failed to write secrets file: %v", err)
		}
	}

	// Create Claude client (will be nil if no token)
	var claudeClient *claude.Client
	if hasClaude {
		claudeClient, err = claude.NewClientFromSecretsFile(secretsFile)
		if err != nil {
			t.Logf("Warning: Failed to create Claude client: %v", err)
			hasClaude = false
		}
	}

	// Create Podman runner with defaults
	podmanRunner := runner.NewPodmanRunnerWithDefaults(
		"stromboli-agent:latest",
		secretsFile,
		sessionsDir,
		nil, // No workspace restrictions
		"512m",
		"1",
		"30m",
	)

	// Create job manager
	jobManager := job.NewManager(1*time.Hour, 5*time.Minute)

	// Create server with test configuration
	server := api.NewServer(
		podmanRunner,
		jobManager,
		claudeClient,
		false, // Auth disabled for E2E tests
		false, // Rate limiting disabled for E2E tests
		0,
		0,
	)

	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Start server in background
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	baseURL := fmt.Sprintf("http://%s", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: server.Handler(),
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("Starting E2E test server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Wait for server to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		select {
		case err := <-serverErrors:
			os.RemoveAll(tempDir)
			t.Fatalf("server failed to start: %v", err)
		case <-ctx.Done():
			srv.Shutdown(context.Background())
			os.RemoveAll(tempDir)
			t.Fatalf("server failed to become ready within timeout")
		default:
			resp, err := http.Get(baseURL + "/health")
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					goto serverReady
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

serverReady:
	slog.Info("E2E test server ready", "url", baseURL)

	// Create cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		os.RemoveAll(tempDir)
	}

	env := &testEnv{
		Server:      server,
		BaseURL:     baseURL,
		ClaudeToken: claudeToken,
		TempDir:     tempDir,
		SessionsDir: sessionsDir,
		HasClaude:   hasClaude,
		Cleanup:     cleanup,
	}

	// Register cleanup
	t.Cleanup(cleanup)

	return env
}

// TestMain is the entry point for E2E tests
func TestMain(m *testing.M) {
	// Set up logging for tests
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Check if running in CI without required setup
	if os.Getenv("CI") == "true" && os.Getenv("ANTHROPIC_API_KEY") == "" {
		slog.Info("Skipping E2E tests in CI without ANTHROPIC_API_KEY")
		os.Exit(0)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}
