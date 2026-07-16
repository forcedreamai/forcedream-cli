// Package reputation is a standalone subsystem, architecturally independent from
// discovery/entity/graph/confidence: its own data model, its own scoring, its own tests.
//
// The explicit, permanent rule this package enforces: Developer, Agent, Connector, and
// Package reputation are four genuinely separate scores -- never averaged, never
// collapsed into one number. A developer with a poor track record and a specific,
// excellent agent they published are both real, true, separate facts; conflating them
// into one score would destroy exactly the information a person needs. Same honesty rule
// as confidence: a dimension with no real, available data returns Insufficient Data,
// never a fabricated or guessed number.
package reputation

import (
	"github.com/forcedreamai/forcedream-cli/internal/discovery"
	"github.com/forcedreamai/forcedream-cli/internal/entity"
	"github.com/forcedreamai/forcedream-cli/internal/graph"
)

const Version = "1.0.0"

const InsufficientData = "Insufficient Data"

type ComponentScore struct {
	Dimension   string  `json:"dimension"`
	Available   bool    `json:"available"`
	Score       float64 `json:"score,omitempty"`
	Explanation string  `json:"explanation"`
}

// Reputation is one subject's (a developer, an agent, a connector, or a package) real,
// transparent breakdown -- never a single opaque number, same principle as Confidence.
type Reputation struct {
	Subject    string           `json:"subject"`
	Components []ComponentScore `json:"components"`
}

// DeveloperReputation is scoped to a real, mechanically-extracted organization -- built
// only from what the local knowledge graph actually shows (how many distinct entities
// this org has published, as discovered on this machine so far). Everything the original
// spec lists beyond that -- successful invocations, SDK quality, release cadence, support
// responsiveness, security incidents -- has no real data source anywhere in this
// codebase, and is honestly reported as Insufficient Data rather than invented.
func DeveloperReputation(organization string) Reputation {
	published := graph.Load().EdgesTo("org:" + organization)
	components := []ComponentScore{
		publishedCount(published),
		unavailable("successful_invocations", "no accessible record of real invocation history exists from the CLI's perspective"),
		unavailable("sdk_quality", "no real SDK-quality signal is collected anywhere in this codebase"),
		unavailable("release_cadence", "no real, fetched version/release history exists for any connector"),
		unavailable("support_responsiveness", "no real support-interaction data is collected anywhere"),
		unavailable("security_incidents", "no real vulnerability/incident data source is integrated"),
	}
	return Reputation{Subject: organization, Components: components}
}

func publishedCount(edges []graph.Edge) ComponentScore {
	n := len(edges)
	if n == 0 {
		return ComponentScore{Dimension: "published_entities", Available: false, Explanation: "no entities from this organization have been discovered and resolved on this machine yet"}
	}
	score := 100.0
	if n < 5 {
		score = 40 + float64(n)*12
	}
	return ComponentScore{Dimension: "published_entities", Available: true, Score: score, Explanation: "real count of distinct resolved entities published by this organization, as discovered so far on this machine"}
}

// AgentReputation is scoped to a single, specific resolved entity -- built from real
// popularity (stars/downloads) and verification signals genuinely present on it.
// Invocation success rate, failure rate, and security incidents are honestly
// Insufficient Data -- this CLI has no accessible record of any of them.
func AgentReputation(e entity.Entity, matchingResults []discovery.Result) Reputation {
	components := []ComponentScore{
		realPopularity(matchingResults),
		realVerification(matchingResults),
		unavailable("invocation_success_rate", "no accessible record of real invocation outcomes exists from the CLI's perspective"),
		unavailable("failure_rate", "same reason -- no real invocation history is accessible here"),
		unavailable("security_incidents", "no real vulnerability/incident data source is integrated"),
	}
	return Reputation{Subject: e.CanonicalName, Components: components}
}

func realPopularity(results []discovery.Result) ComponentScore {
	var maxMetric int
	found := false
	for _, r := range results {
		if r.Stars != nil && *r.Stars > maxMetric {
			maxMetric = *r.Stars
			found = true
		}
		if r.UseCount != nil && *r.UseCount > maxMetric {
			maxMetric = *r.UseCount
			found = true
		}
	}
	if !found {
		return ComponentScore{Dimension: "popularity", Available: false, Explanation: "no contributing source exposed a real stars/download-count metric"}
	}
	// log-ish, capped scale -- consistent in spirit with ranking's own popularity
	// normalization, without importing that package (architectural independence).
	score := 100.0
	switch {
	case maxMetric < 10:
		score = 30
	case maxMetric < 100:
		score = 55
	case maxMetric < 1000:
		score = 75
	case maxMetric < 10000:
		score = 90
	}
	return ComponentScore{Dimension: "popularity", Available: true, Score: score, Explanation: "based on the real, highest stars/download-count metric among contributing sources"}
}

func realVerification(results []discovery.Result) ComponentScore {
	for _, r := range results {
		if r.Verified {
			return ComponentScore{Dimension: "verification", Available: true, Score: 100, Explanation: "cryptographically verified by at least one contributing source"}
		}
	}
	return ComponentScore{Dimension: "verification", Available: true, Score: 30, Explanation: "no cryptographic verification found"}
}

// ConnectorReputation surfaces the real, already-built connector reliability tracking
// (from tonight's earlier hardening work) as its own, explicit reputation view -- this
// dimension has always had genuine, real, locally-observed data behind it.
func ConnectorReputation(connectorName string) Reputation {
	rel := discovery.ReliabilityForSource(connectorName)
	var component ComponentScore
	if !rel.HasData {
		component = unavailable("reliability", "no real, recorded search history exists for this connector on this machine yet")
	} else {
		component = ComponentScore{
			Dimension: "reliability", Available: true, Score: rel.SuccessRate * 100,
			Explanation: "real, locally-observed success rate over this connector's actual search attempts",
		}
	}
	return Reputation{Subject: connectorName, Components: []ComponentScore{component}}
}

// PackageReputation is scoped to package-sourced entities (currently: npm) -- built from
// the one real adoption signal available (weekly download count). Security-advisory
// history and maintenance-activity signals are honestly Insufficient Data -- no real
// vulnerability-database or release-history integration exists.
func PackageReputation(matchingResults []discovery.Result) Reputation {
	subject := "package"
	if len(matchingResults) > 0 {
		subject = matchingResults[0].Name
	}
	components := []ComponentScore{
		realPopularity(matchingResults),
		unavailable("security_advisories", "no real vulnerability-database integration exists for any connector"),
		unavailable("maintenance_activity", "no real, fetched release-history data exists for any connector"),
	}
	return Reputation{Subject: subject, Components: components}
}

func unavailable(dimension, reason string) ComponentScore {
	return ComponentScore{Dimension: dimension, Available: false, Explanation: reason}
}
