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
