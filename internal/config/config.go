// Package config provides configuration management for the database MCP server.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/joho/godotenv"

	"github.com/FreePeak/db-mcp-server/internal/logger"
	"github.com/FreePeak/db-mcp-server/pkg/db"
)

// Config holds all server configuration
type Config struct {
	ServerPort     int
	TransportMode  string
	LogLevel       string
	DBConfig       DatabaseConfig    // Legacy single database config
	MultiDBConfig  *db.MultiDBConfig // New multi-database config
	ConfigPath     string            // Path to the configuration file
	DisableLogging bool              // When true, disables logging in stdio/SSE transport
}

// DatabaseConfig holds database configuration (legacy support)
type DatabaseConfig struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

// LoadConfig loads the configuration from environment variables and optional JSON config
func LoadConfig(logDir string) (*Config, error) {
	// Check for verbose flag - logging enabled only when explicitly requested
	// Default to error level (silent), debug when VERBOSE=true
	logLevel := "error"
	if os.Getenv("VERBOSE") == "true" || os.Getenv("VERBOSE") == "1" {
		logLevel = "debug"
	}

	// Initialize logger with appropriate level
	logger.Initialize(logger.Config{Level: logLevel, LogDir: logDir})

	// Load .env file if it exists
	err := godotenv.Load()
	if err != nil {
		logger.Info("Warning: .env file not found, using environment variables only")
	} else {
		logger.Info("Loaded configuration from .env file")
	}

	port, err := strconv.Atoi(getEnv("SERVER_PORT", "9092"))
	if err != nil {
		logger.Warn("Warning: Invalid SERVER_PORT value, using default 9092")
		port = 9092
	}

	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "3306"))
	if err != nil {
		logger.Warn("Warning: Invalid DB_PORT value, using default 3306")
		dbPort = 3306
	}

	// Get config path from environment or use default
	configPath := getEnv("CONFIG_PATH", "")
	if configPath == "" {
		configPath = getEnv("DB_CONFIG_FILE", "config.json")
	}

	// Resolve absolute path if relative path is provided
	if !filepath.IsAbs(configPath) {
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			logger.Warn("Warning: Could not resolve absolute path for config file: %v", err)
		} else {
			configPath = absPath
		}
	}

	// Check for verbose flag - logging enabled only when explicitly requested
	disableLogging := false
	if v := getEnv("DISABLE_LOGGING", "false"); v == "true" || v == "1" {
		disableLogging = true
	}

	config := &Config{
		ServerPort:     port,
		TransportMode:  getEnv("TRANSPORT_MODE", "stdio"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		ConfigPath:     configPath,
		DisableLogging: disableLogging,
		DBConfig: DatabaseConfig{
			Type:     getEnv("DB_TYPE", "mysql"),
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", ""),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", ""),
		},
	}

	// Load exactly one canonical multi-database configuration. The parsed
	// object is passed to dbtools.InitDatabase; the file is never re-parsed by a
	// second, reduced connection type.
	if _, statErr := os.Stat(config.ConfigPath); statErr == nil {
		logger.Info("Loading configuration from: %s", config.ConfigPath)
		configData, err := os.ReadFile(config.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", config.ConfigPath, err)
		}
		multiDBConfig, err := decodeMultiDBConfig(configData, config.ConfigPath)
		if err != nil {
			return nil, err
		}
		if len(multiDBConfig.Connections) == 0 {
			return nil, fmt.Errorf("config file %s contains no database connections", config.ConfigPath)
		}
		resolveSQLitePaths(multiDBConfig, filepath.Dir(config.ConfigPath))
		config.MultiDBConfig = multiDBConfig
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("failed to inspect config file %s: %w", config.ConfigPath, statErr)
	} else if inline := os.Getenv("DB_CONFIG"); inline != "" {
		logger.Info("Loading configuration from DB_CONFIG")
		multiDBConfig, err := decodeMultiDBConfig([]byte(inline), "DB_CONFIG")
		if err != nil {
			return nil, err
		}
		if len(multiDBConfig.Connections) == 0 {
			return nil, fmt.Errorf("DB_CONFIG contains no database connections")
		}
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve DB_CONFIG SQLite paths: %w", err)
		}
		resolveSQLitePaths(multiDBConfig, cwd)
		config.MultiDBConfig = multiDBConfig
	} else {
		logger.Info("Warning: Config file not found at %s, using environment variables", config.ConfigPath)
		// If no JSON config found, create a single connection config from environment variables
		config.MultiDBConfig = &db.MultiDBConfig{
			Connections: []db.DatabaseConnectionConfig{
				{
					ID:       "default",
					Type:     config.DBConfig.Type,
					Host:     config.DBConfig.Host,
					Port:     config.DBConfig.Port,
					User:     config.DBConfig.User,
					Password: config.DBConfig.Password,
					Name:     config.DBConfig.Name,
				},
			},
		}
	}

	secretBaseDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve secret file paths: %w", err)
	}
	if _, statErr := os.Stat(config.ConfigPath); statErr == nil {
		secretBaseDir = filepath.Dir(config.ConfigPath)
	}
	for i := range config.MultiDBConfig.Connections {
		if err := db.ResolveSecretReferences(&config.MultiDBConfig.Connections[i], secretBaseDir); err != nil {
			return nil, err
		}
	}

	// Apply the global statement-timeout override to connections that do not
	// set their own query_timeout, so env-only deployments get time caps too.
	// Negative values disable the cap; JSON configs keep precedence.
	if v := getEnv("QUERY_TIMEOUT_SECONDS", ""); v != "" {
		applyQueryTimeoutOverride(config.MultiDBConfig.Connections, v, logger.Warn)
	}
	if err := validateMaskingRules(config.MultiDBConfig.Connections); err != nil {
		return nil, err
	}

	return config, nil
}

// decodeMultiDBConfig parses the canonical configuration and rejects unknown
// fields or trailing JSON values. A typo in a security-sensitive key must not
// silently fall back to a writable connection.
func decodeMultiDBConfig(data []byte, source string) (*db.MultiDBConfig, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var multiDBConfig db.MultiDBConfig
	if err := decoder.Decode(&multiDBConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", source, err)
	}
	if multiDBConfig.SchemaVersion != 0 && multiDBConfig.SchemaVersion != 1 {
		return nil, fmt.Errorf("config %s uses unsupported schema_version %d", source, multiDBConfig.SchemaVersion)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("config %s contains multiple JSON values", source)
		}
		return nil, fmt.Errorf("failed to parse config %s: %w", source, err)
	}
	return &multiDBConfig, nil
}

// validateMaskingRules compiles every configured masking rule at startup.
// A broken pattern silently disables the mask it was meant to enforce,
// which is exactly when graceful degradation is dangerous: data the
// operator believes is hidden would flow through unmasked. Fail closed.
func validateMaskingRules(conns []db.DatabaseConnectionConfig) error {
	for i := range conns {
		for j, rule := range conns[i].MaskingRules {
			where := fmt.Sprintf("database %q masking_rules[%d]", conns[i].ID, j)
			if _, err := regexp.Compile(rule.Pattern); err != nil {
				return fmt.Errorf("%s: invalid pattern %q: %w", where, rule.Pattern, err)
			}
			switch rule.Strategy {
			case "fixed_string", "null", "partial":
				// valid; fixed_string with empty value is legitimate (mask to "")
			default:
				return fmt.Errorf("%s: unknown strategy %q (want fixed_string, null, or partial)", where, rule.Strategy)
			}
		}
	}
	return nil
}

// applyQueryTimeoutOverride fills unset (0) per-connection timeouts from the
// QUERY_TIMEOUT_SECONDS value; invalid or out-of-range input is ignored with
// a warning so one bad variable cannot block startup.
func applyQueryTimeoutOverride(conns []db.DatabaseConnectionConfig, raw string, warnf func(format string, args ...interface{})) {
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < -1 {
		warnf("Warning: invalid QUERY_TIMEOUT_SECONDS %q ignored", raw)
		return
	}
	for i := range conns {
		if conns[i].QueryTimeout == 0 {
			conns[i].QueryTimeout = secs
		}
	}
}

// resolveSQLitePaths resolves SQLite database_path values to absolute paths
// relative to the config file directory
func resolveSQLitePaths(multiDBConfig *db.MultiDBConfig, configDir string) {
	if multiDBConfig == nil {
		return
	}

	for i := range multiDBConfig.Connections {
		conn := &multiDBConfig.Connections[i]
		if conn.Type == "sqlite" && conn.DatabasePath != "" && !filepath.IsAbs(conn.DatabasePath) {
			// Resolve relative path against config file directory
			conn.DatabasePath = filepath.Join(configDir, conn.DatabasePath)
		}
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
