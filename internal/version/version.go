// Package version holds the FileGator release number.
//
// It is written here, by hand, and bumped when a release is made. The binary
// and an exported frontend both record it, so a customised interface can be
// checked against the build that serves it. The release workflows compare it
// with the tag and fail before building when they differ.
package version

// Version is the release. Bump it when tagging a release.
const Version = "v8.0.0-rc4"

// String returns the release number.
func String() string { return Version }
