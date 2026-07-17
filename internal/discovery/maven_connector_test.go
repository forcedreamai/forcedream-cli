package discovery

import (
	"context"
	"testing"
)

var _ Connector = MavenCentralConnector{}

func TestMavenCentralConnectorCapabilities(t *testing.T) {
	c := MavenCentralConnector{}
	if c.Name() != "Maven Central" {
		t.Fatalf("expected Name() to be 'Maven Central', got %q", c.Name())
	}
	caps := c.Capabilities()
	if caps.RequiresPayment || caps.RequiresAPIKey {
		t.Fatal("Maven Central's public search API is real, free, and needs no credential")
	}
}

func TestMavenCentralConnectorSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Maven Central network test in -short mode")
	}
	resetStatsForTest(t)
	resetCacheForTest(t)
	c := MavenCentralConnector{}
	outcome, err := c.Search(context.Background(), "guava", 5)
	if err != nil {
		t.Fatalf("real Maven Central connector search failed: %v", err)
	}
	if !outcome.Available {
		t.Fatalf("expected Available=true, got Reason=%q Message=%q", outcome.Reason, outcome.Message)
	}
	if len(outcome.Results) == 0 {
		t.Fatal("expected at least one real result")
	}
}

func TestMavenCentralConnectorHealthLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Maven Central network test in -short mode")
	}
	c := MavenCentralConnector{}
	health := c.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected Maven Central to be genuinely reachable right now, got: %s", health.Message)
	}
}

func TestMavenCentralConnectorLatencyAndReliabilityReadPath(t *testing.T) {
	resetStatsForTest(t)
	c := MavenCentralConnector{}
	if before := c.Latency(); before.HasData {
		t.Fatalf("expected no latency data before any recorded outcome, got %+v", before)
	}
	recordSearchOutcome(c.Name(), true, 180)
	latency := c.Latency()
	if !latency.HasData || latency.AverageMs != 180 {
		t.Fatalf("expected real latency data of 180ms, got %+v", latency)
	}
}
