package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/logger"
	// Import database drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
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
	logger.Info("Connected to %s database at %s:%d/%s", d.config.Type, d.config.Host, d.config.Port, d.config.Name)

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
	default:
		return "unknown"
	}
}

// QueryTimeout returns the configured query timeout in seconds
func (d *database) QueryTimeout() int {
	return d.config.QueryTimeout
}
