package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/forcedreamai/forcedream-cli/internal/confidence"
	"github.com/forcedreamai/forcedream-cli/internal/discovery"
	"github.com/forcedreamai/forcedream-cli/internal/entity"
	"github.com/forcedreamai/forcedream-cli/internal/graph"
	"github.com/forcedreamai/forcedream-cli/internal/reputation"
	"github.com/forcedreamai/forcedream-cli/internal/telemetry"
)

// ── forcedream status ────────────────────────────────────────────────────────────────
// Real, live health for every registered connector, plus real, locally-observed
// latency/reliability where any exists. Health checks run concurrently -- 13 sequential
// live network round-trips would make this command needlessly slow; a single slow or
// hanging connector must never block the others (hardened via the shared ctx's deadline,
// same one every other command already respects).
func cmdStatus(ctx context.Context, args []string) {
	connectors := discovery.AllConnectors()
	type row struct {
		name    string
		health  discovery.HealthStatus
		latency discovery.LatencyInfo
		rel     discovery.ReliabilityInfo
		caps    discovery.Capabilities
	}
	rows := make([]row, len(connectors))

	var wg sync.WaitGroup
	for i, c := range connectors {
		wg.Add(1)
		go func(i int, c discovery.Connector) {
			defer wg.Done()
			rows[i] = row{
				name:    c.Name(),
				health:  c.Health(ctx),
				latency: c.Latency(),
				rel:     c.Reliability(),
				caps:    c.Capabilities(),
			}
		}(i, c)
	}
	wg.Wait()

	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	fmt.Println("Connector status (live health + real, locally-observed history):")
	for _, r := range rows {
		status := "reachable"
		if !r.health.Healthy {
			status = "unreachable: " + r.health.Message
		}
		extra := "no recorded search history on this machine yet"
		if r.rel.HasData {
			extra = fmt.Sprintf("%.0f%% success over %d attempt(s)", r.rel.SuccessRate*100, r.rel.Attempts)
			if r.latency.HasData {
				extra += fmt.Sprintf(", avg %dms", r.latency.AverageMs)
			}
		}
		paidNote := ""
		if r.caps.RequiresPayment {
			paidNote = " [requires FD_LIVE_KEY]"
		}
		fmt.Printf("  %-16s %-12s %s%s\n", r.name, status, extra, paidNote)
	}
}

// ── forcedream reputation <name> ─────────────────────────────────────────────────────
// Real connector reputation for a known connector name. Agent-level reputation (a
// specific, discovered package/agent rather than a connector itself) genuinely needs
// entity + search-result context this standalone command doesn't have -- honestly
// explained, not silently faked.
func cmdReputation(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: forcedream reputation <connector-name>")
		fmt.Println("Example: forcedream reputation npm")
		os.Exit(1)
	}
	name := args[0]

	var matched discovery.Connector
	for _, c := range discovery.AllConnectors() {
		if strings.EqualFold(c.Name(), name) {
			matched = c
			break
		}
	}
	if matched == nil {
		fmt.Printf("%q is not a known connector name.\n", name)
		fmt.Println("Known connectors:")
		for _, c := range discovery.AllConnectors() {
			fmt.Println("  " + c.Name())
		}
		fmt.Println("\nAgent-level reputation (a specific discovered package or agent, rather than a connector) requires real search-result context -- run `forcedream search <query>` first; this standalone command can't honestly compute it without that.")
		os.Exit(1)
	}

	rep := reputation.ConnectorReputation(matched.Name())
	printReputation(rep)
}

func printReputation(rep reputation.Reputation) {
	fmt.Printf("Reputation: %s\n", rep.Subject)
	for _, c := range rep.Components {
		if !c.Available {
			fmt.Printf("  %-24s Insufficient Data -- %s\n", c.Dimension, c.Explanation)
			continue
		}
		fmt.Printf("  %-24s %.1f -- %s\n", c.Dimension, c.Score, c.Explanation)
	}
}

// ── forcedream confidence <query> ────────────────────────────────────────────────────
// Runs a real search, resolves real entities, and shows a real, transparent confidence
// breakdown per resolved entity -- reusing the same, already-proven search/resolve
// pipeline cmdSearch uses, not a separate, parallel implementation that could drift.
func cmdConfidence(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: forcedream confidence <query>")
		os.Exit(1)
	}
	query := strings.Join(args, " ")

	manager := discovery.NewSearchManager(discovery.AllConnectors()...)
	managed := manager.SearchAll(ctx, query, 30)

	var all []discovery.Result
	for _, m := range managed {
		all = append(all, m.Outcome.Results...)
	}
	if len(all) == 0 {
		fmt.Println("No real search results found for this query -- nothing to compute confidence for.")
		return
	}

	resolved := entity.ResolveWithOverrides(all, entity.DefaultMergeThreshold)
	if len(resolved) == 0 {
		fmt.Println("Real results were found, but entity resolution produced no resolved entities.")
		return
	}

	// Honest, stated criterion for ordering -- entities whose real, canonical name
	// contains the query surface first, rather than relying on resolution's internal,
	// unspecified order.
	qLower := strings.ToLower(query)
	sort.SliceStable(resolved, func(i, j int) bool {
		iMatch := strings.Contains(strings.ToLower(resolved[i].CanonicalName), qLower)
		jMatch := strings.Contains(strings.ToLower(resolved[j].CanonicalName), qLower)
		if iMatch != jMatch {
			return iMatch
		}
		return len(resolved[i].Sources) > len(resolved[j].Sources)
	})

	limit := 5
	if len(resolved) < limit {
		limit = len(resolved)
	}
	for i := 0; i < limit; i++ {
		e := resolved[i]
		fmt.Printf("\n%s\n", e.CanonicalName)
		conf := confidence.Compute(e, all)
		// Real, separate, optional enrichment (makes real, live network calls, unlike
		// Compute() itself) -- honestly replaces the package_authenticity component only
		// when a real, verified-mechanism source (Maven Central, npm) is present, and
		// recomputes Overall so it doesn't stay stale relative to what's now available.
		if authScore := confidence.CheckPackageAuthenticity(ctx, e, all); authScore.Available {
			for i, c := range conf.Components {
				if c.Component == confidence.ComponentPackageAuthenticity {
					conf.Components[i] = authScore
				}
			}
			conf.Overall = confidence.RecomputeOverall(conf.Components)
		}
		if !conf.Overall.Available {
			fmt.Println("  Overall: Insufficient Data")
		} else {
			fmt.Printf("  Overall: %.1f\n", conf.Overall.Score)
		}
		for _, c := range conf.Components {
			if !c.Available {
				fmt.Printf("    %-24s Insufficient Data -- %s\n", c.Component, c.Explanation)
				continue
			}
			fmt.Printf("    %-24s %.1f -- %s\n", c.Component, c.Score, c.Explanation)
		}
	}
	if len(resolved) > limit {
		fmt.Printf("\n(%d more resolved entities not shown)\n", len(resolved)-limit)
	}
}

// ── forcedream graph <name> ──────────────────────────────────────────────────────────
// Queries the real, local knowledge graph (already populated by past real searches on
// this machine) by name -- a case-insensitive substring match, since the person running
// this is unlikely to know the exact, internal node-ID format.
func cmdGraph(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: forcedream graph <name>")
		os.Exit(1)
	}
	name := strings.ToLower(strings.Join(args, " "))

	g := graph.Load()
	if g == nil || len(g.Nodes) == 0 {
		fmt.Println("No local graph data yet on this machine. Run `forcedream search <query>` first -- the graph is built from real, resolved search results.")
		return
	}

	type match struct {
		id   string
		node graph.Node
	}
	var matches []match
	for id, n := range g.Nodes {
		if strings.Contains(strings.ToLower(n.Name), name) {
			matches = append(matches, match{id: id, node: n})
		}
	}
	if len(matches) == 0 {
		fmt.Printf("No real, local graph nodes match %q.\n", strings.Join(args, " "))
		return
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].node.Name < matches[j].node.Name })

	limit := 10
	if len(matches) < limit {
		limit = len(matches)
	}
	for i := 0; i < limit; i++ {
		m := matches[i]
		fmt.Printf("\n%s (%s) [%s]\n", m.node.Name, m.id, m.node.Type)
		from := g.EdgesFrom(m.id)
		to := g.EdgesTo(m.id)
		if len(from) == 0 && len(to) == 0 {
			fmt.Println("  no real edges recorded yet")
		}
		for _, e := range from {
			fmt.Printf("  --%s--> %s\n", e.Type, e.To)
		}
		for _, e := range to {
			fmt.Printf("  <--%s-- %s\n", e.Type, e.From)
		}
	}
	if len(matches) > limit {
		fmt.Printf("\n(%d more matching nodes not shown)\n", len(matches)-limit)
	}
}

// ── forcedream telemetry ─────────────────────────────────────────────────────────────
// Shows real, local telemetry state (level, install ID), plus a best-effort, real fetch
// of the backend's real, aggregate summary -- hardened so a network failure here never
// blocks or crashes the command; the real, local info still displays regardless.
func cmdTelemetry(ctx context.Context, args []string) {
	fmt.Println("Local telemetry state:")
	fmt.Printf("  Level:       %s (set FD_TELEMETRY=off|basic|detailed to change)\n", telemetry.CurrentLevel())
	fmt.Printf("  Install ID:  %s (random, non-identifying; never a real account, key, or device ID)\n", telemetry.CurrentInstallID())

	fmt.Println("\nReal, aggregate backend summary (today):")
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "https://api.forcedream.ai/v1/telemetry/summary", nil)
	if err != nil {
		fmt.Println("  (could not build request)")
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("  Could not reach the backend right now -- this doesn't affect local telemetry collection, only this summary view.")
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != http.StatusOK {
		fmt.Println("  Could not fetch a real summary right now.")
		return
	}
	var summary map[string]any
	if err := json.Unmarshal(body, &summary); err != nil {
		fmt.Println("  Received an unexpected response shape from the backend.")
		return
	}
	pretty, _ := json.MarshalIndent(summary, "  ", "  ")
	fmt.Println("  " + string(pretty))
}
