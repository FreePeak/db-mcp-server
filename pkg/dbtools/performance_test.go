package dbtools

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPerformanceAnalyzer(t *testing.T) {
	// Get the global performance analyzer and reset it to ensure clean state
	analyzer := GetPerformanceAnalyzer()
	analyzer.Reset()

	// Ensure we restore previous state after test
	defer func() {
		analyzer.Reset()
	}()

	// Test tracking a query
	ctx := context.Background()
	result, err := analyzer.TrackQuery(ctx, "SELECT * FROM test_table", []interface{}{}, func() (interface{}, error) {
		// Simulate query execution with sleep
		time.Sleep(5 * time.Millisecond)
		return "test result", nil
	})

	// Check results
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "test result" {
		t.Errorf("Expected result to be 'test result', got %v", result)
	}

	// Add a small delay to ensure async metrics update completes
	time.Sleep(10 * time.Millisecond)

	// Check metrics were collected
	metrics := analyzer.GetAllMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be collected, got none")
	}

	// Find the test query in metrics
	var foundMetrics *QueryMetrics
	for _, m := range metrics {
		if normalizeQuery(m.Query) == normalizeQuery("SELECT * FROM test_table") {
			foundMetrics = m
			break
		}
	}

	if foundMetrics == nil {
		t.Error("Expected to find metrics for the test query, got none")
	} else {
		// Check metrics values
		if foundMetrics.Count != 1 {
			t.Errorf("Expected count to be 1, got %d", foundMetrics.Count)
		}

		if foundMetrics.AvgDuration < time.Millisecond {
			t.Errorf("Expected average duration to be at least 1ms, got %v", foundMetrics.AvgDuration)
		}
	}
}

func TestPerformanceAnalyzerConcurrentTrackingAndReads(t *testing.T) {
	analyzer := NewPerformanceAnalyzer()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = analyzer.TrackQuery(context.Background(), "SELECT ?", []interface{}{1}, func() (interface{}, error) {
				return nil, nil
			})
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				_ = analyzer.GetAllMetrics()
				_ = analyzer.SlowQueries()
			}
		}()
	}
	wg.Wait()

	metrics := analyzer.GetAllMetrics()
	if len(metrics) != 1 || metrics[0].Count != 100 {
		t.Fatalf("expected 100 normalized query metrics, got %#v", metrics)
	}
}

func TestQueryAnalyzer(t *testing.T) {
	testCases := []struct {
		name        string
		query       string
		expectation string
	}{
		{
			name:        "SELECT * detection",
			query:       "SELECT * FROM users",
			expectation: "Avoid using SELECT * - specify only the columns you need",
		},
		{
			name:        "Missing WHERE detection",
			query:       "SELECT id, name FROM users",
			expectation: "Consider adding a WHERE clause to limit the result set",
		},
		{
			name:        "JOIN without ON detection",
			query:       "SELECT u.id, p.name FROM users u JOIN profiles p",
			expectation: "Ensure all JOINs have proper conditions",
		},
		{
			name:        "ORDER BY detection",
			query:       "SELECT id, name FROM users WHERE id > 100 ORDER BY name",
			expectation: "Verify that ORDER BY columns are properly indexed",
		},
		{
			name:        "Subquery detection",
			query:       "SELECT id, name FROM users WHERE id IN (SELECT user_id FROM orders)",
			expectation: "Consider replacing subqueries with JOINs where possible",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suggestions := AnalyzeQuery(tc.query)

			// Check if the expected suggestion is in the list
			found := false
			for _, s := range suggestions {
				if s == tc.expectation {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected to find suggestion '%s' for query '%s', but got suggestions: %v",
					tc.expectation, tc.query, suggestions)
			}
		})
	}
}

func TestNormalizeQuery(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Number replacement",
			input:    "SELECT * FROM users WHERE id = 123",
			expected: "SELECT * FROM users WHERE id = ?",
		},
		{
			name:     "String replacement",
			input:    "SELECT * FROM users WHERE name = 'John Doe'",
			expected: "SELECT * FROM users WHERE name = '?'",
		},
		{
			name:     "Double quotes replacement",
			input:    "SELECT * FROM \"users\" WHERE \"name\" = \"John Doe\"",
			expected: "SELECT * FROM \"?\" WHERE \"?\" = \"?\"",
		},
		{
			name:     "Multiple whitespace",
			input:    "SELECT   *   FROM   users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "Complex query",
			input:    "SELECT u.id, p.name FROM users u JOIN profiles p ON u.id = 123 AND p.name = 'test'",
			expected: "SELECT u.id, p.name FROM users u JOIN profiles p ON u.id = ? AND p.name = '?'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeQuery(tc.input)
			if result != tc.expected {
				t.Errorf("Expected normalized query '%s', got '%s'", tc.expected, result)
			}
		})
	}
}
