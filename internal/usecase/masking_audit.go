package usecase

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// MaskingAuditEvent records one query execution that triggered PII
// redactions, giving operators visibility into when personal data was
// withheld from agent context.
type MaskingAuditEvent struct {
	DatabaseID  string    `json:"database_id"`
	Query       string    `json:"query"`
	CellsMasked int       `json:"cells_masked"`
	Timestamp   time.Time `json:"timestamp"`
}

// maskingAuditCapacity bounds the per-database ring buffer; older events
// are evicted as new ones arrive.
const maskingAuditCapacity = 100

var whitespaceRe = regexp.MustCompile(`\s+`)

// maskingAudit is a concurrency-safe bounded log of redaction events with
// an optional durable JSONL sink.
type maskingAudit struct {
	mu     sync.Mutex
	events map[string][]MaskingAuditEvent // databaseID -> recent events
	file   *os.File                       // optional append-only JSONL sink
	path   string                         // sink path when enabled
}

func newMaskingAudit() *maskingAudit {
	return &maskingAudit{events: map[string][]MaskingAuditEvent{}}
}

// sinkPath reports the configured durable sink path (empty when in-memory only).
func (m *maskingAudit) sinkPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.path
}

// enableFile opens path in append mode so restarts continue existing trails.
func (m *maskingAudit) enableFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file != nil {
		if cerr := m.file.Close(); cerr != nil {
			return fmt.Errorf("close previous masking audit sink: %w", cerr)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open masking audit sink: %w", err)
	}
	m.file = f
	m.path = path
	return nil
}

// closeFile flushes and releases the sink; safe to call when never enabled.
func (m *maskingAudit) closeFile() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file == nil {
		return nil
	}
	err := m.file.Close()
	m.file = nil
	return err
}

func (m *maskingAudit) record(dbID, query string, cells int) {
	if cells <= 0 {
		return // only actual redactions are auditable events
	}
	ev := MaskingAuditEvent{
		DatabaseID:  dbID,
		Query:       truncateQuery(query),
		CellsMasked: cells,
		Timestamp:   time.Now().UTC(),
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	log := m.events[dbID]
	log = append(log, ev)
	if len(log) > maskingAuditCapacity {
		log = log[len(log)-maskingAuditCapacity:]
	}
	m.events[dbID] = log

	// Durable trail: one JSON object per line; sink failures must never
	// break query serving, so write errors are deliberately ignored.
	if m.file != nil {
		if buf, err := json.Marshal(ev); err == nil {
			_, _ = m.file.Write(append(buf, '\n')) //nolint:errcheck // best-effort durable trail
		}
	}
}

func (m *maskingAudit) snapshot(dbID string) []MaskingAuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MaskingAuditEvent, len(m.events[dbID]))
	copy(out, m.events[dbID])
	return out
}

// truncateQuery keeps audit rows compact: first line, capped length.
func truncateQuery(q string) string {
	if i := strings.IndexByte(q, '\n'); i >= 0 {
		q = q[:i]
	}
	q = strings.TrimSpace(whitespaceRe.ReplaceAllString(q, " "))
	if len(q) > 200 {
		q = q[:197] + "..."
	}
	return q
}
