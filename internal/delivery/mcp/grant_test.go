package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateGrant_RejectsUnknownScope(t *testing.T) {
	err := validateGrant(`agent key "agent-a"`, Grant{Scopes: []string{"db:root"}, Databases: []string{"*"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `agent key "agent-a": unknown scope "db:root"`)
}

func TestValidateGrant_RejectsEmptyDatabases(t *testing.T) {
	err := validateGrant(`OIDC role "db-analysts"`, Grant{Scopes: []string{"db:read"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `OIDC role "db-analysts": databases is required`)
}

func TestValidateGrant_RejectsNegativeCost(t *testing.T) {
	err := validateGrant(`agent key "agent-a"`, Grant{Scopes: []string{"db:read"}, Databases: []string{"*"}, MaxQueryCost: -1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `agent key "agent-a": max_query_cost must not be negative`)
}
