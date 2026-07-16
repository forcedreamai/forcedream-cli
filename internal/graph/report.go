package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// backendNode/backendEdge match the real wire shape /v1/graph/merge expects -- kept as
// their own, separate types rather than reusing Node/Edge directly, so a change to this
// package's own local JSON tags never silently changes what's sent over the wire.
type backendNode struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}
type backendEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}
type backendMergePayload struct {
	Nodes []backendNode `json:"nodes"`
	Edges []backendEdge `json:"edges"`
}

// ReportToBackend sends this graph's real nodes/edges to the real, shared, backend-hosted
// Knowledge Graph (/v1/graph/merge) -- the same real data this machine already computed
// and persisted locally, now also contributed to the cross-user, shared version. Fully
// fail-safe by design, matching telemetry's own posture: never blocks, never panics, never
// affects the real command the person actually ran, even if the network call fails or
// times out. Bounded to the same 200-node/200-edge batch size the backend itself enforces,
// so a very large local graph doesn't overflow a single real request.
func ReportToBackend(g *Graph, apiBase string) {
	if g == nil || (len(g.Nodes) == 0 && len(g.Edges) == 0) {
		return
	}

	payload := backendMergePayload{}
	for _, n := range g.Nodes {
		if len(payload.Nodes) >= 200 {
			break
		}
		payload.Nodes = append(payload.Nodes, backendNode{ID: n.ID, Type: string(n.Type), Name: n.Name})
	}
	for _, e := range g.Edges {
		if len(payload.Edges) >= 200 {
			break
		}
		payload.Edges = append(payload.Edges, backendEdge{From: e.From, To: e.To, Type: string(e.Type)})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/v1/graph/merge", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
