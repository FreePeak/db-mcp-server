package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Principal string  `json:"principal,omitempty"`
	Session   string  `json:"session,omitempty"`
	Tool      string  `json:"tool,omitempty"`
	Rows      *int    `json:"rows,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// Caller identifies who issued a request for audit attribution. The delivery
// layer sets it on the context; the use case only reads it.
type Caller struct {
	Principal string // authenticated identity; empty when unauthenticated
	Session   string // MCP session ID; empty when the transport has none
	Tool      string // MCP tool name of the call; empty outside tools/call
}

type callerKey struct{}

// WithCaller returns a copy of ctx that carries caller for audit records.
func WithCaller(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, caller)
}

// CallerFromContext returns the caller set by WithCaller, or the zero Caller.
func CallerFromContext(ctx context.Context) Caller {
	caller, ok := ctx.Value(callerKey{}).(Caller)
	if !ok {
		return Caller{}
	}
	return caller
}

// knownRows turns a driver's rows-affected report into an audit count; nil
// when the driver could not report one, so the field is omitted.
func knownRows(n int64, err error) *int {
	if err != nil {
		return nil
	}
	v := int(n)
	return &v
}

type auditLogger struct {
	mu sync.Mutex
	w  *os.File

	// defaultPrincipal labels records whose caller has no principal, such as
	// stdio sessions. Read from DB_MCP_AUDIT_PRINCIPAL when the sink opens.
	defaultPrincipal string

	once sync.Once // lazy DB_MCP_AUDIT_LOG pickup on first record
}

// AuditDenied records a call refused before it reached the engine, such as a
// tool scope denial. Caller fields come from ctx; nothing is written while the
// audit log is disabled.
func AuditDenied(ctx context.Context, dbID, statement, reason string) {
	audit.record(ctx, "denied", dbID, statement, 0, nil, errors.New(reason))
}

// RequireAuditLog opens the DB_MCP_AUDIT_LOG sink now and fails when the path
// is unset or cannot be opened. Startup calls it when auditing is mandatory.
func RequireAuditLog() error {
	path := os.Getenv("DB_MCP_AUDIT_LOG")
	if path == "" {
		return errors.New("DB_MCP_AUDIT_REQUIRED is set but DB_MCP_AUDIT_LOG is empty")
	}
	if err := EnableAuditLog(path); err != nil {
		return fmt.Errorf("DB_MCP_AUDIT_LOG %s cannot be opened: %w", path, err)
	}
	return nil
}

var audit = &auditLogger{}

// initFromEnv opens the audit file exactly once when DB_MCP_AUDIT_LOG is
// set; startup ordering does not matter because records only flow after
// first use.
func (a *auditLogger) initFromEnv() {
	a.once.Do(func() {
		if path := os.Getenv("DB_MCP_AUDIT_LOG"); path != "" {
			if err := openAuditSink(path); err != nil {
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
	if err := openAuditSink(path); err != nil {
		return err
	}
	// Mark the lazy initializer done so an explicit enable wins over env.
	audit.once.Do(func() {})
	return nil
}

// openAuditSink swaps path in as the sink. It must not touch audit.once:
// initFromEnv calls it from inside that Once.
func openAuditSink(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	principal := os.Getenv("DB_MCP_AUDIT_PRINCIPAL")
	audit.mu.Lock()
	defer audit.mu.Unlock()
	if audit.w != nil {
		if err := audit.w.Close(); err != nil {
			logger.Warn("audit: closing previous sink failed: %v", err)
		}
	}
	audit.w = f
	audit.defaultPrincipal = principal
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
func (a *auditLogger) record(ctx context.Context, op, dbID, statement string, dur time.Duration, rows *int, execErr error) {
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
	caller := CallerFromContext(ctx)
	principal := caller.Principal
	if principal == "" {
		principal = a.defaultPrincipal
	}
	rec := auditRecord{
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		Op:        op,
		Database:  dbID,
		Statement: stmt,
		DurMS:     float64(dur.Microseconds()) / 1000.0,
		Principal: principal,
		Session:   caller.Session,
		Tool:      caller.Tool,
		Rows:      rows,
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
