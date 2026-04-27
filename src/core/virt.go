package core

import (
	"os"
	"os/exec"
	"strings"
)

// VirtType identifies a virtualization technology. Values mirror the strings
// returned by `systemd-detect-virt` so tests can use the same vocabulary.
type VirtType string

const (
	VirtNone       VirtType = "none"
	VirtKVM        VirtType = "kvm"
	VirtQEMU       VirtType = "qemu"
	VirtVMware     VirtType = "vmware"
	VirtVirtualBox VirtType = "oracle"
	VirtHyperV     VirtType = "microsoft"
	VirtXen        VirtType = "xen"
	VirtWSL        VirtType = "wsl"
	VirtLXC        VirtType = "lxc"
	VirtDocker     VirtType = "docker"
	VirtPodman     VirtType = "podman"
	// VirtContainer covers other systemd-detect-virt container outputs
	// we don't have specific handling for: openvz, systemd-nspawn, rkt,
	// proot, pouch, container-other.
	VirtContainer VirtType = "container"
	// VirtUnknown is "we couldn't classify this". Treated as NOT a
	// hardware VM — better to skip guest-tool installation than to
	// install qemu-guest-agent inside a future container type we
	// don't recognize.
	VirtUnknown VirtType = "unknown"
)

// IsHardwareVirt reports whether v represents hardware virtualization (a
// true VM that benefits from guest tools). Containers, WSL, none, and
// unknown all return false. Exported so callers and tests share a single
// source of truth for what counts as a "VM" for our purposes.
func IsHardwareVirt(v VirtType) bool {
	switch v {
	case VirtKVM, VirtQEMU, VirtVMware, VirtVirtualBox, VirtHyperV, VirtXen:
		return true
	}
	return false
}

// DetectVirt returns the virtualization technology in use, or VirtNone for
// bare metal. Prefers systemd-detect-virt; falls back to DMI inspection so
// minimal images without systemd still get a useful answer.
func DetectVirt() VirtType {
	if v, ok := detectVirtSystemd(); ok {
		return v
	}
	return detectVirtDMI()
}

// IsVM returns true when running inside a hardware-virtualized guest.
// Excludes containers and WSL.
func IsVM() bool {
	return IsHardwareVirt(DetectVirt())
}

func detectVirtSystemd() (VirtType, bool) {
	if _, err := exec.LookPath("systemd-detect-virt"); err != nil {
		return "", false
	}
	out, err := exec.Command("systemd-detect-virt").Output()
	if err != nil {
		// systemd-detect-virt exits 1 when nothing is detected. That's not
		// an error — it's the "none" answer.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return VirtNone, true
		}
		return "", false
	}
	return ParseSystemdVirt(strings.TrimSpace(string(out))), true
}

// ParseSystemdVirt maps a systemd-detect-virt output string to a VirtType.
// Exported for testing.
func ParseSystemdVirt(s string) VirtType {
	switch s {
	case "kvm":
		return VirtKVM
	case "qemu":
		return VirtQEMU
	case "vmware":
		return VirtVMware
	case "oracle":
		return VirtVirtualBox
	case "microsoft":
		return VirtHyperV
	case "xen":
		return VirtXen
	case "wsl":
		return VirtWSL
	case "lxc", "lxc-libvirt":
		return VirtLXC
	case "docker":
		return VirtDocker
	case "podman":
		return VirtPodman
	case "openvz", "systemd-nspawn", "rkt", "proot", "pouch", "container-other":
		return VirtContainer
	case "none", "":
		return VirtNone
	default:
		return VirtUnknown
	}
}

func detectVirtDMI() VirtType {
	vendor, _ := os.ReadFile("/sys/class/dmi/id/sys_vendor")
	product, _ := os.ReadFile("/sys/class/dmi/id/product_name")
	return ParseDMIVendor(strings.TrimSpace(string(vendor)), strings.TrimSpace(string(product)))
}

// ParseDMIVendor maps DMI vendor/product strings to a VirtType. Returns
// VirtNone when nothing virtualization-related is recognised. Exported for
// testing.
func ParseDMIVendor(vendor, product string) VirtType {
	v := strings.ToLower(vendor)
	p := strings.ToLower(product)
	switch {
	case strings.Contains(v, "qemu") || strings.Contains(p, "kvm"):
		return VirtKVM
	case strings.Contains(v, "vmware") || strings.Contains(p, "vmware"):
		return VirtVMware
	case strings.Contains(v, "innotek") || strings.Contains(p, "virtualbox"):
		return VirtVirtualBox
	case strings.Contains(v, "microsoft") && strings.Contains(p, "virtual"):
		return VirtHyperV
	case strings.Contains(v, "xen"):
		return VirtXen
	}
	return VirtNone
}
