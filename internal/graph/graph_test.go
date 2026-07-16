package graph

import (
	"path/filepath"
	"testing"

	"github.com/forcedreamai/forcedream-cli/internal/entity"
)

func resetGraphForTest(t *testing.T) {
	t.Helper()
	graphFilePathOverride = filepath.Join(t.TempDir(), "graph_test.json")
	graphCache = nil
	t.Cleanup(func() {
		graphFilePathOverride = ""
		graphCache = nil
	})
}

func TestBuildFromEntitiesCreatesAgentNode(t *testing.T) {
	entities := []entity.Entity{
		{ID: "ent_abc123", CanonicalName: "forcedream-mcp"},
	}
	g := BuildFromEntities(entities)
	node, ok := g.Nodes["agent:ent_abc123"]
	if !ok {
		t.Fatal("expected a real agent node for the resolved entity")
	}
	if node.Type != NodeAgent || node.Name != "forcedream-mcp" {
		t.Fatalf("unexpected node: %+v", node)
	}
}

func TestBuildFromEntitiesCreatesOrgNodeAndEdgeWhenOrgKnown(t *testing.T) {
	entities := []entity.Entity{
		{ID: "ent_abc123", CanonicalName: "forcedream-mcp", Organization: "forcedreamai"},
	}
	g := BuildFromEntities(entities)
	if _, ok := g.Nodes["org:forcedreamai"]; !ok {
		t.Fatal("expected a real organisation node when Organization is known")
	}
	edges := g.EdgesFrom("agent:ent_abc123")
	if len(edges) != 1 || edges[0].Type != EdgePublishedBy || edges[0].To != "org:forcedreamai" {
		t.Fatalf("expected a real published_by edge to the organisation, got %+v", edges)
	}
}

func TestBuildFromEntitiesSkipsOrgEdgeWhenOrgUnknown(t *testing.T) {
	entities := []entity.Entity{
		{ID: "ent_xyz", CanonicalName: "Some Standalone Tool"}, // no Organization
	}
	g := BuildFromEntities(entities)
	if len(g.Edges) != 0 {
		t.Fatalf("expected no edges when no real organisation signal exists, got %+v", g.Edges)
	}
}

func TestEdgesToFindsIncomingEdges(t *testing.T) {
	entities := []entity.Entity{
		{ID: "ent_a", CanonicalName: "Agent A", Organization: "shared-org"},
		{ID: "ent_b", CanonicalName: "Agent B", Organization: "shared-org"},
	}
	g := BuildFromEntities(entities)
	incoming := g.EdgesTo("org:shared-org")
	if len(incoming) != 2 {
		t.Fatalf("expected 2 real incoming published_by edges to the shared org, got %d", len(incoming))
	}
}

func TestMergeDeduplicatesNodesAndEdges(t *testing.T) {
	resetGraphForTest(t)

	entities := []entity.Entity{
		{ID: "ent_a", CanonicalName: "Agent A", Organization: "org-x"},
	}
	g1 := BuildFromEntities(entities)
	Merge(g1)
	Merge(g1) // merge the exact same graph again

	loaded := Load()
	if len(loaded.Nodes) != 2 { // 1 agent + 1 org
		t.Fatalf("expected exactly 2 nodes after merging the same graph twice (deduplicated), got %d", len(loaded.Nodes))
	}
	if len(loaded.Edges) != 1 {
		t.Fatalf("expected exactly 1 edge after merging the same graph twice (deduplicated), got %d", len(loaded.Edges))
	}
}

func TestMergePersistsAcrossReload(t *testing.T) {
	resetGraphForTest(t)

	entities := []entity.Entity{{ID: "ent_persist", CanonicalName: "Persisted Agent", Organization: "persist-org"}}
	Merge(BuildFromEntities(entities))

	graphCache = nil // force a real reload from disk, not the in-memory value just set
	loaded := Load()
	if _, ok := loaded.Nodes["agent:ent_persist"]; !ok {
		t.Fatal("expected the merged graph to genuinely persist to disk and be reloadable")
	}
}
