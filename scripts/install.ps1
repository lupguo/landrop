# LanDrop installer for Windows (PowerShell 5.1+).
#
#   irm https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.ps1 | iex
#
# Environment:
#   LANDROP_VERSION         tag to install, e.g. v0.1.0 (default: latest)
#   LANDROP_DOWNLOAD_BASE   override the download base URL (for mirrors and testing)
#
# Everything lives in a function so that a failure uses `throw` and never `exit`,
# which would close the user's PowerShell window when run through `iex`.

function Install-LanDrop {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'   # progress bar makes Invoke-WebRequest very slow on 5.1
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $releasesUrl = 'https://github.com/lupguo/landrop/releases'

    # --- platform (PROCESSOR_ARCHITEW6432 is set when a 32-bit PowerShell runs on 64-bit Windows)
    $arch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    if ($arch -ne 'AMD64') {
        throw "unsupported architecture '$arch'. Download a build manually from $releasesUrl"
    }

    $archive = 'landrop_windows_amd64.zip'

    # --- download base
    if ($env:LANDROP_DOWNLOAD_BASE) {
        $base = $env:LANDROP_DOWNLOAD_BASE
    } elseif ($env:LANDROP_VERSION) {
        $base = "$releasesUrl/download/$($env:LANDROP_VERSION)"
    } else {
        $base = "$releasesUrl/latest/download"
    }
    $base = $base.TrimEnd('/')

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("landrop-install-" + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading $archive from $base ..."
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$archive" -OutFile (Join-Path $tmp $archive)
        } catch {
            throw "failed to download $base/$archive ($($_.Exception.Message))"
        }
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')
        } catch {
            throw "failed to download $base/checksums.txt ($($_.Exception.Message))"
        }

        # --- verify (lines look like "<sha256>  <filename>"; a missing entry is a failure)
        $expected = $null
        foreach ($line in Get-Content -LiteralPath (Join-Path $tmp 'checksums.txt')) {
            $parts = $line.Trim() -split '\s+'
            if ($parts.Count -ge 2 -and $parts[1].TrimStart('*') -eq $archive) {
                $expected = $parts[0]
                break
            }
        }
        if (-not $expected) {
            throw "no checksum entry for $archive in checksums.txt"
        }
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $tmp $archive)).Hash
        if ($expected -ine $actual) {
            throw "checksum mismatch for $archive (expected $expected, got $actual); nothing installed"
        }
        Write-Host 'Checksum OK.'

        # --- extract to a staging dir first so a bad archive installs nothing
        $stage = Join-Path $tmp 'extract'
        Expand-Archive -LiteralPath (Join-Path $tmp $archive) -DestinationPath $stage -Force
        if (-not (Test-Path -LiteralPath (Join-Path $stage 'landrop.exe'))) {
            throw 'archive does not contain landrop.exe'
        }

        # --- install
        $dir = Join-Path $env:LOCALAPPDATA 'Programs\landrop'
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        try {
            Copy-Item -Path (Join-Path $stage '*') -Destination $dir -Recurse -Force
        } catch {
            throw "cannot write to $dir ($($_.Exception.Message)). Is landrop still running? Close it and retry."
        }

        # --- PATH (user level, no admin needed); never add a duplicate entry
        $norm = { param($p) $p.Trim().TrimEnd('\') }
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $userEntries = @(($userPath -split ';') | Where-Object { $_ })
        if (-not ($userEntries | Where-Object { (& $norm $_) -ieq (& $norm $dir) })) {
            $newPath = if ($userPath) { $userPath.TrimEnd(';') + ';' + $dir } else { $dir }
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        }
        $sessionEntries = @(($env:Path -split ';') | Where-Object { $_ })
        if (-not ($sessionEntries | Where-Object { (& $norm $_) -ieq (& $norm $dir) })) {
            $env:Path = $env:Path.TrimEnd(';') + ';' + $dir
        }

        Write-Host "Installed to $dir\landrop.exe"
        & (Join-Path $dir 'landrop.exe') --version
        Write-Host ''
        Write-Host 'Open a new terminal, then run: landrop'
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Install-LanDrop
