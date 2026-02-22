package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
	"github.com/lattice-substrate/json-canon/offline/runtime/container"
	"github.com/lattice-substrate/json-canon/offline/runtime/executil"
	"github.com/lattice-substrate/json-canon/offline/runtime/libvirt"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		writeUsage(stdout)
		return 0
	}

	sub := args[0]
	flags, err := parseKV(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	switch sub {
	case "prepare":
		if err := cmdPrepare(flags, stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		return 0
	case "run":
		if err := cmdRun(flags, stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		return 0
	case "verify-evidence":
		if err := cmdVerifyEvidence(flags, stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		return 0
	case "report":
		if err := cmdReport(flags, stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		return 0
	default:
		fmt.Fprintf(stderr, "error: unknown subcommand %q\n", sub)
		writeUsage(stderr)
		return 2
	}
}

func cmdPrepare(flags map[string]string, stdout io.Writer) error {
	matrixPath := requireFlag(flags, "--matrix")
	profilePath := requireFlag(flags, "--profile")
	bundlePath := requireFlag(flags, "--bundle")
	binaryPath := requireFlag(flags, "--binary")
	if matrixPath == "" || profilePath == "" || bundlePath == "" || binaryPath == "" {
		return fmt.Errorf("prepare requires --matrix, --profile, --binary, --bundle")
	}
	if _, err := replay.LoadMatrix(matrixPath); err != nil {
		return err
	}
	if _, err := replay.LoadProfile(profilePath); err != nil {
		return err
	}
	manifest, err := replay.CreateBundle(replay.BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  binaryPath,
		MatrixPath:  matrixPath,
		ProfilePath: profilePath,
		VectorsGlob: "conformance/vectors/*.jsonl",
		Version:     "bundle.v1",
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "bundle: %s\n", bundlePath)
	fmt.Fprintf(stdout, "binary_sha256: %s\n", manifest.BinarySHA256)
	fmt.Fprintf(stdout, "vector_set_sha256: %s\n", manifest.VectorSetSHA256)
	return nil
}

func cmdRun(flags map[string]string, stdout io.Writer) error {
	matrixPath := requireFlag(flags, "--matrix")
	profilePath := requireFlag(flags, "--profile")
	bundlePath := requireFlag(flags, "--bundle")
	evidencePath := requireFlag(flags, "--evidence")
	if matrixPath == "" || profilePath == "" || bundlePath == "" || evidencePath == "" {
		return fmt.Errorf("run requires --matrix, --profile, --bundle, --evidence")
	}
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		return err
	}
	profile, err := replay.LoadProfile(profilePath)
	if err != nil {
		return err
	}
	manifest, bundleSHA, err := replay.VerifyBundle(bundlePath)
	if err != nil {
		return err
	}
	matrixSHA, err := fileSHA256(matrixPath)
	if err != nil {
		return err
	}
	profileSHA, err := fileSHA256(profilePath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	evidence, err := replay.RunMatrix(ctx, matrix, profile, adapterFactory(), replay.RunOptions{
		BundlePath:          bundlePath,
		BundleSHA256:        bundleSHA,
		ControlBinarySHA256: manifest.BinarySHA256,
		MatrixSHA256:        matrixSHA,
		ProfileSHA256:       profileSHA,
		Orchestrator:        "jcs-offline-replay",
	})
	if err != nil {
		return err
	}
	if err := replay.WriteEvidence(evidencePath, evidence); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "evidence: %s\n", evidencePath)
	fmt.Fprintf(stdout, "runs: %d\n", len(evidence.NodeReplays))
	fmt.Fprintf(stdout, "aggregate_canonical_sha256: %s\n", evidence.AggregateCanonical)
	return nil
}

func cmdVerifyEvidence(flags map[string]string, stdout io.Writer) error {
	matrixPath := requireFlag(flags, "--matrix")
	profilePath := requireFlag(flags, "--profile")
	evidencePath := requireFlag(flags, "--evidence")
	if matrixPath == "" || profilePath == "" || evidencePath == "" {
		return fmt.Errorf("verify-evidence requires --matrix, --profile, --evidence")
	}
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		return err
	}
	profile, err := replay.LoadProfile(profilePath)
	if err != nil {
		return err
	}
	evidence, err := replay.LoadEvidence(evidencePath)
	if err != nil {
		return err
	}
	if err := replay.ValidateEvidenceBundle(evidence, matrix, profile); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "ok")
	return nil
}

func cmdReport(flags map[string]string, stdout io.Writer) error {
	evidencePath := requireFlag(flags, "--evidence")
	if evidencePath == "" {
		return fmt.Errorf("report requires --evidence")
	}
	evidence, err := replay.LoadEvidence(evidencePath)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "schema: %s\n", evidence.SchemaVersion)
	fmt.Fprintf(stdout, "profile: %s\n", evidence.ProfileName)
	fmt.Fprintf(stdout, "architecture: %s\n", evidence.Architecture)
	fmt.Fprintf(stdout, "runs: %d\n", len(evidence.NodeReplays))
	fmt.Fprintf(stdout, "aggregate canonical: %s\n", evidence.AggregateCanonical)

	byNode := make(map[string]int)
	for _, r := range evidence.NodeReplays {
		byNode[r.NodeID]++
	}
	nodes := make([]string, 0, len(byNode))
	for id := range byNode {
		nodes = append(nodes, id)
	}
	sort.Strings(nodes)
	for _, id := range nodes {
		fmt.Fprintf(stdout, "node %s: %d replays\n", id, byNode[id])
	}
	return nil
}

func adapterFactory() replay.AdapterFactory {
	baseRunner := executil.OSRunner{}
	containerAdapter := container.NewAdapter(baseRunner)
	libvirtAdapter := libvirt.NewAdapter(baseRunner)

	return func(node replay.NodeSpec) (replay.NodeAdapter, error) {
		switch node.Mode {
		case replay.NodeModeContainer:
			if !strings.HasPrefix(node.Runner.Kind, "container") {
				return nil, fmt.Errorf("node %s mode=container requires runner.kind prefix container", node.ID)
			}
			return containerAdapter, nil
		case replay.NodeModeVM:
			if !strings.HasPrefix(node.Runner.Kind, "libvirt") && !strings.HasPrefix(node.Runner.Kind, "vm") {
				return nil, fmt.Errorf("node %s mode=vm requires runner.kind prefix libvirt or vm", node.ID)
			}
			return libvirtAdapter, nil
		default:
			return nil, fmt.Errorf("node %s unsupported mode %q", node.ID, node.Mode)
		}
	}
}

func parseKV(args []string) (map[string]string, error) {
	flags := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--help" || arg == "-h" {
			flags[arg] = "true"
			continue
		}
		if !strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("unexpected argument %q", arg)
		}
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			flags[parts[0]] = parts[1]
			continue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("flag %s requires value", arg)
		}
		flags[arg] = args[i+1]
		i++
	}
	return flags, nil
}

func requireFlag(flags map[string]string, name string) string {
	return strings.TrimSpace(flags[name])
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: jcs-offline-replay <prepare|run|verify-evidence|report> [flags]")
	fmt.Fprintln(w, "  prepare --matrix <path> --profile <path> --binary <path> --bundle <path>")
	fmt.Fprintln(w, "  run --matrix <path> --profile <path> --bundle <path> --evidence <path>")
	fmt.Fprintln(w, "  verify-evidence --matrix <path> --profile <path> --evidence <path>")
	fmt.Fprintln(w, "  report --evidence <path>")
}
