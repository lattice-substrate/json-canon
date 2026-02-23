# Governance

## Maintainer Policy

### Review Requirements

- All changes require review by an active maintainer.
- ABI-impacting changes (commands, flags, exit codes, output format) require:
  - review from two maintainers when two or more active maintainers exist;
  - documented self-review (risk checklist + rationale) when exactly one active
    maintainer exists.
- Major version release requires explicit signoff from all active maintainers.
- Any new shell-script-based required gate (CI/release/conformance/traceability)
  requires explicit maintainer approval with written rationale in the PR.

### Maintainer Responsibilities

1. Triage incoming issues within 10 business days.
2. Review pull requests within 15 business days.
3. Follow the security triage process defined in `SECURITY.md`.
4. Maintain traceability: update registries, matrix, and tests for all
   behavioral changes.
5. Enforce Go-first automation for infrastructure-critical checks; permit shell
   usage only via explicit, documented exception.
6. Enforce no-outbound-call runtime policy: no network egress or subprocess
   execution in core runtime packages.

### Maintainer Succession

- If two or more maintainers are active, inactive maintainers (6+ months) may
  be replaced by documented consensus of remaining maintainers.
- If exactly one maintainer is active and becomes inactive for 6+ months, the
  project enters maintenance-only status until a successor is appointed.
- The release process is documented sufficiently for a new maintainer to
  execute independently (see `VERIFICATION.md`, `CONTRIBUTING.md`).

## Support Window Policy

Supported operating environment: Linux only.

| Version | Support Level |
|---------|-------------|
| Pre-v1 release candidates (`v0.x.y-rcN`) | Best effort: compatibility stabilization, critical bug fixes, and release process hardening only |
| Current major (v1.x.y) | Full: bug fixes, security patches, compatibility maintenance |
| Previous major (v0.x.y) | Security-only: critical and high severity fixes for 12 months after current major release |
| Older versions | Unsupported |

## Deprecation Policy

### CLI Behavior

1. Deprecations are announced in the `CHANGELOG.md` at least one minor version
   before removal.
2. Deprecated features emit a warning to stderr when used.
3. Removal occurs only in a new major version.
4. The ABI manifest (`abi_manifest.json`) is updated to reflect deprecation status.

### Diagnostics

1. Failure class names are stable and never deprecated (they are ABI).
2. Diagnostic message wording may change in any release (non-ABI).
3. Exit code mappings are stable and never deprecated (they are ABI).

## Decision Records

Decisions with compatibility impact are recorded in:

- `CHANGELOG.md` under the relevant release section
- `docs/adr/` (authoritative architectural decision records)

Rationale details for specific domains are captured in:
- `FAILURE_TAXONOMY.md` (error classification rationale)
- `BOUNDS.md` (resource limit rationale)
- `standards/CITATION_INDEX.md` (standards interpretation decisions)
