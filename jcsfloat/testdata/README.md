# Golden Vectors

`golden_vectors.csv` is a pinned, vendored reference dataset used by `jcsfloat` tests.

Properties enforced in Go tests:

- 54,445 rows
- CSV format: `<16-hex-bits>,<expected-string>`
- SHA-256: `b7cf58a7d9de15cd27adb95ee596f4a3092ec3ace2fc52a6e065a28dbe81f438`

This repository's production validation flow is Go-only and does not require external runtimes.
