package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestPasswordAuthProves probes read the server default and the per-role
// stored-hash format.
func TestPasswordAuthProbes(t *testing.T) {
	q := passwordEncryptionQuery("postgres")
	if !strings.Contains(q, "password_encryption") {
		t.Fatalf("server-default probe wrong:\n%s", q)
	}
	q = md5RolesQuery("postgres")
	for _, want := range []string{"pg_authid", "rolcanlogin", "md5"} {
		if !strings.Contains(q, want) {
			t.Fatalf("role probe missing %q:\n%s", want, q)
		}
	}
	if passwordEncryptionQuery("mysql") != "" || md5RolesQuery("sqlite") != "" {
		t.Fatal("only postgres exposes password_encryption/pg_authid")
	}
}

// TestEncryptionVerdict proves the server-default classifier.
func TestEncryptionVerdict(t *testing.T) {
	if got := encryptionVerdict("md5"); !strings.Contains(got, "WARNING") || !strings.Contains(got, "scram-sha-256") {
		t.Fatalf("md5 default not escalated:\n%s", got)
	}
	if got := encryptionVerdict("scram-sha-256"); got == "" || strings.Contains(got, "WARNING") {
		t.Fatalf("scram default misjudged:\n%s", got)
	}
}

// TestAuditPasswordAuth_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditPasswordAuth_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditPasswordAuth(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
