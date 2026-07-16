package discovery

import (
	"context"
	"time"
)

// NpmConnector wraps the already-proven SearchNpmMCPServers function (confirmed live
// before this framework existed) in the unified Connector interface -- the real logic is
// not duplicated here, only adapted to the new, shared shape.
type NpmConnector struct{}

func (NpmConnector) Name() string { return "npm" }

func (c NpmConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	if cached, ok := getCached(c.Name(), query, limit); ok {
		return Outcome{Results: cached, Available: true}, nil
	}
	start := time.Now()
	var results []Result
	err := withRetry(2, 300*time.Millisecond, func() error {
		var e error
		results, e = SearchNpmMCPServers(ctx, query, limit)
		return e
	})
	recordSearchOutcome(c.Name(), err == nil, time.Since(start).Milliseconds())
	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	setCached(c.Name(), query, limit, results)
	return Outcome{Results: results, Available: true}, nil
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
