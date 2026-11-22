package mcp

import "fmt"

// Grant is what one principal may do: the scopes it holds for every tool
// call, the databases it may use ("*" for all of them), and an optional
// EXPLAIN cost ceiling for its queries (0 means none). Agent keys and OIDC
// roles both carry a Grant.
type Grant struct {
	Scopes       []string `json:"scopes"`
	Databases    []string `json:"databases"`
	MaxQueryCost float64  `json:"max_query_cost,omitempty"`
}

// validateGrant rejects grants that would silently grant nothing or name an
// unknown scope. label names the grant in errors and never carries secrets.
func validateGrant(label string, g Grant) error {
	if len(g.Scopes) == 0 {
		return fmt.Errorf("%s: at least one scope is required", label)
	}
	for _, scope := range g.Scopes {
		if !isKnownScope(scope) {
			return fmt.Errorf("%s: unknown scope %q", label, scope)
		}
	}
	if len(g.Databases) == 0 {
		return fmt.Errorf(`%s: databases is required (list database ids, or "*" for all)`, label)
	}
	if g.MaxQueryCost < 0 {
		return fmt.Errorf("%s: max_query_cost must not be negative", label)
	}
	return nil
}

// extra returns the TokenInfo.Extra entries that the tool bridge reads:
// "databases" for the allowlist and "max_query_cost" for the caller's limit.
func (g Grant) extra() map[string]any {
	return map[string]any{"databases": g.Databases, "max_query_cost": g.MaxQueryCost}
}
