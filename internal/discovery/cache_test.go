package discovery

import (
	"path/filepath"
	"testing"
	"time"
)

func resetCacheForTest(t *testing.T) {
	t.Helper()
	cacheFilePathOverride = filepath.Join(t.TempDir(), "search_cache_test.json")
	cacheCache = nil
	t.Cleanup(func() {
		cacheFilePathOverride = ""
		cacheCache = nil
	})
}

func TestCacheMissBeforeAnySet(t *testing.T) {
	resetCacheForTest(t)
	_, ok := getCached("npm", "translation", 30)
	if ok {
		t.Fatal("expected a cache miss before anything has been cached")
	}
}

func TestCacheHitAfterSet(t *testing.T) {
	resetCacheForTest(t)
	real := []Result{{Name: "real-result", Source: "npm"}}
	setCached("npm", "translation", 30, real)

	cached, ok := getCached("npm", "translation", 30)
	if !ok {
		t.Fatal("expected a cache hit after setCached")
	}
	if len(cached) != 1 || cached[0].Name != "real-result" {
		t.Fatalf("expected the real, cached result back, got %+v", cached)
	}
}

func TestCacheIsKeyedBySourceAndQuerySeparately(t *testing.T) {
	resetCacheForTest(t)
	setCached("npm", "translation", 30, []Result{{Name: "npm-result"}})
	setCached("github", "translation", 30, []Result{{Name: "github-result"}})

	npmResult, ok := getCached("npm", "translation", 30)
	if !ok || npmResult[0].Name != "npm-result" {
		t.Fatalf("expected npm's own cached result, got %+v ok=%v", npmResult, ok)
	}
	ghResult, ok := getCached("github", "translation", 30)
	if !ok || ghResult[0].Name != "github-result" {
		t.Fatalf("expected github's own cached result, got %+v ok=%v", ghResult, ok)
	}

	// A different query for the same source should still genuinely miss.
	_, ok = getCached("npm", "a completely different query", 30)
	if ok {
		t.Fatal("expected a cache miss for a different query, even for a source with other cached entries")
	}
}

func TestCacheIsKeyedByLimitSeparately(t *testing.T) {
	resetCacheForTest(t)
	setCached("npm", "translation", 5, []Result{{Name: "five-results"}})

	// A request for a different limit on the same source+query must not be satisfied by
	// a cached result fetched for a smaller limit -- a real correctness concern this test
	// exists specifically to guard.
	_, ok := getCached("npm", "translation", 30)
	if ok {
		t.Fatal("expected a cache miss when limit differs from what was actually cached")
	}
}

func TestCacheExpiresAfterTTL(t *testing.T) {
	resetCacheForTest(t)
	cf := loadCache()
	// Directly inject an entry timestamped well past the real TTL, rather than actually
	// sleeping 5 real minutes in a test.
	cf.Entries[cacheKey("npm", "old-query", 30)] = cacheEntry{
		Results:  []Result{{Name: "stale"}},
		CachedAt: time.Now().Add(-cacheTTL - time.Minute).UTC().Format(time.RFC3339),
	}
	saveCache(cf)
	cacheCache = nil // force a real reload, not just the in-memory value we just set

	_, ok := getCached("npm", "old-query", 30)
	if ok {
		t.Fatal("expected an expired cache entry to honestly report a miss, not stale data")
	}
}
