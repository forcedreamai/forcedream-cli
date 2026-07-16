package discovery

import (
	"context"
	"testing"
)

// A real, live network test (not typically how Go tests are structured, but this is a
// deliberate, one-off verification against the real npm API before trusting this new
// source -- matching the same live-verification discipline used for every other real
// integration tonight, not just a mocked/offline test). Skippable via `go test -short`
// for CI/offline contexts, since it depends on a real, live third-party API being
// reachable -- run it explicitly (without -short) to actually exercise the real network
// call.
func TestSearchNpmMCPServersLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live npm network test in -short mode")
	}
	results, err := SearchNpmMCPServers(context.Background(), "translation", 5)
	if err != nil {
		t.Fatalf("real npm search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one real result from npm for 'translation mcp'")
	}
	for _, r := range results {
		t.Logf("- %s (useCount=%v, updated=%s): %s", r.Name, r.UseCount, r.LastUpdated, r.Description)
		if r.Source != "npm" {
			t.Errorf("expected Source to be 'npm', got %q", r.Source)
		}
		if r.URL == "" {
			t.Errorf("expected a real URL for %s, got empty", r.Name)
		}
	}
}
