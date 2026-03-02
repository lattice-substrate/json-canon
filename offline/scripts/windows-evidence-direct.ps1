# Windows Native Evidence Generation
# Direct approach for Windows that bypasses Unix-based replay harness
# This generates deterministic evidence natively on Windows

param(
    [Parameter(Mandatory=$true)]
    [string]$RCTag
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Windows Native Evidence Generation" -ForegroundColor Cyan
Write-Host "RC Tag: $RCTag" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# Linux reference digests that Windows MUST match
$LinuxReferenceDigests = @{
    "v0.2.5-rc.1" = @{
        canonical = "2818166c21e1b445d59b061c5a546eccb54f71566325ea9366ddde30ddd5ebc6"
        exit_code = "73d91ef3f2fd6d709fd8491bb9c547290a1b3a13c234423ca96432f4258235d2"
        failure_class = "af58643f979138dadd16e4c78fd6d60d44d0818a5ce5269696ac3966f1d3306b"
        verify = "66d329b3bd829da527feb00eb97fdb681a0e15c28ac14d8bfed29ecae13e70f6"
    }
}

# Step 1: Build Windows binary
Write-Host "`n[STEP 1] Building Windows binary..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

$buildOutput = & go build -trimpath -buildvcs=false -o .tmp\jcs-canon.exe .\cmd\jcs-canon 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed: $buildOutput" -ForegroundColor Red
    exit 1
}
Write-Host "  OK: jcs-canon.exe built" -ForegroundColor Green

# Step 2: Extract and load test vectors from bundle
Write-Host "`n[STEP 2] Loading test vectors from bundle..." -ForegroundColor Yellow

# The bundle was already created during the Linux phase
$bundlePath = "offline\runs\releases\$RCTag\x86_64\offline-bundle.tgz"
if (-not (Test-Path $bundlePath)) {
    Write-Host "  ERROR: Bundle not found at $bundlePath" -ForegroundColor Red
    Write-Host "  Make sure Linux evidence was generated first" -ForegroundColor Red
    exit 1
}

# Extract bundle to temp directory
$tempDir = Join-Path $env:TEMP "json-canon-windows-test"
Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $tempDir | Out-Null

# Use tar to extract (available on Windows 10+)
& tar -xzf $bundlePath -C $tempDir 2>&1 | Out-Null

# Load test vectors from JSONL files
$vectorFiles = @(
    "$tempDir\bundle\vectors\core.jsonl",
    "$tempDir\bundle\vectors\offsets.jsonl",
    "$tempDir\bundle\vectors\reject.jsonl",
    "$tempDir\bundle\vectors\verify.jsonl"
)

$allVectors = @()
foreach ($file in $vectorFiles) {
    if (Test-Path $file) {
        $lines = Get-Content $file
        foreach ($line in $lines) {
            if ($line.Trim()) {
                $vector = $line | ConvertFrom-Json
                $allVectors += $vector
            }
        }
        Write-Host "  Loaded: $(Split-Path -Leaf $file)" -ForegroundColor Gray
    }
}
Write-Host "  OK: Loaded $($allVectors.Count) test vectors" -ForegroundColor Green

# Step 3: Run tests and collect results
Write-Host "`n[STEP 3] Running tests..." -ForegroundColor Yellow
$results = @{
    canonical = @()
    exit_code = @()
    failure_class = @()
    verify = @()
}

$totalTests = $allVectors.Count * 5  # 5 replays per vector
$currentTest = 0

foreach ($vector in $allVectors) {
    # Save vector input to temp file
    $tempInput = [System.IO.Path]::GetTempFileName()

    # Handle different vector formats
    $inputContent = if ($vector.PSObject.Properties['input']) {
        $vector.input
    } elseif ($vector.PSObject.Properties['json']) {
        $vector.json
    } else {
        $vector | ConvertTo-Json -Compress
    }

    # Write UTF8 without BOM
    [System.IO.File]::WriteAllText($tempInput, $inputContent, [System.Text.UTF8Encoding]::new($false))

    # Run 5 replays for determinism verification
    for ($replay = 1; $replay -le 5; $replay++) {
        $currentTest++
        Write-Progress -Activity "Running tests" -Status "Test $currentTest of $totalTests" -PercentComplete (($currentTest / $totalTests) * 100)

        # Test canonicalize output - use stdin redirection
        $canonicalOutput = $inputContent | & .\.tmp\jcs-canon.exe canonicalize - 2>&1
        $canonicalExitCode = $LASTEXITCODE

        if ($canonicalExitCode -eq 0) {
            # Only hash the actual output, not error messages
            $outputStr = ($canonicalOutput | Where-Object { $_ -is [string] }) -join ""
            $canonicalBytes = [System.Text.Encoding]::UTF8.GetBytes($outputStr)
            $sha256 = [System.Security.Cryptography.SHA256]::Create()
            $hash = $sha256.ComputeHash($canonicalBytes)
            $hashHex = [System.BitConverter]::ToString($hash).Replace("-", "").ToLower()
            $results.canonical += $hashHex
        }
        $results.exit_code += $canonicalExitCode

        # Test verify command
        $verifyOutput = $inputContent | & .\.tmp\jcs-canon.exe verify - 2>&1
        $verifyExitCode = $LASTEXITCODE
        $results.verify += $verifyExitCode

        # Map exit codes to failure classes
        $failureClass = switch ($canonicalExitCode) {
            0 { "success" }
            1 { "parse_error" }
            2 { "validation_error" }
            default { "unknown_error" }
        }
        $results.failure_class += $failureClass
    }

    Remove-Item $tempInput -Force -ErrorAction SilentlyContinue
}

Write-Progress -Activity "Running tests" -Completed
Write-Host "  OK: Completed $totalTests tests" -ForegroundColor Green

# Clean up temp directory
Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue

# Step 4: Calculate aggregate digests
Write-Host "`n[STEP 4] Calculating aggregate digests..." -ForegroundColor Yellow

function Get-AggregateDigest {
    param([array]$data)

    $sortedData = $data | Sort-Object
    $concatenated = $sortedData -join ""
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($concatenated)
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    $hash = $sha256.ComputeHash($bytes)
    return [System.BitConverter]::ToString($hash).Replace("-", "").ToLower()
}

$aggregateDigests = @{
    canonical = Get-AggregateDigest $results.canonical
    exit_code = Get-AggregateDigest $results.exit_code
    failure_class = Get-AggregateDigest $results.failure_class
    verify = Get-AggregateDigest $results.verify
}

# Step 5: Verify against Linux reference
Write-Host "`n[STEP 5] Verifying against Linux reference..." -ForegroundColor Yellow
$allMatch = $true

if ($LinuxReferenceDigests.ContainsKey($RCTag)) {
    $refDigests = $LinuxReferenceDigests[$RCTag]

    foreach ($digestType in @("canonical", "exit_code", "failure_class", "verify")) {
        $actual = $aggregateDigests[$digestType]
        $expected = $refDigests[$digestType]

        if ($actual -eq $expected) {
            Write-Host "  OK: $digestType matches" -ForegroundColor Green
            Write-Host "      $actual" -ForegroundColor Gray
        }
        else {
            Write-Host "  FAIL: $digestType mismatch!" -ForegroundColor Red
            Write-Host "      Expected: $expected" -ForegroundColor Red
            Write-Host "      Actual:   $actual" -ForegroundColor Red
            $allMatch = $false
        }
    }
}
else {
    Write-Host "  WARNING: No reference digests for $RCTag" -ForegroundColor Yellow
    $allMatch = $false
}

# Step 6: Generate evidence file
Write-Host "`n[STEP 6] Generating evidence file..." -ForegroundColor Yellow
$evidenceDir = "offline\runs\releases\$RCTag\windows_amd64"
New-Item -ItemType Directory -Path $evidenceDir -Force | Out-Null

$evidence = @{
    platform = "windows_amd64"
    source_git_commit = "12c5162085f90582c8e70690d40a2acba79028ce"
    source_git_tag = $RCTag
    timestamp = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
    aggregate_canonical_sha256 = $aggregateDigests.canonical
    aggregate_exit_code_sha256 = $aggregateDigests.exit_code
    aggregate_failure_class_sha256 = $aggregateDigests.failure_class
    aggregate_verify_sha256 = $aggregateDigests.verify
    total_tests = $totalTests
    test_vectors = $allVectors.Count
    replays_per_vector = 5
    matches_linux = $allMatch
}

$evidenceJson = $evidence | ConvertTo-Json -Depth 10
$evidenceJson | Out-File -FilePath "$evidenceDir\offline-evidence.json" -Encoding ASCII
Write-Host "  OK: Evidence written to $evidenceDir\offline-evidence.json" -ForegroundColor Green

# Step 7: Summary
Write-Host "`n========================================" -ForegroundColor Cyan
if ($allMatch) {
    Write-Host "SUCCESS: Windows evidence matches Linux!" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "`nWindows evidence has been generated and verified." -ForegroundColor Green
    Write-Host "All digests match the Linux reference values." -ForegroundColor Green

    Write-Host "`nNext steps:" -ForegroundColor Yellow
    Write-Host "  1. Commit the Windows evidence:" -ForegroundColor Gray
    Write-Host "     git add -f offline/runs/releases/$RCTag/windows_amd64/" -ForegroundColor White
    Write-Host "     git commit -m `"evidence: add Windows native evidence for $RCTag`"" -ForegroundColor White
    Write-Host "     git push origin feat/windows-cross-arch-testing-ldSsm" -ForegroundColor White
    Write-Host "  2. Return to WSL2/Linux for Phase 3 finalization" -ForegroundColor Gray
}
else {
    Write-Host "FAILURE: Windows evidence does NOT match Linux!" -ForegroundColor Red
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "`nWindows and Linux digests do not match." -ForegroundColor Red
    Write-Host "This indicates a cross-platform determinism issue." -ForegroundColor Red
    Write-Host "DO NOT PROCEED with the release until this is resolved." -ForegroundColor Red
    exit 1
}