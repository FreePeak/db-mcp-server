// Package logger provides logging utilities for the public API of the db-mcp-server.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	intLogger "github.com/FreePeak/db-mcp-server/internal/logger"
)

var (
	initialized bool
	level       string
	logFile     *os.File
)

// Initialize sets up the logger with the specified level
func Initialize(logLevel string) {
	level = logLevel

	// If in stdio mode, redirect logs to a file
	if os.Getenv("TRANSPORT_MODE") == "stdio" {
		// Create logs directory if it doesn't exist
		logsDir := "logs"
		if _, err := os.Stat(logsDir); os.IsNotExist(err) {
			if err := os.Mkdir(logsDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
			}
		}

		// Create log file with timestamp
		timestamp := time.Now().Format("20060102-150405")
		logFilePath := filepath.Join(logsDir, fmt.Sprintf("pkg-logger-%s.log", timestamp))

		// Try to open the log file
		var err error
		logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Last message to stderr before giving up
			fmt.Fprintf(os.Stderr, "Failed to create pkg logger file: %v\n", err)
		}
	}

	initialized = true
}

// ensureInitialized makes sure the logger is initialized
func ensureInitialized() {
	if !initialized {
		// Default to info level
		Initialize("info")
	}
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	ensureInitialized()
	if !shouldLog("debug") {
		return
	}
	logMessage("DEBUG", format, v...)
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	ensureInitialized()
	if !shouldLog("info") {
		return
	}
	logMessage("INFO", format, v...)
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	ensureInitialized()
	if !shouldLog("warn") {
		return
	}
	logMessage("WARN", format, v...)
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	ensureInitialized()
	if !shouldLog("error") {
		return
	}
	logMessage("ERROR", format, v...)
}

// shouldLog determines if we should log a message based on the level
func shouldLog(msgLevel string) bool {
	// Always try to use the internal logger first as it's more sophisticated
	// and handles stdio mode properly
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	currentLevel := levels[strings.ToLower(level)]
	messageLevel := levels[strings.ToLower(msgLevel)]

	return messageLevel >= currentLevel
}

// logMessage sends a log message to the appropriate destination
func logMessage(level string, format string, v ...interface{}) {
	// Forward to the internal logger if possible
	message := fmt.Sprintf(format, v...)

	// If we're in stdio mode, avoid stdout completely
	if os.Getenv("TRANSPORT_MODE") == "stdio" {
		if logFile != nil {
			// Format the message with timestamp
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			formattedMsg := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)

			// Write to log file directly
			if _, err := logFile.WriteString(formattedMsg); err != nil {
				// We can't use stdout since we're in stdio mode, so we have to suppress this error
				// or write to stderr as a last resort
				fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
			}
		}
		return
	}

	// For non-stdio mode or if file writing failed
	switch strings.ToUpper(level) {
	case "DEBUG":
		intLogger.Debug(message)
	case "INFO":
		intLogger.Info(message)
	case "WARN":
		intLogger.Warn(message)
	case "ERROR":
		intLogger.Error(message)
	}
}
