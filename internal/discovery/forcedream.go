package discovery

import (
	"context"

	forcedream "github.com/forcedreamai/forcedream-sdk-go"
)

// SearchForceDream searches the real ForceDream agent marketplace, using the real,
// published Go SDK -- not reimplemented here, to avoid two copies of the same logic
// drifting apart over time.
func SearchForceDream(ctx context.Context, query string) ([]Result, error) {
	res, err := forcedream.SearchAgents(ctx, forcedream.DefaultAPIBase, "", query)
	if err != nil {
		return nil, err
	}

	out := make([]Result, 0, len(res.Agents))
	for _, a := range res.Agents {
		price := a.PricePerCallPence
		verified := false
		if pc, ok := a.Metrics["proof_count"]; ok {
			if n, ok := pc.(float64); ok && n > 0 {
				verified = true
			}
		}
		out = append(out, Result{
			Name:          a.Name,
			Description:   a.Description,
			Source:        "ForceDream",
			URL:           "https://forcedream.com/marketplace/agents/" + a.Slug,
			PricePence:    &price,
			Verified:      verified,
			Tags:          a.Capabilities,
			InvokeCommand: "forcedream invoke " + a.Slug + " \"<your task>\"",
			dedupKey:      normalizeForDedup("", "forcedream:"+a.Slug),
		})
	}
	return out, nil
}
