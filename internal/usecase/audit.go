package usecase

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/logger"
)

// Audit logging gives operators a persistent JSONL trail of every statement
// executed through this server — the governance counterpart of Bytebase's
// audit log, sized for an MCP process: one append-only file, one record per
// execution, best-effort writes that never fail a query.
//
// Enable with DB_MCP_AUDIT_LOG=/path/audit.jsonl (env-only by design: an
// audit destination belongs to deployment config, not per-database JSON).

// auditStatementCap bounds a single record; statements beyond it are
// truncated with a marker so pathological payloads cannot balloon the file.
const auditStatementCap = 10_000

// auditRecord is one JSONL line in the audit log.
type auditRecord struct {
	TS        string  `json:"ts"`
	Op        string  `json:"op"`
	Database  string  `json:"database"`
	Statement string  `json:"statement"`
	DurMS     float64 `json:"duration_ms"`
	Error     string  `json:"error,omitempty"`
}

type auditLogger struct {
	mu sync.Mutex
	w  *os.File

	once sync.Once // lazy DB_MCP_AUDIT_LOG pickup on first record
}

var audit = &auditLogger{}

// initFromEnv opens the audit file exactly once when DB_MCP_AUDIT_LOG is
// set; startup ordering does not matter because records only flow after
// first use.
func (a *auditLogger) initFromEnv() {
	a.once.Do(func() {
		if path := os.Getenv("DB_MCP_AUDIT_LOG"); path != "" {
			if err := EnableAuditLog(path); err != nil {
				// The once is consumed either way: a failed env path never
				// retries mid-run; an explicit EnableAuditLog can still win
				// because it sets w directly.
				logger.Warn("audit log disabled: %v", err)
			}
		}
	})
}

// EnableAuditLog opens path as an append-only audit sink. Callers may use
// it directly (tests, future config plumbing); the server picks the env
// variable up automatically.
func EnableAuditLog(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	audit.mu.Lock()
	defer audit.mu.Unlock()
	if audit.w != nil {
		if err := audit.w.Close(); err != nil {
			logger.Warn("audit: closing previous sink failed: %v", err)
		}
	}
	audit.w = f
	// Mark the lazy initializer done so an explicit enable wins over env.
	audit.once.Do(func() {})
	return nil
}

// DisableAuditLog closes the sink; used between tests sharing the process.
func DisableAuditLog() {
	audit.mu.Lock()
	defer audit.mu.Unlock()
	if audit.w != nil {
		if err := audit.w.Close(); err != nil {
			logger.Warn("audit: closing sink failed: %v", err)
		}
		audit.w = nil
	}
}

// record writes one line; failures are logged and dropped — auditing must
// never turn into an availability problem for the queries it observes.
func (a *auditLogger) record(op, dbID, statement string, dur time.Duration, execErr error) {
	a.initFromEnv()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.w == nil {
		return
	}

	stmt := statement
	if len(stmt) > auditStatementCap {
		stmt = stmt[:auditStatementCap] + "...[truncated]"
	}
	rec := auditRecord{
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		Op:        op,
		Database:  dbID,
		Statement: stmt,
		DurMS:     float64(dur.Microseconds()) / 1000.0,
	}
	if execErr != nil {
		rec.Error = execErr.Error()
	}

	line, err := json.Marshal(rec)
	if err != nil {
		logger.Warn("audit: marshal failed: %v", err)
		return
	}
	if _, err := a.w.Write(append(line, '\n')); err != nil {
		logger.Warn("audit: write failed: %v", err)
	}
}
