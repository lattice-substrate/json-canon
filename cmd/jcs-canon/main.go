// Command jcs-canon canonicalizes and verifies JSON using RFC 8785 JCS.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"jcs-canon/jcs"
	"jcs-canon/jcstoken"
)

const (
	exitSuccess  = 0
	exitInvalid  = 2
	exitInternal = 10
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		if err := writeLine(stderr, "usage: jcs-canon <canonicalize|verify> [options] [file|-]"); err != nil {
			return exitInternal
		}
		return exitInvalid
	}

	switch args[0] {
	case "canonicalize":
		return cmdCanonicalize(args[1:], stdin, stdout, stderr)
	case "verify":
		return cmdVerify(args[1:], stdin, stderr)
	default:
		if err := writef(stderr, "unknown command: %s\n", args[0]); err != nil {
			return exitInternal
		}
		if err := writeLine(stderr, "usage: jcs-canon <canonicalize|verify> [options] [file|-]"); err != nil {
			return exitInternal
		}
		return exitInvalid
	}
}

type flags struct {
	quiet bool
	help  bool
}

func parseFlags(args []string) (flags, []string, error) {
	var f flags
	var positional []string
	consumeAsPositional := false
	for _, arg := range args {
		if consumeAsPositional {
			positional = append(positional, arg)
			continue
		}

		switch arg {
		case "--quiet", "-q":
			f.quiet = true
		case "--help", "-h":
			f.help = true
		case "--":
			consumeAsPositional = true
		case "-":
			positional = append(positional, arg)
		default:
			if strings.HasPrefix(arg, "-") {
				return flags{}, nil, fmt.Errorf("unknown option: %s", arg)
			}
			positional = append(positional, arg)
		}
	}
	return f, positional, nil
}

func cmdCanonicalize(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fl, positional, err := parseFlags(args)
	if err != nil {
		return writeErrorAndReturn(stderr, exitInvalid, "error: %v\n", err)
	}

	if fl.help {
		if err := writeCanonicalizeHelp(stderr); err != nil {
			return exitInternal
		}
		return exitSuccess
	}

	if exitCode, ok := ensureSingleInput(positional, stderr); ok {
		return exitCode
	}

	input, err := readInput(positional, stdin, jcstoken.DefaultMaxInputSize)
	if err != nil {
		return writeErrorAndReturn(stderr, exitInvalid, "error: reading input: %v\n", err)
	}

	parsed, err := jcstoken.Parse(input)
	if err != nil {
		return writeErrorAndReturn(stderr, exitInvalid, "error: %v\n", err)
	}

	canonical, err := jcs.Serialize(parsed)
	if err != nil {
		return writeErrorAndReturn(stderr, exitInternal, "error: serialization failed: %v\n", err)
	}

	if _, err := stdout.Write(canonical); err != nil {
		return writeErrorAndReturn(stderr, exitInternal, "error: writing output: %v\n", err)
	}

	return exitSuccess
}

func cmdVerify(args []string, stdin io.Reader, stderr io.Writer) int {
	fl, positional, err := parseFlags(args)
	if err != nil {
		return writeErrorAndReturn(stderr, exitInvalid, "error: %v\n", err)
	}

	if fl.help {
		if err := writeVerifyHelp(stderr); err != nil {
			return exitInternal
		}
		return exitSuccess
	}

	if exitCode, ok := ensureSingleInput(positional, stderr); ok {
		return exitCode
	}

	input, err := readInput(positional, stdin, jcstoken.DefaultMaxInputSize)
	if err != nil {
		return writeErrorAndReturn(stderr, exitInvalid, "error: reading input: %v\n", err)
	}

	parsed, err := jcstoken.Parse(input)
	if err != nil {
		return writeErrorAndReturn(stderr, exitInvalid, "error: %v\n", err)
	}

	canonical, err := jcs.Serialize(parsed)
	if err != nil {
		return writeErrorAndReturn(stderr, exitInternal, "error: serialization failed: %v\n", err)
	}

	if !bytes.Equal(input, canonical) {
		return writeErrorAndReturn(stderr, exitInvalid, "error: input is not canonical\n")
	}

	if !fl.quiet {
		if err := writeLine(stderr, "ok"); err != nil {
			return exitInternal
		}
	}
	return exitSuccess
}

func readInput(positional []string, stdin io.Reader, maxInputSize int) ([]byte, error) {
	if len(positional) == 0 || positional[0] == "-" {
		return readBounded(stdin, maxInputSize)
	}

	f, err := os.Open(positional[0])
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", positional[0], err)
	}
	defer func() {
		_ = f.Close()
	}()

	data, err := readBounded(f, maxInputSize)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", positional[0], err)
	}
	return data, nil
}

func readBounded(r io.Reader, maxInputSize int) ([]byte, error) {
	lr := io.LimitReader(r, int64(maxInputSize)+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if len(data) > maxInputSize {
		return nil, fmt.Errorf("input exceeds maximum size %d bytes", maxInputSize)
	}
	return data, nil
}

func ensureSingleInput(positional []string, stderr io.Writer) (int, bool) {
	if len(positional) <= 1 {
		return 0, false
	}
	if err := writeLine(stderr, "error: multiple input files specified"); err != nil {
		return exitInternal, true
	}
	return exitInvalid, true
}

func writeErrorAndReturn(stderr io.Writer, code int, format string, args ...any) int {
	if err := writef(stderr, format, args...); err != nil {
		return exitInternal
	}
	return code
}

func writeCanonicalizeHelp(stderr io.Writer) error {
	if err := writeLine(stderr, "usage: jcs-canon canonicalize [--quiet] [file|-]"); err != nil {
		return err
	}
	if err := writeLine(stderr, "  Read JSON from file (or stdin), emit canonical bytes to stdout."); err != nil {
		return err
	}
	return writeLine(stderr, "  --quiet   Accepted for command symmetry; canonicalize is silent on success")
}

func writeVerifyHelp(stderr io.Writer) error {
	if err := writeLine(stderr, "usage: jcs-canon verify [--quiet] [file|-]"); err != nil {
		return err
	}
	if err := writeLine(stderr, "  Parse, canonicalize, and compare bytes to verify canonical form."); err != nil {
		return err
	}
	return writeLine(stderr, "  --quiet  Suppress success messages")
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
