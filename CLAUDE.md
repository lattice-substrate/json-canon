# CLAUDE.md

## Project

`json-canon` — RFC 8785 JSON Canonicalization Scheme. Pure Go, zero external dependencies.

**Module path:** `github.com/lattice-substrate/json-canon`
**Go version:** 1.22.5

## Commands

```bash
# Run all tests (unit + conformance + golden oracles)
go test ./... -count=1 -timeout=10m

# Run only unit tests (fast, ~3s)
go test ./jcserr ./jcsfloat ./jcstoken ./jcs -count=1

# Run conformance harness (builds CLI binary, runs all requirement checks)
go test ./conformance -count=1 -timeout=10m

# Run a single conformance requirement
go test ./conformance -count=1 -run 'TestConformanceRequirements/CANON-SORT-001'

# Lint
go vet ./...

# Build static binary
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" -o jcs-canon ./cmd/jcs-canon

# Run everything
go test ./... -count=1 -timeout=10m
```

## Architecture

Five packages, strict dependency order:

```
jcserr          ← failure taxonomy (no deps)
  ↑
jcsfloat        ← ECMA-262 Number::toString (depends on jcserr)
  ↑
jcstoken        ← strict JSON parser (depends on jcserr)
  ↑
jcs             ← RFC 8785 serializer (depends on jcsfloat, jcstoken)
  ↑
cmd/jcs-canon   ← CLI binary (depends on jcs, jcstoken, jcserr)
```

`conformance/` is a test-only package that builds the CLI binary and runs black-box subprocess tests.

## Normative Specs

Every requirement traces to one of these. Do not invent requirements that aren't in these documents:

| Spec | What it governs |
|------|-----------------|
| RFC 8785 | Canonicalization scheme (sorting, escaping, whitespace, number format) |
| RFC 8259 | JSON grammar |
| RFC 7493 | I-JSON (duplicate keys, surrogates, noncharacters) |
| RFC 3629 | UTF-8 validity |
| ECMA-262 §6.1.6.1.20 | Number::toString algorithm |
| IEEE 754-2008 | Binary64 semantics (NaN, Infinity, -0, subnormals) |

## Key Design Decisions

- **All errors are `*jcserr.Error`** with a `FailureClass` that maps to exit codes. This enables conformance vectors to verify failure classification, not just "did it fail."
- **UTF-16 code-unit sort** for object keys (RFC 8785 §3.2.3). Uses `unicode/utf16.Encode([]rune(key))` for comparison. This differs from UTF-8 byte order for supplementary-plane characters.
- **-0 handled at two layers**: lexical `-0` token rejected at parse time (`PROF-NEGZ-001`); IEEE 754 negative zero bit pattern serializes as `"0"` at format time (`ECMA-FMT-002`). These are distinct requirements.
- **Burger-Dybvig algorithm** with `math/big.Int` for exact arithmetic and ECMA-262 Note 2 even-digit tie-breaking. Pre-computed `pow10Cache[700]`.
- **No nondeterminism sources** in core packages — no `math/rand`, `crypto/rand`, `time`, or map iteration for ordering.

## Requirement System

Three files form the requirement traceability chain:

1. **`REQ_REGISTRY.md`** — 86 formal requirements, each citing a specific RFC section
2. **`FAILURE_TAXONOMY.md`** — 13 failure classes with exit code mappings
3. **`REQ_ENFORCEMENT_MATRIX.md`** — CSV mapping every requirement to implementation symbols and test functions

The conformance harness (`conformance/harness_test.go`) parses `REQ_REGISTRY.md` to extract requirement IDs and validates bidirectional coverage: every requirement has a check, every check maps to a requirement.

## Test Naming Convention

- `TestParse_<REQ_ID>` — parser unit tests in `jcstoken/token_test.go`
- `TestSerialize_<REQ_ID>` — serializer unit tests in `jcs/serialize_test.go`
- `TestFormatDouble_<REQ_ID>` — float formatter unit tests in `jcsfloat/jcsfloat_test.go`
- `TestGoldenOracle` / `TestStressOracle` — 286K V8 oracle vectors
- `TestConformanceRequirements/<REQ_ID>` — conformance harness subtests

## Sacred Files (Never Delete)

These golden vector files are V8-generated oracles pinned by SHA-256. They cannot be regenerated without Node.js and must be preserved across rewrites:

- `jcsfloat/testdata/golden_vectors.csv` (54,445 rows, SHA-256: `593bdec...`)
- `jcsfloat/testdata/golden_stress_vectors.csv` (231,917 rows, SHA-256: `287d21a...`)
- `jcsfloat/testdata/generate_golden.js`
- `jcsfloat/testdata/generate_stress_golden.js`
- `jcsfloat/testdata/README.md`

## CLI ABI

```
jcs-canon canonicalize [--quiet] [file|-]   # parse → canonicalize → stdout
jcs-canon verify [--quiet] [file|-]         # parse → canonicalize → byte-compare
jcs-canon --help                            # top-level stable help (exit 0)
jcs-canon --version                         # top-level stable version (exit 0)
```

Exit codes: `0` success, `2` input/validation/non-canonical/usage, `10` internal/IO error.

Compatibility policy: strict SemVer for CLI ABI.

## Common Pitfalls

- `jcstoken.Parse` returns `(*Value, error)` — the `error` is always `*jcserr.Error` underneath, but the interface type is `error`. Use `errors.As` to extract.
- `jcsfloat.FormatDouble` returns `(string, *jcserr.Error)` — concrete type, not interface.
- `jcs/serialize.go` returns classified `*jcserr.Error` values (wrapped as `error`) from validation paths; use `errors.As` to extract classes.
- The fuzz test for `FormatDouble` must allow -0 → +0 bit change (ECMA-FMT-002 mandates `-0` → `"0"`).
- Test strings containing U+E000 need Go escape `\uE000`, not raw copy-paste (renders identically but may not byte-match in source).
