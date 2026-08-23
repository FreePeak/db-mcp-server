package usecase

import (
	"fmt"
	"strings"
)

// Health trend: the last N health observations per database, rendered
// as deltas, so pool exhaustion reads as chronic or one-off instead of
// requiring the agent to poll health_<db> repeatedly and remember.

const maxHealthSamples = 20

type healthSample struct {
	openConns int
	maxOpen   int
}

// RecordHealthSample appends one observation (called by HealthCheck);
// history is bounded to the most recent maxHealthSamples entries.
func (uc *DatabaseUseCase) RecordHealthSample(dbID string, openConns, maxOpen int) {
	uc.healthMu.Lock()
	defer uc.healthMu.Unlock()
	uc.healthSamples[dbID] = append(uc.healthSamples[dbID], healthSample{openConns, maxOpen})
	if len(uc.healthSamples[dbID]) > maxHealthSamples {
		uc.healthSamples[dbID] = uc.healthSamples[dbID][1:]
	}
}

// HealthTrend renders the recorded samples with open-connection deltas.
func (uc *DatabaseUseCase) HealthTrend(dbID string) (string, error) {
	uc.healthMu.Lock()
	samples := append([]healthSample(nil), uc.healthSamples[dbID]...)
	uc.healthMu.Unlock()

	if len(samples) == 0 {
		return fmt.Sprintf("No health samples recorded for %q yet; run health_%s to start the trend.", dbID, dbID), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Health trend for %s (%d sample(s), oldest first):\n", dbID, len(samples))
	for i, s := range samples {
		line := fmt.Sprintf("- open: %d/%d", s.openConns, s.maxOpen)
		if i > 0 {
			d := s.openConns - samples[i-1].openConns
			sign := "+"
			if d < 0 {
				sign = ""
			}
			line += fmt.Sprintf(" (%s%d)", sign, d)
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
