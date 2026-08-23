package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Size baselines: capture per-table row counts once, compare later to
// answer "is this table growing?" without the agent snapshotting sizes
// by hand and diffing mentally. One baseline per database; re-capture
// overwrites.

type sizeBaseline struct {
	counts     map[string]int64
	capturedAt time.Time
}

type sizeBaselineStore struct {
	mu        sync.Mutex
	baselines map[string]sizeBaseline
}

func newSizeBaselineStore() *sizeBaselineStore {
	return &sizeBaselineStore{baselines: map[string]sizeBaseline{}}
}

func (uc *DatabaseUseCase) countAllTables(ctx context.Context, dbID string) (map[string]int64, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no table listing available for %q", dbID)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}
	counts := map[string]int64{}
	for _, tr := range tablesRaw {
		name := metaString(tr, "table_name")
		if name == "" {
			name = metaString(tr, "name")
		}
		if name == "" || !isPlainIdentifier(name) || strings.HasPrefix(name, "sqlite_") {
			continue
		}
		n, cerr := countTableRows(ctx, db, name)
		if cerr != nil {
			continue // unreadable table: absent from both sides
		}
		counts[name] = n
	}
	return counts, nil
}

// CaptureSizeBaseline records current row counts as the database's
// comparison point.
func (uc *DatabaseUseCase) CaptureSizeBaseline(ctx context.Context, dbID string) (string, error) {
	counts, err := uc.countAllTables(ctx, dbID)
	if err != nil {
		return "", err
	}
	uc.sizeBaselines.mu.Lock()
	uc.sizeBaselines.baselines[dbID] = sizeBaseline{counts: counts, capturedAt: time.Now()}
	uc.sizeBaselines.mu.Unlock()
	return fmt.Sprintf("Baseline captured: %d table(s) on %s.", len(counts), dbID), nil
}

// CompareSizeBaseline renders per-table deltas against the captured
// baseline: growth (+N), shrinkage (−N), and tables created since.
func (uc *DatabaseUseCase) CompareSizeBaseline(ctx context.Context, dbID string) (string, error) {
	uc.sizeBaselines.mu.Lock()
	base, ok := uc.sizeBaselines.baselines[dbID]
	uc.sizeBaselines.mu.Unlock()
	if !ok {
		return fmt.Sprintf("No baseline captured for %q yet; run capture first.", dbID), nil
	}
	now, err := uc.countAllTables(ctx, dbID)
	if err != nil {
		return "", err
	}

	names := make([]string, 0, len(now))
	for n := range now {
		names = append(names, n)
	}
	sort.Strings(names)

	elapsed := time.Since(base.capturedAt)
	var b strings.Builder
	changed := 0
	for _, n := range names {
		cur := now[n]
		before, existed := base.counts[n]
		if !existed {
			fmt.Fprintf(&b, "- %s: %d row(s) (new since baseline)\n", n, cur)
			changed++
			continue
		}
		d := cur - before
		if d == 0 {
			fmt.Fprintf(&b, "- %s: unchanged (%d)\n", n, cur)
			continue
		}
		sign := "+"
		if d < 0 {
			sign = ""
		}
		fmt.Fprintf(&b, "- %s: %+d (%d -> %d)%s\n", sign, d, before, cur, growthRate(d, elapsed))
		changed++
	}
	if changed == 0 {
		return fmt.Sprintf("No changes since baseline (%d table(s)) on %s.", len(names), dbID), nil
	}
	out := baselineHeader(dbID, len(base.counts), elapsed) + "\n" + b.String()
	return strings.TrimRight(out, "\n"), nil
}

// growthRate projects a positive multi-day delta as rows/day; sub-day
// windows and shrinkage stay unprojected rather than inventing noise.
func growthRate(delta int64, elapsed time.Duration) string {
	days := int(elapsed.Hours() / 24)
	if days < 1 || delta <= 0 {
		return ""
	}
	return fmt.Sprintf(" (+%d/day)", delta/int64(days))
}

// baselineHeader renders the compare report's opening line with the
// baseline age so stale baselines are obvious.
func baselineHeader(dbID string, tableCount int, elapsed time.Duration) string {
	return fmt.Sprintf("Size delta vs baseline for %s (captured %d day(s) ago, %d table(s)):", dbID, int(elapsed.Hours()/24), tableCount)
}
