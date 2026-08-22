package usecase

import (
	"context"

	"encoding/csv"
	"fmt"
	"github.com/FreePeak/db-mcp-server/internal/domain"
	"io"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// CSV import: parse CSV content and insert every row in one atomic
// transaction. Bounded (10k rows) so a pasted blob can't become an
// unbounded load.

const maxCSVRows = 10000

// ImportCSV parses header + rows from csvContent and inserts them into
// table atomically: any failure rolls back the whole batch.
func (uc *DatabaseUseCase) ImportCSV(ctx context.Context, dbID, table, csvContent string) (string, error) {
	if err := validateIdentifier(table); err != nil {
		return "", err
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	if db.IsReadOnly() {
		return "", fmt.Errorf("database %q is configured as read-only; imports are not allowed", dbID)
	}

	r := csv.NewReader(strings.NewReader(csvContent))
	r.FieldsPerRecord = 0 // first record sets the count; mismatches error
	header, err := r.Read()
	if err != nil {
		return "", fmt.Errorf("failed to read CSV header: %w", err)
	}
	for i, h := range header {
		if err := validateIdentifier(h); err != nil {
			return "", fmt.Errorf("invalid column name %q at position %d: %w", h, i, err)
		}
	}

	var stmts []string
	for {
		rec, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("malformed CSV at row %d: %w", len(stmts)+2, readErr)
		}
		if len(stmts) >= maxCSVRows {
			return "", fmt.Errorf("import exceeds %d row cap; split it into batches", maxCSVRows)
		}
		vals := make([]string, len(rec))
		for i, v := range rec {
			vals[i] = "'" + strings.ReplaceAll(v, "'", "''") + "'"
		}
		stmts = append(stmts, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			quoteIdent(table), quoteIdentList(header), strings.Join(vals, ", ")))
	}
	if len(stmts) == 0 {
		return "Import complete: 0 row(s) inserted (no data rows).", nil
	}

	tx, err := db.Begin(ctx, &domain.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to start transaction: %w", err)
	}
	for i, stmt := range stmts {
		if _, execErr := tx.Exec(ctx, stmt); execErr != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.Error("csv import rollback after row %d failed: %v", i+2, rbErr)
				return "", fmt.Errorf("row %d failed (%v) AND rollback failed: %w", i+2, execErr, rbErr)
			}
			return "", fmt.Errorf("row %d failed, import rolled back: %w", i+2, execErr)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit import: %w", err)
	}
	return fmt.Sprintf("Import complete: %d row(s) inserted into %s.", len(stmts), table), nil
}
