// Package db provides a unified interface for connecting to and interacting with
// multiple database types including MySQL, PostgreSQL, SQLite, and TimescaleDB.
// It implements common database operations with type-specific optimizations.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/logger"
	// Import database drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/sijms/go-ora/v2"
	_ "modernc.org/sqlite"
)

// Common database errors
var (
	ErrNotFound       = errors.New("record not found")
	ErrAlreadyExists  = errors.New("record already exists")
	ErrInvalidInput   = errors.New("invalid input")
	ErrNotImplemented = errors.New("not implemented")
	ErrNoDatabase     = errors.New("no database connection")
)

// PostgresSSLMode defines the SSL mode for PostgreSQL connections
type PostgresSSLMode string

// SSLMode constants for PostgreSQL
const (
	SSLDisable    PostgresSSLMode = "disable"
	SSLRequire    PostgresSSLMode = "require"
	SSLVerifyCA   PostgresSSLMode = "verify-ca"
	SSLVerifyFull PostgresSSLMode = "verify-full"
	SSLPrefer     PostgresSSLMode = "prefer"
)

// SQLiteJournalMode defines the journal mode for SQLite connections
type SQLiteJournalMode string

// SQLiteJournalMode constants
const (
	JournalDelete   SQLiteJournalMode = "DELETE"
	JournalTruncate SQLiteJournalMode = "TRUNCATE"
	JournalPersist  SQLiteJournalMode = "PERSIST"
	JournalWAL      SQLiteJournalMode = "WAL"
	JournalOff      SQLiteJournalMode = "OFF"
)

// Config represents database connection configuration
type Config struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	Name     string

	// Additional PostgreSQL specific options
	SSLMode            PostgresSSLMode
	SSLCert            string
	SSLKey             string
	SSLRootCert        string
	ApplicationName    string
	ConnectTimeout     int               // in seconds
	QueryTimeout       int               // in seconds, default is 30 seconds
	TargetSessionAttrs string            // for PostgreSQL 10+
	Options            map[string]string // Extra connection options

	// SQLite specific options
	DatabasePath     string            // Path to SQLite database file
	EncryptionKey    string            // Key for SQLCipher encryption
	ReadOnly         bool              // Open database in read-only mode
	CacheSize        int               // SQLite cache size (in pages)
	JournalMode      SQLiteJournalMode // Journal mode for SQLite
	UseModerncDriver bool              // Use modernc.org/sqlite driver instead of mattn/go-sqlite3

	// Oracle specific options
	ServiceName     string // Oracle service name (preferred over SID)
	SID             string // Oracle SID (legacy)
	WalletLocation  string // Path to Oracle Cloud wallet
	TNSAdmin        string // Path to tnsnames.ora directory
	TNSEntry        string // TNS entry name from tnsnames.ora
	Edition         string // Oracle Edition-Based Redefinition
	Pooling         bool   // Enable connection pooling
	StandbySessions bool   // Allow connections to standby database
	NLSLang         string // NLS_LANG setting (e.g., "AMERICAN_AMERICA.AL32UTF8")

	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// SetDefaults sets default values for the configuration if they are not set
func (c *Config) SetDefaults() {
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 25
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = 5 * time.Minute
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 5 * time.Minute
	}
	if c.Type == "postgres" && c.SSLMode == "" {
		c.SSLMode = SSLDisable
	}
	if c.Type == "oracle" {
		if c.Port == 0 {
			c.Port = 1521
		}
		// Default to service name if both service name and SID are empty
		if c.ServiceName == "" && c.SID == "" {
			c.ServiceName = c.Name
		}
		// Default NLS settings for UTF-8
		if c.NLSLang == "" {
			c.NLSLang = "AMERICAN_AMERICA.AL32UTF8"
		}
		// Oracle benefits from larger connection pool
		if c.MaxOpenConns == 25 {
			c.MaxOpenConns = 50
		}
		if c.MaxIdleConns == 5 {
			c.MaxIdleConns = 10
		}
		if c.ConnMaxLifetime == 5*time.Minute {
			c.ConnMaxLifetime = 30 * time.Minute
		}
	}
	if c.Type == "sqlite" {
		if c.JournalMode == "" {
			c.JournalMode = JournalWAL // Default to WAL mode for better concurrency
		}
		if c.CacheSize == 0 {
			c.CacheSize = 2000 // Default cache size
		}
		// For SQLite, use modernc driver by default (CGO-free)
		if c.DatabasePath == "" && c.Name != "" {
			c.DatabasePath = c.Name
		}
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 10 // Default 10 seconds
	}
	if c.QueryTimeout == 0 {
		c.QueryTimeout = 30 // Default 30 seconds
	}
}

// Database represents a generic database interface
type Database interface {
	// Core database operations
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Transaction support
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

	// Connection management
	Connect() error
	Close() error
	Ping(ctx context.Context) error

	// Metadata
	DriverName() string
	ConnectionString() string
	QueryTimeout() int

	// DB object access (for specific DB operations)
	DB() *sql.DB
}

// database is the concrete implementation of the Database interface
type database struct {
	config     Config
	db         *sql.DB
	driverName string
	dsn        string
}

// buildPostgresConnStr builds a PostgreSQL connection string with all options
func buildPostgresConnStr(config Config) string {
	params := make([]string, 0)

	// Required parameters
	params = append(params, fmt.Sprintf("host=%s", config.Host))
	params = append(params, fmt.Sprintf("port=%d", config.Port))
	params = append(params, fmt.Sprintf("user=%s", config.User))

	if config.Password != "" {
		params = append(params, fmt.Sprintf("password=%s", config.Password))
	}

	if config.Name != "" {
		params = append(params, fmt.Sprintf("dbname=%s", config.Name))
	}

	// SSL configuration
	params = append(params, fmt.Sprintf("sslmode=%s", config.SSLMode))

	if config.SSLCert != "" {
		params = append(params, fmt.Sprintf("sslcert=%s", config.SSLCert))
	}

	if config.SSLKey != "" {
		params = append(params, fmt.Sprintf("sslkey=%s", config.SSLKey))
	}

	if config.SSLRootCert != "" {
		params = append(params, fmt.Sprintf("sslrootcert=%s", config.SSLRootCert))
	}

	// Connection timeout
	if config.ConnectTimeout > 0 {
		params = append(params, fmt.Sprintf("connect_timeout=%d", config.ConnectTimeout))
	}

	// Application name for better identification in pg_stat_activity
	if config.ApplicationName != "" {
		params = append(params, fmt.Sprintf("application_name=%s", url.QueryEscape(config.ApplicationName)))
	}

	// Target session attributes for load balancing and failover (PostgreSQL 10+)
	if config.TargetSessionAttrs != "" {
		params = append(params, fmt.Sprintf("target_session_attrs=%s", config.TargetSessionAttrs))
	}

	// Add any additional options from the map
	if config.Options != nil {
		for key, value := range config.Options {
			params = append(params, fmt.Sprintf("%s=%s", key, url.QueryEscape(value)))
		}
	}

	return strings.Join(params, " ")
}

// buildOracleConnStr builds an Oracle connection string from the config
func buildOracleConnStr(config Config) string {
	// Priority 1: TNS Entry (tnsnames.ora)
	if config.TNSEntry != "" && config.TNSAdmin != "" {
		// Use TNS alias with admin directory
		connStr := fmt.Sprintf("oracle://%s:%s@%s",
			config.User,
			config.Password,
			config.TNSEntry)

		connStr += fmt.Sprintf("?tns admin=%s", config.TNSAdmin)

		return addOracleOptions(connStr, config)
	}

	// Priority 2: Oracle Cloud Wallet
	if config.WalletLocation != "" {
		// For Oracle Cloud Autonomous Database
		serviceName := config.ServiceName
		if serviceName == "" {
			serviceName = config.Name
		}

		connStr := fmt.Sprintf("oracle://%s:%s@%s",
			config.User,
			config.Password,
			serviceName)

		connStr += fmt.Sprintf("?wallet location=%s", config.WalletLocation)

		return addOracleOptions(connStr, config)
	}

	// Priority 3: Standard Connection (host:port/service)
	host := config.Host
	port := config.Port
	if port == 0 {
		port = 1521
	}

	// Prefer service name over SID
	databaseIdentifier := config.ServiceName
	if databaseIdentifier == "" && config.SID != "" {
		databaseIdentifier = config.SID
	} else if databaseIdentifier == "" {
		databaseIdentifier = config.Name
	}

	// EZ Connect format: oracle://user:password@host:port/service_name
	connStr := fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		config.User,
		config.Password,
		host,
		port,
		databaseIdentifier)

	return addOracleOptions(connStr, config)
}

// addOracleOptions adds optional parameters to Oracle connection string
func addOracleOptions(connStr string, config Config) string {
	params := make(map[string]string)

	// Connection timeout (in seconds)
	if config.ConnectTimeout > 0 {
		params["timeout"] = fmt.Sprintf("%d", config.ConnectTimeout)
	}

	// NLS Language settings
	if config.NLSLang != "" {
		params["language"] = config.NLSLang
	}

	// Edition-based redefinition
	if config.Edition != "" {
		params["edition"] = config.Edition
	}

	// Connection pooling
	if config.Pooling {
		params["pooling"] = "true"
	}

	// Standby database support (read-only)
	if config.StandbySessions {
		params["standby"] = "true"
	}

	// Additional custom options
	for key, value := range config.Options {
		params[key] = value
	}

	// Append parameters
	if len(params) > 0 {
		separator := "?"
		if strings.Contains(connStr, "?") {
			separator = "&"
		}

		var paramPairs []string
		for key, value := range params {
			paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", key, value))
		}

		connStr += separator + strings.Join(paramPairs, "&")
	}

	return connStr
}

// buildSQLiteConnStr builds a SQLite connection string with all options
func buildSQLiteConnStr(config Config) string {
	// Validate and clean the database path
	dbPath := config.DatabasePath
	if dbPath == "" {
		dbPath = config.Name
	}

	// Handle special SQLite paths
	if dbPath == ":memory:" {
		return ":memory:"
	}

	// Clean the path
	dbPath = filepath.Clean(dbPath)

	// Build query parameters
	params := make(url.Values)

	// Read-only mode
	if config.ReadOnly {
		params.Set("mode", "ro")
	} else {
		params.Set("mode", "rwc")
	}

	// Cache size
	if config.CacheSize > 0 {
		params.Set("cache", "shared")
		// Cache size will be set via PRAGMA after connection
	}

	// Journal mode
	if config.JournalMode != "" {
		params.Set("_journal_mode", string(config.JournalMode))
	}

	// Foreign key constraints
	params.Set("_foreign_keys", "enabled")

	// SQLCipher encryption key
	if config.EncryptionKey != "" {
		params.Set("_pragma_key", config.EncryptionKey)
		// Set cipher page size for SQLCipher compatibility
		params.Set("_cipher_page_size", "4096")
	}

	// Additional options
	if config.Options != nil {
		for key, value := range config.Options {
			params.Set(key, value)
		}
	}

	// Build the final connection string
	connStr := fmt.Sprintf("file:%s", dbPath)
	if len(params) > 0 {
		connStr += "?" + params.Encode()
	}

	return connStr
}

// NewDatabase creates a new database connection based on the provided configuration
func NewDatabase(config Config) (Database, error) {
	// Set default values for the configuration
	config.SetDefaults()

	var dsn string
	var driverName string

	// Create DSN string based on database type
	switch config.Type {
	case "mysql":
		driverName = "mysql"
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			config.User, config.Password, config.Host, config.Port, config.Name)
	case "postgres":
		driverName = "postgres"
		dsn = buildPostgresConnStr(config)
	case "oracle":
		driverName = "oracle"
		dsn = buildOracleConnStr(config)
	case "sqlite":
		// Choose driver based on configuration
		if config.UseModerncDriver || config.EncryptionKey == "" {
			driverName = "sqlite"
		} else {
			// Use mattn/go-sqlite3 driver for SQLCipher
			driverName = "sqlite3"
		}
		dsn = buildSQLiteConnStr(config)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	return &database{
		config:     config,
		driverName: driverName,
		dsn:        dsn,
	}, nil
}

// Connect establishes a connection to the database
func (d *database) Connect() error {
	db, err := sql.Open(d.driverName, d.dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(d.config.MaxOpenConns)
	db.SetMaxIdleConns(d.config.MaxIdleConns)
	db.SetConnMaxLifetime(d.config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(d.config.ConnMaxIdleTime)

	// Verify connection is working
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			logger.Error("Error closing database connection: %v", closeErr)
		}
		return fmt.Errorf("failed to ping database: %w", err)
	}

	d.db = db

	// SQLite-specific initialization
	if d.config.Type == "sqlite" {
		// Set cache size if specified
		if d.config.CacheSize > 0 {
			_, err := db.Exec(fmt.Sprintf("PRAGMA cache_size = %d", d.config.CacheSize))
			if err != nil {
				logger.Warn("Failed to set SQLite cache size: %v", err)
			}
		}

		// Additional SQLite optimizations
		pragmas := []string{
			"PRAGMA synchronous = NORMAL",
			"PRAGMA temp_store = MEMORY",
			"PRAGMA mmap_size = 268435456", // 256MB
		}

		for _, pragma := range pragmas {
			_, err := db.Exec(pragma)
			if err != nil {
				logger.Warn("Failed to execute SQLite pragma '%s': %v", pragma, err)
			}
		}
	}

	// Log connection info
	switch d.config.Type {
	case "sqlite":
		dbPath := d.config.DatabasePath
		if dbPath == "" {
			dbPath = d.config.Name
		}
		if dbPath == ":memory:" {
			logger.Info("Connected to %s in-memory database", d.config.Type)
		} else {
			logger.Info("Connected to %s database at %s", d.config.Type, dbPath)
		}
	case "oracle":
		if d.config.WalletLocation != "" {
			logger.Info("Connected to Oracle Cloud database (wallet auth)")
		} else if d.config.TNSEntry != "" {
			logger.Info("Connected to Oracle database via TNS: %s", d.config.TNSEntry)
		} else {
			logger.Info("Connected to Oracle database at %s:%d/%s", d.config.Host, d.config.Port, d.config.Name)
		}
	default:
		logger.Info("Connected to %s database at %s:%d/%s", d.config.Type, d.config.Host, d.config.Port, d.config.Name)
	}

	return nil
}

// Close closes the database connection
func (d *database) Close() error {
	if d.db == nil {
		return nil
	}
	if err := d.db.Close(); err != nil {
		logger.Error("Error closing database connection: %v", err)
		return err
	}
	return nil
}

// Ping checks if the database connection is still alive
func (d *database) Ping(ctx context.Context) error {
	if d.db == nil {
		return ErrNoDatabase
	}
	return d.db.PingContext(ctx)
}

// Query executes a query that returns rows
func (d *database) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if d.db == nil {
		return nil, ErrNoDatabase
	}
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that is expected to return at most one row
func (d *database) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if d.db == nil {
		return nil
	}
	return d.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning any rows
func (d *database) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if d.db == nil {
		return nil, ErrNoDatabase
	}
	return d.db.ExecContext(ctx, query, args...)
}

// BeginTx starts a transaction
func (d *database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if d.db == nil {
		return nil, ErrNoDatabase
	}
	return d.db.BeginTx(ctx, opts)
}

// DB returns the underlying database connection
func (d *database) DB() *sql.DB {
	return d.db
}

// DriverName returns the name of the database driver
func (d *database) DriverName() string {
	return d.driverName
}

// ConnectionString returns the database connection string with password masked
func (d *database) ConnectionString() string {
	// Return masked DSN (hide password)
	switch d.config.Type {
	case "mysql":
		return fmt.Sprintf("%s:***@tcp(%s:%d)/%s",
			d.config.User, d.config.Host, d.config.Port, d.config.Name)
	case "postgres":
		// Create a sanitized version of the connection string
		params := make([]string, 0)

		params = append(params, fmt.Sprintf("host=%s", d.config.Host))
		params = append(params, fmt.Sprintf("port=%d", d.config.Port))
		params = append(params, fmt.Sprintf("user=%s", d.config.User))
		params = append(params, "password=***")
		params = append(params, fmt.Sprintf("dbname=%s", d.config.Name))

		if string(d.config.SSLMode) != "" {
			params = append(params, fmt.Sprintf("sslmode=%s", d.config.SSLMode))
		}

		if d.config.ApplicationName != "" {
			params = append(params, fmt.Sprintf("application_name=%s", d.config.ApplicationName))
		}

		return strings.Join(params, " ")
	case "oracle":
		// Mask Oracle connection string
		if d.config.WalletLocation != "" {
			serviceName := d.config.ServiceName
			if serviceName == "" {
				serviceName = d.config.Name
			}
			return fmt.Sprintf("oracle://%s:***@%s (wallet: %s)",
				d.config.User, serviceName, d.config.WalletLocation)
		}
		if d.config.TNSEntry != "" {
			return fmt.Sprintf("oracle://%s:***@%s (TNS)",
				d.config.User, d.config.TNSEntry)
		}
		return fmt.Sprintf("oracle://%s:***@%s:%d/%s",
			d.config.User, d.config.Host, d.config.Port, d.config.Name)
	case "sqlite":
		dbPath := d.config.DatabasePath
		if dbPath == "" {
			dbPath = d.config.Name
		}
		if dbPath == ":memory:" {
			return "SQLite in-memory database"
		}

		// Mask encryption key if present
		if d.config.EncryptionKey != "" {
			return fmt.Sprintf("SQLite database: %s (encrypted)", dbPath)
		}
		return fmt.Sprintf("SQLite database: %s", dbPath)
	default:
		return "unknown"
	}
}

// QueryTimeout returns the configured query timeout in seconds
func (d *database) QueryTimeout() int {
	return d.config.QueryTimeout
}
