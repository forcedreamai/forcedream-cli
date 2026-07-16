package discovery

import (
	"path/filepath"
	"testing"
)

// resetStatsForTest redirects storage to a fresh, temporary file and clears the in-memory
// cache, so each test starts from real, known-empty state rather than whatever the real
// machine's actual connector_stats.json happens to contain.
func resetStatsForTest(t *testing.T) {
	t.Helper()
	statsFilePathOverride = filepath.Join(t.TempDir(), "connector_stats_test.json")
	statsCache = nil
	t.Cleanup(func() {
		statsFilePathOverride = ""
		statsCache = nil
	})
}

func TestGetStatsHonestlyReportsNoDataBeforeAnySearch(t *testing.T) {
	resetStatsForTest(t)
	_, ok := getStats("brand-new-source")
	if ok {
		t.Fatal("expected no data for a source that has never been searched")
	}
}

func TestRecordSearchOutcomeAccumulatesRealHistory(t *testing.T) {
	resetStatsForTest(t)
	recordSearchOutcome("test-source", true, 100)
	recordSearchOutcome("test-source", true, 200)
	recordSearchOutcome("test-source", false, 50)

	stats, ok := getStats("test-source")
	if !ok {
		t.Fatal("expected real data after 3 recorded outcomes")
	}
	if stats.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", stats.Attempts)
	}
	if stats.Successes != 2 {
		t.Errorf("expected 2 successes, got %d", stats.Successes)
	}
	if stats.TotalLatencyMs != 350 {
		t.Errorf("expected total latency 350ms, got %d", stats.TotalLatencyMs)
	}
	if stats.LastSearchedAt == "" {
		t.Error("expected LastSearchedAt to be set after a real recorded outcome")
	}
}

func TestStatsPersistAcrossCacheReload(t *testing.T) {
	resetStatsForTest(t)
	recordSearchOutcome("persisted-source", true, 150)

	// Force a real reload from disk, not just the in-memory cache, to confirm this is
	// genuinely persisted, not only held in memory for the current process.
	statsCache = nil
	stats, ok := getStats("persisted-source")
	if !ok {
		t.Fatal("expected stats to persist to disk and be reloadable")
	}
	if stats.Attempts != 1 {
		t.Errorf("expected 1 attempt after reload, got %d", stats.Attempts)
	}
}

func TestStatsAreIndependentPerSource(t *testing.T) {
	resetStatsForTest(t)
	recordSearchOutcome("source-a", true, 100)
	recordSearchOutcome("source-b", false, 500)

	statsA, _ := getStats("source-a")
	statsB, _ := getStats("source-b")
	if statsA.Attempts != 1 || statsA.Successes != 1 {
		t.Errorf("source-a stats leaked or wrong: %+v", statsA)
	}
	if statsB.Attempts != 1 || statsB.Successes != 0 {
		t.Errorf("source-b stats leaked or wrong: %+v", statsB)
	}
}
