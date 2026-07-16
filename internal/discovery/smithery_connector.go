package discovery

import "context"

// SmitheryConnector wraps the already-proven SearchSmithery function. Retry/cache/stats
// are centralized in instrumentedPaidSearch (instrumentation.go), which correctly never
// retries or caches a real gate rejection (auth_required/insufficient_funds/etc) -- only
// genuine request failures are retried, and only real successes are cached.
type SmitheryConnector struct{}

func (SmitheryConnector) Name() string { return "Smithery" }

func (c SmitheryConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return instrumentedPaidSearch(c.Name(), query, limit, func() ([]Result, PaidSourceStatus, error) {
		return SearchSmithery(ctx, query)
	})
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
