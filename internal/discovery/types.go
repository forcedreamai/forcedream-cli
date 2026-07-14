package discovery

// Result is the unified shape every search source produces, so the CLI can merge, rank,
// and dedupe across genuinely different APIs with different native schemas.
type Result struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Source        string   `json:"source"` // "ForceDream", "MCP Registry", "GitHub", "Smithery", "Web"
	URL           string   `json:"url"`
	PricePence    *int     `json:"price_pence,omitempty"`
	Verified      bool     `json:"cryptographically_verified"`
	Tags          []string `json:"tags,omitempty"`
	Stars         *int     `json:"github_stars,omitempty"`
	UseCount      *int     `json:"smithery_use_count,omitempty"`
	LastUpdated   string   `json:"last_updated,omitempty"`
	InvokeCommand string   `json:"invoke_command,omitempty"`

	// dedupKey is used internally to merge results referring to the same real thing across
	// sources (e.g. the same GitHub repo appearing in both a GitHub search and an MCP
	// Registry entry). Not serialized.
	dedupKey string
}
