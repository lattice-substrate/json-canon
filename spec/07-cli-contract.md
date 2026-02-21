# 07. CLI Contract

Binary name: `jcs-canon`

## 7.1 Commands

### canonicalize

Usage:

`jcs-canon canonicalize [--quiet] [file|-]`

Behavior:

- Reads input from file if provided, else stdin.
- Parses using strict profile.
- Writes canonical RFC 8785 bytes to stdout.

### verify

Usage:

`jcs-canon verify [--quiet] [file|-]`

Behavior:

- Reads JSON bytes from file or stdin.
- Parses using strict profile.
- Canonicalizes and compares byte-for-byte with input.
- On success without `--quiet`, writes `ok` plus newline to stderr.

## 7.2 Input Source Rules

- No positional path or `-` means stdin.
- `-` explicitly means stdin.
- More than one positional path MUST fail.
- Unknown options MUST fail.

## 7.3 Output Channels

- Canonical bytes MUST be written to stdout.
- Diagnostics MUST be written to stderr.
