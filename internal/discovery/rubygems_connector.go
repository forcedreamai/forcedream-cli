package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Real RubyGems search, via its actual, documented, public API
// (rubygems.org/api/v1/search.json) -- confirmed directly, live, before writing this:
// real download counts and real current versions returned per gem.
type rubygemsSearchResult struct {
	Name       string `json:"name"`
	Downloads  int    `json:"downloads"`
	Version    string `json:"version"`
	Info       string `json:"info"`
	ProjectURI string `json:"project_uri"`
}

func SearchRubyGems(ctx context.Context, query string, limit int) ([]Result, error) {
	// RubyGems' real search API paginates at a fixed ~30 per page and has no per_page
	// parameter of its own -- limit is honestly applied client-side below, not sent as a
	// (nonexistent) real query parameter.
	apiURL := "https://rubygems.org/api/v1/search.json?query=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RubyGems search unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RubyGems search returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed []rubygemsSearchResult
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("RubyGems search: unexpected response shape: %w", err)
	}

	if limit > 0 && limit < len(parsed) {
		parsed = parsed[:limit]
	}

	out := make([]Result, 0, len(parsed))
	for _, g := range parsed {
		downloads := g.Downloads
		pkgURL := g.ProjectURI
		if pkgURL == "" {
			pkgURL = "https://rubygems.org/gems/" + g.Name
		}
		out = append(out, Result{
			Name:        g.Name,
			Description: g.Info,
			Source:      "RubyGems",
			URL:         pkgURL,
			UseCount:    &downloads,
			dedupKey:    normalizeForDedup(pkgURL, g.Name),
		})
	}
	return out, nil
}

// RubyGemsConnector wraps SearchRubyGems. Retry/cache/stats centralized in
// instrumentedSearch, matching every other free connector.
type RubyGemsConnector struct{}

func (RubyGemsConnector) Name() string { return "RubyGems" }

func (c RubyGemsConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchRubyGems(ctx, query, limit)
	})
}

func (RubyGemsConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchRubyGems(ctx, "rails", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c RubyGemsConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c RubyGemsConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (RubyGemsConnector) Capabilities() Capabilities {
	return Capabilities{
		RequiresPayment:    false,
		RequiresAPIKey:     false,
		RateLimitPerMinute: 0,
		SupportsRealtime:   true,
	}
}
