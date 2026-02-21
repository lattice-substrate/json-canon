# Documentation Index and Navigation

**Status:** Draft  
**Audience:** implementers, auditors, security reviewers, infra/tooling maintainers

## Reading paths

### Path A: Audit / security review
1. **01-motivation.md**
2. **02-normative-references.md**
3. **03-conformance-terms.md**
4. **10-requirements/requirements.md**
5. **05-security-considerations.md**
6. **12-comparative-analysis/cyberphone-analysis.md**
7. **11-testing/test-plan.md** + **11-testing/determinism-matrix.md**

### Path B: Implementation (build it)
1. **02-normative-references.md**
2. **10-requirements/requirements.md**
3. **06-architecture.md**
4. **07-algorithms/** (tokenizer → UTF‑8 → strings → sorting → numbers → emitter)
5. **08-api.md**
6. **09-error-codes.md**
7. **11-testing/** (vector format + harness)

### Path C: Integrate into other infrastructure tooling
1. **08-api.md**
2. **09-error-codes.md**
3. **13-operations/integration-guide.md**
4. **13-operations/release-and-compatibility.md**
5. **13-operations/performance-and-limits.md**

## Document map

### Core specification
- **01-motivation.md** — why strict JCS (fail-closed) is required for auditable crypto use
- **02-normative-references.md** — authoritative source documents and what they contribute
- **03-conformance-terms.md** — conformance language (BCP 14), terminology, and “what does conforming mean”
- **04-scope.md** — in-scope / out-of-scope; compatibility promises

### Design and algorithms
- **06-architecture.md** — package boundaries, data flow, streaming constraints
- **07-algorithms/** — normative algorithm documents

### Interfaces and errors
- **08-api.md** — stable API contract (Go) and compatibility rules
- **09-error-codes.md** — stable error code registry and semantics

### Requirements and traceability
- **10-requirements/requirements.md** — requirement IDs + sources + acceptance criteria
- **10-requirements/requirements.csv** — machine-readable form
- **10-requirements/traceability.md** — mapping requirements → code → tests → vectors

### Testing and evidence
- **11-testing/test-plan.md**
- **11-testing/vector-format.md**
- **11-testing/v8-differential.md**
- **11-testing/determinism-matrix.md**
- **11-testing/fuzzing.md**

### Comparative analysis
- **12-comparative-analysis/stdlib-analysis.md**
- **12-comparative-analysis/cyberphone-analysis.md**

### Operations
- **13-operations/integration-guide.md**
- **13-operations/release-and-compatibility.md**
- **13-operations/performance-and-limits.md**

### Architectural decisions (ADR)
- **14-adr/** — decision records, suitable for long-lived infra projects

### Documentation standards
- **15-style-guide.md** — how docs are written, normative wording rules, references

## Corpus (test vectors)
See **corpus/README.md** for the vector layout and how it is used by the test harness.
