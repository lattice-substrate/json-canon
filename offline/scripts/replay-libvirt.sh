#!/usr/bin/env bash
set -euo pipefail

domain="${1:-}"
snapshot="${2:-snapshot-cold}"
if [[ -z "$domain" ]]; then
  echo "usage: replay-libvirt.sh <domain> [snapshot]" >&2
  exit 2
fi

: "${JCS_BUNDLE_PATH:?JCS_BUNDLE_PATH is required}"
: "${JCS_EVIDENCE_PATH:?JCS_EVIDENCE_PATH is required}"
: "${JCS_REPLAY_INDEX:?JCS_REPLAY_INDEX is required}"
: "${JCS_NODE_ID:?JCS_NODE_ID is required}"
: "${JCS_NODE_MODE:?JCS_NODE_MODE is required}"
: "${JCS_NODE_DISTRO:?JCS_NODE_DISTRO is required}"
: "${JCS_NODE_KERNEL_FAMILY:?JCS_NODE_KERNEL_FAMILY is required}"

for cmd in virsh ssh scp tar; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 2
  fi
done

if [[ ! -f "$JCS_BUNDLE_PATH" ]]; then
  echo "bundle does not exist: $JCS_BUNDLE_PATH" >&2
  exit 2
fi

ssh_target="${JCS_VM_SSH_TARGET:-root@$domain}"
ssh_opts=()
if [[ -n "${JCS_VM_SSH_OPTIONS:-}" ]]; then
  # shellcheck disable=SC2206
  ssh_opts=(${JCS_VM_SSH_OPTIONS})
fi

if virsh domstate "$domain" >/dev/null 2>&1; then
  if [[ "$snapshot" != "-" ]]; then
    virsh destroy "$domain" >/dev/null 2>&1 || true
    virsh snapshot-revert "$domain" "$snapshot" --force
  fi
  virsh start "$domain" >/dev/null 2>&1 || true
fi

for _ in $(seq 1 90); do
  if ssh "${ssh_opts[@]}" "$ssh_target" "true" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
if ! ssh "${ssh_opts[@]}" "$ssh_target" "true" >/dev/null 2>&1; then
  echo "vm ssh not reachable: $ssh_target" >&2
  exit 2
fi

evidence_dir="$(dirname "$JCS_EVIDENCE_PATH")"
mkdir -p "$evidence_dir"

host_tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$host_tmp"
}
trap cleanup EXIT

if ! tar -xzf "$JCS_BUNDLE_PATH" -C "$host_tmp" bundle/jcs-offline-worker >/dev/null 2>&1; then
  echo "failed to extract worker from bundle" >&2
  exit 2
fi
worker_host="$host_tmp/bundle/jcs-offline-worker"
chmod +x "$worker_host"

remote_tmp="/tmp/jcs-offline-${JCS_NODE_ID}-${JCS_REPLAY_INDEX}-$$"
ssh "${ssh_opts[@]}" "$ssh_target" "mkdir -p '$remote_tmp'"

scp "${ssh_opts[@]}" "$JCS_BUNDLE_PATH" "$ssh_target:$remote_tmp/bundle.tgz" >/dev/null
scp "${ssh_opts[@]}" "$worker_host" "$ssh_target:$remote_tmp/jcs-offline-worker" >/dev/null

ssh "${ssh_opts[@]}" "$ssh_target" \
  "chmod +x '$remote_tmp/jcs-offline-worker' && \
   LC_ALL=C LANG=C TZ=UTC '$remote_tmp/jcs-offline-worker' \
     --bundle '$remote_tmp/bundle.tgz' \
     --evidence '$remote_tmp/evidence.json' \
     --node-id '$JCS_NODE_ID' \
     --mode '$JCS_NODE_MODE' \
     --distro '$JCS_NODE_DISTRO' \
     --kernel-family '$JCS_NODE_KERNEL_FAMILY' \
     --replay-index '$JCS_REPLAY_INDEX'"

scp "${ssh_opts[@]}" "$ssh_target:$remote_tmp/evidence.json" "$JCS_EVIDENCE_PATH" >/dev/null
ssh "${ssh_opts[@]}" "$ssh_target" "rm -rf '$remote_tmp'" >/dev/null 2>&1 || true
