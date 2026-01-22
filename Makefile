.PHONY: build run test lint dev clean claude-setup claude-status claude-logout docs docs-swagger docs-godoc

# Binary name
BINARY=stromboli

# Build the application
build:
	go build -o bin/$(BINARY) ./cmd/stromboli

# Run the application
run: build
	./bin/$(BINARY)

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build the agent Docker image
build-agent-image:
	podman build -t stromboli-agent:latest -f deployments/docker/Dockerfile.agent .

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
	@podman run --rm -v $(PWD):/app -w /app golang:1.24 sh -c '\
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
	@podman run --rm -v $(PWD):/app -w /app golang:1.24 sh -c '\
		echo "# Stromboli Code Documentation" > /app/docs/godoc/README.md && \
		echo "" >> /app/docs/godoc/README.md && \
		echo "Generated: $$(date)" >> /app/docs/godoc/README.md && \
		echo "" >> /app/docs/godoc/README.md && \
		for pkg in api claude podman runner container; do \
			echo "## Package $$pkg" >> /app/docs/godoc/README.md; \
			echo "" >> /app/docs/godoc/README.md; \
			echo "\`\`\`" >> /app/docs/godoc/README.md; \
			go doc -all ./internal/$$pkg 2>/dev/null >> /app/docs/godoc/README.md || true; \
			echo "\`\`\`" >> /app/docs/godoc/README.md; \
			echo "" >> /app/docs/godoc/README.md; \
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
			-e CLAUDE_CODE_OAUTH_TOKEN="$$(cat $(CLAUDE_SECRETS))" \
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
	@echo "  test             Run all tests"
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
