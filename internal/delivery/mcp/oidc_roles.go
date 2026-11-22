package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// OIDCRoles maps one token claim onto grants. The grant for a token is the
// first role, in file order, whose value appears in that claim.
type OIDCRoles struct {
	Claim string     `json:"claim"`
	Roles []OIDCRole `json:"roles"`
}

// OIDCRole is one claim value and the grant it carries. Its grant fields sit
// beside value in the JSON document.
type OIDCRole struct {
	Value string `json:"value"`
	Grant
}

// LoadOIDCRoles reads and validates the DB_MCP_OIDC_ROLES_FILE JSON document.
func LoadOIDCRoles(path string) (*OIDCRoles, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OIDC roles file: %w", err)
	}
	var roles OIDCRoles
	if err := json.Unmarshal(data, &roles); err != nil {
		return nil, fmt.Errorf("parse OIDC roles file: %w", err)
	}
	roles.Claim = strings.TrimSpace(roles.Claim)
	if roles.Claim == "" {
		return nil, fmt.Errorf("OIDC roles file: claim is required")
	}
	seen := make(map[string]struct{}, len(roles.Roles))
	for i, role := range roles.Roles {
		if strings.TrimSpace(role.Value) == "" {
			return nil, fmt.Errorf("OIDC role #%d: value is required", i+1)
		}
		if _, dup := seen[role.Value]; dup {
			return nil, fmt.Errorf("OIDC role %q: duplicate value", role.Value)
		}
		seen[role.Value] = struct{}{}
		if err := validateGrant(fmt.Sprintf("OIDC role %q", role.Value), role.Grant); err != nil {
			return nil, err
		}
	}
	return &roles, nil
}

// Resolve returns the grant for a verified token's claims. A claim that
// matches no role yields a grant with no scopes and an empty database list,
// which denies every tool call.
func (r *OIDCRoles) Resolve(claims map[string]any) Grant {
	values := claimStrings(claims[r.Claim])
	for _, role := range r.Roles {
		if slices.Contains(values, role.Value) {
			return role.Grant
		}
	}
	return Grant{Databases: []string{}}
}
