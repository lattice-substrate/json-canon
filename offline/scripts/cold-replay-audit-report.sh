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

SUMMARY_JSON="$TMPDIR/audit-summary.json"
SUMMARY_MD="$TMPDIR/audit-summary.md"

python3 - "$EVIDENCE" "$MATRIX" "$PROFILE" "$SUMMARY_JSON" "$SUMMARY_MD" <<'PY'
import json,sys,datetime

evidence_path,matrix_path,profile_path,json_out,md_out=sys.argv[1:]
with open(evidence_path,encoding='utf-8') as f:
    e=json.load(f)

runs=e.get('node_replays',[])
by_node={}
for r in runs:
    by_node.setdefault(r['node_id'],[]).append(r)

canon={r.get('canonical_sha256') for r in runs}
verify={r.get('verify_sha256') for r in runs}
classes={r.get('failure_class_sha256') for r in runs}
exits={r.get('exit_code_sha256') for r in runs}

summary={
    'generated_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'matrix_path': matrix_path,
    'profile_path': profile_path,
    'evidence_path': evidence_path,
    'schema_version': e.get('schema_version',''),
    'profile_name': e.get('profile_name',''),
    'architecture': e.get('architecture',''),
    'hard_release_gate': bool(e.get('hard_release_gate',False)),
    'node_count': len(by_node),
    'run_count': len(runs),
    'required_suites': e.get('required_suites',[]),
    'aggregate': {
        'canonical': e.get('aggregate_canonical_sha256',''),
        'verify': e.get('aggregate_verify_sha256',''),
        'failure_class': e.get('aggregate_failure_class_sha256',''),
        'exit_code': e.get('aggregate_exit_code_sha256',''),
    },
    'digest_sets': {
        'canonical_unique': sorted(x for x in canon if x),
        'verify_unique': sorted(x for x in verify if x),
        'failure_class_unique': sorted(x for x in classes if x),
        'exit_code_unique': sorted(x for x in exits if x),
    },
    'node_replay_counts': {k: len(v) for k,v in sorted(by_node.items())},
    'node_replay_case_counts': {
        k: [int(x.get('case_count',0)) for x in sorted(v,key=lambda i:int(i.get('replay_index',0)))]
        for k,v in sorted(by_node.items())
    },
    'parity': {
        'canonical_single_digest': len(canon)==1,
        'verify_single_digest': len(verify)==1,
        'failure_class_single_digest': len(classes)==1,
        'exit_code_single_digest': len(exits)==1,
    },
}
summary['result']='PASS' if all(summary['parity'].values()) else 'FAIL'

with open(json_out,'w',encoding='utf-8') as f:
    json.dump(summary,f,indent=2)
    f.write('\n')

lines=[]
lines.append('# Offline Replay Audit Summary')
lines.append('')
lines.append(f"- Result: **{summary['result']}**")
lines.append(f"- Evidence: `{evidence_path}`")
lines.append(f"- Matrix: `{matrix_path}`")
lines.append(f"- Profile: `{profile_path}`")
lines.append(f"- Schema: `{summary['schema_version']}`")
lines.append(f"- Profile Name: `{summary['profile_name']}`")
lines.append(f"- Architecture: `{summary['architecture']}`")
lines.append(f"- Hard Release Gate: `{summary['hard_release_gate']}`")
lines.append(f"- Node Count: `{summary['node_count']}`")
lines.append(f"- Replay Rows: `{summary['run_count']}`")
lines.append('')
lines.append('## Aggregate Digests')
lines.append('')
for k,v in summary['aggregate'].items():
    lines.append(f"- {k}: `{v}`")
lines.append('')
lines.append('## Parity Checks')
lines.append('')
for k,v in summary['parity'].items():
    lines.append(f"- {k}: `{v}`")
lines.append('')
lines.append('## Node Replay Counts')
lines.append('')
for k,v in summary['node_replay_counts'].items():
    lines.append(f"- {k}: `{v}`")
lines.append('')
lines.append('## Node Case Counts By Replay Index')
lines.append('')
for k,v in summary['node_replay_case_counts'].items():
    lines.append(f"- {k}: `{v}`")
lines.append('')
with open(md_out,'w',encoding='utf-8') as f:
    f.write('\n'.join(lines)+'\n')
PY

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
