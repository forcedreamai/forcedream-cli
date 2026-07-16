package discovery

import "context"

// WebConnector wraps the already-proven SearchWeb function (SerpAPI-backed). Same
// centralized retry/cache/stats behavior as SmitheryConnector, via instrumentedPaidSearch.
type WebConnector struct{}

func (WebConnector) Name() string { return "Web search" }

func (c WebConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return instrumentedPaidSearch(c.Name(), query, limit, func() ([]Result, PaidSourceStatus, error) {
		return SearchWeb(ctx, query)
	})
}

func (WebConnector) Health(ctx context.Context) HealthStatus {
	_, status, err := SearchWeb(ctx, "")
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	if !status.Available && status.Reason != "auth_required" {
		return HealthStatus{Healthy: false, Message: status.Message}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c WebConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c WebConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (WebConnector) Capabilities() Capabilities {
	// SerpAPI-backed, real quota tracked server-side (250/month free tier) -- 0 here
	// since that's a monthly cap, not a documented per-minute rate limit.
	return Capabilities{RequiresPayment: true, RequiresAPIKey: false, RateLimitPerMinute: 0, SupportsRealtime: true}
}

var _ Connector = WebConnector{}
