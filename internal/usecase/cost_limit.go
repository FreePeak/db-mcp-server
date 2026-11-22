package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// costExplainSQL returns the EXPLAIN statement that yields the planner cost
// estimate for sql without executing it. Engines without a cost estimate
// return false.
func costExplainSQL(dbType, sql string) (string, bool) {
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		return "EXPLAIN (FORMAT JSON) " + sql, true
	case "mysql":
		return "EXPLAIN FORMAT=JSON " + sql, true
	}
	return "", false
}

// parsePlanCost extracts the total planner cost from an EXPLAIN JSON document.
// PostgreSQL reports one root Total Cost. MySQL reports a query_cost per query
// block, so the blocks of a UNION are summed. A document without a readable
// cost is an error, and the caller denies the query.
func parsePlanCost(dbType, planJSON string) (float64, error) {
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		var plan []struct {
			Plan struct {
				TotalCost *float64 `json:"Total Cost"`
			} `json:"Plan"`
		}
		if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
			return 0, fmt.Errorf("parse postgres plan: %w", err)
		}
		if len(plan) == 0 || plan[0].Plan.TotalCost == nil {
			return 0, errors.New("postgres plan has no Total Cost")
		}
		return *plan[0].Plan.TotalCost, nil
	case "mysql":
		var doc interface{}
		if err := json.Unmarshal([]byte(planJSON), &doc); err != nil {
			return 0, fmt.Errorf("parse mysql plan: %w", err)
		}
		var total float64
		var found bool
		if err := sumMySQLQueryCost(doc, &total, &found); err != nil {
			return 0, err
		}
		if !found {
			return 0, errors.New("mysql plan has no query_cost")
		}
		return total, nil
	}
	return 0, fmt.Errorf("no plan cost parser for engine %q", dbType)
}

// sumMySQLQueryCost walks a decoded EXPLAIN FORMAT=JSON tree and adds every
// cost_info.query_cost it finds. MySQL reports the cost as a string.
func sumMySQLQueryCost(node interface{}, total *float64, found *bool) error {
	switch v := node.(type) {
	case map[string]interface{}:
		// A block that reads no table carries no cost_info and costs nothing.
		if v["message"] == "No tables used" {
			*found = true
		}
		if info, ok := v["cost_info"].(map[string]interface{}); ok {
			if raw, ok := info["query_cost"]; ok {
				s, isString := raw.(string)
				if !isString {
					return fmt.Errorf("mysql query_cost is %T, want string", raw)
				}
				cost, err := strconv.ParseFloat(s, 64)
				if err != nil {
					return fmt.Errorf("parse mysql query_cost %q: %w", s, err)
				}
				*total += cost
				*found = true
			}
		}
		for _, child := range v {
			if err := sumMySQLQueryCost(child, total, found); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, child := range v {
			if err := sumMySQLQueryCost(child, total, found); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkQueryCost estimates sql with EXPLAIN and denies it when the estimate
// is above limit. The EXPLAIN goes straight to the engine rather than through
// ExecuteQuery, so it stays out of audit and performance tracking. Failing to
// obtain or read an estimate denies the statement.
func (uc *DatabaseUseCase) checkQueryCost(ctx context.Context, dbID string, db domain.Database, dbType, sql string, params []interface{}, limit float64) error {
	dbType = strings.ToLower(dbType)
	body := strings.TrimRight(strings.TrimSpace(sql), "; \t\r\n")
	// The cost gate estimates exactly one statement.
	if strings.Contains(stripSQLLiterals(body), ";") {
		return fmt.Errorf("max_query_cost: send exactly one statement per query on database %q", dbID)
	}
	explainSQL, ok := costExplainSQL(dbType, body)
	if !ok {
		return fmt.Errorf("max_query_cost: engine %q has no cost estimate on database %q", dbType, dbID)
	}
	planJSON, err := readPlanJSON(ctx, db, explainSQL, params)
	if err != nil {
		return fmt.Errorf("max_query_cost: query denied, cost estimate unavailable on database %q: %w", dbID, err)
	}
	cost, err := parsePlanCost(dbType, planJSON)
	if err != nil {
		return fmt.Errorf("max_query_cost: query denied, plan unreadable on database %q: %w", dbID, err)
	}
	if cost > limit {
		return fmt.Errorf("estimated query cost %.1f exceeds max_query_cost %v for database %q", cost, limit, dbID)
	}
	return nil
}

// readPlanJSON runs an EXPLAIN statement and returns the plan text from its
// first column of the first row.
func readPlanJSON(ctx context.Context, db domain.Database, explainSQL string, params []interface{}) (string, error) {
	rows, err := db.Query(ctx, explainSQL, params...)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", err
		}
		return "", errors.New("EXPLAIN returned no rows")
	}
	var raw interface{}
	if err := rows.Scan(&raw); err != nil {
		return "", err
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	switch v := raw.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	}
	return "", fmt.Errorf("EXPLAIN returned %T, want text", raw)
}

// databaseMaxQueryCost returns the database's max_query_cost, or 0 when the
// database does not expose one. Mirrors databaseMaskPII.
func databaseMaxQueryCost(d domain.Database) float64 {
	if m, ok := d.(interface{ MaxQueryCost() float64 }); ok {
		return m.MaxQueryCost()
	}
	return 0
}

// enforceQueryCost applies the cost gate to an agent-issued query. It refuses
// a raw EXPLAIN ANALYZE, which executes its statement, and estimates each
// SELECT or WITH before it reaches the engine. Other statements are not gated.
func (uc *DatabaseUseCase) enforceQueryCost(ctx context.Context, dbID string, db domain.Database, query string, params []interface{}) error {
	limit := databaseMaxQueryCost(db)
	if limit <= 0 {
		return nil
	}
	if isExplainAnalyze(query) {
		return fmt.Errorf("EXPLAIN ANALYZE executes the statement, so max_query_cost cannot estimate it on database %q; use the explain tool with analyze=true instead", dbID)
	}
	if !isCostGatedRead(query) {
		return nil
	}
	dbType, err := uc.GetDatabaseType(dbID)
	if err != nil {
		return fmt.Errorf("max_query_cost: database type unavailable for %q: %w", dbID, err)
	}
	return uc.checkQueryCost(ctx, dbID, db, dbType, query, params, limit)
}

// isCostGatedRead reports whether a statement is a SELECT or WITH, optionally
// wrapped in parentheses. The leading keyword comes from leadingKeyword, which
// stops at '(' and so needs the wrapping parentheses removed first.
func isCostGatedRead(query string) bool {
	upper := strings.ToUpper(strings.TrimSpace(stripSQLLiterals(query)))
	switch leadingKeyword(strings.TrimLeft(upper, "( \t\r\n")) {
	case "SELECT", "WITH":
		return true
	}
	return false
}

// isExplainAnalyze reports whether a statement is EXPLAIN with an ANALYZE
// option, in either the parenthesized or the bare form.
func isExplainAnalyze(query string) bool {
	tokens := sqlWordTokens(stripSQLLiterals(query))
	if len(tokens) == 0 || !strings.EqualFold(tokens[0], "EXPLAIN") {
		return false
	}
	for _, tok := range tokens[1:] {
		if strings.EqualFold(tok, "ANALYZE") || strings.EqualFold(tok, "ANALYSE") {
			return true
		}
	}
	return false
}
