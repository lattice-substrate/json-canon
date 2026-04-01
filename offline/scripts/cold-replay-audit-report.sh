#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: cold-replay-audit-report.sh [options]

Options:
  --matrix <path>        Matrix config JSON (required)
  --profile <path>       Profile config JSON (required)
  --evidence <path>      Evidence JSON (required)
  --controller <path>    jcs-offline-replay binary (auto-build if omitted)
  --output-dir <path>    Output directory for markdown/json summaries
  -h, --help             Show help
USAGE
}

MATRIX=""
PROFILE=""
EVIDENCE=""
CONTROLLER=""
OUTDIR=""

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
    --evidence)
      EVIDENCE="$2"
      shift 2
      ;;
    --controller)
      CONTROLLER="$2"
      shift 2
      ;;
    --output-dir)
      OUTDIR="$2"
      shift 2
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

if [[ -z "$MATRIX" || -z "$PROFILE" || -z "$EVIDENCE" ]]; then
  usage >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

TMPDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT

if [[ -z "$CONTROLLER" ]]; then
  CONTROLLER="$TMPDIR/jcs-offline-replay"
  CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$CONTROLLER" ./cmd/jcs-offline-replay
fi

if [[ ! -x "$CONTROLLER" ]]; then
  echo "controller not executable: $CONTROLLER" >&2
  exit 2
fi

"$CONTROLLER" verify-evidence --matrix "$MATRIX" --profile "$PROFILE" --evidence "$EVIDENCE" >/dev/null

REPORT_TXT="$TMPDIR/controller-report.txt"
"$CONTROLLER" report --evidence "$EVIDENCE" > "$REPORT_TXT"

AUDIT_TMP="$TMPDIR/audit"
"$CONTROLLER" audit-summary --matrix "$MATRIX" --profile "$PROFILE" --evidence "$EVIDENCE" --output-dir "$AUDIT_TMP" >/dev/null

SUMMARY_JSON="$AUDIT_TMP/audit-summary.json"
SUMMARY_MD="$AUDIT_TMP/audit-summary.md"

cat "$SUMMARY_MD"

if [[ -n "$OUTDIR" ]]; then
  mkdir -p "$OUTDIR"
  cp "$SUMMARY_JSON" "$OUTDIR/audit-summary.json"
  cp "$SUMMARY_MD" "$OUTDIR/audit-summary.md"
  cp "$REPORT_TXT" "$OUTDIR/controller-report.txt"
  echo "[audit] wrote: $OUTDIR/audit-summary.json"
  echo "[audit] wrote: $OUTDIR/audit-summary.md"
  echo "[audit] wrote: $OUTDIR/controller-report.txt"
fi
