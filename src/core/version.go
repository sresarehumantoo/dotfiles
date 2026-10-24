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

// How a binary was installed decides how `update` replaces it. A release
// tarball updates itself; a package manager or the Go toolchain owns the
// binary it installed, so dfinstall only says what to run.
const (
	ChannelSource  = "source"  // built from a clone with make; update rebuilds it
	ChannelRelease = "release" // a release archive; update downloads the next one
	ChannelBrew    = "brew"
	ChannelDeb     = "deb"
	ChannelGo      = "go" // go install
)

// Channel is set at build time via -ldflags by the release pipeline. Empty
// means BuildChannel infers it.
var Channel = ""

// BuildChannel reports how this binary was installed. Without an explicit
// Channel, a build that carries a module version but no VCS stamp came from
// `go install pkg@version` (a build inside a checkout records the commit), and
// anything else is a source build.
func BuildChannel() string {
	if Channel != "" {
		return Channel
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi.Main.Version == "" || bi.Main.Version == "(devel)" {
		return ChannelSource
	}
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			return ChannelSource
		}
	}
	return ChannelGo
}

// MainPackagePath is the import path of the running binary's main package, ""
// when Go recorded none. It is what `go install` takes.
func MainPackagePath() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		return bi.Path
	}
	return ""
}
