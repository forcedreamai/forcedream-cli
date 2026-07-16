package discovery

import (
	"context"
	"testing"
)

// fakeConnector is a real, in-memory Connector implementation used only to test
// SearchManager's own orchestration logic (concurrency, aggregation, mixed
// available/unavailable outcomes) in isolation from any real network dependency -- the
// live connector tests above already cover the real, network-backed behavior.
type fakeConnector struct {
	name    string
	outcome Outcome
	err     error
}

func (f fakeConnector) Name() string { return f.name }
func (f fakeConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return f.outcome, f.err
}
func (f fakeConnector) Health(ctx context.Context) HealthStatus {
	return HealthStatus{Healthy: f.err == nil}
}
func (f fakeConnector) Latency() LatencyInfo             { return LatencyInfo{} }
func (f fakeConnector) Reliability() ReliabilityInfo     { return ReliabilityInfo{} }
func (f fakeConnector) Capabilities() Capabilities       { return Capabilities{} }

var _ Connector = fakeConnector{}

func TestSearchManagerAggregatesAllConnectors(t *testing.T) {
	mgr := NewSearchManager(
		fakeConnector{name: "A", outcome: Outcome{Available: true, Results: []Result{{Name: "a1"}, {Name: "a2"}}}},
		fakeConnector{name: "B", outcome: Outcome{Available: true, Results: []Result{{Name: "b1"}}}},
	)
	managed := mgr.SearchAll(context.Background(), "query", 10)
	if len(managed) != 2 {
		t.Fatalf("expected 2 managed results (one per connector), got %d", len(managed))
	}

	all := AllResults(managed)
	if len(all) != 3 {
		t.Fatalf("expected 3 total results across both connectors, got %d", len(all))
	}
}

func TestSearchManagerExcludesUnavailableConnectorsFromResults(t *testing.T) {
	mgr := NewSearchManager(
		fakeConnector{name: "Working", outcome: Outcome{Available: true, Results: []Result{{Name: "real"}}}},
		fakeConnector{name: "Gated", outcome: Outcome{Available: false, Reason: "auth_required", Message: "needs a key"}},
	)
	managed := mgr.SearchAll(context.Background(), "query", 10)

	all := AllResults(managed)
	if len(all) != 1 {
		t.Fatalf("expected only the working connector's real result, got %d results", len(all))
	}

	// Confirm the gated connector's real reason is still present in the managed results
	// (not silently dropped) even though its results are correctly excluded from AllResults.
	found := false
	for _, m := range managed {
		if m.Name == "Gated" {
			found = true
			if m.Outcome.Reason != "auth_required" {
				t.Errorf("expected the gated connector's real reason to be preserved, got %q", m.Outcome.Reason)
			}
		}
	}
	if !found {
		t.Fatal("expected the gated connector to still appear in managed results, just excluded from AllResults")
	}
}

func TestSearchManagerHandlesZeroConnectors(t *testing.T) {
	mgr := NewSearchManager()
	managed := mgr.SearchAll(context.Background(), "query", 10)
	if len(managed) != 0 {
		t.Fatalf("expected 0 managed results for a manager with no connectors, got %d", len(managed))
	}
}
