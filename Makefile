.PHONY: build run test lint dev clean

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
