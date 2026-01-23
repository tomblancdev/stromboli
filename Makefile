.PHONY: build run test test-integration test-e2e test-all test-coverage lint dev clean claude-setup claude-status claude-logout docs docs-swagger docs-godoc

# Binary name
BINARY=stromboli

# Go container command - uses --userns=keep-id to preserve host file ownership
GO_IMAGE=golang:1.24
GO_RUN=podman run --rm --userns=keep-id -v $(PWD):/app -w /app $(GO_IMAGE)

# Build the application
build:
	$(GO_RUN) go build -o bin/$(BINARY) ./cmd/stromboli

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

# Serve documentation locally (interactive)
docs-serve:
	@echo "🌐 Starting documentation server..."
	@echo "   Swagger UI: http://localhost:8081"
	@echo "   Godoc:      http://localhost:6060"
	@echo ""
	@echo "Press Ctrl+C to stop"
	@podman run --rm -d --name stromboli-swagger -p 8081:8080 \
		-v $(PWD)/docs/swagger:/usr/share/nginx/html/swagger:ro \
		-e SWAGGER_JSON=/usr/share/nginx/html/swagger/swagger.json \
		swaggerapi/swagger-ui 2>/dev/null || echo "Swagger UI container already running or docs not generated"

# Stop documentation servers
docs-stop:
	@podman stop stromboli-swagger 2>/dev/null || true

# ============================================================================
# Claude Token Management
# ============================================================================

# Claude secrets file
CLAUDE_SECRETS=.claude-secrets

# Setup Claude token (extract from existing credentials)
claude-setup:
	@echo "🔐 Claude Token Setup"
	@echo ""
	@echo "Extracting token from ~/.claude/.credentials.json..."
	@if [ -f ~/.claude/.credentials.json ]; then \
		python3 -c "import json; print(json.load(open('$$HOME/.claude/.credentials.json'))['claudeAiOauth']['accessToken'])" > $(CLAUDE_SECRETS) \
		&& chmod 600 $(CLAUDE_SECRETS) \
		&& echo "✅ Token saved to $(CLAUDE_SECRETS)"; \
	else \
		echo "❌ No credentials found at ~/.claude/.credentials.json"; \
		echo "   Run 'claude' first to authenticate"; \
	fi

# Check Claude token status
claude-status:
	@if [ -f $(CLAUDE_SECRETS) ]; then \
		podman run --rm \
			-v "$$(pwd)/$(CLAUDE_SECRETS):/run/secrets/claude-token:ro" \
			stromboli-agent:latest \
			-p "respond with 'ok'" 2>&1 | grep -q "Invalid\|Error" \
			&& echo "❌ Token invalid - run 'make claude-setup'" \
			|| echo "✅ Claude authenticated"; \
	else \
		echo "❌ No token - run 'make claude-setup'"; \
	fi

# Remove Claude token
claude-logout:
	@rm -f $(CLAUDE_SECRETS) && echo "✅ Token removed" || echo "ℹ️  No token found"

# ============================================================================
# Help
# ============================================================================

help:
	@echo "Stromboli - Claude Code Container Orchestration"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build & Run:"
	@echo "  build            Build the binary"
	@echo "  run              Build and run"
	@echo "  dev              Run with hot reload"
	@echo "  clean            Clean build artifacts"
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
	@echo "  docs-serve       Serve docs locally (Swagger UI)"
	@echo "  docs-stop        Stop documentation servers"
	@echo ""
	@echo "Claude:"
	@echo "  claude-setup     Extract and save Claude token"
	@echo "  claude-status    Check Claude token validity"
	@echo "  claude-logout    Remove saved token"
	@echo ""
	@echo "Docker:"
	@echo "  build-agent-image  Build the agent container image"
