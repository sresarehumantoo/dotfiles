package modules

import (
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type VMGuestModule struct{}

func (VMGuestModule) Name() string { return "vmguest" }

// guestPackages maps a hypervisor type to the canonical guest tool packages
// to install. Names are Debian-canonical; installPkg translates for pacman.
// dnf and brew use the same names.
var guestPackages = map[core.VirtType][]string{
	core.VirtKVM:        {"qemu-guest-agent", "spice-vdagent"},
	core.VirtQEMU:       {"qemu-guest-agent", "spice-vdagent"},
	core.VirtVMware:     {"open-vm-tools"},
	core.VirtVirtualBox: {"virtualbox-guest-utils"},
	core.VirtHyperV:     {"hyperv-daemons"},
}

// guestServices maps a hypervisor type to the systemd units to enable after
// installing the guest packages.
var guestServices = map[core.VirtType][]string{
	core.VirtKVM:    {"qemu-guest-agent", "spice-vdagentd"},
	core.VirtQEMU:   {"qemu-guest-agent", "spice-vdagentd"},
	core.VirtVMware: {"open-vm-tools"},
}

func (VMGuestModule) Install() error {
	if core.IsWSL() {
		core.Ok("Not a hardware VM (WSL), skipping vmguest")
		return nil
	}

	virt := core.DetectVirt()
	if !core.IsVM() {
		core.Ok("Not running in a VM (%s), skipping vmguest", virt)
		return nil
	}

	pkgs := guestPackages[virt]
	if len(pkgs) == 0 {
		core.Notice("vmguest: detected %s but no guest tools mapping — skipping", virt)
		return nil
	}

	if core.DryRun {
		core.Info("would install %s guest tools: %v", virt, pkgs)
		return nil
	}

	core.Info("Installing %s guest tools: %v", virt, pkgs)
	if err := installPkg(pkgs...); err != nil {
		core.Warn("Some guest tools may have failed to install: %v", err)
	}

	for _, svc := range guestServices[virt] {
		if !systemdAvailable() {
			core.Notice("systemctl not found — skipping service enablement for %s", svc)
			break
		}
		core.Info("Enabling %s...", svc)
		if err := runCmd("sudo", "systemctl", "enable", "--now", svc); err != nil {
			core.Warn("enable %s: %v", svc, err)
		}
	}

	switch virt {
	case core.VirtVMware:
		core.Notice("For clipboard/drag-drop in a graphical session, also install: open-vm-tools-desktop")
	case core.VirtVirtualBox:
		core.Notice("For clipboard/drag-drop in a graphical session, also install: virtualbox-guest-x11")
	}

	core.Ok("vmguest: %s tools done", virt)
	return nil
}

// systemdAvailable reports whether systemctl is on PATH (good enough proxy
// for "systemd-managed system" without parsing /proc/1/comm).
func systemdAvailable() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func (VMGuestModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "vmguest"}
	virt := core.DetectVirt()
	s.Extra = string(virt)

	if core.IsWSL() || !core.IsVM() {
		return s
	}

	for _, p := range guestPackages[virt] {
		if pkgInstalled(p) {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	return s
}
