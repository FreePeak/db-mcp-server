package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestListCustomTypes proves cycle 85: user-defined enum/composite types
// are listed; engines without them report a clean empty list.
func TestListCustomTypes(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ListCustomTypes(context.Background(), "db1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "No custom types") {
		t.Fatalf("expected clean-empty report:\n%s", out)
	}
}
