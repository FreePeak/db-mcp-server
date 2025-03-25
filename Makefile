.PHONY: build run-stdio run-sse clean test client client-simple test-script

# Build the server
build:
	go build -o mcp-server cmd/server/main.go
	GOOS=linux GOARCH=amd64 go build -o server cmd/server/main.go

# Run the server in stdio mode
run-stdio: build
	./mcp-server -t stdio

# Run the server in SSE mode
run-sse: clean build
	./mcp-server -t sse -port 9090

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
	go test ./...

# Clean build artifacts
clean:
	rm -f mcp-server mcp-client mcp-simple-client
	# lsof -i :9090 | grep LISTEN | awk '{print $2}' | xargs kill -9

# Default target
all: build 