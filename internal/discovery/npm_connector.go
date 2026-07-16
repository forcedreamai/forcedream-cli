package discovery

import "context"

// NpmConnector wraps the already-proven SearchNpmMCPServers function. Retry/cache/stats
// are no longer implemented here at all -- centralized in instrumentedSearch (see
// instrumentation.go), which every free connector now shares.
type NpmConnector struct{}

func (NpmConnector) Name() string { return "npm" }

func (c NpmConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchNpmMCPServers(ctx, query, limit)
	})
}

func (NpmConnector) Health(ctx context.Context) HealthStatus {
	// A real, minimal, free query -- not a paid call, and cheap enough that a status
	// command could reasonably call this for every connector without real cost or delay.
	_, err := SearchNpmMCPServers(ctx, "mcp", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c NpmConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c NpmConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (NpmConnector) Capabilities() Capabilities {
	return Capabilities{
		RequiresPayment:    false,
		RequiresAPIKey:     false,
		RateLimitPerMinute: 0, // npm's public search API does not document a specific per-minute limit
		SupportsRealtime:   true,
	}
}
