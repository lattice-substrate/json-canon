#!/usr/bin/env bash
# bootstrap-pinned-toolchain.sh: materialize the pinned host/remote toolchain from
# offline/toolchain.lock.tsv without relying on ambient go/tofu/jq binaries.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOL_LOCK="$ROOT/offline/toolchain.lock.tsv"
OUTPUT_DIR=""
ENV_FILE=""
HOST_ARCH=""

usage() {
  cat <<'EOF'
usage: ./scripts/bootstrap-pinned-toolchain.sh --output-dir <path> --env-file <path> [--host-arch <amd64|arm64>] [--lock <path>]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --host-arch)
      HOST_ARCH="$2"
      shift 2
      ;;
    --lock)
      TOOL_LOCK="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$OUTPUT_DIR" || -z "$ENV_FILE" ]]; then
  echo "error: --output-dir and --env-file are required" >&2
  usage >&2
  exit 2
fi

if [[ ! -f "$TOOL_LOCK" ]]; then
  echo "error: pinned toolchain lock not found: $TOOL_LOCK" >&2
  exit 2
fi

for cmd in curl sha256sum awk sed tar unzip; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "error: missing required command: $cmd" >&2; exit 2; }
done

if [[ -z "$HOST_ARCH" ]]; then
  HOST_ARCH="$(uname -m)"
fi
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

mkdir -p "$OUTPUT_DIR"

lookup_tool_field() {
  local artifact_id="$1"
  local field_name="$2"
  awk -F '\t' -v artifact_id="$artifact_id" -v field_name="$field_name" '
    /^#/ || NF == 0 { next }
    $1 == "id" {
      for (i = 1; i <= NF; i++) {
        idx[$i] = i
      }
      next
    }
    $1 == artifact_id {
      if (!(field_name in idx)) {
        exit 2
      }
      print $(idx[field_name])
      found = 1
      exit
    }
    END {
      if (!found) exit 1
    }
  ' "$TOOL_LOCK"
}

download_bootstrap_tool() {
  local artifact_id="$1"
  local url sha dest
  url="$(lookup_tool_field "$artifact_id" source_url)"
  sha="$(lookup_tool_field "$artifact_id" sha256)"
  dest="$OUTPUT_DIR/downloads/$artifact_id/$(basename "$url")"
  mkdir -p "$(dirname "$dest")"
  if [[ -f "$dest" ]]; then
    local current_sha
    current_sha="$(sha256sum "$dest" | awk '{print $1}')"
    if [[ "$current_sha" == "$sha" ]]; then
      printf '%s\n' "$dest"
      return
    fi
  fi
  curl -fsSL "$url" -o "$dest"
  local downloaded_sha
  downloaded_sha="$(sha256sum "$dest" | awk '{print $1}')"
  if [[ "$downloaded_sha" != "$sha" ]]; then
    echo "error: bootstrap tool sha256 mismatch for $artifact_id" >&2
    exit 2
  fi
  printf '%s\n' "$dest"
}

bootstrap_go() {
  local artifact_id archive_path extract_root
  artifact_id="go-linux-$HOST_ARCH"
  archive_path="$(download_bootstrap_tool "$artifact_id")"
  extract_root="$OUTPUT_DIR/extracted/$artifact_id"
  rm -rf "$extract_root"
  mkdir -p "$extract_root"
  tar -xzf "$archive_path" -C "$extract_root"
  chmod -R u+rwX,go-rwx "$extract_root/go"
  printf '%s\n' "$extract_root/go/bin/go"
}

GO_BIN="$(bootstrap_go)"
"$GO_BIN" run -mod=readonly "$ROOT/cmd/jcs-offline-replay" sync-toolchain \
  --lock "$TOOL_LOCK" \
  --output-dir "$OUTPUT_DIR" \
  --env-file "$ENV_FILE" \
  --host-arch "$HOST_ARCH" >/dev/null

echo "pinned toolchain ready"
echo "  lock:      $TOOL_LOCK"
echo "  host arch: $HOST_ARCH"
echo "  root:      $OUTPUT_DIR"
echo "  env:       $ENV_FILE"
