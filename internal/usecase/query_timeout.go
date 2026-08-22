package usecase

import (
	"context"
	"fmt"
	"time"
)

// Per-query timeout: lets an agent bound a speculative or recursive query
// without operator-side statement_timeout configuration.

// ExecuteQueryWithTimeout runs ExecuteQuery under a deadline. timeoutMs <= 0
// means no timeout (identical to ExecuteQuery).
func (uc *DatabaseUseCase) ExecuteQueryWithTimeout(ctx context.Context, dbID, query string, params []interface{}, timeoutMs int) (string, error) {
	if timeoutMs <= 0 {
		return uc.ExecuteQuery(ctx, dbID, query, params)
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	out, err := uc.ExecuteQuery(tctx, dbID, query, params)
	if err != nil && ctx.Err() == nil && tctx.Err() != nil {
		return "", fmt.Errorf("query exceeded %dms timeout: %w", timeoutMs, err)
	}
	return out, err
}
