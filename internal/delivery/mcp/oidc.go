package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	sdkAuth "github.com/modelcontextprotocol/go-sdk/auth"
)

// NewOIDCVerifier validates bearer tokens against an OIDC provider's
// discovery/JWKS metadata and maps common scope claim shapes into MCP
// TokenInfo. Audience validation is mandatory. When roles is set, the scopes
// and Extra of each token come from its resolved role grant instead.
func NewOIDCVerifier(ctx context.Context, issuerURL, audience string, roles *OIDCRoles) (sdkAuth.TokenVerifier, error) {
	issuerURL = strings.TrimSpace(issuerURL)
	audience = strings.TrimSpace(audience)
	if issuerURL == "" || audience == "" {
		return nil, fmt.Errorf("OIDC issuer and audience are required")
	}
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: audience})
	return func(ctx context.Context, token string, _ *http.Request) (*sdkAuth.TokenInfo, error) {
		idToken, err := verifier.Verify(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", sdkAuth.ErrInvalidToken, err)
		}
		claims := map[string]interface{}{}
		if err := idToken.Claims(&claims); err != nil {
			return nil, fmt.Errorf("%w: invalid OIDC claims: %v", sdkAuth.ErrInvalidToken, err)
		}
		if idToken.Subject == "" {
			return nil, fmt.Errorf("%w: OIDC subject is missing", sdkAuth.ErrInvalidToken)
		}
		scopes := scopesFromClaims(claims)
		var extra map[string]any
		if roles != nil {
			grant := roles.Resolve(claims)
			scopes, extra = grant.Scopes, grant.extra()
		}
		return &sdkAuth.TokenInfo{
			UserID:     idToken.Subject,
			Scopes:     scopes,
			Expiration: idToken.Expiry,
			Extra:      extra,
		}, nil
	}, nil
}

func scopesFromClaims(claims map[string]interface{}) []string {
	seen := make(map[string]struct{})
	var scopes []string
	for _, key := range []string{"scope", "scp", "permissions", "db_scopes"} {
		for _, scope := range claimStrings(claims[key]) {
			if _, ok := seen[scope]; !ok {
				seen[scope] = struct{}{}
				scopes = append(scopes, scope)
			}
		}
	}
	return scopes
}

// claimStrings flattens a claim that is a space-separated string or an array
// of such strings into its words, in order.
func claimStrings(value interface{}) []string {
	switch typed := value.(type) {
	case string:
		return strings.Fields(typed)
	case []interface{}:
		var words []string
		for _, item := range typed {
			words = append(words, claimStrings(item)...)
		}
		return words
	case []string:
		var words []string
		for _, item := range typed {
			words = append(words, claimStrings(item)...)
		}
		return words
	}
	return nil
}
