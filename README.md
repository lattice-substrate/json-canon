# json-canon

RFC 8785 JSON Canonicalization Scheme - pure Go canonicalization core with deterministic CLI and infrastructure-grade conformance evidence.

## Overview

`json-canon` implements the JSON Canonicalization Scheme (JCS) as specified in
[RFC 8785](https://www.rfc-editor.org/rfc/rfc8785). It produces deterministic,
byte-identical canonical JSON output suitable for cryptographic hashing and
signature verification.

## Features

- **Pure Go core runtime** - canonicalization engine remains standard-library only
- **ECMA-262 Number::toString** - hand-written Burger-Dybvig algorithm validated
  against 286,362 V8 oracle vectors
- **Strict input validation** - RFC 8259 grammar, RFC 7493 I-JSON constraints
  (duplicate keys, surrogates, noncharacters), RFC 3629 UTF-8
- **UTF-16 code-unit key sorting** - correct supplementary-plane ordering
- **Deterministic and static** - CGO_ENABLED=0 static binary, no nondeterminism sources, no outbound network/subprocess runtime calls
- **Resource bounded** - configurable depth, size, and count limits

## CLI

```text
jcs-canon canonicalize [--quiet] [file|-]
jcs-canon verify [--quiet] [file|-]
jcs-canon --help
jcs-canon --version
```

Exit codes: `0` success, `2` input/validation error, `10` internal error.

## Stability Policy

This project enforces strict SemVer for the published CLI ABI.

Stable ABI scope:
- command set and flag semantics
- exit code taxonomy and failure classes
- canonical stdout bytes for accepted inputs
- stderr success/error channel contract (`verify` uses stderr for `ok\n`)

## Build

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" -o jcs-canon ./cmd/jcs-canon
```

## Test

```bash
go test ./... -count=1 -v
```

## Conformance

```bash
go test ./conformance -count=1 -v -timeout=10m
```

## Live Differential Demo (Cyberphone Go vs json-canon)

Copy-paste this block from repo root to show concrete non-compliance acceptance
in Cyberphone Go and strict rejection in `json-canon`:

```bash
cat > /tmp/cyberphone-canon-demo.go <<'EOF'
package main

import (
	"fmt"
	"io"
	"os"

	cyberphone "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

func main() {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(2)
	}
	out, err := cyberphone.Transform(in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	_, _ = os.Stdout.Write(out)
}
EOF

echo "== Case 1: hex float literal (invalid JSON) =="
printf '%s' '{"n":0x1p-2}' | go run /tmp/cyberphone-canon-demo.go ; echo
printf '%s' '{"n":0x1p-2}' | go run ./cmd/jcs-canon canonicalize -

echo "== Case 2: plus-prefixed number (invalid JSON) =="
printf '%s' '{"n":+1}' | go run /tmp/cyberphone-canon-demo.go ; echo
printf '%s' '{"n":+1}' | go run ./cmd/jcs-canon canonicalize -

echo "== Case 3: leading zero number (invalid JSON) =="
printf '%s' '{"n":01}' | go run /tmp/cyberphone-canon-demo.go ; echo
printf '%s' '{"n":01}' | go run ./cmd/jcs-canon canonicalize -
```

Expected behavior:
- Cyberphone emits canonical-looking JSON for these invalid numeric forms.
- `json-canon` rejects them with deterministic parse-class failures.

Offline cold-replay evidence gate (release workflow):

```bash
JCS_OFFLINE_EVIDENCE=$(pwd)/offline/runs/releases/<tag>/x86_64/offline-evidence.json \
JCS_OFFLINE_CONTROL_BINARY=/abs/path/to/release-control/jcs-canon \
JCS_OFFLINE_EXPECTED_GIT_COMMIT=<release-commit-sha> \
JCS_OFFLINE_EXPECTED_GIT_TAG=<tag> \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1

JCS_OFFLINE_EVIDENCE=$(pwd)/offline/runs/releases/<tag>/arm64/offline-evidence.json \
JCS_OFFLINE_CONTROL_BINARY=/abs/path/to/release-control/jcs-canon \
JCS_OFFLINE_MATRIX=$(pwd)/offline/matrix.arm64.yaml \
JCS_OFFLINE_PROFILE=$(pwd)/offline/profiles/maximal.arm64.yaml \
JCS_OFFLINE_EXPECTED_GIT_COMMIT=<release-commit-sha> \
JCS_OFFLINE_EXPECTED_GIT_TAG=<tag> \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1
```

Operator harness docs and Go-native one-command workflows:
- `docs/OFFLINE_REPLAY_HARNESS.md`
- `jcs-offline-replay run-suite --matrix offline/matrix.yaml --profile offline/profiles/maximal.yaml`
- `jcs-offline-replay cross-arch --run-official-vectors --run-official-es6-100m`

Requirement registries are split for audit clarity:
- `REQ_REGISTRY_NORMATIVE.md` (RFC/ECMA obligations)
- `REQ_REGISTRY_POLICY.md` (profile/ABI/process policy)

## Documentation

Official engineering docs are indexed in `docs/README.md`.
Key references:
- `docs/book/README.md`
- `docs/BOOK.md`
- `ARCHITECTURE.md`
- `ABI.md`
- `NORMATIVE_REFERENCES.md`
- `SPECIFICATION.md`
- `CONFORMANCE.md`
- `THREAT_MODEL.md`
- `RELEASE_PROCESS.md`
- `docs/TRACEABILITY_MODEL.md`
- `docs/VECTOR_FORMAT.md`
- `docs/ALGORITHMIC_INVARIANTS.md`
- `docs/CYBERPHONE_DIFFERENTIAL_EXAMPLES.md`
- `docs/adr/` (architectural decisions)

## Normative References

| Spec | Scope |
|------|-------|
| [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) | JSON Canonicalization Scheme |
| [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) | JSON grammar and data model |
| [RFC 7493](https://www.rfc-editor.org/rfc/rfc7493) | I-JSON: duplicate keys, surrogates, noncharacters |
| [RFC 3629](https://www.rfc-editor.org/rfc/rfc3629) | UTF-8 encoding validity |
| [ECMA-262 §6.1.6.1.20](https://tc39.es/ecma262/#sec-numeric-types-number-tostring) | Number::toString |
| IEEE 754-2008 | Binary64 double-precision semantics |

## License

See `LICENSE`.
