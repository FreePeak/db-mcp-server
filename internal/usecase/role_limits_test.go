package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestRoleLimitCatalog proves the SELECT joins pg_roles limits to live
// per-role session counts, only for login roles with finite limits.
func TestRoleLimitCatalog(t *testing.T) {
	q := roleLimitQuery("postgres")
	for _, want := range []string{"pg_roles", "rolconnlimit", "pg_stat_activity", "rolcanlogin"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if roleLimitQuery("mysql") != "" || roleLimitQuery("sqlite") != "" {
		t.Fatal("only postgres exposes per-role connection limits")
	}
}

// TestRoleLimitVerdict proves the at-limit escalation and the skip of
// roles far below their cap.
func TestRoleLimitVerdict(t *testing.T) {
	if got := roleLimitLine("svc_app", 10, 10); !strings.Contains(got, "AT LIMIT") || !strings.Contains(got, "rejecting") {
		t.Fatalf("at-limit not escalated:\n%s", got)
	}
	if got := roleLimitLine("svc_app", 10, 9); !strings.Contains(got, "WARNING") {
		t.Fatalf("near-limit not warned:\n%s", got)
	}
	if got := roleLimitLine("svc_app", 10, 3); got != "" {
		t.Fatalf("comfortable role should render no line: %q", got)
	}
}

// TestListRoleConnectionLimits_Unsupported proves unsupported engines
// get an explicit error.
func TestListRoleConnectionLimits_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListRoleConnectionLimits(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
