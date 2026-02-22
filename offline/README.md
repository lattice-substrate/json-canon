# Offline Cold Replay Suite

This directory defines the offline cold-replay matrix for Linux distro and kernel variance.

## Contracts

- Matrix lanes: `offline/matrix.yaml`
- Maximal profile: `offline/profiles/maximal.yaml`
- Evidence schema: `offline/schema/evidence.v1.json`

## Operator CLI

`cmd/jcs-offline-replay` provides four subcommands:

- `prepare` builds an immutable replay bundle (binary + vectors + manifests).
- `run` executes the configured matrix and emits evidence JSON.
- `verify-evidence` validates evidence against matrix/profile policy.
- `report` prints a compact node/replay summary.

## Environment-Specific Runners

`offline/scripts/replay-container.sh` and `offline/scripts/replay-libvirt.sh` are fail-closed placeholders.
They must be implemented for your lab runtime (container engine and libvirt topology).

## Hard Release Gate

When release evidence exists, validate it with:

```bash
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1
```

Set `JCS_OFFLINE_EVIDENCE=/path/to/evidence.json` for that gate.
