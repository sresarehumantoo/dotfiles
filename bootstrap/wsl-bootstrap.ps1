#Requires -Version 5.1
<#
.SYNOPSIS
    Bootstrap a fresh WSL Debian distro for dotfiles installation.

.DESCRIPTION
    Interactive wizard that sets up a new WSL Debian distro from scratch:
    - Creates a user account with sudo access
    - Installs base development packages
    - Builds Neovim from source
    - Installs latest Ghostty terminal
    - Clones and builds the dotfiles repo

    Run from PowerShell on Windows. Requires WSL to be installed.

.PARAMETER Distro
    WSL distro name. If omitted, presents a selection menu.

.PARAMETER Username
    Linux username to create. If omitted, prompts interactively.

.PARAMETER Branch
    Dotfiles branch to checkout. Defaults to 'develop'.

.PARAMETER SkipNeovim
    Skip building Neovim from source.

.PARAMETER SkipGhostty
    Skip installing Ghostty terminal.

.PARAMETER SkipDotfiles
    Skip cloning and installing dotfiles. Useful for quick WSL setup with just core tools.

.PARAMETER PreHook
    Path to a script to run after root setup (e.g. proxy/repo configuration).
    Copied into the distro and executed before Neovim/Ghostty.

.PARAMETER PostHook
    Path to a script to run after dotfiles install (e.g. extra tools/config).
    Copied into the distro and executed as the last step.

.PARAMETER CorpCert
    Path to a corporate root CA certificate (PEM or DER / .crt / .cer / .pem).
    Copied into the distro, installed into the system trust store, and activated
    with update-ca-certificates BEFORE any network phase (Neovim, Ghostty,
    dotfiles, hooks). Use this on a TLS-inspecting corporate network where apt
    works but git/curl to the internet fail with certificate errors.

    If this is omitted and a certificate failure is detected (apt succeeds but a
    direct HTTPS fetch fails to verify), the bootstrap aborts and asks you to
    re-run with -CorpCert, rather than letting every download fail downstream.

.EXAMPLE
    .\wsl-bootstrap.ps1
    .\wsl-bootstrap.ps1 -Distro Debian -Username owen
    .\wsl-bootstrap.ps1 -SkipGhostty
    .\wsl-bootstrap.ps1 -SkipDotfiles -SkipNeovim -SkipGhostty
    .\wsl-bootstrap.ps1 -PreHook .\setup-proxy.sh
    .\wsl-bootstrap.ps1 -CorpCert .\corp-root-ca.crt
#>

[CmdletBinding()]
param(
    [string]$Distro,
    [string]$Username,
    [string]$Branch = "develop",
    [switch]$SkipNeovim,
    [switch]$SkipGhostty,
    [switch]$SkipDotfiles,
    [string]$PreHook,
    [string]$PostHook,
    [string]$CorpCert
)

$ErrorActionPreference = "Stop"

# ── Output helpers ───────────────────────────────────────────────
# ASCII-only markers on purpose: this runs on a fresh distro before any
# Nerd/patched fonts are installed, so Unicode glyphs would render as tofu.

function Write-Header {
    param([string]$Text)
    $label = "== $Text =="
    $pad = 60 - $label.Length
    if ($pad -lt 0) { $pad = 0 }
    $trail = [string]::new('=', $pad)
    Write-Host ""
    Write-Host "$label$trail" -ForegroundColor Cyan
    Write-Host ""
}

function Write-Step {
    param([string]$Text)
    Write-Host "  [*] $Text" -ForegroundColor DarkGray
}

function Write-Ok {
    param([string]$Text)
    Write-Host "  [+] $Text" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Text)
    Write-Host "  [!] $Text" -ForegroundColor Yellow
}

function Write-Err {
    param([string]$Text)
    Write-Host "  [x] $Text" -ForegroundColor Red
}

function Write-Info {
    param([string]$Text)
    Write-Host "  [>] $Text" -ForegroundColor Blue
}

# ── WSL helpers ──────────────────────────────────────────────────

function Test-WslInstalled {
    if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
        Write-Err "WSL is not installed."
        Write-Info "Install with: wsl --install"
        exit 1
    }
}

function Get-WslDistros {
    # wsl --list --quiet returns UTF-16LE; PowerShell handles encoding
    $raw = wsl.exe --list --quiet 2>$null
    if (-not $raw) { return @() }

    $distros = $raw -split "`n" |
        ForEach-Object { $_.Trim().Trim([char]0) } |
        Where-Object { $_ -ne "" -and $_ -ne "Windows Subsystem for Linux" }

    return @($distros)
}

function Invoke-WslRoot {
    param(
        [string]$DistroName,
        [string]$Command
    )
    wsl.exe -d $DistroName -u root -- bash -c $Command
    if ($LASTEXITCODE -ne 0) {
        throw "WSL command failed (exit $LASTEXITCODE): $Command"
    }
}

function Invoke-WslUser {
    param(
        [string]$DistroName,
        [string]$Command
    )
    wsl.exe -d $DistroName -- bash -c $Command
    if ($LASTEXITCODE -ne 0) {
        throw "WSL command failed (exit $LASTEXITCODE): $Command"
    }
}

# ── Distro selection ─────────────────────────────────────────────

function Select-Distro {
    param([string]$Requested)

    $distros = Get-WslDistros

    if ($distros.Count -eq 0) {
        Write-Err "No WSL distros found."
        Write-Info "Install one with: wsl --install -d Debian"
        exit 1
    }

    if ($Requested) {
        if ($Requested -notin $distros) {
            Write-Err "Distro '$Requested' not found."
            Write-Info "Available: $($distros -join ', ')"
            exit 1
        }
        return $Requested
    }

    if ($distros.Count -eq 1) {
        $name = $distros[0]
        Write-Info "Found distro: $name"
        $confirm = Read-Host "  ? Use this distro? [Y/n]"
        if ($confirm -eq "n" -or $confirm -eq "N") { exit 0 }
        return $name
    }

    Write-Info "Available WSL distros:"
    for ($i = 0; $i -lt $distros.Count; $i++) {
        Write-Host "    [$($i + 1)] $($distros[$i])"
    }

    do {
        $choice = Read-Host "  ? Select distro (1-$($distros.Count))"
        $idx = [int]$choice - 1
    } while ($idx -lt 0 -or $idx -ge $distros.Count)

    return $distros[$idx]
}

# ── Username prompt ──────────────────────────────────────────────

function Get-LinuxUsername {
    param([string]$Requested)

    if ($Requested) { return $Requested }

    $suggestion = $env:USERNAME.ToLower()
    $input = Read-Host "  ? Linux username [$suggestion]"
    if ([string]::IsNullOrWhiteSpace($input)) {
        return $suggestion
    }

    # Validate: alphanumeric, hyphens, underscores, starts with letter
    if ($input -notmatch '^[a-z][a-z0-9_-]*$') {
        Write-Err "Invalid username. Must start with a letter, only lowercase letters, digits, hyphens, underscores."
        exit 1
    }

    return $input
}

# ── Locate repo root ─────────────────────────────────────────────

function Get-RepoRoot {
    # Walk up from script dir to find go.mod (repo root)
    $dir = $PSScriptRoot
    if (-not $dir) { $dir = Split-Path -Parent $MyInvocation.ScriptName }
    while ($dir) {
        if (Test-Path (Join-Path $dir "go.mod")) { return $dir }
        $parent = Split-Path -Parent $dir
        if ($parent -eq $dir) { break }
        $dir = $parent
    }
    Write-Err "Cannot find dotfiles repo root (no go.mod found)"
    exit 1
}

# ── Copy setup script into distro ────────────────────────────────

function Copy-SetupScript {
    param([string]$DistroName)

    $setupScript = Join-Path $PSScriptRoot "wsl-setup.sh"
    if (-not (Test-Path $setupScript)) {
        Write-Err "Cannot find wsl-setup.sh at: $setupScript"
        exit 1
    }

    Write-Step "Copying setup script into distro..."

    # Pipe script into WSL, stripping \r on the Linux side (PS pipeline adds CRLF)
    $content = [System.IO.File]::ReadAllText($setupScript)
    $cmd = "tr -d '\r' > /tmp/wsl-setup.sh; chmod +x /tmp/wsl-setup.sh"
    $content | wsl.exe -d $DistroName -u root -- bash -c $cmd

    if ($LASTEXITCODE -ne 0) {
        throw "Failed to copy setup script into distro"
    }

    Write-Ok "Setup script ready"
}

# ── Copy hook script into distro ─────────────────────────────────

function Copy-HookScript {
    param(
        [string]$DistroName,
        [string]$HookPath,
        [string]$DestName
    )

    if (-not $HookPath) { return "" }

    if (-not (Test-Path $HookPath)) {
        Write-Err "Hook script not found: $HookPath"
        exit 1
    }

    Write-Step "Copying $DestName into distro..."
    $content = [System.IO.File]::ReadAllText($HookPath)
    $dest = "/tmp/$DestName"
    $cmd = "tr -d '\r' > $dest; chmod +x $dest"
    $content | wsl.exe -d $DistroName -u root -- bash -c $cmd

    if ($LASTEXITCODE -ne 0) {
        throw "Failed to copy $DestName into distro"
    }

    Write-Ok "$DestName ready"
    return $dest
}

# ── Corporate CA certificate ─────────────────────────────────────

function Copy-CertToDistro {
    param(
        [string]$DistroName,
        [string]$CertPath
    )

    if (-not (Test-Path $CertPath)) {
        Write-Err "Corp cert not found: $CertPath"
        exit 1
    }

    Write-Step "Copying corporate CA into distro..."

    # Certs may be DER (binary) or PEM (text); base64 the bytes so nothing is
    # mangled by newline translation on the way in.
    $bytes = [System.IO.File]::ReadAllBytes($CertPath)
    $b64 = [System.Convert]::ToBase64String($bytes)
    $dest = "/tmp/corp-ca.input"
    $b64 | wsl.exe -d $DistroName -u root -- bash -c "base64 -d > $dest"

    if ($LASTEXITCODE -ne 0) {
        throw "Failed to copy corp cert into distro"
    }

    Write-Ok "Corp cert staged at $dest"
    return $dest
}

function Install-CorpCert {
    param(
        [string]$DistroName,
        [string]$CertPath
    )

    Write-Header "Corporate CA"
    $dest = Copy-CertToDistro -DistroName $DistroName -CertPath $CertPath
    Invoke-Phase `
        -Name "Install corp CA" `
        -DistroName $DistroName `
        -User "root" `
        -Command "/tmp/wsl-setup.sh install-cert '$dest'"
}

# Probe HTTPS from inside the distro. Returns the setup script's exit code:
#   0 = HTTPS trusted, 2 = certificate verification failed, other = network error.
function Test-CertHealth {
    param([string]$DistroName)

    Write-Header "TLS Check"
    wsl.exe -d $DistroName -u root -- bash -c "/tmp/wsl-setup.sh check-tls"
    return $LASTEXITCODE
}

# ── Copy repo into distro ────────────────────────────────────────

function Copy-RepoToDistro {
    param(
        [string]$DistroName,
        [string]$LinuxUser
    )

    $repoRoot = Get-RepoRoot
    Write-Step "Copying dotfiles repo into distro..."

    # Convert Windows path to WSL path for the distro
    $wslRepoPath = wsl.exe -d $DistroName -- wslpath -u $repoRoot.Replace('\', '/')
    if ($LASTEXITCODE -ne 0) {
        Write-Warn "Could not resolve WSL path - will clone from GitHub instead"
        return ""
    }

    $wslRepoPath = $wslRepoPath.Trim()
    $targetPath = "/home/$LinuxUser/dotfiles"

    # Copy from the Windows mount into the Linux filesystem (much faster I/O)
    $cmd = "rm -rf $targetPath; cp -a '$wslRepoPath' '$targetPath'; chown -R ${LinuxUser}:${LinuxUser} '$targetPath'"
    wsl.exe -d $DistroName -u root -- bash -c $cmd

    if ($LASTEXITCODE -ne 0) {
        Write-Warn "Repo copy failed - will clone from GitHub instead"
        return ""
    }

    Write-Ok "Repo copied to $targetPath"
    return $targetPath
}

# ── Phase runners ────────────────────────────────────────────────

function Invoke-Phase {
    param(
        [string]$Name,
        [string]$DistroName,
        [string]$User,
        [string]$Command
    )

    try {
        if ($User -eq "root") {
            Invoke-WslRoot -DistroName $DistroName -Command $Command
        } else {
            Invoke-WslUser -DistroName $DistroName -Command $Command
        }
        return $true
    }
    catch {
        Write-Err "$Name failed: $_"
        $retry = Read-Host "  ? [R]etry / [S]kip / [A]bort?"
        switch ($retry.ToUpper()) {
            "R" { return Invoke-Phase -Name $Name -DistroName $DistroName -User $User -Command $Command }
            "S" { Write-Warn "Skipping $Name"; return $false }
            default { Write-Err "Aborting."; exit 1 }
        }
    }
}

# ── Banner ───────────────────────────────────────────────────────

function Show-Banner {
    $banner = @'

     _  __ _         _        _ _
  __| |/ _(_)_ _  __| |_ __ _| | |
 / _`|  _| | ' \(_-<  _/ _`| | |
 \__,_|_| |_|_||_/__/\__\__,_|_|_|

          WSL Bootstrap Wizard

'@
    Write-Host $banner -ForegroundColor Cyan
}

# ── Main ─────────────────────────────────────────────────────────

function Main {
    Show-Banner
    Test-WslInstalled

    # Step 1: Select distro
    Write-Header "Configuration"
    $selectedDistro = Select-Distro -Requested $Distro
    $linuxUser = Get-LinuxUsername -Requested $Username

    # Confirm plan
    Write-Host ""
    Write-Info "Distro:   $selectedDistro"
    Write-Info "Username: $linuxUser"
    Write-Info "Branch:   $Branch"
    Write-Info "Neovim:   $(if ($SkipNeovim) { 'skip' } else { 'build from source' })"
    Write-Info "Ghostty:  $(if ($SkipGhostty) { 'skip' } else { 'latest .deb' })"
    Write-Info "Dotfiles: $(if ($SkipDotfiles) { 'skip' } else { 'clone + install' })"
    if ($CorpCert) { Write-Info "Corp CA:  $CorpCert" }
    if ($PreHook)  { Write-Info "Pre-hook: $PreHook" }
    if ($PostHook) { Write-Info "Post-hook: $PostHook" }
    Write-Host ""

    $confirm = Read-Host "  ? Proceed with setup? [Y/n]"
    if ($confirm -eq "n" -or $confirm -eq "N") { exit 0 }

    # Step 2: Copy helper script
    Write-Header "Preparing"
    Copy-SetupScript -DistroName $selectedDistro

    # Step 3: Root setup (packages, user, wsl.conf)
    Write-Header "Root Setup"
    $rootOk = Invoke-Phase `
        -Name "Root setup" `
        -DistroName $selectedDistro `
        -User "root" `
        -Command "/tmp/wsl-setup.sh setup-root $linuxUser"

    if ($rootOk) {
        # Terminate just this distro to apply wsl.conf (default user, interop, systemd)
        # Other running WSL distros are unaffected.
        Write-Step "Terminating '$selectedDistro' to apply wsl.conf changes..."
        wsl.exe --terminate $selectedDistro
        Start-Sleep -Seconds 3
        Write-Ok "Distro restarted - interop and systemd now active"

        # Re-copy setup script and hooks (/tmp may not survive terminate)
        Copy-SetupScript -DistroName $selectedDistro
        if ($PreHook)  { $preHookDest  = Copy-HookScript -DistroName $selectedDistro -HookPath $PreHook  -DestName "pre-hook.sh" }
        if ($PostHook) { $postHookDest = Copy-HookScript -DistroName $selectedDistro -HookPath $PostHook -DestName "post-hook.sh" }

        # Install the corporate CA (if supplied) BEFORE any network phase, so
        # hooks, Neovim, Ghostty and dotfiles all inherit the trust.
        if ($CorpCert) {
            Install-CorpCert -DistroName $selectedDistro -CertPath $CorpCert
        }

        # Detect the classic corporate-proxy failure: apt works (internal mirror
        # / pre-trusted) while direct HTTPS fails to verify. Only worth checking
        # when something later actually reaches the network.
        $needsNetwork = (-not $SkipNeovim) -or (-not $SkipGhostty) -or (-not $SkipDotfiles) -or $PreHook -or $PostHook
        if ($needsNetwork) {
            $tls = Test-CertHealth -DistroName $selectedDistro
            if ($tls -eq 2) {
                Write-Err "HTTPS certificate verification failed inside the distro."
                if ($CorpCert) {
                    Write-Info "The supplied cert did not establish trust. Check that it is the"
                    Write-Info "root CA your proxy presents (export the full chain, PEM or DER)."
                } else {
                    Write-Info "apt works but direct HTTPS does not - the signature of a"
                    Write-Info "TLS-inspecting corporate proxy. Re-run with the corporate root CA:"
                    Write-Info "  .\wsl-bootstrap.ps1 -CorpCert <path-to-corp-root-ca.crt>"
                }
                exit 1
            }
        }
    }

    # Step 4: Run pre-hook
    if ($preHookDest) {
        Write-Header "Pre-hook"
        Invoke-Phase `
            -Name "Pre-hook" `
            -DistroName $selectedDistro `
            -User "root" `
            -Command "$preHookDest"
    }

    # Step 5: Build Neovim
    if (-not $SkipNeovim) {
        Write-Header "Neovim"
        Invoke-Phase `
            -Name "Neovim build" `
            -DistroName $selectedDistro `
            -User "root" `
            -Command "/tmp/wsl-setup.sh build-neovim"
    }

    # Step 5: Install Ghostty
    if (-not $SkipGhostty) {
        Write-Header "Ghostty"
        Invoke-Phase `
            -Name "Ghostty install" `
            -DistroName $selectedDistro `
            -User "root" `
            -Command "/tmp/wsl-setup.sh install-ghostty"
    }

    # Step 6: Copy repo into distro and install dotfiles
    if (-not $SkipDotfiles) {
        Write-Header "Dotfiles"
        $repoPath = Copy-RepoToDistro -DistroName $selectedDistro -LinuxUser $linuxUser
        if ($repoPath) {
            Invoke-Phase `
                -Name "Install dotfiles" `
                -DistroName $selectedDistro `
                -User "user" `
                -Command "DFINSTALL_SUDO_PASS=root /tmp/wsl-setup.sh install-dotfiles $Branch '$repoPath'"
        } else {
            Invoke-Phase `
                -Name "Install dotfiles" `
                -DistroName $selectedDistro `
                -User "user" `
                -Command "DFINSTALL_SUDO_PASS=root /tmp/wsl-setup.sh install-dotfiles $Branch"
        }
    }

    # Step 8: Run post-hook
    if ($postHookDest) {
        Write-Header "Post-hook"
        Invoke-Phase `
            -Name "Post-hook" `
            -DistroName $selectedDistro `
            -User "user" `
            -Command "$postHookDest"
    }

    # Cleanup
    wsl.exe -d $selectedDistro -u root -- rm -f /tmp/wsl-setup.sh /tmp/pre-hook.sh /tmp/post-hook.sh /tmp/corp-ca.input 2>$null

    # Summary
    Write-Header "Done"
    Write-Ok "WSL distro '$selectedDistro' is ready"
    Write-Host ""
    Write-Warn "Your password is 'root' - change it after setup with: passwd"
    Write-Host ""
    Write-Info "Next steps:"
    Write-Host "    1. Open a new terminal for $selectedDistro"
    Write-Host "    2. Change your password when prompted"
    if ($SkipDotfiles) {
        Write-Host "    3. Install dotfiles: git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles"
        Write-Host "       cd ~/dotfiles && make build && ./bin/dfinstall install all"
    } else {
        Write-Host "    3. Open a new shell or run: exec zsh"
    }
    Write-Host ""
}

Main
