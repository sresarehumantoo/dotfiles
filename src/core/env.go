package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DefaultDotfilesDir is set at build time via -ldflags.
var DefaultDotfilesDir string

// Distro represents a Linux distribution family.
type Distro int

const (
	DistroUnknown Distro = iota
	DistroDebian         // debian, ubuntu, raspbian, etc.
	DistroFedora         // fedora, rhel, centos, rocky, etc.
	DistroArch           // arch, manjaro, endeavouros, etc.
	DistroSteamOS        // steamos (Arch-based, readonly root)
)

var (
	dotfilesDir string
	isWSL       bool
	isGitBash   bool
	distro      Distro
	envDetected bool
)

// DetectEnvironment probes the runtime to determine WSL/GitBash status.
func DetectEnvironment() {
	if envDetected {
		return
	}
	envDetected = true

	// Git Bash / MSYS / MinGW
	if os.Getenv("MSYSTEM") != "" || os.Getenv("MINGW_PREFIX") != "" {
		isGitBash = true
		return
	}

	if IsMac() {
		addBrewToPath()
		return
	}

	// WSL detection via /proc/version
	isWSL = checkWSL()

	// Distro detection via /etc/os-release
	distro = detectDistro()
}

func checkWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// addBrewToPath puts the chosen Homebrew (BrewPrefix) first on this process's
// PATH: on a fresh Mac no shell has run `brew shellenv` yet, and on a managed
// one a read-only IT Homebrew can be on PATH ahead of the user's. Without it
// every LookPath misses what brew installs, so each run reinstalls, or every
// install runs a brew that cannot write.
func addBrewToPath() {
	prefix := BrewPrefix()
	if prefix == "" {
		return
	}
	bin := filepath.Join(prefix, "bin")
	if brew, err := exec.LookPath("brew"); err == nil && filepath.Dir(brew) == bin {
		return
	}
	os.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// ParseProcVersion checks whether content from /proc/version indicates WSL.
// Exported for testing.
func ParseProcVersion(content string) bool {
	return strings.Contains(strings.ToLower(content), "microsoft")
}

// AssertEnvironment exits if the environment is unsupported.
func AssertEnvironment() {
	if isGitBash {
		Err("===========================================================")
		Err("  Git Bash / MSYS / MinGW detected.")
		Err("")
		Err("  This dotfiles setup requires a full Linux environment.")
		Err("  Please install WSL2 and run this from inside WSL:")
		Err("")
		Err("    1.  wsl --install -d Debian")
		Err("    2.  Open the new Debian terminal")
		Err("    3.  git clone <this-repo> ~/dotfiles")
		Err("    4.  cd ~/dotfiles && ./install.sh")
		Err("")
		Err("  See: https://learn.microsoft.com/en-us/windows/wsl/install")
		Err("===========================================================")
		os.Exit(1)
	}

	if isWSL {
		data, _ := os.ReadFile("/proc/version")
		version := "WSL2"
		if idx := strings.Index(string(data), "WSL"); idx >= 0 {
			end := idx
			for end < len(data) && data[end] != ' ' && data[end] != '\n' {
				end++
			}
			version = string(data[idx:end])
		}
		Info("WSL detected (%s)", version)
	}

	if distro == DistroSteamOS {
		Info("SteamOS detected (Arch-based, readonly root)")
	}
}

// detectDistro reads /etc/os-release to determine the Linux distribution.
func detectDistro() Distro {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return DistroUnknown
	}
	return ParseOsRelease(string(data))
}

// ParseOsRelease parses /etc/os-release content and returns the detected distro.
// Exported for testing.
func ParseOsRelease(content string) Distro {
	var id, idLike string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"'")
		}
		if strings.HasPrefix(line, "ID_LIKE=") {
			idLike = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"'")
		}
	}

	switch id {
	case "steamos":
		return DistroSteamOS
	case "arch", "manjaro", "endeavouros", "artix":
		return DistroArch
	case "debian", "ubuntu", "raspbian", "linuxmint", "kali", "devuan", "elementary", "parrot", "parrotsec":
		return DistroDebian
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		return DistroFedora
	}

	// Fallback to ID_LIKE
	for _, like := range strings.Fields(idLike) {
		switch like {
		case "arch":
			return DistroArch
		case "debian", "ubuntu":
			return DistroDebian
		case "fedora", "rhel":
			return DistroFedora
		}
	}

	return DistroUnknown
}

// knownUpstreamCodenames are Debian and Ubuntu release codenames safe to
// pass as the suite name in third-party apt repos (Docker, Hashicorp, etc.).
// Derivative codenames (parrot's "lory", kali's "kali-rolling", mint's
// "wilma") are deliberately absent — they cause 404s against upstream repos.
var knownUpstreamCodenames = map[string]bool{
	// Debian
	"trixie": true, "bookworm": true, "bullseye": true, "buster": true, "sid": true,
	// Ubuntu LTS + recent interim
	"noble": true, "jammy": true, "focal": true, "bionic": true,
}

// debianVersionToCodename maps the major version found in /etc/debian_version
// to the corresponding upstream Debian codename. Used as a fallback when the
// derivative doesn't ship DEBIAN_CODENAME and its VERSION_CODENAME isn't a
// real Debian/Ubuntu release.
var debianVersionToCodename = map[string]string{
	"10": "buster",
	"11": "bullseye",
	"12": "bookworm",
	"13": "trixie",
}

// DistroCodename returns VERSION_CODENAME from /etc/os-release as the distro
// itself names it ("trixie", "resolute", "kali-rolling"), or "" if unset.
// Unlike UpstreamDebianCodename it is not mapped onto a known upstream release.
func DistroCodename() string {
	osRelease, _ := os.ReadFile("/etc/os-release")
	for _, line := range strings.Split(string(osRelease), "\n") {
		if v, ok := strings.CutPrefix(line, "VERSION_CODENAME="); ok {
			return strings.Trim(v, "\"'")
		}
	}
	return ""
}

// UpstreamDebianCodename returns the Debian/Ubuntu codename to use when
// configuring third-party apt repos. Reads /etc/os-release and
// /etc/debian_version. Defaults to "bookworm" if nothing usable is found.
func UpstreamDebianCodename() string {
	osRelease, _ := os.ReadFile("/etc/os-release")
	debianVersion, _ := os.ReadFile("/etc/debian_version")
	return ParseUpstreamDebianCodename(string(osRelease), string(debianVersion))
}

// ParseUpstreamDebianCodename is the testable core of UpstreamDebianCodename.
// Resolution order:
//  1. DEBIAN_CODENAME from os-release (Parrot 6+, some other derivatives)
//  2. VERSION_CODENAME from os-release, only if it's a known upstream codename
//  3. /etc/debian_version major version mapped to a codename
//  4. "bookworm" as a last resort
func ParseUpstreamDebianCodename(osRelease, debianVersion string) string {
	var versionCodename, debianCodename string
	for _, line := range strings.Split(osRelease, "\n") {
		switch {
		case strings.HasPrefix(line, "DEBIAN_CODENAME="):
			debianCodename = strings.Trim(strings.TrimPrefix(line, "DEBIAN_CODENAME="), "\"'")
		case strings.HasPrefix(line, "VERSION_CODENAME="):
			versionCodename = strings.Trim(strings.TrimPrefix(line, "VERSION_CODENAME="), "\"'")
		}
	}

	if debianCodename != "" && knownUpstreamCodenames[debianCodename] {
		return debianCodename
	}
	if knownUpstreamCodenames[versionCodename] {
		return versionCodename
	}

	// /etc/debian_version: "12", "12.4", "trixie/sid", "kali-rolling", etc.
	v := strings.TrimSpace(debianVersion)
	if v != "" {
		major := v
		if i := strings.IndexAny(v, "./"); i > 0 {
			major = v[:i]
		}
		if codename, ok := debianVersionToCodename[major]; ok {
			return codename
		}
	}

	return "bookworm"
}

// GetDistro returns the detected distribution.
func GetDistro() Distro {
	return distro
}

// IsSteamOS returns true if running on SteamOS.
func IsSteamOS() bool {
	return distro == DistroSteamOS
}

// IsArchBased returns true for Arch Linux and SteamOS.
func IsArchBased() bool {
	return distro == DistroArch || distro == DistroSteamOS
}

// IsDebianBased returns true for Debian, Ubuntu, and derivatives.
func IsDebianBased() bool {
	return distro == DistroDebian
}

// ubuntuFamilyIDs are os-release IDs whose apt suites track Ubuntu releases.
// They must use vendor repos' linux/ubuntu tree (e.g. Docker), not linux/debian.
var ubuntuFamilyIDs = map[string]bool{
	"ubuntu": true, "linuxmint": true, "elementary": true,
	"pop": true, "neon": true, "zorin": true,
}

// IsUbuntuFamily reports whether this host is Ubuntu or an Ubuntu derivative
// (vs a pure-Debian one). Needed to pick the right upstream apt repo for
// vendors like Docker that ship separate ubuntu/ and debian/ trees — pairing
// linux/debian with an Ubuntu codename (e.g. "noble") yields a missing suite.
func IsUbuntuFamily() bool {
	osRelease, _ := os.ReadFile("/etc/os-release")
	return ParseIsUbuntuFamily(string(osRelease))
}

// ParseIsUbuntuFamily is the testable core of IsUbuntuFamily.
func ParseIsUbuntuFamily(osRelease string) bool {
	var id, idLike string
	for _, line := range strings.Split(osRelease, "\n") {
		switch {
		case strings.HasPrefix(line, "ID="):
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"'")
		case strings.HasPrefix(line, "ID_LIKE="):
			idLike = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"'")
		}
	}
	if ubuntuFamilyIDs[id] {
		return true
	}
	for _, like := range strings.Fields(idLike) {
		if like == "ubuntu" {
			return true
		}
	}
	return false
}

func setSteamOSReadonly(ctx context.Context, verb string) error {
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()

	PauseSpinner()
	defer ResumeSpinner()
	cmd := SudoCmd(ctx, "steamos-readonly", verb)
	if Level >= LogVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("steamos-readonly %s: %w", verb, err)
	}
	return nil
}

// DisableReadonly disables the SteamOS readonly filesystem.
func DisableReadonly(ctx context.Context) error {
	return setSteamOSReadonly(ctx, "disable")
}

// EnableReadonly re-enables the SteamOS readonly filesystem.
//
// It deliberately detaches from the caller's context: this runs as deferred
// cleanup, and a canceled ctx (Ctrl-C mid-install) must not stop us restoring
// the filesystem state we changed.
func EnableReadonly(ctx context.Context) error {
	if err := setSteamOSReadonly(context.WithoutCancel(ctx), "enable"); err != nil {
		return err
	}
	return nil
}

// IsWSL returns true if running under Windows Subsystem for Linux.
func IsWSL() bool {
	return isWSL
}

// IsMac returns true on macOS.
func IsMac() bool {
	return runtime.GOOS == "darwin"
}

// IsGitBash returns true if running under Git Bash/MSYS/MinGW.
func IsGitBash() bool {
	return isGitBash
}

// ResetDotfilesDir clears the cached dotfiles directory so it is re-resolved
// on the next call to DotfilesDir. Exported for testing.
func ResetDotfilesDir() {
	dotfilesDir = ""
}

// InvokingCloneDir resolves the dotfiles clone this binary is physically
// running from, ignoring any recorded canonical pointer. Order:
//  1. $DOTFILES env var (explicit override)
//  2. build-time baked path (`make build` sets it to the clone's CURDIR)
//  3. walk up from the executable looking for go.mod
//  4. current working directory
//
// This answers "where am I running from", as opposed to DotfilesDir which
// answers "which clone should I link from". It is used when adopting a new
// canonical (install all) and when warning about non-canonical runs.
func InvokingCloneDir() string {
	// 1. $DOTFILES env var
	if env := os.Getenv("DOTFILES"); env != "" {
		return env
	}

	// 2. Build-time baked path
	if DefaultDotfilesDir != "" {
		return DefaultDotfilesDir
	}

	// 3. Walk up from executable looking for go.mod
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 4. Fallback: current working directory
	cwd, _ := os.Getwd()
	return cwd
}

// DotfilesDir returns the root of the dotfiles repository that symlinks should
// point into. Resolution order:
//  1. $DOTFILES env var — explicit override, always wins (tests, CI, power users)
//  2. the machine-global canonical pointer, if it points at a valid checkout —
//     this makes every clone link to one authoritative source, preventing drift
//  3. the clone we're physically running from (InvokingCloneDir)
//
// The result is cached; call ResetDotfilesDir after changing $DOTFILES or the
// canonical pointer.
func DotfilesDir() string {
	if dotfilesDir != "" {
		return dotfilesDir
	}

	// 1. Explicit override.
	if env := os.Getenv("DOTFILES"); env != "" {
		dotfilesDir = env
		Debug("dotfiles dir from $DOTFILES: %s", dotfilesDir)
		return dotfilesDir
	}

	// 2. Machine-global canonical pointer (self-healing: a stale pointer is
	//    ignored by ReadCanonicalDir and we fall through).
	if canon := ReadCanonicalDir(); canon != "" {
		dotfilesDir = canon
		Debug("dotfiles dir from canonical pointer: %s", dotfilesDir)
		return dotfilesDir
	}

	// 3. The clone this binary lives in.
	dotfilesDir = InvokingCloneDir()
	Debug("dotfiles dir from invoking clone: %s", dotfilesDir)
	return dotfilesDir
}

// ConfigDir returns the path to config/ under the dotfiles root.
func ConfigDir() string {
	return filepath.Join(DotfilesDir(), "config")
}

// HomeDir returns the user's home directory.
//
// os.UserHomeDir returns ("", err) when $HOME is unset — under cron, some
// systemd units, or `sudo` without -H. Callers used to discard that error and
// filepath.Join the empty string anyway, and Join("", ".oh-my-zsh") yields a
// *relative* path, so every managed location silently retargeted at the
// current working directory. Two of those are os.RemoveAll'd.
//
// Deliberately not cached: tests use t.Setenv("HOME", ...) per test, and a
// cached first value would leak across them.
func HomeDir() (string, error) {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h, nil
	}
	if h := os.Getenv("HOME"); h != "" {
		return h, nil
	}
	return "", errors.New("cannot determine home directory: $HOME is unset")
}

// XDGConfigHome returns XDG_CONFIG_HOME or ~/.config. Empty when the home
// directory can't be resolved — see HomeTarget.
func XDGConfigHome() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, err := HomeDir()
	if err != nil {
		warnNoHome(err)
		return ""
	}
	return filepath.Join(home, ".config")
}

// XDGDataHome returns XDG_DATA_HOME or ~/.local/share. Empty when the home
// directory can't be resolved — see HomeTarget.
//
// Distinct from XDGConfigHome, and not interchangeable: fontconfig's default
// config carries <dir prefix="xdg">fonts</dir>, which resolves against
// XDG_DATA_HOME. The fonts module used to hardcode ~/.local/share/fonts, so a
// machine with XDG_DATA_HOME set elsewhere installed fonts into a directory
// fontconfig never scanned — they were simply not found.
func XDGDataHome() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg
	}
	home, err := HomeDir()
	if err != nil {
		warnNoHome(err)
		return ""
	}
	return filepath.Join(home, ".local", "share")
}

// warnNoHome reports an unresolvable home directory once per run, so a broken
// environment is loud rather than silently producing relative paths.
var noHomeOnce sync.Once

func warnNoHome(err error) {
	noHomeOnce.Do(func() {
		AlwaysWarn("%v — managed paths cannot be resolved; no files will be touched", err)
	})
}

// ── Sudo credential management ─────────────────────────────────

// sudoState tracks this process's sudo session. Guarded by sudoMu.
var (
	sudoMu sync.Mutex
	// sudoPrimed is set once credentials have been checked this session, so a
	// failed or declined prompt is not repeated before every command.
	sudoPrimed bool
	// sudoPrompted records that dfinstall itself asked for the password, which
	// is what makes the cached credentials ours to drop in ReleaseSudo.
	sudoPrompted  bool
	sudoKeepAlive chan struct{}
)

// Interactive reports whether it is safe to prompt on stdin. True for the CLI;
// the MCP server sets it false because there os.Stdin carries the JSON-RPC
// request stream — reading from it consumes protocol bytes and wedges the
// session. Code that would prompt must check this and pick a safe default.
var Interactive = true

// EnsureSudo makes sure sudo credentials are cached before a command that needs
// them, prompting at most once per session, then keeps them fresh in the
// background so a long install does not outlive sudo's timestamp. It is called
// from SudoCmd, so a run that never escalates never asks for a password.
func EnsureSudo(ctx context.Context) {
	sudoMu.Lock()
	defer sudoMu.Unlock()
	// Nothing dfinstall runs on macOS needs sudo: Homebrew refuses root, and
	// chsh asks for the user's own password.
	if sudoPrimed || IsMac() || DryRun || os.Geteuid() == 0 {
		return
	}
	sudoPrimed = true

	// Check if sudo even needs a password (e.g. NOPASSWD configured)
	probeCtx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	if exec.CommandContext(probeCtx, "sudo", "-n", "true").Run() == nil {
		Debug("sudo: credentials available without a prompt")
		startSudoKeepAlive()
		return
	}

	// Nothing cached. Prompting requires a terminal; if stdin is a protocol
	// stream, reading it would corrupt the session, so leave credentials
	// unprimed and let the command fail loudly.
	if !Interactive {
		Debug("sudo: not primed (non-interactive) — steps requiring sudo will fail")
		return
	}

	PauseSpinner()
	defer ResumeSpinner()
	Status("This step requires sudo access")
	cmd := exec.CommandContext(ctx, "sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		Warn("sudo authentication failed — some steps may prompt again")
		return
	}
	sudoPrompted = true
	startSudoKeepAlive()
}

// startSudoKeepAlive refreshes sudo credentials in the background until
// ReleaseSudo. Caller holds sudoMu.
func startSudoKeepAlive() {
	stop := make(chan struct{})
	sudoKeepAlive = stop
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				refresh, cancel := context.WithTimeout(context.Background(), ProbeTimeout)
				if err := exec.CommandContext(refresh, "sudo", "-n", "-v").Run(); err != nil {
					Debug("sudo keepalive refresh failed: %v", err)
				}
				cancel()
			}
		}
	}()
}

// ReleaseSudo ends the session EnsureSudo started: it stops the keepalive and,
// when dfinstall was the one that asked for the password, invalidates the
// cached credentials so they do not stay usable in this terminal after the run.
// Credentials that were already cached before dfinstall ran are left alone.
func ReleaseSudo() {
	sudoMu.Lock()
	defer sudoMu.Unlock()
	if sudoKeepAlive != nil {
		close(sudoKeepAlive)
		sudoKeepAlive = nil
	}
	if sudoPrompted {
		// context.Background: runs during shutdown, after the command's own
		// context may already be canceled (Ctrl-C), which is when it matters most.
		ctx, cancel := context.WithTimeout(context.Background(), ProbeTimeout)
		if err := exec.CommandContext(ctx, "sudo", "-k").Run(); err != nil {
			Debug("sudo -k failed: %v", err)
		}
		cancel()
	}
	sudoPrimed, sudoPrompted = false, false
}

// SudoCmd returns an *exec.Cmd for running a command via sudo, with stdin
// connected to the terminal when interactive.
//
// Already running as root, it execs the command directly — sudo may not even
// be installed in a root container, and escalating from root is pointless.
func SudoCmd(ctx context.Context, args ...string) *exec.Cmd {
	if os.Geteuid() == 0 {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if Interactive {
			cmd.Stdin = os.Stdin
		}
		return cmd
	}

	EnsureSudo(ctx)
	cmd := exec.CommandContext(ctx, "sudo", args...)
	if Interactive {
		cmd.Stdin = os.Stdin
	}
	// Non-interactive: leave Stdin nil so sudo gets EOF and fails, rather than
	// reading the caller's protocol stream while waiting for a password.
	return cmd
}
