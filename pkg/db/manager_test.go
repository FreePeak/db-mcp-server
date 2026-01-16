package db

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestNewDBManager(t *testing.T) {
	manager := NewDBManager()

	if manager == nil {
		t.Fatal("NewDBManager returned nil")
	}

	if manager.connections == nil {
		t.Error("connections map not initialized")
	}

	if manager.configs == nil {
		t.Error("configs map not initialized")
	}

	if manager.lazyLoading {
		t.Error("lazy loading should be disabled by default")
	}
}

func TestSetLazyLoading(t *testing.T) {
	manager := NewDBManager()

	// Test enabling lazy loading (direct field access to avoid logger dependency)
	manager.mu.Lock()
	manager.lazyLoading = true
	manager.mu.Unlock()

	if !manager.IsLazyLoading() {
		t.Error("lazy loading should be enabled")
	}

	// Test disabling lazy loading
	manager.mu.Lock()
	manager.lazyLoading = false
	manager.mu.Unlock()

	if manager.IsLazyLoading() {
		t.Error("lazy loading should be disabled")
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid postgres config",
			configJSON: `{
				"connections": [
					{
						"id": "test-postgres",
						"type": "postgres",
						"host": "localhost",
						"port": 5432,
						"user": "testuser",
						"password": "testpass",
						"name": "testdb"
					}
				]
			}`,
			expectError: false,
		},
		{
			name: "valid mysql config",
			configJSON: `{
				"connections": [
					{
						"id": "test-mysql",
						"type": "mysql",
						"host": "localhost",
						"port": 3306,
						"user": "testuser",
						"password": "testpass",
						"name": "testdb"
					}
				]
			}`,
			expectError: false,
		},
		{
			name: "multiple databases",
			configJSON: `{
				"connections": [
					{
						"id": "db1",
						"type": "postgres",
						"host": "localhost",
						"port": 5432,
						"user": "user1",
						"password": "pass1",
						"name": "db1"
					},
					{
						"id": "db2",
						"type": "mysql",
						"host": "localhost",
						"port": 3306,
						"user": "user2",
						"password": "pass2",
						"name": "db2"
					}
				]
			}`,
			expectError: false,
		},
		{
			name:        "invalid json",
			configJSON:  `{invalid json}`,
			expectError: true,
			errorMsg:    "failed to parse config JSON",
		},
		{
			name: "empty database id",
			configJSON: `{
				"connections": [
					{
						"id": "",
						"type": "postgres",
						"host": "localhost",
						"port": 5432,
						"user": "testuser",
						"password": "testpass",
						"name": "testdb"
					}
				]
			}`,
			expectError: true,
			errorMsg:    "database connection ID cannot be empty",
		},
		{
			name: "unsupported database type",
			configJSON: `{
				"connections": [
					{
						"id": "test-db",
						"type": "mongodb",
						"host": "localhost",
						"port": 27017,
						"user": "testuser",
						"password": "testpass",
						"name": "testdb"
					}
				]
			}`,
			expectError: true,
			errorMsg:    "unsupported database type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewDBManager()
			err := manager.LoadConfig([]byte(tt.configJSON))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetDatabaseType(t *testing.T) {
	manager := NewDBManager()

	// Load test config
	configJSON := `{
		"connections": [
			{
				"id": "postgres-db",
				"type": "postgres",
				"host": "localhost",
				"port": 5432,
				"user": "testuser",
				"password": "testpass",
				"name": "testdb"
			},
			{
				"id": "mysql-db",
				"type": "mysql",
				"host": "localhost",
				"port": 3306,
				"user": "testuser",
				"password": "testpass",
				"name": "testdb"
			}
		]
	}`

	if err := manager.LoadConfig([]byte(configJSON)); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	tests := []struct {
		name         string
		dbID         string
		expectedType string
		expectError  bool
	}{
		{
			name:         "get postgres type",
			dbID:         "postgres-db",
			expectedType: "postgres",
			expectError:  false,
		},
		{
			name:         "get mysql type",
			dbID:         "mysql-db",
			expectedType: "mysql",
			expectError:  false,
		},
		{
			name:        "non-existent database",
			dbID:        "non-existent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbType, err := manager.GetDatabaseType(tt.dbID)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if dbType != tt.expectedType {
					t.Errorf("expected type %s, got %s", tt.expectedType, dbType)
				}
			}
		})
	}
}

func TestListDatabases(t *testing.T) {
	manager := NewDBManager()

	// Initially empty
	if len(manager.ListDatabases()) != 0 {
		t.Error("expected empty database list")
	}

	// Load config with multiple databases
	configJSON := `{
		"connections": [
			{"id": "db1", "type": "postgres", "host": "localhost", "port": 5432, "user": "user", "password": "pass", "name": "db1"},
			{"id": "db2", "type": "mysql", "host": "localhost", "port": 3306, "user": "user", "password": "pass", "name": "db2"},
			{"id": "db3", "type": "postgres", "host": "localhost", "port": 5432, "user": "user", "password": "pass", "name": "db3"}
		]
	}`

	if err := manager.LoadConfig([]byte(configJSON)); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	dbs := manager.ListDatabases()
	if len(dbs) != 3 {
		t.Errorf("expected 3 databases, got %d", len(dbs))
	}

	// Check that all database IDs are present
	dbMap := make(map[string]bool)
	for _, id := range dbs {
		dbMap[id] = true
	}

	expectedIDs := []string{"db1", "db2", "db3"}
	for _, id := range expectedIDs {
		if !dbMap[id] {
			t.Errorf("expected database ID %s not found in list", id)
		}
	}
}

func TestGetDatabaseConfig(t *testing.T) {
	manager := NewDBManager()

	configJSON := `{
		"connections": [
			{
				"id": "test-db",
				"type": "postgres",
				"host": "test-host",
				"port": 5432,
				"user": "testuser",
				"password": "testpass",
				"name": "testdb",
				"ssl_mode": "require",
				"max_open_conns": 10,
				"max_idle_conns": 5
			}
		]
	}`

	if err := manager.LoadConfig([]byte(configJSON)); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Test getting existing config
	cfg, err := manager.GetDatabaseConfig("test-db")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.ID != "test-db" {
		t.Errorf("expected ID 'test-db', got '%s'", cfg.ID)
	}
	if cfg.Type != "postgres" {
		t.Errorf("expected type 'postgres', got '%s'", cfg.Type)
	}
	if cfg.Host != "test-host" {
		t.Errorf("expected host 'test-host', got '%s'", cfg.Host)
	}
	if cfg.MaxOpenConns != 10 {
		t.Errorf("expected MaxOpenConns 10, got %d", cfg.MaxOpenConns)
	}

	// Test getting non-existent config
	_, err = manager.GetDatabaseConfig("non-existent")
	if err == nil {
		t.Error("expected error for non-existent database, got nil")
	}
}

func TestBuildDatabaseConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    DatabaseConnectionConfig
		validate func(t *testing.T, cfg Config)
	}{
		{
			name: "postgres with ssl options",
			input: DatabaseConnectionConfig{
				ID:              "test-db",
				Type:            "postgres",
				Host:            "localhost",
				Port:            5432,
				User:            "testuser",
				Password:        "testpass",
				Name:            "testdb",
				SSLMode:         "require",
				SSLCert:         "/path/to/cert",
				SSLKey:          "/path/to/key",
				SSLRootCert:     "/path/to/root",
				ApplicationName: "test-app",
				ConnectTimeout:  30,
				QueryTimeout:    60,
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.Type != "postgres" {
					t.Errorf("expected type 'postgres', got '%s'", cfg.Type)
				}
				if cfg.SSLMode != "require" {
					t.Errorf("expected SSLMode 'require', got '%s'", cfg.SSLMode)
				}
				if cfg.SSLCert != "/path/to/cert" {
					t.Errorf("expected SSLCert '/path/to/cert', got '%s'", cfg.SSLCert)
				}
				if cfg.ApplicationName != "test-app" {
					t.Errorf("expected ApplicationName 'test-app', got '%s'", cfg.ApplicationName)
				}
				if cfg.ConnectTimeout != 30 {
					t.Errorf("expected ConnectTimeout 30, got %d", cfg.ConnectTimeout)
				}
				if cfg.QueryTimeout != 60 {
					t.Errorf("expected QueryTimeout 60, got %d", cfg.QueryTimeout)
				}
			},
		},
		{
			name: "mysql with connection pool settings",
			input: DatabaseConnectionConfig{
				ID:              "mysql-db",
				Type:            "mysql",
				Host:            "localhost",
				Port:            3306,
				User:            "testuser",
				Password:        "testpass",
				Name:            "testdb",
				MaxOpenConns:    25,
				MaxIdleConns:    10,
				ConnMaxLifetime: 300,
				ConnMaxIdleTime: 60,
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.Type != "mysql" {
					t.Errorf("expected type 'mysql', got '%s'", cfg.Type)
				}
				if cfg.MaxOpenConns != 25 {
					t.Errorf("expected MaxOpenConns 25, got %d", cfg.MaxOpenConns)
				}
				if cfg.MaxIdleConns != 10 {
					t.Errorf("expected MaxIdleConns 10, got %d", cfg.MaxIdleConns)
				}
				if cfg.ConnMaxLifetime != 300*time.Second {
					t.Errorf("expected ConnMaxLifetime 300s, got %v", cfg.ConnMaxLifetime)
				}
				if cfg.ConnMaxIdleTime != 60*time.Second {
					t.Errorf("expected ConnMaxIdleTime 60s, got %v", cfg.ConnMaxIdleTime)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildDatabaseConfig(tt.input)
			tt.validate(t, cfg)
		})
	}
}

func TestLazyLoadingConcurrency(t *testing.T) {
	manager := NewDBManager()
	// Enable lazy loading (direct field access to avoid logger dependency)
	manager.mu.Lock()
	manager.lazyLoading = true
	manager.mu.Unlock()

	// Load config with multiple databases
	configJSON := `{
		"connections": [
			{"id": "db1", "type": "postgres", "host": "localhost", "port": 5432, "user": "user", "password": "pass", "name": "db1"},
			{"id": "db2", "type": "postgres", "host": "localhost", "port": 5432, "user": "user", "password": "pass", "name": "db2"},
			{"id": "db3", "type": "postgres", "host": "localhost", "port": 5432, "user": "user", "password": "pass", "name": "db3"}
		]
	}`

	if err := manager.LoadConfig([]byte(configJSON)); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify lazy loading is enabled
	if !manager.IsLazyLoading() {
		t.Fatal("lazy loading should be enabled")
	}

	// Test concurrent access to GetDatabaseType (should not panic with race conditions)
	var wg sync.WaitGroup
	numGoroutines := 10
	numIterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// Rotate through database IDs
				dbID := []string{"db1", "db2", "db3"}[j%3]
				_, err := manager.GetDatabaseType(dbID)
				if err != nil {
					t.Errorf("GetDatabaseType failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

func TestGetConnectedDatabases(t *testing.T) {
	manager := NewDBManager()

	// Initially empty
	if len(manager.GetConnectedDatabases()) != 0 {
		t.Error("expected no connected databases initially")
	}

	// Load config
	configJSON := `{
		"connections": [
			{"id": "db1", "type": "postgres", "host": "localhost", "port": 5432, "user": "user", "password": "pass", "name": "db1"},
			{"id": "db2", "type": "postgres", "host": "localhost", "port": 5432, "user": "user", "password": "pass", "name": "db2"}
		]
	}`

	if err := manager.LoadConfig([]byte(configJSON)); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// With lazy loading enabled, should have no connections initially
	manager.mu.Lock()
	manager.lazyLoading = true
	manager.mu.Unlock()
	if len(manager.GetConnectedDatabases()) != 0 {
		t.Error("expected no connected databases with lazy loading before Connect()")
	}

	// Note: We skip calling Connect() since it requires:
	// 1. Logger to be initialized (would panic in test environment)
	// 2. Actual database connectivity (not available in unit tests)
	// The connection establishment logic is tested through integration tests
}

func TestConfigMarshaling(t *testing.T) {
	// Test that DatabaseConnectionConfig can be properly marshaled/unmarshaled
	original := DatabaseConnectionConfig{
		ID:                 "test-db",
		Type:               "postgres",
		Host:               "localhost",
		Port:               5432,
		User:               "testuser",
		Password:           "testpass",
		Name:               "testdb",
		SSLMode:            "require",
		SSLCert:            "/path/to/cert",
		SSLKey:             "/path/to/key",
		SSLRootCert:        "/path/to/root",
		ApplicationName:    "test-app",
		ConnectTimeout:     30,
		QueryTimeout:       60,
		TargetSessionAttrs: "read-write",
		Options:            map[string]string{"option1": "value1"},
		MaxOpenConns:       10,
		MaxIdleConns:       5,
		ConnMaxLifetime:    300,
		ConnMaxIdleTime:    60,
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	// Unmarshal back
	var unmarshaled DatabaseConnectionConfig
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	// Compare key fields
	if unmarshaled.ID != original.ID {
		t.Errorf("ID mismatch: expected %s, got %s", original.ID, unmarshaled.ID)
	}
	if unmarshaled.Type != original.Type {
		t.Errorf("Type mismatch: expected %s, got %s", original.Type, unmarshaled.Type)
	}
	if unmarshaled.SSLMode != original.SSLMode {
		t.Errorf("SSLMode mismatch: expected %s, got %s", original.SSLMode, unmarshaled.SSLMode)
	}
	if unmarshaled.MaxOpenConns != original.MaxOpenConns {
		t.Errorf("MaxOpenConns mismatch: expected %d, got %d", original.MaxOpenConns, unmarshaled.MaxOpenConns)
	}
}

// TestOracleConfigValidation tests Oracle-specific configuration validation
func TestOracleConfigValidation(t *testing.T) {
	manager := NewDBManager()

	tests := []struct {
		name      string
		config    string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid oracle config with service name",
			config: `{
				"connections": [{
					"id": "oracle1",
					"type": "oracle",
					"host": "localhost",
					"port": 1521,
					"user": "testuser",
					"password": "testpass",
					"name": "TESTDB",
					"options": {
						"service_name": "TESTDB"
					}
				}]
			}`,
			shouldErr: false,
		},
		{
			name: "valid oracle config with wallet",
			config: `{
				"connections": [{
					"id": "oracle_cloud",
					"type": "oracle",
					"user": "ADMIN",
					"password": "pass123",
					"name": "mydb_high",
					"options": {
						"wallet_location": "/app/wallet",
						"service_name": "mydb_high"
					}
				}]
			}`,
			shouldErr: false,
		},
		{
			name: "valid oracle config with TNS",
			config: `{
				"connections": [{
					"id": "oracle_tns",
					"type": "oracle",
					"user": "user",
					"password": "pass",
					"options": {
						"tns_entry": "PROD_DB",
						"tns_admin": "/opt/oracle/admin"
					}
				}]
			}`,
			shouldErr: false,
		},
		{
			name: "invalid database type",
			config: `{
				"connections": [{
					"id": "invalid",
					"type": "oracledb",
					"host": "localhost",
					"port": 1521,
					"user": "test",
					"password": "test",
					"name": "test"
				}]
			}`,
			shouldErr: true,
			errMsg:    "unsupported database type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.LoadConfig([]byte(tt.config))
			if tt.shouldErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && err.Error() == "" {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestBuildOracleConfig tests Oracle-specific config building
func TestBuildOracleConfig(t *testing.T) {
	input := DatabaseConnectionConfig{
		ID:       "oracle-test",
		Type:     "oracle",
		Host:     "localhost",
		Port:     1521,
		User:     "testuser",
		Password: "testpass",
		Name:     "TESTDB",
		Options: map[string]string{
			"service_name":     "TESTDB",
			"nls_lang":         "AMERICAN_AMERICA.AL32UTF8",
			"wallet_location":  "/app/wallet",
			"tns_admin":        "/opt/oracle/admin",
			"tns_entry":        "PROD_DB",
			"edition":          "ORA$BASE",
			"pooling":          "true",
			"standby_sessions": "true",
		},
		ConnectTimeout:  30,
		QueryTimeout:    60,
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 1800,
		ConnMaxIdleTime: 300,
	}

	cfg := buildDatabaseConfig(input)

	if cfg.Type != "oracle" {
		t.Errorf("expected type 'oracle', got '%s'", cfg.Type)
	}
	if cfg.ServiceName != "TESTDB" {
		t.Errorf("expected ServiceName 'TESTDB', got '%s'", cfg.ServiceName)
	}
	if cfg.NLSLang != "AMERICAN_AMERICA.AL32UTF8" {
		t.Errorf("expected NLSLang 'AMERICAN_AMERICA.AL32UTF8', got '%s'", cfg.NLSLang)
	}
	if cfg.WalletLocation != "/app/wallet" {
		t.Errorf("expected WalletLocation '/app/wallet', got '%s'", cfg.WalletLocation)
	}
	if cfg.TNSAdmin != "/opt/oracle/admin" {
		t.Errorf("expected TNSAdmin '/opt/oracle/admin', got '%s'", cfg.TNSAdmin)
	}
	if cfg.TNSEntry != "PROD_DB" {
		t.Errorf("expected TNSEntry 'PROD_DB', got '%s'", cfg.TNSEntry)
	}
	if cfg.Edition != "ORA$BASE" {
		t.Errorf("expected Edition 'ORA$BASE', got '%s'", cfg.Edition)
	}
	if !cfg.Pooling {
		t.Error("expected Pooling to be true")
	}
	if !cfg.StandbySessions {
		t.Error("expected StandbySessions to be true")
	}
	if cfg.ConnectTimeout != 30 {
		t.Errorf("expected ConnectTimeout 30, got %d", cfg.ConnectTimeout)
	}
	if cfg.QueryTimeout != 60 {
		t.Errorf("expected QueryTimeout 60, got %d", cfg.QueryTimeout)
	}
	if cfg.MaxOpenConns != 50 {
		t.Errorf("expected MaxOpenConns 50, got %d", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("expected MaxIdleConns 10, got %d", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 1800*time.Second {
		t.Errorf("expected ConnMaxLifetime 1800s, got %v", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 300*time.Second {
		t.Errorf("expected ConnMaxIdleTime 300s, got %v", cfg.ConnMaxIdleTime)
	}
}
