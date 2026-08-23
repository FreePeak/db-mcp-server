package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestMaxPacketProbe proves the probe reads the global setting.
func TestMaxPacketProbe(t *testing.T) {
	q := maxAllowedPacketQuery("mysql")
	if !strings.Contains(q, "max_allowed_packet") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if maxAllowedPacketQuery("postgres") != "" || maxAllowedPacketQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose max_allowed_packet")
	}
}

// TestMaxPacketVerdict proves the escalation ladder.
func TestMaxPacketVerdict(t *testing.T) {
	if got := maxAllowedPacketVerdict(64 * 1024 * 1024); got != "" {
		t.Fatalf("healthy size must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := maxAllowedPacketVerdict(1024 * 1024)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "gone away") {
		t.Fatalf("1MB not escalated:\n%s", got)
	}
	if got := maxAllowedPacketVerdict(0); got == "" || strings.Contains(got, "MB") {
		t.Fatalf("zero/unreadable misjudged:\n%s", got)
	}
}

// TestAuditMaxPacket_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditMaxPacket_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditMaxAllowedPacket(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
