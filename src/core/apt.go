package core

import "os/exec"

// AptBin returns the preferred apt-family binary on the current system.
// Prefers apt-get because it has a stable CLI explicitly intended for
// scripts; falls back to apt for distros that ship only the newer wrapper.
// Returns "" when neither is on PATH.
func AptBin() string {
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "apt-get"
	}
	if _, err := exec.LookPath("apt"); err == nil {
		return "apt"
	}
	return ""
}
