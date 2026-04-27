package modules

import (
	"os/exec"
	"strings"

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
		if virt == core.VirtUnknown {
			core.Notice("vmguest: virtualization detected but type unrecognized — skipping. Install guest tools manually if needed.")
		} else {
			core.Ok("Not running in a VM (%s), skipping vmguest", virt)
		}
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
		if err := startSystemdUnit(svc); err != nil {
			core.Warn("start %s: %v", svc, err)
		}
	}

	switch virt {
	case core.VirtKVM, core.VirtQEMU:
		core.Notice("If clipboard sharing isn't working, run: clipboard-vm")
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

// startSystemdUnit ensures a unit is running. For "static" units (e.g.
// qemu-guest-agent, newer spice-vdagentd — no [Install] section because
// they're activated by socket/udev/host signal) `systemctl enable` refuses
// with "no installation config", so just `start` instead.
func startSystemdUnit(svc string) error {
	state := unitInstallState(svc)
	switch state {
	case "static", "alias", "indirect":
		// Can't `enable` these — just start.
		core.Info("Starting %s (static unit)...", svc)
		return runCmd("sudo", "systemctl", "start", svc)
	case "enabled", "enabled-runtime":
		// Already enabled; just make sure it's running.
		core.Info("Starting %s (already enabled)...", svc)
		return runCmd("sudo", "systemctl", "start", svc)
	default:
		// "disabled", "masked", "" (unknown) — try enable --now and fall
		// back to plain start if enable refuses.
		core.Info("Enabling %s...", svc)
		if err := runCmd("sudo", "systemctl", "enable", "--now", svc); err != nil {
			core.Notice("enable %s failed (state=%q) — starting only", svc, state)
			return runCmd("sudo", "systemctl", "start", svc)
		}
		return nil
	}
}

// unitInstallState returns the systemctl is-enabled status for a unit, or
// "" if the call failed. Trims trailing newline.
func unitInstallState(svc string) string {
	out, err := exec.Command("systemctl", "is-enabled", svc).Output()
	if err != nil {
		// is-enabled returns non-zero for "disabled", "static" etc but
		// still prints the state to stdout. Honor the output even on
		// non-zero exit.
		if exitErr, ok := err.(*exec.ExitError); ok {
			_ = exitErr
		}
	}
	return strings.TrimSpace(string(out))
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
