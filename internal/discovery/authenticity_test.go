package discovery

import (
	"context"
	"testing"
)

func TestParseMavenGAV(t *testing.T) {
	group, artifact, version, ok := ParseMavenGAV("Maven artifact com.google.guava:guava, latest 33.4.8-jre")
	if !ok {
		t.Fatal("expected a real, successful parse")
	}
	if group != "com.google.guava" || artifact != "guava" || version != "33.4.8-jre" {
		t.Fatalf("expected (com.google.guava, guava, 33.4.8-jre), got (%s, %s, %s)", group, artifact, version)
	}
}

func TestParseMavenGAVRejectsUnexpectedShape(t *testing.T) {
	_, _, _, ok := ParseMavenGAV("something entirely different")
	if ok {
		t.Fatal("expected a real, honest failure to parse -- not a silently wrong result")
	}
}

func TestCheckNpmProvenanceLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in -short mode")
	}
	// sigstore itself is a real, known, currently-provenance-published package (confirmed
	// via direct research before writing this check) -- a genuine positive case, not an
	// assumed one.
	check, err := CheckNpmProvenance(context.Background(), "sigstore")
	if err != nil {
		t.Fatalf("real npm provenance check failed: %v", err)
	}
	if !check.Present {
		t.Fatal("expected sigstore's real, currently-published version to have a real provenance attestation")
	}
}

func TestCheckNpmProvenanceHonestlyReportsAbsence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in -short mode")
	}
	// left-pad is real and old enough to predate provenance entirely -- a genuine
	// negative case.
	check, err := CheckNpmProvenance(context.Background(), "left-pad")
	if err != nil {
		t.Fatalf("real npm provenance check failed: %v", err)
	}
	if check.Present {
		t.Fatal("expected left-pad to honestly report no provenance attestation")
	}
}
