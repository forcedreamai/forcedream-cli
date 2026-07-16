package discovery

import (
	"context"
	"time"
)

// ForceDreamConnector wraps the already-proven SearchForceDream function.
type ForceDreamConnector struct{}

func (ForceDreamConnector) Name() string { return "ForceDream" }

func (c ForceDreamConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	if cached, ok := getCached(c.Name(), query, limit); ok {
		return Outcome{Results: cached, Available: true}, nil
	}
	// SearchForceDream takes no limit -- the real marketplace listing is small enough that
	// this has never needed one; not adding a fake, unused parameter to the underlying
	// function just to satisfy this interface's shape.
	start := time.Now()
	var results []Result
	err := withRetry(2, 300*time.Millisecond, func() error {
		var e error
		results, e = SearchForceDream(ctx, query)
		return e
	})
	recordSearchOutcome(c.Name(), err == nil, time.Since(start).Milliseconds())
	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	setCached(c.Name(), query, limit, results)
	return Outcome{Results: results, Available: true}, nil
}

func (ForceDreamConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchForceDream(ctx, "")
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c ForceDreamConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c ForceDreamConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (ForceDreamConnector) Capabilities() Capabilities {
	return Capabilities{RequiresPayment: false, RequiresAPIKey: false, RateLimitPerMinute: 0, SupportsRealtime: true}
}

// MCPRegistryConnector wraps the already-proven SearchMCPRegistry function.
type MCPRegistryConnector struct{}

func (MCPRegistryConnector) Name() string { return "MCP Registry" }

func (c MCPRegistryConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	if cached, ok := getCached(c.Name(), query, limit); ok {
		return Outcome{Results: cached, Available: true}, nil
	}
	start := time.Now()
	var results []Result
	err := withRetry(2, 300*time.Millisecond, func() error {
		var e error
		results, e = SearchMCPRegistry(ctx, query, limit)
		return e
	})
	recordSearchOutcome(c.Name(), err == nil, time.Since(start).Milliseconds())
	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	setCached(c.Name(), query, limit, results)
	return Outcome{Results: results, Available: true}, nil
}

func (MCPRegistryConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchMCPRegistry(ctx, "", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c MCPRegistryConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c MCPRegistryConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (MCPRegistryConnector) Capabilities() Capabilities {
	return Capabilities{RequiresPayment: false, RequiresAPIKey: false, RateLimitPerMinute: 0, SupportsRealtime: true}
}

// GitHubConnector wraps the already-proven SearchGitHubMCPServers function.
type GitHubConnector struct{}

func (GitHubConnector) Name() string { return "GitHub" }

func (c GitHubConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	if cached, ok := getCached(c.Name(), query, limit); ok {
		return Outcome{Results: cached, Available: true}, nil
	}
	start := time.Now()
	var results []Result
	err := withRetry(2, 300*time.Millisecond, func() error {
		var e error
		results, e = SearchGitHubMCPServers(ctx, query, limit)
		return e
	})
	recordSearchOutcome(c.Name(), err == nil, time.Since(start).Milliseconds())
	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	setCached(c.Name(), query, limit, results)
	return Outcome{Results: results, Available: true}, nil
}

func (GitHubConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchGitHubMCPServers(ctx, "", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c GitHubConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c GitHubConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (GitHubConnector) Capabilities() Capabilities {
	// GitHub documents a real 10 req/min limit unauthenticated (raised to 30/min with a
	// real GITHUB_TOKEN) -- a real, known number, not a guess.
	return Capabilities{RequiresPayment: false, RequiresAPIKey: false, RateLimitPerMinute: 10, SupportsRealtime: true}
}

var (
	_ Connector = ForceDreamConnector{}
	_ Connector = MCPRegistryConnector{}
	_ Connector = GitHubConnector{}
)
