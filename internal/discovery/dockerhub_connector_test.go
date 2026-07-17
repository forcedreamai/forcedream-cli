package discovery

import (
	"context"
	"testing"
)

var _ Connector = DockerHubConnector{}

func TestDockerHubConnectorCapabilities(t *testing.T) {
	c := DockerHubConnector{}
	if c.Name() != "Docker Hub" {
		t.Fatalf("expected Name() to be 'Docker Hub', got %q", c.Name())
	}
	caps := c.Capabilities()
	if caps.RequiresPayment || caps.RequiresAPIKey {
		t.Fatal("Docker Hub's real search endpoint is free and needs no credential")
	}
}

func TestDockerHubConnectorSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Docker Hub network test in -short mode")
	}
	resetStatsForTest(t)
	resetCacheForTest(t)
	c := DockerHubConnector{}
	outcome, err := c.Search(context.Background(), "nginx", 5)
	if err != nil {
		t.Fatalf("real Docker Hub connector search failed: %v", err)
	}
	if !outcome.Available {
		t.Fatalf("expected Available=true, got Reason=%q Message=%q", outcome.Reason, outcome.Message)
	}
	if len(outcome.Results) == 0 {
		t.Fatal("expected at least one real result")
	}
}

func TestDockerHubConnectorHealthLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Docker Hub network test in -short mode")
	}
	c := DockerHubConnector{}
	health := c.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected Docker Hub to be genuinely reachable right now, got: %s", health.Message)
	}
}

func TestDockerHubConnectorLatencyAndReliabilityReadPath(t *testing.T) {
	resetStatsForTest(t)
	c := DockerHubConnector{}
	if before := c.Latency(); before.HasData {
		t.Fatalf("expected no latency data before any recorded outcome, got %+v", before)
	}
	recordSearchOutcome(c.Name(), true, 160)
	latency := c.Latency()
	if !latency.HasData || latency.AverageMs != 160 {
		t.Fatalf("expected real latency data of 160ms, got %+v", latency)
	}
}
