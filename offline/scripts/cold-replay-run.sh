#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: cold-replay-run.sh [options]

Options:
  --matrix <path>            Matrix config JSON (default: offline/matrix.yaml)
  --profile <path>           Profile config JSON (default: offline/profiles/maximal.yaml)
  --output-dir <path>        Output directory (default: offline/runs/<timestamp>)
  --timeout <duration>       Replay timeout for jcs-offline-replay run (default: 12h)
  --version <string>         Version string for jcs-canon build (default: v0.0.0-dev)
  --skip-preflight           Skip preflight checks
  --skip-release-gate        Skip offline conformance release gate test
  -h, --help                 Show help
USAGE
}

MATRIX="offline/matrix.yaml"
PROFILE="offline/profiles/maximal.yaml"
OUTDIR=""
TIMEOUT="12h"
VERSION="v0.0.0-dev"
SKIP_PREFLIGHT=0
SKIP_RELEASE_GATE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --matrix)
      MATRIX="$2"
      shift 2
      ;;
    --profile)
      PROFILE="$2"
      shift 2
      ;;
    --output-dir)
      OUTDIR="$2"
      shift 2
      ;;
    --timeout)
      TIMEOUT="$2"
      shift 2
      ;;
    --version)
      VERSION="$2"
      shift 2
      ;;
    --skip-preflight)
      SKIP_PREFLIGHT=1
      shift
      ;;
    --skip-release-gate)
      SKIP_RELEASE_GATE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ -z "$OUTDIR" ]]; then
  OUTDIR="offline/runs/$(date -u +%Y%m%dT%H%M%SZ)"
fi

mkdir -p "$OUTDIR/bin" "$OUTDIR/logs" "$OUTDIR/audit"
OUTDIR_ABS="$(cd "$OUTDIR" && pwd)"
MATRIX_ABS="$(realpath "$MATRIX")"
PROFILE_ABS="$(realpath "$PROFILE")"

CANON_BIN="$OUTDIR_ABS/bin/jcs-canon"
CTL_BIN="$OUTDIR_ABS/bin/jcs-offline-replay"
BUNDLE="$OUTDIR_ABS/offline-bundle.tgz"
EVIDENCE="$OUTDIR_ABS/offline-evidence.json"

echo "[run] repo: $ROOT"
echo "[run] matrix: $MATRIX"
echo "[run] profile: $PROFILE"
echo "[run] output: $OUTDIR_ABS"

echo "[run] build jcs-canon"
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=$VERSION" \
  -o "$CANON_BIN" ./cmd/jcs-canon 2>&1 | tee "$OUTDIR_ABS/logs/build-jcs-canon.log"

echo "[run] build jcs-offline-replay"
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags='-s -w -buildid=' \
  -o "$CTL_BIN" ./cmd/jcs-offline-replay 2>&1 | tee "$OUTDIR_ABS/logs/build-jcs-offline-replay.log"

if [[ "$SKIP_PREFLIGHT" -eq 0 ]]; then
  echo "[run] preflight checks"
  ./offline/scripts/cold-replay-preflight.sh \
    --matrix "$MATRIX" \
    --controller "$CTL_BIN" \
    --report "$OUTDIR_ABS/logs/preflight.log"
else
  echo "[run] preflight skipped"
fi

echo "[run] prepare bundle"
"$CTL_BIN" prepare \
  --matrix "$MATRIX" \
  --profile "$PROFILE" \
  --binary "$CANON_BIN" \
  --bundle "$BUNDLE" 2>&1 | tee "$OUTDIR_ABS/logs/prepare.log"

echo "[run] execute cold replay matrix"
"$CTL_BIN" run \
  --matrix "$MATRIX" \
  --profile "$PROFILE" \
  --bundle "$BUNDLE" \
  --evidence "$EVIDENCE" \
  --timeout "$TIMEOUT" 2>&1 | tee "$OUTDIR_ABS/logs/run.log"

echo "[run] verify evidence"
"$CTL_BIN" verify-evidence \
  --matrix "$MATRIX" \
  --profile "$PROFILE" \
  --evidence "$EVIDENCE" 2>&1 | tee "$OUTDIR_ABS/logs/verify-evidence.log"

echo "[run] controller report"
"$CTL_BIN" report --evidence "$EVIDENCE" 2>&1 | tee "$OUTDIR_ABS/logs/report.log"

echo "[run] audit summary"
./offline/scripts/cold-replay-audit-report.sh \
  --matrix "$MATRIX" \
  --profile "$PROFILE" \
  --evidence "$EVIDENCE" \
  --controller "$CTL_BIN" \
  --output-dir "$OUTDIR_ABS/audit" 2>&1 | tee "$OUTDIR_ABS/logs/audit.log"

if [[ "$SKIP_RELEASE_GATE" -eq 0 ]]; then
  echo "[run] release gate test"
  JCS_OFFLINE_EVIDENCE="$EVIDENCE" \
    JCS_OFFLINE_MATRIX="$MATRIX_ABS" \
    JCS_OFFLINE_PROFILE="$PROFILE_ABS" \
    go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1 -v \
    2>&1 | tee "$OUTDIR_ABS/logs/release-gate.log"
else
  echo "[run] release gate skipped by flag" | tee "$OUTDIR_ABS/logs/release-gate.log"
fi

sha256sum "$BUNDLE" > "$OUTDIR_ABS/audit/bundle.sha256"
sha256sum "$EVIDENCE" > "$OUTDIR_ABS/audit/evidence.sha256"

cat > "$OUTDIR_ABS/RUN_INDEX.txt" <<INDEX
offline_cold_replay_run_dir=$OUTDIR_ABS
matrix=$MATRIX
profile=$PROFILE
bundle=$BUNDLE
evidence=$EVIDENCE
controller=$CTL_BIN
canonicalizer=$CANON_BIN
audit_markdown=$OUTDIR_ABS/audit/audit-summary.md
audit_json=$OUTDIR_ABS/audit/audit-summary.json
INDEX

echo "[run] RESULT=PASS"
echo "[run] inspect: $OUTDIR_ABS/RUN_INDEX.txt"
