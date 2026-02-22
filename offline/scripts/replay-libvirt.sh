#!/usr/bin/env bash
set -euo pipefail

echo "offline libvirt runner is environment-specific and must be implemented for your lab." >&2
echo "received domain: ${1:-unset} snapshot: ${2:-unset}" >&2
echo "required env: JCS_BUNDLE_PATH, JCS_EVIDENCE_PATH, JCS_REPLAY_INDEX" >&2
exit 1
