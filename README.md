# json-canon

RFC 8785 JSON Canonicalization Scheme - pure Go, zero dependencies.

## Overview

`json-canon` implements the JSON Canonicalization Scheme (JCS) as specified in
[RFC 8785](https://www.rfc-editor.org/rfc/rfc8785). It produces deterministic,
byte-identical canonical JSON output suitable for cryptographic hashing and
signature verification.

## Features

- **Pure Go, zero dependencies** - standard library only
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

Requirement registries are split for audit clarity:
- `REQ_REGISTRY_NORMATIVE.md` (RFC/ECMA obligations)
- `REQ_REGISTRY_POLICY.md` (profile/ABI/process policy)

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
