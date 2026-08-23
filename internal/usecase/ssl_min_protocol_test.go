package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSSLMinProtocolProbe proves the probe reads both the TLS floor
// and whether ssl is even on, targeting PostgreSQL only.
func TestSSLMinProtocolProbe(t *testing.T) {
	q := sslMinProtocolProbe("postgres")
	for _, frag := range []string{"ssl_min_protocol_version", "'ssl'"} {
		if !strings.Contains(q, frag) {
			t.Fatalf("probe missing %s:\n%s", frag, q)
		}
	}
	if sslMinProtocolProbe("mysql") != "" || sslMinProtocolProbe("sqlite") != "" {
		t.Fatal("only postgres exposes ssl_min_protocol_version")
	}
}

// TestSSLMinProtocolVerdict proves the escalation ladder.
func TestSSLMinProtocolVerdict(t *testing.T) {
	if got := sslMinProtocolVerdict(true, "TLSv1.2"); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := sslMinProtocolVerdict(true, "TLSv1.3"); got != "" {
		t.Fatalf("TLSv1.3 misjudged:\n%s", got)
	}
	deprecated := sslMinProtocolVerdict(true, "TLSv1")
	if !strings.Contains(deprecated, "WARNING") || !strings.Contains(deprecated, "PCI-DSS") {
		t.Fatalf("deprecated protocol not escalated:\n%s", deprecated)
	}
	if !strings.Contains(deprecated, "ALTER SYSTEM SET ssl_min_protocol_version") {
		t.Fatalf("verdict must name the fix, got:\n%s", deprecated)
	}
	off := sslMinProtocolVerdict(false, "")
	if !strings.Contains(off, "WARNING") || !strings.Contains(off, "unencrypted") {
		t.Fatalf("ssl=off not escalated:\n%s", off)
	}
	if got := sslMinProtocolVerdict(true, "weird"); !strings.Contains(got, "unreadable") {
		t.Fatalf("unknown value misjudged:\n%s", got)
	}
}

// TestAuditSSLMinProtocol_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditSSLMinProtocol_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditSSLMinProtocol(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
