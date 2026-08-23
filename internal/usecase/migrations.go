package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Migration runner: apply versioned .sql files from a directory in name
// order, tracking applied names in _mcp_migrations so re-runs are noops.
// Each migration is its own transaction (multi-statement files included,
// Flyway-style): a failure stops the run, keeps earlier migrations
// committed and recorded, and never records the failing file.

const migrationsTable = "_mcp_migrations"

// RunMigrations applies every pending .sql file in dir. Files sort
// lexicographically — prefix with 001_, 002_, … to order them.
func (uc *DatabaseUseCase) RunMigrations(ctx context.Context, dbID, dir string) (string, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	if db.IsReadOnly() {
		return "", fmt.Errorf("database %q is configured as read-only; migrations are not allowed", dbID)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read migration directory: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// Tracking table first, outside any migration's transaction.
	if _, err := db.Exec(ctx, "CREATE TABLE IF NOT EXISTS "+migrationsTable+
		" (name VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)"); err != nil {
		return "", fmt.Errorf("failed to ensure %s: %w", migrationsTable, err)
	}
	rows, err := db.Query(ctx, "SELECT name FROM "+migrationsTable)
	if err != nil {
		return "", fmt.Errorf("failed to read applied migrations: %w", err)
	}
	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr == nil {
			applied[name] = true
		}
	}
	if cerr := rows.Close(); cerr != nil {
		logger.Error("error closing applied-migration rows: %v", cerr)
	}

	var ran []string
	for _, f := range files {
		if applied[f] {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(dir, f))
		if readErr != nil {
			return "", fmt.Errorf("failed to read %s: %w", f, readErr)
		}
		stmts := splitScript(string(body))
		tx, txErr := db.Begin(ctx, &domain.TxOptions{})
		if txErr != nil {
			return "", fmt.Errorf("failed to start transaction for %s: %w", f, txErr)
		}
		failed := false
		for i, stmt := range stmts {
			if _, execErr := tx.Exec(ctx, stmt); execErr != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					logger.Error("migration %s rollback failed: %v", f, rbErr)
				}
				if len(ran) > 0 {
					return "", fmt.Errorf("%s statement %d failed after applying [%s], run aborted: %w",
						f, i+1, strings.Join(ran, ", "), execErr)
				}
				return "", fmt.Errorf("%s statement %d failed, run aborted: %w", f, i+1, execErr)
			}
		}
		if !failed {
			if _, execErr := tx.Exec(ctx,
				"INSERT INTO "+migrationsTable+" (name) VALUES (?)", f); execErr != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					logger.Error("migration %s rollback failed: %v", f, rbErr)
				}
				return "", fmt.Errorf("failed to record %s: %w", f, execErr)
			}
			if err := tx.Commit(); err != nil {
				return "", fmt.Errorf("failed to commit %s: %w", f, err)
			}
			ran = append(ran, f)
		}
	}

	if len(ran) == 0 {
		return fmt.Sprintf("No pending migrations (%d already applied).", len(files)), nil
	}
	return fmt.Sprintf("%d migration(s) applied:\n- %s", len(ran), strings.Join(ran, "\n- ")), nil
}
