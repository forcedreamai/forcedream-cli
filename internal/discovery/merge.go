package discovery

// Merge deduplicates results referring to the same real thing across sources (keeping the
// entry with the most complete data). Ranking is a deliberately separate concern, now owned
// entirely by the standalone internal/ranking package (see that package's own docs) --
// this function no longer sorts by score at all, matching the architectural-independence
// requirement that ranking not be embedded inside discovery's own merge step. Callers
// dedupe here, then rank separately: ranking.Rank(discovery.Merge(results), weights).
func Merge(results []Result) []Result {
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
