package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTCPKeepalivesProbe proves the probe reads the idle-keepalive
// setting in seconds and targets PostgreSQL only.
func TestTCPKeepalivesProbe(t *testing.T) {
	q := tcpKeepalivesProbe("postgres")
	if !strings.Contains(q, "current_setting('tcp_keepalives_idle')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if tcpKeepalivesProbe("mysql") != "" || tcpKeepalivesProbe("sqlite") != "" {
		t.Fatal("only postgres exposes tcp_keepalives_idle")
	}
}

// TestTCPKeepaliveVerdict proves the escalation ladder: dead clients
// holding slots is the failure mode, so anything above ~10 minutes
// escalates with the named fix.
func TestTCPKeepaliveVerdict(t *testing.T) {
	if got := tcpKeepaliveVerdict(60); got != "" {
		t.Fatalf("tight keepalive must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := tcpKeepaliveVerdict(600); got != "" {
		t.Fatalf("10-minute keepalive misjudged:\n%s", got)
	}
	stale := tcpKeepaliveVerdict(0)
	if !strings.Contains(stale, "WARNING") || !strings.Contains(stale, "dead client") {
		t.Fatalf("OS-default not escalated:\n%s", stale)
	}
	if !strings.Contains(stale, "ALTER SYSTEM SET tcp_keepalives_idle") {
		t.Fatalf("verdict must name the fix, got:\n%s", stale)
	}
	twoHours := tcpKeepaliveVerdict(7200)
	if !strings.Contains(twoHours, "2h0m0s") && !strings.Contains(twoHours, "7200s") {
		t.Fatalf("large value should render its duration, got:\n%s", twoHours)
	}
	if got := tcpKeepaliveVerdict(-1); !strings.Contains(got, "unreadable") {
		t.Fatalf("negative/unreadable misjudged:\n%s", got)
	}
}

// TestAuditTCPKeepalives_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditTCPKeepalives_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditTCPKeepalives(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
