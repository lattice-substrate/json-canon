// Command jcs-canon canonicalizes and verifies JSON using RFC 8785 JCS.
//
// Stable ABI:
//
//	jcs-canon canonicalize [--quiet] [file|-]
//	jcs-canon verify [--quiet] [file|-]
//	jcs-canon --help
//	jcs-canon --version
//
// Exit codes: 0 (success), 2 (input/profile/non-canonical/usage), 10 (internal/IO).
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lattice-substrate/json-canon/jcs"
	"github.com/lattice-substrate/json-canon/jcserr"
	"github.com/lattice-substrate/json-canon/jcstoken"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 1 {
		switch args[0] {
		case "--help", "-h":
			if err := writeGlobalHelp(stdout); err != nil {
				return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "write help output", err))
			}
			return 0
		case "--version":
			if err := writeVersion(stdout); err != nil {
				return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "write version output", err))
			}
			return 0
		}
	}

	if len(args) == 0 {
		// CLI-EXIT-001
		code := writeClassifiedError(stderr, jcserr.New(jcserr.CLIUsage, -1, "no command specified"))
		if err := writeGlobalHelp(stderr); err != nil {
			return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "write usage output", err))
		}
		return code
	}

	switch args[0] {
	case "canonicalize":
		return cmdCanonicalize(args[1:], stdin, stdout, stderr)
	case "verify":
		return cmdVerify(args[1:], stdin, stdout, stderr)
	default:
		// CLI-EXIT-002
		code := writeClassifiedError(stderr, jcserr.New(jcserr.CLIUsage, -1, fmt.Sprintf("unknown command: %s", args[0])))
		if err := writeGlobalHelp(stderr); err != nil {
			return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "write usage output", err))
		}
		return code
	}
}

type flags struct {
	quiet bool
	help  bool
}

func parseFlags(args []string) (flags, []string, error) {
	var f flags
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--quiet", "-q":
			f.quiet = true
		case "--help", "-h":
			f.help = true
		case "-":
			positional = append(positional, arg)
		default:
			if strings.HasPrefix(arg, "-") {
				// CLI-FLAG-001
				return flags{}, nil, jcserr.New(jcserr.CLIUsage, -1, fmt.Sprintf("unknown option: %s", arg))
			}
			positional = append(positional, arg)
		}
	}
	return f, positional, nil
}

func cmdCanonicalize(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fl, positional, err := parseFlags(args)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}

	// CLI-FLAG-003: subcommand --help writes to stdout (frozen stream policy).
	if fl.help {
		if err := writeCanonicalizeHelp(stdout); err != nil {
			return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "write canonicalize help output", err))
		}
		return 0
	}

	// CLI-IO-002
	if err := ensureSingleInput(positional); err != nil {
		return writeClassifiedError(stderr, err)
	}

	input, err := readInput(positional, stdin, jcstoken.DefaultMaxInputSize)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}

	parsed, err := jcstoken.Parse(input)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}

	canonical, err := jcs.Serialize(parsed)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}

	// CLI-IO-004: output to stdout only
	if _, err := stdout.Write(canonical); err != nil {
		return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "writing output", err))
	}

	return 0
}

func cmdVerify(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fl, positional, err := parseFlags(args)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}

	// CLI-FLAG-003: subcommand --help writes to stdout (frozen stream policy).
	if fl.help {
		if err := writeVerifyHelp(stdout); err != nil {
			return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "write verify help output", err))
		}
		return 0
	}

	if err := ensureSingleInput(positional); err != nil {
		return writeClassifiedError(stderr, err)
	}

	input, err := readInput(positional, stdin, jcstoken.DefaultMaxInputSize)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}

	parsed, err := jcstoken.Parse(input)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}

	canonical, err := jcs.Serialize(parsed)
	if err != nil {
		return writeClassifiedError(stderr, err)
	}

	// VERIFY-ORDER-001, VERIFY-WS-001
	if !bytes.Equal(input, canonical) {
		return writeClassifiedError(stderr, jcserr.New(jcserr.NotCanonical, -1, "input is not canonical"))
	}

	// CLI-IO-005, CLI-FLAG-002
	if !fl.quiet {
		if err := writeLine(stderr, "ok"); err != nil {
			return writeClassifiedError(stderr, jcserr.Wrap(jcserr.InternalIO, -1, "writing verify success output", err))
		}
	}
	return 0
}

// writeClassifiedError extracts jcserr.Error if possible and uses its exit code.
func writeClassifiedError(stderr io.Writer, err error) int {
	var je *jcserr.Error
	if errors.As(err, &je) {
		_ = writef(stderr, "error: %v\n", err)
		return je.Class.ExitCode()
	}
	return writeErrorAndReturn(stderr, jcserr.InternalError.ExitCode(), "error: %v\n", err)
}

func readInput(positional []string, stdin io.Reader, maxInputSize int) ([]byte, error) {
	// CLI-IO-001
	if len(positional) == 0 || positional[0] == "-" {
		return readBounded(stdin, maxInputSize)
	}

	f, err := os.Open(positional[0])
	if err != nil {
		return nil, jcserr.Wrap(jcserr.CLIUsage, -1, fmt.Sprintf("read file %q", positional[0]), err)
	}
	defer func() { _ = f.Close() }()

	data, err := readBounded(f, maxInputSize)
	if err != nil {
		var je *jcserr.Error
		if errors.As(err, &je) && je.Class == jcserr.BoundExceeded {
			return nil, err
		}
		return nil, jcserr.Wrap(jcserr.CLIUsage, -1, fmt.Sprintf("read file %q", positional[0]), err)
	}
	return data, nil
}

func readBounded(r io.Reader, maxInputSize int) ([]byte, error) {
	lr := io.LimitReader(r, int64(maxInputSize)+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, jcserr.Wrap(jcserr.InternalIO, -1, "read input stream", err)
	}
	if len(data) > maxInputSize {
		return nil, jcserr.New(
			jcserr.BoundExceeded,
			0,
			fmt.Sprintf("input exceeds maximum size %d bytes", maxInputSize),
		)
	}
	return data, nil
}

func ensureSingleInput(positional []string) error {
	if len(positional) <= 1 {
		return nil
	}
	return jcserr.New(jcserr.CLIUsage, -1, "multiple input files specified")
}

func writeErrorAndReturn(stderr io.Writer, code int, format string, args ...any) int {
	_ = writef(stderr, format, args...)
	return code
}

func writeCanonicalizeHelp(w io.Writer) error {
	if err := writeLine(w, "usage: jcs-canon canonicalize [--quiet] [file|-]"); err != nil {
		return err
	}
	if err := writeLine(w, "  Read JSON from file (or stdin), emit canonical bytes to stdout."); err != nil {
		return err
	}
	return writeLine(w, "  --quiet   Accepted for command symmetry; canonicalize is silent on success")
}

func writeGlobalHelp(w io.Writer) error {
	if err := writeLine(w, "usage: jcs-canon <canonicalize|verify> [options] [file|-]"); err != nil {
		return err
	}
	if err := writeLine(w, "       jcs-canon --help"); err != nil {
		return err
	}
	if err := writeLine(w, "       jcs-canon --version"); err != nil {
		return err
	}
	if err := writeLine(w, "commands: canonicalize, verify"); err != nil {
		return err
	}
	return writeLine(w, "flags: --help, -h, --version")
}

func writeVersion(w io.Writer) error {
	return writeLine(w, "jcs-canon "+version)
}

func writeVerifyHelp(w io.Writer) error {
	if err := writeLine(w, "usage: jcs-canon verify [--quiet] [file|-]"); err != nil {
		return err
	}
	if err := writeLine(w, "  Parse, canonicalize, and compare bytes to verify canonical form."); err != nil {
		return err
	}
	return writeLine(w, "  --quiet  Suppress success messages")
}

func writeLine(w io.Writer, msg string) error {
	return writef(w, "%s\n", msg)
}

func writef(w io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return fmt.Errorf("write stream: %w", err)
	}
	return nil
}

var version = "v0.0.0-dev"
