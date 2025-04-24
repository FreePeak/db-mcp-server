.PHONY: build run-stdio run-sse clean test client client-simple test-script build-example docker-build docker-run docker-run-stdio docker-stop docker-build-local docker-build-multiarch docker-pull-platform deploy-docker deploy-docker-simple

# Build the server
build:
	CGO_ENABLE=0 go build -o ./bin/server cmd/server/main.go
	CGO_ENABLE=0 GOOS=linux GOARCH=amd64 go build -o ./bin/server-linux cmd/server/main.go

# Build the example stdio server
build-example:
	cd examples && go build -o mcp-example mcp_stdio_example.go

# Run the server in stdio mode
run-stdio: build
	TRANSPORT_MODE=stdio ./bin/server

# Run the server in SSE mode
run-sse: clean build
	TRANSPORT_MODE=sse ./bin/server -t sse -p 9090 -h 127.0.0.1 -c config.json

# Build and run the example client
client:
	go build -o mcp-client examples/client/client.go
	./mcp-client

# Build and run the simple client (no SSE dependency)
client-simple:
	go build -o mcp-simple-client examples/client/simple_client.go
	./mcp-simple-client

# Run the test script
test-script:
	./examples/test_script.sh

# Run tests
test:
	go test ./... -race -cover -count=1

# Clean build artifacts
clean:
	rm -f server server-linux mcp-client mcp-simple-client
	# lsof -i :9090 | grep LISTEN | awk '{print $2}' | xargs kill -9

# Run linter
lint:
	golangci-lint run ./...

# Setup
setup:
	go mod tidy
	go mod download

# Docker targets
docker-build:
	docker build -t db-mcp-server:latest .

# Build Docker image for local platform only (no push)
docker-build-local:
	docker build -t db-mcp-server:local .

# Build multi-platform Docker image without pushing (for testing)
# Usage: make docker-build-multiarch VERSION=v1.6.3
# VERSION: Specify version tag (default: v1.6.3)
docker-build-multiarch:
	@echo "Building multi-architecture Docker image locally (linux/amd64,linux/arm64)..."
	@VERSION=$${VERSION:-v1.6.3}; \
	echo "Version: $$VERSION"; \
	docker buildx create --name multiplatform-builder --use || true; \
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t freepeak/db-mcp-server:$$VERSION \
		--load .; \
	docker buildx rm multiplatform-builder

# Run the Docker container in SSE mode
docker-run:
	docker run -d --name db-mcp-server -p 9092:9092 -v $(PWD)/config.json:/app/config.json -v $(PWD)/logs:/app/logs db-mcp-server:${TAG:-latest}

# Run Docker container in STDIO mode (for debugging)
docker-run-stdio:
	docker run -it --rm --name db-mcp-server-stdio -v $(PWD)/config.json:/app/config.json -e TRANSPORT_MODE=stdio db-mcp-server:${TAG:-latest}

# Stop and remove the Docker container
docker-stop:
	docker stop db-mcp-server || true
	docker rm db-mcp-server || true

# Pull the latest multi-platform Docker image with the correct architecture for your system
# This is useful when switching between different architectures (AMD64/ARM64)
docker-pull-platform:
	@echo "Pulling the latest Docker image with the correct platform for your system..."
	docker pull --platform $${DOCKER_PLATFORM:-linux/amd64} freepeak/db-mcp-server:latest

# Build and deploy Docker image for current architecture only
# Usage: make deploy-docker-simple VERSION=v1.6.3 LATEST=true
# VERSION: Specify version tag (default: v1.6.3)
# LATEST: Whether to also tag as latest (default: true)
deploy-docker-simple:
	@echo "Building Docker image for current architecture..."
	@VERSION=$${VERSION:-v1.6.3}; \
	LATEST=$${LATEST:-true}; \
	echo "Version: $$VERSION | Tag as latest: $$LATEST"; \
	docker build -t freepeak/db-mcp-server:$$VERSION .; \
	if [ "$$LATEST" = "true" ]; then \
		docker tag freepeak/db-mcp-server:$$VERSION freepeak/db-mcp-server:latest; \
	fi; \
	echo "To push to Docker Hub, run:"; \
	echo "docker push freepeak/db-mcp-server:$$VERSION"; \
	if [ "$$LATEST" = "true" ]; then \
		echo "docker push freepeak/db-mcp-server:latest"; \
	fi

# Build and deploy multi-platform Docker image (AMD64 and ARM64)
# Requires Docker Buildx: https://docs.docker.com/buildx/working-with-buildx/
# Usage: make deploy-docker VERSION=v1.6.3 LATEST=true
# VERSION: Specify version tag (default: v1.6.3)
# LATEST: Whether to also tag as latest (default: true)
deploy-docker:
	@echo "Building multi-architecture Docker image (linux/amd64,linux/arm64)..."
	@VERSION=$${VERSION:-v1.6.3}; \
	LATEST=$${LATEST:-true}; \
	echo "Version: $$VERSION | Tag as latest: $$LATEST"; \
	if ! command -v docker buildx &> /dev/null; then \
		echo "Error: Docker Buildx is not available."; \
		echo "Please install it first: https://docs.docker.com/buildx/working-with-buildx/"; \
		echo "Or use 'make deploy-docker-simple' instead for single-architecture builds."; \
		exit 1; \
	fi; \
	docker buildx create --name multiplatform-builder --use || true; \
	if [ "$$LATEST" = "true" ]; then \
		docker buildx build --platform linux/amd64,linux/arm64 \
			-t freepeak/db-mcp-server:$$VERSION \
			-t freepeak/db-mcp-server:latest \
			--push .; \
	else \
		docker buildx build --platform linux/amd64,linux/arm64 \
			-t freepeak/db-mcp-server:$$VERSION \
			--push .; \
	fi; \
	docker buildx rm multiplatform-builder || true

# Default target
all: build 

