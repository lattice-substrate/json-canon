[Previous: Security, Trust, and Threat Model](08-security-trust-and-threat-model.md) | [Book Home](README.md) | [Next: Troubleshooting](10-troubleshooting.md)

# Chapter 10: Operations and Runbooks

This chapter is for operators running validation, release prep, or incident
triage.

## Daily/PR Validation

Required local gates:

```bash
go vet ./...
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

## Static Release Build Validation

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o ./jcs-canon ./cmd/jcs-canon
file ./jcs-canon
ldd ./jcs-canon
```

Expected result includes static linkage and no dynamic executable dependency.

## Offline Proof Runbooks

- Single architecture: `./offline/scripts/cold-replay-run.sh`
- Cross architecture: `./offline/scripts/cold-replay-cross-arch.sh`
- Preflight only: `./offline/scripts/cold-replay-preflight.sh --matrix <path>`

## Release Candidate Evidence Pack

A complete RC evidence pack should include:

1. command transcript or CI logs for required gates,
2. x86_64 offline run artifacts,
3. arm64 offline run artifacts,
4. release gate test outputs for both architectures,
5. checksums and attestation verification outputs.

## Authoritative Runbooks

- `docs/OFFLINE_REPLAY_HARNESS.md`
- `RELEASE_PROCESS.md`
- `VERIFICATION.md`

[Previous: Security, Trust, and Threat Model](08-security-trust-and-threat-model.md) | [Book Home](README.md) | [Next: Troubleshooting](10-troubleshooting.md)
