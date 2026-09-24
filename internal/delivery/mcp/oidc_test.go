package mcp

import (
	"context"
	"reflect"
	"testing"
)

func TestScopesFromClaims(t *testing.T) {
	claims := map[string]interface{}{
		"scope":       "db:read db:write",
		"scp":         []interface{}{"db:read", "db:custom"},
		"permissions": []string{"db:admin"},
	}
	got := scopesFromClaims(claims)
	want := []string{"db:read", "db:write", "db:custom", "db:admin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected scopes: got %v, want %v", got, want)
	}
}

func TestNewOIDCVerifierRequiresIssuerAndAudience(t *testing.T) {
	if _, err := NewOIDCVerifier(context.Background(), "", "audience"); err == nil {
		t.Fatal("expected missing issuer error")
	}
	if _, err := NewOIDCVerifier(context.Background(), "https://issuer.example", ""); err == nil {
		t.Fatal("expected missing audience error")
	}
}
