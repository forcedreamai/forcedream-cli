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

// Real npm registry search, via npm's actual, documented, public search API
// (registry.npmjs.org/-/v1/search) -- confirmed directly, live, before writing this: no
// auth needed, real weekly-download counts and real last-published timestamps returned per
// package. Query is scoped with "mcp" appended, the same approach already used for
// GitHub's topic-based scoping, since this tool is specifically for discovering MCP
// servers/AI agents, not general npm packages.
type npmSearchResponse struct {
	Objects []struct {
		Downloads struct {
			Weekly int `json:"weekly"`
		} `json:"downloads"`
		Updated string `json:"updated"`
		Package struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Keywords    []string `json:"keywords"`
			Links       struct {
				NPM      string `json:"npm"`
				Homepage string `json:"homepage"`
			} `json:"links"`
		} `json:"package"`
	} `json:"objects"`
}

// SearchNpmMCPServers searches the real npm registry for packages matching the query,
// scoped to MCP-relevant results.
func SearchNpmMCPServers(ctx context.Context, query string, limit int) ([]Result, error) {
	q := strings.TrimSpace(query + " mcp")
	apiURL := "https://registry.npmjs.org/-/v1/search?text=" + url.QueryEscape(q) + "&size=" + fmt.Sprint(limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("npm search unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm search returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed npmSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("npm search: unexpected response shape: %w", err)
	}

	out := make([]Result, 0, len(parsed.Objects))
	for _, obj := range parsed.Objects {
		useCount := obj.Downloads.Weekly
		pkgURL := obj.Package.Links.NPM
		if pkgURL == "" {
			pkgURL = "https://www.npmjs.com/package/" + obj.Package.Name
		}
		out = append(out, Result{
			Name:        obj.Package.Name,
			Description: obj.Package.Description,
			Source:      "npm",
			URL:         pkgURL,
			Tags:        obj.Package.Keywords,
			UseCount:    &useCount,
			LastUpdated: obj.Updated,
			dedupKey:    normalizeForDedup(pkgURL, obj.Package.Name),
		})
	}
	return out, nil
}
