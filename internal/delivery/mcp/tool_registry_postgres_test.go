package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
)

// TestRegisterDatabaseTools_PostgresDoesNotCallGetDatabaseInfo locks in the
// fix for FreePeak/db-mcp-server issue #22: PostgreSQL 16+ rejects the older
// schema-introspection path with "pq: unrecognized configuration parameter
// \"tables\"". The current registration path uses GetDatabaseType and skips
// the introspective GetDatabaseInfo call for postgres databases, so a live
// postgres 16 connection is no longer required during startup.
//
// The previous code path called GetDatabaseInfo unconditionally; that call has
// been removed from the postgres branch in tool_registry.go (see the early
// "special handling for PostgreSQL" block). This test asserts the contract:
// GetDatabaseInfo must not be invoked for a postgres database during
// registration when lazy loading is enabled (which is the only supported mode
// in this minimal harness).
func TestRegisterDatabaseTools_PostgresDoesNotCallGetDatabaseInfo(t *testing.T) {
	mockUseCase := new(MockDatabaseUseCase)
	mockUseCase.On("GetDatabaseType", "pg16").Return("postgres", nil)
	mockUseCase.On("IsLazyLoading").Return(true)

	registry := &ToolRegistry{
		server:          nil,
		databaseUseCase: mockUseCase,
		factory:         NewToolTypeFactory(),
		unifiedMode:     false,
	}

	// Recovery guards the nil server: the contract under test is the call
	// ordering, not a successful end-to-end registration.
	defer func() {
		_ = recover()
		mockUseCase.AssertNotCalled(t, "GetDatabaseInfo", mock.Anything)
		mockUseCase.AssertCalled(t, "GetDatabaseType", "pg16")
	}()
	_ = registry.registerDatabaseTools(context.Background(), "pg16")
}
