package entity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
)

// Persistent storage for manual override decisions and resolved-entity history -- its own
// file (entities.json), separate from discovery's stats/cache files and every other
// subsystem's storage, matching the established pattern (each concern owns its own local
// file, never reaching into another's).

// referenceKey is the stable identifier used to refer to a specific discovered result for
// override purposes -- prefers the real URL (genuinely unique per real item) when present,
// falls back to source+name when a source (like Smithery) doesn't carry one.
func referenceKey(r discovery.Result) string {
	if r.URL != "" {
		return r.Source + "|" + normalizeURL(r.URL)
	}
	return r.Source + "|" + normalizeName(r.Name)
}

// OverrideDecision is a real, human-made decision that takes precedence over the
// automatic MatchScore -- "merge" forces two results together regardless of score;
// "split" prevents a merge even if the automatic score would otherwise qualify.
type OverrideDecision string

const (
	OverrideMerge OverrideDecision = "merge"
	OverrideSplit OverrideDecision = "split"
)

type overrideRecord struct {
	KeyA     string           `json:"key_a"`
	KeyB     string           `json:"key_b"`
	Decision OverrideDecision `json:"decision"`
}

type storageFile struct {
	Overrides []overrideRecord         `json:"overrides"`
	Entities  map[string]Entity        `json:"entities"` // keyed by Entity.ID -- every resolved entity ever seen, persisted permanently
}

var (
	storageMu             sync.Mutex
	storageCache          *storageFile
	storageFilePathOverride string
)

func storageFilePath() (string, error) {
	if storageFilePathOverride != "" {
		return storageFilePathOverride, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "forcedream", "entities.json"), nil
}

func loadStorage() *storageFile {
	if storageCache != nil {
		return storageCache
	}
	sf := &storageFile{Entities: map[string]Entity{}}
	path, err := storageFilePath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, sf)
		}
	}
	if sf.Entities == nil {
		sf.Entities = map[string]Entity{}
	}
	storageCache = sf
	return sf
}

func saveStorage(sf *storageFile) {
	path, err := storageFilePath()
	if err != nil {
		return
	}
	data, err := json.Marshal(sf)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)
}

// AddOverride records a real, permanent manual decision about two specific discovered
// results, taking precedence over the automatic MatchScore in all future Resolve calls
// that pass it via ResolveWithOverrides.
func AddOverride(a, b discovery.Result, decision OverrideDecision) {
	storageMu.Lock()
	defer storageMu.Unlock()
	sf := loadStorage()
	sf.Overrides = append(sf.Overrides, overrideRecord{KeyA: referenceKey(a), KeyB: referenceKey(b), Decision: decision})
	saveStorage(sf)
}

// LoadOverrides returns every real, persisted override decision.
func LoadOverrides() []overrideRecord {
	storageMu.Lock()
	defer storageMu.Unlock()
	sf := loadStorage()
	out := make([]overrideRecord, len(sf.Overrides))
	copy(out, sf.Overrides)
	return out
}

// PersistEntities permanently records every resolved entity from a Resolve call -- "persist
// every decision", not just the immediate result of one run. Keyed by Entity.ID (stable/
// deterministic per entity.go's deriveEntityID), so re-resolving the same real sources
// later overwrites with the same, consistent record rather than accumulating duplicates.
func PersistEntities(entities []Entity) {
	storageMu.Lock()
	defer storageMu.Unlock()
	sf := loadStorage()
	for _, e := range entities {
		sf.Entities[e.ID] = e
	}
	saveStorage(sf)
}

// AllPersistedEntities returns every entity ever resolved and persisted on this machine.
func AllPersistedEntities() []Entity {
	storageMu.Lock()
	defer storageMu.Unlock()
	sf := loadStorage()
	out := make([]Entity, 0, len(sf.Entities))
	for _, e := range sf.Entities {
		out = append(out, e)
	}
	return out
}
