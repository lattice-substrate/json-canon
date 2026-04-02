# ADR-0007: Go-Native Post-Bootstrap Server Evidence Orchestration

- Status: Accepted
- Date: 2026-04-01
- Related Requirements: OFFLINE-AUTO-001, AWS-RELEASE-001, AWS-GATE-001

## Context

The pinned-toolchain model established by ADR-0006 removed ambient `go`, `tofu`,
and `jq` from the official AWS evidence path, but the billed orchestration still
lived in shell. That left release-critical automation split across:

- `scripts/init-infra-lock.sh`
- `scripts/release-server.sh`
- transport integrity and host-discovery steps that were not yet fully governed in
  the Go-native path

This conflicted with the project policy that required validation and
release-critical automation be Go-native once the trusted toolchain existed.

## Decision

After pinned Go has been bootstrapped, all release-critical official AWS
automation MUST run through Go subcommands in `jcs-offline-replay`.

The supported model is:

1. `scripts/bootstrap-pinned-toolchain.sh` remains the only shell step with a
   technical bootstrap justification.
2. `jcs-offline-replay init-infra-lock` runs the pinned OpenTofu binary to create
   `infra/.terraform.lock.hcl`.
3. `jcs-offline-replay server-evidence` performs the full billed AWS workflow:
   - pinned OpenTofu init/apply/output/destroy
   - remote CPU / kernel discovery on each native lane
   - AWS instance-identity verification against pinned regional certificates
   - per-replay transport attestation binding evidence bytes to a caller nonce
   - infra-manifest emission
   - per-architecture control/worker builds
   - replay execution
   - release gate execution
4. The billed path uses AWS SSM plus private S3 staging; post-bootstrap
   orchestration no longer depends on ambient host `ssh` / `scp`.
5. `scripts/init-infra-lock.sh` and `scripts/release-server.sh` are reduced to
   bootstrap wrappers that invoke the Go subcommands, and conformant
   `release-server.sh` execution requires remote OpenTofu state.

## Consequences

Positive:

- Post-bootstrap release automation now matches the project’s Go-native policy.
- Ambient host SSH/SCP binaries are removed from the billed official AWS path.
- The requirement/enforcement chain can point to one auditable CLI surface for
  both lockfile generation and official AWS evidence generation.

Costs:

- The CLI surface expands with `init-infra-lock` and `server-evidence`.
- The repository now carries AWS SDK and schema-validation dependencies in source form.

## Rejected Alternative

Keeping the shell orchestration was rejected because it left required
release-critical behavior outside the Go-native conformance surface and preserved
ambient host binary dependencies after the pinned toolchain already existed.
