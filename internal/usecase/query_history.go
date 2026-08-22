package usecase

import (
	"sync"
	"time"
)

// Query history: a bounded per-database log of executed statements with
// duration and outcome, giving agents (and humans reviewing transcripts)
// introspection into what ran.

const queryHistoryCapacity = 100

// HistoryEntry is one executed statement.
type HistoryEntry struct {
	DatabaseID string    `json:"database_id"`
	Kind       string    `json:"kind"` // read | write
	Statement  string    `json:"statement"`
	DurationMs float64   `json:"duration_ms"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type queryHistoryStore struct {
	mu    sync.Mutex
	next  int64
	perDB map[string][]HistoryEntry
}

func newQueryHistoryStore() *queryHistoryStore {
	return &queryHistoryStore{perDB: map[string][]HistoryEntry{}}
}

func (s *queryHistoryStore) record(dbID, kind, statement string, durationMs float64, success bool, errText string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	entry := HistoryEntry{
		DatabaseID: dbID,
		Kind:       kind,
		Statement:  truncateQuery(statement),
		DurationMs: durationMs,
		Success:    success,
		Timestamp:  time.Now().UTC(),
	}
	if errText != "" {
		entry.Error = truncateQuery(errText)
	}
	log := s.perDB[dbID]
	log = append(log, entry)
	if len(log) > queryHistoryCapacity {
		log = log[len(log)-queryHistoryCapacity:]
	}
	s.perDB[dbID] = log
}

func (s *queryHistoryStore) list(dbID string) []HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]HistoryEntry, len(s.perDB[dbID]))
	copy(out, s.perDB[dbID])
	return out
}

// GetQueryHistory returns recent executions for the database, oldest first.
func (uc *DatabaseUseCase) GetQueryHistory(dbID string) []HistoryEntry {
	return uc.queryHist.list(dbID)
}

// recordQueryHistory wraps an execution with timing and outcome capture.
func (uc *DatabaseUseCase) recordQueryHistory(dbID, statement string, start time.Time, err error) {
	kind := "read"
	if IsWriteStatement(statement) {
		kind = "write"
	}
	success := err == nil
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	uc.queryHist.record(dbID, kind, statement,
		float64(time.Since(start).Microseconds())/1000.0, success, errText)
}
