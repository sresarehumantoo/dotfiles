package tests

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestParseSystemdVirt(t *testing.T) {
	tests := []struct {
		in   string
		want core.VirtType
	}{
		{"kvm", core.VirtKVM},
		{"qemu", core.VirtQEMU},
		{"vmware", core.VirtVMware},
		{"oracle", core.VirtVirtualBox},
		{"microsoft", core.VirtHyperV},
		{"xen", core.VirtXen},
		{"wsl", core.VirtWSL},
		{"lxc", core.VirtLXC},
		{"lxc-libvirt", core.VirtLXC},
		{"docker", core.VirtDocker},
		{"podman", core.VirtPodman},
		{"openvz", core.VirtContainer},
		{"systemd-nspawn", core.VirtContainer},
		{"rkt", core.VirtContainer},
		{"proot", core.VirtContainer},
		{"pouch", core.VirtContainer},
		{"container-other", core.VirtContainer},
		{"none", core.VirtNone},
		{"", core.VirtNone},
		{"some-future-thing", core.VirtUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := core.ParseSystemdVirt(tt.in); got != tt.want {
				t.Errorf("ParseSystemdVirt(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsHardwareVirt(t *testing.T) {
	// Each entry locks in whether a given VirtType counts as a hardware
	// VM (and thus should get guest tools installed). Containers, WSL,
	// none, and unknown must all be false — we don't want to install
	// qemu-guest-agent inside a container or in an environment we can't
	// classify.
	tests := []struct {
		v    core.VirtType
		want bool
	}{
		// True hardware virtualization
		{core.VirtKVM, true},
		{core.VirtQEMU, true},
		{core.VirtVMware, true},
		{core.VirtVirtualBox, true},
		{core.VirtHyperV, true},
		{core.VirtXen, true},

		// Anything else must be false
		{core.VirtNone, false},
		{core.VirtWSL, false},
		{core.VirtLXC, false},
		{core.VirtDocker, false},
		{core.VirtPodman, false},
		{core.VirtContainer, false},
		{core.VirtUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.v), func(t *testing.T) {
			if got := core.IsHardwareVirt(tt.v); got != tt.want {
				t.Errorf("IsHardwareVirt(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestParseDMIVendor(t *testing.T) {
	tests := []struct {
		name    string
		vendor  string
		product string
		want    core.VirtType
	}{
		{"QEMU vendor", "QEMU", "Standard PC", core.VirtKVM},
		{"KVM via product", "Red Hat", "KVM", core.VirtKVM},
		{"VMware", "VMware, Inc.", "VMware Virtual Platform", core.VirtVMware},
		{"VirtualBox via innotek", "innotek GmbH", "VirtualBox", core.VirtVirtualBox},
		{"Hyper-V", "Microsoft Corporation", "Virtual Machine", core.VirtHyperV},
		{"Microsoft Surface (not Hyper-V)", "Microsoft Corporation", "Surface Pro", core.VirtNone},
		{"Xen", "Xen", "HVM domU", core.VirtXen},
		{"bare metal", "Dell Inc.", "Latitude 7420", core.VirtNone},
		{"empty", "", "", core.VirtNone},
		{"case insensitive", "qemu", "", core.VirtKVM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := core.ParseDMIVendor(tt.vendor, tt.product)
			if got != tt.want {
				t.Errorf("ParseDMIVendor(%q, %q) = %q, want %q", tt.vendor, tt.product, got, tt.want)
			}
		})
	}
}
