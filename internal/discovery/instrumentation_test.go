package discovery

import (
	"errors"
	"testing"
)

func TestInstrumentedSearchCachesSuccessfulResult(t *testing.T) {
	resetStatsForTest(t)
	resetCacheForTest(t)

	calls := 0
	fn := func() ([]Result, error) {
		calls++
		return []Result{{Name: "real"}}, nil
	}

	out1, err := instrumentedSearch("test-src", "q", 10, fn)
	if err != nil || !out1.Available {
		t.Fatalf("expected first call to succeed, got out=%+v err=%v", out1, err)
	}
	out2, err := instrumentedSearch("test-src", "q", 10, fn)
	if err != nil || !out2.Available {
		t.Fatalf("expected second call to succeed from cache, got out=%+v err=%v", out2, err)
	}

	if calls != 1 {
		t.Fatalf("expected the real search function to be called exactly once (second call served from cache), got %d calls", calls)
	}
}

func TestInstrumentedSearchRetriesOnFailureThenSucceeds(t *testing.T) {
	resetStatsForTest(t)
	resetCacheForTest(t)

	calls := 0
	fn := func() ([]Result, error) {
		calls++
		if calls < 2 {
			return nil, errors.New("transient")
		}
		return []Result{{Name: "real"}}, nil
	}

	out, err := instrumentedSearch("test-src-2", "q", 10, fn)
	if err != nil || !out.Available {
		t.Fatalf("expected eventual success after retry, got out=%+v err=%v", out, err)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 calls (1 failure + 1 success), got %d", calls)
	}
}

func TestInstrumentedSearchRecordsFailureStats(t *testing.T) {
	resetStatsForTest(t)
	resetCacheForTest(t)

	fn := func() ([]Result, error) { return nil, errors.New("permanent") }
	_, err := instrumentedSearch("test-src-3", "q", 10, fn)
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}

	stats, ok := getStats("test-src-3")
	if !ok {
		t.Fatal("expected real stats to be recorded even for a failed search")
	}
	if stats.Successes != 0 {
		t.Fatalf("expected 0 successes recorded, got %d", stats.Successes)
	}
}

func TestInstrumentedPaidSearchNeverRetriesGateRejection(t *testing.T) {
	resetStatsForTest(t)
	resetCacheForTest(t)

	calls := 0
	fn := func() ([]Result, PaidSourceStatus, error) {
		calls++
		return nil, PaidSourceStatus{Available: false, Reason: "insufficient_funds", Message: "top up"}, nil
	}

	out, err := instrumentedPaidSearch("paid-src", "q", 10, fn)
	if err != nil {
		t.Fatalf("a gate rejection is not a Go error -- expected err=nil, got %v", err)
	}
	if out.Available {
		t.Fatal("expected Available=false for a gate rejection")
	}
	if out.Reason != "insufficient_funds" {
		t.Fatalf("expected the real gate reason to be preserved, got %q", out.Reason)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call -- a gate rejection must never be retried, got %d calls", calls)
	}
}

func TestInstrumentedPaidSearchNeverCachesGateRejection(t *testing.T) {
	resetStatsForTest(t)
	resetCacheForTest(t)

	calls := 0
	fn := func() ([]Result, PaidSourceStatus, error) {
		calls++
		return nil, PaidSourceStatus{Available: false, Reason: "auth_required", Message: "sign in"}, nil
	}

	instrumentedPaidSearch("paid-src-2", "q", 10, fn)
	instrumentedPaidSearch("paid-src-2", "q", 10, fn)

	if calls != 2 {
		t.Fatalf("expected a gate rejection to never be cached -- both calls should genuinely execute, got %d calls", calls)
	}
}

func TestInstrumentedPaidSearchCachesRealSuccess(t *testing.T) {
	resetStatsForTest(t)
	resetCacheForTest(t)

	calls := 0
	fn := func() ([]Result, PaidSourceStatus, error) {
		calls++
		return []Result{{Name: "real-paid-result"}}, PaidSourceStatus{Available: true}, nil
	}

	instrumentedPaidSearch("paid-src-3", "q", 10, fn)
	out2, _ := instrumentedPaidSearch("paid-src-3", "q", 10, fn)

	if calls != 1 {
		t.Fatalf("expected a real success to be cached -- second call should not re-execute, got %d calls", calls)
	}
	if len(out2.Results) != 1 || out2.Results[0].Name != "real-paid-result" {
		t.Fatalf("expected the real, cached result back, got %+v", out2.Results)
	}
}
