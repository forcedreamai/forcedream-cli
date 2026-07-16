package discovery

import (
	"time"
)

// withRetry retries fn up to maxAttempts times on any error, with a short, fixed delay
// between attempts -- a simple, honest policy for the free sources' real failure modes
// (network blips, transient rate-limit responses), which don't have the auth/balance/
// entitlement-style permanent failures paid sources do. Returns the last real error if
// every attempt fails.
func withRetry(maxAttempts int, delay time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}
