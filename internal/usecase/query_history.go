package usecase

import (
	"encoding/json"
	"fmt"
	"os"
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
	perDB map[string][]HistoryEntry
	file  *os.File // optional append-only JSONL sink
}

func newQueryHistoryStore() *queryHistoryStore {
	return &queryHistoryStore{perDB: map[string][]HistoryEntry{}}
}

// enableFile opens path in append mode for a durable trail.
func (s *queryHistoryStore) enableFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		if cerr := s.file.Close(); cerr != nil {
			return fmt.Errorf("close previous query history sink: %w", cerr)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open query history sink: %w", err)
	}
	s.file = f
	return nil
}

// closeFile flushes and releases the sink; safe when never enabled.
func (s *queryHistoryStore) closeFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

func (s *queryHistoryStore) record(dbID, kind, statement string, durationMs float64, success bool, errText string) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

	// Durable trail best-effort: sink failures must not break execution.
	if s.file != nil {
		if buf, err := json.Marshal(entry); err == nil {
			_, _ = s.file.Write(append(buf, '\n')) //nolint:errcheck // best-effort trail
		}
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

// EnableQueryHistoryFile persists every subsequent execution to path as
// JSON Lines (append mode).
func (uc *DatabaseUseCase) EnableQueryHistoryFile(path string) error {
	return uc.queryHist.enableFile(path)
}

// CloseQueryHistoryFile flushes and closes the durable sink, if configured.
func (uc *DatabaseUseCase) CloseQueryHistoryFile() error {
	return uc.queryHist.closeFile()
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
