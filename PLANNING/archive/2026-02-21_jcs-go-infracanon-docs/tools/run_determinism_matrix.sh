#!/usr/bin/env bash
# run_determinism_matrix.sh
#
# Example driver to execute corpus tests across multiple container images.
# This is a scaffold; adapt to your repo layout and test command.
#
# References:
# - RFC 8785 output is intended for cryptographic methods: https://www.rfc-editor.org/rfc/rfc8785
# - Docker multi-platform builds: https://docs.docker.com/build/building/multi-platform/

set -euo pipefail

IMAGES=(
  "debian:stable-slim"
  "ubuntu:24.04"
  "alpine:3.19"
  "fedora:latest"
)

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
echo "Repo root: $ROOT"

for img in "${IMAGES[@]}"; do
  echo "== $img =="
  docker run --rm -v "$ROOT:/w" -w /w "$img" sh -lc '
    set -e
    # Expect a prebuilt static binary at ./bin/jcs
    ./bin/jcs --selftest-corpus ./corpus
  '
done
