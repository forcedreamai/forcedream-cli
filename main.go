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
  FD_LIVE_KEY       Required for invoke (a real fd_live_... billing key)
  SMITHERY_API_KEY  Optional -- enables Smithery search if set
  SERPAPI_API_KEY   Optional -- enables open-web search if set
  GITHUB_TOKEN      Optional -- raises GitHub search's rate limit if set`)
}

func cmdSearch(ctx context.Context, args []string) {
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

	// Paid sources (Smithery, SerpAPI): built separately, gated on the billing-safety
	// architecture question -- see BillingSafety.md. Not wired in yet.

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var all []discovery.Result
	var sourceStatus []string
	for r := range resultsCh {
		if r.err != nil {
			sourceStatus = append(sourceStatus, fmt.Sprintf("%s: unavailable (%v)", r.name, r.err))
			continue
		}
		sourceStatus = append(sourceStatus, fmt.Sprintf("%s: %d results", r.name, len(r.results)))
		all = append(all, r.results...)
	}

	merged := discovery.MergeAndRank(all)

	fmt.Fprintln(os.Stderr, "Sources queried:")
	for _, s := range sourceStatus {
		fmt.Fprintln(os.Stderr, "  "+s)
	}
	fmt.Fprintln(os.Stderr, "  Smithery: not configured (SMITHERY_API_KEY not set)")
	fmt.Fprintln(os.Stderr, "  Web search: not configured (SERPAPI_API_KEY not set)")
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
