# v0.3.0-rc.1 Release Candidate — Live Workflow Progress

## Overview

- **RC Tag**: `v0.3.0-rc.1`
- **Branch**: `feat/windows-cross-arch-testing-ldSsm`
- **Commit A (evidence source)**: `e58e775a508931d64fd95ec80b1ebbd4115054e4`
- **Runbook**: `docs/OFFLINE_REPLAY_HARNESS.md` section 10
- **Latest released tag**: `v0.2.4`

## Current Status: Phase 1 Complete, Phase 2 Pending

Linux cross-arch evidence is fully generated and verified. Windows evidence
must be generated natively on a Windows machine before we can finalize.

---

## Phase 1: Linux — COMPLETE

### Step 1: CHANGELOG update + commit A — DONE

- Bumped `[Unreleased]` → `[v0.3.0-rc.1] - 2026-03-01` in CHANGELOG.md
- Committed as `e58e775` ("release: prepare CHANGELOG for v0.3.0-rc.1")
- This is **commit A** — all evidence binds to this SHA

### Step 2: Cross-arch evidence (x86_64 + arm64) — DONE

- Output dir: `offline/runs/release-prep-20260302T041152Z/`
- `JCS_OFFLINE_SOURCE_GIT_TAG=v0.3.0-rc.1` pre-bound at generation time
- `--run-official-vectors` included, `--run-official-es6-100m` skipped (RC)

**x86_64 results:**
- Profile: `maximal-offline-linux-x86_64`
- 12 nodes (6 container + 6 VM) × 5 replays = 60 runs, 74 vectors each
- Generated: `2026-03-02T04:11:54Z`
- Release gate test: **PASS**
- Audit: **PASS**

**arm64 results:**
- Profile: `maximal-offline-linux-arm64`
- 12 nodes (6 container + 6 VM) × 5 replays = 60 runs, 74 vectors each
- Generated: `2026-03-02T04:16:59Z`
- Release gate test: **PASS**
- Audit: **PASS**

**Cross-arch comparison: PASS** — all four aggregate digests match:

| Digest | SHA-256 |
|--------|---------|
| canonical | `2818166c21e1b445d59b061c5a546eccb54f71566325ea9366ddde30ddd5ebc6` |
| exit_code | `73d91ef3f2fd6d709fd8491bb9c547290a1b3a13c234423ca96432f4258235d2` |
| failure_class | `af58643f979138dadd16e4c78fd6d60d44d0818a5ce5269696ac3966f1d3306b` |
| verify | `66d329b3bd829da527feb00eb97fdb681a0e15c28ac14d8bfed29ecae13e70f6` |

### Step 3: Cross-OS from Linux — SKIPPED (by design)

`replay-direct.sh` executes the worker binary directly. A Windows .exe cannot
execute on Linux without Wine (which is not installed). Windows evidence must
be gathered natively.

### Linux evidence artifacts (NOT YET promoted or committed)

```
offline/runs/release-prep-20260302T041152Z/
  x86_64/offline-evidence.json       ← PASS, complete
  arm64/offline-evidence.json        ← PASS, complete
  cross-arch-compare.json            ← PASS
  cross-arch-compare.md              ← PASS
  x86_64/audit/                      ← audit-summary.json, .md, checksums
  arm64/audit/                       ← audit-summary.json, .md, checksums
  x86_64/bin/                        ← jcs-canon, jcs-offline-worker, jcs-offline-replay
  arm64/bin/                         ← jcs-canon, jcs-offline-worker, jcs-offline-replay
```

---

## Phase 2: Windows — PENDING (new session on Windows machine)

### Context for Claude in the new session

You are continuing the v0.3.0-rc.1 RC release process. Phase 1 (Linux cross-arch
evidence) is complete. You need to generate Windows evidence natively, then
finalize the release.

Read this document first, then follow the steps below exactly.

### Prerequisites on Windows

1. **Go toolchain** — install Go (1.22+ required; match the project's go.mod).
2. **Git for Windows** — provides `bash` in PATH, needed to run
   `offline/scripts/replay-direct.sh`.
3. **Clone and checkout** the branch:
   ```powershell
   git clone <repo-url> json-canon
   cd json-canon
   git checkout feat/windows-cross-arch-testing-ldSsm
   ```
4. Verify HEAD is commit A:
   ```powershell
   git rev-parse HEAD
   # Must be: e58e775a508931d64fd95ec80b1ebbd4115054e4
   ```

### Step 4: Build jcs-offline-replay on Windows

```powershell
$env:CGO_ENABLED = "0"
go build -trimpath -o .tmp\jcs-offline-replay.exe .\cmd\jcs-offline-replay
```

### Step 5: Generate Windows amd64 evidence (native execution)

```powershell
$env:JCS_OFFLINE_SOURCE_GIT_TAG = "v0.3.0-rc.1"
.\.tmp\jcs-offline-replay.exe run-suite `
  --matrix offline/matrix.windows-amd64.yaml `
  --profile offline/profiles/maximal.windows-amd64.yaml `
  --output-dir offline/runs/windows-evidence/windows_amd64 `
  --skip-preflight
```

**Important notes:**
- `--skip-preflight` is required because the Windows matrix has no container/VM
  nodes, and strict preflight treats their absence as warnings → failure.
- The matrix uses `replay-direct.sh` via the direct runner. Git Bash must be
  in PATH for this to work.
- The evidence must record `source_git_commit: e58e775a...` and
  `source_git_tag: v0.3.0-rc.1`.

### Step 6: Generate Windows arm64 evidence (cross-compiled from x64)

If running on an x64 Windows host (no arm64 hardware):

```powershell
$env:JCS_OFFLINE_SOURCE_GIT_TAG = "v0.3.0-rc.1"
.\.tmp\jcs-offline-replay.exe run-suite `
  --matrix offline/matrix.windows-arm64.yaml `
  --profile offline/profiles/maximal.windows-arm64.yaml `
  --output-dir offline/runs/windows-evidence/windows_arm64 `
  --target-goarch arm64 `
  --skip-preflight
```

**Note:** The `--target-goarch arm64` flag cross-compiles the jcs-canon and
worker binaries for arm64, then runs them via the direct runner. This may fail
with the same exec format error as Linux. If so, this step may need to be run
on actual arm64 Windows hardware, or the approach may need revision. Document
what happens.

### Step 7: Verify Windows evidence

```powershell
# Check source identity in each evidence file:
foreach ($arch in @("windows_amd64", "windows_arm64")) {
  Write-Host "=== $arch ==="
  Get-Content "offline\runs\windows-evidence\$arch\offline-evidence.json" |
    ConvertFrom-Json |
    Select-Object source_git_commit, source_git_tag,
      aggregate_canonical_sha256, aggregate_exit_code_sha256,
      aggregate_failure_class_sha256, aggregate_verify_sha256
}
```

**Required checks:**
- `source_git_commit` = `e58e775a508931d64fd95ec80b1ebbd4115054e4`
- `source_git_tag` = `v0.3.0-rc.1`
- All four aggregate digests should match the Linux values listed above

### Step 8: Transfer evidence to release directory

Still on Windows (or after copying files back to Linux):

```bash
TAG=v0.3.0-rc.1
PREP_LINUX=offline/runs/release-prep-20260302T041152Z
PREP_WINDOWS=offline/runs/windows-evidence

mkdir -p offline/runs/releases/${TAG}
cp -r ${PREP_LINUX}/x86_64     offline/runs/releases/${TAG}/x86_64
cp -r ${PREP_LINUX}/arm64      offline/runs/releases/${TAG}/arm64
cp -r ${PREP_WINDOWS}/windows_amd64  offline/runs/releases/${TAG}/windows_amd64
cp -r ${PREP_WINDOWS}/windows_arm64  offline/runs/releases/${TAG}/windows_arm64
```

If doing this on Windows, you could also `git add -f` and commit the Windows
evidence, then push. Then pull on Linux to finish.

---

## Phase 3: Finalize — PENDING (after Phase 2)

### Step 9: Verify all four evidence files

```bash
TAG=v0.3.0-rc.1
for arch in x86_64 arm64 windows_amd64 windows_arm64; do
  echo "=== ${arch} ==="
  jq '.source_git_commit, .source_git_tag' \
    offline/runs/releases/${TAG}/${arch}/offline-evidence.json
done
```

All must show commit A and `v0.3.0-rc.1`.

### Step 10: Commit evidence (commit B)

```bash
git add -f offline/runs/releases/v0.3.0-rc.1/
git commit -m "evidence: offline replay evidence for v0.3.0-rc.1"
```

**Note:** `git add -f` is required because `offline/runs/*` is in `.gitignore`.
The `releases/` subdirectory is force-tracked for release evidence (same as
v0.2.0–v0.2.4).

### Step 11: Create annotated tag on commit B

```bash
git tag -a v0.3.0-rc.1 -m "Release v0.3.0-rc.1"
```

### Step 12: Local release gate verification

```bash
TAG=v0.3.0-rc.1
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

All four must **PASS**.

### Step 13: Push and monitor CI

```bash
git push origin feat/windows-cross-arch-testing-ldSsm
git push origin v0.3.0-rc.1
```

Monitor CI release workflow on GitHub. The workflow:
1. `pre_release` job: vet, tests, race, conformance, lint, fuzz, then
   runs `TestOfflineReplayEvidenceReleaseGate` for all four architectures.
2. `windows_pre_release` job: native Windows vet, test, race, conformance.
3. Build + checksum + attestation + publish (RC marked as prerelease).

---

## Key Reference

| Item | Value |
|------|-------|
| Commit A SHA | `e58e775a508931d64fd95ec80b1ebbd4115054e4` |
| RC tag | `v0.3.0-rc.1` |
| Branch | `feat/windows-cross-arch-testing-ldSsm` |
| Linux evidence dir | `offline/runs/release-prep-20260302T041152Z/` |
| Release evidence dir | `offline/runs/releases/v0.3.0-rc.1/` |
| Runbook | `docs/OFFLINE_REPLAY_HARNESS.md` section 10 |
| .gitignore note | `offline/runs/*` is ignored; use `git add -f` for releases |

## Known Issues

1. **Cross-OS from Linux fails**: `replay-direct.sh` tries to execute Windows
   .exe on Linux, which fails with "Exec format error". Wine is not installed.
   This is why we generate Windows evidence natively instead.
2. **Windows arm64 on x64 host**: Cross-compiling the binary for arm64 and
   running it via `replay-direct.sh` on an x64 Windows host will likely also
   fail with exec format error. May need actual arm64 hardware or a different
   approach (e.g., skip arm64 evidence for RC, or use Windows arm64 VM).
3. **Preflight strict mode**: Windows matrices have no container/VM nodes.
   Strict preflight treats these warnings as failures. Always pass
   `--skip-preflight` for Windows runs.
