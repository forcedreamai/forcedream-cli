package entity

import (
	"testing"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
)

func TestResolveMergesOnRealExactURLMatch(t *testing.T) {
	// GitHub and MCP Registry share the real, exact same URL -- the strongest possible
	// signal, correctly merges at the default threshold.
	results := []discovery.Result{
		{Name: "forcedream-mcp", Source: "GitHub", URL: "https://github.com/forcedreamai/forcedream-mcp"},
		{Name: "forcedream-mcp", Source: "MCP Registry", URL: "https://github.com/forcedreamai/forcedream-mcp"},
	}
	entities := Resolve(results, DefaultMergeThreshold)
	if len(entities) != 1 {
		t.Fatalf("expected an exact URL match to merge at the default threshold, got %d entities", len(entities))
	}
	if entities[0].MatchReasons[0] != ReasonExactURL {
		t.Fatalf("expected exact_url as the match reason, got %v", entities[0].MatchReasons)
	}
}

// TestResolveNameOnlyMatchIsHonestlyCautiousAtDefaultThreshold documents a real, important
// property, not a bug: a Smithery result genuinely carries no URL at all (a known fact
// about that source), so a name-only match against a GitHub/MCP-Registry pair (which do
// have a real, shared URL) has no organization signal to corroborate it. Its score (60,
// exact-name-no-org) is deliberately below the cautious default threshold (70) -- the
// same conservatism that correctly prevents the "mcp-server, different real orgs" false
// positive in match_test.go. This is exactly what "configurable merge thresholds" and
// "manual overrides" exist for: the algorithm honestly flags uncertainty rather than
// silently guessing, and a person (or a lower, deliberately-chosen threshold) can decide.
func TestResolveNameOnlyMatchIsHonestlyCautiousAtDefaultThreshold(t *testing.T) {
	githubEntry := discovery.Result{Name: "forcedream-mcp", Source: "GitHub", URL: "https://github.com/forcedreamai/forcedream-mcp"}
	smitheryEntry := discovery.Result{Name: "ForceDream MCP", Source: "Smithery", URL: ""} // no URL -- real, known fact about this source

	atDefault := Resolve([]discovery.Result{githubEntry, smitheryEntry}, DefaultMergeThreshold)
	if len(atDefault) != 2 {
		t.Fatalf("expected the cautious default threshold to NOT merge a name-only, no-org-corroboration match, got %d entities", len(atDefault))
	}

	// A person who's confirmed these really are the same thing (or a deliberately lower
	// threshold) can still merge them -- the signal isn't wrong, just weaker.
	atLowerThreshold := Resolve([]discovery.Result{githubEntry, smitheryEntry}, scoreExactNameNoOrg)
	if len(atLowerThreshold) != 1 {
		t.Fatalf("expected a deliberately lower threshold to merge the same name-only match, got %d entities", len(atLowerThreshold))
	}
}

func TestResolveTransitiveMerging(t *testing.T) {
	// A matches B (same org, exact name), B matches C (exact URL) -- but A and C alone
	// might not directly cross the threshold. Union-find must still unify all three via
	// the transitive chain through B.
	a := discovery.Result{Name: "forcedream-mcp", Source: "GitHub", URL: "https://github.com/forcedreamai/repo-a"}
	b := discovery.Result{Name: "forcedream-mcp", Source: "MCP Registry", URL: "https://github.com/forcedreamai/forcedream-mcp"}
	c := discovery.Result{Name: "totally different display name here", Source: "Smithery", URL: "github.com/forcedreamai/forcedream-mcp"}

	entities := Resolve([]discovery.Result{a, b, c}, DefaultMergeThreshold)
	if len(entities) != 1 {
		t.Fatalf("expected transitive merging to unify all 3 into 1 entity, got %d: %+v", len(entities), entities)
	}
	if len(entities[0].Sources) != 3 {
		t.Fatalf("expected all 3 sources in the transitively-merged entity, got %d", len(entities[0].Sources))
	}
}

func TestResolveDoesNotMergeUnrelatedResults(t *testing.T) {
	a := discovery.Result{Name: "Translation Bridge", Source: "GitHub", URL: "https://github.com/forcedreamai/translation-bridge"}
	b := discovery.Result{Name: "OCR Extraction Agent", Source: "GitHub", URL: "https://github.com/some-other-org/ocr-tool"}
	entities := Resolve([]discovery.Result{a, b}, DefaultMergeThreshold)
	if len(entities) != 2 {
		t.Fatalf("expected 2 genuinely unrelated results to stay as 2 separate entities, got %d", len(entities))
	}
	for _, e := range entities {
		if e.Merged {
			t.Fatalf("expected no entity to be marked Merged when nothing should have merged, got %+v", e)
		}
	}
}

func TestResolveRespectsConfigurableThreshold(t *testing.T) {
	// Same-org-only match (score 50) -- merges at a low threshold, doesn't at a high one.
	a := discovery.Result{Name: "forcedream-mcp", Source: "GitHub", URL: "https://github.com/forcedreamai/forcedream-mcp"}
	b := discovery.Result{Name: "completely-different-name", Source: "GitHub", URL: "https://github.com/forcedreamai/other-repo"}

	lowThreshold := Resolve([]discovery.Result{a, b}, 40.0)
	if len(lowThreshold) != 1 {
		t.Fatalf("expected a low (40) threshold to merge a same-org-only match (score 50), got %d entities", len(lowThreshold))
	}

	highThreshold := Resolve([]discovery.Result{a, b}, 80.0)
	if len(highThreshold) != 2 {
		t.Fatalf("expected a high (80) threshold to NOT merge a same-org-only match (score 50), got %d entities", len(highThreshold))
	}
}

func TestResolveConfidenceIsLowestLinkInChain(t *testing.T) {
	// A-B score high (exact URL, 100); B-C score lower (same org, similar name, 75).
	// The merged entity's confidence must honestly reflect the weakest real link (75),
	// not the strongest one (100) -- a chain is only as strong as its weakest link.
	a := discovery.Result{Name: "forcedream-mcp", Source: "GitHub", URL: "https://github.com/forcedreamai/forcedream-mcp"}
	b := discovery.Result{Name: "any-name", Source: "MCP Registry", URL: "https://github.com/forcedreamai/forcedream-mcp"} // exact URL match with a
	c := discovery.Result{Name: "any-nome", Source: "Smithery", URL: "https://github.com/forcedreamai/other-path"}          // same org + similar (typo) name vs b

	entities := Resolve([]discovery.Result{a, b, c}, DefaultMergeThreshold)
	if len(entities) != 1 {
		t.Fatalf("expected all 3 to merge, got %d entities: %+v", len(entities), entities)
	}
	if entities[0].Confidence != scoreSameOrgSimilarName {
		t.Fatalf("expected confidence to reflect the weakest link (%f), got %f", scoreSameOrgSimilarName, entities[0].Confidence)
	}
}

func TestResolveSingletonIsNotMarkedMerged(t *testing.T) {
	a := discovery.Result{Name: "Unique Standalone Agent", Source: "GitHub", URL: "https://github.com/nobody/unique"}
	entities := Resolve([]discovery.Result{a}, DefaultMergeThreshold)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity for 1 input result, got %d", len(entities))
	}
	if entities[0].Merged {
		t.Fatal("expected a standalone, unmerged result to have Merged=false")
	}
	if entities[0].Confidence != 0 {
		t.Fatalf("expected Confidence to be the honest zero-value for a non-merge, got %f", entities[0].Confidence)
	}
}

func TestResolveEntityIDIsDeterministic(t *testing.T) {
	results := []discovery.Result{
		{Name: "forcedream-mcp", Source: "GitHub", URL: "https://github.com/forcedreamai/forcedream-mcp"},
		{Name: "ForceDream MCP", Source: "Smithery", URL: ""},
	}
	first := Resolve(results, DefaultMergeThreshold)
	second := Resolve(results, DefaultMergeThreshold)
	if first[0].ID != second[0].ID {
		t.Fatalf("expected the same real inputs to always produce the same entity ID, got %q vs %q", first[0].ID, second[0].ID)
	}
}

func TestResolveEmptyInput(t *testing.T) {
	entities := Resolve(nil, DefaultMergeThreshold)
	if entities != nil {
		t.Fatalf("expected nil for empty input, got %+v", entities)
	}
}
