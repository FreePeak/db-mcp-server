package db

import (
	"context"
	"database/sql/driver"
	"fmt"
	"io"
	"sort"
	"strings"

	go_ora "github.com/sijms/go-ora/v2"

	"github.com/FreePeak/db-mcp-server/pkg/logger"
)

// Oracle has no session-level read-only switch (ALTER SESSION SET READ ONLY
// is ORA-02248), so engine-level enforcement works through privileges: a
// session can only write if its credentials carry write privileges. This
// connector audits every new pooled session before first use and refuses
// read-only databases whose credentials could ever write — fail-closed,
// because privileges are static for the session's lifetime, "verified once"
// means "enforced throughout". It is the Oracle counterpart of PostgreSQL's
// default_transaction_read_only=on and MySQL's transaction_read_only=1 DSN
// params, which need no such audit because those engines enforce in-engine.
type readOnlySessionConnector struct {
	inner driver.Connector
}

// writeSystemPrivileges are system privileges that permit writes anywhere
// in the instance (the ANY variants) or within the user's own schema
// (CREATE TABLE makes a schema owner-equivalent writer).
var writeSystemPrivileges = []string{
	"CREATE ANY TABLE", "ALTER ANY TABLE", "DROP ANY TABLE",
	"INSERT ANY TABLE", "UPDATE ANY TABLE", "DELETE ANY TABLE",
	"CREATE TABLE", "CREATE ANY TRIGGER",
}

// newReadOnlySessionConnector builds a connector for the given go-ora DSN.
func newReadOnlySessionConnector(dsn string) driver.Connector {
	return &readOnlySessionConnector{inner: go_ora.NewConnector(dsn)}
}

// Connect opens an underlying Oracle connection and audits its privileges;
// any write capability aborts the connection rather than serving a
// read-only database that can silently be written.
func (c *readOnlySessionConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}

	sysPrivs, err := queryFirstColumn(ctx, conn,
		"SELECT privilege FROM session_privs WHERE privilege IN ("+quotedList(writeSystemPrivileges)+")")
	if err != nil {
		closeConn(conn)
		return nil, fmt.Errorf("cannot verify oracle read_only (fail closed): %w", err)
	}
	objPrivs, err := queryFirstColumn(ctx, conn,
		"SELECT privilege FROM user_tab_privs_recd WHERE privilege IN ('INSERT','UPDATE','DELETE')")
	if err != nil {
		closeConn(conn)
		return nil, fmt.Errorf("cannot verify oracle read_only (fail closed): %w", err)
	}

	privSet := map[string]bool{}
	for _, p := range sysPrivs {
		privSet[strings.ToUpper(p)] = true
	}
	for _, p := range objPrivs {
		privSet["object-level "+strings.ToUpper(p)] = true
	}
	var writable []string
	for _, p := range append(append([]string{}, writeSystemPrivileges...),
		"object-level INSERT", "object-level UPDATE", "object-level DELETE") {
		if privSet[p] {
			writable = append(writable, p)
		}
	}
	sort.Strings(writable)

	if len(writable) > 0 {
		closeConn(conn)
		return nil, fmt.Errorf(
			"oracle read_only cannot hold: credentials hold write privilege(s) %s — "+
				"grant only CREATE SESSION (plus SELECT grants) to make the engine refuse writes", writable)
	}
	return conn, nil
}

// Driver delegates to the underlying connector's driver.
func (c *readOnlySessionConnector) Driver() driver.Driver {
	return c.inner.Driver()
}

// closeConn discards a connection whose audit failed; Close errors on an
// abort path carry no information for the caller but are logged for ops.
func closeConn(conn driver.Conn) {
	if err := conn.Close(); err != nil {
		logger.Warn("oracle read-only audit: closing rejected session failed: %v", err)
	}
}

// quotedList renders compile-time constant identifiers as an IN-list.
func quotedList(items []string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = "'" + it + "'"
	}
	return strings.Join(parts, ",")
}

// queryFirstColumn runs a statement over a raw driver connection and
// returns the first column of every row as strings.
func queryFirstColumn(ctx context.Context, conn driver.Conn, stmt string) ([]string, error) {
	if q, ok := conn.(driver.QueryerContext); ok {
		rows, err := q.QueryContext(ctx, stmt, nil)
		if err != nil {
			return nil, err
		}
		return collectFirstColumn(rows)
	}
	st, err := conn.Prepare(stmt)
	if err != nil {
		return nil, err
	}
	qc, ok := st.(driver.StmtQueryContext)
	if !ok {
		_ = st.Close() //nolint:errcheck // statement never entered service
		return nil, fmt.Errorf("oracle driver statement does not support context queries")
	}
	rows, err := qc.QueryContext(ctx, nil)
	if closeErr := st.Close(); closeErr != nil {
		logger.Warn("oracle read-only audit: closing audit statement failed: %v", closeErr)
	}
	if err != nil {
		return nil, err
	}
	return collectFirstColumn(rows)
}

// collectFirstColumn drains a raw driver.Rows into a string slice.
func collectFirstColumn(rows driver.Rows) ([]string, error) {
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Warn("oracle read-only audit: closing audit rows failed: %v", err)
		}
	}()
	cols := rows.Columns()
	if len(cols) == 0 {
		return nil, fmt.Errorf("query returned no columns")
	}
	vals := make([]driver.Value, len(cols))
	out := []string{}
	for {
		switch err := rows.Next(vals); err {
		case nil:
			if s, ok := vals[0].(string); ok {
				out = append(out, s)
			}
		case io.EOF:
			return out, nil
		default:
			return nil, err
		}
	}
}
