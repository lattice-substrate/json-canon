# Correctness Runbook

This runbook produces objective evidence that the implementation is behaving correctly for a release candidate.

## 1. Preconditions

- Linux environment.
- Go 1.22+.
- Node.js available for vector regeneration.
- Run from repository root.

Optional for restricted environments:

```bash
export GOCACHE=/tmp/go-build-cache
export GOMODCACHE=/tmp/go-mod-cache
mkdir -p "$GOCACHE" "$GOMODCACHE"
```

## 2. Regenerate and validate golden vectors

```bash
node jcsfloat/testdata/generate_golden.js > jcsfloat/testdata/golden_vectors.csv
wc -l jcsfloat/testdata/golden_vectors.csv
sha256sum jcsfloat/testdata/golden_vectors.csv
```

Acceptance criteria:

- generator reports `Generated 54445 golden vectors` on stderr.
- line count is exactly `54445`.
- checksum is recorded in release evidence.

## 3. Full build and test

```bash
go build ./...
go test ./... -count=1
```

Acceptance criteria:

- all packages pass.
- zero test failures.

## 4. Static release binary build

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o lattice-canon ./cmd/lattice-canon
file ./lattice-canon
sha256sum ./lattice-canon
```

Acceptance criteria:

- binary is ELF, statically linked, stripped.
- checksum recorded in release evidence.

## 5. Functional smoke checks

```bash
echo '{"z":3,"a":1}' | ./lattice-canon canonicalize
printf '{"a":1,"z":3}\n' | ./lattice-canon verify --quiet -
printf '%s\n' '-0' | ./lattice-canon verify --quiet -; echo $?
```

Expected:

- canonicalize emits `{"a":1,"z":3}`.
- verify on canonical input exits `0`.
- verify on `-0` exits `2` (profile rejection).

## 6. Enforced evidence bundle

Store the following artifacts per release:

- `go version` output.
- vector generation stderr line.
- vector file line count and SHA-256.
- full `go test ./... -count=1` output.
- release binary SHA-256 and `file` output.
- smoke check transcript with exit codes.

## 7. One-command evidence collection

Use the helper script:

```bash
bash docs/scripts/prove_correctness.sh
```

It writes timestamped evidence files under `.evidence/`.

## 8. Release stop conditions

Do not release if any of the following occurs:

- golden vectors not exactly `54,445`.
- any package test fails.
- smoke checks do not match expected outcomes.
- binary build fails or is not static.
