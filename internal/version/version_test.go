package version

import "testing"

// frontendVersion is the shape written to version.json.
type frontendVersion struct {
	Version string `json:"version"`
}

func TestVersionIsSet(t *testing.T) {
	if Version == "" || Version == "dev" {
		t.Fatalf("Version = %q; it must be a real release number", Version)
	}
}
