package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Real, local, short-TTL result cache -- deliberately its own file, separate from
// stats.go's connector_stats.json, keeping "how fast/reliable is this source" and
// "what did we last get back for this exact query" as genuinely separate concerns even
// though both live in the same package. A single CLI invocation only searches once per
// source per command, so an in-memory-only cache would never help across invocations;
// this persists to a local file the same way stats does, so a repeated identical query
// within the TTL window reuses the real, recent result instead of hitting the live API
// again.

const cacheTTL = 5 * time.Minute

type cacheEntry struct {
	Results   []Result `json:"results"`
	CachedAt  string   `json:"cached_at"` // RFC3339
}

type cacheFile struct {
	Entries map[string]cacheEntry `json:"entries"`
}

var (
	cacheMu             sync.Mutex
	cacheCache          *cacheFile
	cacheFilePathOverride string
)

func cacheFilePath() (string, error) {
	if cacheFilePathOverride != "" {
		return cacheFilePathOverride, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "forcedream", "search_cache.json"), nil
}

func cacheKey(source, query string, limit int) string {
	h := sha256.Sum256([]byte(source + ":" + query + ":" + fmt.Sprint(limit)))
	return hex.EncodeToString(h[:])
}

func loadCache() *cacheFile {
	if cacheCache != nil {
		return cacheCache
	}
	cf := &cacheFile{Entries: map[string]cacheEntry{}}
	path, err := cacheFilePath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, cf) // fail-safe: a corrupt/missing cache just starts empty
		}
	}
	if cf.Entries == nil {
		cf.Entries = map[string]cacheEntry{}
	}
	cacheCache = cf
	return cf
}

func saveCache(cf *cacheFile) {
	path, err := cacheFilePath()
	if err != nil {
		return // fail-safe: caching is a nice-to-have, never allowed to break a real search
	}
	data, err := json.Marshal(cf)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)
}

// getCached returns real, recent results for this exact source+query if they exist and are
// still within the TTL window -- honestly returns false (not stale data) once expired.
func getCached(source, query string, limit int) ([]Result, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cf := loadCache()
	entry, ok := cf.Entries[cacheKey(source, query, limit)]
	if !ok {
		return nil, false
	}
	cachedAt, err := time.Parse(time.RFC3339, entry.CachedAt)
	if err != nil || time.Since(cachedAt) > cacheTTL {
		return nil, false
	}
	return entry.Results, true
}

// getCachedIgnoringExpiry is used only by adaptive scheduling's skip-and-fallback path: a
// connector judged currently unhealthy skips a real call entirely, and a stale-but-present
// cached result is honestly better than nothing for a source likely to fail anyway. Never
// used by a connector's own normal Search() path -- that always respects the real TTL via
// getCached above.
func getCachedIgnoringExpiry(source, query string, limit int) ([]Result, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cf := loadCache()
	entry, ok := cf.Entries[cacheKey(source, query, limit)]
	if !ok {
		return nil, false
	}
	return entry.Results, true
}

// setCached stores a real, just-fetched result set for this exact source+query+limit.
func setCached(source, query string, limit int, results []Result) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cf := loadCache()
	cf.Entries[cacheKey(source, query, limit)] = cacheEntry{Results: results, CachedAt: time.Now().UTC().Format(time.RFC3339)}
	saveCache(cf)
}
