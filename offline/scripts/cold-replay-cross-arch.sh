#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: cold-replay-cross-arch.sh [options]

Options:
  --x86-matrix <path>       x86_64 matrix (default: offline/matrix.yaml)
  --x86-profile <path>      x86_64 profile (default: offline/profiles/maximal.yaml)
  --arm64-matrix <path>     arm64 matrix (default: offline/matrix.arm64.yaml)
  --arm64-profile <path>    arm64 profile (default: offline/profiles/maximal.arm64.yaml)
  --local-no-rocky          Use local no-rocky matrices for both architectures
  --output-dir <path>       Output directory (default: offline/runs/cross-arch-<timestamp>)
  --timeout <duration>      Timeout for each run (default: 12h)
  --skip-preflight          Skip per-architecture preflight
  -h, --help                Show help
USAGE
}

X86_MATRIX="offline/matrix.yaml"
X86_PROFILE="offline/profiles/maximal.yaml"
ARM_MATRIX="offline/matrix.arm64.yaml"
ARM_PROFILE="offline/profiles/maximal.arm64.yaml"
OUTDIR=""
TIMEOUT="12h"
SKIP_PREFLIGHT=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --x86-matrix)
      X86_MATRIX="$2"
      shift 2
      ;;
    --x86-profile)
      X86_PROFILE="$2"
      shift 2
      ;;
    --arm64-matrix)
      ARM_MATRIX="$2"
      shift 2
      ;;
    --arm64-profile)
      ARM_PROFILE="$2"
      shift 2
      ;;
    --local-no-rocky)
      X86_MATRIX="offline/matrix.local-no-rocky.yaml"
      ARM_MATRIX="offline/matrix.local-no-rocky.arm64.yaml"
      shift
      ;;
    --output-dir)
      OUTDIR="$2"
      shift 2
      ;;
    --timeout)
      TIMEOUT="$2"
      shift 2
      ;;
    --skip-preflight)
      SKIP_PREFLIGHT=1
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
  OUTDIR="offline/runs/cross-arch-$(date -u +%Y%m%dT%H%M%SZ)"
fi
mkdir -p "$OUTDIR"
OUTDIR_ABS="$(cd "$OUTDIR" && pwd)"

x86_args=()
if [[ "$SKIP_PREFLIGHT" -eq 1 ]]; then
  x86_args+=(--skip-preflight)
fi

arm_args=(--skip-release-gate)
if [[ "$SKIP_PREFLIGHT" -eq 1 ]]; then
  arm_args+=(--skip-preflight)
fi

echo "[cross-arch] running x86_64 harness"
./offline/scripts/cold-replay-run.sh \
  --matrix "$X86_MATRIX" \
  --profile "$X86_PROFILE" \
  --output-dir "$OUTDIR_ABS/x86_64" \
  --timeout "$TIMEOUT" \
  "${x86_args[@]}"

echo "[cross-arch] running arm64 harness"
./offline/scripts/cold-replay-run.sh \
  --matrix "$ARM_MATRIX" \
  --profile "$ARM_PROFILE" \
  --output-dir "$OUTDIR_ABS/arm64" \
  --timeout "$TIMEOUT" \
  "${arm_args[@]}"

X86_EVIDENCE="$OUTDIR_ABS/x86_64/offline-evidence.json"
ARM_EVIDENCE="$OUTDIR_ABS/arm64/offline-evidence.json"
COMPARE_JSON="$OUTDIR_ABS/cross-arch-compare.json"
COMPARE_MD="$OUTDIR_ABS/cross-arch-compare.md"

python3 - "$X86_EVIDENCE" "$ARM_EVIDENCE" "$COMPARE_JSON" "$COMPARE_MD" <<'PY'
import json,sys,datetime
x86_path,arm_path,json_out,md_out=sys.argv[1:]
with open(x86_path,encoding='utf-8') as f:
    x86=json.load(f)
with open(arm_path,encoding='utf-8') as f:
    arm=json.load(f)

fields=[
    ('aggregate_canonical_sha256','canonical'),
    ('aggregate_verify_sha256','verify'),
    ('aggregate_failure_class_sha256','failure_class'),
    ('aggregate_exit_code_sha256','exit_code'),
]
checks=[]
all_ok=True
for key,label in fields:
    xv=x86.get(key,'')
    av=arm.get(key,'')
    ok=(xv==av)
    all_ok=all_ok and ok
    checks.append({'field':key,'label':label,'x86':xv,'arm64':av,'match':ok})

out={
    'generated_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'x86_evidence': x86_path,
    'arm64_evidence': arm_path,
    'result': 'PASS' if all_ok else 'FAIL',
    'checks': checks,
}
with open(json_out,'w',encoding='utf-8') as f:
    json.dump(out,f,indent=2)
    f.write('\n')

lines=[]
lines.append('# Cross-Arch Replay Comparison')
lines.append('')
lines.append(f"- Result: **{out['result']}**")
lines.append(f"- x86 evidence: `{x86_path}`")
lines.append(f"- arm64 evidence: `{arm_path}`")
lines.append('')
lines.append('| Field | x86_64 | arm64 | Match |')
lines.append('|---|---|---|---|')
for c in checks:
    lines.append(f"| {c['field']} | `{c['x86']}` | `{c['arm64']}` | `{c['match']}` |")
lines.append('')
with open(md_out,'w',encoding='utf-8') as f:
    f.write('\n'.join(lines)+'\n')

if not all_ok:
    sys.exit(1)
PY

echo "[cross-arch] compare report: $COMPARE_MD"
echo "[cross-arch] compare json: $COMPARE_JSON"
echo "[cross-arch] RESULT=PASS"
