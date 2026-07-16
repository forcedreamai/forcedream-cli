package discovery

import "context"

// NpmConnector wraps the already-proven SearchNpmMCPServers function (confirmed live
// before this framework existed) in the unified Connector interface -- the real logic is
// not duplicated here, only adapted to the new, shared shape. This is the first real
// implementation of the framework, used to verify the interface design actually holds up
// against a genuine, working source before any other connector is migrated to it.
type NpmConnector struct{}

func (NpmConnector) Name() string { return "npm" }

func (NpmConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	results, err := SearchNpmMCPServers(ctx, query, limit)
	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
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

func (NpmConnector) Capabilities() Capabilities {
	return Capabilities{
		RequiresPayment:    false,
		RequiresAPIKey:     false,
		RateLimitPerMinute: 0, // npm's public search API does not document a specific per-minute limit
	}
}
