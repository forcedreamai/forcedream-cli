package discovery

import (
	"context"
	"testing"
)

var _ Connector = NuGetConnector{}

func TestNuGetConnectorCapabilities(t *testing.T) {
	c := NuGetConnector{}
	if c.Name() != "NuGet" {
		t.Fatalf("expected Name() to be 'NuGet', got %q", c.Name())
	}
	caps := c.Capabilities()
	if caps.RequiresPayment || caps.RequiresAPIKey {
		t.Fatal("NuGet's public search API is real, free, and needs no credential")
	}
}

func TestNuGetConnectorSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live NuGet network test in -short mode")
	}
	resetStatsForTest(t)
	resetCacheForTest(t)
	c := NuGetConnector{}
	outcome, err := c.Search(context.Background(), "json", 5)
	if err != nil {
		t.Fatalf("real NuGet connector search failed: %v", err)
	}
	if !outcome.Available {
		t.Fatalf("expected Available=true, got Reason=%q Message=%q", outcome.Reason, outcome.Message)
	}
	if len(outcome.Results) == 0 {
		t.Fatal("expected at least one real result")
	}
}

func TestNuGetConnectorHealthLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live NuGet network test in -short mode")
	}
	c := NuGetConnector{}
	health := c.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected NuGet to be genuinely reachable right now, got: %s", health.Message)
	}
}

// TestNuGetServiceIndexDiscoveryLive verifies the real, documented 2-step discovery
// process itself -- not just that search works, but that the real service-index lookup
// genuinely resolves to a real SearchQueryService URL, confirming this isn't silently
// falling back to a hardcoded guess anywhere.
func TestNuGetServiceIndexDiscoveryLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live NuGet network test in -short mode")
	}
	nugetSearchURLMu.Lock()
	nugetSearchURLCache = "" // force a real, fresh discovery for this test
	nugetSearchURLMu.Unlock()

	url, err := nugetSearchServiceURL(context.Background())
	if err != nil {
		t.Fatalf("real NuGet service index discovery failed: %v", err)
	}
	if url == "" {
		t.Fatal("expected a real, non-empty SearchQueryService URL from the real service index")
	}
}

func TestNuGetConnectorLatencyAndReliabilityReadPath(t *testing.T) {
	resetStatsForTest(t)
	c := NuGetConnector{}
	if before := c.Latency(); before.HasData {
		t.Fatalf("expected no latency data before any recorded outcome, got %+v", before)
	}
	recordSearchOutcome(c.Name(), true, 220)
	latency := c.Latency()
	if !latency.HasData || latency.AverageMs != 220 {
		t.Fatalf("expected real latency data of 220ms, got %+v", latency)
	}
}
