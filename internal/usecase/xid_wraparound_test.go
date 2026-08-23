package usecase

import (
	"strings"
	"testing"
)

func TestXidWraparoundQuery(t *testing.T) {
	if xidWraparoundQuery("postgres") == "" {
		t.Fatal("postgres must expose XID ages")
	}
	if got := xidWraparoundQuery("postgresql"); got == "" {
		t.Fatal("alias 'postgresql' must be accepted")
	}
	if xidWraparoundQuery("mysql") != "" || xidWraparoundQuery("sqlite") != "" || xidWraparoundQuery("oracle") != "" {
		t.Fatal("only postgres exposes transaction-ID wraparound")
	}
}

// TestXidVerdict covers the ladder: under the freeze-max-age default is
// clean, at/over it is a warning, halfway to the 2^31 limit escalates
// to critical.
func TestXidVerdict(t *testing.T) {
	if got := xidVerdict("app", xidWarnAge-1); got != "" {
		t.Fatalf("healthy age must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := xidVerdict("app", xidWarnAge)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "app") || !strings.Contains(got, "autovacuum_freeze_max_age") {
		t.Fatalf("freeze-max-age threshold not escalated:\n%s", got)
	}
	crit := xidVerdict("app", xidCriticalAge)
	if !strings.Contains(crit, "CRITICAL") || !strings.Contains(crit, "wraparound") {
		t.Fatalf("critical age not escalated:\n%s", crit)
	}
}
