package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Real Maven Central search, via its actual, documented, public Solr-backed API
// (search.maven.org/solrsearch/select) -- confirmed directly, live, before writing this.
type mavenSearchResponse struct {
	Response struct {
		Docs []struct {
			ID            string `json:"id"` // "groupId:artifactId"
			G             string `json:"g"`
			A             string `json:"a"`
			LatestVersion string `json:"latestVersion"`
			Timestamp     int64  `json:"timestamp"` // real, epoch milliseconds of the latest real release
			VersionCount  int    `json:"versionCount"`
		} `json:"docs"`
	} `json:"response"`
}

func SearchMavenCentral(ctx context.Context, query string, limit int) ([]Result, error) {
	apiURL := "https://search.maven.org/solrsearch/select?q=" + url.QueryEscape(query) + "&rows=" + fmt.Sprint(limit) + "&wt=json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Maven Central search unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Maven Central search returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed mavenSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("Maven Central search: unexpected response shape: %w", err)
	}

	out := make([]Result, 0, len(parsed.Response.Docs))
	for _, d := range parsed.Response.Docs {
		versionCount := d.VersionCount
		pkgURL := "https://search.maven.org/artifact/" + d.G + "/" + d.A
		// Real, epoch-ms timestamp -> RFC3339, matching every other connector's
		// LastUpdated convention. Maven Central genuinely returns milliseconds.
		lastUpdated := ""
		if d.Timestamp > 0 {
			lastUpdated = time.UnixMilli(d.Timestamp).UTC().Format(time.RFC3339)
		}
		out = append(out, Result{
			Name:        d.ID, // "groupId:artifactId" -- Maven's real, canonical identifier; there is no single "name" field
			Description: "Maven artifact " + d.G + ":" + d.A + ", latest " + d.LatestVersion,
			Source:      "Maven Central",
			URL:         pkgURL,
			UseCount:    &versionCount, // real count of published versions -- not downloads (Maven Central's search API doesn't expose download counts)
			LastUpdated: lastUpdated,
			dedupKey:    normalizeForDedup(pkgURL, d.ID),
		})
	}
	return out, nil
}

// MavenCentralConnector wraps SearchMavenCentral. Retry/cache/stats centralized in
// instrumentedSearch, matching every other free connector.
type MavenCentralConnector struct{}

func (MavenCentralConnector) Name() string { return "Maven Central" }

func (c MavenCentralConnector) Search(ctx context.Context, query string, limit int) (Outcome, error) {
	return instrumentedSearch(c.Name(), query, limit, func() ([]Result, error) {
		return SearchMavenCentral(ctx, query, limit)
	})
}

func (MavenCentralConnector) Health(ctx context.Context) HealthStatus {
	_, err := SearchMavenCentral(ctx, "guava", 1)
	if err != nil {
		return HealthStatus{Healthy: false, Message: err.Error()}
	}
	return HealthStatus{Healthy: true, Message: "reachable"}
}

func (c MavenCentralConnector) Latency() LatencyInfo         { return latencyInfoFor(c.Name()) }
func (c MavenCentralConnector) Reliability() ReliabilityInfo { return reliabilityInfoFor(c.Name()) }

func (MavenCentralConnector) Capabilities() Capabilities {
	return Capabilities{
		RequiresPayment:    false,
		RequiresAPIKey:     false,
		RateLimitPerMinute: 0,
		SupportsRealtime:   true,
	}
}
