package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// AuthenticityCheck is the real, honest result of an authenticity-presence check.
// Deliberately named "Present", not "Verified" -- this package checks whether a real
// signature/attestation artifact exists at its real, expected public location, which is
// a real, true fact on its own, but is NOT the same claim as having cryptographically
// verified that signature (which would additionally require fetching and trusting the
// real publisher's key/certificate against a real trust root -- a separate, larger
// undertaking this package does not attempt). Conflating "present" with "verified" would
// itself be a form of fabrication -- the two are genuinely different claims.
type AuthenticityCheck struct {
	Mechanism string // fixed, real name of the mechanism checked, e.g. "maven_gpg_signature", "npm_provenance_attestation"
	Present   bool
	Detail    string
}

var mavenGAVRegex = regexp.MustCompile(`^Maven artifact (\S+):(\S+), latest (\S+)$`)

// ParseMavenGAV extracts the real groupId/artifactId/version this package's own Maven
// connector encodes into a Result's Description (see maven_connector.go) -- there is no
// separate, structured version field on Result, so this is the one real, current place
// that information lives for a Maven-sourced Result.
func ParseMavenGAV(description string) (group, artifact, version string, ok bool) {
	m := mavenGAVRegex.FindStringSubmatch(description)
	if m == nil {
		return "", "", "", false
	}
	return m[1], m[2], m[3], true
}

// CheckMavenSignature checks whether a real, public .asc GPG signature file exists at
// its real, documented location on Maven Central's direct repository
// (repo1.maven.org) -- confirmed live before writing this. A HEAD request, since only
// existence is being checked, not the signature's content.
func CheckMavenSignature(ctx context.Context, group, artifact, version string) (AuthenticityCheck, error) {
	groupPath := strings.ReplaceAll(group, ".", "/")
	ascURL := fmt.Sprintf("https://repo1.maven.org/maven2/%s/%s/%s/%s-%s.jar.asc", groupPath, artifact, version, artifact, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, ascURL, nil)
	if err != nil {
		return AuthenticityCheck{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return AuthenticityCheck{}, fmt.Errorf("Maven signature check unreachable: %w", err)
	}
	defer res.Body.Close()

	present := res.StatusCode == http.StatusOK
	detail := "no real .asc signature file found at the expected, real Maven Central location"
	if present {
		detail = "a real .asc GPG signature file exists at Maven Central's real, direct repository -- file presence only, not a cryptographic verification of the signature itself"
	}
	return AuthenticityCheck{Mechanism: "maven_gpg_signature", Present: present, Detail: detail}, nil
}

type npmPackageDoc struct {
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
	Versions map[string]struct {
		Dist struct {
			Attestations struct {
				URL string `json:"url"`
			} `json:"attestations"`
		} `json:"dist"`
	} `json:"versions"`
}

// CheckNpmProvenance checks whether the real, currently-published latest version of an
// npm package has a real provenance attestation, via the real, public, standard package
// metadata (registry.npmjs.org/{name}) -- confirmed live before writing this: the real
// dist.attestations.url field is only populated by the registry when a real Sigstore
// attestation was genuinely submitted and accepted at publish time.
func CheckNpmProvenance(ctx context.Context, packageName string) (AuthenticityCheck, error) {
	apiURL := "https://registry.npmjs.org/" + url.PathEscape(packageName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return AuthenticityCheck{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return AuthenticityCheck{}, fmt.Errorf("npm provenance check unreachable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return AuthenticityCheck{}, err
	}
	if res.StatusCode != http.StatusOK {
		return AuthenticityCheck{}, fmt.Errorf("npm provenance check returned HTTP %d", res.StatusCode)
	}

	var doc npmPackageDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return AuthenticityCheck{}, fmt.Errorf("npm provenance check: unexpected response shape: %w", err)
	}

	latest, ok := doc.Versions[doc.DistTags.Latest]
	present := ok && latest.Dist.Attestations.URL != ""
	detail := "the real, currently-published latest version has no provenance attestation in the real registry metadata"
	if present {
		detail = "the real, currently-published latest version has a real provenance attestation URL in the registry metadata -- attestation presence only, not a cryptographic verification of the Sigstore bundle itself"
	}
	return AuthenticityCheck{Mechanism: "npm_provenance_attestation", Present: present, Detail: detail}, nil
}
