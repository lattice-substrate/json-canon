// Command jcs-gate runs the repository's required verification gates in order.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lattice-substrate/json-canon/internal/traceability"
)

type gateStep struct {
	label string
	args  []string
}

type commandRunner interface {
	Run(ctx context.Context, name string, args []string, stdout io.Writer, stderr io.Writer) error
}

type realRunner struct{}

var syncTraceabilityFunc = traceability.Sync

var requiredGateSteps = []gateStep{
	{label: "go vet", args: []string{"vet", "./..."}},
	{label: "golangci-lint", args: []string{"run", "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8", "run", "--config=golangci.yml"}},
	{label: "unit tests", args: []string{"test", "./...", "-count=1", "-timeout=20m"}},
	{label: "race tests", args: []string{"test", "./...", "-race", "-count=1", "-timeout=25m"}},
	{label: "conformance", args: []string{"test", "./conformance", "-count=1", "-timeout=10m", "-v"}},
	{label: "offline evidence gate", args: []string{"test", "./offline/conformance", "-run", "TestOfflineReplayEvidenceReleaseGate", "-count=1", "-v"}},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, realRunner{}))
}

//nolint:gocyclo,cyclop,gocognit // REQ:LINT-GATE-001 gate orchestration dispatch is intentionally explicit and linear.
func run(args []string, stdout, stderr io.Writer, runner commandRunner) int {
	if len(args) > 0 {
		return runSubcommand(args, stdout, stderr)
	}

	ctx := context.Background()
	for i, step := range requiredGateSteps {
		if err := writef(stdout, "[%d/%d] %s\n", i+1, len(requiredGateSteps), step.label); err != nil {
			return 1
		}
		if err := runner.Run(ctx, "go", step.args, stdout, stderr); err != nil {
			if writeErr := writef(stderr, "gate failed: %s: %v\n", step.label, err); writeErr != nil {
				return 1
			}
			return 1
		}
	}

	if err := writeLine(stdout, "all gates passed"); err != nil {
		return 1
	}
	return 0
}

func runSubcommand(args []string, stdout, stderr io.Writer) int {
	switch args[0] {
	case "--help", "-h":
		if err := writeUsage(stdout); err != nil {
			return 1
		}
		return 0
	case "sync-traceability":
		return runSyncTraceability(args[1:], stdout, stderr)
	default:
		if err := writef(stderr, "error: unknown argument %q\n", args[0]); err != nil {
			return 1
		}
		if err := writeUsage(stderr); err != nil {
			return 1
		}
		return 2
	}
}

func runSyncTraceability(args []string, stdout, stderr io.Writer) int {
	root, err := parseSyncTraceabilityArgs(args)
	if err != nil {
		if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		if writeErr := writeUsage(stderr); writeErr != nil {
			return 1
		}
		return 2
	}
	if err := syncTraceabilityFunc(root); err != nil {
		if writeErr := writef(stderr, "sync-traceability failed: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}
	if err := writef(stdout, "traceability artifacts synced under %s\n", root); err != nil {
		return 1
	}
	return 0
}

func parseSyncTraceabilityArgs(args []string) (string, error) {
	if len(args) == 0 {
		return ".", nil
	}
	if len(args) == 2 && args[0] == "--root" && strings.TrimSpace(args[1]) != "" {
		return filepath.Clean(args[1]), nil
	}
	return "", fmt.Errorf("sync-traceability accepts no arguments or --root <path>")
}

func (realRunner) Run(ctx context.Context, name string, args []string, stdout io.Writer, stderr io.Writer) error {
	// #nosec G204 -- command and args are fixed repository gate invocations.
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}

func writeUsage(w io.Writer) error {
	if err := writeLine(w, "usage: go run ./cmd/jcs-gate [--help] | sync-traceability [--root <path>]"); err != nil {
		return err
	}
	if err := writeLine(w, "runs: vet, golangci-lint, tests, race, conformance, offline evidence gate"); err != nil {
		return err
	}
	if err := writeLine(w, "offline gate uses current JCS_OFFLINE_* env vars (and may skip if unset)"); err != nil {
		return err
	}
	return writeLine(w, "sync-traceability regenerates matrix mirrors, symbol anchors, and nolint inventory from live source")
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
