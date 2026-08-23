package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestStrictModeProbe proves the probe reads the global sql_mode.
func TestStrictModeProbe(t *testing.T) {
	q := sqlModeQuery("mysql")
	if !strings.Contains(q, "sql_mode") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if sqlModeQuery("postgres") != "" || sqlModeQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose sql_mode")
	}
}

// TestStrictModeVerdict proves the corruption-risk escalation.
func TestStrictModeVerdict(t *testing.T) {
	if got := strictModeVerdict("STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO"); got == "" || strings.Contains(got, "WARNING") {
		t.Fatalf("strict mode misjudged:\n%s", got)
	}
	got := strictModeVerdict("")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "silently") {
		t.Fatalf("empty mode not escalated:\n%s", got)
	}
	if got := strictModeVerdict("NO_ENGINE_SUBSTITUTION"); !strings.Contains(got, "WARNING") || !strings.Contains(got, "truncat") {
		t.Fatalf("non-strict mode not escalated:\n%s", got)
	}
}

// TestAuditStrictMode_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditStrictMode_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditStrictMode(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
