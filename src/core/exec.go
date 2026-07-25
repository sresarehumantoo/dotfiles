package core

import "time"

// Command timeouts.
//
// Every subprocess gets a deadline. Without one, a curl to a blackholed host,
// an apt waiting on a dpkg lock, or a stalled crates.io mirror blocks the whole
// run behind a spinner with no way out but Ctrl-C — and under the MCP server
// there isn't even a terminal to Ctrl-C from.
//
// These are backstops against hangs, not performance budgets: each is set well
// past what the slowest healthy run of that class takes.
const (
	// ProbeTimeout bounds quick informational commands — version checks,
	// `pipx list`, `dpkg --print-architecture`, `locale -a`.
	ProbeTimeout = 30 * time.Second

	// NetworkTimeout bounds downloads and API calls.
	NetworkTimeout = 10 * time.Minute

	// InstallTimeout bounds package installs and compiles, which legitimately
	// run long — cargo builds from source, the .NET SDK download.
	InstallTimeout = 45 * time.Minute
)
