package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Real, confirmed response shape from https://registry.modelcontextprotocol.io/v0/servers,
// fetched and inspected directly before writing this parser -- not guessed at. The registry
// is explicitly documented as "preview," with no data-durability guarantee, so this parser
// is deliberately defensive about missing/optional fields.
type mcpRegistryResponse struct {
	Servers []struct {
		Server struct {
			Name        string `json:"name"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Version     string `json:"version"`
			WebsiteURL  string `json:"websiteUrl"`
			Repository  struct {
				URL string `json:"url"`
			} `json:"repository"`
			Remotes []struct {
				URL string `json:"url"`
			} `json:"remotes"`
		} `json:"server"`
		Meta struct {
			Official struct {
				Status   string `json:"status"`
				IsLatest bool   `json:"isLatest"`
			} `json:"io.modelcontextprotocol.registry/official"`
		} `json:"_meta"`
	} `json:"servers"`
	Metadata struct {
		NextCursor string `json:"nextCursor"`
		Count      int    `json:"count"`
	} `json:"metadata"`
}

// SearchMCPRegistry queries the real, official MCP Registry. Public, no auth required.
// Filters to isLatest + active only, since the registry returns every historical version
// of every server (confirmed directly: the same server name can appear many times).
//
// Paginates through the registry's real cursor-based pagination up to maxPages, since the
// API has no server-side search/query parameter (confirmed directly) and results are
// namespace-sorted, not relevance-sorted -- checking only the first page would silently
// miss almost every real match for anything other than names starting with "a".
func SearchMCPRegistry(ctx context.Context, query string, limit int) ([]Result, error) {
	// Confirmed live, three times, on a real machine: even 3 sequential pages with a 30s
	// budget didn't reliably complete. The real registry API (explicitly "preview" status)
	// appears to be genuinely, severely slow per-page in practice, not just a page-count
	// tuning problem -- widening the timeout and shrinking the page count further didn't
	// fix it. Rather than keep chasing a multi-page approach that keeps failing in
	// practice, this is now a single, bounded request: less coverage, but a source that
	// reliably returns real results instead of one that usually times out reaching for more.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	const maxPages = 1
	const pageSize = 100

	var out []Result
	cursor := ""
	for page := 0; page < maxPages; page++ {
		url := fmt.Sprintf("https://registry.modelcontextprotocol.io/v0/servers?limit=%d", pageSize)
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return out, err
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return out, fmt.Errorf("MCP Registry unreachable: %w", err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return out, err
		}
		if res.StatusCode != http.StatusOK {
			return out, fmt.Errorf("MCP Registry returned HTTP %d", res.StatusCode)
		}

		var parsed mcpRegistryResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return out, fmt.Errorf("MCP Registry: unexpected response shape: %w", err)
		}

		for _, entry := range parsed.Servers {
			if !entry.Meta.Official.IsLatest || entry.Meta.Official.Status == "deprecated" {
				continue
			}
			s := entry.Server
			if query != "" && !containsFold(s.Name, query) && !containsFold(s.Title, query) && !containsFold(s.Description, query) {
				continue
			}
			entryURL := s.WebsiteURL
			if entryURL == "" && s.Repository.URL != "" {
				entryURL = s.Repository.URL
			}
			if entryURL == "" && len(s.Remotes) > 0 {
				entryURL = s.Remotes[0].URL
			}
			name := s.Title
			if name == "" {
				name = s.Name
			}
			out = append(out, Result{
				Name:        name,
				Description: s.Description,
				Source:      "MCP Registry",
				URL:         entryURL,
				dedupKey:    normalizeForDedup(s.Repository.URL, name),
			})
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}

		if parsed.Metadata.NextCursor == "" {
			break
		}
		cursor = parsed.Metadata.NextCursor
	}
	return out, nil
}
