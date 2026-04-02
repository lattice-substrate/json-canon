#!/usr/bin/env bash
# release-server.sh: bootstrap pinned Go/toolchain artifacts, then hand off the full
# server-backed evidence orchestration to jcs-offline-replay server-evidence.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

: "${AWS_ACCESS_KEY_ID:?'AWS_ACCESS_KEY_ID is required'}"
: "${AWS_SECRET_ACCESS_KEY:?'AWS_SECRET_ACCESS_KEY is required'}"
AWS_REGION="${AWS_REGION:-us-east-1}"
: "${TAG:?'TAG is required (e.g. v0.4.0)'}"
: "${SSH_KEY_PATH:?'SSH_KEY_PATH must point to a private key file'}"
: "${SSH_INGRESS_CIDR:?'SSH_INGRESS_CIDR is required (for example 203.0.113.10/32)'}"

SSH_KEY_PATH="$(cd "$(dirname "$SSH_KEY_PATH")" && pwd)/$(basename "$SSH_KEY_PATH")"
SSH_PUB_PATH="${SSH_KEY_PATH}.pub"
if [[ ! -f "$SSH_PUB_PATH" ]]; then
  echo "error: public key not found at $SSH_PUB_PATH" >&2
  exit 2
fi

LOCK_FILE="$ROOT/infra/.terraform.lock.hcl"
if [[ ! -f "$LOCK_FILE" ]]; then
  echo "error: $LOCK_FILE not found; generate it with ./scripts/init-infra-lock.sh and commit it" >&2
  exit 2
fi

TOOL_LOCK="$ROOT/offline/toolchain.lock.tsv"
if [[ ! -f "$TOOL_LOCK" ]]; then
  echo "error: missing pinned toolchain lock: $TOOL_LOCK" >&2
  exit 2
fi

HOST_ARCH="$(uname -m)"
case "$HOST_ARCH" in
  x86_64|amd64)
    HOST_ARCH="amd64"
    ;;
  aarch64|arm64)
    HOST_ARCH="arm64"
    ;;
  *)
    echo "error: unsupported host architecture: $HOST_ARCH" >&2
    exit 2
    ;;
esac

OUT_DIR="$ROOT/offline/runs/releases/$TAG"
TOOLCHAIN_ROOT="$OUT_DIR/toolchain"
TOOLCHAIN_ENV="$TOOLCHAIN_ROOT/env.sh"
mkdir -p "$OUT_DIR"

"$ROOT/scripts/bootstrap-pinned-toolchain.sh" \
  --output-dir "$TOOLCHAIN_ROOT" \
  --env-file "$TOOLCHAIN_ENV" \
  --host-arch "$HOST_ARCH" >/dev/null

# shellcheck disable=SC1090
source "$TOOLCHAIN_ENV"

"$JCS_TOOL_GO" run -mod=readonly "$ROOT/cmd/jcs-offline-replay" server-evidence \
  --tag "$TAG" \
  --aws-region "$AWS_REGION" \
  --ssh-key-path "$SSH_KEY_PATH" \
  --ssh-ingress-cidr "$SSH_INGRESS_CIDR" \
  --toolchain-lock "$TOOL_LOCK" \
  --toolchain-root "$TOOLCHAIN_ROOT" \
  --host-arch "$HOST_ARCH" \
  --output-dir "$OUT_DIR"
