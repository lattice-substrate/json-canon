package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const (
	defaultMatrixPath       = "offline/matrix.yaml"
	defaultProfilePath      = "offline/profiles/maximal.yaml"
	defaultARMMatrixPath    = "offline/matrix.arm64.yaml"
	defaultARMProfilePath   = "offline/profiles/maximal.arm64.yaml"
	defaultRunTimeout       = 12 * time.Hour
	defaultBuildVersion     = "v0.0.0-dev"
	defaultCrossArchTimeout = 12 * time.Hour
	dirPerm                 = 0o750
	filePerm                = 0o600
	resultPass              = "PASS"
	resultFail              = "FAIL"
)

type runSuiteOptions struct {
	MatrixPath      string
	ProfilePath     string
	OutputDir       string
	Timeout         time.Duration
	Version         string
	SkipPreflight   bool
	SkipReleaseGate bool
}

type runSuiteArtifacts struct {
	OutputDir      string
	BundlePath     string
	EvidencePath   string
	ControllerPath string
	CanonicalPath  string
	MatrixPath     string
	ProfilePath    string
	MatrixAbsPath  string
	ProfileAbsPath string
}

type auditSummary struct {
	GeneratedAtUTC      string              `json:"generated_at_utc"`
	MatrixPath          string              `json:"matrix_path"`
	ProfilePath         string              `json:"profile_path"`
	EvidencePath        string              `json:"evidence_path"`
	SchemaVersion       string              `json:"schema_version"`
	ProfileName         string              `json:"profile_name"`
	Architecture        string              `json:"architecture"`
	HardReleaseGate     bool                `json:"hard_release_gate"`
	NodeCount           int                 `json:"node_count"`
	RunCount            int                 `json:"run_count"`
	RequiredSuites      []string            `json:"required_suites"`
	Aggregate           map[string]string   `json:"aggregate"`
	DigestSets          map[string][]string `json:"digest_sets"`
	NodeReplayCounts    map[string]int      `json:"node_replay_counts"`
	NodeReplayCaseCount map[string][]int    `json:"node_replay_case_counts"`
	Parity              map[string]bool     `json:"parity"`
	Result              string              `json:"result"`
}

type crossArchCheck struct {
	Field string `json:"field"`
	Label string `json:"label"`
	X86   string `json:"x86"`
	Arm64 string `json:"arm64"`
	Match bool   `json:"match"`
}

type crossArchReport struct {
	GeneratedAtUTC string           `json:"generated_at_utc"`
	X86Evidence    string           `json:"x86_evidence"`
	Arm64Evidence  string           `json:"arm64_evidence"`
	Result         string           `json:"result"`
	Checks         []crossArchCheck `json:"checks"`
}

type preflightReporter struct {
	w        io.Writer
	failures int
	warnings int
}

type vmLane struct {
	NodeID     string
	Domain     string
	Snapshot   string
	SSHTarget  string
	SSHOptions string
}

func cmdRunSuite(flags map[string]string, stdout io.Writer) error {
	opts, err := parseRunSuiteOptions(flags)
	if err != nil {
		return err
	}
	_, err = runSuite(opts, stdout)
	return err
}

//nolint:gocognit,gocyclo,cyclop,funlen // REQ:OFFLINE-LOCAL-001 cross-arch workflow intentionally keeps each gate explicit for auditability.
func cmdCrossArch(flags map[string]string, stdout io.Writer) error {
	skipPreflight, err := parseBoolFlag(flags, "--skip-preflight")
	if err != nil {
		return err
	}
	skipReleaseGate, err := parseBoolFlag(flags, "--skip-release-gate")
	if err != nil {
		return err
	}
	useLocalNoRocky, err := parseBoolFlag(flags, "--local-no-rocky")
	if err != nil {
		return err
	}
	runOfficialVectors, err := parseBoolFlag(flags, "--run-official-vectors")
	if err != nil {
		return err
	}
	runOfficialES6100M, err := parseBoolFlag(flags, "--run-official-es6-100m")
	if err != nil {
		return err
	}

	x86Matrix := defaultString(flags, "--x86-matrix", defaultMatrixPath)
	x86Profile := defaultString(flags, "--x86-profile", defaultProfilePath)
	armMatrix := defaultString(flags, "--arm64-matrix", defaultARMMatrixPath)
	armProfile := defaultString(flags, "--arm64-profile", defaultARMProfilePath)
	if useLocalNoRocky {
		x86Matrix = "offline/matrix.local-no-rocky.yaml"
		armMatrix = "offline/matrix.local-no-rocky.arm64.yaml"
	}

	timeout := defaultCrossArchTimeout
	if raw := strings.TrimSpace(flags["--timeout"]); raw != "" {
		parsed, parseErr := time.ParseDuration(raw)
		if parseErr != nil || parsed <= 0 {
			return fmt.Errorf("invalid --timeout value %q", raw)
		}
		timeout = parsed
	}

	version := defaultString(flags, "--version", defaultBuildVersion)
	outDir := strings.TrimSpace(flags["--output-dir"])
	if outDir == "" {
		outDir = filepath.Join("offline", "runs", "cross-arch-"+utcStamp())
	}
	outDirAbs, err := filepath.Abs(outDir)
	if err != nil {
		return fmt.Errorf("resolve output dir: %w", err)
	}
	if err = os.MkdirAll(outDirAbs, dirPerm); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if writeErr := writef(stdout, "[cross-arch] running x86_64 harness\n"); writeErr != nil {
		return writeErr
	}
	x86Run, err := runSuite(runSuiteOptions{
		MatrixPath:      x86Matrix,
		ProfilePath:     x86Profile,
		OutputDir:       filepath.Join(outDirAbs, "x86_64"),
		Timeout:         timeout,
		Version:         version,
		SkipPreflight:   skipPreflight,
		SkipReleaseGate: skipReleaseGate,
	}, stdout)
	if err != nil {
		return err
	}

	if writeErr := writef(stdout, "[cross-arch] running arm64 harness\n"); writeErr != nil {
		return writeErr
	}
	armRun, err := runSuite(runSuiteOptions{
		MatrixPath:      armMatrix,
		ProfilePath:     armProfile,
		OutputDir:       filepath.Join(outDirAbs, "arm64"),
		Timeout:         timeout,
		Version:         version,
		SkipPreflight:   skipPreflight,
		SkipReleaseGate: skipReleaseGate,
	}, stdout)
	if err != nil {
		return err
	}

	compareJSON := filepath.Join(outDirAbs, "cross-arch-compare.json")
	compareMD := filepath.Join(outDirAbs, "cross-arch-compare.md")
	report, err := compareCrossArchEvidence(x86Run.EvidencePath, armRun.EvidencePath, compareJSON, compareMD)
	if err != nil {
		return err
	}

	if runOfficialVectors {
		if err := runOfficialVectorGates(outDirAbs, stdout); err != nil {
			return err
		}
	}
	if runOfficialES6100M {
		if err := runOfficialES6100MGate(outDirAbs, stdout); err != nil {
			return err
		}
	}

	if writeErr := writef(stdout, "[cross-arch] compare report: %s\n", compareMD); writeErr != nil {
		return writeErr
	}
	if writeErr := writef(stdout, "[cross-arch] compare json: %s\n", compareJSON); writeErr != nil {
		return writeErr
	}
	if writeErr := writef(stdout, "[cross-arch] RESULT=%s\n", report.Result); writeErr != nil {
		return writeErr
	}
	return nil
}

func cmdPreflight(flags map[string]string, stdout io.Writer) error {
	matrixPath := defaultString(flags, "--matrix", defaultMatrixPath)
	strict := true
	if raw := strings.TrimSpace(flags["--strict"]); raw != "" {
		parsed, err := parseBoolToken(raw)
		if err != nil {
			return fmt.Errorf("parse --strict: %w", err)
		}
		strict = parsed
	}
	if raw := strings.TrimSpace(flags["--no-strict"]); raw != "" {
		parsed, err := parseBoolToken(raw)
		if err != nil {
			return fmt.Errorf("parse --no-strict: %w", err)
		}
		if parsed {
			strict = false
		}
	}
	return runPreflight(matrixPath, strict, stdout)
}

//nolint:gocyclo,cyclop // REQ:OFFLINE-EVIDENCE-001 audit emission keeps output/error checks explicit for deterministic artifacts.
func cmdAuditSummary(flags map[string]string, stdout io.Writer) error {
	matrixPath := strings.TrimSpace(flags["--matrix"])
	profilePath := strings.TrimSpace(flags["--profile"])
	evidencePath := strings.TrimSpace(flags["--evidence"])
	if matrixPath == "" || profilePath == "" || evidencePath == "" {
		return fmt.Errorf("audit-summary requires --matrix, --profile, --evidence")
	}
	outputDir := strings.TrimSpace(flags["--output-dir"])
	controllerReport, summary, markdown, err := buildAuditOutputs(matrixPath, profilePath, evidencePath)
	if err != nil {
		return err
	}

	if writeErr := writef(stdout, "%s", markdown); writeErr != nil {
		return writeErr
	}
	if outputDir == "" {
		_ = summary
		_ = controllerReport
		return nil
	}
	if err = os.MkdirAll(outputDir, dirPerm); err != nil {
		return fmt.Errorf("create audit output dir: %w", err)
	}
	jsonPath := filepath.Join(outputDir, "audit-summary.json")
	mdPath := filepath.Join(outputDir, "audit-summary.md")
	reportPath := filepath.Join(outputDir, "controller-report.txt")

	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal audit summary: %w", err)
	}
	encoded = append(encoded, '\n')
	if err = os.WriteFile(jsonPath, encoded, filePerm); err != nil {
		return fmt.Errorf("write audit summary json: %w", err)
	}
	if err = os.WriteFile(mdPath, []byte(markdown), filePerm); err != nil {
		return fmt.Errorf("write audit summary markdown: %w", err)
	}
	if err = os.WriteFile(reportPath, []byte(controllerReport), filePerm); err != nil {
		return fmt.Errorf("write controller report: %w", err)
	}
	if writeErr := writef(stdout, "[audit] wrote: %s\n", jsonPath); writeErr != nil {
		return writeErr
	}
	if writeErr := writef(stdout, "[audit] wrote: %s\n", mdPath); writeErr != nil {
		return writeErr
	}
	return writef(stdout, "[audit] wrote: %s\n", reportPath)
}

func parseRunSuiteOptions(flags map[string]string) (runSuiteOptions, error) {
	skipPreflight, err := parseBoolFlag(flags, "--skip-preflight")
	if err != nil {
		return runSuiteOptions{}, err
	}
	skipReleaseGate, err := parseBoolFlag(flags, "--skip-release-gate")
	if err != nil {
		return runSuiteOptions{}, err
	}
	timeout := defaultRunTimeout
	if raw := strings.TrimSpace(flags["--timeout"]); raw != "" {
		parsed, parseErr := time.ParseDuration(raw)
		if parseErr != nil || parsed <= 0 {
			return runSuiteOptions{}, fmt.Errorf("invalid --timeout value %q", raw)
		}
		timeout = parsed
	}
	outDir := strings.TrimSpace(flags["--output-dir"])
	if outDir == "" {
		outDir = filepath.Join("offline", "runs", utcStamp())
	}
	return runSuiteOptions{
		MatrixPath:      defaultString(flags, "--matrix", defaultMatrixPath),
		ProfilePath:     defaultString(flags, "--profile", defaultProfilePath),
		OutputDir:       outDir,
		Timeout:         timeout,
		Version:         defaultString(flags, "--version", defaultBuildVersion),
		SkipPreflight:   skipPreflight,
		SkipReleaseGate: skipReleaseGate,
	}, nil
}

//nolint:gocognit,gocyclo,cyclop,funlen // REQ:OFFLINE-EVIDENCE-001 suite orchestration keeps all replay gates explicit for operator audits.
func runSuite(opts runSuiteOptions, stdout io.Writer) (*runSuiteArtifacts, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultRunTimeout
	}
	if strings.TrimSpace(opts.Version) == "" {
		opts.Version = defaultBuildVersion
	}
	outDirAbs, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output dir: %w", err)
	}
	if err = os.MkdirAll(filepath.Join(outDirAbs, "bin"), dirPerm); err != nil {
		return nil, fmt.Errorf("create bin dir: %w", err)
	}
	if err = os.MkdirAll(filepath.Join(outDirAbs, "logs"), dirPerm); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}
	if err = os.MkdirAll(filepath.Join(outDirAbs, "audit"), dirPerm); err != nil {
		return nil, fmt.Errorf("create audit dir: %w", err)
	}

	matrixAbs, err := filepath.Abs(opts.MatrixPath)
	if err != nil {
		return nil, fmt.Errorf("resolve matrix path: %w", err)
	}
	profileAbs, err := filepath.Abs(opts.ProfilePath)
	if err != nil {
		return nil, fmt.Errorf("resolve profile path: %w", err)
	}

	canonBin := filepath.Join(outDirAbs, "bin", "jcs-canon")
	controllerBin := filepath.Join(outDirAbs, "bin", "jcs-offline-replay")
	bundlePath := filepath.Join(outDirAbs, "offline-bundle.tgz")
	evidencePath := filepath.Join(outDirAbs, "offline-evidence.json")

	if writeErr := writef(stdout, "[run] matrix: %s\n", opts.MatrixPath); writeErr != nil {
		return nil, writeErr
	}
	if writeErr := writef(stdout, "[run] profile: %s\n", opts.ProfilePath); writeErr != nil {
		return nil, writeErr
	}
	if writeErr := writef(stdout, "[run] output: %s\n", outDirAbs); writeErr != nil {
		return nil, writeErr
	}

	if buildErr := buildCanonicalizer(canonBin, opts.Version, filepath.Join(outDirAbs, "logs", "build-jcs-canon.log"), stdout); buildErr != nil {
		return nil, buildErr
	}
	if buildErr := buildController(controllerBin, filepath.Join(outDirAbs, "logs", "build-jcs-offline-replay.log"), stdout); buildErr != nil {
		return nil, buildErr
	}

	if !opts.SkipPreflight {
		if stepErr := runLoggedStep(filepath.Join(outDirAbs, "logs", "preflight.log"), stdout, func(w io.Writer) error {
			return runPreflight(opts.MatrixPath, true, w)
		}); stepErr != nil {
			return nil, stepErr
		}
	} else if stepErr := runLoggedStep(filepath.Join(outDirAbs, "logs", "preflight.log"), stdout, func(w io.Writer) error {
		return writeLine(w, "[run] preflight skipped")
	}); stepErr != nil {
		return nil, stepErr
	}

	prepareFlags := map[string]string{
		"--matrix":  opts.MatrixPath,
		"--profile": opts.ProfilePath,
		"--binary":  canonBin,
		"--bundle":  bundlePath,
	}
	if stepErr := runLoggedStep(filepath.Join(outDirAbs, "logs", "prepare.log"), stdout, func(w io.Writer) error {
		return cmdPrepare(prepareFlags, w)
	}); stepErr != nil {
		return nil, stepErr
	}

	runFlags := map[string]string{
		"--matrix":   opts.MatrixPath,
		"--profile":  opts.ProfilePath,
		"--bundle":   bundlePath,
		"--evidence": evidencePath,
		"--timeout":  opts.Timeout.String(),
	}
	if stepErr := runLoggedStep(filepath.Join(outDirAbs, "logs", "run.log"), stdout, func(w io.Writer) error {
		return cmdRun(runFlags, w)
	}); stepErr != nil {
		return nil, stepErr
	}

	verifyFlags := map[string]string{
		"--matrix":   opts.MatrixPath,
		"--profile":  opts.ProfilePath,
		"--evidence": evidencePath,
	}
	if stepErr := runLoggedStep(filepath.Join(outDirAbs, "logs", "verify-evidence.log"), stdout, func(w io.Writer) error {
		return cmdVerifyEvidence(verifyFlags, w)
	}); stepErr != nil {
		return nil, stepErr
	}

	reportFlags := map[string]string{"--evidence": evidencePath}
	controllerReport, err := runLoggedStepCapture(filepath.Join(outDirAbs, "logs", "report.log"), stdout, func(w io.Writer) error {
		return cmdReport(reportFlags, w)
	})
	if err != nil {
		return nil, err
	}

	if stepErr := runLoggedStep(filepath.Join(outDirAbs, "logs", "audit.log"), stdout, func(w io.Writer) error {
		_, _, _, auditErr := writeAuditOutputs(opts.MatrixPath, opts.ProfilePath, evidencePath, filepath.Join(outDirAbs, "audit"), controllerReport, w)
		return auditErr
	}); stepErr != nil {
		return nil, stepErr
	}

	if !opts.SkipReleaseGate {
		if gateErr := runOfflineReleaseGate(matrixAbs, profileAbs, evidencePath, filepath.Join(outDirAbs, "logs", "release-gate.log"), stdout); gateErr != nil {
			return nil, gateErr
		}
	} else if stepErr := runLoggedStep(filepath.Join(outDirAbs, "logs", "release-gate.log"), stdout, func(w io.Writer) error {
		return writeLine(w, "[run] release gate skipped by flag")
	}); stepErr != nil {
		return nil, stepErr
	}

	if checksumErr := writeChecksumFile(filepath.Join(outDirAbs, "audit", "bundle.sha256"), bundlePath); checksumErr != nil {
		return nil, checksumErr
	}
	if checksumErr := writeChecksumFile(filepath.Join(outDirAbs, "audit", "evidence.sha256"), evidencePath); checksumErr != nil {
		return nil, checksumErr
	}

	if indexErr := writeRunIndex(filepath.Join(outDirAbs, "RUN_INDEX.txt"), runSuiteArtifacts{
		OutputDir:      outDirAbs,
		BundlePath:     bundlePath,
		EvidencePath:   evidencePath,
		ControllerPath: controllerBin,
		CanonicalPath:  canonBin,
		MatrixPath:     opts.MatrixPath,
		ProfilePath:    opts.ProfilePath,
	}); indexErr != nil {
		return nil, indexErr
	}

	if writeErr := writeLine(stdout, "[run] RESULT=PASS"); writeErr != nil {
		return nil, writeErr
	}
	if writeErr := writef(stdout, "[run] inspect: %s\n", filepath.Join(outDirAbs, "RUN_INDEX.txt")); writeErr != nil {
		return nil, writeErr
	}
	return &runSuiteArtifacts{
		OutputDir:      outDirAbs,
		BundlePath:     bundlePath,
		EvidencePath:   evidencePath,
		ControllerPath: controllerBin,
		CanonicalPath:  canonBin,
		MatrixPath:     opts.MatrixPath,
		ProfilePath:    opts.ProfilePath,
		MatrixAbsPath:  matrixAbs,
		ProfileAbsPath: profileAbs,
	}, nil
}

func buildCanonicalizer(outputPath, version, logPath string, stdout io.Writer) error {
	if err := writeLine(stdout, "[run] build jcs-canon"); err != nil {
		return err
	}
	return runGoCommandLogged(logPath, stdout, map[string]string{"CGO_ENABLED": "0"},
		"build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid= -X main.version="+version, "-o", outputPath, "./cmd/jcs-canon")
}

func buildController(outputPath, logPath string, stdout io.Writer) error {
	if err := writeLine(stdout, "[run] build jcs-offline-replay"); err != nil {
		return err
	}
	return runGoCommandLogged(logPath, stdout, map[string]string{"CGO_ENABLED": "0"},
		"build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", outputPath, "./cmd/jcs-offline-replay")
}

func runOfflineReleaseGate(matrixPath, profilePath, evidencePath, logPath string, stdout io.Writer) error {
	if err := writeLine(stdout, "[run] release gate test"); err != nil {
		return err
	}
	return runGoCommandLogged(logPath, stdout, map[string]string{
		"JCS_OFFLINE_EVIDENCE": evidencePath,
		"JCS_OFFLINE_MATRIX":   matrixPath,
		"JCS_OFFLINE_PROFILE":  profilePath,
	}, "test", "./offline/conformance", "-run", "TestOfflineReplayEvidenceReleaseGate", "-count=1", "-v")
}

func runOfficialVectorGates(outputDir string, stdout io.Writer) error {
	if err := writeLine(stdout, "[cross-arch] run official vector gates (10K set)"); err != nil {
		return err
	}
	logPath := filepath.Join(outputDir, "logs", "official-vectors.log")
	return runGoCommandLogged(logPath, stdout, nil,
		"test", "./conformance", "-run", "TestOfficialCyberphoneCanonicalPairs|TestOfficialRFC8785Vectors|TestOfficialES6CorpusChecksums10K", "-count=1", "-timeout=30m")
}

func runOfficialES6100MGate(outputDir string, stdout io.Writer) error {
	if err := writeLine(stdout, "[cross-arch] run official ES6 100M gate"); err != nil {
		return err
	}
	logPath := filepath.Join(outputDir, "logs", "official-es6-100m.log")
	return runGoCommandLogged(logPath, stdout, map[string]string{"JCS_OFFICIAL_ES6_ENABLE_100M": "1"},
		"test", "./conformance", "-run", "TestOfficialES6CorpusChecksums100M", "-count=1", "-timeout=6h")
}

//nolint:gocyclo,cyclop // REQ:OFFLINE-LOCAL-001 preflight keeps per-dependency diagnostics explicit for offline operators.
func runPreflight(matrixPath string, strict bool, out io.Writer) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	if writeErr := writef(out, "[preflight] repo: %s\n", cwd); writeErr != nil {
		return writeErr
	}
	if writeErr := writef(out, "[preflight] matrix: %s\n", matrixPath); writeErr != nil {
		return writeErr
	}
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		return fmt.Errorf("load matrix: %w", err)
	}

	r := &preflightReporter{w: out}
	r.checkBaseTooling()
	r.checkContainerLanes(matrix.Nodes)
	r.checkVMLanes(matrix.Nodes)

	if writeErr := writef(out, "[preflight] failures=%d warnings=%d\n", r.failures, r.warnings); writeErr != nil {
		return writeErr
	}
	if r.failures > 0 {
		if writeErr := writeLine(out, "[preflight] RESULT=FAIL"); writeErr != nil {
			return writeErr
		}
		return fmt.Errorf("preflight failed: failures=%d warnings=%d", r.failures, r.warnings)
	}
	if strict && r.warnings > 0 {
		if writeErr := writeLine(out, "[preflight] RESULT=FAIL (strict mode: warnings present)"); writeErr != nil {
			return writeErr
		}
		return fmt.Errorf("preflight failed in strict mode: warnings=%d", r.warnings)
	}
	if writeErr := writeLine(out, "[preflight] RESULT=PASS"); writeErr != nil {
		return writeErr
	}
	return nil
}

func (r *preflightReporter) checkBaseTooling() {
	r.info("[preflight] checking base toolchain")
	for _, cmd := range []string{"go", "tar"} {
		if _, err := exec.LookPath(cmd); err != nil {
			r.failf("missing command: %s", cmd)
		} else {
			r.passf("command available: %s", cmd)
		}
	}
}

func (r *preflightReporter) checkContainerLanes(nodes []replay.NodeSpec) {
	containerNodes := make([]replay.NodeSpec, 0)
	for _, node := range nodes {
		if node.Mode == replay.NodeModeContainer {
			containerNodes = append(containerNodes, node)
		}
	}
	if len(containerNodes) == 0 {
		r.warnf("no container nodes found in matrix")
		return
	}
	engine := detectContainerEngine()
	if engine == "" {
		r.failf("container lanes exist but no container engine found (podman/docker)")
		return
	}
	if _, err := exec.LookPath(engine); err != nil {
		r.failf("container engine not executable: %s", engine)
		return
	}
	if out, err := runCommandCapture(engine, "info"); err != nil {
		r.failf("container engine not reachable: %s (%s)", engine, out)
		return
	}
	r.passf("container engine reachable: %s", engine)
	r.info("[preflight] checking offline container images")
	for _, node := range containerNodes {
		image := containerImageFromNode(node)
		if image == "" {
			r.failf("container node %s has empty image in matrix", node.ID)
			continue
		}
		if out, err := runCommandCapture(engine, "image", "inspect", image); err != nil {
			r.failf("container image missing/unreachable: %s -> %s (%s)", node.ID, image, out)
		} else {
			r.passf("container image present: %s -> %s", node.ID, image)
		}
	}
}

//nolint:gocognit,gocyclo,cyclop // REQ:OFFLINE-LOCAL-001 VM lane checks keep failure attribution explicit per node.
func (r *preflightReporter) checkVMLanes(nodes []replay.NodeSpec) {
	vmNodes := make([]vmLane, 0)
	for _, node := range nodes {
		if node.Mode != replay.NodeModeVM {
			continue
		}
		vmNodes = append(vmNodes, vmLaneFromNode(node))
	}
	if len(vmNodes) == 0 {
		r.warnf("no vm nodes found in matrix")
		return
	}
	r.info("[preflight] checking vm/libvirt dependencies")
	for _, cmd := range []string{"virsh", "ssh", "scp"} {
		if _, err := exec.LookPath(cmd); err != nil {
			r.failf("missing command: %s", cmd)
		} else {
			r.passf("command available: %s", cmd)
		}
	}

	for _, lane := range vmNodes {
		if lane.Domain == "" {
			r.failf("vm node %s has empty domain in matrix", lane.NodeID)
			continue
		}
		if _, err := runCommandCapture("virsh", "dominfo", lane.Domain); err != nil {
			r.failf("libvirt domain missing/unreachable: %s -> %s", lane.NodeID, lane.Domain)
			continue
		}
		r.passf("libvirt domain exists: %s -> %s", lane.NodeID, lane.Domain)
		if lane.Snapshot != "-" {
			snapsOut, err := runCommandCapture("virsh", "snapshot-list", "--name", lane.Domain)
			if err != nil {
				r.failf("snapshot check failed: %s/%s (%s)", lane.Domain, lane.Snapshot, snapsOut)
			} else if containsLine(snapsOut, lane.Snapshot) {
				r.passf("snapshot exists: %s/%s", lane.Domain, lane.Snapshot)
			} else {
				r.failf("snapshot missing: %s/%s", lane.Domain, lane.Snapshot)
			}
		}
		if out, err := runSSHProbe(lane.SSHTarget, lane.SSHOptions); err != nil {
			r.failf("vm ssh unreachable: %s -> %s (%s)", lane.NodeID, lane.SSHTarget, out)
		} else {
			r.passf("vm ssh reachable: %s -> %s", lane.NodeID, lane.SSHTarget)
		}
	}
}

func (r *preflightReporter) info(msg string) {
	if err := writeLine(r.w, msg); err != nil {
		r.failures++
	}
}

func (r *preflightReporter) passf(format string, args ...any) {
	if err := writef(r.w, "[PASS] "+format+"\n", args...); err != nil {
		r.failures++
	}
}

func (r *preflightReporter) warnf(format string, args ...any) {
	r.warnings++
	if err := writef(r.w, "[WARN] "+format+"\n", args...); err != nil {
		r.failures++
	}
}

func (r *preflightReporter) failf(format string, args ...any) {
	r.failures++
	if err := writef(r.w, "[FAIL] "+format+"\n", args...); err != nil {
		r.failures++
	}
}

func runCommandCapture(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func runSSHProbe(target, options string) (string, error) {
	args := make([]string, 0, 12)
	if strings.TrimSpace(options) != "" && strings.TrimSpace(options) != "-" {
		args = append(args, strings.Fields(options)...)
	}
	args = append(args, "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", target, "true")
	return runCommandCapture("ssh", args...)
}

func detectContainerEngine() string {
	if engine := lookupEnvTrimmed("JCS_CONTAINER_ENGINE"); engine != "" {
		return engine
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	return ""
}

func vmLaneFromNode(node replay.NodeSpec) vmLane {
	domain := ""
	snapshot := "snapshot-cold"
	if len(node.Runner.Replay) > 1 {
		domain = strings.TrimSpace(node.Runner.Replay[1])
	}
	if len(node.Runner.Replay) > 2 {
		snapshot = strings.TrimSpace(node.Runner.Replay[2])
		if snapshot == "" {
			snapshot = "snapshot-cold"
		}
	}
	sshTarget := strings.TrimSpace(node.Runner.Env["JCS_VM_SSH_TARGET"])
	if sshTarget == "" {
		sshTarget = "root@" + domain
	}
	sshOptions := strings.TrimSpace(node.Runner.Env["JCS_VM_SSH_OPTIONS"])
	if sshOptions == "" {
		sshOptions = "-"
	}
	return vmLane{
		NodeID:     node.ID,
		Domain:     domain,
		Snapshot:   snapshot,
		SSHTarget:  sshTarget,
		SSHOptions: sshOptions,
	}
}

func containerImageFromNode(node replay.NodeSpec) string {
	if len(node.Runner.Replay) > 1 {
		return strings.TrimSpace(node.Runner.Replay[1])
	}
	return ""
}

func containsLine(text, want string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}

func buildAuditOutputs(matrixPath, profilePath, evidencePath string) (string, auditSummary, string, error) {
	if err := cmdVerifyEvidence(map[string]string{
		"--matrix":   matrixPath,
		"--profile":  profilePath,
		"--evidence": evidencePath,
	}, io.Discard); err != nil {
		return "", auditSummary{}, "", err
	}
	reportOut, err := runLoggedStepCapture("", io.Discard, func(w io.Writer) error {
		return cmdReport(map[string]string{"--evidence": evidencePath}, w)
	})
	if err != nil {
		return "", auditSummary{}, "", err
	}
	evidence, err := replay.LoadEvidence(evidencePath)
	if err != nil {
		return "", auditSummary{}, "", fmt.Errorf("load evidence: %w", err)
	}
	summary := buildAuditSummary(evidence, matrixPath, profilePath, evidencePath, wallClockNowUTC())
	markdown := renderAuditSummaryMarkdown(summary)
	return reportOut, summary, markdown, nil
}

//nolint:gocyclo,cyclop // REQ:OFFLINE-EVIDENCE-001 audit output writer keeps artifact writes and diagnostics explicit.
func writeAuditOutputs(matrixPath, profilePath, evidencePath, outputDir, controllerReport string, out io.Writer) (auditSummary, string, string, error) {
	evidence, err := replay.LoadEvidence(evidencePath)
	if err != nil {
		return auditSummary{}, "", "", fmt.Errorf("load evidence: %w", err)
	}
	summary := buildAuditSummary(evidence, matrixPath, profilePath, evidencePath, wallClockNowUTC())
	markdown := renderAuditSummaryMarkdown(summary)

	jsonPath := filepath.Join(outputDir, "audit-summary.json")
	mdPath := filepath.Join(outputDir, "audit-summary.md")
	reportPath := filepath.Join(outputDir, "controller-report.txt")
	if dirErr := os.MkdirAll(outputDir, dirPerm); dirErr != nil {
		return auditSummary{}, "", "", fmt.Errorf("create audit output dir: %w", dirErr)
	}
	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return auditSummary{}, "", "", fmt.Errorf("marshal audit summary: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(jsonPath, encoded, filePerm); err != nil {
		return auditSummary{}, "", "", fmt.Errorf("write audit summary json: %w", err)
	}
	if err := os.WriteFile(mdPath, []byte(markdown), filePerm); err != nil {
		return auditSummary{}, "", "", fmt.Errorf("write audit summary markdown: %w", err)
	}
	if err := os.WriteFile(reportPath, []byte(controllerReport), filePerm); err != nil {
		return auditSummary{}, "", "", fmt.Errorf("write controller report: %w", err)
	}
	if err := writef(out, "%s", markdown); err != nil {
		return auditSummary{}, "", "", err
	}
	if err := writef(out, "[audit] wrote: %s\n", jsonPath); err != nil {
		return auditSummary{}, "", "", err
	}
	if err := writef(out, "[audit] wrote: %s\n", mdPath); err != nil {
		return auditSummary{}, "", "", err
	}
	if err := writef(out, "[audit] wrote: %s\n", reportPath); err != nil {
		return auditSummary{}, "", "", err
	}
	return summary, jsonPath, mdPath, nil
}

func buildAuditSummary(evidence *replay.EvidenceBundle, matrixPath, profilePath, evidencePath string, now time.Time) auditSummary {
	byNode := make(map[string][]replay.NodeRunEvidence)
	canonical := make(map[string]struct{})
	verify := make(map[string]struct{})
	failureClass := make(map[string]struct{})
	exitCode := make(map[string]struct{})

	for _, run := range evidence.NodeReplays {
		byNode[run.NodeID] = append(byNode[run.NodeID], run)
		if run.CanonicalSHA256 != "" {
			canonical[run.CanonicalSHA256] = struct{}{}
		}
		if run.VerifySHA256 != "" {
			verify[run.VerifySHA256] = struct{}{}
		}
		if run.FailureClassSHA256 != "" {
			failureClass[run.FailureClassSHA256] = struct{}{}
		}
		if run.ExitCodeSHA256 != "" {
			exitCode[run.ExitCodeSHA256] = struct{}{}
		}
	}

	nodeReplayCounts := make(map[string]int, len(byNode))
	nodeReplayCaseCounts := make(map[string][]int, len(byNode))
	for nodeID, runs := range byNode {
		sort.Slice(runs, func(i, j int) bool {
			return runs[i].ReplayIndex < runs[j].ReplayIndex
		})
		nodeReplayCounts[nodeID] = len(runs)
		caseCounts := make([]int, 0, len(runs))
		for _, run := range runs {
			caseCounts = append(caseCounts, run.CaseCount)
		}
		nodeReplayCaseCounts[nodeID] = caseCounts
	}

	parity := map[string]bool{
		"canonical_single_digest":     len(canonical) == 1,
		"verify_single_digest":        len(verify) == 1,
		"failure_class_single_digest": len(failureClass) == 1,
		"exit_code_single_digest":     len(exitCode) == 1,
	}
	result := resultPass
	for _, ok := range parity {
		if !ok {
			result = resultFail
			break
		}
	}

	return auditSummary{
		GeneratedAtUTC:  now.Format(time.RFC3339Nano),
		MatrixPath:      matrixPath,
		ProfilePath:     profilePath,
		EvidencePath:    evidencePath,
		SchemaVersion:   evidence.SchemaVersion,
		ProfileName:     evidence.ProfileName,
		Architecture:    evidence.Architecture,
		HardReleaseGate: evidence.HardReleaseGate,
		NodeCount:       len(byNode),
		RunCount:        len(evidence.NodeReplays),
		RequiredSuites:  append([]string(nil), evidence.RequiredSuites...),
		Aggregate: map[string]string{
			"canonical":     evidence.AggregateCanonical,
			"verify":        evidence.AggregateVerify,
			"failure_class": evidence.AggregateClass,
			"exit_code":     evidence.AggregateExitCode,
		},
		DigestSets: map[string][]string{
			"canonical_unique":     sortedSetKeys(canonical),
			"verify_unique":        sortedSetKeys(verify),
			"failure_class_unique": sortedSetKeys(failureClass),
			"exit_code_unique":     sortedSetKeys(exitCode),
		},
		NodeReplayCounts:    nodeReplayCounts,
		NodeReplayCaseCount: nodeReplayCaseCounts,
		Parity:              parity,
		Result:              result,
	}
}

func renderAuditSummaryMarkdown(summary auditSummary) string {
	lines := make([]string, 0, 48)
	lines = append(lines, "# Offline Replay Audit Summary")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("- Result: **%s**", summary.Result))
	lines = append(lines, fmt.Sprintf("- Evidence: `%s`", summary.EvidencePath))
	lines = append(lines, fmt.Sprintf("- Matrix: `%s`", summary.MatrixPath))
	lines = append(lines, fmt.Sprintf("- Profile: `%s`", summary.ProfilePath))
	lines = append(lines, fmt.Sprintf("- Schema: `%s`", summary.SchemaVersion))
	lines = append(lines, fmt.Sprintf("- Profile Name: `%s`", summary.ProfileName))
	lines = append(lines, fmt.Sprintf("- Architecture: `%s`", summary.Architecture))
	lines = append(lines, fmt.Sprintf("- Hard Release Gate: `%t`", summary.HardReleaseGate))
	lines = append(lines, fmt.Sprintf("- Node Count: `%d`", summary.NodeCount))
	lines = append(lines, fmt.Sprintf("- Replay Rows: `%d`", summary.RunCount))
	lines = append(lines, "")
	lines = append(lines, "## Aggregate Digests")
	lines = append(lines, "")
	aggKeys := sortedMapKeys(summary.Aggregate)
	for _, key := range aggKeys {
		lines = append(lines, fmt.Sprintf("- %s: `%s`", key, summary.Aggregate[key]))
	}
	lines = append(lines, "")
	lines = append(lines, "## Parity Checks")
	lines = append(lines, "")
	parityKeys := sortedBoolMapKeys(summary.Parity)
	for _, key := range parityKeys {
		lines = append(lines, fmt.Sprintf("- %s: `%t`", key, summary.Parity[key]))
	}
	lines = append(lines, "")
	lines = append(lines, "## Node Replay Counts")
	lines = append(lines, "")
	nodeKeys := sortedIntMapKeys(summary.NodeReplayCounts)
	for _, key := range nodeKeys {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", key, summary.NodeReplayCounts[key]))
	}
	lines = append(lines, "")
	lines = append(lines, "## Node Case Counts By Replay Index")
	lines = append(lines, "")
	caseKeys := sortedSliceMapKeys(summary.NodeReplayCaseCount)
	for _, key := range caseKeys {
		lines = append(lines, fmt.Sprintf("- %s: `%v`", key, summary.NodeReplayCaseCount[key]))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func compareCrossArchEvidence(x86EvidencePath, armEvidencePath, jsonPath, mdPath string) (*crossArchReport, error) {
	x86Evidence, err := replay.LoadEvidence(x86EvidencePath)
	if err != nil {
		return nil, fmt.Errorf("load x86 evidence: %w", err)
	}
	armEvidence, err := replay.LoadEvidence(armEvidencePath)
	if err != nil {
		return nil, fmt.Errorf("load arm64 evidence: %w", err)
	}
	report := buildCrossArchReport(x86Evidence, armEvidence, x86EvidencePath, armEvidencePath, wallClockNowUTC())
	if err := writeCrossArchReport(report, jsonPath, mdPath); err != nil {
		return nil, err
	}
	if report.Result != resultPass {
		return report, fmt.Errorf("cross-arch digest comparison failed")
	}
	return report, nil
}

func buildCrossArchReport(x86Evidence, armEvidence *replay.EvidenceBundle, x86Path, armPath string, now time.Time) *crossArchReport {
	checks := []crossArchCheck{
		{Field: "aggregate_canonical_sha256", Label: "canonical", X86: x86Evidence.AggregateCanonical, Arm64: armEvidence.AggregateCanonical},
		{Field: "aggregate_verify_sha256", Label: "verify", X86: x86Evidence.AggregateVerify, Arm64: armEvidence.AggregateVerify},
		{Field: "aggregate_failure_class_sha256", Label: "failure_class", X86: x86Evidence.AggregateClass, Arm64: armEvidence.AggregateClass},
		{Field: "aggregate_exit_code_sha256", Label: "exit_code", X86: x86Evidence.AggregateExitCode, Arm64: armEvidence.AggregateExitCode},
	}
	allMatch := true
	for i := range checks {
		checks[i].Match = checks[i].X86 == checks[i].Arm64
		allMatch = allMatch && checks[i].Match
	}
	result := resultFail
	if allMatch {
		result = resultPass
	}
	return &crossArchReport{
		GeneratedAtUTC: now.Format(time.RFC3339Nano),
		X86Evidence:    x86Path,
		Arm64Evidence:  armPath,
		Result:         result,
		Checks:         checks,
	}
}

func writeCrossArchReport(report *crossArchReport, jsonPath, mdPath string) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cross-arch report: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(jsonPath, encoded, filePerm); err != nil {
		return fmt.Errorf("write cross-arch json report: %w", err)
	}
	lines := make([]string, 0, len(report.Checks)+8)
	lines = append(lines, "# Cross-Arch Replay Comparison")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("- Result: **%s**", report.Result))
	lines = append(lines, fmt.Sprintf("- x86 evidence: `%s`", report.X86Evidence))
	lines = append(lines, fmt.Sprintf("- arm64 evidence: `%s`", report.Arm64Evidence))
	lines = append(lines, "")
	lines = append(lines, "| Field | x86_64 | arm64 | Match |")
	lines = append(lines, "|---|---|---|---|")
	for _, check := range report.Checks {
		lines = append(lines, fmt.Sprintf("| %s | `%s` | `%s` | `%t` |", check.Field, check.X86, check.Arm64, check.Match))
	}
	lines = append(lines, "")
	if err := os.WriteFile(mdPath, []byte(strings.Join(lines, "\n")), filePerm); err != nil {
		return fmt.Errorf("write cross-arch markdown report: %w", err)
	}
	return nil
}

func runGoCommandLogged(logPath string, mirror io.Writer, env map[string]string, args ...string) error {
	if logPath == "" {
		return runCommandToWriter(io.Discard, mirror, env, "go", args...)
	}
	if err := os.MkdirAll(filepath.Dir(logPath), dirPerm); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 operator-configured log paths are intentional offline harness outputs.
	fd, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file %s: %w", logPath, err)
	}
	defer func() {
		if closeErr := fd.Close(); closeErr != nil {
			_ = closeErr
		}
	}()
	return runCommandToWriter(fd, mirror, env, "go", args...)
}

func runCommandToWriter(log io.Writer, mirror io.Writer, env map[string]string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = mergedEnv(env)
	writer := log
	if mirror != nil {
		writer = io.MultiWriter(log, mirror)
	}
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s %v: %w", name, args, err)
	}
	return nil
}

func mergedEnv(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return os.Environ()
	}
	env := os.Environ()
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		env = append(env, key+"="+overrides[key])
	}
	return env
}

func runLoggedStep(logPath string, stdout io.Writer, fn func(io.Writer) error) error {
	if logPath == "" {
		return fn(stdout)
	}
	if err := os.MkdirAll(filepath.Dir(logPath), dirPerm); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 operator-configured log paths are intentional offline harness outputs.
	fd, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer func() {
		if closeErr := fd.Close(); closeErr != nil {
			_ = closeErr
		}
	}()
	writer := io.MultiWriter(fd, stdout)
	return fn(writer)
}

func runLoggedStepCapture(logPath string, stdout io.Writer, fn func(io.Writer) error) (string, error) {
	var buf bytes.Buffer
	err := runLoggedStep(logPath, stdout, func(w io.Writer) error {
		return fn(io.MultiWriter(w, &buf))
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeChecksumFile(outputPath, artifactPath string) error {
	sum, err := fileSHA256(artifactPath)
	if err != nil {
		return err
	}
	line := sum + "  " + artifactPath + "\n"
	if err := os.WriteFile(outputPath, []byte(line), filePerm); err != nil {
		return fmt.Errorf("write checksum file %s: %w", outputPath, err)
	}
	return nil
}

func writeRunIndex(path string, artifacts runSuiteArtifacts) error {
	content := strings.Join([]string{
		"offline_cold_replay_run_dir=" + artifacts.OutputDir,
		"matrix=" + artifacts.MatrixPath,
		"profile=" + artifacts.ProfilePath,
		"bundle=" + artifacts.BundlePath,
		"evidence=" + artifacts.EvidencePath,
		"controller=" + artifacts.ControllerPath,
		"canonicalizer=" + artifacts.CanonicalPath,
		"audit_markdown=" + filepath.Join(artifacts.OutputDir, "audit", "audit-summary.md"),
		"audit_json=" + filepath.Join(artifacts.OutputDir, "audit", "audit-summary.json"),
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), filePerm); err != nil {
		return fmt.Errorf("write run index: %w", err)
	}
	return nil
}

func parseBoolFlag(flags map[string]string, name string) (bool, error) {
	raw := strings.TrimSpace(flags[name])
	if raw == "" {
		return false, nil
	}
	return parseBoolToken(raw)
}

func parseBoolToken(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", raw)
	}
}

func defaultString(flags map[string]string, key, fallback string) string {
	if value := strings.TrimSpace(flags[key]); value != "" {
		return value
	}
	return fallback
}

func utcStamp() string {
	return wallClockNowUTC().Format("20060102T150405Z")
}

func lookupEnvTrimmed(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

//nolint:forbidigo // REQ:OFFLINE-EVIDENCE-001 offline orchestration timestamps represent real execution time for audit artifacts.
func wallClockNowUTC() time.Time {
	return time.Now().UTC()
}

func sortedSetKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedBoolMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedIntMapKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSliceMapKeys(m map[string][]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
