package reputation

import (
	"path/filepath"
	"testing"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
	"github.com/forcedreamai/forcedream-cli/internal/entity"
	"github.com/forcedreamai/forcedream-cli/internal/graph"
)

func resetGraphForTest(t *testing.T) {
	t.Helper()
	graph.SetStoragePathForTesting(filepath.Join(t.TempDir(), "graph_test.json"))
	t.Cleanup(func() { graph.SetStoragePathForTesting("") })
}

func TestDeveloperReputationInsufficientDataWithNoPublishedEntities(t *testing.T) {
	resetGraphForTest(t)
	rep := DeveloperReputation("never-seen-before-org")
	published := findComponent(rep, "published_entities")
	if published.Available {
		t.Fatal("expected Available=false for an organization with no discovered entities on this machine")
	}
}

func TestDeveloperReputationAvailableWithRealPublishedEntities(t *testing.T) {
	resetGraphForTest(t)
	entities := []entity.Entity{
		{ID: "ent_a", CanonicalName: "Agent A", Organization: "real-org"},
		{ID: "ent_b", CanonicalName: "Agent B", Organization: "real-org"},
	}
	graph.Merge(graph.BuildFromEntities(entities))

	rep := DeveloperReputation("real-org")
	published := findComponent(rep, "published_entities")
	if !published.Available {
		t.Fatal("expected Available=true with real, discovered published entities")
	}
}

func TestDeveloperReputationNeverInventsUnavailableDimensions(t *testing.T) {
	resetGraphForTest(t)
	rep := DeveloperReputation("some-org")
	for _, dim := range []string{"successful_invocations", "sdk_quality", "release_cadence", "support_responsiveness", "security_incidents"} {
		c := findComponent(rep, dim)
		if c.Available {
			t.Fatalf("expected %q to be honestly Insufficient Data -- no real data source exists for it, got Available=true", dim)
		}
	}
}

func TestAgentReputationRewardsRealPopularityAndVerification(t *testing.T) {
	stars := 5000
	popular := []discovery.Result{{Stars: &stars, Verified: true}}
	rep := AgentReputation(entity.Entity{CanonicalName: "Popular Agent"}, popular)

	pop := findComponent(rep, "popularity")
	if !pop.Available || pop.Score < 80 {
		t.Fatalf("expected a high, real popularity score for 5000 stars, got %+v", pop)
	}
	ver := findComponent(rep, "verification")
	if !ver.Available || ver.Score != 100 {
		t.Fatalf("expected verification score 100 for a real, verified result, got %+v", ver)
	}
}

func TestAgentReputationNeverInventsUnavailableDimensions(t *testing.T) {
	rep := AgentReputation(entity.Entity{}, nil)
	for _, dim := range []string{"invocation_success_rate", "failure_rate", "security_incidents"} {
		c := findComponent(rep, dim)
		if c.Available {
			t.Fatalf("expected %q to be honestly Insufficient Data, got Available=true", dim)
		}
	}
}

func TestConnectorReputationUsesRealReliabilityData(t *testing.T) {
	// Uses discovery.ReliabilityForSource directly (real behavior) rather than a fake --
	// a genuinely never-used-before connector name reliably has no real history yet.
	rep := ConnectorReputation("a-connector-name-that-has-never-been-used-in-any-test")
	rel := findComponent(rep, "reliability")
	if rel.Available {
		t.Fatal("expected Available=false for a connector with no real, recorded history")
	}
}

func TestPackageReputationNeverInventsSecurityData(t *testing.T) {
	rep := PackageReputation(nil)
	for _, dim := range []string{"security_advisories", "maintenance_activity"} {
		c := findComponent(rep, dim)
		if c.Available {
			t.Fatalf("expected %q to be honestly Insufficient Data, got Available=true", dim)
		}
	}
}

func TestFourReputationTypesAreGenuinelySeparateSubjects(t *testing.T) {
	// The explicit, permanent rule: these must never be collapsed into one score. This
	// test simply confirms each returns its own, independently-labeled Subject and
	// Components -- not a shared or merged structure.
	resetGraphForTest(t)
	dev := DeveloperReputation("org-x")
	agent := AgentReputation(entity.Entity{CanonicalName: "agent-x"}, nil)
	conn := ConnectorReputation("connector-x")
	pkg := PackageReputation([]discovery.Result{{Name: "package-x"}})

	subjects := map[string]bool{dev.Subject: true, agent.Subject: true, conn.Subject: true, pkg.Subject: true}
	if len(subjects) != 4 {
		t.Fatalf("expected 4 genuinely distinct subjects, got %d: %v", len(subjects), subjects)
	}
}

func findComponent(rep Reputation, dimension string) ComponentScore {
	for _, c := range rep.Components {
		if c.Dimension == dimension {
			return c
		}
	}
	return ComponentScore{}
}
