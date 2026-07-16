// Package ranking is a standalone subsystem, architecturally independent from the
// discovery package it ranks results from: its own data model (Weights), its own engine
// (Rank/Score), its own test suite, and its own version/release cycle (Version below). It
// integrates with discovery only through the shared discovery.Result type -- no shared
// internal state, no reaching into discovery's own internals.
package ranking

// Version is this ranking engine's own version, independent of the CLI's overall version --
// the "own release cycle" this subsystem is required to have.
const Version = "2.0.0"

// Weights are the real, configurable tuning parameters for the v2 ranking engine. All
// fields have sensible defaults via DefaultWeights() -- callers needing different behavior
// construct their own Weights rather than editing constants in engine.go, keeping the
// engine itself free of hardcoded tuning values.
type Weights struct {
	// SourceWeight is a per-source multiplier applied to the popularity component,
	// reflecting that a GitHub star and a Smithery use-count aren't equally strong
	// signals of real quality/adoption. Keyed by the exact Source string discovery.Result
	// uses ("GitHub", "Smithery", "MCP Registry", "Web"). A source not present here gets
	// 1.0 (neutral) via SourceWeightFor.
	SourceWeight map[string]float64

	// PopularityWeight scales the (log-normalized, source-weighted) popularity component
	// in the final composite score.
	PopularityWeight float64

	// RecencyWeight scales the recency component. Results with no LastUpdated data get
	// a neutral (zero) recency contribution rather than being penalized for missing data
	// a given source simply doesn't provide.
	RecencyWeight float64

	// RecencyHalfLifeDays controls how fast the recency component decays: a result
	// updated exactly this many days ago scores half of a result updated today.
	RecencyHalfLifeDays float64

	// MCPNativeBonus is a flat bonus added for results from the MCP Registry source
	// specifically -- real, formally-registered MCP servers, a stronger signal than a
	// GitHub repo merely topic-tagged "mcp-server" by its own author.
	MCPNativeBonus float64

	// MCPTagBonus is a smaller bonus for any result (regardless of source) that carries
	// a real "mcp-server" or "mcp" tag, recognizing genuine MCP-native metadata even when
	// the result didn't come from the formal registry itself.
	MCPTagBonus float64

	// VerificationBonus is a flat bonus for results with real cryptographic verification
	// (discovery.Result.Verified) -- real, already-existing data on every ForceDream
	// result that v2's original scoring never actually used. Not a new signal invented
	// for this framework, just a genuine gap being closed.
	VerificationBonus float64
}

// DefaultWeights returns the real, shipped defaults. ForceDream's own always-first rule
// (independent of these weights entirely) is enforced in engine.go, not here.
func DefaultWeights() Weights {
	return Weights{
		SourceWeight: map[string]float64{
			"MCP Registry": 1.4, // formally-registered; the strongest non-ForceDream signal
			"GitHub":       1.0,
			"Smithery":     0.9, // real, but a shallower listing bar than GitHub stars
			"Web":          0.5, // weakest signal: no popularity metric at all, most results
		},
		PopularityWeight:    1.0,
		RecencyWeight:       0.35,
		RecencyHalfLifeDays: 180,
		MCPNativeBonus:      25,
		MCPTagBonus:         8,
		VerificationBonus:   15,
	}
}

// SourceWeightFor looks up the configured weight for a source, defaulting to neutral (1.0)
// for any source name not explicitly configured -- so a new source added to discovery
// later doesn't silently get zero-weighted just because ranking wasn't updated for it.
func (w Weights) SourceWeightFor(source string) float64 {
	if v, ok := w.SourceWeight[source]; ok {
		return v
	}
	return 1.0
}
