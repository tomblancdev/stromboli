.PHONY: build run test lint dev clean claude-setup claude-status claude-logout

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

# Development: run with hot reload (requires air)
dev:
	air -c .air.toml || go run ./cmd/stromboli

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
