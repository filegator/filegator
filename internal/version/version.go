// Package version holds the FileGator release number.
//
// It is written here, by hand, and bumped when a release is made. The binary
// and an exported frontend both record it, so a customised interface can be
// checked against the build that serves it.
package version

// Version is the release. Bump it when tagging a release.
const Version = "v8.0.0-beta.1"

// String returns the release number.
func String() string { return Version }
