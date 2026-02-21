# Correctness Runbook

This runbook produces release evidence for `jcs-canon`.

## 1. Preconditions

- Linux environment.
- Go 1.22+.
- Run from repository root.

## 2. Validate pinned number oracle

```bash
go test ./jcsfloat -run 'TestFormatDoubleGoldenVectors|TestGoldenVectorsChecksum' -count=1
```

Acceptance:

- both tests pass,
- vectors are exactly 54,445 rows,
- checksum matches pinned value.

## 3. Full build and test

```bash
go build ./...
go test ./... -count=1
go test ./... -race -count=1
```

## 4. Static-friendly release build

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o jcs-canon ./cmd/jcs-canon
file ./jcs-canon
sha256sum ./jcs-canon
```

## 5. Functional smoke checks

```bash
echo '{"z":3,"a":1}' | ./jcs-canon canonicalize
printf '%s' '{"a":1,"z":3}' | ./jcs-canon verify --quiet -; echo $?
printf '%s' '-0' | ./jcs-canon verify --quiet -; echo $?
```

Expected:

- canonicalize emits `{"a":1,"z":3}`,
- verify canonical input exits `0`,
- verify `-0` exits `2`.

## 6. Evidence bundle

Store:

- `go version`,
- targeted oracle-test output,
- full test output,
- binary checksum and metadata,
- smoke-check transcript with exit codes.
