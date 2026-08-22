package db

import (
	"fmt"
	"time"
)

// connectWithRetry retries a database connect with linear backoff, absorbing
// transient refusals from scale-to-zero cloud tiers (Neon sleeps after idle;
// first wake attempts can fail). Non-transient errors burn through the same
// attempt budget, which stays bounded and simple.
func connectWithRetry(db Database, attempts int, backoff time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(backoff * time.Duration(i))
		}
		if err = db.Connect(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("connect failed after %d attempts: %w", attempts, err)
}
