package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
)

// Version is this subsystem's own version, independent of the CLI's overall version.
const Version = "1.0.0"

// DefaultMergeThreshold: results scoring below this are never merged into the same
// entity -- configurable per the "configurable merge thresholds" requirement, this is
// only the shipped default, not a hardcoded rule Resolve enforces internally.
const DefaultMergeThreshold = 70.0

// SourceRef records exactly which real result (from which real source) contributed to an
// entity -- so a merge decision is always traceable back to its real, original inputs.
type SourceRef struct {
	Source string `json:"source"`
	Name   string `json:"name"`
	URL    string `json:"url"`
}

// Entity is a canonical, resolved identity -- potentially built from several real results
// across different sources that were judged, with a real, computed and explainable
// confidence, to refer to the same underlying thing.
type Entity struct {
	ID            string        `json:"id"`
	CanonicalName string        `json:"canonical_name"`
	Aliases       []string      `json:"aliases,omitempty"`
	URLs          []string      `json:"urls,omitempty"`
	Organization  string        `json:"organization,omitempty"`
	// Merged is false for a standalone result that didn't match anything else strongly
	// enough to merge -- Confidence is only meaningful when Merged is true; a singleton
	// isn't "100% confident", there was simply no merge decision to be confident about.
	Merged        bool          `json:"merged"`
	Confidence    float64       `json:"confidence,omitempty"`
	MatchReasons  []MatchReason `json:"match_reasons,omitempty"`
	Sources       []SourceRef   `json:"sources"`
}

// Resolve groups results into canonical entities. Any two results whose MatchScore meets
// or exceeds threshold are merged; matches are transitive (if A merges with B, and B
// merges with C, all three become one entity) via union-find, a real, standard algorithm
// for exactly this kind of grouping problem -- not an ad-hoc, one-off implementation.
func Resolve(results []discovery.Result, threshold float64) []Entity {
	return resolveCore(results, threshold, nil)
}

// ResolveWithOverrides is Resolve, but consulting real, persisted manual override
// decisions first: a "merge" override forces two specific results together regardless of
// their automatic MatchScore; a "split" override prevents a merge even if the automatic
// score would otherwise qualify. Every resulting entity is also permanently persisted
// (PersistEntities) -- "persist every decision", not just the immediate result of this run.
func ResolveWithOverrides(results []discovery.Result, threshold float64) []Entity {
	overrides := LoadOverrides()
	forceDecision := func(a, b discovery.Result) (OverrideDecision, bool) {
		keyA, keyB := referenceKey(a), referenceKey(b)
		for _, o := range overrides {
			if (o.KeyA == keyA && o.KeyB == keyB) || (o.KeyA == keyB && o.KeyB == keyA) {
				return o.Decision, true
			}
		}
		return "", false
	}
	entities := resolveCore(results, threshold, forceDecision)
	PersistEntities(entities)
	return entities
}

// resolveCore is the real, shared union-find grouping logic both Resolve and
// ResolveWithOverrides use -- forceDecision, when non-nil, lets a caller override the
// automatic MatchScore-based union/skip decision for a specific pair.
func resolveCore(results []discovery.Result, threshold float64, forceDecision func(a, b discovery.Result) (OverrideDecision, bool)) []Entity {
	n := len(results)
	if n == 0 {
		return nil
	}

	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(i, j int) {
		ri, rj := find(i), find(j)
		if ri != rj {
			parent[ri] = rj
		}
	}

	type pairMatch struct {
		i, j   int
		score  float64
		reason MatchReason
	}
	var matches []pairMatch
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if forceDecision != nil {
				if decision, ok := forceDecision(results[i], results[j]); ok {
					if decision == OverrideMerge {
						union(i, j)
						matches = append(matches, pairMatch{i, j, 100, "manual_override_merge"})
					}
					// OverrideSplit: deliberately skip the automatic score entirely for
					// this pair, whatever it would have been -- a real, forced non-merge.
					continue
				}
			}
			score, reason := MatchScore(results[i], results[j])
			if score >= threshold {
				union(i, j)
				matches = append(matches, pairMatch{i, j, score, reason})
			}
		}
	}

	groups := map[int][]int{}
	for i := 0; i < n; i++ {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	// Lowest pairwise score within each group -- a merge chain is only as confident as its
	// weakest real link, not its strongest.
	lowestScore := map[int]float64{}
	reasonsByRoot := map[int]map[MatchReason]bool{}
	for _, m := range matches {
		root := find(m.i)
		if cur, ok := lowestScore[root]; !ok || m.score < cur {
			lowestScore[root] = m.score
		}
		if reasonsByRoot[root] == nil {
			reasonsByRoot[root] = map[MatchReason]bool{}
		}
		reasonsByRoot[root][m.reason] = true
	}

	var entities []Entity
	for root, idxs := range groups {
		e := buildEntity(results, idxs, root, lowestScore, reasonsByRoot)
		entities = append(entities, e)
	}

	// Deterministic ordering (by canonical name) -- the grouping itself doesn't depend on
	// map iteration order (union-find + explicit lowestScore/reasonsByRoot tracking
	// already fixed that), but the returned slice's order should still be stable across
	// runs for the same input, not vary with Go's randomized map iteration.
	sort.Slice(entities, func(i, j int) bool { return entities[i].CanonicalName < entities[j].CanonicalName })
	return entities
}

func buildEntity(results []discovery.Result, idxs []int, root int, lowestScore map[int]float64, reasonsByRoot map[int]map[MatchReason]bool) Entity {
	aliasSet := map[string]bool{}
	urlSet := map[string]bool{}
	var sources []SourceRef
	var org string

	for _, idx := range idxs {
		r := results[idx]
		aliasSet[r.Name] = true
		if r.URL != "" {
			urlSet[r.URL] = true
		}
		sources = append(sources, SourceRef{Source: r.Source, Name: r.Name, URL: r.URL})
		if o := extractOrganization(r.Name, r.URL); o != "" && org == "" {
			org = o
		}
	}

	canonicalName := chooseCanonicalName(results, idxs)

	aliases := make([]string, 0, len(aliasSet))
	for a := range aliasSet {
		if a != canonicalName {
			aliases = append(aliases, a)
		}
	}
	sort.Strings(aliases)

	urls := make([]string, 0, len(urlSet))
	for u := range urlSet {
		urls = append(urls, u)
	}
	sort.Strings(urls)

	merged := len(idxs) > 1
	var confidence float64
	var reasons []MatchReason
	if merged {
		confidence = lowestScore[root]
		for r := range reasonsByRoot[root] {
			reasons = append(reasons, r)
		}
		sort.Slice(reasons, func(i, j int) bool { return reasons[i] < reasons[j] })
	}

	return Entity{
		ID:            deriveEntityID(sources),
		CanonicalName: canonicalName,
		Aliases:       aliases,
		URLs:          urls,
		Organization:  org,
		Merged:        merged,
		Confidence:    confidence,
		MatchReasons:  reasons,
		Sources:       sources,
	}
}

// chooseCanonicalName picks the name representing this merged entity: ForceDream's own
// name if present in the group (cryptographically verified, the platform's own primary
// source), otherwise the longest name seen (a simple, documented heuristic for "most
// complete/descriptive form" -- not a claim of definitive correctness).
func chooseCanonicalName(results []discovery.Result, idxs []int) string {
	for _, idx := range idxs {
		if results[idx].Source == "ForceDream" {
			return results[idx].Name
		}
	}
	longest := results[idxs[0]].Name
	for _, idx := range idxs[1:] {
		if len(results[idx].Name) > len(longest) {
			longest = results[idx].Name
		}
	}
	return longest
}

// deriveEntityID produces a stable, deterministic ID from this entity's real source
// references -- the same underlying sources always produce the same ID, so re-running
// resolution on the same data (e.g. a later search returning the same results) doesn't
// mint a new, different ID for something already resolved before.
func deriveEntityID(sources []SourceRef) string {
	keys := make([]string, 0, len(sources))
	for _, s := range sources {
		keys = append(keys, s.Source+"|"+s.Name+"|"+s.URL)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
	}
	return "ent_" + hex.EncodeToString(h.Sum(nil))[:16]
}
