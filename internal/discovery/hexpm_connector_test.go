package discovery

import (
	"context"
	"testing"
)

var _ Connector = HexPMConnector{}

func TestHexPMConnectorCapabilities(t *testing.T) {
	c := HexPMConnector{}
	if c.Name() != "Hex.pm" {
		t.Fatalf("expected Name() to be 'Hex.pm', got %q", c.Name())
	}
	caps := c.Capabilities()
	if caps.RequiresPayment || caps.RequiresAPIKey {
		t.Fatal("Hex.pm's public search API is real, free, and needs no credential")
	}
}

func TestHexPMConnectorSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Hex.pm network test in -short mode")
	}
	resetStatsForTest(t)
	resetCacheForTest(t)
	c := HexPMConnector{}
	outcome, err := c.Search(context.Background(), "phoenix", 5)
	if err != nil {
		t.Fatalf("real Hex.pm connector search failed: %v", err)
	}
	if !outcome.Available {
		t.Fatalf("expected Available=true, got Reason=%q Message=%q", outcome.Reason, outcome.Message)
	}
	if len(outcome.Results) == 0 {
		t.Fatal("expected at least one real result")
	}
}

func TestHexPMConnectorHealthLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Hex.pm network test in -short mode")
	}
	c := HexPMConnector{}
	health := c.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected Hex.pm to be genuinely reachable right now, got: %s", health.Message)
	}
}

func TestHexPMConnectorLatencyAndReliabilityReadPath(t *testing.T) {
	resetStatsForTest(t)
	c := HexPMConnector{}
	if before := c.Latency(); before.HasData {
		t.Fatalf("expected no latency data before any recorded outcome, got %+v", before)
	}
	recordSearchOutcome(c.Name(), true, 170)
	latency := c.Latency()
	if !latency.HasData || latency.AverageMs != 170 {
		t.Fatalf("expected real latency data of 170ms, got %+v", latency)
	}
}
