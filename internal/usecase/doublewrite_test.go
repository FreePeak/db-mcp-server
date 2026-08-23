package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestDoublewriteProbe proves the probe targets MySQL/MariaDB only.
func TestDoublewriteProbe(t *testing.T) {
	if q := doublewriteQuery("mysql"); !strings.Contains(q, "innodb_doublewrite") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if doublewriteQuery("postgres") != "" || doublewriteQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb exposes innodb_doublewrite")
	}
}

// TestDoublewriteVerdict proves the escalation ladder.
func TestDoublewriteVerdict(t *testing.T) {
	for _, ok := range []string{"ON", "1"} {
		if got := doublewriteVerdict(ok); got != "" {
			t.Fatalf("healthy %q must render empty, got:\n%s", ok, got)
		}
	}
	got := doublewriteVerdict("OFF")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "torn") {
		t.Fatalf("off not escalated:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "restart") && !strings.Contains(strings.ToLower(got), "config") {
		t.Fatalf("warning must note config/restart, got:\n%s", got)
	}
	if got := doublewriteVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty misjudged:\n%s", got)
	}
}

// TestAuditDoublewrite_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditDoublewrite_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditDoublewrite(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
