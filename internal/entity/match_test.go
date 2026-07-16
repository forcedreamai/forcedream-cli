package entity

import (
	"testing"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
)

func TestMatchScoreExactURL(t *testing.T) {
	a := discovery.Result{Name: "forcedream-mcp", URL: "https://github.com/forcedreamai/forcedream-mcp"}
	b := discovery.Result{Name: "ForceDream MCP Server", URL: "github.com/forcedreamai/forcedream-mcp/"} // trailing slash, no protocol
	score, reason := MatchScore(a, b)
	if reason != ReasonExactURL {
		t.Fatalf("expected exact_url reason, got %q (score %f)", reason, score)
	}
	if score != scoreExactURL {
		t.Fatalf("expected score %f, got %f", scoreExactURL, score)
	}
}

func TestMatchScoreSameOrgExactName(t *testing.T) {
	a := discovery.Result{Name: "forcedream-mcp", URL: "https://github.com/forcedreamai/forcedream-mcp"}
	b := discovery.Result{Name: "ForceDream MCP", URL: "https://github.com/forcedreamai/some-other-path"}
	score, reason := MatchScore(a, b)
	if reason != ReasonSameOrgExactName {
		t.Fatalf("expected same_org_exact_name, got %q (score %f)", reason, score)
	}
	if score != scoreSameOrgExactName {
		t.Fatalf("expected score %f, got %f", scoreSameOrgExactName, score)
	}
}

func TestMatchScoreSameOrgSimilarName(t *testing.T) {
	a := discovery.Result{Name: "forcedream-mcp", URL: "https://github.com/forcedreamai/repo-a"}
	b := discovery.Result{Name: "forcedreem-mcp", URL: "https://github.com/forcedreamai/repo-b"} // single-char typo
	score, reason := MatchScore(a, b)
	if reason != ReasonSameOrgSimilarName {
		t.Fatalf("expected same_org_similar_name, got %q (score %f)", reason, score)
	}
	if score != scoreSameOrgSimilarName {
		t.Fatalf("expected score %f, got %f", scoreSameOrgSimilarName, score)
	}
}

func TestMatchScoreSameOrgOnlyDifferentNames(t *testing.T) {
	a := discovery.Result{Name: "forcedream-mcp", URL: "https://github.com/forcedreamai/forcedream-mcp"}
	b := discovery.Result{Name: "completely-different-tool-name", URL: "https://github.com/forcedreamai/other-project"}
	score, reason := MatchScore(a, b)
	if reason != ReasonSameOrgOnly {
		t.Fatalf("expected same_org_only, got %q (score %f)", reason, score)
	}
	if score != scoreSameOrgOnly {
		t.Fatalf("expected score %f, got %f", scoreSameOrgOnly, score)
	}
}

func TestMatchScoreExactNameNoOrgSignal(t *testing.T) {
	// Two results with identical normalized names but no extractable org (e.g. Smithery
	// and Web results, which don't carry GitHub-style URLs).
	a := discovery.Result{Name: "Translation Bridge", URL: ""}
	b := discovery.Result{Name: "translation-bridge", URL: ""}
	score, reason := MatchScore(a, b)
	if reason != ReasonExactNameNoOrg {
		t.Fatalf("expected exact_name_no_org_signal, got %q (score %f)", reason, score)
	}
	if score != scoreExactNameNoOrg {
		t.Fatalf("expected score %f, got %f", scoreExactNameNoOrg, score)
	}
}

func TestMatchScoreSimilarNameNoOrgSignal(t *testing.T) {
	a := discovery.Result{Name: "Translation Bridge", URL: ""}
	b := discovery.Result{Name: "Translatian Bridge", URL: ""} // single-char typo, no org signal either side
	score, reason := MatchScore(a, b)
	if reason != ReasonSimilarNameNoOrg {
		t.Fatalf("expected similar_name_no_org_signal, got %q (score %f)", reason, score)
	}
	if score != scoreSimilarNameNoOrg {
		t.Fatalf("expected score %f, got %f", scoreSimilarNameNoOrg, score)
	}
}

func TestMatchScoreNoMatchAtAll(t *testing.T) {
	a := discovery.Result{Name: "Translation Bridge", URL: "https://github.com/forcedreamai/translation-bridge"}
	b := discovery.Result{Name: "OCR Extraction Agent", URL: "https://github.com/some-other-org/ocr-tool"}
	score, reason := MatchScore(a, b)
	if reason != "" || score != 0 {
		t.Fatalf("expected no match at all (score 0, no reason), got score=%f reason=%q", score, reason)
	}
}

func TestMatchScoreDifferentOrgsDoesNotFalsePositive(t *testing.T) {
	// Same exact name, but different, real organizations -- must not be conflated just
	// because the name matches; genuinely different projects can share a generic name.
	a := discovery.Result{Name: "mcp-server", URL: "https://github.com/forcedreamai/mcp-server"}
	b := discovery.Result{Name: "mcp-server", URL: "https://github.com/someone-else/mcp-server"}
	score, reason := MatchScore(a, b)
	// Falls through to the no-org-signal path since orgs differ (not "same org"), scored
	// on name alone -- exact_name_no_org_signal, not a same-org tier.
	if reason == ReasonSameOrgExactName || reason == ReasonSameOrgOnly || reason == ReasonSameOrgSimilarName {
		t.Fatalf("must not report a same-org match for genuinely different organizations, got %q", reason)
	}
	if reason != ReasonExactNameNoOrg {
		t.Fatalf("expected exact_name_no_org_signal (orgs differ, so org signal doesn't apply), got %q (score %f)", reason, score)
	}
}

func TestExtractOrganizationGitHub(t *testing.T) {
	org := extractOrganization("forcedream-mcp", "https://github.com/forcedreamai/forcedream-mcp")
	if org != "forcedreamai" {
		t.Fatalf("expected 'forcedreamai', got %q", org)
	}
}

func TestExtractOrganizationNpmScope(t *testing.T) {
	org := extractOrganization("@forcedreamai/mcp-server", "https://www.npmjs.com/package/@forcedreamai/mcp-server")
	if org != "forcedreamai" {
		t.Fatalf("expected 'forcedreamai' from npm scope, got %q", org)
	}
}

func TestExtractOrganizationReturnsEmptyWhenNoSignal(t *testing.T) {
	org := extractOrganization("some-tool", "https://example.com/some-tool")
	if org != "" {
		t.Fatalf("expected no organization signal for a non-GitHub, non-scoped URL, got %q", org)
	}
}
