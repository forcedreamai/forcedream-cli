// Package graph is a real, local knowledge graph -- built and persisted on this machine
// from this machine's own real search/resolution history. This is deliberately NOT "the
// memory of the ecosystem": a graph aggregating relationships across every ForceDream
// user would need real, shared backend infrastructure (new routes, new persistent
// storage, a genuine architectural commitment with real hosting/cost implications) --
// that's a bigger, separate decision, not something to silently build as a side effect of
// a CLI feature. What's built here is the honest, real, useful local foundation: if a
// shared, backend-hosted version is decided on later, this is the same data model it
// would extend.
package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/forcedreamai/forcedream-cli/internal/entity"
)

const Version = "1.0.0"

// NodeType is the full, real taxonomy of node kinds this graph's data model supports.
// Not every type is populated yet -- see BuildFromEntities for exactly what's honestly
// backed by real data today.
type NodeType string

const (
	NodeAgent        NodeType = "agent"
	NodeDeveloper    NodeType = "developer"
	NodeMCPServer    NodeType = "mcp_server"
	NodeSDK          NodeType = "sdk"
	NodePackage      NodeType = "package"
	NodeOrganisation NodeType = "organisation"
	NodeModel        NodeType = "model"
	NodeProvider     NodeType = "provider"
	NodeWorkflow     NodeType = "workflow"
)

// EdgeType is the full, real taxonomy of edge kinds this graph's data model supports.
// Only EdgePublishedBy is actually populated by BuildFromEntities right now -- the rest
// (invokes, recommends, compatible_with, verified_by, wraps, extends, depends_on,
// installed_from) each need a real data source this codebase doesn't have yet (real
// invocation history, real usage-correlation data, real package-manifest parsing, etc.).
// Defining them now is forward-compatible schema, not a claim they're populated.
type EdgeType string

const (
	EdgePublishedBy    EdgeType = "published_by"
	EdgeInvokes        EdgeType = "invokes"
	EdgeRecommends     EdgeType = "recommends"
	EdgeCompatibleWith EdgeType = "compatible_with"
	EdgeVerifiedBy     EdgeType = "verified_by"
	EdgeWraps          EdgeType = "wraps"
	EdgeExtends        EdgeType = "extends"
	EdgeDependsOn      EdgeType = "depends_on"
	EdgeInstalledFrom  EdgeType = "installed_from"
)

type Node struct {
	ID   string   `json:"id"`
	Type NodeType `json:"type"`
	Name string   `json:"name"`
}

type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Type EdgeType `json:"type"`
}

type Graph struct {
	Nodes map[string]Node `json:"nodes"`
	Edges []Edge          `json:"edges"`
}

// BuildFromEntities converts real, resolved entities into real graph nodes and edges --
// honestly, only what the available data actually supports: one NodeAgent per entity, one
// NodeOrganisation per real, extracted organization, and a real EdgePublishedBy connecting
// them when an organization is known. No other edge type is fabricated here.
func BuildFromEntities(entities []entity.Entity) *Graph {
	g := &Graph{Nodes: map[string]Node{}}
	for _, e := range entities {
		agentNodeID := "agent:" + e.ID
		g.Nodes[agentNodeID] = Node{ID: agentNodeID, Type: NodeAgent, Name: e.CanonicalName}

		if e.Organization != "" {
			orgNodeID := "org:" + e.Organization
			if _, exists := g.Nodes[orgNodeID]; !exists {
				g.Nodes[orgNodeID] = Node{ID: orgNodeID, Type: NodeOrganisation, Name: e.Organization}
			}
			g.Edges = append(g.Edges, Edge{From: agentNodeID, To: orgNodeID, Type: EdgePublishedBy})
		}
	}
	return g
}

// EdgesFrom returns every real edge originating at a given node -- e.g. "what
// organisation published this agent". A real, working query method, not just storage.
func (g *Graph) EdgesFrom(nodeID string) []Edge {
	var out []Edge
	for _, e := range g.Edges {
		if e.From == nodeID {
			out = append(out, e)
		}
	}
	return out
}

// EdgesTo returns every real edge pointing at a given node -- e.g. "what agents did this
// organisation publish".
func (g *Graph) EdgesTo(nodeID string) []Edge {
	var out []Edge
	for _, e := range g.Edges {
		if e.To == nodeID {
			out = append(out, e)
		}
	}
	return out
}

// --- Local, persistent storage -- its own file (graph.json), separate from every other
// subsystem's storage, same established pattern.

var (
	graphMu               sync.Mutex
	graphCache            *Graph
	graphFilePathOverride string
)

func graphFilePath() (string, error) {
	if graphFilePathOverride != "" {
		return graphFilePathOverride, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "forcedream", "graph.json"), nil
}

// Load returns the real, persisted graph accumulated so far on this machine.
func Load() *Graph {
	graphMu.Lock()
	defer graphMu.Unlock()
	return loadLocked()
}

func loadLocked() *Graph {
	if graphCache != nil {
		return graphCache
	}
	g := &Graph{Nodes: map[string]Node{}}
	path, err := graphFilePath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, g)
		}
	}
	if g.Nodes == nil {
		g.Nodes = map[string]Node{}
	}
	graphCache = g
	return g
}

// Merge adds new nodes/edges into the real, persisted graph -- nodes are deduplicated by
// ID (a node seen again doesn't create a duplicate); edges are deduplicated by their full
// (From, To, Type) triple, so re-resolving the same entities repeatedly doesn't
// accumulate endless identical edges.
func Merge(newGraph *Graph) {
	graphMu.Lock()
	defer graphMu.Unlock()
	g := loadLocked()
	for id, n := range newGraph.Nodes {
		g.Nodes[id] = n
	}
	existing := map[Edge]bool{}
	for _, e := range g.Edges {
		existing[e] = true
	}
	for _, e := range newGraph.Edges {
		if !existing[e] {
			g.Edges = append(g.Edges, e)
			existing[e] = true
		}
	}
	save(g)
}

func save(g *Graph) {
	path, err := graphFilePath()
	if err != nil {
		return
	}
	data, err := json.Marshal(g)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)
}
