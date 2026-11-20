// Package version holds the FileGator release number.
//
// The release workflows replace the constant with the pushed tag before
// building, so a release binary reports exactly the tag it was published
// under. The value below is the development default; it is not bumped for a
// release. The binary and an exported frontend both record it, so a
// customised interface can be checked against the build that serves it.
package version

// Version is the release. Release builds replace this constant with the tag.
const Version = "dev"

// String returns the release number.
func String() string { return Version }
