# v0.2.5-rc.1 Release Candidate — Live Workflow Progress

## Overview

- **RC Tag**: `v0.2.5-rc.1`
- **Branch**: `feat/windows-cross-arch-testing-ldSsm`
- **Commit A (evidence source)**: `12c5162085f90582c8e70690d40a2acba79028ce`
- **Runbook**: `docs/OFFLINE_REPLAY_HARNESS.md` section 10
- **Latest released tag**: `v0.2.4`

## Current Status

**Phase 1 (Linux) is COMPLETE.** Linux x86_64 + arm64 evidence has been
generated, verified, promoted to `offline/runs/releases/v0.2.5-rc.1/`, and
committed to the feature branch.

**Phase 2 (Windows) is PENDING.** Resume on a Windows machine by reading
this document and following the Phase 2 instructions below.

---

## Phase 1: Linux — COMPLETE

### Step 1: CHANGELOG update + commit A — DONE

- Bumped `[Unreleased]` → `[v0.2.5-rc.1] - 2026-03-01` in CHANGELOG.md
- Committed as `12c5162` ("release: prepare CHANGELOG for v0.2.5-rc.1")
- This is **commit A** — all evidence binds to this SHA

### Step 2: Cross-arch evidence (x86_64 + arm64) — DONE

- Output dir: `offline/runs/release-prep-v0.2.5-rc.1/` (gitignored, local only)
- `JCS_OFFLINE_SOURCE_GIT_TAG=v0.2.5-rc.1` pre-bound at generation time
- `--run-official-vectors` included, `--run-official-es6-100m` skipped (RC)

**x86_64**: 12 nodes × 5 replays = 60 runs, 74 vectors each — **PASS**
**arm64**: 12 nodes × 5 replays = 60 runs, 74 vectors each — **PASS**
**Cross-arch comparison**: **PASS**

### Step 3: Evidence promoted and committed — DONE

Linux evidence committed to feature branch at:
```
offline/runs/releases/v0.2.5-rc.1/
  x86_64/offline-evidence.json
  arm64/offline-evidence.json
  cross-arch-compare.json
  cross-arch-compare.md
```

### Aggregate Digests (identical across x86_64 and arm64)

These are the reference values. Windows evidence MUST produce identical digests.

| Digest | SHA-256 |
|--------|---------|
| canonical | `2818166c21e1b445d59b061c5a546eccb54f71566325ea9366ddde30ddd5ebc6` |
| exit_code | `73d91ef3f2fd6d709fd8491bb9c547290a1b3a13c234423ca96432f4258235d2` |
| failure_class | `af58643f979138dadd16e4c78fd6d60d44d0818a5ce5269696ac3966f1d3306b` |
| verify | `66d329b3bd829da527feb00eb97fdb681a0e15c28ac14d8bfed29ecae13e70f6` |

---

## Phase 2: Windows — Instructions for New Session

### What you are doing

You are generating native Windows offline replay evidence for `v0.2.5-rc.1`.
The Linux evidence is already committed to the feature branch. You need to:

1. Generate Windows amd64 evidence natively on this Windows machine
2. Generate Windows arm64 evidence (cross-compiled if no arm64 hardware)
3. Add the Windows evidence to `offline/runs/releases/v0.2.5-rc.1/`
4. Compare Windows aggregate digests against the Linux reference values above
5. Commit the Windows evidence
6. Return to Linux (or stay here) for Phase 3 finalization

### Prerequisites

1. **Go toolchain** — install Go (1.22+ required, matching go.mod)
2. **Git for Windows** — provides `bash` in PATH, needed to run
   `offline/scripts/replay-direct.sh` (the Windows matrix direct runner)
3. **Clone and checkout**:
   ```powershell
   git clone <repo-url> json-canon
   cd json-canon
   git checkout feat/windows-cross-arch-testing-ldSsm
   git pull origin feat/windows-cross-arch-testing-ldSsm
   ```
4. Verify commit A is in history:
   ```powershell
   git log --oneline | Select-String "12c5162"
   # Should show: 12c5162 release: prepare CHANGELOG for v0.2.5-rc.1
   ```
5. Verify Linux evidence is present:
   ```powershell
   Test-Path offline\runs\releases\v0.2.5-rc.1\x86_64\offline-evidence.json
   # Should return True
   ```

### Step 4: Build jcs-offline-replay on Windows

```powershell
$env:CGO_ENABLED = "0"
go build -trimpath -o .tmp\jcs-offline-replay.exe .\cmd\jcs-offline-replay
```

### Step 5: Generate Windows amd64 evidence (native)

```powershell
$env:JCS_OFFLINE_SOURCE_GIT_TAG = "v0.2.5-rc.1"
.\.tmp\jcs-offline-replay.exe run-suite `
  --matrix offline/matrix.windows-amd64.yaml `
  --profile offline/profiles/maximal.windows-amd64.yaml `
  --output-dir offline/runs/releases/v0.2.5-rc.1/windows_amd64 `
  --skip-preflight
```

**Why `--skip-preflight`**: The Windows matrix has no container/VM nodes.
Strict preflight treats their absence as warnings → failure. The only node
is a direct-execution lane which needs no preflight checks.

**Verify** after completion:
```powershell
Get-Content offline\runs\releases\v0.2.5-rc.1\windows_amd64\offline-evidence.json |
  ConvertFrom-Json |
  Select-Object source_git_commit, source_git_tag,
    aggregate_canonical_sha256, aggregate_exit_code_sha256,
    aggregate_failure_class_sha256, aggregate_verify_sha256
```

Check:
- `source_git_commit` = `12c5162085f90582c8e70690d40a2acba79028ce`
- `source_git_tag` = `v0.2.5-rc.1`
- `aggregate_canonical_sha256` = `2818166c21e1b445d59b061c5a546eccb54f71566325ea9366ddde30ddd5ebc6`
- All four digests match the Linux reference values above

### Step 6: Generate Windows arm64 evidence

If on an x64 Windows host with no arm64 hardware, cross-compile:

```powershell
$env:JCS_OFFLINE_SOURCE_GIT_TAG = "v0.2.5-rc.1"
.\.tmp\jcs-offline-replay.exe run-suite `
  --matrix offline/matrix.windows-arm64.yaml `
  --profile offline/profiles/maximal.windows-arm64.yaml `
  --output-dir offline/runs/releases/v0.2.5-rc.1/windows_arm64 `
  --target-goarch arm64 `
  --skip-preflight
```

**Known risk**: Cross-compiling for arm64 and executing via `replay-direct.sh`
on an x64 host will fail with "exec format error" — the same issue we hit on
Linux. If this fails:
- Option A: Skip arm64 evidence for this RC (document the gap)
- Option B: Run on actual arm64 Windows hardware
- Option C: Build arm64 binaries but validate them in CI only

Verify the same way as Step 5 if it succeeds.

### Step 7: Commit Windows evidence

```powershell
git add -f offline/runs/releases/v0.2.5-rc.1/windows_amd64/
# If arm64 succeeded:
git add -f offline/runs/releases/v0.2.5-rc.1/windows_arm64/
git commit -m "evidence: add Windows offline replay evidence for v0.2.5-rc.1"
git push origin feat/windows-cross-arch-testing-ldSsm
```

**Note**: `git add -f` is required because `offline/runs/*` is in `.gitignore`.
Release evidence is force-tracked (same pattern as v0.2.0–v0.2.4).

---

## Phase 3: Finalize — After Phase 2

This can be done on either the Linux or Windows machine, wherever all evidence
is available. Pull the latest branch state first.

### Step 8: Verify all evidence files

```bash
TAG=v0.2.5-rc.1
for arch in x86_64 arm64 windows_amd64 windows_arm64; do
  echo "=== ${arch} ==="
  jq '.source_git_commit, .source_git_tag' \
    offline/runs/releases/${TAG}/${arch}/offline-evidence.json
done
```

All must show `12c5162085f90582c8e70690d40a2acba79028ce` and `v0.2.5-rc.1`.

### Step 9: Create annotated tag

```bash
git tag -a v0.2.5-rc.1 -m "Release v0.2.5-rc.1"
```

### Step 10: Local release gate verification

```bash
TAG=v0.2.5-rc.1
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=${TAG}" \
  -o .tmp/release-control/jcs-canon ./cmd/jcs-canon

SOURCE_COMMIT=$(jq -r '.source_git_commit' \
  offline/runs/releases/${TAG}/x86_64/offline-evidence.json)

for arch in x86_64 arm64 windows_amd64 windows_arm64; do
  case ${arch} in
    x86_64)
      MATRIX=offline/matrix.yaml
      PROFILE=offline/profiles/maximal.yaml ;;
    arm64)
      MATRIX=offline/matrix.arm64.yaml
      PROFILE=offline/profiles/maximal.arm64.yaml ;;
    windows_amd64)
      MATRIX=offline/matrix.windows-amd64.yaml
      PROFILE=offline/profiles/maximal.windows-amd64.yaml ;;
    windows_arm64)
      MATRIX=offline/matrix.windows-arm64.yaml
      PROFILE=offline/profiles/maximal.windows-arm64.yaml ;;
  esac

  echo "=== release gate: ${arch} ==="
  JCS_OFFLINE_EVIDENCE=$(pwd)/offline/runs/releases/${TAG}/${arch}/offline-evidence.json \
  JCS_OFFLINE_CONTROL_BINARY=$(pwd)/.tmp/release-control/jcs-canon \
  JCS_OFFLINE_MATRIX=$(pwd)/${MATRIX} \
  JCS_OFFLINE_PROFILE=$(pwd)/${PROFILE} \
  JCS_OFFLINE_EXPECTED_GIT_COMMIT=${SOURCE_COMMIT} \
  JCS_OFFLINE_EXPECTED_GIT_TAG=${TAG} \
  go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1 -v
done
```

All four (or three, if arm64 was skipped) must **PASS**.

### Step 11: Push and monitor CI

```bash
git push origin feat/windows-cross-arch-testing-ldSsm
git push origin v0.2.5-rc.1
```

Monitor CI release workflow. Expected jobs:
1. `pre_release`: vet, tests, race, conformance, lint, fuzz, then
   `TestOfflineReplayEvidenceReleaseGate` for all architectures
2. `windows_pre_release`: native Windows vet, test, race, conformance
3. Build + checksum + attestation + publish (RC marked as prerelease)

---

## Key Reference

| Item | Value |
|------|-------|
| Commit A SHA | `12c5162085f90582c8e70690d40a2acba79028ce` |
| RC tag | `v0.2.5-rc.1` |
| Branch | `feat/windows-cross-arch-testing-ldSsm` |
| Release evidence dir | `offline/runs/releases/v0.2.5-rc.1/` |
| Runbook | `docs/OFFLINE_REPLAY_HARNESS.md` section 10 |
| .gitignore note | `offline/runs/*` is ignored; use `git add -f` for releases |

## Known Issues

1. **Cross-OS from Linux fails**: `replay-direct.sh` executes the worker
   binary directly. A Windows .exe cannot run on Linux without Wine.
2. **Windows arm64 on x64 host**: Same exec format error risk. May need
   actual arm64 hardware or CI-only validation.
3. **Preflight strict mode**: Windows matrices lack container/VM nodes.
   Always pass `--skip-preflight` for Windows runs.
