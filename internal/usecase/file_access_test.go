package usecase

import (
	"strings"
	"testing"
)

// TestFileAccessQuery proves only MySQL-family engines expose the
// @@GLOBAL file-access surface; other engines report unsupported.
func TestFileAccessQuery(t *testing.T) {
	if q := fileAccessQuery("mysql"); !strings.Contains(q, "local_infile") || !strings.Contains(q, "secure_file_priv") {
		t.Fatalf("mysql probe missing settings:\n%s", q)
	}
	if q := fileAccessQuery("mariadb"); q == "" {
		t.Fatal("mariadb must be supported")
	}
	if fileAccessQuery("postgres") != "" || fileAccessQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose local_infile/secure_file_priv")
	}
}

// TestFileAccessVerdict covers the full matrix: ON local_infile warns,
// empty secure_file_priv warns (arbitrary paths), NULL means import/
// export disabled entirely (clean), and a dedicated path is clean.
func TestFileAccessVerdict(t *testing.T) {
	path := "/var/lib/mysql-files/"
	tests := []struct {
		name        string
		infileOn    bool
		sfpSet      bool // false = NULL
		sfp         string
		wantWarn    []string // substrings that must appear
		notContains []string
	}{
		{
			name:     "all_secure_clean",
			infileOn: false, sfpSet: true, sfp: path,
			notContains: []string{"WARNING"},
		},
		{
			name:     "local_infile_on",
			infileOn: true, sfpSet: true, sfp: path,
			wantWarn: []string{"WARNING", "local_infile"},
		},
		{
			name:     "empty_secure_file_priv",
			infileOn: false, sfpSet: true, sfp: "",
			wantWarn: []string{"WARNING", "secure_file_priv"},
		},
		{
			name:     "null_disables_io_entirely",
			infileOn: false, sfpSet: false,
			notContains: []string{"WARNING"},
		},
		{
			name:     "both_bad_reports_both",
			infileOn: true, sfpSet: true, sfp: "",
			wantWarn: []string{"local_infile", "secure_file_priv"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileAccessVerdict(tt.infileOn, tt.sfpSet, tt.sfp)
			for _, w := range tt.wantWarn {
				if !strings.Contains(got, w) {
					t.Fatalf("verdict missing %q:\n%s", w, got)
				}
			}
			for _, n := range tt.notContains {
				if strings.Contains(got, n) {
					t.Fatalf("unexpected %q in verdict:\n%s", n, got)
				}
			}
		})
	}
}

// TestParseInfileValue proves the scan-value normalizer accepts every
// shape drivers hand back for local_infile.
func TestParseInfileValue(t *testing.T) {
	for _, on := range []interface{}{int64(1), uint64(1), "1", "ON", []byte("1"), true} {
		if !parseInfileValue(on) {
			t.Fatalf("%#v should parse as ON", on)
		}
	}
	for _, off := range []interface{}{int64(0), "0", "OFF", "", nil, false} {
		if parseInfileValue(off) {
			t.Fatalf("%#v should parse as OFF", off)
		}
	}
}
