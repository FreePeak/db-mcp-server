package db

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/logger"
)

// DatabaseConnectionConfig represents a single database connection configuration
type DatabaseConnectionConfig struct {
	ID       string `json:"id"`   // Unique identifier for this connection
	Type     string `json:"type"` // mysql, postgres, oracle, or sqlite
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`

	// PostgreSQL specific options
	SSLMode            string            `json:"ssl_mode,omitempty"`
	SSLCert            string            `json:"ssl_cert,omitempty"`
	SSLKey             string            `json:"ssl_key,omitempty"`
	SSLRootCert        string            `json:"ssl_root_cert,omitempty"`
	ApplicationName    string            `json:"application_name,omitempty"`
	ConnectTimeout     int               `json:"connect_timeout,omitempty"`
	QueryTimeout       int               `json:"query_timeout,omitempty"` // in seconds
	TargetSessionAttrs string            `json:"target_session_attrs,omitempty"`
	Options            map[string]string `json:"options,omitempty"`

	// SQLite specific options
	DatabasePath     string `json:"database_path,omitempty"`      // Path to SQLite database file
	EncryptionKey    string `json:"encryption_key,omitempty"`     // Key for SQLCipher encryption
	ReadOnly         bool   `json:"read_only,omitempty"`          // Open database in read-only mode
	CacheSize        int    `json:"cache_size,omitempty"`         // SQLite cache size (in pages)
	JournalMode      string `json:"journal_mode,omitempty"`       // Journal mode for SQLite
	UseModerncDriver bool   `json:"use_modernc_driver,omitempty"` // Use modernc.org/sqlite driver instead of mattn/go-sqlite3

	// Connection pool settings
	MaxOpenConns    int `json:"max_open_conns,omitempty"`
	MaxIdleConns    int `json:"max_idle_conns,omitempty"`
	ConnMaxLifetime int `json:"conn_max_lifetime_seconds,omitempty"`  // in seconds
	ConnMaxIdleTime int `json:"conn_max_idle_time_seconds,omitempty"` // in seconds
}

// MultiDBConfig represents the configuration for multiple database connections
type MultiDBConfig struct {
	Connections []DatabaseConnectionConfig `json:"connections"`
}

// Manager manages multiple database connections
type Manager struct {
	mu          sync.RWMutex
	connections map[string]Database
	configs     map[string]DatabaseConnectionConfig
	lazyLoading bool // When true, connections are established on first use instead of startup
}

// NewDBManager creates a new database manager
func NewDBManager() *Manager {
	return &Manager{
		connections: make(map[string]Database),
		configs:     make(map[string]DatabaseConnectionConfig),
		lazyLoading: false, // Default: eager loading for backward compatibility
	}
}

// SetLazyLoading enables or disables lazy loading mode
// When enabled, database connections are established on first use instead of during initialization.
// This is recommended when managing many database connections (10+) to reduce startup time and memory usage.
func (m *Manager) SetLazyLoading(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lazyLoading = enabled
	if enabled {
		logger.Info("Lazy loading enabled: connections will be established on first use")
	} else {
		logger.Info("Lazy loading disabled: all connections will be established during initialization")
	}
}

// IsLazyLoading returns whether lazy loading mode is currently enabled
func (m *Manager) IsLazyLoading() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lazyLoading
}

// LoadConfig loads database configurations from JSON
func (m *Manager) LoadConfig(configJSON []byte) error {
	var config MultiDBConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Validate and store configurations
	for _, conn := range config.Connections {
		if conn.ID == "" {
			return fmt.Errorf("database connection ID cannot be empty")
		}
		if conn.Type != "mysql" && conn.Type != "postgres" && conn.Type != "oracle" && conn.Type != "sqlite" {
			return fmt.Errorf("unsupported database type for connection %s: %s (must be mysql, postgres, oracle, or sqlite)", conn.ID, conn.Type)
		}

		// SQLite-specific validation
		if conn.Type == "sqlite" {
			if conn.DatabasePath == "" && conn.Name == "" {
				return fmt.Errorf("SQLite database %s requires either database_path or name to be specified", conn.ID)
			}
		}

		m.configs[conn.ID] = conn
	}

	return nil
}

// createAndConnectDatabase creates a database instance, connects to it, and returns it
func createAndConnectDatabase(id string, cfg DatabaseConnectionConfig) (Database, error) {
	// Build configuration
	dbConfig := buildDatabaseConfig(cfg)

	// Create database instance
	db, err := NewDatabase(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create database instance for %s: %w", id, err)
	}

	// Connect to database
	if err := db.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to database %s: %w", id, err)
	}

	return db, nil
}

// buildDatabaseConfig creates a Config from DatabaseConnectionConfig
func buildDatabaseConfig(cfg DatabaseConnectionConfig) Config {
	dbConfig := Config{
		Type:     cfg.Type,
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		Name:     cfg.Name,
	}

	// Set database-specific options based on type
	switch cfg.Type {
	case "postgres":
		dbConfig.SSLMode = PostgresSSLMode(cfg.SSLMode)
		dbConfig.SSLCert = cfg.SSLCert
		dbConfig.SSLKey = cfg.SSLKey
		dbConfig.SSLRootCert = cfg.SSLRootCert
		dbConfig.ApplicationName = cfg.ApplicationName
		dbConfig.ConnectTimeout = cfg.ConnectTimeout
		dbConfig.QueryTimeout = cfg.QueryTimeout
		dbConfig.TargetSessionAttrs = cfg.TargetSessionAttrs
		dbConfig.Options = cfg.Options
	case "oracle":
		// Extract Oracle-specific options from Options map
		if serviceName, ok := cfg.Options["service_name"]; ok {
			dbConfig.ServiceName = serviceName
		}
		if sid, ok := cfg.Options["sid"]; ok {
			dbConfig.SID = sid
		}
		if walletLoc, ok := cfg.Options["wallet_location"]; ok {
			dbConfig.WalletLocation = walletLoc
		}
		if tnsAdmin, ok := cfg.Options["tns_admin"]; ok {
			dbConfig.TNSAdmin = tnsAdmin
		}
		if tnsEntry, ok := cfg.Options["tns_entry"]; ok {
			dbConfig.TNSEntry = tnsEntry
		}
		if edition, ok := cfg.Options["edition"]; ok {
			dbConfig.Edition = edition
		}
		if pooling, ok := cfg.Options["pooling"]; ok {
			dbConfig.Pooling = pooling == "true"
		}
		if standby, ok := cfg.Options["standby_sessions"]; ok {
			dbConfig.StandbySessions = standby == "true"
		}
		if nlsLang, ok := cfg.Options["nls_lang"]; ok {
			dbConfig.NLSLang = nlsLang
		}
		dbConfig.ConnectTimeout = cfg.ConnectTimeout
		dbConfig.QueryTimeout = cfg.QueryTimeout
		dbConfig.Options = cfg.Options
	case "mysql":
		// Set MySQL-specific options
		dbConfig.ConnectTimeout = cfg.ConnectTimeout
		dbConfig.QueryTimeout = cfg.QueryTimeout
	case "sqlite":
		// Set SQLite-specific options
		dbConfig.DatabasePath = cfg.DatabasePath
		dbConfig.EncryptionKey = cfg.EncryptionKey
		dbConfig.ReadOnly = cfg.ReadOnly
		dbConfig.CacheSize = cfg.CacheSize
		if cfg.JournalMode != "" {
			dbConfig.JournalMode = SQLiteJournalMode(cfg.JournalMode)
		}
		dbConfig.UseModerncDriver = cfg.UseModerncDriver
	default:
		// Default case - common configuration
		dbConfig.ConnectTimeout = cfg.ConnectTimeout
		dbConfig.QueryTimeout = cfg.QueryTimeout
		dbConfig.Options = cfg.Options
	}

	// Connection pool settings
	if cfg.MaxOpenConns > 0 {
		dbConfig.MaxOpenConns = cfg.MaxOpenConns
	}
	if cfg.MaxIdleConns > 0 {
		dbConfig.MaxIdleConns = cfg.MaxIdleConns
	}
	if cfg.ConnMaxLifetime > 0 {
		dbConfig.ConnMaxLifetime = time.Duration(cfg.ConnMaxLifetime) * time.Second
	}
	if cfg.ConnMaxIdleTime > 0 {
		dbConfig.ConnMaxIdleTime = time.Duration(cfg.ConnMaxIdleTime) * time.Second
	}

	return dbConfig
}

// Connect establishes connections to all configured databases
// If lazy loading is enabled, performs health check on one database of each type
func (m *Manager) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If lazy loading is enabled, perform health check only
	if m.lazyLoading {
		logger.Info("Lazy loading enabled: skipping connection establishment for %d databases", len(m.configs))

		// Perform health check: connect to one database of each type to validate credentials
		if len(m.configs) > 0 {
			// Find one database of each type for health check
			typesToCheck := make(map[string]DatabaseConnectionConfig)
			for id, cfg := range m.configs {
				// If we haven't seen this database type yet, use it for health check
				if _, exists := typesToCheck[cfg.Type]; !exists {
					typesToCheck[cfg.Type] = cfg
					// Store the ID in a temporary field for logging
					tempCfg := cfg
					tempCfg.ID = id
					typesToCheck[cfg.Type] = tempCfg
				}
			}

			// Connect to one database of each type
			for dbType, cfg := range typesToCheck {
				logger.Info("Health check: testing %s connection to %s", dbType, cfg.ID)

				db, err := createAndConnectDatabase(cfg.ID, cfg)
				if err != nil {
					return fmt.Errorf("health check failed for %s database %s: %w", dbType, cfg.ID, err)
				}

				// Store the connection
				m.connections[cfg.ID] = db
				logger.Info("Health check passed for %s: connected to %s", dbType, cfg.ID)
			}

			logger.Info("Health check complete: validated %d database type(s), %d databases will connect on first use",
				len(typesToCheck), len(m.configs)-len(typesToCheck))
		}

		return nil
	}

	// Connect to each database
	for id, cfg := range m.configs {
		// Skip if already connected
		if _, exists := m.connections[id]; exists {
			continue
		}

		// Create and connect to database
		db, err := createAndConnectDatabase(id, cfg)
		if err != nil {
			return err
		}

		// Store connected database
		m.connections[id] = db

		// Log connection info based on database type
		if cfg.Type == "sqlite" {
			dbPath := cfg.DatabasePath
			if dbPath == "" {
				dbPath = cfg.Name
			}
			if dbPath == ":memory:" {
				logger.Info("Successfully connected to database %s (%s in-memory database)", id, cfg.Type)
			} else {
				logger.Info("Successfully connected to database %s (%s at %s)", id, cfg.Type, dbPath)
			}
		} else {
			logger.Info("Successfully connected to database %s (%s at %s:%d/%s)", id, cfg.Type, cfg.Host, cfg.Port, cfg.Name)
		}
	}

	return nil
}

// GetDatabase retrieves a database connection by ID
// If lazy loading is enabled and the connection doesn't exist, it will be established on-demand
func (m *Manager) GetDatabase(id string) (Database, error) {
	// First, try to get existing connection with read lock
	m.mu.RLock()
	db, exists := m.connections[id]
	lazyEnabled := m.lazyLoading
	m.mu.RUnlock()

	// If connection exists, return it
	if exists {
		return db, nil
	}

	// If lazy loading is disabled, connection should already exist
	if !lazyEnabled {
		return nil, fmt.Errorf("database connection %s not found", id)
	}

	// Lazy loading is enabled - establish connection on-demand
	return m.connectOnDemand(id)
}

// connectOnDemand establishes a connection to a specific database on first use
func (m *Manager) connectOnDemand(id string) (Database, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check if connection was established by another goroutine
	if db, exists := m.connections[id]; exists {
		return db, nil
	}

	// Get configuration
	cfg, exists := m.configs[id]
	if !exists {
		return nil, fmt.Errorf("database configuration %s not found", id)
	}

	logger.Info("Lazy loading: establishing connection to database %s on first use", id)

	// Create and connect to database
	db, err := createAndConnectDatabase(id, cfg)
	if err != nil {
		return nil, err
	}

	// Store connected database
	m.connections[id] = db

	// Log connection info based on database type
	if cfg.Type == "sqlite" {
		dbPath := cfg.DatabasePath
		if dbPath == "" {
			dbPath = cfg.Name
		}
		if dbPath == ":memory:" {
			logger.Info("Successfully connected to database %s (%s in-memory database)", id, cfg.Type)
		} else {
			logger.Info("Successfully connected to database %s (%s at %s)", id, cfg.Type, dbPath)
		}
	} else {
		logger.Info("Successfully connected to database %s (%s at %s:%d/%s)", id, cfg.Type, cfg.Host, cfg.Port, cfg.Name)
	}

	return db, nil
}

// GetDatabaseType returns the type of a database by its ID
func (m *Manager) GetDatabaseType(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if the database configuration exists
	cfg, exists := m.configs[id]
	if !exists {
		return "", fmt.Errorf("database configuration %s not found", id)
	}

	return cfg.Type, nil
}

// CloseAll closes all database connections
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error

	// Close each database connection
	for id, db := range m.connections {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close database %s: %v", id, err)
			if firstErr == nil {
				firstErr = err
			}
		}
		delete(m.connections, id)
	}

	return firstErr
}

// Close closes a specific database connection
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if the database exists
	db, exists := m.connections[id]
	if !exists {
		return fmt.Errorf("database connection %s not found", id)
	}

	// Close the connection
	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close database %s: %w", id, err)
	}

	// Remove from connections map
	delete(m.connections, id)

	return nil
}

// ListDatabases returns a list of all configured databases
func (m *Manager) ListDatabases() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.configs))
	for id := range m.configs {
		ids = append(ids, id)
	}

	return ids
}

// GetConnectedDatabases returns a list of all connected databases
func (m *Manager) GetConnectedDatabases() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.connections))
	for id := range m.connections {
		ids = append(ids, id)
	}

	return ids
}

// GetDatabaseConfig returns the configuration for a specific database
func (m *Manager) GetDatabaseConfig(id string) (DatabaseConnectionConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, exists := m.configs[id]
	if !exists {
		return DatabaseConnectionConfig{}, fmt.Errorf("database configuration %s not found", id)
	}

	return cfg, nil
}
