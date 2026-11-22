package usecase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// readAuditLines parses every line of an audit file into records; any
// partial or corrupt line fails the test — interleaved writes would show
// up exactly there.
func readAuditLines(t *testing.T, path string) []auditRecord {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	var out []auditRecord
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		var rec auditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("corrupt audit line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

// TestAudit_QueryAndStatementEntries runs a query, a statement, and a
// failing statement through a real SQLite database with auditing enabled,
// then checks each record: op, database, statement text, and error capture.
func TestAudit_QueryAndStatementEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	ctx := context.Background()

	if _, err := uc.ExecuteQuery(ctx, "db", "SELECT v FROM t", nil); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if _, err := uc.ExecuteStatement(ctx, "db", "INSERT INTO t VALUES ('x')", nil); err != nil {
		t.Fatalf("statement failed: %v", err)
	}
	_, _ = uc.ExecuteQuery(ctx, "db", "SELECT nope FROM missing_table", nil) // expected failure

	recs := readAuditLines(t, path)
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d: %+v", len(recs), recs)
	}
	wantOps := []string{"query", "execute", "query"}
	for i, rec := range recs {
		if rec.Op != wantOps[i] || rec.Database != "db" || rec.Statement == "" || rec.DurMS < 0 {
			t.Errorf("record %d unexpected: %+v", i, rec)
		}
		if rec.TS == "" {
			t.Errorf("record %d missing timestamp", i)
		}
	}
	if recs[2].Error == "" {
		t.Errorf("failing statement must carry its error, got %+v", recs[2])
	}
}

// TestAudit_ConcurrentWritesStayLineIntact hammers the sink from many
// goroutines; the mutex must keep every JSONL line whole.
func TestAudit_ConcurrentWritesStayLineIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	const goroutines, perG = 25, 20
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				audit.record(context.Background(), "query", "db", "SELECT 1", 0, nil, nil)
			}
		}()
	}
	wg.Wait()

	recs := readAuditLines(t, path)
	if len(recs) != goroutines*perG {
		t.Errorf("expected %d intact lines, got %d", goroutines*perG, len(recs))
	}
}

// TestAudit_TruncatesHugeStatements bounds single records so a
// pathological payload cannot balloon the audit file.
func TestAudit_TruncatesHugeStatements(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	huge := strings.Repeat("x", 50_000)
	audit.record(context.Background(), "query", "db", huge, 0, nil, nil)

	recs := readAuditLines(t, path)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if !strings.HasSuffix(recs[0].Statement, "...[truncated]") || len(recs[0].Statement) > auditStatementCap+20 {
		t.Errorf("statement not truncated to cap, length %d", len(recs[0].Statement))
	}
}

// TestAudit_DisabledIsNoop verifies the disabled sink drops records
// silently instead of erroring or writing anywhere.
func TestAudit_DisabledIsNoop(t *testing.T) {
	DisableAuditLog()
	audit.record(context.Background(), "query", "db", "SELECT 1", 0, nil, nil) // must not panic or write
}

// TestAudit_EnvVarSinkOpensOnFirstRecord covers the DB_MCP_AUDIT_LOG path
// on its own: no EnableAuditLog call consumes the lazy initializer first, so
// the first record must open the env sink without re-entering that Once.
func TestAudit_EnvVarSinkOpensOnFirstRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("DB_MCP_AUDIT_LOG", path)
	DisableAuditLog()
	audit.once = sync.Once{}

	first := make(chan struct{})
	go func() {
		audit.record(context.Background(), "query", "db", "SELECT 1", 0, nil, nil)
		close(first)
	}()
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("first audited record blocked: DB_MCP_AUDIT_LOG sink never opened")
	}
	// Registered after the wait: a hung record holds audit.mu, so a cleanup
	// that calls DisableAuditLog would block on the failure path too.
	t.Cleanup(func() {
		DisableAuditLog()
		audit.once = sync.Once{}
	})
	if recs := readAuditLines(t, path); len(recs) != 1 {
		t.Fatalf("expected 1 record after first call, got %d", len(recs))
	}

	audit.record(context.Background(), "query", "db", "SELECT 2", 0, nil, nil)
	if recs := readAuditLines(t, path); len(recs) != 2 {
		t.Fatalf("expected 2 records after second call, got %d", len(recs))
	}
}

// readonlyDB forces IsReadOnly so the rejection path is auditable.
type readonlyDB struct {
	domain.Database
}

func (r *readonlyDB) IsReadOnly() bool { return true }

// TestAudit_RecordsRejectedWrites locks in that attempted writes against a
// read-only database are audited even though nothing reached the engine —
// those attempts are precisely what an operator needs to see.
func TestAudit_RecordsRejectedWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &readonlyDB{Database: &sqliteDB{db: raw}}, dbType: "sqlite"})

	_, qErr := uc.ExecuteQuery(context.Background(), "db", "DELETE FROM t", nil)
	_, sErr := uc.ExecuteStatement(context.Background(), "db", "INSERT INTO t VALUES ('x')", nil)
	if qErr == nil || sErr == nil {
		t.Fatalf("read-only enforcement must reject both paths, got q=%v s=%v", qErr, sErr)
	}

	recs := readAuditLines(t, path)
	if len(recs) != 2 {
		t.Fatalf("expected 2 audited rejections, got %d: %+v", len(recs), recs)
	}
	for i, rec := range recs {
		if rec.Error == "" || !strings.Contains(rec.Error, "read-only") {
			t.Errorf("rejection %d must carry the guardrail error: %+v", i, rec)
		}
	}
}

// TestAudit_RecordsCallerAndRows checks that the caller set on the context
// and the rows returned or affected reach the record for every entry point.
func TestAudit_RecordsCallerAndRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t VALUES ('a'), ('b')`); err != nil {
		t.Fatalf("seed rows failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	ctx := WithCaller(context.Background(), Caller{Principal: "user@example.com", Session: "mcp-session-example"})

	if _, err := uc.ExecuteQuery(ctx, "db", "SELECT v FROM t", nil); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if _, err := uc.ExecuteStatement(ctx, "db", "UPDATE t SET v = 'x'", nil); err != nil {
		t.Fatalf("statement failed: %v", err)
	}
	_, meta, err := uc.ExecuteTransaction(ctx, "db", "begin", "", "", nil, false)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	txID, _ := meta["transactionId"].(string)
	if _, _, err := uc.ExecuteTransaction(ctx, "db", "execute", txID, "INSERT INTO t VALUES ('c')", nil, false); err != nil {
		t.Fatalf("tx execute failed: %v", err)
	}
	if _, _, err := uc.ExecuteTransaction(ctx, "db", "commit", txID, "", nil, false); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	recs := readAuditLines(t, path)
	wantOps := []string{"query", "execute", "tx_begin", "tx_execute", "tx_commit"}
	wantRows := []int{2, 2, -1, 1, -1} // -1: no row count expected
	if len(recs) != len(wantOps) {
		t.Fatalf("expected %d records, got %d: %+v", len(wantOps), len(recs), recs)
	}
	for i, rec := range recs {
		if rec.Op != wantOps[i] {
			t.Errorf("record %d op = %q, want %q", i, rec.Op, wantOps[i])
		}
		if rec.Principal != "user@example.com" || rec.Session != "mcp-session-example" {
			t.Errorf("record %d missing caller: %+v", i, rec)
		}
		switch {
		case wantRows[i] < 0 && rec.Rows != nil:
			t.Errorf("record %d must omit rows, got %d", i, *rec.Rows)
		case wantRows[i] >= 0 && (rec.Rows == nil || *rec.Rows != wantRows[i]):
			t.Errorf("record %d rows = %v, want %d", i, rec.Rows, wantRows[i])
		}
	}
}

// TestAudit_OmitsCallerAndRowsWhenUnknown keeps the new fields out of the
// JSON when there is no caller and no row count, so existing consumers see
// the same record shape as before.
func TestAudit_OmitsCallerAndRowsWhenUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	audit.record(context.Background(), "query", "db", "SELECT 1", 0, nil, nil)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("corrupt audit line: %v", err)
	}
	for _, key := range []string{"principal", "session", "rows", "tool"} {
		if _, ok := fields[key]; ok {
			t.Errorf("field %q must be omitted when unknown, line: %s", key, raw)
		}
	}
}

// TestAudit_TruncatedQueryRecordsCappedRows checks that a query cut off by
// max_rows records the rows shown, not the rows the query would have matched.
func TestAudit_TruncatedQueryRecordsCappedRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	db := cappedSQLite{sqliteDB: &sqliteDB{db: openSQLiteForTest(t)}, maxRows: 3}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "sqlite"})
	ctx := WithCaller(context.Background(), Caller{Principal: "user@example.com", Session: "mcp-session-example"})

	if _, err := uc.ExecuteQueryMasked(ctx, "db", "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 10) SELECT x FROM c", nil, false); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	recs := readAuditLines(t, path)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d: %+v", len(recs), recs)
	}
	if recs[0].Rows == nil || *recs[0].Rows != 3 {
		t.Errorf("truncated query rows = %v, want 3 (max_rows)", recs[0].Rows)
	}
}

// TestAudit_FailedQueryOmitsRows checks that a query that errors records the
// error and leaves out rows, since no rows were shown.
func TestAudit_FailedQueryOmitsRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: openSQLiteForTest(t)}, dbType: "sqlite"})
	if _, err := uc.ExecuteQuery(context.Background(), "db", "SELECT v FROM missing_table", nil); err == nil {
		t.Fatal("expected query against a missing table to fail")
	}

	recs := readAuditLines(t, path)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d: %+v", len(recs), recs)
	}
	if recs[0].Error == "" {
		t.Error("failed query record must carry the error")
	}
	if recs[0].Rows != nil {
		t.Errorf("failed query record must omit rows, got %d", *recs[0].Rows)
	}
}

// TestAudit_RollbackFailureOmitsRows checks that a query whose cleanup fails
// after its rows were formatted is recorded as failed without a row count,
// because the call returns an error.
func TestAudit_RollbackFailureOmitsRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	db := &fakeDB{readOnly: true, beginTx: &fakeTx{rollbackErr: true}}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "postgres"})
	if _, err := uc.ExecuteQuery(context.Background(), "db", "SELECT 1", nil); err == nil {
		t.Fatal("expected rollback failure to surface as an error")
	}

	recs := readAuditLines(t, path)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d: %+v", len(recs), recs)
	}
	if recs[0].Error == "" {
		t.Error("record must carry the rollback error")
	}
	if recs[0].Rows != nil {
		t.Errorf("failed call must omit rows, got %d", *recs[0].Rows)
	}
}

// TestAudit_UnauthenticatedQueryOmitsPrincipalAndSession checks the stdio or
// unauthenticated path end to end: no caller on the context means the record
// carries neither principal nor session.
func TestAudit_UnauthenticatedQueryOmitsPrincipalAndSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: openSQLiteForTest(t)}, dbType: "sqlite"})
	if _, err := uc.ExecuteQuery(context.Background(), "db", "SELECT 1", nil); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("corrupt audit line: %v", err)
	}
	for _, key := range []string{"principal", "session"} {
		if _, ok := fields[key]; ok {
			t.Errorf("field %q must be omitted for an unauthenticated call, line: %s", key, raw)
		}
	}
}

// TestAudit_RecordsToolFromCaller checks that the MCP tool name carried on the
// caller reaches the record.
func TestAudit_RecordsToolFromCaller(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: openSQLiteForTest(t)}, dbType: "sqlite"})
	ctx := WithCaller(context.Background(), Caller{Principal: "user@example.com", Tool: "query_orders"})
	if _, err := uc.ExecuteQuery(ctx, "orders", "SELECT 1", nil); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	recs := readAuditLines(t, path)
	if len(recs) != 1 || recs[0].Tool != "query_orders" {
		t.Fatalf("expected one record with tool query_orders, got %+v", recs)
	}
}

// TestAuditDenied_WritesDeniedRecord checks that a refused call is written as a
// denied record carrying the reason, the target and the caller.
func TestAuditDenied_WritesDeniedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	ctx := WithCaller(context.Background(), Caller{Principal: "ops", Session: "sess-1", Tool: "query_orders"})
	AuditDenied(ctx, "orders", "DELETE FROM orders", "lacks scope")

	recs := readAuditLines(t, path)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d: %+v", len(recs), recs)
	}
	rec := recs[0]
	if rec.Op != "denied" || rec.Error != "lacks scope" || rec.DurMS != 0 {
		t.Errorf("denied record op/error/duration wrong: %+v", rec)
	}
	if rec.Database != "orders" || rec.Statement != "DELETE FROM orders" {
		t.Errorf("denied record target wrong: %+v", rec)
	}
	if rec.Principal != "ops" || rec.Session != "sess-1" || rec.Tool != "query_orders" {
		t.Errorf("denied record caller wrong: %+v", rec)
	}
}

// TestAuditDenied_NoopWhenDisabled checks that a denial with no audit sink
// neither opens one nor fails.
func TestAuditDenied_NoopWhenDisabled(t *testing.T) {
	t.Setenv("DB_MCP_AUDIT_LOG", "")
	DisableAuditLog()
	audit.once = sync.Once{}
	t.Cleanup(func() { audit.once = sync.Once{} })

	AuditDenied(context.Background(), "orders", "DELETE FROM orders", "lacks scope")
	if audit.w != nil {
		t.Fatal("AuditDenied opened a sink while audit logging was disabled")
	}
}

// TestAudit_DefaultPrincipalFromEnv checks that DB_MCP_AUDIT_PRINCIPAL labels
// calls with no caller principal, and that a caller principal still wins.
func TestAudit_DefaultPrincipalFromEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("DB_MCP_AUDIT_PRINCIPAL", "ops-laptop")
	if err := EnableAuditLog(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	t.Cleanup(DisableAuditLog)

	audit.record(context.Background(), "query", "db", "SELECT 1", 0, nil, nil)
	audit.record(WithCaller(context.Background(), Caller{Principal: "user@example.com"}), "query", "db", "SELECT 1", 0, nil, nil)

	recs := readAuditLines(t, path)
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d: %+v", len(recs), recs)
	}
	if recs[0].Principal != "ops-laptop" {
		t.Errorf("empty caller principal = %q, want default ops-laptop", recs[0].Principal)
	}
	if recs[1].Principal != "user@example.com" {
		t.Errorf("caller principal = %q, want user@example.com (token principal wins)", recs[1].Principal)
	}
}

// TestRequireAuditLog_FailsWhenUnset checks that a required audit log with no
// DB_MCP_AUDIT_LOG path is an error.
func TestRequireAuditLog_FailsWhenUnset(t *testing.T) {
	t.Setenv("DB_MCP_AUDIT_LOG", "")
	if err := RequireAuditLog(); err == nil {
		t.Fatal("expected an error when DB_MCP_AUDIT_LOG is unset")
	}
}

// TestRequireAuditLog_FailsOnUnwritablePath checks that a required audit log
// whose file cannot be created is an error.
func TestRequireAuditLog_FailsOnUnwritablePath(t *testing.T) {
	t.Setenv("DB_MCP_AUDIT_LOG", filepath.Join(t.TempDir(), "missing-dir", "audit.jsonl"))
	if err := RequireAuditLog(); err == nil {
		t.Fatal("expected an error when the audit file cannot be opened")
	}
}

// TestRequireAuditLog_OKWithWritablePath checks that a writable path opens the
// sink at startup instead of on the first record.
func TestRequireAuditLog_OKWithWritablePath(t *testing.T) {
	t.Setenv("DB_MCP_AUDIT_LOG", filepath.Join(t.TempDir(), "audit.jsonl"))
	t.Cleanup(func() {
		DisableAuditLog()
		audit.once = sync.Once{}
	})
	if err := RequireAuditLog(); err != nil {
		t.Fatalf("RequireAuditLog failed for a writable path: %v", err)
	}
	if audit.w == nil {
		t.Fatal("RequireAuditLog returned nil without opening the sink")
	}
}
