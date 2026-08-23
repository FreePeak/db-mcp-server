package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Saved queries: named bookmarks per database so an agent can replay an
// exploratory SELECT without re-pasting it. In-memory registry, same
// lifetime as snapshots and query history.

const maxSavedQueriesPerDB = 100

type savedQuery struct {
	sql string
}

type savedQueryStore struct {
	mu      sync.Mutex
	queries map[string]map[string]savedQuery // dbID -> name -> query
}

func newSavedQueryStore() *savedQueryStore {
	return &savedQueryStore{queries: map[string]map[string]savedQuery{}}
}

// SaveQuery stores (or overwrites) a named query for one database.
func (uc *DatabaseUseCase) SaveQuery(dbID, name, query string) error {
	name = strings.TrimSpace(name)
	query = strings.TrimSpace(query)
	if name == "" || len(name) > 128 {
		return fmt.Errorf("query name must be 1-128 characters")
	}
	if query == "" {
		return fmt.Errorf("query SQL must not be empty")
	}
	uc.savedQueries.mu.Lock()
	defer uc.savedQueries.mu.Unlock()
	m := uc.savedQueries.queries[dbID]
	if m == nil {
		m = map[string]savedQuery{}
		uc.savedQueries.queries[dbID] = m
	}
	if _, exists := m[name]; !exists && len(m) >= maxSavedQueriesPerDB {
		return fmt.Errorf("saved-query limit (%d) reached for %q; remove one first", maxSavedQueriesPerDB, dbID)
	}
	m[name] = savedQuery{sql: query}
	return nil
}

// ListSavedQueries renders the database's saved names with SQL previews.
func (uc *DatabaseUseCase) ListSavedQueries(dbID string) (string, error) {
	uc.savedQueries.mu.Lock()
	m := uc.savedQueries.queries[dbID]
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	previews := make(map[string]string, len(names))
	for _, n := range names {
		previews[n] = m[n].sql
	}
	uc.savedQueries.mu.Unlock()

	if len(names) == 0 {
		return fmt.Sprintf("No saved queries for %q.", dbID), nil
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "%d saved query(ies) for %s:\n", len(names), dbID)
	for _, n := range names {
		sqlText := previews[n]
		if len(sqlText) > 120 {
			sqlText = sqlText[:117] + "..."
		}
		fmt.Fprintf(&b, "- %s: %s\n", n, sqlText)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// RunSavedQuery executes a saved query on its owning database through
// the normal read path (auto-limit and masking apply).
func (uc *DatabaseUseCase) RunSavedQuery(ctx context.Context, dbID, name string) (string, error) {
	uc.savedQueries.mu.Lock()
	sq, ok := uc.savedQueries.queries[dbID][name]
	uc.savedQueries.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("no saved query %q for database %q", name, dbID)
	}
	return uc.ExecuteQuery(ctx, dbID, sq.sql, nil)
}
