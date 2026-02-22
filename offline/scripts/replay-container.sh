#!/usr/bin/env bash
set -euo pipefail

image="${1:-}"
if [[ -z "$image" ]]; then
  echo "usage: replay-container.sh <image>" >&2
  exit 2
fi

: "${JCS_BUNDLE_PATH:?JCS_BUNDLE_PATH is required}"
: "${JCS_EVIDENCE_PATH:?JCS_EVIDENCE_PATH is required}"
: "${JCS_REPLAY_INDEX:?JCS_REPLAY_INDEX is required}"
: "${JCS_NODE_ID:?JCS_NODE_ID is required}"
: "${JCS_NODE_MODE:?JCS_NODE_MODE is required}"
: "${JCS_NODE_DISTRO:?JCS_NODE_DISTRO is required}"
: "${JCS_NODE_KERNEL_FAMILY:?JCS_NODE_KERNEL_FAMILY is required}"

if [[ ! -f "$JCS_BUNDLE_PATH" ]]; then
  echo "bundle does not exist: $JCS_BUNDLE_PATH" >&2
  exit 2
fi

engine="${JCS_CONTAINER_ENGINE:-}"
if [[ -z "$engine" ]]; then
  if command -v podman >/dev/null 2>&1; then
    engine="podman"
  elif command -v docker >/dev/null 2>&1; then
    engine="docker"
  else
    echo "neither podman nor docker found" >&2
    exit 2
  fi
fi

if ! command -v "$engine" >/dev/null 2>&1; then
  echo "container engine not found: $engine" >&2
  exit 2
fi

if ! inspect_out="$("$engine" image inspect "$image" 2>&1)"; then
  if grep -qi "permission denied\\|cannot connect\\|daemon" <<<"$inspect_out"; then
    echo "$inspect_out" >&2
  else
    echo "offline image missing locally: $image" >&2
  fi
  exit 2
fi

evidence_dir="$(dirname "$JCS_EVIDENCE_PATH")"
evidence_file="$(basename "$JCS_EVIDENCE_PATH")"
mkdir -p "$evidence_dir"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

if ! tar -xzf "$JCS_BUNDLE_PATH" -C "$tmpdir" bundle/jcs-offline-worker >/dev/null 2>&1; then
  echo "failed to extract worker from bundle" >&2
  exit 2
fi
worker_host="$tmpdir/bundle/jcs-offline-worker"
chmod +x "$worker_host"

container_name="jcs-replay-${JCS_NODE_ID}-${JCS_REPLAY_INDEX}-$$"

"$engine" run --rm --name "$container_name" \
  --network none \
  -v "$JCS_BUNDLE_PATH:/work/bundle.tgz:ro" \
  -v "$worker_host:/work/jcs-offline-worker:ro" \
  -v "$evidence_dir:/work/out" \
  -e LC_ALL=C \
  -e LANG=C \
  -e TZ=UTC \
  "$image" \
  /work/jcs-offline-worker \
    --bundle /work/bundle.tgz \
    --evidence "/work/out/$evidence_file" \
    --node-id "$JCS_NODE_ID" \
    --mode "$JCS_NODE_MODE" \
    --distro "$JCS_NODE_DISTRO" \
    --kernel-family "$JCS_NODE_KERNEL_FAMILY" \
    --replay-index "$JCS_REPLAY_INDEX"
