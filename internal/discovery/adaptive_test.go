package discovery

import (
	"testing"
	"time"
)

func TestShouldSkipConnectorFalseWithNoData(t *testing.T) {
	resetStatsForTest(t)
	if shouldSkipConnector("brand-new-connector") {
		t.Fatal("expected never to skip a connector with no real history yet")
	}
}

func TestShouldSkipConnectorFalseWithTooFewAttempts(t *testing.T) {
	resetStatsForTest(t)
	// 2 failures out of 2 attempts is a 0% rate, but too small a sample to judge from --
	// minAttemptsBeforeJudging is 3.
	recordSearchOutcome("flaky-but-new", false, 100)
	recordSearchOutcome("flaky-but-new", false, 100)
	if shouldSkipConnector("flaky-but-new") {
		t.Fatal("expected not to skip based on only 2 real attempts, below the judging threshold")
	}
}

func TestShouldSkipConnectorTrueWithGenuinelyPoorReliability(t *testing.T) {
	resetStatsForTest(t)
	recordSearchOutcome("genuinely-unhealthy", false, 100)
	recordSearchOutcome("genuinely-unhealthy", false, 100)
	recordSearchOutcome("genuinely-unhealthy", false, 100)
	recordSearchOutcome("genuinely-unhealthy", true, 100) // 1/4 = 25%, below the 30% threshold
	if !shouldSkipConnector("genuinely-unhealthy") {
		t.Fatal("expected to skip a connector with a real, sufficient sample showing genuinely poor reliability")
	}
}

func TestShouldSkipConnectorFalseWithGoodReliability(t *testing.T) {
	resetStatsForTest(t)
	recordSearchOutcome("reliable-source", true, 100)
	recordSearchOutcome("reliable-source", true, 100)
	recordSearchOutcome("reliable-source", false, 100)
	if shouldSkipConnector("reliable-source") {
		t.Fatal("expected not to skip a connector with a 2/3 (66%) success rate, above the threshold")
	}
}

func TestShouldSkipConnectorFalseAfterForceRetryWindow(t *testing.T) {
	resetStatsForTest(t)
	recordSearchOutcome("was-unhealthy-long-ago", false, 100)
	recordSearchOutcome("was-unhealthy-long-ago", false, 100)
	recordSearchOutcome("was-unhealthy-long-ago", false, 100)

	// Directly age the recorded LastSearchedAt past forceRetryAfter, rather than actually
	// sleeping 10 real minutes in a test.
	stats := loadStats()
	stats.Sources["was-unhealthy-long-ago"].LastSearchedAt = time.Now().Add(-forceRetryAfter - time.Minute).UTC().Format(time.RFC3339)
	saveStats(stats)
	statsCache = nil // force a real reload, not the in-memory value we just set

	if shouldSkipConnector("was-unhealthy-long-ago") {
		t.Fatal("expected a genuinely poor-reliability connector to still get a fresh chance once forceRetryAfter has passed")
	}
}
