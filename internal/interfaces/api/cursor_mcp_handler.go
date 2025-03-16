package api

import (
	"encoding/json"
	"fmt"
	"log"
	"mcpserver/internal/domain/entities"
	"mcpserver/internal/usecase"
	"net/http"
)

// CursorMCPHandler handles Cursor MCP protocol requests over SSE
type CursorMCPHandler struct {
	mcpUseCase *usecase.CursorMCPUseCase
}

// NewCursorMCPHandler creates a new Cursor MCP handler
func NewCursorMCPHandler(mcpUseCase *usecase.CursorMCPUseCase) *CursorMCPHandler {
	return &CursorMCPHandler{
		mcpUseCase: mcpUseCase,
	}
}

// ServeHTTP handles HTTP requests for Cursor MCP
func (h *CursorMCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow cross-origin requests

	// Create a channel for events
	eventChan := make(chan *entities.MCPEvent)

	// Send initial tools event
	go func() {
		toolsEvent, err := h.mcpUseCase.GetToolsEvent(r.Context())
		if err != nil {
			log.Printf("Error getting tools: %v", err)
			eventChan <- createErrorEvent(err)
			return
		}
		eventChan <- toolsEvent
	}()

	// Listen for tool requests on a separate goroutine
	go h.handleToolRequests(r, eventChan)

	// Write events to response
	for {
		select {
		case event := <-eventChan:
			if err := h.writeEvent(w, event); err != nil {
				log.Printf("Error writing event: %v", err)
				return
			}
		case <-r.Context().Done():
			log.Println("Client disconnected")
			return
		}
	}
}

// handleToolRequests handles tool requests from the Cursor client
func (h *CursorMCPHandler) handleToolRequests(r *http.Request, eventChan chan *entities.MCPEvent) {
	// Check for POST data in the request body
	if r.Method == http.MethodPost {
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()

		// Handle as a tool request
		var toolRequest entities.MCPToolRequest
		if err := decoder.Decode(&toolRequest); err != nil {
			log.Printf("Error decoding tool request: %v", err)
			eventChan <- createErrorEvent(err)
			return
		}

		// Execute the tool
		responseEvent, err := h.mcpUseCase.ExecuteTool(r.Context(), &toolRequest)
		if err != nil {
			log.Printf("Error executing tool: %v", err)
			eventChan <- createErrorEvent(err)
			return
		}

		// Send the response
		eventChan <- responseEvent
	}
}

// writeEvent writes an event to the response
func (h *CursorMCPHandler) writeEvent(w http.ResponseWriter, event *entities.MCPEvent) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error marshaling event: %w", err)
	}

	// Write the event to the response
	fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

// createErrorEvent creates an error event
func createErrorEvent(err error) *entities.MCPEvent {
	return &entities.MCPEvent{
		Type: "error",
		Payload: map[string]string{
			"error": err.Error(),
		},
	}
}
