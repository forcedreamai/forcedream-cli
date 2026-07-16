package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Package-local connector observability -- deliberately independent of internal/telemetry
// (its own file, its own struct, its own read/write code, no shared package import between
// them) per the permanent rule that the four named subsystems (Dashboard/Ranking/Routing/
// Telemetry) share no internal state. This is discovery's own concern: real, measured
// per-connector history, not reused from elsewhere.
//
// Backs Connector.Latency()/Reliability() with real, locally-accumulated observations
// rather than static, guessed numbers -- a connector's latency/reliability isn't knowable
// in the abstract the way its Capabilities are; it only becomes real once actually
// measured. Before any real search has happened for a source, Latency()/Reliability()
// honestly report "no data yet" rather than a fabricated default.

type connectorStats struct {
	Attempts       int    `json:"attempts"`
	Successes      int    `json:"successes"`
	TotalLatencyMs int64  `json:"total_latency_ms"`
	LastSearchedAt string `json:"last_searched_at,omitempty"` // RFC3339, "" if never searched
}

type statsFile struct {
	Sources map[string]*connectorStats `json:"sources"`
}

var (
	statsMu    sync.Mutex
	statsCache *statsFile
)

// statsFilePathOverride lets tests redirect stats storage to a temporary location without
// depending on OS-specific env-var behavior (os.UserConfigDir() doesn't honor
// XDG_CONFIG_HOME on macOS the way it does on Linux, so an env-var-based override
// wouldn't be portable across the environments this actually needs to run in).
var statsFilePathOverride string

func statsFilePath() (string, error) {
	if statsFilePathOverride != "" {
		return statsFilePathOverride, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "forcedream", "connector_stats.json"), nil
}

func loadStats() *statsFile {
	if statsCache != nil {
		return statsCache
	}
	sf := &statsFile{Sources: map[string]*connectorStats{}}
	path, err := statsFilePath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, sf) // fail-safe: a corrupt/missing file just starts fresh
		}
	}
	if sf.Sources == nil {
		sf.Sources = map[string]*connectorStats{}
	}
	statsCache = sf
	return sf
}

func saveStats(sf *statsFile) {
	path, err := statsFilePath()
	if err != nil {
		return // fail-safe: stats are a nice-to-have, never allowed to break a real search
	}
	data, err := json.Marshal(sf)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)
}

// recordSearchOutcome records one real search's outcome for a source -- called from
// within each connector's own Search() implementation, once per real attempt.
func recordSearchOutcome(source string, success bool, latencyMs int64) {
	statsMu.Lock()
	defer statsMu.Unlock()
	sf := loadStats()
	s, ok := sf.Sources[source]
	if !ok {
		s = &connectorStats{}
		sf.Sources[source] = s
	}
	s.Attempts++
	if success {
		s.Successes++
	}
	s.TotalLatencyMs += latencyMs
	s.LastSearchedAt = time.Now().UTC().Format(time.RFC3339)
	saveStats(sf)
}

// getStats returns this source's real, accumulated stats and whether any real data exists
// yet -- the caller (Connector.Latency()/Reliability()) must handle the "no data yet" case
// honestly rather than treating a zero-value struct as a real zero.
func getStats(source string) (connectorStats, bool) {
	statsMu.Lock()
	defer statsMu.Unlock()
	sf := loadStats()
	s, ok := sf.Sources[source]
	if !ok || s.Attempts == 0 {
		return connectorStats{}, false
	}
	return *s, true
}

// latencyInfoFor and reliabilityInfoFor are the shared implementations every connector's
// Latency()/Reliability() method delegates to, keyed by its own Name() -- kept in one
// place rather than reimplemented identically on every connector struct.
func latencyInfoFor(source string) LatencyInfo {
	stats, ok := getStats(source)
	if !ok {
		return LatencyInfo{HasData: false}
	}
	return LatencyInfo{HasData: true, AverageMs: stats.TotalLatencyMs / int64(stats.Attempts)}
}

func reliabilityInfoFor(source string) ReliabilityInfo {
	stats, ok := getStats(source)
	if !ok {
		return ReliabilityInfo{HasData: false}
	}
	return ReliabilityInfo{
		HasData:     true,
		SuccessRate: float64(stats.Successes) / float64(stats.Attempts),
		Attempts:    stats.Attempts,
	}
}
