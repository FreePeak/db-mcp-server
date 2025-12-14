package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	// Setup
	err := os.Setenv("TEST_ENV_VAR", "test_value")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		err := os.Unsetenv("TEST_ENV_VAR")
		if err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	// Test with existing env var
	value := getEnv("TEST_ENV_VAR", "default_value")
	assert.Equal(t, "test_value", value)

	// Test with non-existing env var
	value = getEnv("NON_EXISTING_VAR", "default_value")
	assert.Equal(t, "default_value", value)
}

func TestLoadConfig(t *testing.T) {
	// Clear any environment variables that might affect the test
	vars := []string{
		"SERVER_PORT", "TRANSPORT_MODE", "LOG_LEVEL", "DB_TYPE",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
	}

	for _, v := range vars {
		err := os.Unsetenv(v)
		if err != nil {
			t.Logf("Failed to unset %s: %v", v, err)
		}
	}

	// Get current working directory and handle .env file
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	envPath := filepath.Join(cwd, ".env")
	tempPath := filepath.Join(cwd, ".env.bak")

	// Save existing .env if it exists
	envExists := false
	if _, err := os.Stat(envPath); err == nil {
		envExists = true
		err = os.Rename(envPath, tempPath)
		if err != nil {
			t.Fatalf("Failed to rename .env file: %v", err)
		}
		// Restore at the end
		defer func() {
			if envExists {
				if err := os.Rename(tempPath, envPath); err != nil {
					t.Logf("Failed to restore .env file: %v", err)
				}
			}
		}()
	}

	// Test with default values (no .env file and no environment variables)
	config, err := LoadConfig("")
	assert.NoError(t, err)
	assert.Equal(t, 9090, config.ServerPort)
	assert.Equal(t, "sse", config.TransportMode)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, "mysql", config.DBConfig.Type)
	assert.Equal(t, "localhost", config.DBConfig.Host)
	assert.Equal(t, 3306, config.DBConfig.Port)
	assert.Equal(t, "", config.DBConfig.User)
	assert.Equal(t, "", config.DBConfig.Password)
	assert.Equal(t, "", config.DBConfig.Name)

	// Test with custom environment variables
	err = os.Setenv("SERVER_PORT", "8080")
	if err != nil {
		t.Fatalf("Failed to set SERVER_PORT: %v", err)
	}
	err = os.Setenv("TRANSPORT_MODE", "stdio")
	if err != nil {
		t.Fatalf("Failed to set TRANSPORT_MODE: %v", err)
	}
	err = os.Setenv("LOG_LEVEL", "debug")
	if err != nil {
		t.Fatalf("Failed to set LOG_LEVEL: %v", err)
	}
	err = os.Setenv("DB_TYPE", "postgres")
	if err != nil {
		t.Fatalf("Failed to set DB_TYPE: %v", err)
	}
	err = os.Setenv("DB_HOST", "db.example.com")
	if err != nil {
		t.Fatalf("Failed to set DB_HOST: %v", err)
	}
	err = os.Setenv("DB_PORT", "5432")
	if err != nil {
		t.Fatalf("Failed to set DB_PORT: %v", err)
	}
	err = os.Setenv("DB_USER", "testuser")
	if err != nil {
		t.Fatalf("Failed to set DB_USER: %v", err)
	}
	err = os.Setenv("DB_PASSWORD", "testpass")
	if err != nil {
		t.Fatalf("Failed to set DB_PASSWORD: %v", err)
	}
	err = os.Setenv("DB_NAME", "testdb")
	if err != nil {
		t.Fatalf("Failed to set DB_NAME: %v", err)
	}

	defer func() {
		for _, v := range vars {
			if cleanupErr := os.Unsetenv(v); cleanupErr != nil {
				t.Logf("Failed to unset %s: %v", v, cleanupErr)
			}
		}
	}()

	config, err = LoadConfig("")
	assert.NoError(t, err)
	assert.Equal(t, 8080, config.ServerPort)
	assert.Equal(t, "stdio", config.TransportMode)
	assert.Equal(t, "debug", config.LogLevel)
	assert.Equal(t, "postgres", config.DBConfig.Type)
	assert.Equal(t, "db.example.com", config.DBConfig.Host)
	assert.Equal(t, 5432, config.DBConfig.Port)
	assert.Equal(t, "testuser", config.DBConfig.User)
	assert.Equal(t, "testpass", config.DBConfig.Password)
	assert.Equal(t, "testdb", config.DBConfig.Name)
}
