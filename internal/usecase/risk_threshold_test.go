package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestRiskThreshold_ConfigurableWarnLevel proves operators can raise/lower
// the post-execution warning threshold from the default high.
func TestRiskThreshold_ConfigurableWarnLevel(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE logs (id INTEGER PRIMARY KEY, msg TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	stmt := `INSERT INTO logs (msg) VALUES ('x')` // medium risk

	// Default threshold (high): medium INSERT stays clean.
	out, _ := uc.ExecuteStatement(context.Background(), "db1", stmt, nil)
	if strings.Contains(out, "Risk notice") {
		t.Fatalf("default threshold must not warn on medium:\n%s", out)
	}

	// Lower threshold to medium: same INSERT now warns.
	uc.SetRiskWarnAt("medium")
	out, _ = uc.ExecuteStatement(context.Background(), "db1", stmt, nil)
	if !strings.Contains(out, "Risk notice") {
		t.Fatalf("medium threshold should warn on INSERT:\n%s", out)
	}

	// Raise to critical: clean again.
	uc.SetRiskWarnAt("critical")
	out, _ = uc.ExecuteStatement(context.Background(), "db1", stmt, nil)
	if strings.Contains(out, "Risk notice") {
		t.Fatalf("critical threshold must not warn on medium:\n%s", out)
	}

	// Invalid value falls back to high.
	uc.SetRiskWarnAt("banana")
	out, _ = uc.ExecuteStatement(context.Background(), "db1", stmt, nil)
	if strings.Contains(out, "Risk notice") {
		t.Fatal("invalid threshold falls back to high; medium must not warn")
	}
}
