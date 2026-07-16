package discovery

import (
	"errors"
	"testing"
	"time"
)

func TestWithRetrySucceedsImmediatelyOnFirstTry(t *testing.T) {
	calls := 0
	err := withRetry(3, time.Millisecond, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call when the first attempt succeeds, got %d", calls)
	}
}

func TestWithRetryRetriesUntilSuccess(t *testing.T) {
	calls := 0
	err := withRetry(3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient failure")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 calls (2 failures + 1 success), got %d", calls)
	}
}

func TestWithRetryReturnsLastErrorAfterExhaustingAttempts(t *testing.T) {
	calls := 0
	err := withRetry(3, time.Millisecond, func() error {
		calls++
		return errors.New("permanent failure")
	})
	if err == nil {
		t.Fatal("expected an error after exhausting all retry attempts")
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 attempts (maxAttempts), got %d", calls)
	}
}
