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
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}

// TestVersionMatchesReleaseTag is the release guard. The release workflows
// write the pushed tag into version.go before running this test, so a failure
// means that step did not run or did not write the tag. It is skipped in
// ordinary test runs, where there is no tag to compare against.
func TestVersionMatchesReleaseTag(t *testing.T) {
	tag := os.Getenv("FILEGATOR_RELEASE_TAG")
	if tag == "" {
		t.Skip("FILEGATOR_RELEASE_TAG is not set")
	}
	if Version != tag {
		t.Fatalf("version.Version = %q but the release tag is %q; "+
			"the release workflow must write the tag into internal/version/version.go", Version, tag)
	}
}
