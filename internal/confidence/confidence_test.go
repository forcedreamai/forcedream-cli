package confidence

import (
	"testing"
	"time"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
	"github.com/forcedreamai/forcedream-cli/internal/entity"
)

func TestSourceAgreementScalesWithDistinctSources(t *testing.T) {
	one := entity.Entity{Sources: []entity.SourceRef{{Source: "GitHub"}}}
	two := entity.Entity{Sources: []entity.SourceRef{{Source: "GitHub"}, {Source: "npm"}}}
	three := entity.Entity{Sources: []entity.SourceRef{{Source: "GitHub"}, {Source: "npm"}, {Source: "Smithery"}}}

	s1, s2, s3 := sourceAgreement(one), sourceAgreement(two), sourceAgreement(three)
	if !(s1.Score < s2.Score && s2.Score < s3.Score) {
		t.Fatalf("expected score to strictly increase with distinct source count, got %f, %f, %f", s1.Score, s2.Score, s3.Score)
	}
	if !s1.Available {
		t.Fatal("source agreement must always be Available -- every entity has at least one source")
	}
}

func TestVerificationEvidenceRewardsRealVerification(t *testing.T) {
	verified := []discovery.Result{{Verified: true}}
	unverified := []discovery.Result{{Verified: false}}
	vs, us := verificationEvidence(verified), verificationEvidence(unverified)
	if vs.Score <= us.Score {
		t.Fatalf("expected a verified result to score higher, got verified=%f unverified=%f", vs.Score, us.Score)
	}
	if !vs.Available || !us.Available {
		t.Fatal("verification evidence must always be Available -- it's a real, always-answerable binary check")
	}
}

func TestRepositoryOwnershipInsufficientDataWhenNoOrgSignal(t *testing.T) {
	e := entity.Entity{Organization: ""}
	score := repositoryOwnership(e)
	if score.Available {
		t.Fatal("expected Available=false when no real organization was ever extracted")
	}
	if score.Explanation == "" {
		t.Fatal("expected a real, stated reason for the Insufficient Data result")
	}
}

func TestRepositoryOwnershipAvailableWhenOrgKnown(t *testing.T) {
	e := entity.Entity{
		Organization: "forcedreamai",
		Sources: []entity.SourceRef{
			{Name: "forcedream-mcp", URL: "https://github.com/forcedreamai/forcedream-mcp"},
		},
	}
	score := repositoryOwnership(e)
	if !score.Available {
		t.Fatal("expected Available=true when a real organization was mechanically extracted")
	}
	if score.Score != 100 {
		t.Fatalf("expected score 100 for a single, agreeing source, got %f", score.Score)
	}
}

func TestConnectorAgreementInsufficientDataWithNoRealHistory(t *testing.T) {
	old := reliabilityLookup
	defer func() { reliabilityLookup = old }()
	reliabilityLookup = func(source string) discovery.ReliabilityInfo {
		return discovery.ReliabilityInfo{HasData: false}
	}

	e := entity.Entity{Sources: []entity.SourceRef{{Source: "SomeConnector"}}}
	score := connectorAgreement(e)
	if score.Available {
		t.Fatal("expected Available=false when the fake lookup reports no real data")
	}
}

func TestConnectorAgreementAveragesRealReliability(t *testing.T) {
	old := reliabilityLookup
	defer func() { reliabilityLookup = old }()
	reliabilityLookup = func(source string) discovery.ReliabilityInfo {
		if source == "A" {
			return discovery.ReliabilityInfo{HasData: true, SuccessRate: 1.0}
		}
		return discovery.ReliabilityInfo{HasData: true, SuccessRate: 0.5}
	}

	e := entity.Entity{Sources: []entity.SourceRef{{Source: "A"}, {Source: "B"}}}
	score := connectorAgreement(e)
	if !score.Available {
		t.Fatal("expected Available=true when the fake lookup reports real data")
	}
	expected := (100.0 + 50.0) / 2
	if score.Score != expected {
		t.Fatalf("expected the real average (%f), got %f", expected, score.Score)
	}
}

func TestFreshnessInsufficientDataWithNoParsableTimestamp(t *testing.T) {
	results := []discovery.Result{{LastUpdated: ""}, {LastUpdated: "not-a-real-date"}}
	score := freshness(results)
	if score.Available {
		t.Fatal("expected Available=false when no contributing result has a real, parseable timestamp")
	}
}

func TestFreshnessRewardsRecentTimestamp(t *testing.T) {
	recent := []discovery.Result{{LastUpdated: time.Now().Format(time.RFC3339)}}
	stale := []discovery.Result{{LastUpdated: time.Now().AddDate(-2, 0, 0).Format(time.RFC3339)}}
	rs, ss := freshness(recent), freshness(stale)
	if !rs.Available || !ss.Available {
		t.Fatal("expected both to be Available with real, parseable timestamps")
	}
	if rs.Score <= ss.Score {
		t.Fatalf("expected the recent result to score higher, got recent=%f stale=%f", rs.Score, ss.Score)
	}
}

func TestPackageAuthenticityIsHonestlyAlwaysInsufficientDataRightNow(t *testing.T) {
	score := packageAuthenticity()
	if score.Available {
		t.Fatal("expected Available=false -- no connector currently extracts real package signature data")
	}
}

func TestInvocationHistoryIsHonestlyAlwaysInsufficientDataRightNow(t *testing.T) {
	score := invocationHistory()
	if score.Available {
		t.Fatal("expected Available=false -- the CLI has no accessible real invocation history")
	}
}

func TestOverallAveragesOnlyAvailableComponents(t *testing.T) {
	components := []ComponentScore{
		{Component: "a", Available: true, Score: 80},
		{Component: "b", Available: true, Score: 60},
		{Component: "c", Available: false}, // must not count as 0
	}
	o := overallScore(components)
	if !o.Available {
		t.Fatal("expected Overall to be Available when at least one component is")
	}
	if o.Score != 70 {
		t.Fatalf("expected the average of only the 2 available components (70), got %f -- an unavailable component must never be treated as a zero", o.Score)
	}
}

func TestOverallIsInsufficientDataWhenNothingIsAvailable(t *testing.T) {
	components := []ComponentScore{
		{Component: "a", Available: false},
		{Component: "b", Available: false},
	}
	o := overallScore(components)
	if o.Available {
		t.Fatal("expected Overall to honestly be Available=false when nothing at all could be computed")
	}
}

func TestComputeReturnsAllSevenComponents(t *testing.T) {
	e := entity.Entity{
		Organization: "someorg",
		Sources:      []entity.SourceRef{{Source: "GitHub", Name: "n", URL: "https://github.com/someorg/n"}},
	}
	c := Compute(e, []discovery.Result{{Source: "GitHub", Name: "n", URL: "https://github.com/someorg/n", Verified: true}})
	if len(c.Components) != 7 {
		t.Fatalf("expected all 7 real dimensions present in the breakdown (available or not), got %d", len(c.Components))
	}
}
