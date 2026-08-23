package db

import (
	"errors"
	"testing"
	"time"
)

// retryStubDB simulates a scale-to-zero cloud database that refuses the
// first N connect attempts (cold start) before succeeding.
type retryStubDB struct {
	database
	failures int
	connects int
}

func (r *retryStubDB) Connect() error {
	r.connects++
	if r.connects <= r.failures {
		return errors.New("connection refused")
	}
	return nil
}

func TestConnectWithRetry_RecoversFromColdStart(t *testing.T) {
	stub := &retryStubDB{failures: 2}
	err := connectWithRetry(stub, 4, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("expected success on attempt 3, got: %v", err)
	}
	if stub.connects != 3 {
		t.Fatalf("expected 3 attempts, got %d", stub.connects)
	}
}

func TestConnectWithRetry_ExhaustsAttempts(t *testing.T) {
	stub := &retryStubDB{failures: 99}
	err := connectWithRetry(stub, 3, 1*time.Millisecond)
	if err == nil {
		t.Fatal("expected exhaustion error after 3 failed attempts")
	}
	if stub.connects != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", stub.connects)
	}
}
