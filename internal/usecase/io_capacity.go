package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// innodb_io_capacity audit: the default 200 IOPS was calibrated for a
// single spinning disk. Left untouched on SSD/NVMe-backed servers the
// flusher stays lazy until dirty pages pile up, then write stalls
// arrive in bursts — and checkpoints pace to the same wrong number.
// Both settings are SET GLOBAL-able live; no restart required.

// ioCapacityQuery returns the probe reading both settings in one
// round trip, or "" when unsupported.
func ioCapacityQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.innodb_io_capacity AS iocap,
       @@GLOBAL.innodb_io_capacity_max AS iocap_max`
	default:
		return ""
	}
}

// ioCapacityVerdict classifies flush pacing against modern storage;
// comfortable ceilings render "" so reports stay actionable.
func ioCapacityVerdict(ioCap, ioCapMax int64) string {
	if ioCap <= 0 {
		return "innodb_io_capacity is 0 or unreadable — check server configuration."
	}
	if ioCap <= 400 {
		return fmt.Sprintf("WARNING: innodb_io_capacity=%d is the single-spinning-disk default — on SSD/NVMe storage InnoDB flushes lazily until dirty pages pile up and writes stall in bursts (checkpoints pace to this number too). Fix: SET GLOBAL innodb_io_capacity=2000, innodb_io_capacity_max=4000 (or higher for fast NVMe) — both apply live, no restart.",
			ioCap)
	}
	return ""
}

// AuditIOCapacity renders whether InnoDB's flush pacing plausibly
// matches modern storage; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditIOCapacity(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := ioCapacityQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("io-capacity introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("io-capacity query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing io-capacity rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read io capacity: %w", rerr)
		}
		return "", fmt.Errorf("io-capacity query returned no rows")
	}

	var rawCap, rawMax string
	if scanErr := rows.Scan(&rawCap, &rawMax); scanErr != nil {
		return "", fmt.Errorf("failed to scan io-capacity settings: %w", scanErr)
	}
	parse := func(s string) int64 {
		n, perr := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if perr != nil {
			logger.Error("unparseable io-capacity setting %q: %v", s, perr)
			return 0
		}
		return n
	}
	ioCap, ioCapMax := parse(rawCap), parse(rawMax)
	if verdict := ioCapacityVerdict(ioCap, ioCapMax); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Flush pacing healthy: innodb_io_capacity=%d, innodb_io_capacity_max=%d.",
		ioCap, ioCapMax), nil
}
