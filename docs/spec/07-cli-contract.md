# 07. CLI Contract

Binary name: `lattice-canon`

## 7.1 Commands

### canonicalize

Usage:

`lattice-canon canonicalize [--gjcs1] [--quiet] [file|-]`

Behavior:

- Reads input from file if provided, else stdin.
- Parses using strict profile.
- Writes canonical JCS bytes to stdout.
- With `--gjcs1`, appends trailing LF.

### verify

Usage:

`lattice-canon verify [--quiet] [file|-]`

Behavior:

- Reads governed bytes from file or stdin.
- Verifies GJCS1 constraints and canonical equivalence.

If `--quiet` is absent and verification succeeds, implementation writes `ok` plus newline to stderr.

## 7.2 Input Source Rules

- No positional path or `-` means stdin.
- `-` explicitly means stdin.
- More than one positional path MUST fail.

## 7.3 Output Channels

- Canonical bytes MUST be written to stdout.
- Diagnostic errors MUST be written to stderr.
