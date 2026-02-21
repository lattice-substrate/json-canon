#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_DIR="$ROOT_DIR/.evidence/$TS"
mkdir -p "$OUT_DIR"

export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
export GOMODCACHE="${GOMODCACHE:-/tmp/go-mod-cache}"
mkdir -p "$GOCACHE" "$GOMODCACHE"

{
  echo "timestamp_utc=$TS"
  echo "repo=$ROOT_DIR"
  go version
  node --version
} | tee "$OUT_DIR/environment.txt"

echo "[1/6] Regenerating golden vectors"
node jcsfloat/testdata/generate_golden.js > jcsfloat/testdata/golden_vectors.csv 2> "$OUT_DIR/golden_generate.stderr"
wc -l jcsfloat/testdata/golden_vectors.csv | tee "$OUT_DIR/golden_line_count.txt"
sha256sum jcsfloat/testdata/golden_vectors.csv | tee "$OUT_DIR/golden_sha256.txt"

echo "[2/6] Building all packages"
go build ./... 2>&1 | tee "$OUT_DIR/go_build.txt"

echo "[3/6] Running tests"
go test ./... -count=1 2>&1 | tee "$OUT_DIR/go_test.txt"

echo "[4/6] Building static release binary"
CGO_ENABLED=0 go build -ldflags="-s -w" -o lattice-canon ./cmd/lattice-canon
file ./lattice-canon | tee "$OUT_DIR/binary_file.txt"
sha256sum ./lattice-canon | tee "$OUT_DIR/binary_sha256.txt"

echo "[5/6] Running smoke checks"
{
  echo "canonicalize_basic:"
  echo '{"z":3,"a":1}' | ./lattice-canon canonicalize

  echo "verify_canonical_exit:"
  printf '{"a":1,"z":3}\n' | ./lattice-canon verify --quiet -
  echo "$?"

  echo "verify_negative_zero_exit:"
  set +e
  printf '%s\n' '-0' | ./lattice-canon verify --quiet - >/dev/null 2>&1
  echo "$?"
  set -e
} | tee "$OUT_DIR/smoke.txt"

echo "[6/6] Capturing test function count"
rg -n '^func Test' jcsfloat/jcsfloat_test.go jcstoken/token_test.go jcs/serialize_test.go gjcs1/gjcs1_test.go cmd/lattice-canon/main_test.go \
  | tee "$OUT_DIR/test_functions.txt"

COUNT="$(wc -l < "$OUT_DIR/test_functions.txt")"
echo "test_function_count=$COUNT" | tee "$OUT_DIR/test_function_count.txt"

echo "Evidence bundle written to: $OUT_DIR"
