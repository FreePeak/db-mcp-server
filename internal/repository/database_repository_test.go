package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

var errCapture = errors.New("stop after context capture")

// captureDB satisfies DatabaseAdapter's driver interface and records the
// context each call receives. It deliberately lacks QueryTimeout, simulating
// connections that do not carry the setting.
type captureDB struct {
	gotQueryCtx context.Context
	gotExecCtx  context.Context
	gotBeginCtx context.Context
}

func (c *captureDB) Query(ctx context.Context, _ string, _ ...interface{}) (*sql.Rows, error) {
	c.gotQueryCtx = ctx
	return nil, errCapture
}

func (c *captureDB) Exec(ctx context.Context, _ string, _ ...interface{}) (sql.Result, error) {
	c.gotExecCtx = ctx
	return nil, errCapture
}

func (c *captureDB) BeginTx(ctx context.Context, _ *sql.TxOptions) (*sql.Tx, error) {
	c.gotBeginCtx = ctx
	return nil, errCapture
}

func (c *captureDB) IsReadOnly() bool             { return false }
func (c *captureDB) Ping(_ context.Context) error { return nil }
func (c *captureDB) DB() *sql.DB                  { return nil }

// timedCaptureDB adds a configured timeout in seconds.
type timedCaptureDB struct {
	*captureDB
	secs int
}

func (t *timedCaptureDB) QueryTimeout() int { return t.secs }

func TestDatabaseAdapter_QueryTimeoutAppliesDeadline(t *testing.T) {
	db := &timedCaptureDB{captureDB: &captureDB{}, secs: 5}
	a := &DatabaseAdapter{db: db}

	if _, err := a.Query(context.Background(), "SELECT 1"); !errors.Is(err, errCapture) {
		t.Fatalf("expected capture-stub error, got %v", err)
	}
	deadline, ok := db.gotQueryCtx.Deadline()
	if !ok {
		t.Fatal("expected deadline on query context")
	}
	if until := time.Until(deadline); until > 5*time.Second || until < 4*time.Second {
		t.Errorf("expected ~5s deadline, got %v", until)
	}
}

func TestDatabaseAdapter_ExecTimeoutAppliesDeadline(t *testing.T) {
	db := &timedCaptureDB{captureDB: &captureDB{}, secs: 3}
	a := &DatabaseAdapter{db: db}

	if _, err := a.Exec(context.Background(), "UPDATE t SET x = 1"); !errors.Is(err, errCapture) {
		t.Fatalf("expected capture-stub error, got %v", err)
	}
	deadline, ok := db.gotExecCtx.Deadline()
	if !ok {
		t.Fatal("expected deadline on exec context")
	}
	if until := time.Until(deadline); until > 3*time.Second || until < 2*time.Second {
		t.Errorf("expected ~3s deadline, got %v", until)
	}
}

func TestDatabaseAdapter_NoCapabilityNoDeadline(t *testing.T) {
	db := &captureDB{}
	a := &DatabaseAdapter{db: db}
	if _, err := a.Exec(context.Background(), "UPDATE t SET x = 1"); !errors.Is(err, errCapture) {
		t.Fatalf("expected capture-stub error, got %v", err)
	}
	if _, ok := db.gotExecCtx.Deadline(); ok {
		t.Fatal("expected no deadline when connection carries no timeout setting")
	}
}

func TestDatabaseAdapter_ZeroTimeoutNoDeadline(t *testing.T) {
	db := &timedCaptureDB{captureDB: &captureDB{}, secs: 0}
	a := &DatabaseAdapter{db: db}
	if _, err := a.Begin(context.Background(), nil); !errors.Is(err, errCapture) {
		t.Fatalf("expected capture-stub error, got %v", err)
	}
	if _, ok := db.gotBeginCtx.Deadline(); ok {
		t.Fatal("expected no deadline when timeout is zero (disabled)")
	}
}
