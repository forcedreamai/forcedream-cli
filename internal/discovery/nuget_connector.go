package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Real NuGet search, via its actual, documented V3 API. Per NuGet's own official docs
// (learn.microsoft.com/en-us/nuget/api/search-query-service-resource), the search URL is
// NOT fixed -- it must be dynamically discovered from the real service index
// (api.nuget.org/v3/index.json) by finding the resource whose @type is
// "SearchQueryService". Confirmed directly, live, before writing this: the real, current
// URL (azuresearch-usnc.nuget.org) differs from older, still-circulating documentation
// examples (api-v2v3search-0.nuget.org) -- exactly the scenario NuGet's own docs warn
// about, and exactly why this isn't hardcoded.
type nugetServiceIndex struct {
	Resources []struct {
		ID   string `json:"@id"`
		Type string `json:"@type"`
	} `json:"resources"`
}

type nugetSearchResponse struct {
	Data []struct {
		ID          string `json:"id"`
		Version     string `json:"version"`
		Description string `json:"description"`
	} `json:"data"`
}

var (
	nugetSearchURLCache string
	nugetSearchURLMu    sync.Mutex
)

// nugetSearchServiceURL fetches and caches the real, current SearchQueryService URL from
// NuGet's real service index, per the officially documented discovery process.
func nugetSearchServiceURL(ctx context.Context) (string, error) {
	nugetSearchURLMu.Lock()
	defer nugetSearchURLMu.Unlock()
	if nugetSearchURLCache != "" {
		return nugetSearchURLCache, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.nuget.org/v3/index.json", nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("NuGet service index unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("NuGet service index returned HTTP %d", res.StatusCode)
	}

	var idx nugetServiceIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return "", fmt.Errorf("NuGet service index: unexpected response shape: %w", err)
	}
	for _, r := range idx.Resources {
		if r.Type == "SearchQueryService" {
			nugetSearchURLCache = r.ID
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("NuGet service index: no SearchQueryService resource found")
}

func SearchNuGet(ctx context.Context, query string, limit int) ([]Result, error) {
	searchBase, err := nugetSearchServiceURL(ctx)
	if err != nil {
		return nil, err
	}
	apiURL := searchBase + "?q=" + url.QueryEscape(query) + "&take=" + fmt.Sprint(limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("NuGet search unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NuGet search returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed nugetSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("NuGet search: unexpected response shape: %w", err)
	}

	out := make([]Result, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		pkgURL := "https://www.nuget.org/packages/" + d.ID
		out = append(out, Result{
			Name:        d.ID,
			Description: d.Description,
			Source:      "NuGet",
			URL:         pkgURL,
			dedupKey:    normalizeForDedup(pkgURL, d.ID),
		})
	}
	return out, nil
}

// NuGetConnector wraps SearchNuGet. Retry/cache/stats centralized in instrumentedSearch,
// matching every other free connector.
type NuGetConnector struct{}

func (NuGetConnector) Name() string { return "NuGet" }

func (c NuGetConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchNuGet(ctx, query, limit)
	})
}

func (NuGetConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchNuGet(ctx, "json", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c NuGetConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c NuGetConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (NuGetConnector) Capabilities() Capabilities {
	return Capabilities{
		RequiresPayment:    false,
		RequiresAPIKey:     false,
		RateLimitPerMinute: 0,
		SupportsRealtime:   true,
	}
}
