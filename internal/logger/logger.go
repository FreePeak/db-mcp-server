package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level represents the severity of a log message
type Level int

const (
	// LevelDebug for detailed troubleshooting
	LevelDebug Level = iota
	// LevelInfo for general operational entries
	LevelInfo
	// LevelWarn for non-critical issues
	LevelWarn
	// LevelError for errors that should be addressed
	LevelError
)

var (
	// Default logger
	zapLogger *zap.Logger
	logLevel  Level
	// Flag to indicate if we're in stdio mode
	isStdioMode bool
	// Log file for stdio mode
	stdioLogFile *os.File
	// Mutex to protect log file access
	logMutex sync.Mutex
)

// Config represents logger configuration
type Config struct {
	Level  string // Log level (debug, info, warn, error)
	LogDir string // Directory for log files (optional, defaults to ./logs)
}

// safeStdioWriter is a writer that ensures no output goes to stdout in stdio mode
type safeStdioWriter struct {
	file *os.File
}

// Write implements io.Writer and filters all output in stdio mode
func (w *safeStdioWriter) Write(p []byte) (n int, err error) {
	// In stdio mode, write to the log file instead of stdout
	logMutex.Lock()
	defer logMutex.Unlock()

	if stdioLogFile != nil {
		return stdioLogFile.Write(p)
	}

	// Last resort: write to stderr, never stdout
	return os.Stderr.Write(p)
}

// Sync implements zapcore.WriteSyncer
func (w *safeStdioWriter) Sync() error {
	logMutex.Lock()
	defer logMutex.Unlock()

	if stdioLogFile != nil {
		return stdioLogFile.Sync()
	}
	return nil
}

// Initialize sets up the logger with the specified configuration
func Initialize(cfg Config) {
	setLogLevel(cfg.Level)

	// Check if we're in stdio mode
	transportMode := os.Getenv("TRANSPORT_MODE")
	isStdioMode = transportMode == "stdio"

	if isStdioMode {
		// In stdio mode, we need to avoid ANY JSON output to stdout

		// Use provided log directory or default to "logs" in current directory
		logsDir := cfg.LogDir
		if logsDir == "" {
			logsDir = "logs"
		}

		// Create log directory if it doesn't exist
		if _, err := os.Stat(logsDir); os.IsNotExist(err) {
			if err := os.MkdirAll(logsDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
			}
		}

		timestamp := time.Now().Format("20060102-150405")
		logFileName := filepath.Join(logsDir, fmt.Sprintf("mcp-logger-%s.log", timestamp))

		// Try to create the log file
		var err error
		stdioLogFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// If we can't create a log file, we'll use a null logger
			fmt.Fprintf(os.Stderr, "Failed to create log file: %v - all logs will be suppressed\n", err)
		} else {
			// Write initial log message to stderr only (as a last message before full redirection)
			fmt.Fprintf(os.Stderr, "Stdio mode detected - all logs redirected to: %s\n", logFileName)

			// Create a custom writer that never writes to stdout
			safeWriter := &safeStdioWriter{file: stdioLogFile}

			// Create a development encoder for more readable logs
			encoderConfig := zap.NewDevelopmentEncoderConfig()
			encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			encoder := zapcore.NewConsoleEncoder(encoderConfig)

			// Create core that writes to our safe writer
			core := zapcore.NewCore(encoder, zapcore.AddSync(safeWriter), getZapLevel(logLevel))

			// Create the logger with the core
			zapLogger = zap.New(core)
			return
		}
	}

	// Standard logger initialization for non-stdio mode or fallback
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// In stdio mode with no log file, use a no-op logger to avoid any stdout output
	if isStdioMode {
		zapLogger = zap.NewNop()
		return
	} else {
		config.OutputPaths = []string{"stdout"}
	}

	config.Level = getZapLevel(logLevel)

	var err error
	zapLogger, err = config.Build()
	if err != nil {
		// If Zap logger cannot be built, fall back to noop logger
		zapLogger = zap.NewNop()
	}
}

// InitializeWithWriter sets up the logger with the specified level and output writer
func InitializeWithWriter(level string, writer *os.File) {
	setLogLevel(level)

	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create custom core with the provided writer
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(writer),
		getZapLevel(logLevel),
	)

	zapLogger = zap.New(core)
}

// setLogLevel sets the log level from a string
func setLogLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		logLevel = LevelDebug
	case "info":
		logLevel = LevelInfo
	case "warn":
		logLevel = LevelWarn
	case "error":
		logLevel = LevelError
	default:
		logLevel = LevelInfo
	}
}

// getZapLevel converts our level to zap.AtomicLevel
func getZapLevel(level Level) zap.AtomicLevel {
	switch level {
	case LevelDebug:
		return zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case LevelInfo:
		return zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case LevelWarn:
		return zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case LevelError:
		return zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		return zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	if logLevel > LevelDebug {
		return
	}
	msg := fmt.Sprintf(format, v...)
	zapLogger.Debug(msg)
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	if logLevel > LevelInfo {
		return
	}
	msg := fmt.Sprintf(format, v...)
	zapLogger.Info(msg)
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	if logLevel > LevelWarn {
		return
	}
	msg := fmt.Sprintf(format, v...)
	zapLogger.Warn(msg)
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	if logLevel > LevelError {
		return
	}
	msg := fmt.Sprintf(format, v...)
	zapLogger.Error(msg)
}

// ErrorWithStack logs an error with a stack trace
func ErrorWithStack(err error) {
	if err == nil {
		return
	}
	zapLogger.Error(
		err.Error(),
		zap.String("stack", string(debug.Stack())),
	)
}

// RequestLog logs details of an HTTP request
func RequestLog(method, url, sessionID, body string) {
	if logLevel > LevelDebug {
		return
	}
	zapLogger.Debug("HTTP Request",
		zap.String("method", method),
		zap.String("url", url),
		zap.String("sessionID", sessionID),
		zap.String("body", body),
	)
}

// ResponseLog logs details of an HTTP response
func ResponseLog(statusCode int, sessionID, body string) {
	if logLevel > LevelDebug {
		return
	}
	zapLogger.Debug("HTTP Response",
		zap.Int("statusCode", statusCode),
		zap.String("sessionID", sessionID),
		zap.String("body", body),
	)
}

// SSEEventLog logs details of an SSE event
func SSEEventLog(eventType, sessionID, data string) {
	if logLevel > LevelDebug {
		return
	}
	zapLogger.Debug("SSE Event",
		zap.String("eventType", eventType),
		zap.String("sessionID", sessionID),
		zap.String("data", data),
	)
}

// RequestResponseLog logs a combined request and response log entry
func RequestResponseLog(method, sessionID string, requestData, responseData string) {
	if logLevel > LevelDebug {
		return
	}

	// Format for more readable logs
	formattedRequest := requestData
	formattedResponse := responseData

	// Try to format JSON if it's valid
	if strings.HasPrefix(requestData, "{") || strings.HasPrefix(requestData, "[") {
		var obj interface{}
		if err := json.Unmarshal([]byte(requestData), &obj); err == nil {
			if formatted, err := json.MarshalIndent(obj, "", "  "); err == nil {
				formattedRequest = string(formatted)
			}
		}
	}

	if strings.HasPrefix(responseData, "{") || strings.HasPrefix(responseData, "[") {
		var obj interface{}
		if err := json.Unmarshal([]byte(responseData), &obj); err == nil {
			if formatted, err := json.MarshalIndent(obj, "", "  "); err == nil {
				formattedResponse = string(formatted)
			}
		}
	}

	zapLogger.Debug("Request/Response",
		zap.String("method", method),
		zap.String("sessionID", sessionID),
		zap.String("request", formattedRequest),
		zap.String("response", formattedResponse),
	)
}
