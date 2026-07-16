package discovery

import (
	"context"
	"time"
)

// WebConnector wraps the already-proven SearchWeb function (SerpAPI-backed). Same
// paid-source care as SmitheryConnector: retries only on genuine request failures, never
// on a real gate rejection; caching only real, successful results.
type WebConnector struct{}

func (WebConnector) Name() string { return "Web search" }

func (c WebConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	if cached, ok := getCached(c.Name(), query, limit); ok {
		return Outcome{Results: cached, Available: true}, nil
	}

	start := time.Now()
	var results []Result
	var status PaidSourceStatus
	err := withRetry(2, 300*time.Millisecond, func() error {
		var e error
		results, status, e = SearchWeb(ctx, query)
		return e
	})
	recordSearchOutcome(c.Name(), err == nil && status.Available, time.Since(start).Milliseconds())

	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	if !status.Available {
		return Outcome{Available: false, Reason: status.Reason, Message: status.Message}, nil
	}
	setCached(c.Name(), query, limit, results)
	return Outcome{Results: results, Available: true}, nil
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
