package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSyncCommitProbe proves the probe targets PostgreSQL only.
func TestSyncCommitProbe(t *testing.T) {
	q := syncCommitQuery("postgres")
	if !strings.Contains(q, "synchronous_commit") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if syncCommitQuery("mysql") != "" || syncCommitQuery("sqlite") != "" {
		t.Fatal("only postgres exposes synchronous_commit")
	}
}

// TestSyncCommitVerdict proves the escalation ladder across modes.
func TestSyncCommitVerdict(t *testing.T) {
	if got := syncCommitVerdict("on"); got != "" {
		t.Fatalf("default must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := syncCommitVerdict("remote_apply"); !strings.Contains(got, "durable") {
		t.Fatalf("remote_apply misjudged:\n%s", got)
	}
	got := syncCommitVerdict("off")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "committed") || !strings.Contains(got, "lost") {
		t.Fatalf("off not escalated:\n%s", got)
	}
	if !strings.Contains(got, "ALTER SYSTEM") && !strings.Contains(got, "SET") {
		t.Fatalf("off warning lacks a fix:\n%s", got)
	}
	if got := syncCommitVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty value misjudged:\n%s", got)
	}
}

// TestAuditSyncCommit_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditSyncCommit_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditSyncCommit(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
