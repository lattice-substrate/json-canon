// Command jcs-offline-replay prepares, runs, and verifies offline replay evidence.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
	"github.com/lattice-substrate/json-canon/offline/runtime/container"
	"github.com/lattice-substrate/json-canon/offline/runtime/executil"
	"github.com/lattice-substrate/json-canon/offline/runtime/libvirt"
)

const boolTrue = "true"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		if err := writeUsage(stdout); err != nil {
			return 2
		}
		return 0
	}

	flags, err := parseKV(args[1:])
	if err != nil {
		return writeErrorLine(stderr, err)
	}

	code, subErr := dispatchSubcommand(args[0], flags, stdout, stderr)
	if subErr != nil {
		return writeErrorLine(stderr, subErr)
	}
	return code
}

//nolint:gocyclo,cyclop // REQ:OFFLINE-EVIDENCE-001 explicit subcommand branching keeps CLI behavior deterministic and auditable.
func dispatchSubcommand(sub string, flags map[string]string, stdout io.Writer, stderr io.Writer) (int, error) {
	if helpRequested(flags) {
		return 0, writeUsage(stdout)
	}
	switch sub {
	case "prepare":
		return 0, cmdPrepare(flags, stdout)
	case "run":
		return 0, cmdRun(flags, stdout)
	case "preflight":
		return 0, cmdPreflight(flags, stdout)
	case "audit-summary":
		return 0, cmdAuditSummary(flags, stdout)
	case "run-suite":
		return 0, cmdRunSuite(flags, stdout)
	case "cross-arch":
		return 0, cmdCrossArch(flags, stdout)
	case "verify-evidence":
		return 0, cmdVerifyEvidence(flags, stdout)
	case "report":
		return 0, cmdReport(flags, stdout)
	case "write-infra-manifest":
		return 0, cmdWriteInfraManifest(flags, stdout)
	case "inspect-matrix":
		return 0, cmdInspectMatrix(flags, stdout)
	default:
		if err := writef(stderr, "error: unknown subcommand %q\n", sub); err != nil {
			return 2, err
		}
		if err := writeUsage(stderr); err != nil {
			return 2, err
		}
		return 2, nil
	}
}

func helpRequested(flags map[string]string) bool {
	return strings.EqualFold(strings.TrimSpace(flags["--help"]), boolTrue) ||
		strings.EqualFold(strings.TrimSpace(flags["-h"]), boolTrue)
}

func cmdPrepare(flags map[string]string, stdout io.Writer) error {
	matrixPath, profilePath, bundlePath, binaryPath, err := requirePrepareFlags(flags)
	if err != nil {
		return err
	}
	matrix, loadErr := replay.LoadMatrix(matrixPath)
	if loadErr != nil {
		return fmt.Errorf("load matrix: %w", loadErr)
	}
	if _, loadErr := replay.LoadProfile(profilePath); loadErr != nil {
		return fmt.Errorf("load profile: %w", loadErr)
	}
	if err := validateExecutableArchitecture(binaryPath, matrix.Architecture, "control binary"); err != nil {
		return err
	}

	workerPath, cleanupWorker, err := resolveWorkerPath(flags, matrix.Architecture)
	if err != nil {
		return err
	}
	defer cleanupWorker()

	manifest, err := replay.CreateBundle(replay.BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  binaryPath,
		WorkerPath:  workerPath,
		MatrixPath:  matrixPath,
		ProfilePath: profilePath,
		VectorsGlob: "conformance/vectors/*.jsonl",
		Version:     "bundle.v1",
	})
	if err != nil {
		return fmt.Errorf("create bundle: %w", err)
	}
	return writePrepareSummary(stdout, bundlePath, manifest)
}

func requirePrepareFlags(flags map[string]string) (string, string, string, string, error) {
	matrixPath := requireFlag(flags, "--matrix")
	profilePath := requireFlag(flags, "--profile")
	bundlePath := requireFlag(flags, "--bundle")
	binaryPath := requireFlag(flags, "--binary")
	if matrixPath == "" || profilePath == "" || bundlePath == "" || binaryPath == "" {
		return "", "", "", "", fmt.Errorf("prepare requires --matrix, --profile, --binary, --bundle")
	}
	return matrixPath, profilePath, bundlePath, binaryPath, nil
}

func writePrepareSummary(stdout io.Writer, bundlePath string, manifest *replay.BundleManifest) error {
	if err := writef(stdout, "bundle: %s\n", bundlePath); err != nil {
		return err
	}
	if err := writef(stdout, "binary_sha256: %s\n", manifest.BinarySHA256); err != nil {
		return err
	}
	if err := writef(stdout, "worker_sha256: %s\n", manifest.WorkerSHA256); err != nil {
		return err
	}
	return writef(stdout, "vector_set_sha256: %s\n", manifest.VectorSetSHA256)
}

func cmdRun(flags map[string]string, stdout io.Writer) error {
	matrixPath, profilePath, bundlePath, evidencePath, err := requireRunFlags(flags)
	if err != nil {
		return err
	}
	matrix, profile, manifest, bundleSHA, matrixSHA, profileSHA, err := loadRunInputs(matrixPath, profilePath, bundlePath)
	if err != nil {
		return fmt.Errorf("load run inputs: %w", err)
	}
	runOpts, timeout, err := buildRunOptions(flags, bundlePath, manifest, bundleSHA, matrixSHA, profileSHA)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	evidence, err := replay.RunMatrix(ctx, matrix, profile, adapterFactory(), runOpts)
	if err != nil {
		return fmt.Errorf("run replay matrix: %w", err)
	}
	if err := replay.WriteEvidence(evidencePath, evidence); err != nil {
		return fmt.Errorf("write evidence: %w", err)
	}
	return writeRunSummary(stdout, evidencePath, evidence)
}

func requireRunFlags(flags map[string]string) (matrixPath, profilePath, bundlePath, evidencePath string, err error) {
	matrixPath = requireFlag(flags, "--matrix")
	profilePath = requireFlag(flags, "--profile")
	bundlePath = requireFlag(flags, "--bundle")
	evidencePath = requireFlag(flags, "--evidence")
	if matrixPath == "" || profilePath == "" || bundlePath == "" || evidencePath == "" {
		return "", "", "", "", fmt.Errorf("run requires --matrix, --profile, --bundle, --evidence")
	}
	return matrixPath, profilePath, bundlePath, evidencePath, nil
}

// buildRunOptions assembles replay.RunOptions and the run timeout from flags and pre-loaded digests.
func buildRunOptions(flags map[string]string, bundlePath string, manifest *replay.BundleManifest, bundleSHA, matrixSHA, profileSHA string) (replay.RunOptions, time.Duration, error) {
	timeout, err := parseTimeout(flags)
	if err != nil {
		return replay.RunOptions{}, 0, err
	}
	sourceGitCommit, sourceGitTag, err := resolveSourceIdentity(flags)
	if err != nil {
		return replay.RunOptions{}, 0, err
	}
	infraManifestSHA, infraRepoURL, infraRepoCommit, err := resolveInfraManifest(flags)
	if err != nil {
		return replay.RunOptions{}, 0, err
	}
	return replay.RunOptions{
		BundlePath:          bundlePath,
		BundleSHA256:        bundleSHA,
		ControlBinarySHA256: manifest.BinarySHA256,
		MatrixSHA256:        matrixSHA,
		ProfileSHA256:       profileSHA,
		SourceGitCommit:     sourceGitCommit,
		SourceGitTag:        sourceGitTag,
		Orchestrator:        "jcs-offline-replay",
		InfraManifestSHA256: infraManifestSHA,
		InfraRepoURL:        infraRepoURL,
		InfraRepoCommit:     infraRepoCommit,
	}, timeout, nil
}

// resolveInfraManifest loads and validates the infra manifest at --infra-manifest (if set)
// and returns its SHA-256, repo URL, and repo commit.
func resolveInfraManifest(flags map[string]string) (sha, repoURL, repoCommit string, err error) {
	path := requireFlag(flags, "--infra-manifest")
	if path == "" {
		return "", "", "", nil
	}
	im, loadErr := replay.LoadInfraManifest(path)
	if loadErr != nil {
		return "", "", "", fmt.Errorf("load infra manifest: %w", loadErr)
	}
	sha, err = fileSHA256(path)
	if err != nil {
		return "", "", "", fmt.Errorf("sha256 infra manifest: %w", err)
	}
	return sha, im.InfraRepoURL, im.InfraRepoCommit, nil
}

func resolveWorkerPath(flags map[string]string, targetArch string) (string, func(), error) {
	workerPath := requireFlag(flags, "--worker")
	if workerPath != "" {
		if err := validateExecutableArchitecture(workerPath, targetArch, "worker binary"); err != nil {
			return "", func() {}, err
		}
		return workerPath, func() {}, nil
	}
	workerPath, err := buildWorkerBinary(targetArch)
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() {
		if removeErr := os.Remove(workerPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			_ = removeErr
		}
	}
	return workerPath, cleanup, nil
}

func writeRunSummary(stdout io.Writer, evidencePath string, evidence *replay.EvidenceBundle) error {
	if err := writef(stdout, "evidence: %s\n", evidencePath); err != nil {
		return err
	}
	if err := writef(stdout, "runs: %d\n", len(evidence.NodeReplays)); err != nil {
		return err
	}
	return writef(stdout, "aggregate_canonical_sha256: %s\n", evidence.AggregateCanonical)
}

func loadRunInputs(matrixPath, profilePath, bundlePath string) (*replay.Matrix, *replay.Profile, *replay.BundleManifest, string, string, string, error) {
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		return nil, nil, nil, "", "", "", fmt.Errorf("load matrix: %w", err)
	}
	profile, err := replay.LoadProfile(profilePath)
	if err != nil {
		return nil, nil, nil, "", "", "", fmt.Errorf("load profile: %w", err)
	}
	manifest, bundleSHA, err := replay.VerifyBundle(bundlePath)
	if err != nil {
		return nil, nil, nil, "", "", "", fmt.Errorf("verify bundle: %w", err)
	}
	matrixSHA, err := fileSHA256(matrixPath)
	if err != nil {
		return nil, nil, nil, "", "", "", err
	}
	profileSHA, err := fileSHA256(profilePath)
	if err != nil {
		return nil, nil, nil, "", "", "", err
	}
	return matrix, profile, manifest, bundleSHA, matrixSHA, profileSHA, nil
}

func parseTimeout(flags map[string]string) (time.Duration, error) {
	timeout := 12 * time.Hour
	raw := strings.TrimSpace(flags["--timeout"])
	if raw == "" {
		return timeout, nil
	}
	parsed, parseErr := time.ParseDuration(raw)
	if parseErr != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid --timeout value %q", raw)
	}
	return parsed, nil
}

func cmdVerifyEvidence(flags map[string]string, stdout io.Writer) error {
	matrixPath := requireFlag(flags, "--matrix")
	profilePath := requireFlag(flags, "--profile")
	evidencePath := requireFlag(flags, "--evidence")
	if matrixPath == "" || profilePath == "" || evidencePath == "" {
		return fmt.Errorf("verify-evidence requires --matrix, --profile, --evidence")
	}

	bundlePath, controlBinaryPath := resolveVerifyPaths(flags, evidencePath)
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		return fmt.Errorf("load matrix: %w", err)
	}
	profile, err := replay.LoadProfile(profilePath)
	if err != nil {
		return fmt.Errorf("load profile: %w", err)
	}
	evidence, err := replay.LoadEvidence(evidencePath)
	if err != nil {
		return fmt.Errorf("load evidence: %w", err)
	}
	bundleSHA, controlBinarySHA, matrixSHA, profileSHA, err := loadVerificationDigests(bundlePath, controlBinaryPath, matrixPath, profilePath)
	if err != nil {
		return fmt.Errorf("load verification digests: %w", err)
	}
	expectedSourceCommit, expectedSourceTag := resolveExpectedSourceIdentity(flags)
	expectedInfraManifestSHA, expectedInfraRepoURL, expectedInfraRepoCommit, err := resolveExpectedInfraBinding(flags, evidence, profile)
	if err != nil {
		return err
	}

	if err := replay.ValidateEvidenceBundle(evidence, matrix, profile, replay.EvidenceValidationOptions{
		ExpectedBundleSHA256:        bundleSHA,
		ExpectedControlBinarySHA256: controlBinarySHA,
		ExpectedMatrixSHA256:        matrixSHA,
		ExpectedProfileSHA256:       profileSHA,
		ExpectedArchitecture:        matrix.Architecture,
		ExpectedSourceGitCommit:     expectedSourceCommit,
		ExpectedSourceGitTag:        expectedSourceTag,
		ExpectedInfraManifestSHA256: expectedInfraManifestSHA,
		ExpectedInfraRepoURL:        expectedInfraRepoURL,
		ExpectedInfraRepoCommit:     expectedInfraRepoCommit,
	}); err != nil {
		return fmt.Errorf("validate evidence bundle: %w", err)
	}
	return writeLine(stdout, "ok")
}

func resolveExpectedInfraBinding(flags map[string]string, evidence *replay.EvidenceBundle, profile *replay.Profile) (string, string, string, error) {
	manifestPath := requireFlag(flags, "--infra-manifest")
	if manifestPath == "" {
		if replayProfileRequiresInfraBinding(profile) {
			return "", "", "", fmt.Errorf("verify-evidence requires --infra-manifest for infra-substrate-binding profiles")
		}
		return "", "", "", nil
	}
	im, err := replay.LoadInfraManifest(manifestPath)
	if err != nil {
		return "", "", "", fmt.Errorf("load infra manifest: %w", err)
	}
	manifestSHA, err := fileSHA256(manifestPath)
	if err != nil {
		return "", "", "", fmt.Errorf("sha256 infra manifest: %w", err)
	}
	if evidence.SchemaVersion == replay.EvidenceSchemaVersionV2 {
		if evidence.InfraRepoURL != im.InfraRepoURL {
			return "", "", "", fmt.Errorf("evidence infra_repo_url %q does not match manifest infra_repo_url %q", evidence.InfraRepoURL, im.InfraRepoURL)
		}
		if evidence.InfraRepoCommit != im.InfraRepoCommit {
			return "", "", "", fmt.Errorf("evidence infra_repo_commit %q does not match manifest infra_repo_commit %q", evidence.InfraRepoCommit, im.InfraRepoCommit)
		}
	}
	return manifestSHA, im.InfraRepoURL, im.InfraRepoCommit, nil
}

func replayProfileRequiresInfraBinding(profile *replay.Profile) bool {
	if profile == nil {
		return false
	}
	for _, s := range profile.RequiredSuites {
		if s == "infra-substrate-binding" {
			return true
		}
	}
	return false
}

func resolveVerifyPaths(flags map[string]string, evidencePath string) (string, string) {
	bundlePath := requireFlag(flags, "--bundle")
	controlBinaryPath := requireFlag(flags, "--control-binary")
	if bundlePath == "" || controlBinaryPath == "" {
		defaultBundlePath, defaultControlPath := defaultEvidenceArtifactPaths(evidencePath)
		if bundlePath == "" {
			bundlePath = defaultBundlePath
		}
		if controlBinaryPath == "" {
			controlBinaryPath = defaultControlPath
		}
	}
	return bundlePath, controlBinaryPath
}

func loadVerificationDigests(bundlePath, controlBinaryPath, matrixPath, profilePath string) (string, string, string, string, error) {
	bundleSHA, err := fileSHA256(bundlePath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve bundle sha256: %w", err)
	}
	controlBinarySHA, err := fileSHA256(controlBinaryPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve control binary sha256: %w", err)
	}
	matrixSHA, err := fileSHA256(matrixPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve matrix sha256: %w", err)
	}
	profileSHA, err := fileSHA256(profilePath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve profile sha256: %w", err)
	}
	return bundleSHA, controlBinarySHA, matrixSHA, profileSHA, nil
}

func cmdReport(flags map[string]string, stdout io.Writer) error {
	evidencePath := requireFlag(flags, "--evidence")
	if evidencePath == "" {
		return fmt.Errorf("report requires --evidence")
	}
	evidence, err := replay.LoadEvidence(evidencePath)
	if err != nil {
		return fmt.Errorf("load evidence: %w", err)
	}
	if err := writeReportHeader(stdout, evidence); err != nil {
		return err
	}
	return writeReportNodeBreakdown(stdout, evidence)
}

func cmdWriteInfraManifest(flags map[string]string, stdout io.Writer) error {
	outputPath := requireFlag(flags, "--output")
	if outputPath == "" {
		return fmt.Errorf("write-infra-manifest requires --output")
	}
	manifest := &replay.InfraManifest{
		SchemaVersion:      replay.InfraManifestSchemaVersion,
		GeneratedAtUTC:     time.Now().UTC().Format(time.RFC3339Nano),
		InfraRepoURL:       requireFlag(flags, "--infra-repo-url"),
		InfraRepoCommit:    requireFlag(flags, "--infra-repo-commit"),
		ProviderEngine:     requireFlag(flags, "--provider-engine"),
		ProviderVersion:    requireFlag(flags, "--provider-version"),
		ProviderLockSHA256: requireFlag(flags, "--provider-lock-sha256"),
		Hosts: []replay.InfraManifestHost{
			{
				Role:             "x86_64",
				CloudProvider:    requireFlag(flags, "--cloud-provider"),
				Region:           requireFlag(flags, "--region"),
				InstanceType:     requireFlag(flags, "--x86-instance-type"),
				ImageID:          requireFlag(flags, "--x86-image-id"),
				DiscoveredCPU:    strings.TrimSpace(flags["--x86-discovered-cpu"]),
				DiscoveredKernel: strings.TrimSpace(flags["--x86-discovered-kernel"]),
			},
			{
				Role:             "arm64",
				CloudProvider:    requireFlag(flags, "--cloud-provider"),
				Region:           requireFlag(flags, "--region"),
				InstanceType:     requireFlag(flags, "--arm64-instance-type"),
				ImageID:          requireFlag(flags, "--arm64-image-id"),
				DiscoveredCPU:    strings.TrimSpace(flags["--arm64-discovered-cpu"]),
				DiscoveredKernel: strings.TrimSpace(flags["--arm64-discovered-kernel"]),
			},
		},
	}
	if err := replay.ValidateInfraManifest(manifest); err != nil {
		return fmt.Errorf("validate infra manifest: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal infra manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("write infra manifest: %w", err)
	}
	return writef(stdout, "infra-manifest: %s\n", outputPath)
}

func writeReportHeader(stdout io.Writer, evidence *replay.EvidenceBundle) error {
	if err := writef(stdout, "schema: %s\n", evidence.SchemaVersion); err != nil {
		return err
	}
	if err := writef(stdout, "profile: %s\n", evidence.ProfileName); err != nil {
		return err
	}
	if err := writef(stdout, "architecture: %s\n", evidence.Architecture); err != nil {
		return err
	}
	if err := writef(stdout, "source git commit: %s\n", evidence.SourceGitCommit); err != nil {
		return err
	}
	if err := writef(stdout, "source git tag: %s\n", evidence.SourceGitTag); err != nil {
		return err
	}
	if err := writef(stdout, "runs: %d\n", len(evidence.NodeReplays)); err != nil {
		return err
	}
	return writef(stdout, "aggregate canonical: %s\n", evidence.AggregateCanonical)
}

func writeReportNodeBreakdown(stdout io.Writer, evidence *replay.EvidenceBundle) error {
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
		if err := writef(stdout, "node %s: %d replays\n", id, byNode[id]); err != nil {
			return err
		}
	}
	return nil
}

func cmdInspectMatrix(flags map[string]string, stdout io.Writer) error {
	matrixPath := requireFlag(flags, "--matrix")
	if matrixPath == "" {
		return fmt.Errorf("inspect-matrix requires --matrix")
	}
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		return fmt.Errorf("load matrix: %w", err)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(matrix); err != nil {
		return fmt.Errorf("encode matrix: %w", err)
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
	boolFlags := map[string]struct{}{
		"--local-no-rocky":        {},
		"--skip-preflight":        {},
		"--skip-release-gate":     {},
		"--run-official-vectors":  {},
		"--run-official-es6-100m": {},
		"--strict":                {},
		"--no-strict":             {},
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--help" || arg == "-h" {
			flags[arg] = boolTrue
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
		if _, ok := boolFlags[arg]; ok {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				flags[arg] = boolTrue
				continue
			}
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

func defaultEvidenceArtifactPaths(evidencePath string) (string, string) {
	base := filepath.Dir(evidencePath)
	return filepath.Join(base, "offline-bundle.tgz"), filepath.Join(base, "bin", "jcs-canon")
}

func resolveSourceIdentity(flags map[string]string) (string, string, error) {
	sourceGitCommit := requireFlag(flags, "--source-git-commit")
	if sourceGitCommit == "" {
		sourceGitCommit = lookupEnvTrimmed("JCS_OFFLINE_SOURCE_GIT_COMMIT")
	}
	if sourceGitCommit == "" {
		out, err := runCommandCapture("git", "rev-parse", "HEAD")
		if err != nil {
			return "", "", fmt.Errorf("resolve source git commit (set --source-git-commit or JCS_OFFLINE_SOURCE_GIT_COMMIT): %w (%s)", err, out)
		}
		sourceGitCommit = strings.TrimSpace(out)
	}

	sourceGitTag := requireFlag(flags, "--source-git-tag")
	if sourceGitTag == "" {
		sourceGitTag = lookupEnvTrimmed("JCS_OFFLINE_SOURCE_GIT_TAG")
	}
	if sourceGitTag == "" {
		out, err := runCommandCapture("git", "describe", "--tags", "--exact-match")
		if err == nil {
			sourceGitTag = strings.TrimSpace(out)
		}
	}
	if sourceGitTag == "" {
		sourceGitTag = "untagged"
	}
	return sourceGitCommit, sourceGitTag, nil
}

func resolveExpectedSourceIdentity(flags map[string]string) (string, string) {
	expectedCommit := requireFlag(flags, "--source-git-commit")
	if expectedCommit == "" {
		expectedCommit = lookupEnvTrimmed("JCS_OFFLINE_EXPECTED_GIT_COMMIT")
	}
	expectedTag := requireFlag(flags, "--source-git-tag")
	if expectedTag == "" {
		expectedTag = lookupEnvTrimmed("JCS_OFFLINE_EXPECTED_GIT_TAG")
	}
	return strings.TrimSpace(expectedCommit), strings.TrimSpace(expectedTag)
}

func fileSHA256(path string) (string, error) {
	// #nosec G304 -- offline verification intentionally reads user-specified artifact paths.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func writeUsage(w io.Writer) error {
	if err := writeLine(w, "usage: jcs-offline-replay <prepare|run|preflight|audit-summary|run-suite|cross-arch|verify-evidence|report|inspect-matrix> [flags]"); err != nil {
		return err
	}
	if err := writeLine(w, "  prepare --matrix <path> --profile <path> --binary <path> --bundle <path> [--worker <path>]"); err != nil {
		return err
	}
	if err := writeLine(w, "  run --matrix <path> --profile <path> --bundle <path> --evidence <path> [--infra-manifest <path>] [--timeout 12h] [--source-git-commit <sha>] [--source-git-tag <tag>]"); err != nil {
		return err
	}
	if err := writeLine(w, "  preflight --matrix <path> [--strict] [--no-strict]"); err != nil {
		return err
	}
	if err := writeLine(w, "  audit-summary --matrix <path> --profile <path> --evidence <path> [--output-dir <path>]"); err != nil {
		return err
	}
	if err := writeLine(w, "  run-suite --matrix <path> --profile <path> [--infra-manifest <path>] [--output-dir <path>] [--timeout 12h] [--version v0.0.0-dev] [--skip-preflight] [--skip-release-gate]"); err != nil {
		return err
	}
	if err := writeLine(w, "  cross-arch [--x86-matrix <path>] [--x86-profile <path>] [--arm64-matrix <path>] [--arm64-profile <path>] [--infra-manifest <path>] [--local-no-rocky] [--output-dir <path>] [--timeout 12h] [--run-official-vectors] [--run-official-es6-100m]"); err != nil {
		return err
	}
	if err := writeLine(w, "  verify-evidence --matrix <path> --profile <path> --evidence <path> [--bundle <path>] [--control-binary <path>] [--infra-manifest <path>] [--source-git-commit <sha>] [--source-git-tag <tag>]"); err != nil {
		return err
	}
	if err := writeLine(w, "  report --evidence <path>"); err != nil {
		return err
	}
	if err := writeLine(w, "  write-infra-manifest --output <path> --infra-repo-url <url> --infra-repo-commit <sha> --provider-engine <name> --provider-version <ver> --provider-lock-sha256 <sha> --cloud-provider <name> --region <name> --x86-instance-type <type> --x86-image-id <id> --arm64-instance-type <type> --arm64-image-id <id>"); err != nil {
		return err
	}
	return writeLine(w, "  inspect-matrix --matrix <path>")
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

func writeErrorLine(stderr io.Writer, err error) int {
	if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
		return 2
	}
	return 2
}

func buildWorkerBinary(targetArch string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "jcs-offline-worker-*")
	if err != nil {
		return "", fmt.Errorf("create worker temp dir: %w", err)
	}
	out := filepath.Join(tmpDir, "jcs-offline-worker")
	goArch, err := goArchForMatrixArch(targetArch)
	if err != nil {
		return "", err
	}
	// #nosec G204 -- fixed go tool invocation with controlled arguments.
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", out, "./cmd/jcs-offline-worker")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+goArch)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build worker binary: %w: %s", err, strings.TrimSpace(buf.String()))
	}
	return out, nil
}

func goArchForMatrixArch(matrixArch string) (string, error) {
	switch strings.TrimSpace(matrixArch) {
	case "x86_64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported matrix architecture %q", matrixArch)
	}
}

func validateExecutableArchitecture(path string, matrixArch string, label string) error {
	wantMachine, err := elfMachineForMatrixArch(matrixArch)
	if err != nil {
		return err
	}
	f, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("%s %s is not a readable ELF binary: %w", label, path, err)
	}
	defer func() {
		_ = f.Close()
	}()
	if f.FileHeader.Class != elf.ELFCLASS64 {
		return fmt.Errorf("%s %s must be a 64-bit Linux ELF binary", label, path)
	}
	if f.FileHeader.Machine != wantMachine {
		return fmt.Errorf("%s %s architecture mismatch: got=%s want=%s", label, path, f.FileHeader.Machine, wantMachine)
	}
	return nil
}

func elfMachineForMatrixArch(matrixArch string) (elf.Machine, error) {
	switch strings.TrimSpace(matrixArch) {
	case "x86_64":
		return elf.EM_X86_64, nil
	case "arm64":
		return elf.EM_AARCH64, nil
	default:
		return 0, fmt.Errorf("unsupported matrix architecture %q", matrixArch)
	}
}
