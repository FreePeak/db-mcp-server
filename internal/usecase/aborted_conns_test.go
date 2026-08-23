package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestAbortedConnsProbe proves the probe reads both counters plus the
// connection denominator.
func TestAbortedConnsProbe(t *testing.T) {
	q := abortedConnsQuery("mysql")
	for _, want := range []string{"Aborted_connects", "Aborted_clients", "Connections"} {
		if !strings.Contains(q, want) {
			t.Fatalf("probe missing %q:\n%s", want, q)
		}
	}
	if abortedConnsQuery("postgres") != "" || abortedConnsQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose these status counters")
	}
}

// TestAbortedConnVerdict proves the escalations.
func TestAbortedConnVerdict(t *testing.T) {
	if got := abortedConnVerdict(1000, 30, 120); !strings.Contains(got, "WARNING") || !strings.Contains(got, "handshake") {
		t.Fatalf("high connect-failure ratio not escalated:\n%s", got)
	}
	if got := abortedConnVerdict(1000, 150, 5); !strings.Contains(got, "unclean") {
		t.Fatalf("high aborted-clients ratio not reported:\n%s", got)
	}
	if got := abortedConnVerdict(1000, 3, 8); got != "" {
		t.Fatalf("healthy ratios must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := abortedConnVerdict(0, 0, 0); got == "" {
		t.Fatal("zero-history must still render")
	}
}

// TestAuditAbortedConns_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditAbortedConns_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditAbortedConnections(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
