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
FROM alpine:latest

# Add necessary runtime packages and network diagnostic tools
RUN apk add --no-cache ca-certificates tzdata bash netcat-openbsd bind-tools iputils busybox-extras

# Set the working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/bin/server /app/server

# Copy default config file
COPY config.json /app/config.json

# Create data and logs directories
RUN mkdir -p /app/data /app/logs

# Set environment variables
ENV SERVER_PORT=9092
ENV TRANSPORT_MODE=sse
ENV CONFIG_PATH=/app/config.json

# Expose server port
EXPOSE 9092

# Provide a volume for logs only
VOLUME ["/app/logs"]

# Start the MCP server with proper configuration
CMD ["/bin/bash", "-c", "/app/server -t ${TRANSPORT_MODE} -p ${SERVER_PORT} -c ${CONFIG_PATH}"]

# You can override the port by passing it as a command-line argument
# docker run -p 8080:8080 db-mcp-server -port 8080 