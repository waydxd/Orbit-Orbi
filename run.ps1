# ====================================
# Portable Go & Make Environment Script (PowerShell)
# Single supported script. Idempotent. Non-admin friendly.
# Usage: .\run.ps1 [-AutoConfirm]
# ====================================
param(
    [switch]$AutoConfirm
)

# Defaults (change if you extracted elsewhere)
$env:GOROOT = "$env:USERPROFILE\Go\go"
$env:GOPATH = "$env:USERPROFILE\Go\gopath"
$env:MAKE_HOME = "$env:USERPROFILE\make"

# Ensure parent folders exist
New-Item -ItemType Directory -Force -Path (Join-Path $env:USERPROFILE 'Go') | Out-Null
New-Item -ItemType Directory -Force -Path $env:MAKE_HOME | Out-Null
New-Item -ItemType Directory -Force -Path $env:GOPATH | Out-Null

# Update PATH for this session
$env:PATH = "$($env:GOROOT)\bin;$($env:GOPATH)\bin;$($env:MAKE_HOME)\bin;$env:PATH"

function Prompt-YesNo {
    param([string]$Message)
    if ($AutoConfirm) { return $true }
    $r = Read-Host "$Message [Y/N]"
    return ($r -match '^[Yy]')
}

function Get-LatestGoUrl {
    try {
        $json = Invoke-RestMethod -Uri 'https://go.dev/dl/?mode=json' -UseBasicParsing -ErrorAction Stop
        foreach ($entry in $json) {
            $file = $entry.files | Where-Object { $_.os -eq 'windows' -and $_.arch -eq 'amd64' -and $_.kind -eq 'archive' } | Select-Object -First 1
            if ($file) { return "https://go.dev$($file.url)" }
        }
    } catch {
        return $null
    }
}

function Download-And-ExtractZip {
    param(
        [string]$Url,
        [string]$Destination
    )
    $tmp = [System.IO.Path]::Combine($env:TEMP, [System.IO.Path]::GetFileName($Url))
    Write-Host "Downloading $Url..." -ForegroundColor Cyan
    try {
        Invoke-WebRequest -Uri $Url -OutFile $tmp -UseBasicParsing -ErrorAction Stop
    } catch {
        Write-Error "Failed to download $Url : $_"
        return $false
    }
    Write-Host "Extracting to $Destination..." -ForegroundColor Cyan
    try {
        Expand-Archive -LiteralPath $tmp -DestinationPath $Destination -Force
        Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
        return $true
    } catch {
        Write-Error "Failed to extract $tmp : $_"
        return $false
    }
}

# Check Go
Write-Host "====================================" -ForegroundColor Cyan
Write-Host "Checking installed tools..." -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan

$goExe = Join-Path $env:GOROOT 'bin\go.exe'
if (Test-Path $goExe) {
    Write-Host "✓ Go found at $goExe" -ForegroundColor Green
} else {
    Write-Host "✗ Go not found at $goExe" -ForegroundColor Yellow
    if (Prompt-YesNo "Download and install portable Go into $($env:GOROOT)?") {
        $goUrl = Get-LatestGoUrl
        if (-not $goUrl) {
            Write-Error "Unable to determine latest Go download URL. You can download manually from https://go.dev/dl/"
        } else {
            $parent = Split-Path $env:GOROOT -Parent
            # Ensure parent exists
            New-Item -ItemType Directory -Force -Path $parent | Out-Null
            $ok = Download-And-ExtractZip -Url $goUrl -Destination $parent
            if ($ok -and Test-Path $goExe) {
                Write-Host "✓ Go installed to $env:GOROOT" -ForegroundColor Green
                # After extracting, the archive creates a 'go' folder under parent
                # Ensure environment path includes new bin
                $env:PATH = "$($env:GOROOT)\bin;$env:PATH"
            } else {
                Write-Error "Go installation failed."
            }
        }
    } else {
        Write-Host "Skipping Go install. Commands requiring Go will fail until installed." -ForegroundColor Yellow
    }
}

Write-Host ""

# Check Make
$makeExe = Join-Path $env:MAKE_HOME 'bin\make.exe'
if (Test-Path $makeExe) {
    Write-Host "✓ Make found at $makeExe" -ForegroundColor Green
} else {
    Write-Host "✗ Make not found at $makeExe" -ForegroundColor Yellow
    if (Prompt-YesNo "Download and install Make (ezwinports) into $($env:MAKE_HOME)?") {
        $makeUrl = 'https://sourceforge.net/projects/ezwinports/files/make-4.4.1-without-guile-w32-bin.zip/download'
        $ok = Download-And-ExtractZip -Url $makeUrl -Destination $env:MAKE_HOME
        if ($ok -and Test-Path $makeExe) {
            Write-Host "✓ Make installed to $env:MAKE_HOME" -ForegroundColor Green
            $env:PATH = "$($env:MAKE_HOME)\bin;$env:PATH"
        } else {
            Write-Error "Make installation failed. If the zip structure is different, extract manually to $env:MAKE_HOME\bin"
        }
    } else {
        Write-Host "Skipping Make install. You can still run commands directly without make." -ForegroundColor Yellow
    }
}

Write-Host ""

# Check Protoc (optional)
try {
    $p = & protoc --version 2>$null
    if ($LASTEXITCODE -eq 0) { Write-Host "✓ Protoc: $p" -ForegroundColor Green } else { throw }
} catch {
    Write-Host "✗ Protoc not found in PATH. 'make proto' requires protoc." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "====================================" -ForegroundColor Cyan
Write-Host "Final check: tool versions" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan

try { & go version } catch {}
try { & make --version } catch {}
try { & protoc --version } catch {}

Write-Host ""
Write-Host "Environment setup complete for this session." -ForegroundColor Cyan
Write-Host "You can now run: make build, make run, make test, or run commands directly." -ForegroundColor Cyan
Write-Host "To persist environment permanently, add the above paths to your user PATH." -ForegroundColor Cyan

# End
