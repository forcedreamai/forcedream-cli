package discovery

import "context"

// ForceDreamConnector wraps the already-proven SearchForceDream function.
type ForceDreamConnector struct{}

func (ForceDreamConnector) Name() string { return "ForceDream" }

func (c ForceDreamConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	// SearchForceDream takes no limit -- the real marketplace listing is small enough that
	// this has never needed one; not adding a fake, unused parameter to the underlying
	// function just to satisfy this interface's shape.
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchForceDream(ctx, query)
	})
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
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchMCPRegistry(ctx, query, limit)
	})
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
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchGitHubMCPServers(ctx, query, limit)
	})
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
