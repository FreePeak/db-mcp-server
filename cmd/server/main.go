package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mcpserver/internal/config"
	"mcpserver/internal/logger"
	"mcpserver/internal/mcp"
	"mcpserver/internal/session"
	"mcpserver/internal/transport"
	"mcpserver/pkg/tools"
)

func main() {
	// Initialize random number generator
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Parse command line flags
	transportMode := flag.String("t", "", "Transport mode (sse or stdio)")
	port := flag.Int("port", 0, "Server port")
	flag.Parse()

	// Load configuration
	cfg := config.LoadConfig()

	// Override config with command line flags if provided
	if *transportMode != "" {
		cfg.TransportMode = *transportMode
	}
	if *port != 0 {
		cfg.ServerPort = *port
	}

	// Initialize logger
	logger.Initialize(cfg.LogLevel)
	logger.Info("Starting MCP server with %s transport on port %d", cfg.TransportMode, cfg.ServerPort)

	// Create session manager
	sessionManager := session.NewManager()

	// Start session cleanup goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			sessionManager.CleanupSessions(30 * time.Minute)
		}
	}()

	// Create MCP handler
	mcpHandler := mcp.NewHandler()

	// Register some example tools
	registerExampleTools(mcpHandler)

	// Create and configure the server based on transport mode
	switch cfg.TransportMode {
	case "sse":
		startSSEServer(cfg, sessionManager, mcpHandler)
	case "stdio":
		logger.Info("stdio transport not implemented yet")
		os.Exit(1)
	default:
		logger.Error("Unknown transport mode: %s", cfg.TransportMode)
		os.Exit(1)
	}
}

func startSSEServer(cfg *config.Config, sessionManager *session.Manager, mcpHandler *mcp.Handler) {
	// Create SSE transport
	basePath := fmt.Sprintf("http://localhost:%d", cfg.ServerPort)
	sseTransport := transport.NewSSETransport(sessionManager, basePath)

	// Register method handlers
	methodHandlers := mcpHandler.GetAllMethodHandlers()
	for method, handler := range methodHandlers {
		sseTransport.RegisterMethodHandler(method, handler)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Register SSE endpoint
	mux.HandleFunc("/sse", sseTransport.HandleSSE)

	// Register message endpoint
	mux.HandleFunc("/message", sseTransport.HandleMessage)

	// Create server
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// Shutdown server gracefully
	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error: %v", err)
	}

	logger.Info("Server stopped")
}

func registerExampleTools(mcpHandler *mcp.Handler) {
	// Example echo tool
	echoTool := &tools.Tool{
		Name:        "echo",
		Description: "Echoes back the input",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Message to echo",
				},
			},
			"required": []string{"message"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			message, ok := params["message"].(string)
			if !ok {
				return nil, fmt.Errorf("message must be a string")
			}
			return map[string]interface{}{
				"message": message,
			}, nil
		},
	}

	// Calculator tool
	calculatorTool := &tools.Tool{
		Name:        "calculator",
		Description: "Performs basic mathematical operations",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform (add, subtract, multiply, divide)",
					"enum":        []string{"add", "subtract", "multiply", "divide"},
				},
				"a": map[string]interface{}{
					"type":        "number",
					"description": "First number",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "Second number",
				},
			},
			"required": []string{"operation", "a", "b"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			operation, ok := params["operation"].(string)
			if !ok {
				return nil, fmt.Errorf("operation must be a string")
			}

			a, aOk := params["a"].(float64)
			b, bOk := params["b"].(float64)
			if !aOk || !bOk {
				return nil, fmt.Errorf("a and b must be numbers")
			}

			var result float64
			switch operation {
			case "add":
				result = a + b
			case "subtract":
				result = a - b
			case "multiply":
				result = a * b
			case "divide":
				if b == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				result = a / b
			default:
				return nil, fmt.Errorf("unknown operation: %s", operation)
			}

			return map[string]interface{}{
				"result": result,
			}, nil
		},
	}

	// Timestamp tool
	timestampTool := &tools.Tool{
		Name:        "timestamp",
		Description: "Returns the current timestamp in various formats",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Format of the timestamp (unix, rfc3339, or custom Go time format)",
					"default":     "rfc3339",
				},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			format, ok := params["format"].(string)
			if !ok {
				format = "rfc3339"
			}

			now := time.Now()
			var result string

			switch format {
			case "unix":
				result = fmt.Sprintf("%d", now.Unix())
			case "rfc3339":
				result = now.Format(time.RFC3339)
			default:
				// Try to use the format as a Go time format
				result = now.Format(format)
			}

			return map[string]interface{}{
				"timestamp": result,
			}, nil
		},
	}

	// Random number generator tool
	randomTool := &tools.Tool{
		Name:        "random",
		Description: "Generates random numbers",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"min": map[string]interface{}{
					"type":        "integer",
					"description": "Minimum value (inclusive)",
					"default":     0,
				},
				"max": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum value (exclusive)",
					"default":     100,
				},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			min := 0
			max := 100

			if minParam, ok := params["min"].(float64); ok {
				min = int(minParam)
			}

			if maxParam, ok := params["max"].(float64); ok {
				max = int(maxParam)
			}

			if min >= max {
				return nil, fmt.Errorf("min must be less than max")
			}

			// Generate a random number between min and max
			// Note: This uses a pseudorandom number and isn't cryptographically secure
			result := min + rand.Intn(max-min)

			return map[string]interface{}{
				"value": result,
			}, nil
		},
	}

	// Text tool for string operations
	textTool := &tools.Tool{
		Name:        "text",
		Description: "Performs various text operations",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform (upper, lower, reverse, count)",
					"enum":        []string{"upper", "lower", "reverse", "count"},
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "The text to process",
				},
			},
			"required": []string{"operation", "text"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			operation, ok := params["operation"].(string)
			if !ok {
				return nil, fmt.Errorf("operation must be a string")
			}

			text, ok := params["text"].(string)
			if !ok {
				return nil, fmt.Errorf("text must be a string")
			}

			var result interface{}
			switch operation {
			case "upper":
				result = strings.ToUpper(text)
			case "lower":
				result = strings.ToLower(text)
			case "reverse":
				// Reverse the string
				runes := []rune(text)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				result = string(runes)
			case "count":
				// Count characters and words
				words := len(strings.Fields(text))
				chars := len(text)
				result = map[string]int{
					"characters": chars,
					"words":      words,
				}
			default:
				return nil, fmt.Errorf("unknown operation: %s", operation)
			}

			return map[string]interface{}{
				"result": result,
			}, nil
		},
	}

	// Register tools
	logger.Info("Registering tools:")
	logger.Info("- echo: Simple echo tool")
	mcpHandler.RegisterTool(echoTool)

	logger.Info("- calculator: Mathematical operations tool")
	mcpHandler.RegisterTool(calculatorTool)

	logger.Info("- timestamp: Timestamp formatting tool")
	mcpHandler.RegisterTool(timestampTool)

	logger.Info("- random: Random number generator")
	mcpHandler.RegisterTool(randomTool)

	logger.Info("- text: Text manipulation tool")
	mcpHandler.RegisterTool(textTool)

	logger.Info("Total tools registered: 5")
}
