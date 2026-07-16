// Package telemetry is a standalone subsystem, architecturally independent from the
// discovery/ranking packages it observes: its own event schema, its own batching logic,
// its own privacy guarantees, and its own opt-in flow. It integrates with the rest of the
// CLI only by having call sites construct and hand it Event values -- no shared internal
// state, no reaching into discovery/ranking's own internals.
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Version is this subsystem's own version, independent of the CLI's overall version.
const Version = "1.0.0"

// Event is the real, minimal event schema. Deliberately excludes anything that could be
// PII or a credential: no task/query text, no API keys, no email, no user-chosen agent
// slugs beyond a fixed allowlist of real, known agent names (never arbitrary user input).
// Privacy guarantee: only these fields are ever constructed anywhere in this package or
// its callers -- there is no path for a caller to attach arbitrary extra data.
type Event struct {
	Kind       string `json:"kind"`        // "search" | "invoke" | "verify" | "a2a_register" | "a2a_invoke"
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms"`
	CLIVersion string `json:"cli_version"`
	// SourcesUsed is only populated for "search" events, and only ever contains real,
	// fixed source names ("ForceDream", "GitHub", "MCP Registry", "Smithery", "Web") --
	// never the search query text itself.
	SourcesUsed []string `json:"sources_used,omitempty"`
	// Detail is only populated when the user has opted into detailed telemetry (see
	// Level below) -- even then, it's a fixed struct, not a free-form map, so there is no
	// way for a future call site to accidentally attach something sensitive to it.
	Detail *EventDetail `json:"detail,omitempty"`
}

// EventDetail is the "opt-in detailed telemetry" tier -- still no task content or
// credentials, just finer-grained, still-anonymous operational data.
type EventDetail struct {
	ResultCountsBySource map[string]int `json:"result_counts_by_source,omitempty"`
	ErrorKind            string         `json:"error_kind,omitempty"` // a fixed error category, e.g. "auth_required" -- never a raw error message, which could echo back user input
}

// Level is the real opt-in flow: FD_TELEMETRY environment variable.
//   - unset or "basic" (the default): anonymous kind/success/duration/cli_version only.
//     Matches common CLI-tooling convention (Homebrew, npm, etc.) of anonymous-by-default
//     with an explicit opt-out, rather than opt-in for even the most basic counters.
//   - "off": no telemetry at all, nothing is ever sent or even batched.
//   - "detailed": the above, plus EventDetail -- requires an explicit, deliberate choice.
type Level string

const (
	LevelOff      Level = "off"
	LevelBasic    Level = "basic"
	LevelDetailed Level = "detailed"
)

func CurrentLevel() Level {
	switch os.Getenv("FD_TELEMETRY") {
	case "off":
		return LevelOff
	case "detailed":
		return LevelDetailed
	default:
		return LevelBasic
	}
}

// Batch collects events during a single CLI invocation's lifetime (the CLI is a
// short-lived, per-command process, not a daemon -- there is no cross-invocation
// in-memory state) and flushes them in exactly one real HTTP call at the end of the
// command, rather than one call per event -- the real "batching logic" this subsystem
// is required to own.
type Batch struct {
	mu     sync.Mutex
	events []Event
	level  Level
}

func NewBatch() *Batch {
	return &Batch{level: CurrentLevel()}
}

// Add records one event, stripping Detail entirely unless the user has actually opted
// into detailed telemetry -- enforced here, at the one real chokepoint, rather than
// trusting every future call site to remember to check the level itself.
func (b *Batch) Add(e Event) {
	if b.level == LevelOff {
		return
	}
	if b.level != LevelDetailed {
		e.Detail = nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, e)
}

func (b *Batch) Events() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Event, len(b.events))
	copy(out, b.events)
	return out
}

func (b *Batch) IsEmpty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events) == 0
}

// installID is a random, non-identifying UUID-like string generated once and cached in a
// local file -- never derived from any real account, key, email, or machine-identifying
// data (no MAC address, no hostname, no hardware ID). Its only purpose is letting the
// backend distinguish "10 events from 10 different installs" from "10 events from 1
// install run 10 times" for basic aggregate counts -- not to identify a person or device.
func installID() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ephemeralID() // fail-safe: telemetry must never block or crash the CLI
	}
	path := filepath.Join(dir, "forcedream", "install_id")

	if data, err := os.ReadFile(path); err == nil && len(data) == 32 {
		return string(data)
	}

	id := ephemeralID()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte(id), 0o600)
	return id
}

func ephemeralID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}

// Timer is a tiny helper so call sites get consistent, correct DurationMs values without
// each one re-implementing time.Since(...).Milliseconds() by hand.
func StartTimer() time.Time { return time.Now() }
func ElapsedMs(start time.Time) int64 { return time.Since(start).Milliseconds() }
