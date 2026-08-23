package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Schema snapshots & drift detection: capture the normalized shape of every
// table (name → ordered column name/type list) and diff later captures
// against a stored baseline. This is the migration-verification primitive —
// run a DDL change, then check what actually moved.

// SchemaColumn is one normalized column entry.
type SchemaColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// SchemaSnapshot is a normalized schema capture.
type SchemaSnapshot struct {
	ID         string                    `json:"id"`
	DatabaseID string                    `json:"database_id"`
	Tables     map[string][]SchemaColumn `json:"tables"`
	Timestamp  time.Time                 `json:"timestamp"`
}

// SchemaDriftReport lists every difference between current state and baseline.
type SchemaDriftReport struct {
	BaselineID string   `json:"baseline_id"`
	Drifted    bool     `json:"drifted"`
	Changes    []string `json:"changes"`
}

const schemaSnapshotCapacityPerDB = 20

type schemaSnapshotStore struct {
	mu    sync.Mutex
	next  int
	perDB map[string][]SchemaSnapshot
}

func newSchemaSnapshotStore() *schemaSnapshotStore {
	return &schemaSnapshotStore{perDB: map[string][]SchemaSnapshot{}}
}

func (s *schemaSnapshotStore) add(dbID string, snap SchemaSnapshot) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	snap.ID = fmt.Sprintf("schema_snap_%d", s.next)
	log := s.perDB[dbID]
	log = append(log, snap)
	if len(log) > schemaSnapshotCapacityPerDB {
		log = log[len(log)-schemaSnapshotCapacityPerDB:]
	}
	s.perDB[dbID] = log
	return snap.ID
}

func (s *schemaSnapshotStore) get(dbID, id string) (SchemaSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sn := range s.perDB[dbID] {
		if sn.ID == id {
			return sn, true
		}
	}
	return SchemaSnapshot{}, false
}

func (s *schemaSnapshotStore) list(dbID string) []SchemaSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SchemaSnapshot, len(s.perDB[dbID]))
	copy(out, s.perDB[dbID])
	return out
}

// CaptureSchemaSnapshot records the current table/column shape of dbID.
func (uc *DatabaseUseCase) CaptureSchemaSnapshot(ctx context.Context, dbID string) (*SchemaSnapshot, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no table listing available for %q", dbID)
	}

	snap := &SchemaSnapshot{
		DatabaseID: dbID,
		Tables:     map[string][]SchemaColumn{},
		Timestamp:  time.Now().UTC(),
	}
	for _, tr := range tablesRaw {
		nameRaw := ""
		for _, k := range []string{"name", "table_name", "tableName", "TABLE_NAME"} {
			if v, ok := tr[k].(string); ok && v != "" {
				nameRaw = v
				break
			}
		}
		if strings.TrimSpace(nameRaw) == "" || strings.HasPrefix(nameRaw, "sqlite_") {
			continue
		}
		desc, err := uc.DescribeTable(ctx, dbID, nameRaw)
		if err != nil {
			return nil, fmt.Errorf("describe %q failed: %w", nameRaw, err)
		}
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means empty capture
		cols := make([]SchemaColumn, 0, len(colsRaw))
		for _, cr := range colsRaw {
			name := ""
			for _, k := range []string{"name", "column_name", "COLUMN_NAME"} {
				if v, ok := cr[k].(string); ok && v != "" {
					name = v
					break
				}
			}
			typ := ""
			for _, k := range []string{"type", "data_type", "Type", "DATA_TYPE"} {
				if v, ok := cr[k].(string); ok && v != "" {
					typ = v
					break
				}
			}
			if name == "" {
				continue
			}
			cols = append(cols, SchemaColumn{Name: strings.ToLower(name), Type: strings.ToLower(typ)})
		}
		snap.Tables[strings.ToLower(nameRaw)] = cols
	}

	snap.ID = uc.schemaSnaps.add(dbID, *snap)
	return snap, nil
}

// CheckSchemaDrift compares current state against a stored baseline and
// reports added/removed tables, added/removed columns, and type changes.
func (uc *DatabaseUseCase) CheckSchemaDrift(ctx context.Context, dbID, baselineID string) (*SchemaDriftReport, error) {
	baseline, ok := uc.schemaSnaps.get(dbID, baselineID)
	if !ok {
		return nil, fmt.Errorf("unknown schema snapshot %q for database %q", baselineID, dbID)
	}
	current, err := uc.CaptureSchemaSnapshot(ctx, dbID)
	if err != nil {
		return nil, err
	}

	report := &SchemaDriftReport{BaselineID: baselineID, Changes: []string{}}

	// Removed / changed tables.
	for tName, baseCols := range baseline.Tables {
		curCols, exists := current.Tables[tName]
		if !exists {
			report.Changes = append(report.Changes, fmt.Sprintf("table removed: %s", tName))
			report.Drifted = true
			continue
		}
		baseIdx := colIndex(baseCols)
		curIdx := colIndex(curCols)
		for name, bCol := range baseIdx {
			cCol, still := curIdx[name]
			if !still {
				report.Changes = append(report.Changes, fmt.Sprintf("%s.%s removed", tName, name))
				report.Drifted = true
				continue
			}
			if !strings.EqualFold(bCol.Type, cCol.Type) {
				report.Changes = append(report.Changes,
					fmt.Sprintf("%s.%s type changed: %s -> %s", tName, name, bCol.Type, cCol.Type))
				report.Drifted = true
			}
		}
		for name := range curIdx {
			if _, was := baseIdx[name]; !was {
				report.Changes = append(report.Changes, fmt.Sprintf("%s.%s added (%s)", tName, name, curIdx[name].Type))
				report.Drifted = true
			}
		}
	}
	// Added tables.
	for tName := range current.Tables {
		if _, was := baseline.Tables[tName]; !was {
			report.Changes = append(report.Changes, fmt.Sprintf("table added: %s", tName))
			report.Drifted = true
		}
	}

	sort.Strings(report.Changes)
	return report, nil
}

func colIndex(cols []SchemaColumn) map[string]SchemaColumn {
	idx := make(map[string]SchemaColumn, len(cols))
	for _, c := range cols {
		idx[c.Name] = c
	}
	return idx
}

// ListSchemaSnapshots returns recent schema baselines for a database.
func (uc *DatabaseUseCase) ListSchemaSnapshots(dbID string) []SchemaSnapshot {
	return uc.schemaSnaps.list(dbID)
}
