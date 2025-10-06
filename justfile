# Build the CLI tool
build:
    mkdir -p bin
    go build -o bin/hf ./cmd/hf

# Build for Linux AMD64
build-linux-amd64:
    mkdir -p bin
    GOOS=linux GOARCH=amd64 go build -o bin/hf-linux-amd64 ./cmd/hf

# Build for Linux ARM64
build-linux-arm64:
    mkdir -p bin
    GOOS=linux GOARCH=arm64 go build -o bin/hf-linux-arm64 ./cmd/hf

# Build for all platforms
build-all: build build-linux-amd64 build-linux-arm64

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Install binary system-wide
install: build
    sudo cp bin/hf /usr/local/bin/

# Clean build artifacts
clean:
    rm -rf bin/

# Run API server on port 8888
serve: build
    ./bin/hf serve --port 8888

# Format Go code
fmt:
    go fmt ./...

# Run linter
lint:
    golangci-lint run

# Download and tidy dependencies
deps:
    go mod download
    go mod tidy

# Run tests and build
ci: test build

# Show example usage
examples:
    @echo "# View configuration"
    @echo "hf show network"
    @echo ""
    @echo "# Get value"
    @echo "hf get network.wan.ipaddr"
    @echo ""
    @echo "# Set value"
    @echo "hf set network.wan.ipaddr 192.168.1.1"
    @echo ""
    @echo "# Commit changes"
    @echo "hf commit"

# Build Docker image
docker-build:
    #!/usr/bin/env bash
    mkdir -p bin
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then
        GOOS=linux GOARCH=amd64 go build -o bin/hf ./cmd/hf
    else
        GOOS=linux GOARCH=arm64 go build -o bin/hf ./cmd/hf
    fi
    docker build -t hellfire-router .

# Run Docker compose (router + client)
docker-up: docker-build
    docker-compose up -d

# Stop Docker compose
docker-down:
    docker-compose down

# Build and run in Docker
docker: docker-up

# Test Docker container (API)
docker-test:
    curl http://localhost:8888/health

# Shell into router
docker-shell-router:
    docker exec -it hellfire-router bash

# Shell into client
docker-shell-client:
    docker exec -it hellfire-client bash

# View router logs
docker-logs:
    docker-compose logs -f router

# === Web UI Commands ===

# Install web UI dependencies
web-install:
    cd web && npm install

# Generate OpenAPI client from swagger spec
web-generate-client:
    ./scripts/generate-api-client.sh

# Run web UI development server
web-dev:
    cd web && npm run dev

# Build web UI for production
web-build:
    cd web && npm run build

# Run both backend and frontend in development mode
dev:
    #!/usr/bin/env bash
    trap 'kill 0' EXIT
    just serve &
    just web-dev &
    wait

# Full build: backend + frontend
build-all-full: build web-build

# Clean all build artifacts including web
clean-all: clean
    rm -rf web/dist web/node_modules web/src/lib/api
