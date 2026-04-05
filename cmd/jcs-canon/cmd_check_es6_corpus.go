package main

// cmdCheckES6Corpus implements the check-es6-corpus subcommand.
// It runs the deterministic official ES6 number corpus sweep and writes the
// SHA-256 digest to stdout, enabling governance-layer black-box verification
// without importing jcsfloat internals.

import (
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/lattice-substrate/json-canon/jcserr"
	"github.com/lattice-substrate/json-canon/jcsfloat"
)

const (
	checkES6CorpusDefaultLines = 10_000
	checkES6CorpusMaxLines     = 100_000_000
)

func cmdCheckES6Corpus(args []string, stdout, stderr io.Writer) int {
	fl, positional, err := parseFlags(args)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}
	if fl.help {
		if helpErr := writeCheckES6CorpusHelp(stdout); helpErr != nil {
			return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "write help output", helpErr))
		}
		return 0
	}
	if len(positional) > 0 {
		return writeClassifiedError(stderr, jcserr.New(jcserr.CLIUsage, -1, "check-es6-corpus takes no positional arguments"))
	}

	n, linesErr := resolveCorpusLineCount(fl.lines)
	if linesErr != nil {
		return writeClassifiedError(stderr, linesErr)
	}

	digest, computeErr := computeES6CorpusSHA256(n)
	if computeErr != nil {
		return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "computing ES6 corpus digest", computeErr))
	}

	if _, writeErr := fmt.Fprintf(stdout, "%s\n", digest); writeErr != nil {
		return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "writing output", writeErr))
	}
	if _, statusErr := fmt.Fprintf(stderr, "lines=%d\n", n); statusErr != nil {
		return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "writing status", statusErr))
	}
	return 0
}

func resolveCorpusLineCount(flagLines int) (int, error) {
	n := checkES6CorpusDefaultLines
	if flagLines != -1 {
		n = flagLines
	}
	if n < 1 || n > checkES6CorpusMaxLines {
		return 0, jcserr.New(jcserr.CLIUsage, -1,
			fmt.Sprintf("--lines must be between 1 and %d, got %d", checkES6CorpusMaxLines, n))
	}
	return n, nil
}

// computeES6CorpusSHA256 generates n values from the official ES6 corpus and
// returns the lowercase hex SHA-256 of the sequence.
// Each line has the form: <16-hex-bits>,<formatted-double> followed by a newline.
func computeES6CorpusSHA256(n int) (string, error) {
	next := jcsfloat.NewOfficialES6Generator()
	h := sha256.New()
	line := make([]byte, 0, 96)

	for i := 1; i <= n; i++ {
		f := next()
		formatted, err := jcsfloat.FormatDouble(f)
		if err != nil {
			return "", fmt.Errorf("line %d: format error: %w", i, err)
		}
		line = strconv.AppendUint(line[:0], math.Float64bits(f), 16)
		line = append(line, ',')
		line = append(line, formatted...)
		line = append(line, '\n')
		if _, err := h.Write(line); err != nil {
			return "", fmt.Errorf("line %d: hash write error: %w", i, err)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func writeCheckES6CorpusHelp(w io.Writer) error {
	_, err := fmt.Fprint(w, `Usage: jcs-canon check-es6-corpus [--lines N]

Compute the official ES6 number corpus SHA-256 for the first N lines
of the deterministic float corpus (default: 10000).

Each line is: <16-hex-float-bits>,<ecmascript-formatted-value>\n
The SHA-256 is computed over all lines in order and written to stdout.

This command exists to enable governance-layer black-box verification:
the governance harness invokes this binary and compares the output against
the governed checksum in governance.lock.json.

Flags:
  --lines N   Number of corpus lines (1..100000000, default: 10000)
  --quiet     Accepted for command symmetry; check-es6-corpus produces no success output to suppress
  --help, -h  Display this help and exit 0

Exit codes: 0 (success), 2 (usage error), 10 (I/O error)
`)
	if err != nil {
		return fmt.Errorf("write help output: %w", err)
	}
	return nil
}
