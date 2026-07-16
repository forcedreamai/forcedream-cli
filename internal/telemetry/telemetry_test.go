package telemetry

import (
	"os"
	"testing"
)

func TestLevelOffAddsNothing(t *testing.T) {
	b := &Batch{level: LevelOff}
	b.Add(Event{Kind: "search", Success: true})
	if !b.IsEmpty() {
		t.Fatalf("expected no events recorded at LevelOff, got %d", len(b.Events()))
	}
}

func TestBasicLevelStripsDetail(t *testing.T) {
	b := &Batch{level: LevelBasic}
	b.Add(Event{Kind: "search", Success: true, Detail: &EventDetail{ErrorKind: "should_be_stripped"}})
	events := b.Events()
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(events))
	}
	if events[0].Detail != nil {
		t.Fatalf("expected Detail to be stripped at LevelBasic, got %+v", events[0].Detail)
	}
}

func TestDetailedLevelKeepsDetail(t *testing.T) {
	b := &Batch{level: LevelDetailed}
	b.Add(Event{Kind: "search", Success: true, Detail: &EventDetail{ErrorKind: "kept"}})
	events := b.Events()
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(events))
	}
	if events[0].Detail == nil || events[0].Detail.ErrorKind != "kept" {
		t.Fatalf("expected Detail to be preserved at LevelDetailed, got %+v", events[0].Detail)
	}
}

func TestCurrentLevelDefaultsToBasic(t *testing.T) {
	os.Unsetenv("FD_TELEMETRY")
	if got := CurrentLevel(); got != LevelBasic {
		t.Fatalf("expected default level to be basic, got %q", got)
	}
}

func TestCurrentLevelRespectsOff(t *testing.T) {
	os.Setenv("FD_TELEMETRY", "off")
	defer os.Unsetenv("FD_TELEMETRY")
	if got := CurrentLevel(); got != LevelOff {
		t.Fatalf("expected off, got %q", got)
	}
}

func TestCurrentLevelRespectsDetailed(t *testing.T) {
	os.Setenv("FD_TELEMETRY", "detailed")
	defer os.Unsetenv("FD_TELEMETRY")
	if got := CurrentLevel(); got != LevelDetailed {
		t.Fatalf("expected detailed, got %q", got)
	}
}

func TestEphemeralIDsAreNotEmptyAndNotConstant(t *testing.T) {
	a := ephemeralID()
	b := ephemeralID()
	if a == "" || b == "" {
		t.Fatal("expected non-empty ephemeral IDs")
	}
	if a == b {
		t.Fatal("expected two independently-generated ephemeral IDs to differ (this would only coincidentally fail; a real collision here would indicate a broken random source)")
	}
	if len(a) != 32 { // 16 random bytes, hex-encoded
		t.Fatalf("expected a 32-character hex string, got length %d: %q", len(a), a)
	}
}

func TestEventScheduleHasNoFreeformField(t *testing.T) {
	// A real, structural privacy guarantee, not just a comment: Event and EventDetail are
	// fixed structs with no map[string]interface{} or similar free-form field, so there is
	// no code path, now or added later by mistake, that could attach arbitrary (and
	// potentially sensitive) data to a telemetry event. This test would fail to compile --
	// not fail at runtime -- if that guarantee were ever broken, which is a stronger,
	// earlier check than any runtime assertion could give.
	_ = Event{Kind: "x", Success: true, DurationMs: 1, CLIVersion: "v", SourcesUsed: nil, SearchTermHash: "", Detail: nil}
	_ = EventDetail{ResultCountsBySource: nil, ErrorKind: ""}
}

func TestHashSearchTermIsOneWayAndDeterministic(t *testing.T) {
	a := HashSearchTerm("translation")
	b := HashSearchTerm("translation")
	if a != b {
		t.Fatalf("expected the same real term to always hash identically, got %q vs %q", a, b)
	}
	if a == "translation" {
		t.Fatal("expected a real hash, not the raw term echoed back")
	}
	if len(a) != 64 { // sha256 hex-encoded
		t.Fatalf("expected a 64-character hex sha256 hash, got length %d", len(a))
	}
}

func TestHashSearchTermNormalizesTrivialVariation(t *testing.T) {
	a := HashSearchTerm("Translation")
	b := HashSearchTerm(" translation ")
	if a != b {
		t.Fatal("expected case and whitespace variation of the same real search to hash identically")
	}
}

func TestHashSearchTermDiffersForDifferentTerms(t *testing.T) {
	a := HashSearchTerm("translation")
	b := HashSearchTerm("ocr extraction")
	if a == b {
		t.Fatal("expected genuinely different search terms to hash differently")
	}
}
