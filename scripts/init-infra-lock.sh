#!/usr/bin/env bash
# init-infra-lock.sh: bootstrap pinned Go/toolchain artifacts, then hand off infra lock
# generation to jcs-offline-replay init-infra-lock.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLCHAIN_ROOT="$ROOT/.tmp/pinned-toolchain-init"
TOOLCHAIN_ENV="$TOOLCHAIN_ROOT/env.sh"

if [[ $# -gt 0 ]]; then
  echo "error: init-infra-lock.sh does not accept positional arguments" >&2
  exit 2
fi

"$ROOT/scripts/bootstrap-pinned-toolchain.sh" \
  --output-dir "$TOOLCHAIN_ROOT" \
  --env-file "$TOOLCHAIN_ENV"

# shellcheck disable=SC1090
source "$TOOLCHAIN_ENV"

"$JCS_TOOL_GO" run -mod=readonly "$ROOT/cmd/jcs-offline-replay" init-infra-lock

echo "next:"
echo "  git add infra/.terraform.lock.hcl"
