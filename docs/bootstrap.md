# Bootstrapping a Fresh Machine

The bootstrap scripts take a machine with nothing on it to a full `dfinstall install all`. They install what dfinstall itself needs first (a distro, Homebrew, Ghostty), then dfinstall from its package channel, then run it.

- [WSL from PowerShell](#wsl-from-powershell)
- [macOS](#macos)
  - [Which Homebrew](#which-homebrew)
  - [Company-managed Macs](#company-managed-macs)
  - [TLS-inspecting proxies](#tls-inspecting-proxies)

## WSL from PowerShell

`bootstrap/wsl-bootstrap.ps1` sets up a WSL Debian distro from nothing: it creates a user, installs packages, builds Neovim, installs Ghostty, installs dfinstall from its apt repository, and runs the full dotfiles install.

```powershell
git clone https://github.com/sresarehumantoo/dotfiles
cd dotfiles
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1
```

Windows refuses to run scripts by default (execution policy `Restricted`), so a plain `.\bootstrap\wsl-bootstrap.ps1` fails before doing anything. `-ExecutionPolicy Bypass` applies to that one PowerShell process and changes no setting on the machine.

The wizard prompts for a distro and a username. These flags skip the prompts or trim the scope:

| Flag | Effect |
|------|--------|
| `-Distro Debian -Username <name>` | Skip the prompts |
| `-SkipNeovim` | Skip the Neovim build |
| `-SkipGhostty` | Skip the Ghostty install |
| `-SkipDotfiles` | Core tools only |
| `-Branch <name>` | Use another branch, building dfinstall from source |
| `-FromSource` | Build dfinstall from the clone even on `main` |
| `-CorpCert <path>` | Install a corporate root CA (PEM or DER) into the distro's trust store before any network step, for a TLS-inspecting network |
| `-PreHook <path>` | Run a script inside the distro after root setup, before Neovim and Ghostty (proxy or repository setup, say) |
| `-PostHook <path>` | Run a script inside the distro as the last step |

The flags combine: `-SkipDotfiles -SkipNeovim -SkipGhostty` is the bare minimum.

The released dfinstall is used only on `main`, which releases are cut from. Another branch's configs may need code no release has yet, so on any other branch (or with `-FromSource`) the bootstrap builds dfinstall from the clone with Go. It does the same when the apt repository cannot be reached. Set `DFINSTALL_APT_URL` in the distro to install from a mirror of the repository instead.

## macOS

`bootstrap/macos-setup.sh` installs the Xcode Command Line Tools, Homebrew and Ghostty, then dfinstall from the Homebrew tap. It clones to `~/dotfiles` and runs `dfinstall install all`.

```bash
curl -fsSL https://raw.githubusercontent.com/sresarehumantoo/dotfiles/main/bootstrap/macos-setup.sh | bash
```

Run from an existing clone (`./bootstrap/macos-setup.sh`), it uses that clone instead.

| Flag or variable | Effect |
|------------------|--------|
| `--skip-ghostty`, `--skip-dotfiles` | Trim the scope |
| `--from-source` | Install Go and build dfinstall from the clone (`make install`) |
| `--brew-prefix <path>` | Install a separate Homebrew this account owns ([below](#which-homebrew)) |
| `--yes` | Answer every prompt |
| `DFINSTALL_TAP_URL` | Clone the tap from a mirror |
| `HOMEBREW_ARTIFACT_DOMAIN` | Homebrew's own setting, to mirror the downloads |

On a branch other than `main`, or when the tap cannot be reached, it builds from source as `--from-source` does.

The bootstrap shows its plan and asks before every step that changes the machine. Declining stops there with nothing further changed, and re-running picks up where it left off. Linux-only modules (konsole, sway, wsl, vmguest, windev) skip themselves on macOS.

When Ghostty's download site is blocked, so `brew install --cask ghostty` fails, the bootstrap offers Ghostty's nightly build from GitHub instead (GitHub carries no stable release of it), checked against GitHub's sha256.

### Which Homebrew

The Homebrew to use is discovered, not assumed. The bootstrap takes the first one this account can write to, checking in order:

1. one recorded by an earlier run,
2. the `brew` on `PATH`,
3. `/opt/homebrew`, `/usr/local` and `~/homebrew`.

It records the choice in `~/.config/dfinstall/brew-prefix`, which `~/.profile` and dfinstall read first, so a read-only Homebrew (an IT-owned `/opt/homebrew`, say) never shadows yours. With none usable it offers Homebrew's official installer at its default prefix. If that cannot finish, it stops and lists the options rather than picking a path for you.

`--brew-prefix <path>` installs a separate Homebrew. The prefix length matters:

- **13 characters or fewer** (as long as `/opt/homebrew`): installed through the official installer, and every prebuilt bottle works.
- **Longer**, like `~/homebrew`: installed from Homebrew's release tarball. Bottles that are not relocatable (coreutils, gawk, make, bash, node, neovim, wget, openssl and a few more) compile from source on first install.

### Company-managed Macs

Nothing past the bootstrap needs admin. The bootstrap checks up front and reports what it found: whether the account is an admin, which Homebrew is usable, and whether Santa is in Lockdown mode, which blocks an unsigned `dfinstall`.

- Apps go to `~/Applications` when `/Applications` is not writable, including casks installed by hand (through `HOMEBREW_CASK_OPTS`).
- `install all` creates no login items. Only `install sketchybar`, `install alttab`, `install mackeys` and AeroSpace's own setting do.
- Rust comes from Homebrew's rustup rather than `curl | sh`.
- An SSH agent that is already running is kept.

The bootstrap ends by saving `dfinstall doctor` output to `~/Library/Logs/`. Doctor reports admin status, Homebrew ownership, the app folder, MDM enrollment, Santa, TLS trust and Gatekeeper; run `dfinstall doctor` again any time to see the same.

### TLS-inspecting proxies

The bootstrap detects interception by who issued the certificates a few hosts present: if none chains to Apple's public roots, something re-signed them. It then points every tool at a bundle of Apple's public roots plus the System keychain's certificates, through `~/.config/dfinstall/ca.sh`, which `~/.profile` sources.

On current macOS most tools already trust the keychain, which is where MDM installs the proxy's root. Measured on macOS 26, Apple's curl, git and Python, Homebrew, pip and Go all do. Node and npm carry their own CA list, and they are what the bundle fixes. `SSL_CERT_FILE` is left alone, because setting it would turn off Go's keychain verification.

After the proxy's root CA changes, re-run the bootstrap with `--skip-ghostty --skip-dotfiles`. `dfinstall doctor` flags a bundle that is missing a keychain certificate.

On an older macOS whose `curl` does not trust the keychain, the very first download can fail with a certificate error. Fetch the script with the keychain's certificates instead:

```bash
security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain /Library/Keychains/System.keychain > /tmp/ca.pem
curl --cacert /tmp/ca.pem -fsSL https://raw.githubusercontent.com/sresarehumantoo/dotfiles/main/bootstrap/macos-setup.sh | bash
```
