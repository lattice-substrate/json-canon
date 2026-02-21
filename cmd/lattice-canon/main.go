// Command lattice-canon is the CLI for lattice-canon governed JSON canonicalization.
//
// Commands:
//
//	lattice-canon canonicalize [--gjcs1] [file|-]
//	    Read JSON from file (or stdin if no file or "-"), emit JCS canonical bytes to stdout.
//	    With --gjcs1, emit GJCS1 (append trailing LF).
//
//	lattice-canon verify [file|-]
//	    Verify that file (or stdin if "-") is valid GJCS1 and canonical.
//
// Exit codes:
//
//	0  success
//	2  invalid input or non-canonical governed bytes
//	10 internal error
//
// Flags:
//
//	--quiet    Suppress success messages (errors still go to stderr)
package main

import (
	"fmt"
	"io"
	"os"

	"lattice-canon/gjcs1"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: lattice-canon <canonicalize|verify> [options] [file]")
		return gjcs1.ExitInvalidInput
	}

	switch args[0] {
	case "canonicalize":
		return cmdCanonicalize(args[1:], stdin, stdout, stderr)
	case "verify":
		return cmdVerify(args[1:], stdin, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		fmt.Fprintln(stderr, "usage: lattice-canon <canonicalize|verify> [options] [file]")
		return gjcs1.ExitInvalidInput
	}
}

// parseFlags extracts known flags from args. Returns remaining positional args,
// and the parsed flag values.
type flags struct {
	gjcs1 bool
	quiet bool
}

func parseFlags(args []string) (flags, []string) {
	var f flags
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--gjcs1":
			f.gjcs1 = true
		case "--quiet", "-q":
			f.quiet = true
		default:
			positional = append(positional, arg)
		}
	}
	return f, positional
}

func readInput(positional []string, stdin io.Reader) ([]byte, error) {
	if len(positional) == 0 || positional[0] == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(positional[0])
}

func cmdCanonicalize(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fl, positional := parseFlags(args)

	for _, arg := range positional {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintln(stderr, "usage: lattice-canon canonicalize [--gjcs1] [--quiet] [file|-]")
			fmt.Fprintln(stderr, "  Read JSON from file (or stdin), emit canonical bytes to stdout.")
			fmt.Fprintln(stderr, "  --gjcs1   Emit GJCS1 envelope (append trailing LF)")
			fmt.Fprintln(stderr, "  --quiet   Suppress success messages")
			return gjcs1.ExitSuccess
		}
	}

	if len(positional) > 1 {
		fmt.Fprintln(stderr, "error: multiple input files specified")
		return gjcs1.ExitInvalidInput
	}

	input, err := readInput(positional, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading input: %v\n", err)
		return gjcs1.ExitInternal
	}

	canonical, err := gjcs1.Canonicalize(input)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return gjcs1.ExitInvalidInput
	}

	var output []byte
	if fl.gjcs1 {
		output = gjcs1.Envelope(canonical)
	} else {
		output = canonical
	}

	_, err = stdout.Write(output)
	if err != nil {
		fmt.Fprintf(stderr, "error: writing output: %v\n", err)
		return gjcs1.ExitInternal
	}

	_ = fl.quiet // canonicalize has no success message to suppress
	return gjcs1.ExitSuccess
}

func cmdVerify(args []string, stdin io.Reader, stderr io.Writer) int {
	fl, positional := parseFlags(args)

	for _, arg := range positional {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintln(stderr, "usage: lattice-canon verify [--quiet] [file|-]")
			fmt.Fprintln(stderr, "  Verify that file (or stdin) is valid GJCS1 and canonical.")
			fmt.Fprintln(stderr, "  --quiet  Suppress success messages")
			return gjcs1.ExitSuccess
		}
	}

	if len(positional) > 1 {
		fmt.Fprintln(stderr, "error: multiple input files specified")
		return gjcs1.ExitInvalidInput
	}

	input, err := readInput(positional, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading file: %v\n", err)
		return gjcs1.ExitInternal
	}

	if err := gjcs1.Verify(input); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return gjcs1.ExitInvalidInput
	}

	if !fl.quiet {
		fmt.Fprintln(stderr, "ok")
	}
	return gjcs1.ExitSuccess
}
