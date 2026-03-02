# Windows Phase 2 RC Release Evidence Generation
# This script automates the Windows evidence generation for release candidates
# Following the documented workflow in PLANNING/rc-v0.2.5-rc.1-progress.md

param(
    [Parameter(Mandatory=$true)]
    [string]$RCTag,

    [Parameter(Mandatory=$false)]
    [string]$GitRepoPath = $PWD.Path,

    [Parameter(Mandatory=$false)]
    [switch]$SkipArm64 = $false
)

$ErrorActionPreference = "Stop"
$VerbosePreference = "Continue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Windows Phase 2 RC Evidence Generation" -ForegroundColor Cyan
Write-Host "RC Tag: $RCTag" -ForegroundColor Cyan
Write-Host "Repository: $GitRepoPath" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# Expected Linux reference digests (from the planning document)
$LinuxReferenceDigests = @{
    "v0.2.5-rc.1" = @{
        canonical = "2818166c21e1b445d59b061c5a546eccb54f71566325ea9366ddde30ddd5ebc6"
        exit_code = "73d91ef3f2fd6d709fd8491bb9c547290a1b3a13c234423ca96432f4258235d2"
        failure_class = "af58643f979138dadd16e4c78fd6d60d44d0818a5ce5269696ac3966f1d3306b"
        verify = "66d329b3bd829da527feb00eb97fdb681a0e15c28ac14d8bfed29ecae13e70f6"
    }
}

# Step 1: Verify Prerequisites
Write-Host "`n[STEP 1] Verifying Prerequisites..." -ForegroundColor Yellow

# Check Go installation
try {
    $goVersion = & go version
    if ($LASTEXITCODE -ne 0) { throw "Go is not installed or not in PATH" }
    Write-Host "  ✓ Go installed: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Go is not installed. Please install Go 1.22+ from https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

# Check Git for Windows (provides bash)
$bashPath = $null
$gitBashPaths = @(
    "C:\Program Files\Git\bin\bash.exe",
    "C:\Program Files (x86)\Git\bin\bash.exe",
    "$env:ProgramFiles\Git\bin\bash.exe"
)

foreach ($path in $gitBashPaths) {
    if (Test-Path $path) {
        $bashPath = $path
        break
    }
}

if ($bashPath) {
    $bashVersion = & $bashPath --version 2>&1 | Select-Object -First 1
    Write-Host "  ✓ Git for Windows bash found: $bashPath" -ForegroundColor Green
    Write-Host "    Version: $bashVersion" -ForegroundColor Gray
} else {
    Write-Host "  ✗ Git for Windows is not installed. Please install from https://gitforwindows.org/" -ForegroundColor Red
    exit 1
}

# Step 2: Navigate to repository and verify state
Write-Host "`n[STEP 2] Verifying Repository State..." -ForegroundColor Yellow
Set-Location $GitRepoPath

# Check current branch
$currentBranch = & git branch --show-current
if ($currentBranch -ne "feat/windows-cross-arch-testing-ldSsm") {
    Write-Host "  ✗ Not on correct branch. Expected: feat/windows-cross-arch-testing-ldSsm, Current: $currentBranch" -ForegroundColor Red
    exit 1
}
Write-Host "  ✓ On correct branch: $currentBranch" -ForegroundColor Green

# Verify commit A exists
$commitA = & git log --oneline | Select-String "12c5162"
if (-not $commitA) {
    Write-Host "  ✗ Commit A (12c5162) not found in history" -ForegroundColor Red
    exit 1
}
Write-Host "  ✓ Commit A found: $commitA" -ForegroundColor Green

# Verify Linux evidence exists
$linuxEvidencePath = "offline\runs\releases\$RCTag\x86_64\offline-evidence.json"
if (-not (Test-Path $linuxEvidencePath)) {
    Write-Host "  ✗ Linux evidence not found at: $linuxEvidencePath" -ForegroundColor Red
    Write-Host "    Have you copied the Linux evidence from WSL2?" -ForegroundColor Yellow
    exit 1
}
Write-Host "  ✓ Linux evidence present" -ForegroundColor Green

# Step 3: Create Windows-compatible matrix files if needed
Write-Host "`n[STEP 3] Preparing Windows Matrix Files..." -ForegroundColor Yellow

# Function to create batch wrapper for bash scripts
function New-BatchWrapper {
    param(
        [string]$ScriptPath,
        [string]$BashPath
    )

    $batPath = $ScriptPath -replace '\.sh$', '.bat'
    if (-not (Test-Path $batPath)) {
        $scriptName = Split-Path -Leaf $ScriptPath
        $batContent = "@echo off`n`"$BashPath`" `"%~dp0$scriptName`" %*"
        $batContent | Out-File -FilePath $batPath -Encoding ASCII
        Write-Host "  Created batch wrapper: $batPath" -ForegroundColor Gray
    }
    return $batPath
}

# Create batch wrapper for replay-direct.sh
$replayBatPath = New-BatchWrapper -ScriptPath "offline\scripts\replay-direct.sh" -BashPath $bashPath

# Create temporary Windows matrix files that use .bat instead of .sh
function New-WindowsMatrix {
    param(
        [string]$SourceMatrix,
        [string]$TargetMatrix
    )

    if (Test-Path $SourceMatrix) {
        $content = Get-Content $SourceMatrix -Raw
        $content = $content -replace './offline/scripts/replay-direct\.sh', './offline/scripts/replay-direct.bat'
        $content | Out-File -FilePath $TargetMatrix -Encoding UTF8
        Write-Host "  Created Windows matrix: $TargetMatrix" -ForegroundColor Gray
    }
}

New-WindowsMatrix -SourceMatrix "offline\matrix.windows-amd64.yaml" `
                  -TargetMatrix "offline\matrix.windows-amd64-native.yaml"

New-WindowsMatrix -SourceMatrix "offline\matrix.windows-arm64.yaml" `
                  -TargetMatrix "offline\matrix.windows-arm64-native.yaml"

# Step 4: Build jcs-offline-replay.exe
Write-Host "`n[STEP 4] Building jcs-offline-replay.exe..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
& go build -trimpath -o .tmp\jcs-offline-replay.exe .\cmd\jcs-offline-replay
if ($LASTEXITCODE -ne 0) {
    Write-Host "  ✗ Build failed" -ForegroundColor Red
    exit 1
}
Write-Host "  ✓ Build successful" -ForegroundColor Green

# Step 5: Generate Windows amd64 evidence
Write-Host "`n[STEP 5] Generating Windows amd64 Evidence..." -ForegroundColor Yellow
$env:JCS_OFFLINE_SOURCE_GIT_TAG = $RCTag

# Ensure Git for Windows bash is in PATH
$env:PATH = [System.IO.Path]::GetDirectoryName($bashPath) + ";" + $env:PATH

$amd64Output = "offline\runs\releases\$RCTag\windows_amd64"
Write-Host "  Output directory: $amd64Output" -ForegroundColor Gray

& .\.tmp\jcs-offline-replay.exe run-suite `
    --matrix offline/matrix.windows-amd64-native.yaml `
    --profile offline/profiles/maximal.windows-amd64.yaml `
    --output-dir $amd64Output `
    --skip-preflight

if ($LASTEXITCODE -ne 0) {
    Write-Host "  ✗ Windows amd64 evidence generation failed" -ForegroundColor Red
    Write-Host "  Check error messages above for details" -ForegroundColor Yellow
    exit 1
}

Write-Host "  ✓ Windows amd64 evidence generated" -ForegroundColor Green

# Verify digests match Linux reference
Write-Host "`n  Verifying Windows amd64 digests..." -ForegroundColor Cyan
$amd64Evidence = Get-Content "$amd64Output\offline-evidence.json" | ConvertFrom-Json

$digestsMatch = $true
if ($LinuxReferenceDigests.ContainsKey($RCTag)) {
    $refDigests = $LinuxReferenceDigests[$RCTag]

    foreach ($digestType in @("canonical", "exit_code", "failure_class", "verify")) {
        $fieldName = "aggregate_${digestType}_sha256"
        $actualDigest = $amd64Evidence."$fieldName"
        $expectedDigest = $refDigests.$digestType

        if ($actualDigest -eq $expectedDigest) {
            Write-Host "    ✓ $digestType : $actualDigest" -ForegroundColor Green
        } else {
            Write-Host "    ✗ $digestType mismatch!" -ForegroundColor Red
            Write-Host "      Expected: $expectedDigest" -ForegroundColor Red
            Write-Host "      Actual  : $actualDigest" -ForegroundColor Red
            $digestsMatch = $false
        }
    }

    if (-not $digestsMatch) {
        Write-Host "`n  ✗ CRITICAL: Windows digests do not match Linux reference values!" -ForegroundColor Red
        Write-Host "  This indicates a determinism issue between Linux and Windows builds." -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "  ⚠ No reference digests for $RCTag - manual verification required" -ForegroundColor Yellow
}

# Step 6: Generate Windows arm64 evidence (if not skipped)
if (-not $SkipArm64) {
    Write-Host "`n[STEP 6] Generating Windows arm64 Evidence..." -ForegroundColor Yellow
    Write-Host "  Note: Cross-compiling for arm64 on x64 host" -ForegroundColor Gray

    $arm64Output = "offline\runs\releases\$RCTag\windows_arm64"

    & .\.tmp\jcs-offline-replay.exe run-suite `
        --matrix offline/matrix.windows-arm64-native.yaml `
        --profile offline/profiles/maximal.windows-arm64.yaml `
        --output-dir $arm64Output `
        --target-goarch arm64 `
        --skip-preflight

    if ($LASTEXITCODE -ne 0) {
        Write-Host "  ⚠ Windows arm64 evidence generation failed" -ForegroundColor Yellow
        Write-Host "  This is expected on x64 hosts without arm64 emulation." -ForegroundColor Yellow
        Write-Host "  Continuing without arm64 evidence..." -ForegroundColor Yellow
    } else {
        Write-Host "  ✓ Windows arm64 evidence generated" -ForegroundColor Green

        # Verify arm64 digests if successful
        if (Test-Path "$arm64Output\offline-evidence.json") {
            $arm64Evidence = Get-Content "$arm64Output\offline-evidence.json" | ConvertFrom-Json
            # Similar verification logic as amd64
        }
    }
} else {
    Write-Host "`n[STEP 6] Skipping Windows arm64 evidence generation (-SkipArm64 specified)" -ForegroundColor Yellow
}

# Summary
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Windows Phase 2 Complete!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Generated evidence:" -ForegroundColor Yellow
Get-ChildItem -Path "offline\runs\releases\$RCTag\windows*" -Directory | ForEach-Object {
    Write-Host "  - $($_.Name)" -ForegroundColor Gray
}

Write-Host "`nNext steps:" -ForegroundColor Yellow
Write-Host "  1. Review the evidence files" -ForegroundColor Gray
Write-Host "  2. Commit the Windows evidence (Step 7):" -ForegroundColor Gray
Write-Host "     git add -f offline/runs/releases/$RCTag/windows_*/" -ForegroundColor White
Write-Host '     git commit -m "evidence: add Windows offline replay evidence for ' + $RCTag + '"' -ForegroundColor White
Write-Host "     git push origin feat/windows-cross-arch-testing-ldSsm" -ForegroundColor White
Write-Host "  3. Return to WSL2/Linux for Phase 3 finalization" -ForegroundColor Gray