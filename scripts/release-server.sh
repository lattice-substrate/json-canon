#!/usr/bin/env bash
# release-server.sh: bootstrap pinned Go/toolchain artifacts, then hand off the full
# server-backed evidence orchestration to jcs-offline-replay server-evidence.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

AWS_REGION="${AWS_REGION:-us-east-1}"
: "${TAG:?'TAG is required (e.g. v0.4.0)'}"
AMI_LOCK_PATH="${AMI_LOCK_PATH:-$ROOT/infra/aws_release_hosts.lock.json}"
STATE_MODE="${STATE_MODE:-remote}"
STATE_BUCKET="${STATE_BUCKET:-}"
STATE_REGION="${STATE_REGION:-$AWS_REGION}"
STATE_LOCK_TABLE="${STATE_LOCK_TABLE:-}"
STATE_KEY="${STATE_KEY:-server-evidence/$TAG/terraform.tfstate}"

if [[ "$STATE_MODE" != "remote" ]]; then
  echo "error: release-server.sh only supports STATE_MODE=remote; local state is debug-only and non-conformant" >&2
  exit 2
fi
if [[ -z "$STATE_BUCKET" || -z "$STATE_LOCK_TABLE" ]]; then
  echo "error: remote state requires STATE_BUCKET and STATE_LOCK_TABLE" >&2
  exit 2
fi

AWS_SHARED_CREDENTIALS_FILE="${AWS_SHARED_CREDENTIALS_FILE:-$HOME/.aws/credentials}"
if [[ -z "${AWS_PROFILE:-}" && -z "${AWS_ACCESS_KEY_ID:-}" && ! -f "$AWS_SHARED_CREDENTIALS_FILE" ]]; then
  echo "error: no AWS credentials found; set AWS_PROFILE, export AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, or provide $AWS_SHARED_CREDENTIALS_FILE" >&2
  exit 2
fi

AMI_LOCK_PATH="$(cd "$(dirname "$AMI_LOCK_PATH")" && pwd)/$(basename "$AMI_LOCK_PATH")"
if [[ ! -f "$AMI_LOCK_PATH" ]]; then
  echo "error: aws ami lock not found at $AMI_LOCK_PATH" >&2
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
  --host-arch "$HOST_ARCH" \
  --purposes build,provision >/dev/null

# shellcheck disable=SC1090
source "$TOOLCHAIN_ENV"

server_args=(
  server-evidence
  --tag "$TAG"
  --aws-region "$AWS_REGION"
  --ami-lock "$AMI_LOCK_PATH"
  --toolchain-lock "$TOOL_LOCK"
  --toolchain-root "$TOOLCHAIN_ROOT"
  --host-arch "$HOST_ARCH"
  --output-dir "$OUT_DIR"
  --state-mode "$STATE_MODE"
)

if [[ -n "$STATE_BUCKET" ]]; then
  server_args+=(--state-bucket "$STATE_BUCKET")
fi
if [[ -n "$STATE_REGION" ]]; then
  server_args+=(--state-region "$STATE_REGION")
fi
if [[ -n "$STATE_LOCK_TABLE" ]]; then
  server_args+=(--state-lock-table "$STATE_LOCK_TABLE")
fi
if [[ -n "$STATE_KEY" ]]; then
  server_args+=(--state-key "$STATE_KEY")
fi
if [[ -n "${GOVERNANCE_LOCK_PATH:-}" || -n "${GOVERNANCE_UMBRELLA_COMMIT:-}" || -n "${GOVERNANCE_LOCK_SHA256:-}" ]]; then
  if [[ -z "${GOVERNANCE_LOCK_PATH:-}" || -z "${GOVERNANCE_UMBRELLA_COMMIT:-}" || -z "${GOVERNANCE_LOCK_SHA256:-}" ]]; then
    echo "error: governance binding requires GOVERNANCE_LOCK_PATH, GOVERNANCE_UMBRELLA_COMMIT, and GOVERNANCE_LOCK_SHA256 together" >&2
    exit 2
  fi
  server_args+=(
    --governance-lock "$GOVERNANCE_LOCK_PATH"
    --governance-umbrella-commit "$GOVERNANCE_UMBRELLA_COMMIT"
    --governance-lock-sha256 "$GOVERNANCE_LOCK_SHA256"
  )
fi

"$JCS_TOOL_GO" run -mod=readonly "$ROOT/cmd/jcs-offline-replay" "${server_args[@]}"
