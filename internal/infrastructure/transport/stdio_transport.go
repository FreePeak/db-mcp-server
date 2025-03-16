package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mcpserver/internal/domain/entities"
	"os"
	"strings"
	"sync"
)

// StdioTransport implements the transport repository interface for stdio
type StdioTransport struct {
	eventChan   chan interface{}
	requestChan chan interface{}
	errorChan   chan error
	reader      *bufio.Reader
	writer      io.Writer
	mu          sync.Mutex
	started     bool
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport() *StdioTransport {
	// Set log output to stderr for all logging to avoid corrupting stdout JSON
	log.SetOutput(os.Stderr)

	return &StdioTransport{
		eventChan:   make(chan interface{}),
		requestChan: make(chan interface{}),
		errorChan:   make(chan error),
		reader:      bufio.NewReader(os.Stdin),
		writer:      os.Stdout,
		started:     false,
	}
}

// Start starts the stdio transport
func (t *StdioTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return fmt.Errorf("transport already started")
	}

	// Log to stderr only
	fmt.Fprintln(os.Stderr, "Starting stdio transport...")

	// Start goroutine to handle outgoing events (writing to stdout)
	go t.handleEvents(ctx)

	// Start goroutine to handle incoming requests (reading from stdin)
	go t.handleRequests(ctx)

	t.started = true
	return nil
}

// Stop stops the stdio transport
func (t *StdioTransport) Stop(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.started {
		return nil
	}

	// Log to stderr only
	fmt.Fprintln(os.Stderr, "Stopping stdio transport...")

	close(t.eventChan)
	close(t.requestChan)
	close(t.errorChan)

	t.started = false
	return nil
}

// Send sends an event to the client (legacy method)
func (t *StdioTransport) Send(event interface{}) error {
	t.mu.Lock()
	if !t.started {
		t.mu.Unlock()
		return fmt.Errorf("transport not started")
	}
	t.mu.Unlock()

	t.eventChan <- event
	return nil
}

// SendRaw sends a raw JSON string to the client
func (t *StdioTransport) SendRaw(jsonStr string) error {
	t.mu.Lock()
	if !t.started {
		t.mu.Unlock()
		return fmt.Errorf("transport not started")
	}
	t.mu.Unlock()

	// Write directly to stdout without additional processing
	_, err := fmt.Fprintln(t.writer, jsonStr)
	if err != nil {
		return fmt.Errorf("error writing raw JSON: %w", err)
	}

	// Log to stderr for debugging
	fmt.Fprintf(os.Stderr, "Sent raw JSON: %s\n", jsonStr)
	return nil
}

// Receive receives events from the client
func (t *StdioTransport) Receive() (<-chan interface{}, <-chan error) {
	return t.requestChan, t.errorChan
}

// handleEvents writes events to stdout
func (t *StdioTransport) handleEvents(ctx context.Context) {
	for {
		select {
		case event, ok := <-t.eventChan:
			if !ok {
				// Channel closed
				return
			}
			if err := t.writeEvent(event); err != nil {
				// Log to stderr so it doesn't interfere with protocol
				fmt.Fprintf(os.Stderr, "Error writing event: %v\n", err)
				t.errorChan <- err
			}
		case <-ctx.Done():
			// Log to stderr so it doesn't interfere with protocol
			fmt.Fprintln(os.Stderr, "Context done, stopping stdio events handler")
			return
		}
	}
}

// handleRequests reads requests from stdin
func (t *StdioTransport) handleRequests(ctx context.Context) {
	// Log to stderr so it doesn't interfere with protocol
	fmt.Fprintln(os.Stderr, "Started reading requests from stdin...")

	for {
		select {
		case <-ctx.Done():
			// Log to stderr so it doesn't interfere with protocol
			fmt.Fprintln(os.Stderr, "Context done, stopping request handler")
			return
		default:
			// Read a line from stdin
			line, err := t.reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// EOF means stdin was closed, which is a normal shutdown
					// Log to stderr so it doesn't interfere with protocol
					fmt.Fprintln(os.Stderr, "EOF received, closing request handler")
					return
				}
				// Log to stderr so it doesn't interfere with protocol
				fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
				t.errorChan <- fmt.Errorf("error reading from stdin: %w", err)
				continue
			}

			// Trim any whitespace (including newlines) from the input
			line = strings.TrimSpace(line)

			// Skip empty lines
			if line == "" {
				continue
			}

			// Log to stderr so it doesn't interfere with protocol
			fmt.Fprintf(os.Stderr, "Received request: %s\n", line)

			// Parse the request as a JSON-RPC 2.0 message
			var request entities.MCPToolRequest
			if err := json.Unmarshal([]byte(line), &request); err != nil {
				// Log to stderr so it doesn't interfere with protocol
				fmt.Fprintf(os.Stderr, "Error parsing request: %v, input: %s\n", err, line)
				t.errorChan <- fmt.Errorf("error parsing request: %w", err)

				// Send a properly formatted JSON-RPC error response
				errorResponse := &entities.MCPToolResponse{
					JsonRPC: entities.JSONRPCVersion,
					ID:      "null", // We don't know the ID
					Error: &entities.MCPError{
						Code:    entities.ErrorCodeParseError,
						Message: fmt.Sprintf("Invalid JSON: %v", err),
					},
				}
				errorJSON, _ := json.Marshal(errorResponse)
				t.SendRaw(string(errorJSON))
				continue
			}

			// Log to stderr so it doesn't interfere with protocol
			fmt.Fprintf(os.Stderr, "Parsed tool request: %s\n", request.Method)

			// Validate JSON-RPC 2.0 format
			if request.JsonRPC != entities.JSONRPCVersion {
				fmt.Fprintf(os.Stderr, "Invalid JSON-RPC version: %s\n", request.JsonRPC)
				errorResponse := &entities.MCPToolResponse{
					JsonRPC: entities.JSONRPCVersion,
					ID:      request.ID,
					Error: &entities.MCPError{
						Code:    entities.ErrorCodeInvalidRequest,
						Message: fmt.Sprintf("Invalid JSON-RPC version, expected %s", entities.JSONRPCVersion),
					},
				}
				errorJSON, _ := json.Marshal(errorResponse)
				t.SendRaw(string(errorJSON))
				continue
			}

			// Send the request to the channel
			t.requestChan <- &request
		}
	}
}

// validateToolsEvent validates the format of a tools event to ensure it matches
// what Cursor expects. This is strictly for debugging purposes.
func validateToolsEvent(toolsEvent *entities.MCPToolsEvent) error {
	if toolsEvent.JsonRPC != entities.JSONRPCVersion {
		return fmt.Errorf("incorrect jsonrpc version: %s, expected: %s", toolsEvent.JsonRPC, entities.JSONRPCVersion)
	}

	if toolsEvent.Method != entities.MethodToolsList {
		return fmt.Errorf("incorrect method: %s, expected: %s", toolsEvent.Method, entities.MethodToolsList)
	}

	if len(toolsEvent.Result.Tools) == 0 {
		return fmt.Errorf("no tools defined in the event")
	}

	// Check each tool for correct format
	for i, tool := range toolsEvent.Result.Tools {
		if tool.Name == "" {
			return fmt.Errorf("tool at index %d has no name", i)
		}

		// Check that InputSchema is defined
		if tool.InputSchema == nil {
			return fmt.Errorf("tool '%s' has no input schema defined", tool.Name)
		}
	}

	return nil
}

// writeEvent marshals and writes an event to the writer
func (t *StdioTransport) writeEvent(event interface{}) error {
	var jsonBytes []byte
	var err error

	// Log type of event for debugging
	eventType := fmt.Sprintf("%T", event)
	fmt.Fprintf(os.Stderr, "Writing event of type: %s\n", eventType)

	// Special handling for tools event to ensure correct format
	if toolsEvent, ok := event.(*entities.MCPToolsEvent); ok {
		// Log for debugging
		fmt.Fprintf(os.Stderr, "Processing tools event with %d tools\n", len(toolsEvent.Result.Tools))

		// Validate the tools event format
		if err := validateToolsEvent(toolsEvent); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Invalid tools event format: %v\n", err)
			// Continue anyway, but log the warning
		}

		// Ensure the event is properly formatted for Cursor
		// The key issue is making sure the "params" field has a "tools" array
		jsonBytes, err = json.Marshal(toolsEvent)
		if err != nil {
			return fmt.Errorf("error marshaling tools event: %w", err)
		}

		// Log the JSON for debugging
		fmt.Fprintf(os.Stderr, "Tools event JSON: %s\n", string(jsonBytes))
	} else {
		// For other event types
		jsonBytes, err = json.Marshal(event)
		if err != nil {
			return fmt.Errorf("error marshaling event: %w", err)
		}
	}

	// Write the JSON to stdout
	if _, err := fmt.Fprintln(t.writer, string(jsonBytes)); err != nil {
		return fmt.Errorf("error writing event: %w", err)
	}

	return nil
}
