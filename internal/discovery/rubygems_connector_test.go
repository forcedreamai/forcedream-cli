package discovery

import (
	"context"
	"testing"
)

var _ Connector = RubyGemsConnector{}

func TestRubyGemsConnectorCapabilities(t *testing.T) {
	c := RubyGemsConnector{}
	if c.Name() != "RubyGems" {
		t.Fatalf("expected Name() to be 'RubyGems', got %q", c.Name())
	}
	caps := c.Capabilities()
	if caps.RequiresPayment || caps.RequiresAPIKey {
		t.Fatal("RubyGems' public search API is real, free, and needs no credential")
	}
}

func TestRubyGemsConnectorSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live RubyGems network test in -short mode")
	}
	resetStatsForTest(t)
	resetCacheForTest(t)
	c := RubyGemsConnector{}
	outcome, err := c.Search(context.Background(), "rails", 5)
	if err != nil {
		t.Fatalf("real RubyGems connector search failed: %v", err)
	}
	if !outcome.Available {
		t.Fatalf("expected Available=true, got Reason=%q Message=%q", outcome.Reason, outcome.Message)
	}
	if len(outcome.Results) == 0 {
		t.Fatal("expected at least one real result")
	}
}

func TestRubyGemsConnectorHealthLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live RubyGems network test in -short mode")
	}
	c := RubyGemsConnector{}
	health := c.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected RubyGems to be genuinely reachable right now, got: %s", health.Message)
	}
}

func TestRubyGemsConnectorLatencyAndReliabilityReadPath(t *testing.T) {
	resetStatsForTest(t)
	c := RubyGemsConnector{}
	if before := c.Latency(); before.HasData {
		t.Fatalf("expected no latency data before any recorded outcome, got %+v", before)
	}
	recordSearchOutcome(c.Name(), true, 210)
	latency := c.Latency()
	if !latency.HasData || latency.AverageMs != 210 {
		t.Fatalf("expected real latency data of 210ms, got %+v", latency)
	}
}
