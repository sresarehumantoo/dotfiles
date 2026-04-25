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
