package core

import "runtime/debug"

// Version is the build's version string, set at build time via -ldflags from
// `git describe --tags`. Without ldflags it falls back to the module version Go
// records in the binary, which is what a `go install ...@vX.Y.Z` build has, and
// then to "dev", so a build never claims a release it isn't.
//
// This exists because the MCP server used to announce a hardcoded "1.0.0" to
// every client — a literal that no release process ever bumped, so it would
// have kept reporting 1.0.0 forever. Anything that reports a version must read
// it from here.
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		Version = bi.Main.Version
	}
}
