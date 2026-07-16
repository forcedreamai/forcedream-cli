package discovery

import (
	"time"
)

// Centralized retry + cache + stats-recording for every connector -- "no connector may
// implement its own retry or cache logic" is enforced here: a connector's Search() method
// becomes a thin wrapper handing its own raw search operation to one of these two shared
// helpers (free-shaped or paid-shaped), never calling withRetry/getCached/setCached/
// recordSearchOutcome directly itself. One place to change this behavior for every
// connector at once, rather than six near-identical copies that could quietly drift.

// instrumentedSearch is the shared helper for free (unpaid) connectors: fn is the raw,
// connector-specific search call with no retry/cache/stats concern of its own.
func instrumentedSearch(name, query string, limit int, fn func() ([]Result, error)) (Outcome, error) {
	if cached, ok := getCached(name, query, limit); ok {
		return Outcome{Results: cached, Available: true}, nil
	}

	start := time.Now()
	var results []Result
	err := withRetry(2, 300*time.Millisecond, func() error {
		var e error
		results, e = fn()
		return e
	})
	recordSearchOutcome(name, err == nil, time.Since(start).Milliseconds())

	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	setCached(name, query, limit, results)
	return Outcome{Results: results, Available: true}, nil
}

// instrumentedPaidSearch is the shared helper for paid, gated connectors: fn additionally
// returns a real PaidSourceStatus. Same centralization, plus the paid-specific rule kept
// from the original, per-connector implementations: retries apply only to genuine
// request/network failures, never to a real gate rejection (retrying an insufficient_funds
// response is pointless -- the balance won't have changed in 300ms); caching applies only
// to real, successful results, never a rejection (a person who tops up their balance or
// sets FD_LIVE_KEY between calls must not see a stale "unavailable" result).
func instrumentedPaidSearch(name, query string, limit int, fn func() ([]Result, PaidSourceStatus, error)) (Outcome, error) {
	if cached, ok := getCached(name, query, limit); ok {
		return Outcome{Results: cached, Available: true}, nil
	}

	start := time.Now()
	var results []Result
	var status PaidSourceStatus
	err := withRetry(2, 300*time.Millisecond, func() error {
		var e error
		results, status, e = fn()
		return e
	})
	recordSearchOutcome(name, err == nil && status.Available, time.Since(start).Milliseconds())

	if err != nil {
		return Outcome{Available: false, Reason: "request_failed", Message: err.Error()}, err
	}
	if !status.Available {
		return Outcome{Available: false, Reason: status.Reason, Message: status.Message}, nil
	}
	setCached(name, query, limit, results)
	return Outcome{Results: results, Available: true}, nil
}
