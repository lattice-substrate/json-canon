#!/usr/bin/env bash
set -euo pipefail

for cmd in ssh scp tar; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "error: missing required command: $cmd" >&2; exit 2; }
done

: "${JCS_VM_SSH_TARGET:-}"
: "${JCS_BUNDLE_PATH:?JCS_BUNDLE_PATH is required}"
: "${JCS_EVIDENCE_PATH:?JCS_EVIDENCE_PATH is required}"
: "${JCS_REPLAY_INDEX:?JCS_REPLAY_INDEX is required}"
: "${JCS_NODE_ID:?JCS_NODE_ID is required}"
: "${JCS_NODE_MODE:?JCS_NODE_MODE is required}"
: "${JCS_NODE_DISTRO:?JCS_NODE_DISTRO is required}"
: "${JCS_NODE_KERNEL_FAMILY:?JCS_NODE_KERNEL_FAMILY is required}"

ssh_target="${JCS_VM_SSH_TARGET:-}"
if [[ -z "$ssh_target" && -n "${JCS_VM_SSH_TARGET_ENV:-}" ]]; then
  ssh_target="${!JCS_VM_SSH_TARGET_ENV:-}"
fi
if [[ -z "$ssh_target" ]]; then
  echo "error: ssh target is empty" >&2
  exit 2
fi

ssh_opts_raw="${JCS_VM_SSH_OPTIONS:-}"
if [[ -z "$ssh_opts_raw" && -n "${JCS_VM_SSH_OPTIONS_ENV:-}" ]]; then
  ssh_opts_raw="${!JCS_VM_SSH_OPTIONS_ENV:-}"
fi
ssh_opts=()
if [[ -n "${ssh_opts_raw}" ]]; then
  # shellcheck disable=SC2206
  ssh_opts=(${ssh_opts_raw})
fi

image="${1:-}"
if [[ -z "$image" && -n "${JCS_CONTAINER_IMAGE_ENV:-}" ]]; then
  image="${!JCS_CONTAINER_IMAGE_ENV:-}"
fi
if [[ -z "$image" && -n "${JCS_CONTAINER_IMAGE:-}" ]]; then
  image="${JCS_CONTAINER_IMAGE}"
fi
if [[ -z "$image" ]]; then
  echo "error: container image is required" >&2
  exit 2
fi

if [[ ! -f "$JCS_BUNDLE_PATH" ]]; then
  echo "error: bundle does not exist: $JCS_BUNDLE_PATH" >&2
  exit 2
fi

connected=0
for _ in $(seq 1 90); do
  if ssh "${ssh_opts[@]}" "$ssh_target" "true" >/dev/null 2>&1; then
    connected=1
    break
  fi
  sleep 2
done
if [[ "$connected" -eq 0 ]]; then
  echo "error: ssh unreachable after 3 minutes: $ssh_target" >&2
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
  echo "error: failed to extract worker from bundle" >&2
  exit 2
fi
worker_host="$host_tmp/bundle/jcs-offline-worker"
chmod +x "$worker_host"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
container_script="${script_dir}/replay-container.sh"

remote_tmp="/tmp/jcs-offline-${JCS_NODE_ID}-${JCS_REPLAY_INDEX}-$$"
ssh "${ssh_opts[@]}" "$ssh_target" "mkdir -p '$remote_tmp'"

scp "${ssh_opts[@]}" "$JCS_BUNDLE_PATH" "$ssh_target:$remote_tmp/bundle.tgz" >/dev/null
scp "${ssh_opts[@]}" "$worker_host" "$ssh_target:$remote_tmp/jcs-offline-worker" >/dev/null
scp "${ssh_opts[@]}" "$container_script" "$ssh_target:$remote_tmp/replay-container.sh" >/dev/null

ssh "${ssh_opts[@]}" "$ssh_target" \
  "chmod +x '$remote_tmp/jcs-offline-worker' '$remote_tmp/replay-container.sh' && \
   JCS_BUNDLE_PATH='$remote_tmp/bundle.tgz' \
   JCS_EVIDENCE_PATH='$remote_tmp/evidence.json' \
   JCS_REPLAY_INDEX='$JCS_REPLAY_INDEX' \
   JCS_NODE_ID='$JCS_NODE_ID' \
   JCS_NODE_MODE='$JCS_NODE_MODE' \
   JCS_NODE_DISTRO='$JCS_NODE_DISTRO' \
   JCS_NODE_KERNEL_FAMILY='$JCS_NODE_KERNEL_FAMILY' \
   JCS_EVIDENCE_SCHEMA_VERSION='${JCS_EVIDENCE_SCHEMA_VERSION:-evidence.v1}' \
   '$remote_tmp/replay-container.sh' '$image'"

scp "${ssh_opts[@]}" "$ssh_target:$remote_tmp/evidence.json" "$JCS_EVIDENCE_PATH" >/dev/null

if [[ -f "$JCS_EVIDENCE_PATH" ]]; then
  chmod u+rw,go+r "$JCS_EVIDENCE_PATH" >/dev/null 2>&1 || true
fi

ssh "${ssh_opts[@]}" "$ssh_target" "rm -rf '$remote_tmp'" >/dev/null 2>&1 || true
