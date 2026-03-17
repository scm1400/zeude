#Requires -Version 5.1
# Zeude Installation Script for Windows
# Installs the claude shim and configures PATH for telemetry collection
param(
    [string]$AgentKey = $env:ZEUDE_AGENT_KEY,
    [string]$DownloadBase = $(if ($env:ZEUDE_DOWNLOAD_BASE) { $env:ZEUDE_DOWNLOAD_BASE } else { "https://your-dashboard-url" }),
    [string]$DefaultEndpoint = "https://your-otel-collector-url/",
    [string]$DefaultDashboard = "https://your-dashboard-url"
)

$ErrorActionPreference = "Stop"

function Wait-AndExit($code) {
    Write-Host ""
    Write-Host "Press any key to exit..." -ForegroundColor DarkGray
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
    exit $code
}

$InstallDir = Join-Path $env:USERPROFILE ".zeude\bin"
$ConfigDir = Join-Path $env:USERPROFILE ".zeude"

Write-Host "Zeude Installer (Windows)" -ForegroundColor Green
Write-Host "======================================"

# 0. Check/install jq (required for hooks)
Write-Host -NoNewline "Checking jq... "
if (Get-Command jq -ErrorAction SilentlyContinue) {
    $jqVer = (jq --version 2>&1) -replace 'jq-', ''
    Write-Host $jqVer -ForegroundColor Green
} else {
    Write-Host "not found, installing..." -ForegroundColor Yellow
    $JqArch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
    $JqUrl = "https://github.com/jqlang/jq/releases/latest/download/jq-windows-$JqArch.exe"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    try {
        Invoke-WebRequest -Uri $JqUrl -OutFile (Join-Path $InstallDir "jq.exe") -UseBasicParsing
        Write-Host "OK (installed to .zeude\bin\jq.exe)" -ForegroundColor Green
    } catch {
        Write-Host "FAILED (hooks may not work)" -ForegroundColor Yellow
        Write-Host "  Install manually: winget install jqlang.jq" -ForegroundColor Yellow
    }
}

# 1. Detect platform
Write-Host -NoNewline "Detecting platform... "
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Host "FAILED" -ForegroundColor Red
    Write-Host "Error: 32-bit Windows is not supported."
    Wait-AndExit 1
}
$Platform = "windows-$Arch"
Write-Host $Platform -ForegroundColor Green

# 2. Find real claude.exe
Write-Host -NoNewline "Finding claude binary... "
$ClaudeCmd = Get-Command claude.exe -ErrorAction SilentlyContinue
if (-not $ClaudeCmd) {
    $Candidates = @(
        (Join-Path $env:USERPROFILE ".local\bin\claude.exe"),
        (Join-Path $env:APPDATA "npm\claude.cmd"),
        (Join-Path $env:LOCALAPPDATA "Programs\claude\claude.exe")
    )
    foreach ($c in $Candidates) {
        if (Test-Path $c) {
            $RealClaude = $c
            break
        }
    }
} else {
    $RealClaude = $ClaudeCmd.Source
}

if (-not $RealClaude) {
    Write-Host "FAILED" -ForegroundColor Red
    Write-Host "Error: claude.exe not found. Please install Claude Code first."
    Write-Host "  Visit: https://www.anthropic.com/claude-code"
    Wait-AndExit 1
}

# Ensure we didn't find our own shim
if ($RealClaude -like "*\.zeude\bin\*") {
    $PathFile = Join-Path $ConfigDir "real_binary_path"
    if (Test-Path $PathFile) {
        $RealClaude = (Get-Content $PathFile -Raw).Trim()
    } else {
        Write-Host "FAILED" -ForegroundColor Red
        Write-Host "Error: Could not find original claude binary (only found zeude shim)."
        Wait-AndExit 1
    }
}

Write-Host $RealClaude -ForegroundColor Green

# 3. Create directories
Write-Host -NoNewline "Creating directories... "
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Write-Host "OK" -ForegroundColor Green

# 4. Store real binary path
Write-Host -NoNewline "Storing real binary path... "
Set-Content -Path (Join-Path $ConfigDir "real_binary_path") -Value $RealClaude -NoNewline
Write-Host "OK" -ForegroundColor Green

# 5. Download pre-built binaries
Write-Host -NoNewline "Downloading zeude shim... "
$ShimUrl = "$DownloadBase/releases/claude-$Platform.exe"
$ShimPath = Join-Path $InstallDir "claude.exe"
$ShimTmp = Join-Path $InstallDir "claude.exe.new"
try {
    Invoke-WebRequest -Uri $ShimUrl -OutFile $ShimTmp -UseBasicParsing
    # If existing file is locked (running process), rename it first
    if (Test-Path $ShimPath) {
        $ShimOld = Join-Path $InstallDir "claude.exe.old"
        Remove-Item $ShimOld -Force -ErrorAction SilentlyContinue
        try {
            Rename-Item $ShimPath $ShimOld -Force
        } catch {
            Write-Host "WARNING" -ForegroundColor Yellow
            Write-Host "  Existing claude.exe is locked. New version saved as claude.exe.new"
            Write-Host "  Close all Claude sessions and rename manually, or re-run installer."
            Write-Host ""
        }
    }
    if (-not (Test-Path $ShimPath)) {
        Rename-Item $ShimTmp "claude.exe" -Force
    }
    Write-Host "OK" -ForegroundColor Green
} catch {
    Write-Host "FAILED" -ForegroundColor Red
    Write-Host "Error: Failed to download from $ShimUrl"
    Write-Host "Detail: $($_.Exception.Message)" -ForegroundColor Yellow
    Wait-AndExit 1
}

Write-Host -NoNewline "Downloading zeude doctor... "
$DoctorUrl = "$DownloadBase/releases/zeude-$Platform.exe"
$DoctorPath = Join-Path $InstallDir "zeude.exe"
$DoctorTmp = Join-Path $InstallDir "zeude.exe.new"
try {
    Invoke-WebRequest -Uri $DoctorUrl -OutFile $DoctorTmp -UseBasicParsing
    if (Test-Path $DoctorPath) {
        $DoctorOld = Join-Path $InstallDir "zeude.exe.old"
        Remove-Item $DoctorOld -Force -ErrorAction SilentlyContinue
        try { Rename-Item $DoctorPath $DoctorOld -Force } catch { }
    }
    if (-not (Test-Path $DoctorPath)) {
        Rename-Item $DoctorTmp "zeude.exe" -Force
    }
    Write-Host "OK" -ForegroundColor Green
} catch {
    Write-Host "SKIPPED (optional)" -ForegroundColor Yellow
}

# 6. Configure defaults
Write-Host -NoNewline "Configuring defaults... "
$Endpoint = if ($env:ZEUDE_ENDPOINT) { $env:ZEUDE_ENDPOINT } else { $DefaultEndpoint }
$DashUrl = if ($env:ZEUDE_DASHBOARD_URL) { $env:ZEUDE_DASHBOARD_URL } else { $DefaultDashboard }
Set-Content -Path (Join-Path $ConfigDir "config") -Value "endpoint=$Endpoint`ndashboard_url=$DashUrl"
Write-Host "OK" -ForegroundColor Green

# 7. Add to PATH (User environment variable, persistent)
Write-Host -NoNewline "Configuring PATH... "
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*\.zeude\bin*") {
    $NewPath = "$InstallDir;$UserPath"
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
    $env:PATH = "$InstallDir;$env:PATH"
    Write-Host "Configured" -ForegroundColor Green
} else {
    $Parts = $UserPath -split ";" | Where-Object { $_ -notlike "*\.zeude\bin*" }
    $NewPath = (@($InstallDir) + $Parts) -join ";"
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
    $env:PATH = "$InstallDir;$($env:PATH -replace [regex]::Escape($InstallDir) + ';?', '')"
    Write-Host "Updated (moved to front)" -ForegroundColor Green
}

# 8. Install /zeude skill
Write-Host -NoNewline "Installing /zeude skill... "
$SkillDir = Join-Path $env:USERPROFILE ".claude\commands"
New-Item -ItemType Directory -Path $SkillDir -Force | Out-Null
$SkillContent = @'
---
name: zeude
description: Open Zeude dashboard (auto-login)
allowed-tools: Bash, Read
---

Open the Zeude monitoring dashboard in your browser with automatic authentication.

## Steps

1. **Read Agent Key**
   - Read the agent key from `~/.zeude/credentials`
   - The file contains `agent_key=zd_xxxxxxxx`

2. **Get Dashboard URL**
   - Read the server URL from `~/.zeude/config`
   - Look for `dashboard_url=` line

3. **Request One-Time Token**
   - POST to `{dashboard_url}/api/auth/ott` with JSON body: `{"agentKey": "<agent_key>"}`
   - Extract the `token` from the response

4. **Open Browser**
   - Open `{dashboard_url}/auth?ott={token}` in the default browser
   - Use `start` on Windows, `open` on macOS, or `xdg-open` on Linux
'@
Set-Content -Path (Join-Path $SkillDir "zeude.md") -Value $SkillContent
Write-Host "OK" -ForegroundColor Green

# 9. Configure agent key
if (-not $AgentKey) {
    Write-Host ""
    Write-Host "Agent key required for telemetry collection." -ForegroundColor Yellow
    Write-Host "Get your key from: $DefaultDashboard" -ForegroundColor Cyan
    Write-Host ""
    $AgentKey = Read-Host "Enter your agent key (zd_xxx)"
}

if ($AgentKey) {
    Write-Host -NoNewline "Configuring agent key... "
    if ($AgentKey -match "^zd_") {
        $credPath = Join-Path $ConfigDir "credentials"
        Set-Content -Path $credPath -Value "agent_key=$AgentKey"
        # Restrict permissions to current user only (equivalent to chmod 600)
        $acl = Get-Acl $credPath
        $acl.SetAccessRuleProtection($true, $false)
        $rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
            [System.Security.Principal.WindowsIdentity]::GetCurrent().Name,
            "FullControl", "Allow")
        $acl.SetAccessRule($rule)
        Set-Acl $credPath $acl
        Write-Host "OK" -ForegroundColor Green
    } else {
        Write-Host "Invalid key format (should start with zd_)" -ForegroundColor Red
    }
} else {
    Write-Host "Skipped - No agent key provided" -ForegroundColor Yellow
    Write-Host "Add it later: Set-Content ~\.zeude\credentials 'agent_key=YOUR_KEY'"
}

# 10. Summary
Write-Host ""
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "======================================"
Write-Host "Real claude:  $RealClaude"
Write-Host "Shim binary:  $ShimPath"
Write-Host "Config dir:   $ConfigDir"
Write-Host "Dashboard:    $DefaultDashboard"
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Open a NEW terminal (PATH changes require restart)"
Write-Host "  2. Run 'zeude doctor' to verify installation" -ForegroundColor Cyan
Write-Host "  3. Run 'claude' to start with telemetry enabled" -ForegroundColor Cyan
Write-Host "  4. Type '/zeude' in Claude to open the dashboard" -ForegroundColor Cyan
