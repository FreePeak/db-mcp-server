// Package jsonrpc provides JSON-RPC 2.0 protocol implementation for the MCP server.
package jsonrpc

import (
	"fmt"
)

// Version is the JSON-RPC version string
const Version = "2.0"

// Request represents a JSON-RPC request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// IsNotification returns true if the request is a notification (has no ID)
func (r *Request) IsNotification() bool {
	return r.ID == nil
}

// Response represents a JSON-RPC response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard error codes
const (
	ParseErrorCode     = -32700
	InvalidRequestCode = -32600
	MethodNotFoundCode = -32601
	InvalidParamsCode  = -32602
	InternalErrorCode  = -32603
)

// Error returns a string representation of the error
func (e *Error) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// NewResponse creates a new response for the given request
func NewResponse(req *Request, result interface{}, err *Error) *Response {
	resp := &Response{
		JSONRPC: Version,
		ID:      req.ID,
	}

	if err != nil {
		resp.Error = err
	} else {
		resp.Result = result
	}

	return resp
}

// NewError creates a new Error with the given code and message
func NewError(code int, message string, data interface{}) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// ParseError creates a Parse Error
func ParseError(data interface{}) *Error {
	return &Error{
		Code:    ParseErrorCode,
		Message: "Parse error",
		Data:    data,
	}
}

// InvalidRequestError creates an Invalid Request error
func InvalidRequestError(data interface{}) *Error {
	return &Error{
		Code:    InvalidRequestCode,
		Message: "Invalid request",
		Data:    data,
	}
}

// MethodNotFoundError creates a Method Not Found error
func MethodNotFoundError(method string) *Error {
	return &Error{
		Code:    MethodNotFoundCode,
		Message: "Method not found",
		Data:    method,
	}
}

// InvalidParamsError creates an Invalid Params error
func InvalidParamsError(data interface{}) *Error {
	return &Error{
		Code:    InvalidParamsCode,
		Message: "Invalid params",
		Data:    data,
	}
}

// InternalError creates an Internal Error
func InternalError(data interface{}) *Error {
	return &Error{
		Code:    InternalErrorCode,
		Message: "Internal error",
		Data:    data,
	}
}
