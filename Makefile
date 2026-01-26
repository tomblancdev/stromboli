.PHONY: build run test test-integration test-e2e test-all test-coverage lint dev clean claude-setup claude-check claude-status claude-logout secret-create secret-update secret-remove secret-status docs docs-swagger docs-godoc docs-serve docs-stop docs-logs build-claude-cli build-claude-cli-version

# Binary name
BINARY=stromboli

# Version info from git
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Linker flags to inject version
LDFLAGS=-ldflags "-X stromboli/internal/version.Version=$(VERSION) -X stromboli/internal/version.Commit=$(COMMIT) -X stromboli/internal/version.BuildTime=$(BUILD_TIME)"

# Go container command - uses --userns=keep-id to preserve host file ownership
GO_IMAGE=golang:1.24
GO_RUN=podman run --rm --userns=keep-id -v $(PWD):/app -w /app $(GO_IMAGE)

# Build the application
build:
	$(GO_RUN) go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/stromboli

# Run the application
run: build
	./bin/$(BINARY)

# Run unit tests only (no integration tag)
test:
	$(GO_RUN) go test -v ./...

# Run integration tests only (requires Podman)
test-integration:
	$(GO_RUN) go test -v -tags=integration ./...

# Run E2E tests only (requires server setup and optionally Claude token)
test-e2e:
	$(GO_RUN) go test -v -tags=e2e ./tests/e2e/...

# Run all tests (unit + integration + e2e)
test-all:
	$(GO_RUN) sh -c 'go test -v ./... && go test -v -tags=integration ./... && go test -v -tags=e2e ./tests/e2e/...'

# Run tests with coverage
test-coverage:
	$(GO_RUN) sh -c 'go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html'

# Run linter
lint:
	$(GO_RUN) sh -c 'go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && golangci-lint run'

# Download dependencies
deps:
	$(GO_RUN) sh -c 'go mod download && go mod tidy'

# Build the agent Docker image
build-agent-image:
	podman build -t stromboli-agent:latest -f deployments/docker/Dockerfile.agent deployments/docker

# Build the Claude CLI image (for dynamic image support)
build-claude-cli:
	@echo "🔧 Building Claude CLI image..."
	podman build -t stromboli-claude-cli:latest -f deployments/docker/Dockerfile.claude-cli deployments/docker
	@echo "✅ Claude CLI image built: stromboli-claude-cli:latest"

# Build Claude CLI image with specific version
build-claude-cli-version:
	@echo "🔧 Building Claude CLI image (version: $(VERSION))..."
	podman build --build-arg CLAUDE_CODE_VERSION=$(VERSION) -t stromboli-claude-cli:$(VERSION) -f deployments/docker/Dockerfile.claude-cli deployments/docker
	@echo "✅ Claude CLI image built: stromboli-claude-cli:$(VERSION)"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -rf docs/swagger docs/godoc

# Development: run with hot reload (requires air)
dev:
	air -c .air.toml || go run ./cmd/stromboli

# ============================================================================
# Documentation
# ============================================================================

# Generate all documentation
docs: docs-swagger docs-godoc
	@echo "📚 Documentation generated in docs/"

# Generate OpenAPI/Swagger documentation
docs-swagger:
	@echo "📝 Generating OpenAPI/Swagger docs..."
	@mkdir -p docs/swagger
	@$(GO_RUN) sh -c '\
		go install github.com/swaggo/swag/cmd/swag@latest && \
		swag init -g cmd/stromboli/main.go -o docs/swagger --parseDependency --parseInternal'
	@echo "✅ Swagger docs: docs/swagger/"
	@echo "   - swagger.json"
	@echo "   - swagger.yaml"
	@echo "   - docs.go (for embedding)"

# Generate Go code documentation
docs-godoc:
	@echo "📖 Generating Go documentation..."
	@mkdir -p docs/godoc
	@$(GO_RUN) sh -c '\
		OUT=/app/docs/godoc/README.md && \
		echo "# 📚 Stromboli API Reference" > $$OUT && \
		echo "" >> $$OUT && \
		echo "> Auto-generated Go documentation for all packages" >> $$OUT && \
		echo "" >> $$OUT && \
		echo "**Generated:** $$(date -u +"%Y-%m-%d %H:%M UTC")" >> $$OUT && \
		echo "" >> $$OUT && \
		echo "---" >> $$OUT && \
		echo "" >> $$OUT && \
		echo "## 📋 Table of Contents" >> $$OUT && \
		echo "" >> $$OUT && \
		echo "### Core" >> $$OUT && \
		echo "- [api](#package-api) - HTTP handlers and REST API endpoints" >> $$OUT && \
		echo "- [config](#package-config) - Configuration management with Viper" >> $$OUT && \
		echo "- [errors](#package-errors) - Custom error types" >> $$OUT && \
		echo "" >> $$OUT && \
		echo "### Execution" >> $$OUT && \
		echo "- [runner](#package-runner) - Container execution engine" >> $$OUT && \
		echo "- [podman](#package-podman) - Podman command builder" >> $$OUT && \
		echo "- [claude](#package-claude) - Claude CLI command builder" >> $$OUT && \
		echo "- [job](#package-job) - Async job management" >> $$OUT && \
		echo "" >> $$OUT && \
		echo "### Security" >> $$OUT && \
		echo "- [auth](#package-auth) - JWT authentication and middleware" >> $$OUT && \
		echo "- [secrets](#package-secrets) - Podman secrets management" >> $$OUT && \
		echo "" >> $$OUT && \
		echo "### Infrastructure" >> $$OUT && \
		echo "- [session](#package-session) - Session ID generation" >> $$OUT && \
		echo "- [workspace](#package-workspace) - Workspace validation" >> $$OUT && \
		echo "- [webhook](#package-webhook) - Webhook notifications" >> $$OUT && \
		echo "- [metrics](#package-metrics) - Prometheus metrics" >> $$OUT && \
		echo "- [tracing](#package-tracing) - OpenTelemetry distributed tracing" >> $$OUT && \
		echo "- [types](#package-types) - Shared data types" >> $$OUT && \
		echo "" >> $$OUT && \
		echo "---" >> $$OUT && \
		echo "" >> $$OUT && \
		for pkg in api config errors runner podman claude job auth secrets session workspace webhook metrics tracing types; do \
			echo "## Package $$pkg" >> $$OUT; \
			echo "" >> $$OUT; \
			echo "<details>" >> $$OUT; \
			echo "<summary>📦 Click to expand</summary>" >> $$OUT; \
			echo "" >> $$OUT; \
			echo "\`\`\`go" >> $$OUT; \
			go doc -all ./internal/$$pkg 2>/dev/null >> $$OUT || echo "// Package $$pkg - documentation not available" >> $$OUT; \
			echo "\`\`\`" >> $$OUT; \
			echo "" >> $$OUT; \
			echo "</details>" >> $$OUT; \
			echo "" >> $$OUT; \
			echo "[⬆️ Back to top](#-table-of-contents)" >> $$OUT; \
			echo "" >> $$OUT; \
			echo "---" >> $$OUT; \
			echo "" >> $$OUT; \
		done'
	@echo "✅ Code docs: docs/godoc/README.md"

# Serve documentation locally (uses compose)
docs-serve:
	@echo "🌐 Starting documentation servers..."
	@if [ ! -f docs/swagger/swagger.json ]; then \
		echo "❌ Swagger docs not found. Run: make docs"; \
		exit 1; \
	fi
	podman compose -f deployments/docker/compose.docs.yml up -d
	@echo ""
	@echo "✅ Documentation servers running:"
	@echo "   📖 Swagger UI:    http://localhost:8081"
	@echo "   📚 Godoc Server:  http://localhost:8082/docs/"
	@echo ""
	@echo "Stop with: make docs-stop"

# Stop documentation servers
docs-stop:
	@echo "🛑 Stopping documentation servers..."
	podman compose -f deployments/docker/compose.docs.yml down
	@echo "✅ Documentation servers stopped"

# View documentation server logs
docs-logs:
	podman compose -f deployments/docker/compose.docs.yml logs -f

# ============================================================================
# Claude Credentials Management
# ============================================================================

# Claude credentials file and Podman secret name
CLAUDE_CREDENTIALS=~/.claude/.credentials.json
CLAUDE_SECRET_NAME=claude-credentials

# Check Claude credentials status
claude-check:
	@echo "🔐 Claude Credentials Check"
	@echo ""
	@if [ -f ~/.claude/.credentials.json ]; then \
		echo "✅ Credentials found at ~/.claude/.credentials.json"; \
	else \
		echo "❌ No credentials found at ~/.claude/.credentials.json"; \
		echo "   Run 'claude' first to authenticate"; \
		exit 1; \
	fi

# Alias for backwards compatibility
claude-setup: claude-check

# Create Podman secret from credentials file
secret-create: claude-check
	@echo "🔐 Creating Podman secret '$(CLAUDE_SECRET_NAME)'..."
	@if podman secret exists $(CLAUDE_SECRET_NAME) 2>/dev/null; then \
		echo "⚠️  Secret already exists. Use 'make secret-update' to refresh it."; \
	else \
		podman secret create $(CLAUDE_SECRET_NAME) ~/.claude/.credentials.json && \
		echo "✅ Secret '$(CLAUDE_SECRET_NAME)' created"; \
	fi

# Update Podman secret (remove and recreate)
secret-update: claude-check
	@echo "🔄 Updating Podman secret '$(CLAUDE_SECRET_NAME)'..."
	@podman secret rm $(CLAUDE_SECRET_NAME) 2>/dev/null || true
	@podman secret create $(CLAUDE_SECRET_NAME) ~/.claude/.credentials.json
	@echo "✅ Secret '$(CLAUDE_SECRET_NAME)' updated"
	@echo ""
	@echo "ℹ️  Restart containers to use new credentials:"
	@echo "   make container-restart"

# Remove Podman secret
secret-remove:
	@echo "🗑️  Removing Podman secret '$(CLAUDE_SECRET_NAME)'..."
	@podman secret rm $(CLAUDE_SECRET_NAME) 2>/dev/null && \
		echo "✅ Secret removed" || \
		echo "ℹ️  Secret doesn't exist"

# Check secret status
secret-status:
	@echo "🔐 Podman Secret Status"
	@echo ""
	@if podman secret exists $(CLAUDE_SECRET_NAME) 2>/dev/null; then \
		echo "✅ Secret '$(CLAUDE_SECRET_NAME)' exists"; \
		podman secret inspect $(CLAUDE_SECRET_NAME) --format "   Created: {{.CreatedAt}}"; \
	else \
		echo "❌ Secret '$(CLAUDE_SECRET_NAME)' not found"; \
		echo "   Create it with: make secret-create"; \
	fi

# Check Claude token validity by running a test prompt
claude-status:
	@if [ -f ~/.claude/.credentials.json ]; then \
		echo "Testing Claude authentication..."; \
		podman run --rm \
			-v "$$HOME/.claude/.credentials.json:/home/user/.claude/.credentials.json:ro" \
			-e HOME=/home/user \
			stromboli-agent:latest \
			-p "respond with 'ok'" 2>&1 | grep -q "Invalid\|Error" \
			&& echo "❌ Token invalid or expired - re-run 'claude' to refresh" \
			|| echo "✅ Claude authenticated"; \
	else \
		echo "❌ No credentials - run 'claude' first to authenticate"; \
	fi

# Remove Claude credentials (just shows info, doesn't delete user's credentials)
claude-logout:
	@echo "ℹ️  To logout, run 'claude logout' or delete ~/.claude/.credentials.json"

# ============================================================================
# Container Deployment
# ============================================================================

# Build the server container image
build-server-image:
	@echo "🏗️  Building Stromboli server image..."
	podman build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t stromboli:latest \
		-f deployments/docker/Dockerfile.server .
	@echo "✅ Server image built: stromboli:latest ($(VERSION))"

# Build all images (server + agent + claude-cli)
build-images: build-server-image build-agent-image build-claude-cli
	@echo "✅ All images built"

# Start Stromboli in container (requires Podman socket)
container-start:
	@echo "🚀 Starting Stromboli container..."
	@# Check Podman socket
	@if [ ! -S "$${XDG_RUNTIME_DIR:-/run/user/$$(id -u)}/podman/podman.sock" ]; then \
		echo "❌ Podman socket not found. Enable it with:"; \
		echo "   make podman-socket-enable"; \
		exit 1; \
	fi
	@# Check Claude credentials file exists
	@if [ ! -f ~/.claude/.credentials.json ]; then \
		echo "❌ Claude credentials not found at ~/.claude/.credentials.json"; \
		echo "   Run 'claude' first to authenticate"; \
		exit 1; \
	fi
	@# Warn if agent secret doesn't exist
	@if ! podman secret exists $(CLAUDE_SECRET_NAME) 2>/dev/null; then \
		echo "⚠️  Podman secret '$(CLAUDE_SECRET_NAME)' not found (needed for agents)"; \
		echo "   Create it with: make secret-create"; \
		echo ""; \
	fi
	@# Create sessions directory
	@mkdir -p /tmp/stromboli-sessions
	@# Start container
	podman compose -f deployments/docker/compose.yml up -d
	@echo ""
	@echo "✅ Stromboli running:"
	@echo "   🚀 API:         http://localhost:8080"
	@echo "   📖 Swagger UI:  http://localhost:8081"
	@echo "   ❤️  Health:      http://localhost:8080/health"
	@echo ""
	@echo "Commands:"
	@echo "   make container-logs    View logs"
	@echo "   make container-stop    Stop containers"
	@echo ""
	@echo "After token refresh:"
	@echo "   make container-restart  (server auto-refreshes from file)"
	@echo "   make secret-update      (if agents need updated credentials)"

# Stop Stromboli container
container-stop:
	@echo "🛑 Stopping Stromboli container..."
	podman compose -f deployments/docker/compose.yml down
	@echo "✅ Container stopped"

# View container logs
container-logs:
	podman compose -f deployments/docker/compose.yml logs -f

# Restart container
container-restart: container-stop container-start

# Container status
container-status:
	@podman ps --filter name=stromboli --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# Enable Podman socket (required for container mode)
podman-socket-enable:
	@echo "🔌 Enabling Podman socket..."
	systemctl --user enable --now podman.socket
	@echo "✅ Socket enabled at: $${XDG_RUNTIME_DIR}/podman/podman.sock"

# Full container setup (build images + configure + start)
container-setup: podman-socket-enable build-images secret-create container-start
	@echo ""
	@echo "🎉 Stromboli is fully set up and running!"
	@echo ""
	@echo "Test it with:"
	@echo "  curl http://localhost:8080/health"
	@echo "  curl -X POST http://localhost:8080/run -H 'Content-Type: application/json' -d '{\"prompt\": \"Say hello\"}'"

# ============================================================================
# Help
# ============================================================================

help:
	@echo "Stromboli - Claude Code Container Orchestration"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build & Run (Host):"
	@echo "  build            Build the binary"
	@echo "  run              Build and run"
	@echo "  dev              Run with hot reload"
	@echo "  clean            Clean build artifacts"
	@echo ""
	@echo "Container Deployment:"
	@echo "  container-setup    Full setup: build + configure + start (recommended)"
	@echo "  container-start    Start Stromboli in container"
	@echo "  container-stop     Stop Stromboli container"
	@echo "  container-restart  Restart Stromboli container"
	@echo "  container-logs     View container logs"
	@echo "  container-status   Check container status"
	@echo "  build-images       Build all container images (server + agent + claude-cli)"
	@echo "  build-claude-cli   Build Claude CLI image (for dynamic images)"
	@echo "  podman-socket-enable Enable Podman socket"
	@echo ""
	@echo "Testing:"
	@echo "  test             Run unit tests only"
	@echo "  test-integration Run integration tests (requires Podman)"
	@echo "  test-e2e         Run E2E tests (requires server and optionally Claude token)"
	@echo "  test-all         Run all tests (unit + integration + E2E)"
	@echo "  test-coverage    Run tests with coverage report"
	@echo "  lint             Run linter"
	@echo ""
	@echo "Documentation:"
	@echo "  docs             Generate all documentation"
	@echo "  docs-swagger     Generate OpenAPI/Swagger docs"
	@echo "  docs-godoc       Generate Go code documentation"
	@echo "  docs-serve       Serve docs locally (Swagger UI + Godoc)"
	@echo "  docs-stop        Stop documentation servers"
	@echo "  docs-logs        View documentation server logs"
	@echo ""
	@echo "Claude & Secrets:"
	@echo "  claude-check     Verify Claude credentials exist"
	@echo "  claude-status    Check Claude token validity"
	@echo "  claude-logout    Show logout instructions"
	@echo "  secret-create    Create Podman secret from credentials"
	@echo "  secret-update    Update Podman secret (after token refresh)"
	@echo "  secret-remove    Remove Podman secret"
	@echo "  secret-status    Check Podman secret status"
