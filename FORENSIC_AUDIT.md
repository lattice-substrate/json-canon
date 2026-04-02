# Forensic Audit: AWS Native Conformance Gating Infrastructure

**Branch:** `feat/amzn-s3-conformance-testing`
**Date:** 2026-04-02
**Auditor:** Claude (Opus 4.6), directed by project maintainer
**Method:** Line-by-line code reading against published engineering standards

Sources: [Part 3](https://lattice-substrate.github.io/blog/2026/02/25/small-decisions-infrastructure-primitive/),
[Part 4](https://lattice-substrate.github.io/blog/2026/02/24/proving-determinism-evidence-release/),
[Lattice Substrate](https://lattice-substrate.github.io/),
[ProveMark](https://provemark.io/)

---

## Architecture Summary

The branch replaces the original SSH-based AWS evidence path with a
private-network, SSM-based architecture:

- **Transport:** AWS Systems Manager shell scripts via VPC interface
  endpoints. No SSH, no public IPs, no inbound security group rules.
- **Artifact staging:** Private S3 bucket per release run. Workers
  download bundles and upload evidence via presigned URLs.
- **Evidence integrity:** Worker emits `evidence_sha256=<hex>` to
  stdout; orchestrator downloads the S3 object and rejects on
  mismatch (`server_ssm.go:153-162`).
- **Infrastructure lifecycle:** Signal handler
  (`server_evidence.go:185`), deferred destroy
  (`server_evidence.go:212-217`), explicit cleanup on provision
  failure (`server_evidence.go:206-210`).
- **Network isolation:** Dedicated VPC, private subnets, no internet
  gateway. Egress restricted to SSM, SSM Messages, EC2 Messages, and
  S3 endpoints only (`main.tf:87-112`). IMDSv2 enforced
  (`instances.tf:28-30`).
- **AMI stability:** Official runs default to committed lock file
  (`aws_release_hosts.lock.json`) with pinned AMI IDs, not live
  wildcard resolution (`server_evidence.go:172`).
- **State management:** S3 backend declared (`main.tf:2`), remote
  mode supported via `--state-mode=remote` with bucket, region, lock
  table, and per-tag key (`server_evidence.go:235-261`,
  `server_evidence.go:854-873`).

---

## What Works

1. **SSM + private S3 eliminates SSH trust entirely.** No host keys,
   no TOFU, no key management. IAM credential chain handles auth.
   SSM agent communicates over VPC endpoints. This is stronger than
   any SSH remediation.

2. **Evidence integrity is verified.** The worker computes SHA-256 on
   the remote host (`server_ssm.go:194`), uploads evidence to S3 via
   presigned PUT (`server_ssm.go:195`), and emits the hash to stdout
   (`server_ssm.go:196`). The orchestrator downloads, recomputes,
   and rejects on mismatch (`server_ssm.go:161`). Evidence is
   written atomically via temp-file-then-rename
   (`server_evidence.go:1135-1151`, called at `server_ssm.go:164`).

3. **Signal handling exists.** `signal.NotifyContext` for SIGINT and
   SIGTERM at `server_evidence.go:185`. The signal-aware context is
   passed to `provision` and `execute`.

4. **Network security is correct.** Private subnets
   (`instances.tf:19`), no public IPs, IMDSv2 required
   (`instances.tf:28-30`), egress locked to endpoints only
   (`main.tf:92-106`), IAM instance profile with SSM-only policy
   (`main.tf:195-213`). This is a properly isolated conformance
   environment.

5. **Evidence download is bounded.** `readBoundedStagingObject`
   (`server_aws.go:509-528`) rejects unknown `ContentLength`,
   rejects objects exceeding `serverMaxEvidenceBytes` (256 MiB,
   `server_aws.go:32`), and uses `io.LimitedReader` as a secondary
   guard. Coverage: 84.6%.

6. **Cleanup context is correct.** `destroy()` at
   `server_evidence.go:631-664` uses `cleanupContext()`
   (`server_evidence.go:1131-1133`) for both bucket deletion
   (line 643) and infrastructure destroy (line 649). `cleanupContext`
   creates `context.WithTimeout(context.WithoutCancel(parent), ...)`,
   so neither operation inherits a cancelled parent. Both get a fresh
   20-minute timeout (`serverProvisionTimeout`).

7. **AMI lock file prevents drift.** `aws_release_hosts.lock.json`
   pins exact AMI IDs. `instances.tf:16` reads `ami_id` directly
   from the lock, not from live lookups. The `refresh-ami-lock`
   subcommand (`server_aws.go:398`) resolves AMIs from the catalog
   and writes the lock, but official runs consume the committed lock.

8. **Provider constraint is tight.** `main.tf:7` uses `~> 5.100`,
   not the loose `~> 5.0` cited in the prior audit.

9. **Remote state is supported.** `main.tf:2` declares
   `backend "s3" {}`. `initServerInfrastructure`
   (`server_evidence.go:854-873`) passes `-backend-config` arguments
   for bucket, region, DynamoDB lock table, key, and encryption when
   `--state-mode=remote`. Per-tag state keys default to
   `server-evidence/<tag>/terraform.tfstate`
   (`server_evidence.go:255`).

10. **Substrate discovery validates against provisioning.** Instance
    ID and image ID discovered via SSM are cross-checked against
    OpenTofu-provisioned values (`server_ssm.go:89-94`). IID document
    and signature SHA-256 hashes are collected and recorded in the
    infra manifest (`server_evidence.go:950-951`).

11. **Function-level seams for testing.** Package-level `var` function
    pointers (`server_evidence.go:37-45`, `server_aws.go:36-42`)
    allow test injection of fake AWS clients, fake provisioners, fake
    SSM runners. `provision`, `execute`, and `destroy` methods also
    accept override functions (`server_evidence.go:124-127`).

---

## What Is Blocking

### B-1: Orchestration test coverage is thin

**Severity:** Critical
**Evidence:** `go test -coverprofile` per-function output:

| Function | File:Line | Coverage |
|---|---|---|
| `runServerEvidence` | `server_evidence.go:184` | 87.5% |
| `destroy` | `server_evidence.go:631` | 68.2% |
| `provision` | `server_evidence.go:387` | 9.5% |
| `execute` | `server_evidence.go:419` | 11.8% |
| `newServerEvidenceRuntime` | `server_evidence.go:274` | 0.0% |
| `discoverRemoteFacts` | `server_evidence.go:447` | 0.0% |
| `writeInfraManifest` | `server_evidence.go:519` | 0.0% |
| `buildArtifacts` | `server_evidence.go:558` | 0.0% |
| `runReplays` | `server_evidence.go:579` | 0.0% |
| `runReleaseGates` | `server_evidence.go:608` | 0.0% |
| All `server_aws.go` functions except `readBoundedStagingObject` | | 0.0% |
| All `server_ssm.go` functions | | 0.0% |

83 lines of tests (`server_evidence_test.go`) for 1,992 lines of
orchestration code (`server_evidence.go` + `server_aws.go` +
`server_ssm.go`). The 3 test functions cover string parsing
helpers: `TestResolveGitHeadCommitLooseGitDir`,
`TestBuildRemoteReplaySSMCommandNativeVM`,
`TestParseEvidenceSHA256`.

The function-pointer seams at `server_evidence.go:37-45` and
`server_aws.go:36-42` exist specifically to enable fake-client
testing but are not yet exercised by any test. `provision`,
`execute`, and `destroy` method overrides
(`server_evidence.go:124-127`) are similarly unused in tests.

**Why this blocks merge:** Part 4 ("Proving Determinism") argues that
evidence claims must be executable. The tool that generates
conformance evidence has no test covering: provision failure +
cleanup, cancellation + destroy, SHA mismatch on downloaded evidence,
SSM timeout handling, release-gate failure, or the full
provision-execute-destroy lifecycle. The project's published standard
("governed evidence, not assertions") applies to the orchestrator
just as much as to the canonicalizer.

**Remedy:** Write tests using the existing function-pointer seams.
Minimum coverage targets:
- `runServerEvidence` happy path with fake provision/execute/destroy
- Provision failure triggers destroy
- Cancellation during execute triggers destroy
- SHA mismatch on evidence download rejects the run
- Release gate failure propagates correctly
- Destroy errors are joined, not swallowed

### B-2: `resolveTofuVersion` ignores the run context

**Severity:** Moderate
**Location:** `server_evidence.go:983-984`

```go
func resolveTofuVersion(ctx context.Context, tofuBinary, infraDir string) (string, error) {
	out, err := runCommandInDirFunc(ctx, infraDir, nil, tofuBinary, "version")
```

This function now receives `ctx` from its caller at line 307.
However, `buildGoBinary` at line 996-1004 creates its own
`context.WithTimeout(parent, serverBuildTimeout)` from the parent
context, which is correct — it inherits cancellation from the signal
context.

**Status:** `resolveTofuVersion` is fixed (receives `ctx`).
`buildGoBinary` is correct (derives from parent). `tofuOutputHosts`
at line 885 also receives `ctx`. The subprocess context discipline
is consistent throughout the current code.

**Verdict:** Not blocking.

---

## What Is Not Blocking But Worth Noting

### N-1: No Terraform variable validation blocks

**Severity:** Low
**Location:** `variables.tf:1-30`

No `validation {}` blocks on any variable. Invalid
`infra_repo_commit` or `provider_lock_sha256` values are caught
downstream (by the Go orchestrator or OpenTofu apply) rather than
at plan time. This produces later, less clear errors but does not
affect evidence integrity.

### N-2: Package-level coverage obscures the gap

**Severity:** Informational
**Evidence:** `go test -cover` reports 50.4% for
`cmd/jcs-offline-replay`. This number comes from well-tested
toolchain, lock, and helper code. The 0.0% on the AWS/SSM
orchestration path is invisible in the aggregate. Anyone citing
"50% coverage" without the per-function breakdown would
overstate assurance on the evidence-generation path.

### N-3: Local state is the default

**Severity:** Low
**Location:** `server_evidence.go:238`

`--state-mode` defaults to `local`. Remote state requires explicit
`--state-mode=remote --state-bucket=... --state-lock-table=...`.
For single-operator release runs this is acceptable. For
multi-operator or CI-managed releases, remote state should be
required. Document the operational limitation if local remains the
default.

### N-4: `buildGoBinary` uses a fixed 5-minute timeout

**Severity:** Low
**Location:** `server_evidence.go:1004`

`context.WithTimeout(parent, serverBuildTimeout)` where
`serverBuildTimeout = 5 * time.Minute` (line 32). Cross-compilation
of two binaries per architecture (4 total) runs sequentially. If
any single build exceeds 5 minutes (possible on constrained
hardware), the run fails with a timeout error rather than a build
error. This is a UX issue, not a correctness issue.

### N-5: Egress endpoint SG allows response traffic within VPC

**Severity:** Informational
**Location:** `main.tf:127-133`

The VPC endpoint security group allows all-protocol egress within
the VPC CIDR (`protocol = "-1"`, `cidr_blocks = [aws_vpc.replay.cidr_block]`).
This is required for endpoint response traffic. It does not
weaken the instance egress restriction, which is enforced by the
instance security group (`main.tf:87-112`).

---

## Decision

**Do not merge until B-1 is resolved.** The architecture is sound.
The transport, integrity, lifecycle, and network isolation are all
correct in the current code. The blocking issue is narrow but real:
the orchestration path that generates evidence is untested, and the
test seams to fix this already exist in the code.

### Required before merge

1. Add orchestration tests exercising the function-pointer seams at
   `server_evidence.go:37-45` and `server_aws.go:36-42`. Cover:
   happy path, provision failure + cleanup, cancellation + destroy,
   evidence SHA mismatch, release-gate failure.

### Not required for merge

2. Terraform variable validation blocks (N-1) — take in hardening
   pass.
3. Default to remote state (N-3) — document the limitation, decide
   on default based on operational model.

---

## Verification

```
go test ./cmd/jcs-offline-replay/... -cover -count=1   # 50.4%
go test ./offline/replay/... -count=1                   # pass
go test ./offline/conformance/... -count=1              # pass
go test ./cmd/jcs-offline-worker/... -count=1           # pass
go test ./conformance/... -count=1 -timeout=10m         # pass
```

Per-function coverage was obtained via `go tool cover -func` on the
profile from the first command above. All findings reference the
code at the current HEAD of `feat/amzn-s3-conformance-testing`.
