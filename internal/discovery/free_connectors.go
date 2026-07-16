package discovery

import "context"

// ForceDreamConnector wraps the already-proven SearchForceDream function.
type ForceDreamConnector struct{}

func (ForceDreamConnector) Name() string { return "ForceDream" }

func (ForceDreamConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	// SearchForceDream takes no limit -- the real marketplace listing is small enough that
	// this has never needed one; not adding a fake, unused parameter to the underlying
	// function just to satisfy this interface's shape.
	results, err := SearchForceDream(ctx, query)
	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	return Outcome{Results: results, Available: true}, nil
}

func (ForceDreamConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchForceDream(ctx, "")
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (ForceDreamConnector) Capabilities() Capabilities {
	return Capabilities{RequiresPayment: false, RequiresAPIKey: false, RateLimitPerMinute: 0}
}

// MCPRegistryConnector wraps the already-proven SearchMCPRegistry function.
type MCPRegistryConnector struct{}

func (MCPRegistryConnector) Name() string { return "MCP Registry" }

func (MCPRegistryConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	results, err := SearchMCPRegistry(ctx, query, limit)
	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	return Outcome{Results: results, Available: true}, nil
}

func (MCPRegistryConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchMCPRegistry(ctx, "", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (MCPRegistryConnector) Capabilities() Capabilities {
	return Capabilities{RequiresPayment: false, RequiresAPIKey: false, RateLimitPerMinute: 0}
}

// GitHubConnector wraps the already-proven SearchGitHubMCPServers function.
type GitHubConnector struct{}

func (GitHubConnector) Name() string { return "GitHub" }

func (GitHubConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	results, err := SearchGitHubMCPServers(ctx, query, limit)
	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	return Outcome{Results: results, Available: true}, nil
}

func (GitHubConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchGitHubMCPServers(ctx, "", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (GitHubConnector) Capabilities() Capabilities {
	// GitHub documents a real 10 req/min limit unauthenticated (raised to 30/min with a
	// real GITHUB_TOKEN) -- a real, known number, not a guess.
	return Capabilities{RequiresPayment: false, RequiresAPIKey: false, RateLimitPerMinute: 10}
}

var (
	_ Connector = ForceDreamConnector{}
	_ Connector = MCPRegistryConnector{}
	_ Connector = GitHubConnector{}
)
