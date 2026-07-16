package discovery

import "time"

// Adaptive connector scheduling: before attempting a real Search() call, check whether
// this connector's own, locally-observed reliability suggests it's currently unhealthy
// enough to skip -- falling back to a cached result (even a stale one) rather than
// spending time on a call likely to fail. Deliberately reads only the already-tracked,
// local Reliability() data (no extra network call) -- a live Health() ping before every
// real search would add a round-trip per connector, working directly against the "make
// search faster" goal this exists to serve.
const (
	// unhealthySuccessRateThreshold: a connector with a real, recent success rate below
	// this is considered unhealthy enough to skip.
	unhealthySuccessRateThreshold = 0.3

	// minAttemptsBeforeJudging: don't judge a connector as unhealthy off a tiny sample --
	// a single real failure out of one attempt isn't a pattern yet.
	minAttemptsBeforeJudging = 3

	// forceRetryAfter: always attempt a real call anyway if it's been at least this long
	// since the last real attempt, regardless of historical reliability -- otherwise a
	// connector that's genuinely recovered would stay skipped forever, since skipping it
	// means it never gets a chance to update its own reliability data.
	forceRetryAfter = 10 * time.Minute
)

// shouldSkipConnector decides whether to skip a real Search() call for this connector,
// based only on its own real, locally-observed history -- returns false (never skip)
// whenever there isn't yet a real, sufficient sample to judge from.
func shouldSkipConnector(name string) bool {
	stats, ok := getStats(name)
	if !ok || stats.Attempts < minAttemptsBeforeJudging {
		return false
	}
	successRate := float64(stats.Successes) / float64(stats.Attempts)
	if successRate >= unhealthySuccessRateThreshold {
		return false
	}
	lastSearched, err := time.Parse(time.RFC3339, stats.LastSearchedAt)
	if err != nil {
		return false // can't parse when it last ran -- don't skip based on incomplete data
	}
	if time.Since(lastSearched) >= forceRetryAfter {
		return false // genuinely been long enough -- give it a fresh chance
	}
	return true
}
