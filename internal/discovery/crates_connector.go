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

// Real crates.io search, via its actual, documented, public API (crates.io/api/v1/crates)
// -- confirmed directly, live, before writing this: real download counts and real
// newest-version strings returned per crate. crates.io's own Crawler Policy requires a
// real, identifying User-Agent on every request -- respected here, not skipped.
type cratesSearchResponse struct {
	Crates []struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Downloads     int    `json:"downloads"`
		NewestVersion string `json:"newest_version"`
		Documentation string `json:"documentation"`
		Repository    string `json:"repository"`
		UpdatedAt     string `json:"updated_at"`
	} `json:"crates"`
}

func SearchCratesIO(ctx context.Context, query string, limit int) ([]Result, error) {
	apiURL := "https://crates.io/api/v1/crates?q=" + url.QueryEscape(query) + "&per_page=" + fmt.Sprint(limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	// Required by crates.io's real Crawler Policy -- a real, identifying user agent, not
	// a generic/default one.
	req.Header.Set("User-Agent", "forcedream-cli (https://forcedream.ai; contact: support@forcedream.ai)")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crates.io search unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crates.io search returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed cratesSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("crates.io search: unexpected response shape: %w", err)
	}

	out := make([]Result, 0, len(parsed.Crates))
	for _, c := range parsed.Crates {
		downloads := c.Downloads
		pkgURL := "https://crates.io/crates/" + c.Name
		out = append(out, Result{
			Name:        c.Name,
			Description: c.Description,
			Source:      "crates.io",
			URL:         pkgURL,
			UseCount:    &downloads,
			LastUpdated: c.UpdatedAt,
			dedupKey:    normalizeForDedup(pkgURL, c.Name),
		})
	}
	return out, nil
}

// CratesIOConnector wraps SearchCratesIO. Retry/cache/stats centralized in
// instrumentedSearch, matching every other free connector.
type CratesIOConnector struct{}

func (CratesIOConnector) Name() string { return "crates.io" }

func (c CratesIOConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchCratesIO(ctx, query, limit)
	})
}

func (CratesIOConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchCratesIO(ctx, "serde", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c CratesIOConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c CratesIOConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (CratesIOConnector) Capabilities() Capabilities {
	return Capabilities{
		RequiresPayment:    false,
		RequiresAPIKey:     false,
		RateLimitPerMinute: 0, // crates.io does not document a specific per-minute limit, only a real User-Agent + reasonable-use requirement
		SupportsRealtime:   true,
	}
}
