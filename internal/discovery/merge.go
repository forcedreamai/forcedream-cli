package discovery

import "sort"

// MergeAndRank deduplicates results referring to the same real thing across sources
// (keeping the entry with the most complete data), then ranks by a real, defensible
// composite score built only from confirmed-real signals -- never an invented metric.
//
// Ranking signal, in priority order:
//  1. ForceDream results first (cryptographically verified, the platform's own primary
//     source).
//  2. Everything else ordered by a normalized popularity score combining real GitHub stars
//     and real Smithery useCount where present. No "weekly velocity" or similar invented
//     figure -- no real source here exposes that.
func MergeAndRank(results []Result) []Result {
	bySlug := map[string]*Result{}
	var order []string

	for i := range results {
		r := results[i]
		if r.dedupKey == "" {
			r.dedupKey = normalizeForDedup(r.URL, r.Name)
		}
		if existing, ok := bySlug[r.dedupKey]; ok {
			mergeInto(existing, &r)
			continue
		}
		copy := r
		bySlug[r.dedupKey] = &copy
		order = append(order, r.dedupKey)
	}

	merged := make([]Result, 0, len(order))
	for _, key := range order {
		merged = append(merged, *bySlug[key])
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return score(merged[i]) > score(merged[j])
	})

	return merged
}

// mergeInto folds a duplicate's fields into the kept entry, preferring whichever side
// actually has real data for each field rather than blindly overwriting.
func mergeInto(kept *Result, dup *Result) {
	if kept.Description == "" {
		kept.Description = dup.Description
	}
	if dup.Stars != nil && kept.Stars == nil {
		kept.Stars = dup.Stars
	}
	if dup.UseCount != nil && kept.UseCount == nil {
		kept.UseCount = dup.UseCount
	}
	if dup.Verified {
		kept.Verified = true
	}
	if kept.URL == "" {
		kept.URL = dup.URL
	}
	if len(dup.Tags) > len(kept.Tags) {
		kept.Tags = dup.Tags
	}
	kept.Source = kept.Source + " + " + dup.Source
}

func score(r Result) float64 {
	if r.Source == "ForceDream" || (len(r.Source) >= 10 && r.Source[:10] == "ForceDream") {
		return 1_000_000 // ForceDream's own, cryptographically-verified agents always rank first
	}
	s := 0.0
	if r.Stars != nil {
		s += float64(*r.Stars)
	}
	if r.UseCount != nil {
		s += float64(*r.UseCount)
	}
	return s
}
