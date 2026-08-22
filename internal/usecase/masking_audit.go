package usecase

import (
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

// maskingAudit is a concurrency-safe bounded log of redaction events.
type maskingAudit struct {
	mu     sync.Mutex
	events map[string][]MaskingAuditEvent // databaseID -> recent events
}

func newMaskingAudit() *maskingAudit {
	return &maskingAudit{events: map[string][]MaskingAuditEvent{}}
}

func (m *maskingAudit) record(dbID, query string, cells int) {
	if cells <= 0 {
		return // only actual redactions are auditable events
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	log := m.events[dbID]
	log = append(log, MaskingAuditEvent{
		DatabaseID:  dbID,
		Query:       truncateQuery(query),
		CellsMasked: cells,
		Timestamp:   time.Now().UTC(),
	})
	if len(log) > maskingAuditCapacity {
		log = log[len(log)-maskingAuditCapacity:]
	}
	m.events[dbID] = log
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
