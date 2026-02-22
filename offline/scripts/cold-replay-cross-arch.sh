#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

CGO_ENABLED=0 go run ./cmd/jcs-offline-replay cross-arch "$@"
