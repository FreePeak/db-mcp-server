package db

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
)

// specialCharPasswords is the fixture covering the character classes that
// broke DSN building for issue #106 (Oracle '#' password): hash, URL
// separators, percent-escape start, space, single quote, leading quote,
// backslash and colon.
var specialCharPasswords = []string{
	"pa#ss",       // issue #106 character
	"p@ss/w%rd?x", // URL-separator heavy
	"pa ss",       // space
	"pa'ss",       // single quote (mid value)
	"'pass",       // leading single quote
	`pa\ss`,       // backslash
	"#onlyhash",   // hash at start
	"p:ass",       // colon
}

// TestPQValue locks in the lib/pq key=value rendering rules: bare values
// when safe, single-quoted with backslash escapes when the value contains a
// space, single quote or backslash.
func TestPQValue(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"localhost", "localhost"},
		{"testpass", "testpass"},
		{"pa#ss", "pa#ss"},
		{"p@ss/w%rd?x", "p@ss/w%rd?x"},
		{"p:ass", "p:ass"},
		{"pa ss", "'pa ss'"},
		{"pa'ss", `'pa\'ss'`},
		{"'pass", `'\'pass'`},
		{`pa\ss`, `'pa\\ss'`},
		{`a \ b'c`, `'a \\ b\'c'`},
		{"", ""},
	}
	for _, tc := range cases {
		if got := pqValue(tc.in); got != tc.want {
			t.Errorf("pqValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestBuildPostgresConnStr_SpecialCharPassword asserts the exact DSN segment
// for each problematic password, proves lib/pq accepts the DSN, and reads
// back the parsed password from the driver's connector to prove a true
// round-trip (not just string shape).
func TestBuildPostgresConnStr_SpecialCharPassword(t *testing.T) {
	cases := []struct {
		password string
		segment  string // exact "password=..." segment expected in the DSN
	}{
		{"pa#ss", "password=pa#ss"},
		{"p@ss/w%rd?x", "password=p@ss/w%rd?x"},
		{"pa ss", "password='pa ss'"},
		{"pa'ss", `password='pa\'ss'`},
		{"'pass", `password='\'pass'`},
		{`pa\ss`, `password='pa\\ss'`},
		{"#onlyhash", "password=#onlyhash"},
		{"p:ass", "password=p:ass"},
	}
	for _, tc := range cases {
		cfg := Config{Type: "postgres", Host: "localhost", Port: 5432, User: "user", Password: tc.password, Name: "db1", SSLMode: "disable"}
		dsn := buildPostgresConnStr(cfg)

		if !strings.Contains(dsn, tc.segment) {
			t.Errorf("password %q: DSN %q does not contain segment %q", tc.password, dsn, tc.segment)
			continue
		}

		conn, err := pq.NewConnector(dsn)
		if err != nil {
			t.Errorf("password %q: pq.NewConnector(%q) failed: %v", tc.password, dsn, err)
			continue
		}

		if got := pqConnectorOpt(conn, "password"); got != tc.password {
			t.Errorf("password %q: driver parsed password %q (dsn=%q)", tc.password, got, dsn)
		}
		if got := pqConnectorOpt(conn, "user"); got != "user" {
			t.Errorf("password %q: driver parsed user %q (dsn=%q)", tc.password, got, dsn)
		}
		if got := pqConnectorOpt(conn, "dbname"); got != "db1" {
			t.Errorf("password %q: driver parsed dbname %q (dsn=%q)", tc.password, got, dsn)
		}
	}
}

// TestBuildPostgresConnStr_PlainPasswordUnchanged guards existing users: a
// password without special characters must produce the byte-identical DSN it
// produced before the quoting fix.
func TestBuildPostgresConnStr_PlainPasswordUnchanged(t *testing.T) {
	cfg := Config{Type: "postgres", Host: "localhost", Port: 5432, User: "testuser", Password: "testpass", Name: "testdb", SSLMode: "disable"}
	want := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	got := buildPostgresConnStr(cfg)
	if got != want {
		t.Fatalf("plain-password DSN changed:\n got %q\nwant %q", got, want)
	}
}

// TestMySQLSpecialCharPasswordRegression locks in the probe finding that the
// MySQL DSN builder already handles every password in the fixture: the
// driver parses user, password, database and address back exactly.
func TestMySQLSpecialCharPasswordRegression(t *testing.T) {
	for _, pw := range specialCharPasswords {
		dsn := buildMySQLConnStr(Config{Type: "mysql", Host: "localhost", Port: 3306, User: "user", Password: pw, Name: "db1"})
		parsed, err := mysql.ParseDSN(dsn)
		if err != nil {
			t.Errorf("mysql password %q: ParseDSN(%q) failed: %v", pw, dsn, err)
			continue
		}
		if parsed.Passwd != pw {
			t.Errorf("mysql password %q: parsed %q (dsn=%q)", pw, parsed.Passwd, dsn)
		}
		if parsed.User != "user" || parsed.DBName != "db1" || parsed.Addr != "localhost:3306" {
			t.Errorf("mysql password %q: fields user=%q db=%q addr=%q (dsn=%q)", pw, parsed.User, parsed.DBName, parsed.Addr, dsn)
		}
	}
}

// pqConnectorOpt reads one parsed connection option back out of a
// pq.Connector. lib/pq stores the parsed key=value options in an unexported
// map and exposes no accessor, so the test reaches it directly; this is the
// only way to prove the driver parsed exactly what the builder intended.
func pqConnectorOpt(c *pq.Connector, key string) string {
	v := reflect.ValueOf(c).Elem().FieldByName("opts")
	v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	return v.MapIndex(reflect.ValueOf(key)).String()
}
