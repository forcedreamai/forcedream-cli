package discovery

import (
	"context"
	"time"
)

// SmitheryConnector wraps the already-proven SearchSmithery function. Paid-source specific
// care taken here, distinct from the free connectors: retries apply only to genuine
// request/network failures (err != nil), never to a real gate rejection (status.Available
// == false, e.g. "insufficient_funds") -- retrying a rejection is pointless, since the
// account's balance won't have changed in the few hundred milliseconds between attempts.
// Caching applies only to real, successful results for the same reason in reverse: caching
// a rejection would mean a person who tops up their balance or sets FD_LIVE_KEY between
// calls could still see a stale "unavailable" result. A cache hit here means a real, paid
// call is skipped entirely -- genuinely saves the person money on a repeated identical
// query, not just time.
type SmitheryConnector struct{}

func (SmitheryConnector) Name() string { return "Smithery" }

func (c SmitheryConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	if cached, ok := getCached(c.Name(), query, limit); ok {
		return Outcome{Results: cached, Available: true}, nil
	}

	start := time.Now()
	var results []Result
	var status PaidSourceStatus
	err := withRetry(2, 300*time.Millisecond, func() error {
		var e error
		results, status, e = SearchSmithery(ctx, query)
		return e
	})
	recordSearchOutcome(c.Name(), err == nil && status.Available, time.Since(start).Milliseconds())

	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	if !status.Available {
		// A real, specific reason from the backend gate -- not cached, not retried, since
		// neither would change a real auth/balance/entitlement state.
		return Outcome{Available: false, Reason: status.Reason, Message: status.Message}, nil
	}
	setCached(c.Name(), query, limit, results)
	return Outcome{Results: results, Available: true}, nil
}

func (SmitheryConnector) Health(ctx context.Context) HealthStatus {
	// A real health check must never spend real money -- Smithery requires a funded
	// FD_LIVE_KEY to search at all, so "healthy" here means the backend proxy route
	// itself is reachable, not that a real, billable search would succeed. Reports the
	// same auth_required gate as a non-fatal, expected state rather than "unhealthy".
	_, status, err := SearchSmithery(ctx, "")
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	if !status.Available && status.Reason != "auth_required" {
		return HealthStatus{Healthy: false, Message: status.Message}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c SmitheryConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c SmitheryConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (SmitheryConnector) Capabilities() Capabilities {
	return Capabilities{RequiresPayment: true, RequiresAPIKey: false, RateLimitPerMinute: 0, SupportsRealtime: true}
}

var _ Connector = SmitheryConnector{}
