package mcp

import (
	"fmt"
)

// TextContent represents a text content item in a response
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Response is a standardized response format for MCP tools
type Response struct {
	Content  []TextContent          `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewResponse creates a new empty Response
func NewResponse() *Response {
	return &Response{
		Content: make([]TextContent, 0),
	}
}

// WithText adds a text content item to the response
func (r *Response) WithText(text string) *Response {
	r.Content = append(r.Content, TextContent{
		Type: "text",
		Text: text,
	})
	return r
}

// WithMetadata adds metadata to the response
func (r *Response) WithMetadata(key string, value interface{}) *Response {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
	return r
}

// FromString creates a response from a string
func FromString(text string) *Response {
	return NewResponse().WithText(text)
}

// FromError creates an error response
func FromError(err error) (interface{}, error) {
	return nil, err
}

// FormatResponse converts any response type to a properly formatted MCP response
func FormatResponse(response interface{}, err error) (interface{}, error) {
	if err != nil {
		// Already formatted as JSON-RPC error
		return response, err
	}

	// For nil responses, return empty object to avoid null result
	if response == nil {
		return NewResponse(), nil
	}

	// If response is already an Response, return it
	if mcpResp, ok := response.(*Response); ok {
		// If content is empty, return a new empty response to ensure consistency
		if len(mcpResp.Content) == 0 {
			return NewResponse(), nil
		}
		return mcpResp, nil
	}

	// Handle string responses, checking for empty strings
	if strResponse, ok := response.(string); ok {
		if strResponse == "" || strResponse == "[]" {
			return NewResponse(), nil
		}
		return FromString(strResponse), nil
	}

	// If response is already properly formatted with content as an array
	if respMap, ok := response.(map[string]interface{}); ok {
		// If the map is empty, return a new empty response
		if len(respMap) == 0 {
			return NewResponse(), nil
		}

		if content, exists := respMap["content"]; exists {
			// Accept both []interface{} (from JSON-decoded input) and
			// []map[string]interface{} (typical handler return type).
			// Without this, handler maps fall through to the default
			// fmt.Sprintf("%v", response) branch and the MCP envelope
			// gets Go-stringified into the outer text content.
			switch contentSlice := content.(type) {
			case []interface{}:
				if len(contentSlice) == 0 {
					return NewResponse(), nil
				}
				return respMap, nil
			case []map[string]interface{}:
				if len(contentSlice) == 0 {
					return NewResponse(), nil
				}
				return respMap, nil
			}
		}

		// If it has a metadata field but not a properly formatted content field
		if _, hasContent := respMap["content"]; !hasContent {
			newResp := NewResponse().WithText(fmt.Sprintf("%v", respMap))

			// Copy over metadata if it exists
			if metadata, hasMetadata := respMap["metadata"]; hasMetadata {
				if metadataMap, ok := metadata.(map[string]interface{}); ok {
					for k, v := range metadataMap {
						newResp.WithMetadata(k, v)
					}
				}
			}

			return newResp, nil
		}
	}

	// For any other type, convert to string and wrap in proper content format
	return FromString(fmt.Sprintf("%v", response)), nil
}
