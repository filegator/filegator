package version

import (
	"os"
	"testing"
)

// frontendVersion is the shape written to version.json.
type frontendVersion struct {
	Version string `json:"version"`
}

func TestVersionIsSet(t *testing.T) {
	if Version == "" || Version == "dev" {
		t.Fatalf("Version = %q; it must be a real release number", Version)
	}
}

// TestVersionMatchesReleaseTag is the release guard. The release workflows set
// FILEGATOR_RELEASE_TAG to the pushed tag; a tag that was not accompanied by a
// bump of version.go fails here, before any binary or image is built. It is
// skipped in ordinary test runs, where there is no tag to compare against.
func TestVersionMatchesReleaseTag(t *testing.T) {
	tag := os.Getenv("FILEGATOR_RELEASE_TAG")
	if tag == "" {
		t.Skip("FILEGATOR_RELEASE_TAG is not set")
	}
	if Version != tag {
		t.Fatalf("version.Version = %q but the release tag is %q; "+
			"bump internal/version/version.go before tagging", Version, tag)
	}
}
