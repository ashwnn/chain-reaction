package llm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// retryableError signals that the underlying HTTP call received a transient
// error (429 or 5xx) and should be retried.
type retryableError struct {
	StatusCode int
	Message    string
}

func (e *retryableError) Error() string {
	return fmt.Sprintf("retryable error (status %d): %s", e.StatusCode, e.Message)
}

// retryProvider wraps any Provider with exponential backoff retry logic.
// Backoff sleeps are bounded by the remaining context deadline so that
// retry delays cannot extend a run past its declared time budget.
type retryProvider struct {
	base       Provider
	maxRetries int // default: 3
}

func (p *retryProvider) Complete(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (ChatAction, error) {
	var lastErr error
	for attempt := range p.maxRetries + 1 {
		action, err := p.base.Complete(ctx, messages, tools)
		if err == nil {
			return action, nil
		}

		var retryable *retryableError
		if !errors.As(err, &retryable) {
			// Non-retryable error (400, 401, 403, parse errors, etc.)
			return ChatAction{}, err
		}

		lastErr = err
		if attempt < p.maxRetries {
			backoff := backoffDuration(attempt)
			// Bound backoff by the remaining context deadline so that retry
			// delays cannot push a run past its declared time budget.
			if deadline, ok := ctx.Deadline(); ok {
				remaining := time.Until(deadline)
				if remaining <= 0 {
					// Deadline already expired; stop retrying immediately.
					return ChatAction{}, fmt.Errorf("retry deadline expired: %w", ctx.Err())
				}
				if backoff > remaining {
					backoff = remaining
				}
			}
			// Wait for backoff or context cancellation — whichever fires first.
			// This ensures the run exits promptly when the budget expires,
			// even if the backoff timer has not yet fired.
			select {
			case <-ctx.Done():
				return ChatAction{}, fmt.Errorf("retry cancelled: %w", ctx.Err())
			case <-time.After(backoff):
				// continue to next attempt
			}
		}
	}
	return ChatAction{}, fmt.Errorf("max retries (%d) exceeded: %w", p.maxRetries, lastErr)
}

// backoffDuration returns an exponential backoff with jitter.
// attempt 0 → ~1s, attempt 1 → ~2s, attempt 2 → ~4s.
func backoffDuration(attempt int) time.Duration {
	base := math.Pow(2, float64(attempt)) // 1, 2, 4, ...
	jitter := rand.Float64() * 0.5        // 0–0.5s jitter
	return time.Duration((base+jitter)*1000) * time.Millisecond
}
