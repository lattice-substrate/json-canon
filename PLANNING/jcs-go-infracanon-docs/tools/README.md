# Tools (Optional)

**Status:** Draft

This directory contains **optional** tooling to generate corpora and run environment matrices. These tools may have external dependencies (e.g., Node.js, Docker). The canonicalizer implementation itself remains dependency-free.

## Contents
- `gen_v8_numbers.js` — generate a V8-based binary64 number corpus (RFC 8785 recommended validation strategy).
- `run_determinism_matrix.sh` — scaffold for running corpus tests across container images.

## References
- RFC 8785 number serializer validation guidance: https://www.rfc-editor.org/rfc/rfc8785
- Docker multi-platform builds docs: https://docs.docker.com/build/building/multi-platform/
