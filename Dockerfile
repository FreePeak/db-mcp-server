FROM golang:1.24-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache make gcc musl-dev

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire project
COPY . .

# Build the application
RUN make build

# Create a smaller production image
FROM alpine:3.22

# Install only the runtime packages needed by the server and health check.
RUN apk add --no-cache ca-certificates tzdata bash curl

# Set the working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/bin/server /app/server

# Create an unprivileged runtime user and writable application directories.
# The production image intentionally does not embed config.json; mount a
# reviewed config or use CONFIG_PATH/secret references at runtime.
RUN addgroup -S app && adduser -S -G app app \
    && mkdir -p /app/data /app/logs \
    && chown -R app:app /app

# OCI image metadata (improves Glama MCP directory listing quality)
LABEL org.opencontainers.image.title="DB MCP Server" \
      org.opencontainers.image.description="Multi-database Model Context Protocol server (MySQL, PostgreSQL, SQLite, Oracle, TimescaleDB)" \
      org.opencontainers.image.source="https://github.com/FreePeak/db-mcp-server" \
      org.opencontainers.image.licenses="MIT"

# Set environment variables
ENV SERVER_PORT=9092
ENV TRANSPORT_MODE=sse
ENV CONFIG_PATH=/app/config.json
ENV HOME=/app

# Expose the MCP port and the separate health port.
EXPOSE 9092 9093

# Provide a volume for logs only
VOLUME ["/app/logs"]

# Healthcheck for container orchestrators. The runtime serves /health on 9093.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fsS "http://localhost:9093/health" || exit 1

# Run as the unprivileged application user.
USER app

# Legacy SSE is retained only for this development image. Shared deployments
# must use STDIO or a real authenticated Streamable HTTP endpoint.
CMD ["/bin/bash", "-c", "/app/server -t ${TRANSPORT_MODE} -h 0.0.0.0 -allow-insecure-sse -p ${SERVER_PORT} -c ${CONFIG_PATH}"]