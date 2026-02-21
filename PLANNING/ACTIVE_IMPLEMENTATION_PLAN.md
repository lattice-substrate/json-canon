# Program Plan: Decades-Grade Stable-ABI JCS Infrastructure

Status: ACTIVE
Owner: Maintainers
Effective Date: 2026-02-21
Supersedes: none

## 1. Program Charter

1. Deliver a long-lived, low-level canonicalization toolchain with strict SemVer ABI stability guarantees.
2. Prove correctness against RFC 8785, RFC 8259, RFC 7493, RFC 3629, ECMA-262 Number::toString semantics, and IEEE-754 binary64 behavior.
3. Make conformance and release trust independently verifiable by third parties.
4. Enforce reproducibility, supply-chain integrity, and governance standards suitable for infrastructure consumed by security-sensitive systems.

## 2. Non-Negotiable Invariants

1. No unresolved Critical or High findings before any release.
2. No undocumented ABI behavior.
3. No requirement without executable enforcement evidence.
4. No release artifacts without authenticity and provenance evidence.
5. No standards claim without source-cited proof.

## 3. Formal ABI Contract (v1 Baseline)

1. ABI surface includes command set, flags, flag semantics, exit classes, stdout canonical bytes, stderr channel contract, and process exit behavior.
2. `jcs-canon --help`, `jcs-canon -h`, `jcs-canon --version`, `canonicalize`, `verify` are stable.
3. Exit class mapping is stable for all published failure classes.
4. Any change in command/flag behavior or exit mapping requires major version.
5. Stderr wording is non-stable unless explicitly frozen; channel and semantic class remain stable.
6. ABI contract must be encoded in machine-readable form (`abi_manifest.json`) and verified in CI.

## 4. Standards Baseline Control

1. Mirror authoritative standards into a pinned `standards/` directory with source URL, retrieval date, SHA256, and clause index.
2. Include RFC 8785, RFC 8259, RFC 7493, RFC 3629, ECMA-262 Number::toString clause snapshot, and IEEE references used by numeric claims.
3. Create `standards/CITATION_INDEX.md` mapping each requirement ID to exact clause identifiers.
4. Add CI check that requirement registry clauses resolve to citation index entries.

## 5. Requirement Architecture and Traceability Hardening

1. Keep split registries (`REQ_REGISTRY_NORMATIVE.md`, `REQ_REGISTRY_POLICY.md`) as canonical sources.
2. Enforce strict schema for requirement rows with ID uniqueness, clause presence, and normative level.
3. Keep `REQ_ENFORCEMENT_MATRIX.md` domain-tagged (`normative|policy`) with machine-verified column schema.
4. Add scripts:
- `scripts/req/check_registry_schema.sh`
- `scripts/req/check_registry_matrix_parity.sh`
- `scripts/req/check_impl_symbol_exists.sh`
- `scripts/req/check_test_symbol_exists.sh`
- `scripts/req/check_line_drift.sh`
5. Fail CI if any requirement lacks both implementation symbol and executable test mapping.
6. Fail CI if any conformance check exists without a registered requirement ID.

## 6. Immediate Closure of Existing Audit Findings

1. F-001: Decide and freeze subcommand help output stream policy; update code/tests/docs/ABI manifest consistently.
2. F-002: Correct stale registry references in contributor docs.
3. F-003: Add explicit private vulnerability reporting channel with contact path and SLA.
4. F-004: Pin all third-party GitHub Actions by commit SHA and document update policy.
5. F-005: Add explicit safety invariant comments/assertions for digit buffer bounds without introducing production panic fragility.
6. F-006: Extend CI Go matrix to currently supported versions plus current stable.
7. F-007: Expand JSONL vector corpus to meaningful interoperability-grade breadth.
8. F-008: Keep current file-read classification or formally redesign in next major; document rationale unambiguously.
9. F-009: Convert nondeterministic map-iterated test tables to deterministic slices where practical.
10. F-010: Publish parser memory behavior and amplification notes under bounds configuration.

## 7. Parser and Unicode Assurance Workstream

1. Build a clause-by-clause parser compliance test map for RFC 8259 and RFC 7493.
2. Maintain source-byte offset contract as explicit API behavior for diagnostics.
3. Add adversarial tests for:
- Incomplete and malformed escapes.
- Surrogate pair failure in second escape segment.
- Noncharacters across BMP and supplementary planes.
- UTF-8 invalid classes including overlong and truncated sequences.
4. Add fuzz harnesses with committed seed corpus for grammar, strings, and escapes.
5. Add memory and depth adversarial tests at configured limits and near-limit boundaries.
6. Add deterministic error-class assertions for stdin/file/source-equivalent failures.

## 8. Number Formatting Assurance Workstream

1. Keep oracle parity suite against V8 vectors as mandatory.
2. Add independent differential checks against at least one additional runtime implementation where possible.
3. Add targeted boundary suites:
- 1e-6 and 1e21 formatting transitions.
- Subnormal boundaries including min subnormal.
- Tie-to-even edge cases.
- Negative zero serialization and lexical rejection distinction.
4. Add invariants test suite proving maximum digit path safety assumptions.
5. Add deterministic round-trip property tests for binary64 coverage slices.
6. Require citation-backed justification for all algorithmic shortcuts.

## 9. Canonicalization Assurance Workstream

1. Enforce canonical string escaping behaviors clause-by-clause.
2. Enforce UTF-16 code unit sort behavior with adversarial supplementary-plane cases.
3. Enforce no insignificant whitespace in output.
4. Enforce non-normalization policy with normalization-sensitive corpus.
5. Add interoperability vectors that can be consumed externally independent of this codebase.

## 10. Conformance Harness and Vector Program

1. Keep requirement conformance harness mandatory and gating.
2. Keep JSONL vector execution mandatory and gating.
3. Expand vectors into categorized packs:
- Core canonicalization.
- Grammar rejection.
- Unicode/surrogate/noncharacter.
- Numeric boundaries and exponent behavior.
- Taxonomy classification.
- ABI/CLI behavior.
4. Add vector schema file and validator.
5. Publish vector compatibility policy so external implementations can track compatibility by version.

## 11. Determinism and Reproducibility Program

1. Keep deterministic build flags required.
2. Build twice and compare checksums in CI.
3. Add environment-controlled deterministic build test with pinned Go patch version.
4. Add drift detector for nondeterminism sources across core packages.
5. Document deterministic guarantees and exclusions explicitly.

## 12. Supply Chain and Release Trust Program

1. Pin all workflow actions by SHA.
2. Generate and publish SHA256 checksums and signed checksum manifest.
3. Sign release artifacts and/or attestations.
4. Generate provenance statement for release builds.
5. Publish verification guide for consumers with exact commands.
6. Add release gate that fails if signatures/attestations/checksums are missing.

## 13. CI Architecture (Required Gates)

1. Per-PR gates:
- Lint/vet/static checks.
- Unit tests.
- Conformance requirements.
- JSONL vectors.
- Traceability integrity checks.
2. Main-branch gates:
- Race tests.
- Full matrix across Linux/macOS/Windows.
- Supported Go version matrix.
- Reproducible build check.
3. Scheduled gates:
- Extended fuzz budget.
- Standards drift check (links, clause map integrity).
- Dependency and action pin review.
4. Flake policy:
- Zero tolerated for conformance/traceability/safety gates.
- Any flake requires root-cause issue before merge.

## 14. Governance and OSS Longevity

1. LICENSE/NOTICE/SECURITY/CONTRIBUTING/CHANGELOG must remain versioned and current.
2. Add maintainer policy:
- Minimum two reviewers for ABI-impacting changes.
- Major release checklist signoff.
- Security triage ownership and SLA.
3. Add support window policy:
- Current major supported.
- Previous major security maintenance window explicit.
4. Add deprecation policy for CLI behavior and diagnostics.

## 15. Documentation Quality Program

1. Ensure all docs reference split registries and current conformance model.
2. Publish stable ABI spec page separate from README quickstart.
3. Publish error taxonomy contract page with class semantics and compatibility rules.
4. Publish memory/bounds operational guide.
5. Publish interoperability guide for external reimplementations.

## 16. Evidence and Audit Artifacts

1. Maintain `AUDIT_RFC8785_YYYY-MM-DD.md` as generated evidence summary, not marketing text.
2. Include command transcript summary and result digests for every release candidate.
3. Include registry/matrix counts and parity outputs.
4. Include conformance and vector execution counts.
5. Include signed artifact verification evidence.

## 17. Program Phases and Exit Criteria

1. Phase A (Release Blockers):
- Close F-002 and F-003.
- Freeze F-001 decision and tests.
- Exit: all blocker findings closed.
2. Phase B (Conformance Certainty):
- Standards citation index complete.
- Differential/runtime parity expanded.
- Vector corpus expanded.
- Exit: clause-to-test-to-code mapping complete and green.
3. Phase C (Release Trust):
- Action pinning, signing, provenance.
- Exit: release pipeline emits verifiable trusted artifacts.
4. Phase D (Longevity Hardening):
- Fuzz corpus, benchmarks, memory docs, governance scaling.
- Exit: long-horizon maintenance and performance regression controls active.

## 18. Mandatory Validation Command Set

1. `go vet ./...`
2. `go test ./... -count=1 -timeout=20m`
3. `go test ./... -race -count=1 -timeout=25m`
4. `go test ./conformance -count=1 -timeout=10m -v`
5. Registry/matrix parity and integrity scripts.
6. Reproducible build checksum comparison script.
7. Artifact verification script for release candidates.

## 19. Release Decision Rule

1. GO only when all mandatory gates pass and no unresolved blocker findings exist.
2. NO-GO if any ABI ambiguity, security reporting gap, traceability gap, or unsigned/unverifiable release artifact remains.
3. Every GO decision must cite exact evidence files and command results.

## 20. Success Definition

1. Independent auditors can verify spec claims from source citations and executable evidence.
2. External consumers can validate behavior with portable vectors and ABI manifests.
3. Release artifacts are reproducible, signed, and provenance-backed.
4. ABI compatibility is enforceable by automation, not intent.
5. Maintenance can continue safely beyond individual contributor tenure.
