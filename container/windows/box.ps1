<#
.SYNOPSIS
    `wsl`-equivalent entry point for the dotfiles-box container.

.DESCRIPTION
    One command to enter, manage and address the box, mirroring the ergonomics
    of the `wsl` CLI:

        box                 enter the box (drops into zsh -> tmux session 'main')
        box ssh             enter over SSH instead of docker exec
        box run <cmd...>    run a command inside the box and return
        box up / down       start / stop the container
        box status          container + volume state
        box path <path>     translate a Windows path to its in-box equivalent
        box mount [X]       mount the box's /work as a Windows drive (SSHFS-Win)
        box unmount [X]     unmount it

.NOTES
    Entry mode: `docker exec` is the default because it needs no key material and
    no published port. Use `box ssh` when you want agent forwarding, or when a
    Windows-side tool needs a real SSH endpoint.

    IMPORTANT: `docker exec` must always allocate BOTH -i and -t. The container's
    zshrc auto-execs `tmux new-session -A -s main` on an interactive shell; with
    -t but no -i, tmux gets a tty and no stdin and hangs.
#>

[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Command = 'enter',

    [Parameter(Position = 1, ValueFromRemainingArguments = $true)]
    [string[]]$Rest
)

$ErrorActionPreference = 'Stop'

$BoxName    = if ($env:BOX_NAME)     { $env:BOX_NAME }     else { 'box' }
$BoxUser    = if ($env:BOX_USER)     { $env:BOX_USER }     else { 'owen' }
$BoxPort    = if ($env:BOX_SSH_PORT) { $env:BOX_SSH_PORT } else { '2222' }
$BoxWorkDir = '/work'
# Repo root, two levels up from container/windows/
$RepoRoot   = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$Compose    = Join-Path $RepoRoot 'container\compose.yaml'

function Test-BoxRunning {
    $s = (docker ps --filter "name=^/$BoxName$" --format '{{.Names}}' 2>$null)
    return [bool]$s
}

function Assert-BoxRunning {
    if (-not (Test-BoxRunning)) {
        Write-Host "box is not running. Starting it..." -ForegroundColor Yellow
        docker compose -f $Compose up -d
        Start-Sleep -Seconds 2
        if (-not (Test-BoxRunning)) { throw "box failed to start. Check: docker compose -f `"$Compose`" logs" }
    }
}

switch ($Command.ToLower()) {

    'enter' {
        Assert-BoxRunning
        # TERM must be xterm* or the clipboard silently breaks. tmux decides
        # whether to relay OSC 52 outward from its BUILT-IN terminal-features
        # table keyed on TERM, not from terminfo's Ms capability (measured:
        # xterm-256color relays, screen-256color and vt100 do not, and none of
        # the three advertise Ms). Windows PowerShell normally leaves $env:TERM
        # unset, which would pass an empty TERM and lose the clipboard with no
        # error, so default it here.
        $termValue = if ($env:TERM) { $env:TERM } else { 'xterm-256color' }
        # -i AND -t: see the note in .NOTES about the tmux autostart.
        docker exec -it -u $BoxUser -e TERM=$termValue -e COLORTERM=truecolor $BoxName zsh -l
    }

    'ssh' {
        Assert-BoxRunning
        # SendEnv propagates truecolor; the container's sshd has a matching AcceptEnv.
        ssh -p $BoxPort -o SendEnv=COLORTERM "$BoxUser@127.0.0.1" @Rest
    }

    'run' {
        Assert-BoxRunning
        if (-not $Rest) { throw "usage: box run <command...>" }
        docker exec -i -u $BoxUser $BoxName zsh -lc ($Rest -join ' ')
    }

    'up'     { docker compose -f $Compose up -d }
    'down'   { docker compose -f $Compose stop }

    'status' {
        docker ps -a --filter "name=^/$BoxName$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
        Write-Host ""
        docker volume ls --filter 'name=box-' --format 'table {{.Name}}\t{{.Driver}}'
    }

    'path' {
        # The wslpath analogue. There is no /mnt/c here: the box's source tree
        # lives on a named volume, and Windows reaches INTO it over SSHFS-Win.
        # So translation runs the other way -- a mapped drive letter back to /work.
        if (-not $Rest) { throw "usage: box path <windows-path>" }
        $p = ($Rest -join ' ')
        $full = try { [IO.Path]::GetFullPath($p) } catch { $p }
        if ($full -match '^([A-Za-z]):\\(.*)$') {
            $drive = $Matches[1]
            $tail  = $Matches[2] -replace '\\', '/'
            # Is this drive letter the SSHFS mount of the box?
            $unc = (Get-PSDrive -Name $drive -ErrorAction SilentlyContinue).DisplayRoot
            if ($unc -match 'sshfs') {
                Write-Output ("$BoxWorkDir/$tail" -replace '//', '/')
            } else {
                Write-Warning "$drive`: is not an SSHFS mount of the box."
                Write-Warning "Files under it are on the Windows filesystem and are NOT visible in the box"
                Write-Warning "unless you bind-mount them (slow: gRPC-FUSE across the VM boundary)."
            }
        } else {
            throw "not an absolute Windows path: $p"
        }
    }

    'mount' {
        $drive = if ($Rest) { $Rest[0].TrimEnd(':') } else { 'X' }
        $sshfs = "$env:ProgramFiles\SSHFS-Win\bin\sshfs-win.exe"
        if (-not (Test-Path $sshfs)) { throw "SSHFS-Win not installed. winget install SSHFS-Win.SSHFS-Win" }
        Assert-BoxRunning
        # .kr, NOT .k -- measured on a real Windows host 2026-08-02. ".k" is
        # relative to the remote HOME, so an absolute path like /work fails with
        # "The network name cannot be found"; ".kr" is relative to the remote
        # ROOT and is the only prefix that resolves /work. The drive argument
        # also needs its colon.
        #
        # Contrary to the open bug reports, the !PORT form resolved correctly on
        # the first attempt -- no workaround needed for that part.
        net use "${drive}:" "\\sshfs.kr\$BoxUser@127.0.0.1!$BoxPort$BoxWorkDir"
        Write-Host "Mounted box:$BoxWorkDir at ${drive}:" -ForegroundColor Green
        Write-Warning "If file CREATION fails with 'access denied' while leaving a 0-byte"
        Write-Warning "file behind, you are hitting winfsp/sshfs-win issues #322 / #186 --"
        Write-Warning "a known bug in ELEVATED sessions. Retry from a normal, non-elevated"
        Write-Warning "shell; per those reports it does not affect ordinary user sessions."
        Write-Warning "Reads and writes to existing files are unaffected. See container/README.md."
    }

    'unmount' {
        $drive = if ($Rest) { $Rest[0].TrimEnd(':') } else { 'X' }
        net use "${drive}:" /delete
    }

    default {
        Write-Host @"
box -- WSL-equivalent entry for the dotfiles-box container

  box                  enter (docker exec -> zsh -> tmux 'main')
  box ssh [args]       enter over SSH
  box run <cmd...>     run a command and return
  box up | down        start / stop
  box status           container + volume state
  box path <winpath>   translate a Windows path to its in-box equivalent
  box mount [drive]    mount /work as a Windows drive via SSHFS-Win (default X)
  box unmount [drive]  unmount

Environment: BOX_NAME=$BoxName  BOX_USER=$BoxUser  BOX_SSH_PORT=$BoxPort
"@
    }
}
