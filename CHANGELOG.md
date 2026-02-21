# Changelog

All notable changes to this project are documented in this file.

This project follows strict [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Publication-readiness governance files (`LICENSE`, `NOTICE`, `SECURITY.md`, `CONTRIBUTING.md`).
- Stable top-level CLI flags: `--help`/`-h` and `--version`.

### Changed
- File-based oversized input now preserves `BOUND_EXCEEDED` classification, matching stdin behavior.
- CI expanded with platform/version matrix, race tests, reproducibility checks, and binary tracking guard.
