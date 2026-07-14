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

// Real GitHub code/repo search, via GitHub's actual, documented Search API
// (https://docs.github.com/en/rest/search). No auth required for basic use, though
// unauthenticated requests are rate-limited to 10/min -- callers should serialize or
// space out repeated searches accordingly.
type githubSearchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		FullName    string   `json:"full_name"`
		Description string   `json:"description"`
		HTMLURL     string   `json:"html_url"`
		Stargazers  int      `json:"stargazers_count"`
		Topics      []string `json:"topics"`
		PushedAt    string   `json:"pushed_at"`
	} `json:"items"`
}

// SearchGitHubMCPServers searches GitHub repositories for real MCP servers matching the
// query. Uses the real repository search endpoint, scoped with "mcp-server" or "mcp" in
// the topics/description, combined with the user's query.
func SearchGitHubMCPServers(ctx context.Context, query string, limit int) ([]Result, error) {
	q := fmt.Sprintf("%s topic:mcp-server", query)
	if query == "" {
		q = "topic:mcp-server"
	}
	apiURL := "https://api.github.com/search/repositories?q=" + url.QueryEscape(q) + "&sort=stars&order=desc&per_page=" + fmt.Sprint(limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := githubTokenFromEnv(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub search unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub search returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed githubSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("GitHub search: unexpected response shape: %w", err)
	}

	out := make([]Result, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		stars := item.Stargazers
		out = append(out, Result{
			Name:        item.FullName,
			Description: item.Description,
			Source:      "GitHub",
			URL:         item.HTMLURL,
			Tags:        item.Topics,
			Stars:       &stars,
			LastUpdated: item.PushedAt,
			dedupKey:    normalizeForDedup(item.HTMLURL, item.FullName),
		})
	}
	return out, nil
}

// githubTokenFromEnv is optional -- GitHub search works unauthenticated (10 req/min limit),
// but a token (GITHUB_TOKEN) raises that to 30 req/min if the user happens to have one set.
// Not a paid API, so this doesn't need the same billing-safety treatment as Smithery/SerpAPI.
func githubTokenFromEnv() string {
	return envOrEmpty("GITHUB_TOKEN")
}
