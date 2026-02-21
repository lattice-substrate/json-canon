# Golden Vectors

`golden_vectors.csv` is a pinned, vendored reference dataset used by `jcsfloat` tests.

Regenerate with:

```bash
node jcsfloat/testdata/generate_golden.js > jcsfloat/testdata/golden_vectors.csv
```

Properties enforced in Go tests:

- 54,445 rows
- CSV format: `<16-hex-bits>,<expected-string>`
- SHA-256: `593bdecbe0dccbc182bc3baf570b716887db25739fc61b7808764ecb966d5636`

This repository's production validation flow is Go-only and does not require external runtimes.
