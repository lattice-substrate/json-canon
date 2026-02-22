#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: cold-replay-preflight.sh [options]

Options:
  --matrix <path>       Matrix YAML path (default: offline/matrix.yaml)
  --controller <path>   jcs-offline-replay binary (auto-build if omitted)
  --report <path>       Write full report log to this file
  --strict              Fail on any warning (default)
  --no-strict           Do not fail on warnings
  -h, --help            Show this help
USAGE
}

MATRIX="offline/matrix.yaml"
CONTROLLER=""
REPORT=""
STRICT=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --matrix)
      MATRIX="$2"
      shift 2
      ;;
    --controller)
      CONTROLLER="$2"
      shift 2
      ;;
    --report)
      REPORT="$2"
      shift 2
      ;;
    --strict)
      STRICT=1
      shift
      ;;
    --no-strict)
      STRICT=0
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

TMPDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT

if [[ -n "$REPORT" ]]; then
  mkdir -p "$(dirname "$REPORT")"
  exec > >(tee "$REPORT") 2>&1
fi

echo "[preflight] repo: $ROOT"
echo "[preflight] matrix: $MATRIX"

if [[ -z "$CONTROLLER" ]]; then
  CONTROLLER="$TMPDIR/jcs-offline-replay"
  echo "[preflight] building controller: $CONTROLLER"
  CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$CONTROLLER" ./cmd/jcs-offline-replay
fi

if [[ ! -x "$CONTROLLER" ]]; then
  echo "[preflight] controller is not executable: $CONTROLLER" >&2
  exit 2
fi

MATRIX_JSON="$TMPDIR/matrix.json"
"$CONTROLLER" inspect-matrix --matrix "$MATRIX" > "$MATRIX_JSON"

python3 - "$MATRIX_JSON" "$TMPDIR" <<'PY'
import json,sys,os
matrix=json.load(open(sys.argv[1],encoding='utf-8'))
out=sys.argv[2]
containers=[]
vms=[]
for n in matrix.get('nodes',[]):
    mode=n.get('mode','')
    replay=n.get('runner',{}).get('replay',[])
    env=n.get('runner',{}).get('env',{}) or {}
    if mode=='container':
        image=replay[1] if len(replay)>1 else ''
        containers.append((n.get('id',''), image, str(n.get('replays',0))))
    elif mode=='vm':
        domain=replay[1] if len(replay)>1 else ''
        snapshot=replay[2] if len(replay)>2 else 'snapshot-cold'
        ssh_target=env.get('JCS_VM_SSH_TARGET', f"root@{domain}")
        ssh_opts=env.get('JCS_VM_SSH_OPTIONS','-') or '-'
        vms.append((n.get('id',''), domain, snapshot, ssh_target, ssh_opts, str(n.get('replays',0))))
with open(os.path.join(out,'containers.tsv'),'w',encoding='utf-8') as f:
    for row in containers:
        f.write('\t'.join(row)+'\n')
with open(os.path.join(out,'vms.tsv'),'w',encoding='utf-8') as f:
    for row in vms:
        f.write('\t'.join(row)+'\n')
print(f"architecture={matrix.get('architecture','')}")
print(f"nodes_total={len(matrix.get('nodes',[]))}")
print(f"container_nodes={len(containers)}")
print(f"vm_nodes={len(vms)}")
PY

FAIL=0
WARN=0

warn() {
  WARN=$((WARN+1))
  echo "[WARN] $*"
}

fail() {
  FAIL=$((FAIL+1))
  echo "[FAIL] $*"
}

pass() {
  echo "[PASS] $*"
}

echo "[preflight] checking base toolchain"
for c in go tar python3; do
  if command -v "$c" >/dev/null 2>&1; then
    pass "command available: $c"
  else
    fail "missing command: $c"
  fi
done

container_count="$(wc -l < "$TMPDIR/containers.tsv" | tr -d ' ')"
if [[ "$container_count" != "0" ]]; then
  engine="${JCS_CONTAINER_ENGINE:-}"
  if [[ -z "$engine" ]]; then
    if command -v podman >/dev/null 2>&1; then
      engine="podman"
    elif command -v docker >/dev/null 2>&1; then
      engine="docker"
    else
      engine=""
    fi
  fi

  if [[ -z "$engine" ]]; then
    fail "container lanes exist but no container engine found (podman/docker)"
  elif ! command -v "$engine" >/dev/null 2>&1; then
    fail "container engine not executable: $engine"
  else
    if info_out="$($engine info 2>&1)"; then
      pass "container engine reachable: $engine"
    else
      fail "container engine not reachable: $engine ($info_out)"
    fi

    echo "[preflight] checking offline container images"
    while IFS=$'\t' read -r node image replays; do
      [[ -z "$node" ]] && continue
      if [[ -z "$image" ]]; then
        fail "container node $node has empty image in matrix"
        continue
      fi
      if inspect_out="$($engine image inspect "$image" 2>&1)"; then
        pass "container image present: $node -> $image"
      else
        fail "container image missing/unreachable: $node -> $image ($inspect_out)"
      fi
    done < "$TMPDIR/containers.tsv"
  fi
else
  warn "no container nodes found in matrix"
fi

vm_count="$(wc -l < "$TMPDIR/vms.tsv" | tr -d ' ')"
if [[ "$vm_count" != "0" ]]; then
  echo "[preflight] checking vm/libvirt dependencies"
  for c in virsh ssh scp; do
    if command -v "$c" >/dev/null 2>&1; then
      pass "command available: $c"
    else
      fail "missing command: $c"
    fi
  done

  while IFS=$'\t' read -r node domain snapshot ssh_target ssh_opts replays; do
    [[ -z "$node" ]] && continue
    if [[ -z "$domain" ]]; then
      fail "vm node $node has empty domain in matrix"
      continue
    fi

    if virsh dominfo "$domain" >/dev/null 2>&1; then
      pass "libvirt domain exists: $node -> $domain"
    else
      fail "libvirt domain missing/unreachable: $node -> $domain"
      continue
    fi

    if [[ "$snapshot" != "-" ]]; then
      if virsh snapshot-list --name "$domain" | grep -Fx "$snapshot" >/dev/null 2>&1; then
        pass "snapshot exists: $domain/$snapshot"
      else
        fail "snapshot missing: $domain/$snapshot"
      fi
    fi

    opts=( -o BatchMode=yes -o ConnectTimeout=5 )
    if [[ "$ssh_opts" != "-" ]]; then
      # shellcheck disable=SC2206
      extra=( $ssh_opts )
      opts=( "${extra[@]}" "${opts[@]}" )
    fi

    if ssh_out="$(ssh -n "${opts[@]}" "$ssh_target" true 2>&1)"; then
      pass "vm ssh reachable: $node -> $ssh_target"
    else
      fail "vm ssh unreachable: $node -> $ssh_target ($ssh_out)"
    fi
  done < "$TMPDIR/vms.tsv"
else
  warn "no vm nodes found in matrix"
fi

echo "[preflight] failures=$FAIL warnings=$WARN"
if [[ "$FAIL" -ne 0 ]]; then
  echo "[preflight] RESULT=FAIL"
  exit 1
fi
if [[ "$STRICT" -eq 1 && "$WARN" -ne 0 ]]; then
  echo "[preflight] RESULT=FAIL (strict mode: warnings present)"
  exit 1
fi

echo "[preflight] RESULT=PASS"
