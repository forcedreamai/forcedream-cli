package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// PaidSourceStatus captures the specific gating/quota state for a paid source, so the CLI
// can report exactly why a source didn't return results -- auth_required,
// insufficient_funds, feature_not_enabled, quota_exceeded -- rather than a generic failure.
// These are the real, exact error codes the backend returns; kept as plain strings here so
// the caller can match on them directly.
type PaidSourceStatus struct {
	Available bool
	Reason    string
	Message   string
}

// fdLiveKey reads the same fd_live_... billing key the CLI already uses for invoke.
// Earlier version of this file used a separate FD_ACCOUNT_KEY (sk_fd_... format), matching
// what was then an incorrectly-keyed backend gate against the EARNINGS ledger; the backend
// now correctly gates against the real, established fd_live_ billing substrate (the same
// one invoke already uses), so this reuses the same credential -- no second key needed.
func fdLiveKeyForSearch() string {
	return os.Getenv("FD_LIVE_KEY")
}

type smitheryProxyResponse struct {
	Data struct {
		Servers []struct {
			DisplayName   string `json:"displayName"`
			QualifiedName string `json:"qualifiedName"`
			Description   string `json:"description"`
			Homepage      string `json:"homepage"`
			Verified      bool   `json:"verified"`
			UseCount      int    `json:"useCount"`
		} `json:"servers"`
	} `json:"data"`
}

type proxyErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SearchSmithery calls the real ForceDream backend proxy (never Smithery directly -- the
// real key lives only server-side). Gated behind a real ForceDream account, balance, and
// entitlement; returns a PaidSourceStatus explaining exactly why, rather than a generic
// failure, whenever the gate rejects the request.
func SearchSmithery(ctx context.Context, query string) ([]Result, PaidSourceStatus, error) {
	key := fdLiveKeyForSearch()
	if key == "" {
		return nil, PaidSourceStatus{Available: false, Reason: "auth_required", Message: "Set FD_LIVE_KEY (a real fd_live_... ForceDream billing key) to use Smithery search."}, nil
	}

	apiURL := "https://api.forcedream.ai/v1/search/smithery-proxy?q=" + url.QueryEscape(query) + "&pageSize=10"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, PaidSourceStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, PaidSourceStatus{}, fmt.Errorf("Smithery proxy unreachable: %w", err)
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

	var parsed smitheryProxyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, PaidSourceStatus{}, fmt.Errorf("Smithery proxy: unexpected response shape: %w", err)
	}

	out := make([]Result, 0, len(parsed.Data.Servers))
	for _, s := range parsed.Data.Servers {
		name := s.DisplayName
		if name == "" {
			name = s.QualifiedName
		}
		useCount := s.UseCount
		out = append(out, Result{
			Name:        name,
			Description: s.Description,
			Source:      "Smithery",
			URL:         s.Homepage,
			Verified:    s.Verified,
			UseCount:    &useCount,
			dedupKey:    normalizeForDedup(s.Homepage, "smithery:"+s.QualifiedName),
		})
	}
	return out, PaidSourceStatus{Available: true}, nil
}
