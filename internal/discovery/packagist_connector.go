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

// Real Packagist search, via its actual, documented, public API (packagist.org/search.json)
// -- confirmed directly, live, before writing this: real download counts and real
// "favers" (stars-equivalent) counts returned per package.
type packagistSearchResponse struct {
	Results []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
		Repository  string `json:"repository"`
		Downloads   int    `json:"downloads"`
		Favers      int    `json:"favers"`
	} `json:"results"`
}

func SearchPackagist(ctx context.Context, query string, limit int) ([]Result, error) {
	apiURL := "https://packagist.org/search.json?q=" + url.QueryEscape(query) + "&per_page=" + fmt.Sprint(limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Packagist search unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Packagist search returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed packagistSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("Packagist search: unexpected response shape: %w", err)
	}

	out := make([]Result, 0, len(parsed.Results))
	for _, p := range parsed.Results {
		downloads := p.Downloads
		stars := p.Favers
		pkgURL := p.URL
		if pkgURL == "" {
			pkgURL = "https://packagist.org/packages/" + p.Name
		}
		out = append(out, Result{
			Name:        p.Name,
			Description: p.Description,
			Source:      "Packagist",
			URL:         pkgURL,
			UseCount:    &downloads,
			Stars:       &stars,
			dedupKey:    normalizeForDedup(pkgURL, p.Name),
		})
	}
	return out, nil
}

// PackagistConnector wraps SearchPackagist. Retry/cache/stats centralized in
// instrumentedSearch, matching every other free connector.
type PackagistConnector struct{}

func (PackagistConnector) Name() string { return "Packagist" }

func (c PackagistConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchPackagist(ctx, query, limit)
	})
}

func (PackagistConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchPackagist(ctx, "monolog", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c PackagistConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c PackagistConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (PackagistConnector) Capabilities() Capabilities {
	return Capabilities{
		RequiresPayment:    false,
		RequiresAPIKey:     false,
		RateLimitPerMinute: 0,
		SupportsRealtime:   true,
	}
}
