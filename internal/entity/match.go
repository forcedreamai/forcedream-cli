// Package entity is a standalone subsystem, architecturally independent from the
// discovery/ranking packages it resolves results from: its own data model, its own
// matching engine, its own persistent storage, its own tests. Integrates with discovery
// only through the shared discovery.Result type -- no shared internal state, no reaching
// into discovery's own internals.
//
// This is real, mechanical identity resolution: URL normalization, organization
// extraction, and Levenshtein-based name similarity -- all genuinely computable,
// deterministic, and explainable. It is deliberately NOT semantic/embeddings-based
// README or description matching; that would need a real ML model call (a new
// infrastructure dependency), and claiming to do it with a hand-rolled heuristic instead
// would be dishonest about what the resulting confidence numbers actually mean.
package entity

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
)

// MatchReason is a fixed, machine-readable signal name -- used so a confidence score is
// always explainable (which real signals contributed), never an opaque single number.
type MatchReason string

const (
	ReasonExactURL      MatchReason = "exact_url"
	ReasonSameOrgExactName MatchReason = "same_org_exact_name"
	ReasonSameOrgSimilarName MatchReason = "same_org_similar_name"
	ReasonSameOrgOnly   MatchReason = "same_org_only"
	ReasonExactNameNoOrg MatchReason = "exact_name_no_org_signal"
	ReasonSimilarNameNoOrg MatchReason = "similar_name_no_org_signal"
)

// Real, transparent confidence values per signal -- a fixed, documented scale (0-100),
// not a fabricated or opaque score. Strongest signal (identical URL, the most reliable
// identity indicator available) scores highest; name-similarity alone with no
// organization corroboration scores lowest, since generic names are the weakest,
// most collision-prone signal.
const (
	scoreExactURL            = 100.0
	scoreSameOrgExactName    = 90.0
	scoreSameOrgSimilarName  = 75.0
	scoreSameOrgOnly         = 50.0
	scoreExactNameNoOrg      = 60.0
	scoreSimilarNameNoOrg    = 35.0
	// similarNameThreshold: normalized names must be at least this similar (0-1 ratio,
	// via Levenshtein distance) to count as "similar" rather than "no match at all".
	similarNameThreshold = 0.8
)

var nonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

// normalizeName lowercases and strips everything but letters/digits, so
// "forcedream-mcp", "ForceDream MCP", and "forcedream_mcp" all normalize identically.
func normalizeName(name string) string {
	return nonAlnumRe.ReplaceAllString(strings.ToLower(name), "")
}

// normalizeURL strips protocol/www/trailing-slash/.git, matching discovery's own
// normalizeForDedup approach, so two URLs that are the same real address but written
// slightly differently both resolve to the same comparable string.
func normalizeURL(u string) string {
	lower := strings.ToLower(strings.TrimSpace(u))
	lower = strings.TrimPrefix(lower, "https://")
	lower = strings.TrimPrefix(lower, "http://")
	lower = strings.TrimPrefix(lower, "www.")
	lower = strings.TrimSuffix(lower, "/")
	lower = strings.TrimSuffix(lower, ".git")
	return lower
}

// extractOrganization pulls a real organization/namespace out of a URL or name where one
// is mechanically identifiable -- GitHub's github.com/{org}/{repo} path, or an npm-style
// scoped package name "@{org}/{package}". Returns "" (no signal) rather than guessing when
// neither pattern applies -- an absent org must never be treated as a matching one.
func extractOrganization(name, rawURL string) string {
	if strings.HasPrefix(name, "@") {
		if slash := strings.Index(name, "/"); slash > 1 {
			return strings.ToLower(strings.TrimPrefix(name[:slash], "@"))
		}
	}
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if strings.Contains(parsed.Host, "github.com") {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 1 && parts[0] != "" {
			return strings.ToLower(parts[0])
		}
	}
	return ""
}

// levenshteinRatio returns a real, computed similarity ratio (1.0 = identical, 0.0 =
// completely different) based on Levenshtein edit distance -- a standard, deterministic
// string-similarity algorithm, not a fabricated or ML-derived score.
func levenshteinRatio(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	dist := levenshteinDistance(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	return 1.0 - float64(dist)/float64(maxLen)
}

func levenshteinDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

// MatchScore computes a real, explainable confidence score (0-100) that a and b refer to
// the same real entity, along with the specific reason -- never an opaque number with no
// stated basis.
func MatchScore(a, b discovery.Result) (score float64, reason MatchReason) {
	normURLA, normURLB := normalizeURL(a.URL), normalizeURL(b.URL)
	if normURLA != "" && normURLA == normURLB {
		return scoreExactURL, ReasonExactURL
	}

	orgA := extractOrganization(a.Name, a.URL)
	orgB := extractOrganization(b.Name, b.URL)
	normNameA, normNameB := normalizeName(a.Name), normalizeName(b.Name)

	if orgA != "" && orgA == orgB {
		if normNameA == normNameB {
			return scoreSameOrgExactName, ReasonSameOrgExactName
		}
		if ratio := levenshteinRatio(normNameA, normNameB); ratio >= similarNameThreshold {
			return scoreSameOrgSimilarName, ReasonSameOrgSimilarName
		}
		return scoreSameOrgOnly, ReasonSameOrgOnly
	}

	// No shared organization signal -- name-based matching alone, the weakest, most
	// collision-prone signal, scored accordingly.
	if normNameA == normNameB && normNameA != "" {
		return scoreExactNameNoOrg, ReasonExactNameNoOrg
	}
	if ratio := levenshteinRatio(normNameA, normNameB); ratio >= similarNameThreshold {
		return scoreSimilarNameNoOrg, ReasonSimilarNameNoOrg
	}

	return 0, ""
}
