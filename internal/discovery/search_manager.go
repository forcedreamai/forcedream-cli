package discovery

import (
	"context"
	"sync"
)

// SearchManager is the real orchestrator behind the framework's actual value: adding a new
// source means implementing Connector once and adding it to a slice here -- not hand-
// writing a new goroutine block in main.go and hoping to remember every detail (the
// channel buffer size, the wg.Add(1)/wg.Done() pairing, the result-forwarding) the same
// way each time.
type SearchManager struct {
	connectors []Connector
}

func NewSearchManager(connectors ...Connector) *SearchManager {
	return &SearchManager{connectors: connectors}
}

// ManagedResult is one connector's real outcome, tagged with its name so a caller can
// still report per-source status the way main.go's stderr summary already does.
type ManagedResult struct {
	Name    string
	Outcome Outcome
	Err     error
}

// SearchAll queries every registered connector in parallel and returns once all have
// completed -- the same real concurrency the hand-written version had, now generic over
// however many connectors are registered rather than a fixed, hand-counted set.
func (m *SearchManager) SearchAll(ctx context.Context, query string, limit int) []ManagedResult {
	var wg sync.WaitGroup
	resultsCh := make(chan ManagedResult, len(m.connectors))

	for _, c := range m.connectors {
		wg.Add(1)
		go func(conn Connector) {
			defer wg.Done()
			outcome, err := conn.Search(ctx, query, limit)
			resultsCh <- ManagedResult{Name: conn.Name(), Outcome: outcome, Err: err}
		}(c)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	all := make([]ManagedResult, 0, len(m.connectors))
	for r := range resultsCh {
		all = append(all, r)
	}
	return all
}

// AllResults flattens every connector's real results into one slice, ready for
// discovery.Merge + ranking.Rank the same way main.go's `all` slice already worked --
// this is additive, not a replacement for that pipeline.
func AllResults(managed []ManagedResult) []Result {
	var all []Result
	for _, m := range managed {
		if m.Outcome.Available {
			all = append(all, m.Outcome.Results...)
		}
	}
	return all
}
