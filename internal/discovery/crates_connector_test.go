package discovery

import (
	"context"
	"testing"
)

var _ Connector = CratesIOConnector{}

func TestCratesIOConnectorCapabilities(t *testing.T) {
	c := CratesIOConnector{}
	if c.Name() != "crates.io" {
		t.Fatalf("expected Name() to be 'crates.io', got %q", c.Name())
	}
	caps := c.Capabilities()
	if caps.RequiresPayment {
		t.Fatal("crates.io is a real, free source -- RequiresPayment should be false")
	}
	if caps.RequiresAPIKey {
		t.Fatal("crates.io's public search API needs no credential -- RequiresAPIKey should be false")
	}
}

func TestCratesIOConnectorSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live crates.io network test in -short mode")
	}
	resetStatsForTest(t)
	resetCacheForTest(t)
	c := CratesIOConnector{}
	outcome, err := c.Search(context.Background(), "serde", 5)
	if err != nil {
		t.Fatalf("real crates.io connector search failed: %v", err)
	}
	if !outcome.Available {
		t.Fatalf("expected Available=true for a successful real search, got Reason=%q Message=%q", outcome.Reason, outcome.Message)
	}
	if len(outcome.Results) == 0 {
		t.Fatal("expected at least one real result")
	}
}

func TestCratesIOConnectorHealthLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live crates.io network test in -short mode")
	}
	c := CratesIOConnector{}
	health := c.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected crates.io to be genuinely reachable right now, got: %s", health.Message)
	}
}

func TestCratesIOConnectorLatencyAndReliabilityReadPath(t *testing.T) {
	resetStatsForTest(t)
	c := CratesIOConnector{}

	if before := c.Latency(); before.HasData {
		t.Fatalf("expected no latency data before any recorded outcome, got %+v", before)
	}
	recordSearchOutcome(c.Name(), true, 200)

	latency := c.Latency()
	if !latency.HasData || latency.AverageMs != 200 {
		t.Fatalf("expected real latency data of 200ms, got %+v", latency)
	}
	reliability := c.Reliability()
	if !reliability.HasData || reliability.SuccessRate != 1.0 {
		t.Fatalf("expected real 100%% reliability data, got %+v", reliability)
	}
}
