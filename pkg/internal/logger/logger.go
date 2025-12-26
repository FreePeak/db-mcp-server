// Package logger provides a wrapper around the internal logger for external package use.
package logger

import (
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	logger.Debug(format, v...)
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	logger.Info(format, v...)
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	logger.Warn(format, v...)
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	logger.Error(format, v...)
}

// ErrorWithStack logs an error with a stack trace
func ErrorWithStack(err error) {
	logger.ErrorWithStack(err)
}

// Initialize initializes the logger with specified level
func Initialize(level string) {
	logger.Initialize(logger.Config{Level: level})
}
