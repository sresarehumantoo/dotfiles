package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/sresarehumantoo/dotfiles/src/core"
)

// Ghostty's cursor-trail shader is the most expensive thing on this desktop
// when there is no GPU behind it, and WSLg frequently has none.
//
// Measured here (idle window, nothing typed, 10s samples, Ghostty 1.2.0),
// comparing hardware GL against LIBGL_ALWAYS_SOFTWARE=1 which is what Mesa
// falls back to on WSLg when the d3d12 backend does not bind:
//
//	no shader,                       GPU       0.5%  CPU
//	no shader,                       software  0.0%  CPU
//	shader, animation = always,      GPU      15.6%  CPU
//	shader, animation = false,       software 66.9%  CPU
//	shader, animation = always,      software  762%  CPU
//	shader, animation = true,        software  806%  CPU
//
// Two things that are not obvious from those numbers:
//
//   - `custom-shader-animation = false` is NOT a fix. It still costs 67% of a
//     core at idle, because `cursor-style-blink = true` forces a redraw twice a
//     second and every redraw is a full-screen shader pass. The only effective
//     lever is removing the shader.
//   - `always` vs `true` barely differ; a focused window pays the same either
//     way. Do not "optimise" this by switching animation modes.
//
// So on WSL the shader is a question to ask, not a default to impose.

// ghosttyLocalFile is the per-machine override. Ghostty's `config-file` is
// resolved relative to the directory of the file containing the directive, and
// — measured, this is the surprising part — that is the directory of the
// SYMLINK (~/.config/ghostty/), not of the symlink's target in the repo. That
// differs from `custom-shader`, which does resolve through to the repo. So this
// file lands outside the repo, which is what we want for machine-local state.
const ghosttyLocalFile = "ghostty.local"

const shaderManagedHeader = "# Managed by dfinstall — `ghostty-shader` toggles this file."

func ghosttyLocalPath() string {
	return core.XDGTarget("ghostty", ghosttyLocalFile)
}

// rendererIsSoftware reports whether OpenGL is being software-rasterised, plus
// a human-readable renderer name.
//
// Detection is best-effort and says so: glxinfo/eglinfo are authoritative but
// come from mesa-utils, which is not guaranteed to be installed. The fallback
// is the DRI render node, whose absence is the documented signature of WSLg
// failing to bring up the d3d12 backend (microsoft/wslg#1470 is literally
// titled "/dev/dri/renderD128 not created by dxgkrnl, Mesa falls back to
// llvmpipe"). Absence is strong evidence; presence is not proof, hence the
// third return value.
func rendererIsSoftware(ctx context.Context) (software bool, name string, certain bool) {
	for _, probe := range []struct{ bin, arg string }{
		{"glxinfo", "-B"},
		{"eglinfo", "-B"},
	} {
		out, err := runProbe(ctx, probe.bin, probe.arg)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			low := strings.ToLower(line)
			if !strings.Contains(low, "renderer") {
				continue
			}
			name = strings.TrimSpace(line)
			isSW := strings.Contains(low, "llvmpipe") ||
				strings.Contains(low, "softpipe") ||
				strings.Contains(low, "swrast")
			return isSW, name, true
		}
	}

	if _, err := os.Stat("/dev/dri/renderD128"); err != nil {
		return true, "no /dev/dri/renderD128 (no GPU render node)", false
	}
	return false, "/dev/dri/renderD128 present", false
}

// writeGhosttyShaderOverride enables or disables the shader for this machine.
func writeGhosttyShaderOverride(enabled bool) error {
	if core.DryRun {
		core.Info("would set the Ghostty cursor trail to enabled=%v in %s", enabled, ghosttyLocalPath())
		return nil
	}
	path := ghosttyLocalPath()
	if err := core.CheckTarget(path); err != nil {
		return err
	}
	if err := core.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString(shaderManagedHeader + "\n")
	b.WriteString("# Loaded via `config-file = ?ghostty.local` at the end of the main config,\n")
	b.WriteString("# so it is applied AFTER everything else and wins.\n\n")
	if enabled {
		b.WriteString("# Cursor trail left ON for this machine.\n")
		b.WriteString("# (Nothing to override — the main config already enables it.)\n")
	} else {
		b.WriteString("# Cursor trail OFF for this machine.\n")
		b.WriteString("# An EMPTY value clears the whole list — `custom-shader` is a repeatable\n")
		b.WriteString("# option, so assigning a path here would ADD a second shader instead of\n")
		b.WriteString("# replacing it. Verified with `ghostty +show-config`.\n")
		b.WriteString("custom-shader = \n")
		b.WriteString("custom-shader-animation = false\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ghosttyShaderEnabled reports the current effective choice for this machine.
func ghosttyShaderEnabled() bool {
	data, err := os.ReadFile(ghosttyLocalPath())
	if err != nil {
		return true // no override -> main config's shader applies
	}
	return !strings.Contains(string(data), "custom-shader = \n")
}

// configureGhosttyShader asks the user what to do about the cursor trail, once.
//
// Only ever runs on WSL. It does not touch the shared ghostty config — the
// answer goes to the machine-local override, so a laptop on llvmpipe and a
// desktop with a working GPU can disagree while sharing one repo.
func configureGhosttyShader(ctx context.Context) {
	if _, err := os.Stat(core.XDGTarget("ghostty", "config")); err != nil {
		return // ghostty module not installed here; nothing to override
	}

	// A previous answer is respected. `ghostty-shader` is how it gets changed,
	// so re-prompting on every `install all` would be noise.
	if _, err := os.Stat(ghosttyLocalPath()); err == nil {
		state := "enabled"
		if !ghosttyShaderEnabled() {
			state = "disabled"
		}
		core.Info("Ghostty cursor trail: %s (change with `ghostty-shader`)", state)
		return
	}

	software, renderer, certain := rendererIsSoftware(ctx)

	// Non-interactive (MCP server, CI, piped stdin): never block on a prompt.
	// Safe default is the cheap one, and only when we have reason to think the
	// expensive one would hurt.
	if !core.Interactive {
		if software {
			if err := writeGhosttyShaderOverride(false); err != nil {
				core.Warn("could not write ghostty override: %v", err)
				return
			}
			core.Notice("Ghostty cursor trail disabled (software rendering detected: %s)", renderer)
			core.Notice("Re-enable any time with: ghostty-shader on")
		}
		return
	}

	core.PauseSpinner()
	defer core.ResumeSpinner()

	fmt.Println()
	fmt.Println("  Ghostty cursor trail (custom-shader)")
	fmt.Println("  ────────────────────────────────────")
	fmt.Printf("  Renderer: %s\n", renderer)
	if !certain {
		fmt.Println("  (guessed from the DRI render node — install mesa-utils for a definitive answer)")
	}
	fmt.Println()
	if software {
		fmt.Println("  ⚠ This looks like software rendering, where the trail is very expensive.")
		fmt.Println("    Measured on an idle window with nothing typed:")
		fmt.Println("      software GL, trail on ....... ~760-800% CPU  (≈8 cores)")
		fmt.Println("      software GL, trail off ......    0.0% CPU")
		fmt.Println("    Turning the animation off does not help — it still costs ~67%,")
		fmt.Println("    because the blinking cursor forces a full-screen redraw twice a second.")
	} else {
		fmt.Println("  A GPU renderer was detected, so the trail should cost ~15% of one core.")
		fmt.Println("  (Ghostty documents 'generally less than 10%'.)")
	}
	fmt.Println()

	keep := !software
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable the Ghostty cursor trail on this machine?").
				Description("Changeable later with: ghostty-shader on|off|auto").
				Affirmative("Enable").
				Negative("Disable").
				Value(&keep),
		),
	)
	if err := form.Run(); err != nil {
		core.Warn("no choice made — leaving Ghostty config untouched")
		return
	}

	if err := writeGhosttyShaderOverride(keep); err != nil {
		core.Warn("could not write ghostty override: %v", err)
		return
	}
	if keep {
		core.Ok("Ghostty cursor trail enabled")
	} else {
		core.Ok("Ghostty cursor trail disabled")
	}
	core.Notice("Restart Ghostty for this to take effect (a reload does not recompute it).")
}
