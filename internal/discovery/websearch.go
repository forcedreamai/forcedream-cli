package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type webProxyResponse struct {
	Data struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	} `json:"data"`
}

// SearchWeb calls the real ForceDream backend proxy for open-web search (SerpAPI, real key
// server-side only). Same gating -- real ForceDream fd_live_ account, balance,
// entitlement -- as Smithery, using the same FD_LIVE_KEY.
func SearchWeb(ctx context.Context, query string) ([]Result, PaidSourceStatus, error) {
	key := fdLiveKeyForSearch()
	if key == "" {
		return nil, PaidSourceStatus{Available: false, Reason: "auth_required", Message: "Set FD_LIVE_KEY (a real fd_live_... ForceDream billing key) to use open-web search."}, nil
	}

	apiURL := "https://api.forcedream.ai/v1/search/web-proxy?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, PaidSourceStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, PaidSourceStatus{}, fmt.Errorf("web-search proxy unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, PaidSourceStatus{}, err
	}

	if res.StatusCode != http.StatusOK {
		var errResp proxyErrorResponse
		_ = json.Unmarshal(body, &errResp)
		return nil, PaidSourceStatus{Available: false, Reason: errResp.Error, Message: errResp.Message}, nil
	}

	var parsed webProxyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, PaidSourceStatus{}, fmt.Errorf("web-search proxy: unexpected response shape: %w", err)
	}

	out := make([]Result, 0, len(parsed.Data.OrganicResults))
	for _, r := range parsed.Data.OrganicResults {
		out = append(out, Result{
			Name:        r.Title,
			Description: r.Snippet,
			Source:      "Web",
			URL:         r.Link,
			dedupKey:    normalizeForDedup(r.Link, r.Title),
		})
	}
	return out, PaidSourceStatus{Available: true}, nil
}
