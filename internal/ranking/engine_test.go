package ranking

import (
	"testing"
	"time"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
)

func intPtr(v int) *int { return &v }

func TestForceDreamAlwaysRanksFirst(t *testing.T) {
	huge := intPtr(1_000_000)
	results := []discovery.Result{
		{Name: "wildly-popular-repo", Source: "GitHub", Stars: huge, LastUpdated: time.Now().Format(time.RFC3339)},
		{Name: "forcedream-agent", Source: "ForceDream"},
	}
	ranked := Rank(results, DefaultWeights())
	if ranked[0].Source != "ForceDream" {
		t.Fatalf("expected ForceDream first regardless of popularity, got %q first", ranked[0].Source)
	}
}

func TestHigherStarsScoresHigher(t *testing.T) {
	w := DefaultWeights()
	now := time.Now()
	low := discovery.Result{Source: "GitHub", Stars: intPtr(5)}
	high := discovery.Result{Source: "GitHub", Stars: intPtr(5000)}
	if Score(high, w, now) <= Score(low, w, now) {
		t.Fatalf("expected higher-star result to score higher: low=%f high=%f", Score(low, w, now), Score(high, w, now))
	}
}

func TestMoreRecentScoresHigherThanStale(t *testing.T) {
	w := DefaultWeights()
	now := time.Now()
	stars := intPtr(100)
	recent := discovery.Result{Source: "GitHub", Stars: stars, LastUpdated: now.Format(time.RFC3339)}
	stale := discovery.Result{Source: "GitHub", Stars: stars, LastUpdated: now.AddDate(-2, 0, 0).Format(time.RFC3339)}
	if Score(recent, w, now) <= Score(stale, w, now) {
		t.Fatalf("expected the recently-updated result to score higher: recent=%f stale=%f", Score(recent, w, now), Score(stale, w, now))
	}
}

func TestMissingLastUpdatedIsNeutralNotPenalized(t *testing.T) {
	w := DefaultWeights()
	now := time.Now()
	stars := intPtr(100)
	noDate := discovery.Result{Source: "GitHub", Stars: stars, LastUpdated: ""}
	veryStale := discovery.Result{Source: "GitHub", Stars: stars, LastUpdated: now.AddDate(-10, 0, 0).Format(time.RFC3339)}
	if Score(noDate, w, now) <= Score(veryStale, w, now) {
		t.Fatalf("expected missing LastUpdated to score at least as well as a very stale one: noDate=%f veryStale=%f", Score(noDate, w, now), Score(veryStale, w, now))
	}
}

func TestMCPRegistrySourceGetsRealBoost(t *testing.T) {
	w := DefaultWeights()
	now := time.Now()
	stars := intPtr(50)
	github := discovery.Result{Source: "GitHub", Stars: stars}
	mcpReg := discovery.Result{Source: "MCP Registry", UseCount: stars} // same magnitude, different source
	if Score(mcpReg, w, now) <= Score(github, w, now) {
		t.Fatalf("expected MCP Registry provenance to score higher than an equivalent-magnitude GitHub result: github=%f mcpReg=%f", Score(github, w, now), Score(mcpReg, w, now))
	}
}

func TestExplicitMCPTagGetsBoostRegardlessOfSource(t *testing.T) {
	w := DefaultWeights()
	now := time.Now()
	stars := intPtr(50)
	plain := discovery.Result{Source: "GitHub", Stars: stars, Tags: []string{"cli-tool"}}
	tagged := discovery.Result{Source: "GitHub", Stars: stars, Tags: []string{"mcp-server"}}
	if Score(tagged, w, now) <= Score(plain, w, now) {
		t.Fatalf("expected an explicit mcp-server tag to score higher than an otherwise-identical result without it: plain=%f tagged=%f", Score(plain, w, now), Score(tagged, w, now))
	}
}

func TestSourceWeightingDistinguishesEquivalentRawMetrics(t *testing.T) {
	w := DefaultWeights()
	now := time.Now()
	n := intPtr(200)
	github := discovery.Result{Source: "GitHub", Stars: n}
	web := discovery.Result{Source: "Web"} // Web has no popularity metric at all in real data
	if Score(github, w, now) <= Score(web, w, now) {
		t.Fatalf("expected a GitHub result with real stars to outscore a Web result with none: github=%f web=%f", Score(github, w, now), Score(web, w, now))
	}
}

func TestUnknownSourceDefaultsToNeutralWeight(t *testing.T) {
	w := DefaultWeights()
	got := w.SourceWeightFor("SomeBrandNewSourceAddedLater")
	if got != 1.0 {
		t.Fatalf("expected neutral 1.0 weight for an unconfigured source, got %f", got)
	}
}
