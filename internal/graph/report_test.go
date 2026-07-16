package graph

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReportToBackendSendsRealPayloadShape(t *testing.T) {
	var received backendMergePayload
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	g := &Graph{
		Nodes: map[string]Node{
			"agent:ent_1": {ID: "agent:ent_1", Type: NodeAgent, Name: "Real Agent"},
		},
		Edges: []Edge{
			{From: "agent:ent_1", To: "org:realorg", Type: EdgePublishedBy},
		},
	}
	ReportToBackend(g, server.URL)

	if gotPath != "/v1/graph/merge" {
		t.Fatalf("expected the real /v1/graph/merge path, got %q", gotPath)
	}
	if len(received.Nodes) != 1 || received.Nodes[0].ID != "agent:ent_1" || received.Nodes[0].Type != "agent" {
		t.Fatalf("expected the real node to be sent in the real backend shape, got %+v", received.Nodes)
	}
	if len(received.Edges) != 1 || received.Edges[0].Type != "published_by" {
		t.Fatalf("expected the real edge to be sent in the real backend shape, got %+v", received.Edges)
	}
}

func TestReportToBackendDoesNothingForEmptyGraph(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	ReportToBackend(&Graph{Nodes: map[string]Node{}}, server.URL)
	if called {
		t.Fatal("expected no real network call for a genuinely empty graph")
	}
}

func TestReportToBackendFailSafeOnServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	g := &Graph{Nodes: map[string]Node{"agent:ent_1": {ID: "agent:ent_1", Type: NodeAgent, Name: "x"}}}
	// Must not panic or otherwise disrupt the caller even when the real server errors.
	ReportToBackend(g, server.URL)
}
