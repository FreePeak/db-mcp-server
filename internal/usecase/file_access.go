package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// MySQL file-access surface audit: two settings decide whether the
// server can be turned into a file reader/writer. local_infile=ON lets
// a client's LOAD DATA LOCAL INFILE pull any server-process-readable
// file into a table; an empty secure_file_priv removes every path
// restriction on server-side LOAD DATA INFILE / SELECT ... INTO OUTFILE.
// NULL secure_file_priv disables import/export entirely — that is the
// safest posture and renders clean.

// fileAccessQuery returns the probe for both settings in one round
// trip, or "" when unsupported.
func fileAccessQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return "SELECT @@GLOBAL.local_infile, @@GLOBAL.secure_file_priv"
	default:
		return ""
	}
}

// parseInfileValue normalizes whatever shape the driver hands back for
// local_infile (integer, string, bytes, bool).
func parseInfileValue(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case []byte:
		s := strings.ToUpper(strings.TrimSpace(string(x)))
		return s == "1" || s == "ON" || s == "TRUE"
	default:
		s := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", v)))
		return s == "1" || s == "ON" || s == "TRUE"
	}
}

// fileAccessVerdict renders one warning line per risky setting, or ""
// when the surface is locked down.
func fileAccessVerdict(infileOn, sfpSet bool, sfp string) string {
	var lines []string
	if infileOn {
		lines = append(lines,
			"WARNING: local_infile is ON — a compromised client can read arbitrary "+
				"server-readable files into tables via LOAD DATA LOCAL INFILE; "+
				"SET GLOBAL local_infile = OFF unless you require it.")
	}
	if sfpSet && sfp == "" {
		lines = append(lines,
			"WARNING: secure_file_priv is empty — LOAD DATA INFILE and SELECT ... INTO OUTFILE "+
				"can touch ANY path the server process can read/write; set it to a dedicated directory.")
	}
	return strings.Join(lines, "\n")
}

// AuditFileAccess renders the file read/write exposure of a MySQL
// server: local_infile state plus where secure_file_priv points.
func (uc *DatabaseUseCase) AuditFileAccess(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := fileAccessQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("file-access introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("file-access catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing file-access rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return "", fmt.Errorf("file-access query returned no rows")
	}
	var infileRaw interface{}
	var sfpRaw interface{}
	if scanErr := rows.Scan(&infileRaw, &sfpRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan file-access settings: %w", scanErr)
	}

	infileOn := parseInfileValue(infileRaw)
	sfpSet := sfpRaw != nil // NULL means import/export disabled entirely
	var sfp string
	if sfpSet {
		sfp = strings.TrimSpace(fmt.Sprintf("%v", sfpRaw))
	}

	warning := fileAccessVerdict(infileOn, sfpSet, sfp)
	if warning != "" {
		return warning, nil
	}
	if !sfpSet {
		return "File access locked down: local_infile OFF, secure_file_priv NULL (server-side import/export disabled).", nil
	}
	return fmt.Sprintf("File access locked down: local_infile OFF, imports/exports restricted to %q.", sfp), nil
}
