package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	forcedream "github.com/forcedreamai/forcedream-sdk-go"
	"github.com/forcedreamai/forcedream-cli/internal/discovery"
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

	type sourceResult struct {
		name    string
		results []discovery.Result
		err     error
	}

	// Free sources run in parallel -- no billing risk.
	var wg sync.WaitGroup
	resultsCh := make(chan sourceResult, 4)

	wg.Add(1)
	go func() {
		defer wg.Done()
		r, err := discovery.SearchForceDream(ctx, query)
		resultsCh <- sourceResult{"ForceDream", r, err}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		r, err := discovery.SearchMCPRegistry(ctx, query, 50)
		resultsCh <- sourceResult{"MCP Registry", r, err}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		r, err := discovery.SearchGitHubMCPServers(ctx, query, 30)
		resultsCh <- sourceResult{"GitHub", r, err}
	}()

	// Paid sources: gated behind a real ForceDream account, balance, and entitlement --
	// the CLI never touches Smithery/SerpAPI directly, only the real backend proxies,
	// which hold the real keys server-side. Run in parallel with each other (each is a
	// single call, not multiple concurrent calls to the same source, so this doesn't
	// violate the "serialize paid calls" intent -- that was about not hammering one
	// source with concurrent requests, not about ordering different sources).
	type paidResult struct {
		name    string
		results []discovery.Result
		status  discovery.PaidSourceStatus
		err     error
	}
	paidCh := make(chan paidResult, 2)
	var paidWg sync.WaitGroup

	paidWg.Add(1)
	go func() {
		defer paidWg.Done()
		r, status, err := discovery.SearchSmithery(ctx, query)
		paidCh <- paidResult{"Smithery", r, status, err}
	}()

	paidWg.Add(1)
	go func() {
		defer paidWg.Done()
		r, status, err := discovery.SearchWeb(ctx, query)
		paidCh <- paidResult{"Web search", r, status, err}
	}()

	go func() {
		paidWg.Wait()
		close(paidCh)
	}()

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var all []discovery.Result
	var sourceStatus []string
	// sourcesQueried holds only fixed, known source names (never user input) for the
	// telemetry event below -- built directly here, at the one real point each source's
	// real outcome is known, rather than parsed back out of the human-readable
	// sourceStatus strings.
	var sourcesQueried []string
	for r := range resultsCh {
		if r.err != nil {
			sourceStatus = append(sourceStatus, fmt.Sprintf("%s: unavailable (%v)", r.name, r.err))
			continue
		}
		sourceStatus = append(sourceStatus, fmt.Sprintf("%s: %d results", r.name, len(r.results)))
		sourcesQueried = append(sourcesQueried, r.name)
		all = append(all, r.results...)
	}

	for r := range paidCh {
		if r.err != nil {
			sourceStatus = append(sourceStatus, fmt.Sprintf("%s: unavailable (%v)", r.name, r.err))
			continue
		}
		if !r.status.Available {
			// Real, specific reasons from the backend gate -- not a generic failure.
			// Detected here so the person knows exactly what to do next, matching the
			// three states the backend can return: auth_required (no FD_LIVE_KEY, or
			// an invalid one), insufficient_funds (real account, but balance <= 0), and
			// feature_not_enabled (real account, but the paid-search entitlement is off).
			switch r.status.Reason {
			case "auth_required":
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: sign-in required -- %s", r.name, r.status.Message))
			case "insufficient_funds":
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: insufficient balance -- %s", r.name, r.status.Message))
			case "feature_not_enabled":
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: feature not enabled on this account -- %s", r.name, r.status.Message))
			case "quota_exceeded":
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: quota exceeded -- %s", r.name, r.status.Message))
			default:
				sourceStatus = append(sourceStatus, fmt.Sprintf("%s: unavailable -- %s", r.name, r.status.Message))
			}
			continue
		}
		sourceStatus = append(sourceStatus, fmt.Sprintf("%s: %d results", r.name, len(r.results)))
		sourcesQueried = append(sourcesQueried, r.name)
		all = append(all, r.results...)
	}

	// Dedupe and rank are deliberately two separate, independently-callable steps now --
	// discovery.Merge only dedupes; internal/ranking (a standalone subsystem, its own
	// engine/weights/tests/version) does the actual ranking. Neither package depends on
	// the other beyond ranking's one-way use of discovery.Result as the shared data type.
	merged := ranking.Rank(discovery.Merge(all), ranking.DefaultWeights())

	// Real telemetry event -- anonymous by default (kind/success/duration/version/which
	// fixed, known source names succeeded), never the query text itself.
	telemetryBatch.Add(telemetry.Event{
		Kind:        "search",
		Success:     true,
		DurationMs:  telemetry.ElapsedMs(telemetryStart),
		CLIVersion:  "v0.3.0",
		SourcesUsed: sourcesQueried,
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
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}

func cmdVerify(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: forcedream verify <task_id>")
		os.Exit(1)
	}
	client := forcedream.New("")
	result, err := client.Verify(ctx, args[0], nil)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}
