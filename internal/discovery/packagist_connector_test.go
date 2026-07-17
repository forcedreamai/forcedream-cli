package discovery

import (
	"context"
	"testing"
)

var _ Connector = PackagistConnector{}

func TestPackagistConnectorCapabilities(t *testing.T) {
	c := PackagistConnector{}
	if c.Name() != "Packagist" {
		t.Fatalf("expected Name() to be 'Packagist', got %q", c.Name())
	}
	caps := c.Capabilities()
	if caps.RequiresPayment || caps.RequiresAPIKey {
		t.Fatal("Packagist's public search API is real, free, and needs no credential")
	}
}

func TestPackagistConnectorSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Packagist network test in -short mode")
	}
	resetStatsForTest(t)
	resetCacheForTest(t)
	c := PackagistConnector{}
	outcome, err := c.Search(context.Background(), "monolog", 5)
	if err != nil {
		t.Fatalf("real Packagist connector search failed: %v", err)
	}
	if !outcome.Available {
		t.Fatalf("expected Available=true, got Reason=%q Message=%q", outcome.Reason, outcome.Message)
	}
	if len(outcome.Results) == 0 {
		t.Fatal("expected at least one real result")
	}
}

func TestPackagistConnectorHealthLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Packagist network test in -short mode")
	}
	c := PackagistConnector{}
	health := c.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected Packagist to be genuinely reachable right now, got: %s", health.Message)
	}
}

func TestPackagistConnectorLatencyAndReliabilityReadPath(t *testing.T) {
	resetStatsForTest(t)
	c := PackagistConnector{}
	if before := c.Latency(); before.HasData {
		t.Fatalf("expected no latency data before any recorded outcome, got %+v", before)
	}
	recordSearchOutcome(c.Name(), true, 190)
	latency := c.Latency()
	if !latency.HasData || latency.AverageMs != 190 {
		t.Fatalf("expected real latency data of 190ms, got %+v", latency)
	}
}
