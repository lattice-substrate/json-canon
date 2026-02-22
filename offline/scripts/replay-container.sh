#!/usr/bin/env bash
set -euo pipefail

echo "offline container runner is environment-specific and must be implemented for your lab." >&2
echo "received image lane: ${1:-unset}" >&2
echo "required env: JCS_BUNDLE_PATH, JCS_EVIDENCE_PATH, JCS_REPLAY_INDEX" >&2
exit 1
