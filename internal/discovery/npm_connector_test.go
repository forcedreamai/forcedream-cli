package discovery

import (
	"context"
	"testing"
)

// Compile-time check: NpmConnector genuinely satisfies Connector. If this framework's
// interface design were ever wrong for this real, already-proven source, this line would
// fail to compile -- a stronger, earlier guarantee than any runtime assertion could give.
var _ Connector = NpmConnector{}

func TestNpmConnectorCapabilities(t *testing.T) {
	c := NpmConnector{}
	if c.Name() != "npm" {
		t.Fatalf("expected Name() to be 'npm', got %q", c.Name())
	}
	caps := c.Capabilities()
	if caps.RequiresPayment {
		t.Fatal("npm is a real, free source -- RequiresPayment should be false")
	}
	if caps.RequiresAPIKey {
		t.Fatal("npm's public search API needs no credential -- RequiresAPIKey should be false")
	}
}

func TestNpmConnectorSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live npm network test in -short mode")
	}
	resetStatsForTest(t)
	resetCacheForTest(t)
	c := NpmConnector{}
	outcome, err := c.Search(context.Background(), "translation", 5)
	if err != nil {
		t.Fatalf("real npm connector search failed: %v", err)
	}
	if !outcome.Available {
		t.Fatalf("expected Available=true for a successful real search, got Reason=%q Message=%q", outcome.Reason, outcome.Message)
	}
	if len(outcome.Results) == 0 {
		t.Fatal("expected at least one real result")
	}
}

func TestNpmConnectorHealthLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live npm network test in -short mode")
	}
	c := NpmConnector{}
	health := c.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected npm to be genuinely reachable right now, got: %s", health.Message)
	}
}

// TestNpmConnectorLatencyAndReliabilityReadPath verifies Latency()/Reliability() correctly
// read real, accumulated stats -- using the same, fully-isolated, no-real-network-call
// pattern stats_test.go's own tests already use (recordSearchOutcome called directly),
// rather than a real c.Search() call. This connector's Search() method calling
// recordSearchOutcome on every real attempt is already, separately confirmed by real CLI
// runs (connector_stats.json genuinely accumulating real search history) -- this test
// verifies the read side (Latency/Reliability correctly reporting what's been recorded)
// in isolation, without depending on a real network call succeeding during `go test`.
func TestNpmConnectorLatencyAndReliabilityReadPath(t *testing.T) {
	resetStatsForTest(t)
	c := NpmConnector{}

	if before := c.Latency(); before.HasData {
		t.Fatalf("expected no latency data before any recorded outcome, got %+v", before)
	}
	if before := c.Reliability(); before.HasData {
		t.Fatalf("expected no reliability data before any recorded outcome, got %+v", before)
	}

	recordSearchOutcome(c.Name(), true, 150)

	latency := c.Latency()
	if !latency.HasData {
		t.Fatal("expected real latency data to exist after a recorded outcome")
	}
	if latency.AverageMs != 150 {
		t.Fatalf("expected average latency of 150ms, got %d", latency.AverageMs)
	}

	reliability := c.Reliability()
	if !reliability.HasData {
		t.Fatal("expected real reliability data to exist after a recorded outcome")
	}
	if reliability.SuccessRate != 1.0 {
		t.Fatalf("expected 100%% success rate, got %f", reliability.SuccessRate)
	}
}
