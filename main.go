package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	forcedream "github.com/forcedreamai/forcedream-sdk-go"
	"github.com/forcedreamai/forcedream-cli/internal/discovery"
	"github.com/forcedreamai/forcedream-cli/internal/entity"
	"github.com/forcedreamai/forcedream-cli/internal/graph"
	"github.com/forcedreamai/forcedream-cli/internal/ranking"
	"github.com/forcedreamai/forcedream-cli/internal/telemetry"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch os.Args[1] {
	case "search":
		cmdSearch(ctx, os.Args[2:])
	case "invoke":
		cmdInvoke(ctx, os.Args[2:])
	case "verify":
		cmdVerify(ctx, os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`forcedream -- discover, invoke, and cryptographically verify AI agents.

Usage:
  forcedream search <query>              Search across real, available agent/server sources
  forcedream invoke <agent_slug> <task>  Invoke a real ForceDream agent
  forcedream verify <task_id>            Verify a real Ed25519 proof, entirely client-side

Environment:
  FD_LIVE_KEY       Required for invoke, and for Smithery/open-web search (a real
                    fd_live_... billing key with a positive balance) -- the same key
                    covers all three
  GITHUB_TOKEN      Optional -- raises GitHub search's rate limit if set`)
}

func cmdSearch(ctx context.Context, args []string) {
	telemetryStart := telemetry.StartTimer()
	telemetryBatch := telemetry.NewBatch()

	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	// All six connectors now share the same real interface -- free and paid sources run
	// through one SearchManager, not two separate hand-written goroutine setups. This is
	// also what makes adaptive scheduling (skipping an unhealthy connector, falling back
	// to a cached result) possible to build as one, uniform mechanism later, rather than
	// two different ones for free vs. paid sources.
	manager := discovery.NewSearchManager(
		discovery.ForceDreamConnector{},
		discovery.MCPRegistryConnector{},
		discovery.GitHubConnector{},
		discovery.NpmConnector{},
		discovery.SmitheryConnector{},
		discovery.WebConnector{},
	)
	managed := manager.SearchAll(ctx, query, 30)

	var all []discovery.Result
	var sourceStatus []string
	// sourcesQueried holds only fixed, known source names (never user input) for the
	// telemetry event below -- built directly here, at the one real point each source's
	// real outcome is known, rather than parsed back out of the human-readable
	// sourceStatus strings.
	var sourcesQueried []string
	for _, r := range managed {
		if r.Err != nil {
			sourceStatus = append(sourceStatus, fmt.Sprintf("%s: unavailable (%v)", r.Name, r.Err))
			continue
		}
		if !r.Outcome.Available {
			// Real, specific reasons from the backend gate -- not a generic failure.
			// Detected here so the person knows exactly what to do next, matching the
			// real states the backend can return: auth_required (no FD_LIVE_KEY, or an
			// invalid one), insufficient_funds (real account, but balance <= 0),
			// feature_not_enabled (real account, but the paid-search entitlement is
			// off), and quota_exceeded.
			switch r.Outcome.Reason {
			case "auth_required":
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: sign-in required -- %s", r.Name, r.Outcome.Message))
			case "insufficient_funds":
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: insufficient balance -- %s", r.Name, r.Outcome.Message))
			case "feature_not_enabled":
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: feature not enabled on this account -- %s", r.Name, r.Outcome.Message))
			case "quota_exceeded":
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: quota exceeded -- %s", r.Name, r.Outcome.Message))
			default:
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: unavailable -- %s", r.Name, r.Outcome.Message))
			}
			continue
		}
		sourceStatus = append(sourceStatus, fmt.Sprintf("%s: %d results%s", r.Name, len(r.Outcome.Results), staleLabel(r.Outcome.Stale)))
		sourcesQueried = append(sourcesQueried, r.Name)
		all = append(all, r.Outcome.Results...)
	}

	// Dedupe and rank are deliberately two separate, independently-callable steps now --
	// discovery.Merge only dedupes; internal/ranking (a standalone subsystem, its own
	// engine/weights/tests/version) does the actual ranking. Neither package depends on
	// the other beyond ranking's one-way use of discovery.Result as the shared data type.
	merged := ranking.Rank(discovery.Merge(all), ranking.DefaultWeights())

	// Entity resolution + the local knowledge graph run as an additional, side step on
	// the same raw results -- deliberately not woven into the merge/rank pipeline above,
	// so this brand-new subsystem can't put the already-proven search output at risk.
	// Real, persisted decisions accumulate on this machine across searches; genuine
	// failures here are non-fatal to the actual search the person asked for.
	if resolved := entity.ResolveWithOverrides(all, entity.DefaultMergeThreshold); len(resolved) > 0 {
		graph.Merge(graph.BuildFromEntities(resolved))
	}

	// Real telemetry event -- anonymous by default (kind/success/duration/version/which
	// fixed, known source names succeeded), never the query text itself. SearchTermHash
	// is a real, one-way hash (never the raw term) enabling real repeat-usage/"most
	// searched" aggregate analytics without ever capturing readable search content.
	telemetryBatch.Add(telemetry.Event{
		Kind:           "search",
		Success:        true,
		DurationMs:     telemetry.ElapsedMs(telemetryStart),
		CLIVersion:     "v0.3.0",
		SourcesUsed:    sourcesQueried,
		SearchTermHash: telemetry.HashSearchTerm(query),
	})
	telemetryBatch.Flush("https://api.forcedream.ai")

	fmt.Fprintln(os.Stderr, "Sources queried:")
	for _, s := range sourceStatus {
		fmt.Fprintln(os.Stderr, "  "+s)
	}
	fmt.Fprintln(os.Stderr)

	out, _ := json.MarshalIndent(merged, "", "  ")
	fmt.Println(string(out))
}

func cmdInvoke(ctx context.Context, args []string) {
	telemetryStart := telemetry.StartTimer()
	telemetryBatch := telemetry.NewBatch()

	if len(args) < 2 {
		fmt.Println("Usage: forcedream invoke <agent_slug> <task>")
		os.Exit(1)
	}
	apiKey := os.Getenv("FD_LIVE_KEY")
	if apiKey == "" {
		fmt.Println("Error: FD_LIVE_KEY environment variable required (a real fd_live_... billing key).")
		os.Exit(1)
	}
	client := forcedream.New(apiKey)
	result, err := client.Invoke(ctx, args[0], args[1], 60)

	// Real telemetry for invoke -- this command previously emitted none at all. AgentSlug
	// is the real, public marketplace identifier being invoked (safe -- a product
	// identifier, not personal data); task content and everything else about what was
	// actually asked of the agent is still never captured.
	telemetryBatch.Add(telemetry.Event{
		Kind:       "invoke",
		Success:    err == nil,
		DurationMs: telemetry.ElapsedMs(telemetryStart),
		CLIVersion: "v0.3.0",
		AgentSlug:  args[0],
	})
	telemetryBatch.Flush("https://api.forcedream.ai")

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}

func cmdVerify(ctx context.Context, args []string) {
	telemetryStart := telemetry.StartTimer()
	telemetryBatch := telemetry.NewBatch()

	if len(args) < 1 {
		fmt.Println("Usage: forcedream verify <task_id>")
		os.Exit(1)
	}
	client := forcedream.New("")
	result, err := client.Verify(ctx, args[0], nil)

	// Real telemetry for verify -- same pattern, previously emitted none at all. No task
	// ID, no proof content, only the fixed kind/success/duration.
	telemetryBatch.Add(telemetry.Event{
		Kind:       "verify",
		Success:    err == nil,
		DurationMs: telemetry.ElapsedMs(telemetryStart),
		CLIVersion: "v0.3.0",
	})
	telemetryBatch.Flush("https://api.forcedream.ai")

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}

// staleLabel honestly marks a result as coming from adaptive scheduling's stale-cache
// fallback (the connector was skipped as currently unhealthy), rather than presenting it
// indistinguishably from a real, fresh result.
func staleLabel(stale bool) string {
	if stale {
		return " (unhealthy -- showing cached data)"
	}
	return ""
}
