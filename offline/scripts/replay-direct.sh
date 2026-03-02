#!/usr/bin/env bash
set -euo pipefail

# replay-direct.sh executes the offline replay worker directly on the host OS
# without container or VM isolation. Used for cross-OS determinism verification
# where the binary under test is a cross-compiled artifact (e.g., Windows .exe
# executed on a Windows host, or a native binary on the build host for testing).

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

evidence_dir="$(dirname "$JCS_EVIDENCE_PATH")"
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
worker="$tmpdir/bundle/jcs-offline-worker"
chmod +x "$worker"

export LC_ALL=C
export LANG=C
export TZ=UTC

"$worker" \
  --bundle "$JCS_BUNDLE_PATH" \
  --evidence "$JCS_EVIDENCE_PATH" \
  --node-id "$JCS_NODE_ID" \
  --mode "$JCS_NODE_MODE" \
  --distro "$JCS_NODE_DISTRO" \
  --kernel-family "$JCS_NODE_KERNEL_FAMILY" \
  --replay-index "$JCS_REPLAY_INDEX"
