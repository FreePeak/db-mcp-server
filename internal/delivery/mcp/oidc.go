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
// TokenInfo. Audience validation is mandatory.
func NewOIDCVerifier(ctx context.Context, issuerURL, audience string) (sdkAuth.TokenVerifier, error) {
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
		return &sdkAuth.TokenInfo{
			UserID:     idToken.Subject,
			Scopes:     scopesFromClaims(claims),
			Expiration: idToken.Expiry,
		}, nil
	}, nil
}

func scopesFromClaims(claims map[string]interface{}) []string {
	seen := make(map[string]struct{})
	var scopes []string
	var appendValues func(interface{})
	appendValues = func(value interface{}) {
		switch typed := value.(type) {
		case string:
			for _, scope := range strings.Fields(typed) {
				if scope != "" {
					if _, ok := seen[scope]; !ok {
						seen[scope] = struct{}{}
						scopes = append(scopes, scope)
					}
				}
			}
		case []interface{}:
			for _, item := range typed {
				appendValues(item)
			}
		case []string:
			for _, item := range typed {
				appendValues(item)
			}
		}
	}
	for _, key := range []string{"scope", "scp", "permissions", "db_scopes"} {
		appendValues(claims[key])
	}
	return scopes
}
