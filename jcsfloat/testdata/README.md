# Golden Vectors

`golden_vectors.csv` and `golden_stress_vectors.csv` are pinned, vendored reference datasets used by conformance tests.

Regenerate with:

```bash
node jcsfloat/testdata/generate_golden.js > jcsfloat/testdata/golden_vectors.csv
node jcsfloat/testdata/generate_stress_golden.js > jcsfloat/testdata/golden_stress_vectors.csv
```

Properties enforced in Go tests:

- Base dataset:
  - 54,445 rows
  - SHA-256: `593bdecbe0dccbc182bc3baf570b716887db25739fc61b7808764ecb966d5636`
- Stress dataset:
  - 231,917 rows
  - SHA-256: `287d21ac87e5665550f1baf86038302a0afc67a74a020dffb872f1a93b26d410`
- CSV format for both: `<16-hex-bits>,<expected-string>`

This repository's production validation flow is Go-only and does not require external runtimes.
