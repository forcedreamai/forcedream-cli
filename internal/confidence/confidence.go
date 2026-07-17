// Package confidence is a standalone subsystem, architecturally independent from the
// discovery/entity packages it reads from: its own data model, its own scoring logic, its
// own tests. Integrates only through the shared discovery.Result/entity.Entity types.
//
// The core rule this package exists to enforce: confidence is never a single opaque
// number. Every score exposes exactly which real signals produced it, and any dimension
// that genuinely can't be computed from real, available data returns "Insufficient Data"
// -- never a fabricated or guessed value standing in for evidence that doesn't exist.
package confidence

import (
	"math"
	"time"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
	"github.com/forcedreamai/forcedream-cli/internal/entity"
)

const Version = "1.0.0"

// InsufficientData is the exact, fixed string every genuinely uncomputable component
// returns -- never a numeric placeholder, so a caller can never mistake "not available"
// for a real, low, computed score.
const InsufficientData = "Insufficient Data"

// Component is a fixed, named confidence dimension.
type Component string

const (
	ComponentSourceAgreement      Component = "source_agreement"
	ComponentVerificationEvidence Component = "verification_evidence"
	ComponentRepositoryOwnership  Component = "repository_ownership"
	ComponentConnectorAgreement   Component = "connector_agreement"
	ComponentFreshness            Component = "freshness"
	ComponentPackageAuthenticity  Component = "package_authenticity"
	ComponentInvocationHistory    Component = "invocation_history"
)

// ComponentScore is one real dimension's result -- either a real, computed value with its
// explanation, or an honest Available=false with a stated reason why no real data exists
// for it yet. Score is only meaningful when Available is true.
type ComponentScore struct {
	Component   Component `json:"component"`
	Available   bool      `json:"available"`
	Score       float64   `json:"score,omitempty"`
	Explanation string    `json:"explanation"`
}

// Confidence is the full, transparent breakdown -- every component present, whether
// available or not, plus an Overall figure computed only from the components that
// actually have real data.
type Confidence struct {
	Components []ComponentScore `json:"components"`
	Overall    ComponentScore   `json:"overall"`
}

// Compute produces a real, explainable confidence breakdown for a resolved entity,
// cross-referencing its Sources back to the original discovery.Result data (for fields
// like Verified/LastUpdated that Entity itself doesn't carry directly).
func Compute(e entity.Entity, allResults []discovery.Result) Confidence {
	matching := matchingResults(e, allResults)

	components := []ComponentScore{
		sourceAgreement(e),
		verificationEvidence(matching),
		repositoryOwnership(e),
		connectorAgreement(e),
		freshness(matching),
		packageAuthenticity(),
		invocationHistory(),
	}

	return Confidence{
		Components: components,
		Overall:    overallScore(components),
	}
}

// matchingResults finds the original discovery.Result entries that contributed to this
// entity, by the same (Source, Name, URL) identity entity.SourceRef already tracks.
func matchingResults(e entity.Entity, allResults []discovery.Result) []discovery.Result {
	want := map[string]bool{}
	for _, s := range e.Sources {
		want[s.Source+"|"+s.Name+"|"+s.URL] = true
	}
	var out []discovery.Result
	for _, r := range allResults {
		if want[r.Source+"|"+r.Name+"|"+r.URL] {
			out = append(out, r)
		}
	}
	return out
}

// sourceAgreement: more independent, distinct sources reporting the same real entity is
// genuinely stronger evidence than one source's unconfirmed claim. Always computable --
// every entity has at least one source, itself.
func sourceAgreement(e entity.Entity) ComponentScore {
	distinct := map[string]bool{}
	for _, s := range e.Sources {
		distinct[s.Source] = true
	}
	n := len(distinct)
	var score float64
	switch {
	case n >= 4:
		score = 100
	case n == 3:
		score = 90
	case n == 2:
		score = 75
	default:
		score = 50
	}
	explanation := "reported consistently across multiple independent sources"
	if n == 1 {
		explanation = "reported by 1 source, not independently corroborated elsewhere yet"
	}
	return ComponentScore{Component: ComponentSourceAgreement, Available: true, Score: score, Explanation: explanation}
}

// verificationEvidence: real, always-computable binary check -- was any contributing
// result cryptographically verified (discovery.Result.Verified)?
func verificationEvidence(matching []discovery.Result) ComponentScore {
	for _, r := range matching {
		if r.Verified {
			return ComponentScore{
				Component: ComponentVerificationEvidence, Available: true, Score: 100,
				Explanation: "cryptographically verified by at least one contributing source",
			}
		}
	}
	return ComponentScore{
		Component: ComponentVerificationEvidence, Available: true, Score: 30,
		Explanation: "no cryptographic verification found -- not necessarily untrustworthy, just no extra assurance available",
	}
}

// repositoryOwnership: Available only when a real organization was mechanically
// extractable from at least one source's URL/scoped-name -- genuinely "Insufficient Data"
// when no source carries a traceable org signal at all. Score reflects how many of the
// individual sources independently agree on the same org, not just whether one had one.
func repositoryOwnership(e entity.Entity) ComponentScore {
	if e.Organization == "" {
		return ComponentScore{
			Component: ComponentRepositoryOwnership, Available: false,
			Explanation: "no source carried a mechanically traceable organization signal (no GitHub-style URL or npm scope)",
		}
	}
	agreeing, total := 0, 0
	for _, s := range e.Sources {
		org := entity.ExtractOrganization(s.Name, s.URL)
		if org == "" {
			continue
		}
		total++
		if org == e.Organization {
			agreeing++
		}
	}
	score := 100.0
	if total > 1 {
		score = 60 + 40*(float64(agreeing)/float64(total))
	}
	return ComponentScore{
		Component: ComponentRepositoryOwnership, Available: true, Score: score,
		Explanation: "organization '" + e.Organization + "' mechanically extracted from a real source URL/scope",
	}
}

// reliabilityLookup defaults to the real discovery.ReliabilityForSource -- overridable in
// tests (same pattern used for file-path overrides elsewhere in this codebase) so
// connectorAgreement's own averaging/availability logic can be tested deterministically,
// without depending on discovery's internal, package-private test state or making a real
// network call just to populate reliability data.
var reliabilityLookup = discovery.ReliabilityForSource

// connectorAgreement: real, locally-observed reliability (discovery.ReliabilityForSource)
// for each distinct source that contributed -- Insufficient Data only when none of them
// have any real, recorded search history yet on this machine.
func connectorAgreement(e entity.Entity) ComponentScore {
	distinct := map[string]bool{}
	for _, s := range e.Sources {
		distinct[s.Source] = true
	}
	var total float64
	var n int
	for source := range distinct {
		rel := reliabilityLookup(source)
		if rel.HasData {
			total += rel.SuccessRate * 100
			n++
		}
	}
	if n == 0 {
		return ComponentScore{
			Component: ComponentConnectorAgreement, Available: false,
			Explanation: "no contributing connector has any real, recorded search history on this machine yet",
		}
	}
	return ComponentScore{
		Component: ComponentConnectorAgreement, Available: true, Score: total / float64(n),
		Explanation: "based on real, locally-observed success rate of the contributing connector(s)",
	}
}

// freshness: real exponential decay based on the most recent LastUpdated among the
// contributing results -- Insufficient Data when none of them carry a parseable
// timestamp at all (some real sources, like Smithery/Web, don't expose one).
func freshness(matching []discovery.Result) ComponentScore {
	var mostRecent time.Time
	found := false
	for _, r := range matching {
		t, err := time.Parse(time.RFC3339, r.LastUpdated)
		if err != nil {
			continue
		}
		if !found || t.After(mostRecent) {
			mostRecent = t
			found = true
		}
	}
	if !found {
		return ComponentScore{
			Component: ComponentFreshness, Available: false,
			Explanation: "no contributing source exposed a real, parseable last-updated timestamp",
		}
	}
	const halfLifeDays = 180.0
	days := time.Since(mostRecent).Hours() / 24
	if days < 0 {
		days = 0
	}
	decay := math.Pow(0.5, days/halfLifeDays)
	return ComponentScore{
		Component: ComponentFreshness, Available: true, Score: decay * 100,
		Explanation: "based on the most recent real last-updated timestamp among contributing sources",
	}
}

// packageAuthenticity: honestly, always Insufficient Data right now -- no connector in
// this codebase extracts or verifies a real package signature/provenance record (npm
// provenance, PyPI signing, etc.). Returning a plausible-looking number here without that
// real data would be exactly the fabrication this package exists to refuse.
func packageAuthenticity() ComponentScore {
	return ComponentScore{
		Component: ComponentPackageAuthenticity, Available: false,
		Explanation: "no connector currently extracts real package signature/provenance data -- would need real signing/provenance support added to a connector first",
	}
}

// invocationHistory: honestly, always Insufficient Data right now -- this CLI has no
// local or accessible record of how many times a specific discovered entity has actually
// been invoked (that would be cross-user, backend data this tool doesn't have access to).
func invocationHistory() ComponentScore {
	return ComponentScore{
		Component: ComponentInvocationHistory, Available: false,
		Explanation: "no accessible record of real invocation history for this entity exists from the CLI's perspective",
	}
}

// overallScore averages only the components that are genuinely Available -- an
// unavailable component is never treated as a zero. If nothing at all is available,
// Overall is itself honestly Insufficient Data.
// RecomputeOverall re-averages a set of components -- exported so a caller that swaps in
// an enriched component (e.g. a real package-authenticity check) after Compute() can
// honestly refresh Overall too, rather than displaying a stale average that still
// excludes data that's now genuinely available.
func RecomputeOverall(components []ComponentScore) ComponentScore {
	return overallScore(components)
}

func overallScore(components []ComponentScore) ComponentScore {
	var total float64
	var n int
	for _, c := range components {
		if c.Available {
			total += c.Score
			n++
		}
	}
	if n == 0 {
		return ComponentScore{Component: "overall", Available: false, Explanation: InsufficientData}
	}
	return ComponentScore{
		Component: "overall", Available: true, Score: total / float64(n),
		Explanation: "average of the currently-available real components only",
	}
}
