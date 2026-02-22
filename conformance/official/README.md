# Official Conformance Fixtures

This directory vendors official external conformance fixtures used by Go-native tests.

## cyberphone

`cyberphone/` contains canonicalization fixtures from:

- Repository: `https://github.com/cyberphone/json-canonicalization`
- Source path: `testdata/{input,output,outhex}`

Provenance and per-file checksums are recorded in `cyberphone/UPSTREAM.json`.

## rfc8785

`rfc8785/` contains fixtures derived from RFC 8785 examples:

- `appendix_b.csv`: Appendix B IEEE-754 to canonical string mappings (finite values).
- `key_sorting_input.json` and `key_sorting_output.json`: §3.2.3 object sorting example.

These fixtures are consumed by tests under `conformance/`.
