package core

// Version is the build's version string, set at build time via -ldflags from
// `git describe --tags`. It defaults to "dev" so a plain `go build` (no
// Makefile) still produces something honest rather than claiming a release.
//
// This exists because the MCP server used to announce a hardcoded "1.0.0" to
// every client — a literal that no release process ever bumped, so it would
// have kept reporting 1.0.0 forever. Anything that reports a version must read
// it from here.
var Version = "dev"
