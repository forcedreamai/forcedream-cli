package entity

import (
	"path/filepath"
	"testing"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
)

func resetStorageForTest(t *testing.T) {
	t.Helper()
	storageFilePathOverride = filepath.Join(t.TempDir(), "entities_test.json")
	storageCache = nil
	t.Cleanup(func() {
		storageFilePathOverride = ""
		storageCache = nil
	})
}

func TestOverrideMergeForcesUnionBelowThreshold(t *testing.T) {
	resetStorageForTest(t)

	a := discovery.Result{Name: "Totally Different Name A", Source: "GitHub", URL: "https://github.com/org-a/repo-a"}
	b := discovery.Result{Name: "Totally Different Name B", Source: "GitHub", URL: "https://github.com/org-b/repo-b"}

	// No real automatic signal connects these -- confirm they'd stay separate without an
	// override before testing that the override actually changes that.
	without := Resolve([]discovery.Result{a, b}, DefaultMergeThreshold)
	if len(without) != 2 {
		t.Fatalf("expected no automatic match between genuinely unrelated results, got %d entities", len(without))
	}

	AddOverride(a, b, OverrideMerge)
	withOverride := ResolveWithOverrides([]discovery.Result{a, b}, DefaultMergeThreshold)
	if len(withOverride) != 1 {
		t.Fatalf("expected a manual merge override to force a union regardless of automatic score, got %d entities", len(withOverride))
	}
}

func TestOverrideSplitPreventsAutomaticMerge(t *testing.T) {
	resetStorageForTest(t)

	a := discovery.Result{Name: "forcedream-mcp", Source: "GitHub", URL: "https://github.com/forcedreamai/forcedream-mcp"}
	b := discovery.Result{Name: "forcedream-mcp", Source: "MCP Registry", URL: "https://github.com/forcedreamai/forcedream-mcp"}

	// Confirm these WOULD automatically merge (exact URL match) before testing the split
	// override actually prevents it.
	without := Resolve([]discovery.Result{a, b}, DefaultMergeThreshold)
	if len(without) != 1 {
		t.Fatalf("expected these to automatically merge (exact URL match) before testing the override, got %d entities", len(without))
	}

	AddOverride(a, b, OverrideSplit)
	withOverride := ResolveWithOverrides([]discovery.Result{a, b}, DefaultMergeThreshold)
	if len(withOverride) != 2 {
		t.Fatalf("expected a manual split override to prevent the merge even with a qualifying automatic score, got %d entities", len(withOverride))
	}
}

func TestResolveWithOverridesPersistsEntities(t *testing.T) {
	resetStorageForTest(t)

	a := discovery.Result{Name: "Some Real Agent", Source: "GitHub", URL: "https://github.com/someone/some-real-agent"}
	entities := ResolveWithOverrides([]discovery.Result{a}, DefaultMergeThreshold)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}

	persisted := AllPersistedEntities()
	if len(persisted) != 1 {
		t.Fatalf("expected the resolved entity to be genuinely persisted, got %d persisted entities", len(persisted))
	}
	if persisted[0].ID != entities[0].ID {
		t.Fatalf("expected the persisted entity's ID to match the resolved one, got %q vs %q", persisted[0].ID, entities[0].ID)
	}
}

func TestReferenceKeyPrefersURLWhenPresent(t *testing.T) {
	a := discovery.Result{Name: "Name One", Source: "GitHub", URL: "https://github.com/org/repo"}
	b := discovery.Result{Name: "Completely Different Display Name", Source: "GitHub", URL: "github.com/org/repo/"} // same real URL, different casing/slash
	if referenceKey(a) != referenceKey(b) {
		t.Fatalf("expected the same real, normalized URL to produce the same reference key regardless of display name, got %q vs %q", referenceKey(a), referenceKey(b))
	}
}

func TestReferenceKeyFallsBackToNameWhenNoURL(t *testing.T) {
	a := discovery.Result{Name: "Translation Bridge", Source: "Smithery", URL: ""}
	key := referenceKey(a)
	if key == "" {
		t.Fatal("expected a real, non-empty reference key even with no URL")
	}
}
