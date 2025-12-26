package dbtools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/logger"
)

// QueryMetrics stores performance metrics for a database query
type QueryMetrics struct {
	Query         string        // SQL query text
	Count         int           // Number of times the query was executed
	TotalDuration time.Duration // Total execution time
	MinDuration   time.Duration // Minimum execution time
	MaxDuration   time.Duration // Maximum execution time
	AvgDuration   time.Duration // Average execution time
	LastExecuted  time.Time     // When the query was last executed
}

// PerformanceAnalyzer tracks query performance and provides optimization suggestions
type PerformanceAnalyzer struct {
	slowThreshold time.Duration
	queryHistory  []QueryRecord
	maxHistory    int
}

// QueryRecord stores information about a query execution
type QueryRecord struct {
	Query      string        `json:"query"`
	Params     []interface{} `json:"params"`
	Duration   time.Duration `json:"duration"`
	StartTime  time.Time     `json:"startTime"`
	Error      string        `json:"error,omitempty"`
	Optimized  bool          `json:"optimized"`
	Suggestion string        `json:"suggestion,omitempty"`
}

// SQLIssueDetector detects potential issues in SQL queries
type SQLIssueDetector struct {
	patterns map[string]*regexp.Regexp
}

// singleton instance
var performanceAnalyzer *PerformanceAnalyzer

// GetPerformanceAnalyzer returns the singleton performance analyzer
func GetPerformanceAnalyzer() *PerformanceAnalyzer {
	if performanceAnalyzer == nil {
		performanceAnalyzer = NewPerformanceAnalyzer()
	}
	return performanceAnalyzer
}

// NewPerformanceAnalyzer creates a new performance analyzer
func NewPerformanceAnalyzer() *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		slowThreshold: 500 * time.Millisecond, // Default: 500ms
		queryHistory:  make([]QueryRecord, 0),
		maxHistory:    100, // Default: store last 100 queries
	}
}

// LogSlowQuery logs a warning if a query takes longer than the slow query threshold
func (pa *PerformanceAnalyzer) LogSlowQuery(query string, params []interface{}, duration time.Duration) {
	if duration >= pa.slowThreshold {
		paramStr := formatParams(params)
		logger.Warn("Slow query detected (%.2fms): %s [params: %s]",
			float64(duration.Microseconds())/1000.0,
			query,
			paramStr)
	}
}

// TrackQuery tracks the execution of a query and logs slow queries
func (pa *PerformanceAnalyzer) TrackQuery(_ context.Context, query string, params []interface{}, exec func() (interface{}, error)) (interface{}, error) {
	startTime := time.Now()
	result, err := exec()
	duration := time.Since(startTime)

	// Create query record
	record := QueryRecord{
		Query:     query,
		Params:    params,
		Duration:  duration,
		StartTime: startTime,
	}

	// Check if query is slow
	if duration >= pa.slowThreshold {
		pa.LogSlowQuery(query, params, duration)
		record.Suggestion = "Query execution time exceeds threshold"
	}

	// Record error if any
	if err != nil {
		record.Error = err.Error()
	}

	// Add to history (keeping max size)
	pa.queryHistory = append(pa.queryHistory, record)
	if len(pa.queryHistory) > pa.maxHistory {
		pa.queryHistory = pa.queryHistory[1:]
	}

	return result, err
}

// SQLIssueDetector methods

// NewSQLIssueDetector creates a new SQL issue detector
func NewSQLIssueDetector() *SQLIssueDetector {
	detector := &SQLIssueDetector{
		patterns: make(map[string]*regexp.Regexp),
	}

	// Add known issue patterns
	detector.AddPattern("cartesian-join", `SELECT.*FROM\s+(\w+)\s*,\s*(\w+)`)
	detector.AddPattern("select-star", `SELECT\s+\*\s+FROM`)
	detector.AddPattern("missing-where", `(DELETE\s+FROM|UPDATE)\s+\w+\s+(?:SET\s+(?:\w+\s*=\s*[^,]+)(?:\s*,\s*\w+\s*=\s*[^,]+)*\s*)*(;|\z)`)
	detector.AddPattern("or-in-where", `WHERE.*\s+OR\s+`)
	detector.AddPattern("in-with-many-items", `IN\s*\(\s*(?:'[^']*'\s*,\s*){10,}`)
	detector.AddPattern("not-in", `NOT\s+IN\s*\(`)
	detector.AddPattern("is-null", `IS\s+NULL`)
	detector.AddPattern("function-on-column", `WHERE\s+\w+\s*\(\s*\w+\s*\)`)
	detector.AddPattern("order-by-rand", `ORDER\s+BY\s+RAND\(\)`)
	detector.AddPattern("group-by-number", `GROUP\s+BY\s+\d+`)
	detector.AddPattern("having-without-group", `HAVING.*(?:(?:GROUP\s+BY.*$)|(?:$))`)

	return detector
}

// AddPattern adds a pattern for detecting SQL issues
func (d *SQLIssueDetector) AddPattern(name, pattern string) {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		logger.Error("Error compiling regex pattern '%s': %v", pattern, err)
		return
	}
	d.patterns[name] = re
}

// DetectIssues detects issues in a SQL query
func (d *SQLIssueDetector) DetectIssues(query string) map[string]string {
	issues := make(map[string]string)

	for name, pattern := range d.patterns {
		if pattern.MatchString(query) {
			issues[name] = d.getSuggestionForIssue(name)
		}
	}

	return issues
}

// getSuggestionForIssue returns a suggestion for a detected issue
func (d *SQLIssueDetector) getSuggestionForIssue(issue string) string {
	suggestions := map[string]string{
		"cartesian-join":       "Use explicit JOIN statements instead of comma-syntax joins to avoid unintended Cartesian products.",
		"select-star":          "Specify exact columns needed instead of SELECT * to reduce network traffic and improve query execution.",
		"missing-where":        "Add a WHERE clause to avoid affecting all rows in the table.",
		"or-in-where":          "Consider using IN instead of multiple OR conditions for better performance.",
		"in-with-many-items":   "Too many items in IN clause; consider a temporary table or a JOIN instead.",
		"not-in":               "NOT IN with subqueries can be slow. Consider using NOT EXISTS or LEFT JOIN/IS NULL pattern.",
		"is-null":              "IS NULL conditions prevent index usage. Consider redesigning to avoid NULL values if possible.",
		"function-on-column":   "Applying functions to columns in WHERE clauses prevents index usage. Restructure if possible.",
		"order-by-rand":        "ORDER BY RAND() causes full table scan and sort. Consider alternative randomization methods.",
		"group-by-number":      "Using column position numbers in GROUP BY can be error-prone. Use explicit column names.",
		"having-without-group": "HAVING without GROUP BY may indicate a logical error in query structure.",
	}

	if suggestion, ok := suggestions[issue]; ok {
		return suggestion
	}

	return "Potential issue detected; review query for optimization opportunities."
}

// Helper functions

// formatParams converts query parameters to a readable string format
func formatParams(params []interface{}) string {
	if len(params) == 0 {
		return "none"
	}

	paramStrings := make([]string, len(params))
	for i, param := range params {
		if param == nil {
			paramStrings[i] = "NULL"
		} else {
			paramStrings[i] = fmt.Sprintf("%v", param)
		}
	}

	return "[" + strings.Join(paramStrings, ", ") + "]"
}

// Placeholder for future implementation
// Currently not used but kept for reference
/*
func handlePerformanceAnalyzer(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// This function will be implemented in the future
	return nil, fmt.Errorf("not implemented")
}
*/

// StripComments removes SQL comments from a query string
func StripComments(input string) string {
	// Strip /* ... */ comments
	multiLineRe, err := regexp.Compile(`/\*[\s\S]*?\*/`)
	if err != nil {
		// If there's an error with the regex, just return the input
		logger.Error("Error compiling regex pattern '%s': %v", `\/\*[\s\S]*?\*\/`, err)
		return input
	}
	withoutMultiLine := multiLineRe.ReplaceAllString(input, "")

	// Strip -- comments
	singleLineRe, err := regexp.Compile(`--.*$`)
	if err != nil {
		return withoutMultiLine
	}
	return singleLineRe.ReplaceAllString(withoutMultiLine, "")
}

// GetAllMetrics returns all collected metrics
func (pa *PerformanceAnalyzer) GetAllMetrics() []*QueryMetrics {
	// Group query history by normalized query text
	queryMap := make(map[string]*QueryMetrics)

	for _, record := range pa.queryHistory {
		normalizedQuery := normalizeQuery(record.Query)

		metrics, exists := queryMap[normalizedQuery]
		if !exists {
			metrics = &QueryMetrics{
				Query:        record.Query,
				MinDuration:  record.Duration,
				MaxDuration:  record.Duration,
				LastExecuted: record.StartTime,
			}
			queryMap[normalizedQuery] = metrics
		}

		// Update metrics
		metrics.Count++
		metrics.TotalDuration += record.Duration

		if record.Duration < metrics.MinDuration {
			metrics.MinDuration = record.Duration
		}

		if record.Duration > metrics.MaxDuration {
			metrics.MaxDuration = record.Duration
		}

		if record.StartTime.After(metrics.LastExecuted) {
			metrics.LastExecuted = record.StartTime
		}
	}

	// Calculate averages and convert to slice
	metrics := make([]*QueryMetrics, 0, len(queryMap))
	for _, m := range queryMap {
		m.AvgDuration = time.Duration(int64(m.TotalDuration) / int64(m.Count))
		metrics = append(metrics, m)
	}

	return metrics
}

// Reset clears all collected metrics
func (pa *PerformanceAnalyzer) Reset() {
	pa.queryHistory = make([]QueryRecord, 0)
}

// GetSlowThreshold returns the current slow query threshold
func (pa *PerformanceAnalyzer) GetSlowThreshold() time.Duration {
	return pa.slowThreshold
}

// SetSlowThreshold sets the slow query threshold
func (pa *PerformanceAnalyzer) SetSlowThreshold(threshold time.Duration) {
	pa.slowThreshold = threshold
}

// AnalyzeQuery analyzes a SQL query and returns optimization suggestions
func AnalyzeQuery(query string) []string {
	// Create detector and get suggestions
	detector := NewSQLIssueDetector()
	issues := detector.DetectIssues(query)

	suggestions := make([]string, 0, len(issues))
	for _, suggestion := range issues {
		suggestions = append(suggestions, suggestion)
	}

	// Add default suggestions for query patterns
	if strings.Contains(strings.ToUpper(query), "SELECT *") {
		suggestions = append(suggestions, "Avoid using SELECT * - specify only the columns you need")
	}

	if !strings.Contains(strings.ToUpper(query), "WHERE") &&
		!strings.Contains(strings.ToUpper(query), "JOIN") {
		suggestions = append(suggestions, "Consider adding a WHERE clause to limit the result set")
	}

	if strings.Contains(strings.ToUpper(query), "JOIN") &&
		!strings.Contains(strings.ToUpper(query), "ON") {
		suggestions = append(suggestions, "Ensure all JOINs have proper conditions")
	}

	if strings.Contains(strings.ToUpper(query), "ORDER BY") {
		suggestions = append(suggestions, "Verify that ORDER BY columns are properly indexed")
	}

	if strings.Contains(query, "(SELECT") {
		suggestions = append(suggestions, "Consider replacing subqueries with JOINs where possible")
	}

	return suggestions
}

// normalizeQuery standardizes SQL queries for comparison by replacing literals
func normalizeQuery(query string) string {
	// Trim and normalize whitespace
	query = strings.TrimSpace(query)
	wsRegex := regexp.MustCompile(`\s+`)
	query = wsRegex.ReplaceAllString(query, " ")

	// Replace numeric literals
	numRegex := regexp.MustCompile(`\b\d+\b`)
	query = numRegex.ReplaceAllString(query, "?")

	// Replace string literals in single quotes
	strRegex := regexp.MustCompile(`'[^']*'`)
	query = strRegex.ReplaceAllString(query, "'?'")

	// Replace string literals in double quotes
	dblQuoteRegex := regexp.MustCompile(`"[^"]*"`)
	query = dblQuoteRegex.ReplaceAllString(query, "\"?\"")

	return query
}

// TODO: Implement more sophisticated performance metrics and query analysis
// TODO: Add support for query plan visualization
// TODO: Consider using time-series storage for long-term performance tracking
// TODO: Implement anomaly detection for query performance
// TODO: Add integration with external monitoring systems
// TODO: Implement periodic background performance analysis
