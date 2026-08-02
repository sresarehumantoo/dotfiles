<#
.SYNOPSIS
    Probe a Windows host to determine whether dotfiles-box can run, and how.

.DESCRIPTION
    Answers the questions that decide the container-as-OS design, without
    changing anything:

      * Which virtualization features are enabled (Hyper-V vs VirtualMachinePlatform
        vs WSL) -- these are SEPARATE optional features and an admin can enable one
        and disable the others.
      * Whether this account can create Hyper-V VMs, versus merely use Docker.
        These are different permissions: Docker Desktop ships a LocalSystem helper
        service plus a 'docker-users' group, so a user with no VM-creation rights
        can still run containers. "Containers yes, VMs no" is a real configuration.
      * Which Docker Desktop backend is active, and whether the version supports
        Synchronized File Shares (4.27+, and on the Hyper-V backend specifically).

    Read-only. Makes no changes. Safe to run as a standard user; a few checks
    report "needs elevation" rather than failing.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\probe.ps1
#>

[CmdletBinding()]
param()

$ErrorActionPreference = 'Continue'

function Write-Section { param($T) Write-Host ""; Write-Host "== $T " -ForegroundColor Cyan -NoNewline; Write-Host ("=" * [Math]::Max(0, 58 - $T.Length)) -ForegroundColor Cyan }
function Write-Ok      { param($M) Write-Host "  [+] " -ForegroundColor Green  -NoNewline; Write-Host $M }
function Write-No      { param($M) Write-Host "  [-] " -ForegroundColor Red    -NoNewline; Write-Host $M }
function Write-Warn2   { param($M) Write-Host "  [!] " -ForegroundColor Yellow -NoNewline; Write-Host $M }
function Write-Info2   { param($M) Write-Host "  [i] " -ForegroundColor DarkGray -NoNewline; Write-Host $M }

Write-Host ""
Write-Host "  dotfiles-box host probe" -ForegroundColor White
Write-Host "  $env:COMPUTERNAME  |  $(Get-Date -Format 'yyyy-MM-dd HH:mm')" -ForegroundColor DarkGray

# -- OS -----------------------------------------------------------
Write-Section "Operating system"
$os = Get-CimInstance Win32_OperatingSystem
Write-Info2 "$($os.Caption)  build $($os.BuildNumber)"
# ProductType: 1 = workstation, 2 = domain controller, 3 = server
if ($os.ProductType -ne 1) {
    Write-Warn2 "This is Windows Server. Docker Desktop is NOT supported on Windows Server."
    Write-Info2 "Docker CE / Mirantis is the Server path, and it does not provide the"
    Write-Info2 "Hyper-V LinuxKit VM that this design assumes."
} else {
    Write-Ok "Client Windows -- Docker Desktop is supported here"
}

# -- Optional features --------------------------------------------
# These are independent. WSL2 needs VirtualMachinePlatform; Docker Desktop's
# Hyper-V backend needs Microsoft-Hyper-V. Enabling one says nothing about the others.
Write-Section "Virtualization features"
$features = @(
    'Microsoft-Hyper-V',
    'Microsoft-Hyper-V-Hypervisor',
    'VirtualMachinePlatform',
    'Microsoft-Windows-Subsystem-Linux',
    'Containers'
)
foreach ($f in $features) {
    try {
        $state = (Get-WindowsOptionalFeature -Online -FeatureName $f -ErrorAction Stop).State
        if ($state -eq 'Enabled') { Write-Ok "$f = Enabled" } else { Write-No "$f = $state" }
    } catch {
        Write-Warn2 "$f = unknown (needs elevation, or feature not present on this SKU)"
    }
}

# -- Permissions: can this account create VMs? --------------------
Write-Section "This account's rights"
$me = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($me)
Write-Info2 "user: $($me.Name)"
if ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Ok "running elevated (Administrator)"
} else {
    Write-Info2 "not elevated"
}

# Well-known SIDs beat localized group names (works on non-English Windows).
$groupSids = $me.Groups | ForEach-Object { $_.Value }
$checks = @{
    'S-1-5-32-544' = 'Administrators'
    'S-1-5-32-578' = 'Hyper-V Administrators'
}
foreach ($sid in $checks.Keys) {
    if ($groupSids -contains $sid) { Write-Ok "member of $($checks[$sid])" }
    else { Write-No "NOT a member of $($checks[$sid])" }
}
# docker-users is a local group created by the Docker Desktop installer; it has
# no well-known SID, so match by name.
try {
    $dockerUsers = Get-LocalGroupMember -Group 'docker-users' -ErrorAction Stop |
                   Where-Object { $_.Name -like "*$($env:USERNAME)" }
    if ($dockerUsers) { Write-Ok "member of docker-users" } else { Write-No "NOT a member of docker-users" }
} catch {
    Write-Info2 "docker-users group not present (Docker Desktop may not be installed)"
}

# The decisive test: can we actually create a VM? -WhatIf changes nothing.
Write-Section "Can this account create a Hyper-V VM?"
if (Get-Command New-VM -ErrorAction SilentlyContinue) {
    try {
        New-VM -Name 'dotfiles-box-probe' -MemoryStartupBytes 512MB -WhatIf -ErrorAction Stop | Out-Null
        Write-Ok "New-VM -WhatIf succeeded -- VM creation appears permitted"
    } catch {
        Write-No "New-VM refused: $($_.Exception.Message)"
        Write-Info2 "If Docker still works below, this is the 'containers yes, VMs no' split."
    }
} else {
    Write-No "New-VM cmdlet unavailable (Hyper-V management tools not installed)"
}

# -- Docker -------------------------------------------------------
Write-Section "Docker"
if (Get-Command docker -ErrorAction SilentlyContinue) {
    $sv = (docker version --format '{{.Server.Version}}' 2>$null)
    if ($LASTEXITCODE -eq 0 -and $sv) {
        Write-Ok "docker engine reachable -- server $sv"
        $osType = (docker info --format '{{.OSType}}' 2>$null)
        $kernel = (docker info --format '{{.KernelVersion}}' 2>$null)
        Write-Info2 "OSType=$osType  Kernel=$kernel"
        if ($kernel -match 'linuxkit') {
            Write-Ok "LinuxKit kernel -- Hyper-V (or WSL2) Docker Desktop VM confirmed"
        }
        if ($kernel -match 'microsoft-standard-WSL2') {
            Write-Warn2 "This is the WSL2 backend, not Hyper-V. If WSL is available,"
            Write-Warn2 "reconsider whether you need this container at all."
        }
    } else {
        Write-No "docker CLI present but the engine is not responding"
    }
} else {
    Write-No "docker not on PATH"
}

# Docker Desktop version gates Synchronized File Shares: 4.27+ for the feature,
# and Hyper-V was the only Windows backend supporting it until 4.78.0 (2026-06-15).
$ddPaths = @(
    "$env:ProgramFiles\Docker\Docker\Docker Desktop.exe",
    "${env:ProgramFiles(x86)}\Docker\Docker\Docker Desktop.exe"
)
$dd = $ddPaths | Where-Object { Test-Path $_ } | Select-Object -First 1
if ($dd) {
    $v = (Get-Item $dd).VersionInfo.ProductVersion
    Write-Info2 "Docker Desktop $v"
    try {
        $parsed = [version]($v -split '-' | Select-Object -First 1)
        if ($parsed -ge [version]'4.27') {
            Write-Ok "Synchronized File Shares available (4.27+) -- requires a paid Pro/Team/Business subscription"
        } else {
            Write-No "Synchronized File Shares needs Docker Desktop 4.27+"
        }
    } catch { Write-Info2 "could not parse version '$v'" }
} else {
    Write-Info2 "Docker Desktop not found in Program Files"
}

# -- SSHFS-Win (the \\wsl$ analogue) ------------------------------
Write-Section "SSHFS-Win (Windows -> container filesystem)"
$sshfs = "$env:ProgramFiles\SSHFS-Win\bin\sshfs-win.exe"
if (Test-Path $sshfs) {
    Write-Ok "SSHFS-Win installed"
} else {
    Write-No "SSHFS-Win not installed -- needed to reach /work from Windows editors"
    Write-Info2 "winget install SSHFS-Win.SSHFS-Win  (also installs WinFsp)"
    Write-Warn2 "Note: the Windows port has shipped nothing since Feb 2021 and has"
    Write-Warn2 "documented bugs with the !PORT UNC form against localhost."
}

Write-Host ""
Write-Host "  Probe complete. Nothing was changed." -ForegroundColor DarkGray
Write-Host ""
