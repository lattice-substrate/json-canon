#!/usr/bin/env bash
# release-server.sh: provisions AWS EC2 instances, runs server-backed offline replay,
# emits evidence.v2 with infra-manifest binding, validates via Go release gate, destroys.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

: "${AWS_ACCESS_KEY_ID:?'AWS_ACCESS_KEY_ID is required'}"
: "${AWS_SECRET_ACCESS_KEY:?'AWS_SECRET_ACCESS_KEY is required'}"
AWS_REGION="${AWS_REGION:-us-east-1}"
: "${TAG:?'TAG is required (e.g. v0.4.0)'}"
: "${SSH_KEY_PATH:?'SSH_KEY_PATH must point to a private key file'}"
: "${SSH_INGRESS_CIDR:?'SSH_INGRESS_CIDR is required (for example 203.0.113.10/32)'}"

SSH_KEY_PATH="$(realpath "$SSH_KEY_PATH")"
SSH_PUB_PATH="${SSH_KEY_PATH}.pub"
if [[ ! -f "$SSH_PUB_PATH" ]]; then
  echo "error: public key not found at $SSH_PUB_PATH" >&2
  exit 2
fi

for cmd in tofu ssh scp git go sha256sum awk sed; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "error: missing required command: $cmd" >&2; exit 2; }
done

LOCK_FILE="$ROOT/infra/.terraform.lock.hcl"
if [[ ! -f "$LOCK_FILE" ]]; then
  echo "error: $LOCK_FILE not found; run 'cd infra && tofu init' once and commit the lockfile" >&2
  exit 2
fi

GIT_COMMIT="$(git -C "$ROOT" rev-parse HEAD)"
LOCK_SHA="$(sha256sum "$LOCK_FILE" | awk '{print $1}')"
TOFU_VERSION_RAW="$(tofu version | head -n1)"
TOFU_VERSION="${TOFU_VERSION_RAW##*v}"
SERVER_CONTAINER_IMAGE_TAG="${SERVER_CONTAINER_IMAGE_TAG:-debian:12-slim}"

export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION

echo "==> provisioning infrastructure (tag=$TAG, commit=${GIT_COMMIT:0:12})"

cd "$ROOT/infra"
tofu init -input=false -upgrade=false
tofu apply -auto-approve -input=false \
  -var "ssh_public_key=$(cat "$SSH_PUB_PATH")" \
  -var "infra_repo_url=https://github.com/lattice-substrate/json-canon" \
  -var "infra_repo_commit=$GIT_COMMIT" \
  -var "provider_lock_sha256=$LOCK_SHA" \
  -var "aws_region=$AWS_REGION" \
  -var "ssh_ingress_cidr=$SSH_INGRESS_CIDR"

X86_IP="$(tofu output -raw x86_64_public_ip)"
ARM_IP="$(tofu output -raw arm64_public_ip)"
X86_IMAGE_ID="$(tofu output -raw x86_64_ami)"
ARM_IMAGE_ID="$(tofu output -raw arm64_ami)"
X86_INSTANCE_TYPE="$(tofu output -raw x86_64_instance_type)"
ARM_INSTANCE_TYPE="$(tofu output -raw arm64_instance_type)"
cd "$ROOT"

echo "==> instances ready: x86_64=$X86_IP  arm64=$ARM_IP"

teardown() {
  echo "==> tearing down infrastructure"
  cd "$ROOT/infra"
  tofu destroy -auto-approve -input=false \
    -var "ssh_public_key=$(cat "$SSH_PUB_PATH")" \
    -var "infra_repo_url=https://github.com/lattice-substrate/json-canon" \
    -var "infra_repo_commit=$GIT_COMMIT" \
    -var "provider_lock_sha256=$LOCK_SHA" \
    -var "aws_region=$AWS_REGION" \
    -var "ssh_ingress_cidr=$SSH_INGRESS_CIDR" || true
  cd "$ROOT"
}
trap teardown EXIT

SSH_OPTS="-i $SSH_KEY_PATH -o StrictHostKeyChecking=no -o ConnectTimeout=15 -o ServerAliveInterval=30"

echo "==> waiting for SSH on both instances"
for host in "$X86_IP" "$ARM_IP"; do
  connected=0
  for _ in $(seq 1 90); do
    # shellcheck disable=SC2086
    if ssh $SSH_OPTS "admin@$host" true >/dev/null 2>&1; then
      connected=1
      break
    fi
    sleep 2
  done
  if [[ "$connected" -eq 0 ]]; then
    echo "error: SSH unreachable after 3 minutes: $host" >&2
    exit 2
  fi
done

install_remote_container_runtime() {
  local host="$1"
  # shellcheck disable=SC2086
  ssh $SSH_OPTS "admin@$host" \
    "sudo apt-get update >/dev/null && sudo apt-get install -y docker.io >/dev/null && sudo systemctl enable --now docker >/dev/null"
}

resolve_remote_image_digest() {
  local host="$1"
  # shellcheck disable=SC2086
  ssh $SSH_OPTS "admin@$host" \
    "sudo docker pull '$SERVER_CONTAINER_IMAGE_TAG' >/dev/null && sudo docker image inspect --format='{{index .RepoDigests 0}}' '$SERVER_CONTAINER_IMAGE_TAG'"
}

discover_remote_cpu() {
  local host="$1"
  # shellcheck disable=SC2086
  ssh $SSH_OPTS "admin@$host" "grep 'model name' /proc/cpuinfo | head -1 | cut -d: -f2 | xargs" 2>/dev/null || true
}

discover_remote_kernel() {
  local host="$1"
  # shellcheck disable=SC2086
  ssh $SSH_OPTS "admin@$host" "uname -r" 2>/dev/null || true
}

echo "==> preparing remote container runtime"
install_remote_container_runtime "$X86_IP"
install_remote_container_runtime "$ARM_IP"

echo "==> resolving digest-pinned container images on provisioned hosts"
X86_CONTAINER_IMAGE="$(resolve_remote_image_digest "$X86_IP")"
ARM64_CONTAINER_IMAGE="$(resolve_remote_image_digest "$ARM_IP")"

echo "==> discovering substrate identity"
X86_CPU="$(discover_remote_cpu "$X86_IP")"
X86_KERNEL="$(discover_remote_kernel "$X86_IP")"
ARM_CPU="$(discover_remote_cpu "$ARM_IP")"
ARM_KERNEL="$(discover_remote_kernel "$ARM_IP")"

echo "    x86_64 cpu: $X86_CPU"
echo "    x86_64 kernel: $X86_KERNEL"
echo "    x86_64 image: $X86_CONTAINER_IMAGE"
echo "    arm64 cpu: $ARM_CPU"
echo "    arm64 kernel: $ARM_KERNEL"
echo "    arm64 image: $ARM64_CONTAINER_IMAGE"

OUT_DIR="$ROOT/offline/runs/releases/$TAG"
mkdir -p "$OUT_DIR/x86_64" "$OUT_DIR/arm64"

INFRA_MANIFEST_PATH="$OUT_DIR/infra-manifest.v1.json"
echo "==> writing infra manifest"
go run "$ROOT/cmd/jcs-offline-replay" write-infra-manifest \
  --output "$INFRA_MANIFEST_PATH" \
  --infra-repo-url "https://github.com/lattice-substrate/json-canon" \
  --infra-repo-commit "$GIT_COMMIT" \
  --provider-engine "opentofu" \
  --provider-version "$TOFU_VERSION" \
  --provider-lock-sha256 "$LOCK_SHA" \
  --cloud-provider "aws" \
  --region "$AWS_REGION" \
  --x86-instance-type "$X86_INSTANCE_TYPE" \
  --x86-image-id "$X86_IMAGE_ID" \
  --x86-discovered-cpu "$X86_CPU" \
  --x86-discovered-kernel "$X86_KERNEL" \
  --arm64-instance-type "$ARM_INSTANCE_TYPE" \
  --arm64-image-id "$ARM_IMAGE_ID" \
  --arm64-discovered-cpu "$ARM_CPU" \
  --arm64-discovered-kernel "$ARM_KERNEL"

echo "==> building architecture-specific jcs-canon control binaries"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=$TAG" \
  -o /tmp/jcs-server-canon-x86_64 "$ROOT/cmd/jcs-canon"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=$TAG" \
  -o /tmp/jcs-server-canon-arm64 "$ROOT/cmd/jcs-canon"

export JCS_SERVER_X86_SSH_TARGET="admin@$X86_IP"
export JCS_SERVER_X86_SSH_OPTIONS="$SSH_OPTS"
export JCS_SERVER_X86_CONTAINER_IMAGE="$X86_CONTAINER_IMAGE"
export JCS_SERVER_ARM64_SSH_TARGET="admin@$ARM_IP"
export JCS_SERVER_ARM64_SSH_OPTIONS="$SSH_OPTS"
export JCS_SERVER_ARM64_CONTAINER_IMAGE="$ARM64_CONTAINER_IMAGE"

echo "==> preparing x86_64 bundle"
go run "$ROOT/cmd/jcs-offline-replay" prepare \
  --matrix "$ROOT/offline/matrix.server-x86_64.yaml" \
  --profile "$ROOT/offline/profiles/server-linux-x86_64.yaml" \
  --binary /tmp/jcs-server-canon-x86_64 \
  --bundle /tmp/server-bundle-x86_64.tgz

echo "==> running x86_64 replay"
go run "$ROOT/cmd/jcs-offline-replay" run \
  --matrix "$ROOT/offline/matrix.server-x86_64.yaml" \
  --profile "$ROOT/offline/profiles/server-linux-x86_64.yaml" \
  --bundle /tmp/server-bundle-x86_64.tgz \
  --evidence "$OUT_DIR/x86_64/offline-evidence.json" \
  --infra-manifest "$INFRA_MANIFEST_PATH" \
  --source-git-commit "$GIT_COMMIT" \
  --source-git-tag "$TAG"

echo "==> preparing arm64 bundle"
go run "$ROOT/cmd/jcs-offline-replay" prepare \
  --matrix "$ROOT/offline/matrix.server-arm64.yaml" \
  --profile "$ROOT/offline/profiles/server-linux-arm64.yaml" \
  --binary /tmp/jcs-server-canon-arm64 \
  --bundle /tmp/server-bundle-arm64.tgz

echo "==> running arm64 replay"
go run "$ROOT/cmd/jcs-offline-replay" run \
  --matrix "$ROOT/offline/matrix.server-arm64.yaml" \
  --profile "$ROOT/offline/profiles/server-linux-arm64.yaml" \
  --bundle /tmp/server-bundle-arm64.tgz \
  --evidence "$OUT_DIR/arm64/offline-evidence.json" \
  --infra-manifest "$INFRA_MANIFEST_PATH" \
  --source-git-commit "$GIT_COMMIT" \
  --source-git-tag "$TAG"

cp /tmp/server-bundle-x86_64.tgz "$OUT_DIR/x86_64/offline-bundle.tgz"
cp /tmp/server-bundle-arm64.tgz "$OUT_DIR/arm64/offline-bundle.tgz"

echo "==> running release gate: x86_64"
JCS_OFFLINE_EVIDENCE="$OUT_DIR/x86_64/offline-evidence.json" \
JCS_OFFLINE_BUNDLE="$OUT_DIR/x86_64/offline-bundle.tgz" \
JCS_OFFLINE_MATRIX="$ROOT/offline/matrix.server-x86_64.yaml" \
JCS_OFFLINE_PROFILE="$ROOT/offline/profiles/server-linux-x86_64.yaml" \
JCS_OFFLINE_CONTROL_BINARY=/tmp/jcs-server-canon-x86_64 \
JCS_OFFLINE_EXPECTED_GIT_COMMIT="$GIT_COMMIT" \
JCS_OFFLINE_EXPECTED_GIT_TAG="$TAG" \
JCS_OFFLINE_INFRA_MANIFEST="$INFRA_MANIFEST_PATH" \
go test "$ROOT/offline/conformance" -run TestOfflineReplayEvidenceReleaseGate -count=1 -v

echo "==> running release gate: arm64"
JCS_OFFLINE_EVIDENCE="$OUT_DIR/arm64/offline-evidence.json" \
JCS_OFFLINE_BUNDLE="$OUT_DIR/arm64/offline-bundle.tgz" \
JCS_OFFLINE_MATRIX="$ROOT/offline/matrix.server-arm64.yaml" \
JCS_OFFLINE_PROFILE="$ROOT/offline/profiles/server-linux-arm64.yaml" \
JCS_OFFLINE_CONTROL_BINARY=/tmp/jcs-server-canon-arm64 \
JCS_OFFLINE_EXPECTED_GIT_COMMIT="$GIT_COMMIT" \
JCS_OFFLINE_EXPECTED_GIT_TAG="$TAG" \
JCS_OFFLINE_INFRA_MANIFEST="$INFRA_MANIFEST_PATH" \
go test "$ROOT/offline/conformance" -run TestOfflineReplayEvidenceReleaseGate -count=1 -v

echo ""
echo "==> SUCCESS: server evidence written to $OUT_DIR"
echo "    x86_64 evidence: $OUT_DIR/x86_64/offline-evidence.json"
echo "    arm64 evidence:  $OUT_DIR/arm64/offline-evidence.json"
echo "    infra manifest:  $INFRA_MANIFEST_PATH"
