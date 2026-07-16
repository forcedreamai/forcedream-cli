package discovery

import "context"

// Connector is the unified interface every search source implements. This is the real
// framework behind "adding another ecosystem becomes a 50-line connector, not another
// rewrite": a new source implements this interface once, rather than hand-wiring a new
// function into main.go's goroutine-launching code and hoping the pattern stays
// consistent by convention alone.
type Connector interface {
	// Name is the fixed, real source name used throughout -- matches discovery.Result.Source,
	// ranking's SourceWeight map keys, and telemetry's SourcesUsed list. A single source of
	// truth for this string, rather than it being retyped slightly differently in three
	// different places the way the pre-framework sources did.
	Name() string

	// Search performs a real query against this source. Outcome.Available/Reason/Message
	// unify what PaidSourceStatus already captured for Smithery/Web, generalized to every
	// connector: free sources can now report *why* they're unavailable (rate limited,
	// misconfigured) the same structured way paid sources report auth/balance/entitlement
	// reasons, rather than free sources only ever returning a bare Go error with no fixed
	// reason code a caller could branch on.
	Search(ctx context.Context, query string, limit int) (Outcome, error)

	// Health performs a real, lightweight reachability check -- not a full search, and
	// critically, never a paid one (a health check must never spend real money). Lets a
	// future status command report which sources are currently up without the cost of a
	// full query. This is a live, active probe -- distinct from Latency()/Reliability()
	// below, which report historical trend instead.
	Health(ctx context.Context) HealthStatus

	// Latency reports this connector's real, locally-observed average response time,
	// accumulated from actual past searches -- not a static or guessed number, since
	// latency isn't a fact about a connector in the abstract, only something measurable
	// after real calls have happened. HasData is false (and AverageMs meaningless) until
	// this connector has actually been searched at least once on this machine.
	Latency() LatencyInfo

	// Reliability reports this connector's real, locally-observed success rate,
	// accumulated the same way. Same honesty rule: HasData is false until real attempts
	// exist to compute a rate from.
	Reliability() ReliabilityInfo

	// Capabilities describes this source's real, fixed properties -- known in advance, not
	// detected per-call, since they don't change between invocations.
	Capabilities() Capabilities
}

// LatencyInfo is Latency()'s real result: either a genuine, measured average, or an
// honest "nothing observed yet" -- never a fabricated default standing in for real data.
type LatencyInfo struct {
	HasData   bool
	AverageMs int64
}

// ReliabilityInfo is Reliability()'s real result, same honesty rule as LatencyInfo.
type ReliabilityInfo struct {
	HasData     bool
	SuccessRate float64 // 0.0-1.0
	Attempts    int
}

// Outcome is Search's real result: either real data, or a real, structured reason it
// wasn't available -- never a source silently returning zero results with no explanation.
type Outcome struct {
	Results   []Result
	Available bool
	Reason    string // fixed, machine-readable reason code: "auth_required", "rate_limited", "not_configured", "quota_exceeded", or "" when Available is true
	Message   string // real, human-readable explanation shown to the person running the CLI
}

// Capabilities are this connector's fixed, known-in-advance properties -- the "capabilities()"
// piece of the framework, used by callers (and eventually ranking) to make real decisions
// (e.g. whether to even attempt a source without a configured credential) without
// duplicating per-source knowledge in main.go.
type Capabilities struct {
	RequiresPayment    bool // true for Smithery/SerpAPI -- needs a real FD_LIVE_KEY, spends a real balance
	RequiresAPIKey     bool // true if this source needs its own, non-ForceDream credential (e.g. GITHUB_TOKEN, optional but real when present)
	RateLimitPerMinute int  // 0 = effectively unbounded/unknown; a real, known number where the source documents one (e.g. GitHub's unauthenticated 10/min)
	SupportsRealtime   bool // true if this source's data reflects the live, current state of the underlying registry (all of ForceDream/MCP Registry/GitHub/npm/Smithery/Web genuinely do -- none of them serve a stale, batch-crawled snapshot); false would mean a source backed by a periodic, non-live crawl
}

// HealthStatus is a real, current reachability result -- not a cached or assumed value.
type HealthStatus struct {
	Healthy bool
	Message string
}
