package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Pre-mutation snapshots: before a DELETE or UPDATE runs, the affected rows
// are captured so the mutation can be reversed. Snapshots are kept in a
// bounded per-database ring; rollback rebuilds reverse SQL from the stored
// rows (INSERT for deletes, PK-targeted UPDATE restoring old values).

// snapshotCapacityPerDB bounds retained snapshots per database.
const snapshotCapacityPerDB = 25

// MutationSnapshot is one captured pre-mutation row set.
type MutationSnapshot struct {
	ID         string           `json:"id"`
	DatabaseID string           `json:"database_id"`
	Kind       string           `json:"kind"` // delete | update
	Table      string           `json:"table"`
	Where      string           `json:"where"`
	Columns    []string         `json:"columns"`
	Rows       []map[string]any `json:"-"`
	Timestamp  time.Time        `json:"timestamp"`
}

var (
	mutTableRe = regexp.MustCompile(`(?i)^(?:DELETE\s+FROM|UPDATE)\s+([A-Za-z_][A-Za-z0-9_$.]*)`)
	mutKindRe  = regexp.MustCompile(`(?i)^(DELETE|UPDATE)\b`)
)

// findTopLevelWhere returns the substring of stmt starting at the first
// depth-0 WHERE keyword (original casing), or "" when none exists.
func findTopLevelWhere(stmt string) string {
	upper := strings.ToUpper(stmt)
	depth := 0
	for i := 0; i < len(upper); i++ {
		switch upper[i] {
		case '(':
			depth++
		case ')':
			depth--
		default:
			if depth == 0 && strings.HasPrefix(upper[i:], "WHERE") {
				before := byte(' ')
				if i > 0 {
					before = upper[i-1]
				}
				after := byte(' ')
				if i+5 < len(upper) {
					after = upper[i+5]
				}
				isSep := func(c byte) bool {
					return c == ' ' || c == '\t' || c == '\n' || c == '\r'
				}
				if isSep(before) && isSep(after) {
					return stmt[i:]
				}
			}
		}
	}
	return ""
}

type snapshotStore struct {
	mu    sync.Mutex
	next  int
	perDB map[string][]MutationSnapshot
}

func newSnapshotStore() *snapshotStore {
	return &snapshotStore{perDB: map[string][]MutationSnapshot{}}
}

func (s *snapshotStore) add(dbID string, snap MutationSnapshot) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	snap.ID = fmt.Sprintf("snap_%d", s.next)
	log := s.perDB[dbID]
	log = append(log, snap)
	if len(log) > snapshotCapacityPerDB {
		log = log[len(log)-snapshotCapacityPerDB:]
	}
	s.perDB[dbID] = log
	return snap.ID
}

func (s *snapshotStore) get(dbID, id string) (MutationSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sn := range s.perDB[dbID] {
		if sn.ID == id {
			return sn, true
		}
	}
	return MutationSnapshot{}, false
}

func (s *snapshotStore) list(dbID string) []MutationSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MutationSnapshot, len(s.perDB[dbID]))
	copy(out, s.perDB[dbID])
	return out
}

// captureMutationSnapshot snapshots the rows a DELETE/UPDATE is about to
// affect. Best-effort by design: introspection failures never block execution.
func (uc *DatabaseUseCase) captureMutationSnapshot(ctx context.Context, db domain.Database, dbID, statement string) (string, error) {
	stripped := stripSQLLiterals(statement)
	kindMatch := mutKindRe.FindStringSubmatch(strings.TrimSpace(stripped))
	if kindMatch == nil {
		return "", fmt.Errorf("not a mutating statement")
	}
	tableMatch := mutTableRe.FindStringSubmatch(strings.TrimSpace(stripped))
	if tableMatch == nil {
		return "", fmt.Errorf("could not determine target table")
	}
	kind := strings.ToLower(kindMatch[1])
	table := stripSchema(tableMatch[1])
	where := findTopLevelWhere(statement)

	query := "SELECT * FROM " + table
	if where != "" {
		query += " " + where
	}

	rows, err := db.Query(ctx, query)
	if err != nil {
		return "", fmt.Errorf("snapshot query failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing snapshot rows: %v", cerr)
		}
	}()

	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("snapshot columns failed: %w", err)
	}

	var captured []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", fmt.Errorf("snapshot scan failed: %w", err)
		}
		row := map[string]any{}
		for i, c := range columns {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[c] = v
		}
		captured = append(captured, row)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("snapshot iteration failed: %w", err)
	}

	snap := MutationSnapshot{
		DatabaseID: dbID,
		Kind:       kind,
		Table:      table,
		Where:      where,
		Columns:    columns,
		Rows:       captured,
		Timestamp:  time.Now().UTC(),
	}
	return uc.snapshots.add(dbID, snap), nil
}

// quoteIdent wraps an identifier in double quotes (works on SQLite/Postgres;
// MySQL accepts them in ANSI mode and backticks otherwise — acceptable for a
// best-effort safety net).
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// RollbackSnapshot reverses the mutation recorded in the snapshot:
//   - delete → re-inserts every captured row
//   - update → restores each captured row's old column values, targeted by
//     its id column value (tables without an id column cannot reverse UPDATEs)
func (uc *DatabaseUseCase) RollbackSnapshot(ctx context.Context, dbID, snapshotID string) (string, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	snap, ok := uc.snapshots.get(dbID, snapshotID)
	if !ok {
		return "", fmt.Errorf("unknown snapshot %q for database %q", snapshotID, dbID)
	}
	if len(snap.Rows) == 0 {
		return fmt.Sprintf("Snapshot %s captured no rows — nothing to restore.", snapshotID), nil
	}

	tbl := quoteIdent(snap.Table)
	var restored int
	for _, row := range snap.Rows {
		var q string
		var args []any
		switch snap.Kind {
		case "delete":
			cols := make([]string, 0, len(snap.Columns))
			ph := make([]string, 0, len(snap.Columns))
			for _, c := range snap.Columns {
				cols = append(cols, quoteIdent(c))
				ph = append(ph, "?")
				args = append(args, row[c])
			}
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tbl, strings.Join(cols, ", "), strings.Join(ph, ", "))
		case "update":
			idVal, hasID := row["id"]
			if !hasID {
				return "", fmt.Errorf("cannot reverse UPDATE on %q without an id column", snap.Table)
			}
			sets := make([]string, 0, len(snap.Columns))
			args = nil
			for _, c := range snap.Columns {
				if c == "id" {
					continue
				}
				sets = append(sets, quoteIdent(c)+" = ?")
				args = append(args, row[c])
			}
			if len(sets) == 0 {
				continue
			}
			q = fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", tbl, strings.Join(sets, ", "), quoteIdent("id"))
			args = append(args, idVal)
		default:
			return "", fmt.Errorf("snapshot kind %q cannot be rolled back", snap.Kind)
		}
		if _, err := db.Exec(ctx, q, args...); err != nil {
			return "", fmt.Errorf("rollback failed after restoring %d row(s): %w", restored, err)
		}
		restored++
	}
	return fmt.Sprintf("Restored %d row(s) in %s from snapshot %s.", restored, snap.Table, snapshotID), nil
}

// ListSnapshots returns recent snapshots for a database, oldest first.
func (uc *DatabaseUseCase) ListSnapshots(dbID string) []MutationSnapshot {
	return uc.snapshots.list(dbID)
}
