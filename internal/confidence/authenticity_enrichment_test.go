package confidence

import (
	"context"
	"testing"

	"github.com/forcedreamai/forcedream-cli/internal/discovery"
	"github.com/forcedreamai/forcedream-cli/internal/entity"
)

func TestCheckPackageAuthenticityLiveNpm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in -short mode")
	}
	e := entity.Entity{
		Sources: []entity.SourceRef{{Source: "npm", Name: "sigstore", URL: "https://www.npmjs.com/package/sigstore"}},
	}
	results := []discovery.Result{
		{Source: "npm", Name: "sigstore", URL: "https://www.npmjs.com/package/sigstore"},
	}
	score := CheckPackageAuthenticity(context.Background(), e, results)
	if !score.Available {
		t.Fatal("expected Available=true for a real npm package with a real, known provenance attestation")
	}
	if score.Score != 100.0 {
		t.Fatalf("expected score 100 for a real, present provenance attestation, got %f", score.Score)
	}
}

func TestCheckPackageAuthenticityHonestlyInsufficientForUnverifiedSources(t *testing.T) {
	e := entity.Entity{
		Sources: []entity.SourceRef{{Source: "Packagist", Name: "monolog/monolog", URL: "https://packagist.org/packages/monolog/monolog"}},
	}
	results := []discovery.Result{
		{Source: "Packagist", Name: "monolog/monolog", URL: "https://packagist.org/packages/monolog/monolog"},
	}
	score := CheckPackageAuthenticity(context.Background(), e, results)
	if score.Available {
		t.Fatal("expected Available=false for a source with no verified authenticity-checking mechanism -- Packagist has no real, checked signal here")
	}
}
