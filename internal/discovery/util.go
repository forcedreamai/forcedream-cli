package discovery

import (
	"os"
	"regexp"
	"strings"
)

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// normalizeForDedup builds a stable key for merging results that refer to the same real
// thing across different sources (e.g. the same GitHub repo appearing in both a direct
// GitHub search and an MCP Registry entry that links to it). Prefers a URL if present,
// since that's the most reliable identity signal across sources; falls back to a
// normalized name otherwise.
func normalizeForDedup(url, name string) string {
	basis := url
	if basis == "" {
		basis = name
	}
	basis = strings.ToLower(basis)
	basis = strings.TrimPrefix(basis, "https://")
	basis = strings.TrimPrefix(basis, "http://")
	basis = strings.TrimPrefix(basis, "www.")
	basis = strings.TrimSuffix(basis, "/")
	basis = strings.TrimSuffix(basis, ".git")
	return nonAlnum.ReplaceAllString(basis, "-")
}

func envOrEmpty(key string) string {
	return os.Getenv(key)
}
