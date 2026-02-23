// Command jcs-offline-worker executes replay vectors from an offline bundle.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SolutionsExcite/json-canon/offline/replay"
)

const maxVectorLineBytes = 4 * 1024 * 1024

type vectorCase struct {
	ID                 string   `json:"id"`
	Mode               string   `json:"mode,omitempty"`
	Args               []string `json:"args,omitempty"`
	Input              string   `json:"input"`
	WantStdout         *string  `json:"want_stdout,omitempty"`
	WantStderr         *string  `json:"want_stderr,omitempty"`
	WantStderrContains *string  `json:"want_stderr_contains,omitempty"`
	WantExit           int      `json:"want_exit"`
}

type cliResult struct {
	exitCode int
	stdout   string
	stderr   string
}

type digestAccumulator struct {
	buf bytes.Buffer
}

type workerArgs struct {
	bundlePath   string
	evidencePath string
	nodeID       string
	mode         string
	distro       string
	kernelFamily string
	replayIndex  int
}

func (d *digestAccumulator) Add(parts ...string) {
	for i, part := range parts {
		if i > 0 {
			d.buf.WriteByte('\x1f')
		}
		d.buf.WriteString(part)
	}
	d.buf.WriteByte('\n')
}

func (d *digestAccumulator) Hex() string {
	sum := sha256.Sum256(d.buf.Bytes())
	return hex.EncodeToString(sum[:])
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := parseWorkerArgs(args)
	if err != nil {
		writeErrorLine(stderr, err)
		return 2
	}

	startedAt := wallClockNowUTC()
	tmpDir, err := os.MkdirTemp("", "jcs-offline-worker-*")
	if err != nil {
		writeErrorLine(stderr, fmt.Errorf("create temp dir: %w", err))
		return 2
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			_ = removeErr
		}
	}()

	manifest, err := extractBundle(cfg.bundlePath, tmpDir)
	if err != nil {
		writeErrorLine(stderr, fmt.Errorf("extract bundle: %w", err))
		return 2
	}
	verifyErr := verifyExtractedBundle(tmpDir, manifest)
	if verifyErr != nil {
		writeErrorLine(stderr, fmt.Errorf("verify bundle: %w", verifyErr))
		return 2
	}

	binaryPath := filepath.Join(tmpDir, filepath.FromSlash(manifest.BinaryPath))
	canonicalAcc := &digestAccumulator{}
	verifyAcc := &digestAccumulator{}
	classAcc := &digestAccumulator{}
	exitAcc := &digestAccumulator{}

	caseCount, err := runVectors(binaryPath, tmpDir, manifest, canonicalAcc, verifyAcc, classAcc, exitAcc)
	if err != nil {
		writeErrorLine(stderr, fmt.Errorf("replay vectors: %w", err))
		return 2
	}
	independenceErr := checkEnvironmentIndependence(binaryPath)
	if independenceErr != nil {
		writeErrorLine(stderr, fmt.Errorf("environment-independence check: %w", independenceErr))
		return 2
	}

	completedAt := wallClockNowUTC()
	evidence := replay.NodeRunEvidence{
		NodeID:             cfg.nodeID,
		Mode:               cfg.mode,
		Distro:             cfg.distro,
		KernelFamily:       cfg.kernelFamily,
		ReplayIndex:        cfg.replayIndex,
		SessionID:          fmt.Sprintf("%s-%d-%d", cfg.nodeID, os.Getpid(), startedAt.UnixNano()),
		StartedAtUTC:       startedAt.Format(time.RFC3339Nano),
		CompletedAtUTC:     completedAt.Format(time.RFC3339Nano),
		CaseCount:          caseCount,
		Passed:             true,
		CanonicalSHA256:    canonicalAcc.Hex(),
		VerifySHA256:       verifyAcc.Hex(),
		FailureClassSHA256: classAcc.Hex(),
		ExitCodeSHA256:     exitAcc.Hex(),
	}

	if err := writeEvidence(cfg.evidencePath, evidence); err != nil {
		writeErrorLine(stderr, err)
		return 2
	}
	if err := writef(stdout, "ok node=%s replay=%d cases=%d\n", cfg.nodeID, cfg.replayIndex, caseCount); err != nil {
		return 2
	}
	return 0
}

func parseWorkerArgs(args []string) (workerArgs, error) {
	flags, err := parseKV(args)
	if err != nil {
		return workerArgs{}, err
	}

	cfg := workerArgs{
		bundlePath:   strings.TrimSpace(flags["--bundle"]),
		evidencePath: strings.TrimSpace(flags["--evidence"]),
		nodeID:       strings.TrimSpace(flags["--node-id"]),
		mode:         strings.TrimSpace(flags["--mode"]),
		distro:       strings.TrimSpace(flags["--distro"]),
		kernelFamily: strings.TrimSpace(flags["--kernel-family"]),
	}
	replayIndexRaw := strings.TrimSpace(flags["--replay-index"])

	if validateErr := validateRequiredWorkerFlags(cfg, replayIndexRaw); validateErr != nil {
		return workerArgs{}, validateErr
	}
	cfg.replayIndex, err = strconv.Atoi(replayIndexRaw)
	if err != nil || cfg.replayIndex < 1 {
		return workerArgs{}, fmt.Errorf("invalid --replay-index %q", replayIndexRaw)
	}
	return cfg, nil
}

func validateRequiredWorkerFlags(cfg workerArgs, replayIndexRaw string) error {
	required := []struct {
		name  string
		value string
	}{
		{name: "--bundle", value: cfg.bundlePath},
		{name: "--evidence", value: cfg.evidencePath},
		{name: "--node-id", value: cfg.nodeID},
		{name: "--mode", value: cfg.mode},
		{name: "--distro", value: cfg.distro},
		{name: "--kernel-family", value: cfg.kernelFamily},
		{name: "--replay-index", value: replayIndexRaw},
	}
	for _, item := range required {
		if item.value == "" {
			return fmt.Errorf("required flags: --bundle --evidence --node-id --mode --distro --kernel-family --replay-index")
		}
	}
	return nil
}

func writeEvidence(path string, evidence replay.NodeRunEvidence) error {
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return fmt.Errorf("encode evidence: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write evidence: %w", err)
	}
	return nil
}

func runVectors(binaryPath, root string, manifest *replay.BundleManifest, canonicalAcc, verifyAcc, classAcc, exitAcc *digestAccumulator) (int, error) {
	vectorFiles := append([]string(nil), manifest.VectorFiles...)
	sort.Strings(vectorFiles)

	totalCount := 0
	for _, rel := range vectorFiles {
		count, err := runVectorFile(binaryPath, root, rel, canonicalAcc, verifyAcc, classAcc, exitAcc)
		if err != nil {
			return 0, err
		}
		totalCount += count
	}
	if totalCount == 0 {
		return 0, fmt.Errorf("no vector cases executed")
	}
	return totalCount, nil
}

//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 replay worker reads validated bundle/vector file paths.
func runVectorFile(binaryPath, root, rel string, canonicalAcc, verifyAcc, classAcc, exitAcc *digestAccumulator) (int, error) {
	vectorPath := filepath.Join(root, filepath.FromSlash(rel))
	fd, err := os.Open(vectorPath)
	if err != nil {
		return 0, fmt.Errorf("open vector %s: %w", rel, err)
	}
	defer func() {
		if closeErr := fd.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	sc := bufio.NewScanner(fd)
	sc.Buffer(make([]byte, 0, 64*1024), maxVectorLineBytes)
	lineNo := 0
	executed := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := processVectorLine(binaryPath, rel, lineNo, line, canonicalAcc, verifyAcc, classAcc, exitAcc); err != nil {
			return 0, err
		}
		executed++
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("scan vector %s: %w", rel, err)
	}
	return executed, nil
}

func processVectorLine(binaryPath, rel string, lineNo int, line string, canonicalAcc, verifyAcc, classAcc, exitAcc *digestAccumulator) error {
	var v vectorCase
	if err := json.Unmarshal([]byte(line), &v); err != nil {
		return fmt.Errorf("decode vector %s:%d: %w", rel, lineNo, err)
	}
	args, err := vectorArgs(v, rel, lineNo)
	if err != nil {
		return err
	}

	res, err := runCLI(binaryPath, args, []byte(v.Input), nil)
	if err != nil {
		return fmt.Errorf("run vector %s:%d id=%s: %w", rel, lineNo, v.ID, err)
	}
	if err := assertVectorResult(rel, lineNo, v, res); err != nil {
		return err
	}
	recordDigests(v.ID, args[0], res, canonicalAcc, verifyAcc, classAcc, exitAcc)
	return nil
}

func vectorArgs(v vectorCase, rel string, lineNo int) ([]string, error) {
	args := append([]string(nil), v.Args...)
	if len(args) > 0 {
		return args, nil
	}
	if strings.TrimSpace(v.Mode) == "" {
		return nil, fmt.Errorf("vector %s:%d id=%s missing mode and args", rel, lineNo, v.ID)
	}
	return []string{v.Mode, "-"}, nil
}

func assertVectorResult(rel string, lineNo int, v vectorCase, res cliResult) error {
	if res.exitCode != v.WantExit {
		return fmt.Errorf("vector %s:%d id=%s exit mismatch got=%d want=%d", rel, lineNo, v.ID, res.exitCode, v.WantExit)
	}
	if v.WantStdout != nil && res.stdout != *v.WantStdout {
		return fmt.Errorf("vector %s:%d id=%s stdout mismatch", rel, lineNo, v.ID)
	}
	if v.WantStderr != nil && res.stderr != *v.WantStderr {
		return fmt.Errorf("vector %s:%d id=%s stderr mismatch", rel, lineNo, v.ID)
	}
	if v.WantStderrContains != nil && !strings.Contains(res.stderr, *v.WantStderrContains) {
		return fmt.Errorf("vector %s:%d id=%s stderr missing %q", rel, lineNo, v.ID, *v.WantStderrContains)
	}
	return nil
}

func recordDigests(vectorID, mode string, res cliResult, canonicalAcc, verifyAcc, classAcc, exitAcc *digestAccumulator) {
	exitStr := strconv.Itoa(res.exitCode)
	exitAcc.Add(vectorID, exitStr)
	if mode == "canonicalize" {
		canonicalAcc.Add(vectorID, res.stdout)
	}
	if mode == "verify" {
		verifyAcc.Add(vectorID, exitStr, res.stdout, res.stderr)
	}
	classToken := "OK"
	if res.exitCode != 0 {
		classToken = extractFailureClass(res.stderr)
	}
	classAcc.Add(vectorID, classToken)
}

func checkEnvironmentIndependence(binaryPath string) error {
	input := []byte(`{"b":1,"a":2}`)
	args := []string{"canonicalize", "-"}
	base, err := runCLI(binaryPath, args, input, nil)
	if err != nil {
		return err
	}
	overrides := map[string]string{"LC_ALL": "C", "LANG": "C", "TZ": "UTC"}
	variant, err := runCLI(binaryPath, args, input, overrides)
	if err != nil {
		return err
	}
	if base.exitCode != variant.exitCode || base.stdout != variant.stdout || base.stderr != variant.stderr {
		return fmt.Errorf("output drift under env variation")
	}
	return nil
}

func runCLI(binaryPath string, args []string, stdin []byte, overrides map[string]string) (cliResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	if len(overrides) != 0 {
		env := os.Environ()
		for k, v := range overrides {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			return cliResult{}, fmt.Errorf("run %s %q: %w", binaryPath, args, err)
		}
	}
	return cliResult{exitCode: code, stdout: outBuf.String(), stderr: errBuf.String()}, nil
}

func extractFailureClass(stderr string) string {
	classes := []string{
		"INVALID_UTF8",
		"INVALID_GRAMMAR",
		"DUPLICATE_KEY",
		"LONE_SURROGATE",
		"NONCHARACTER",
		"NUMBER_OVERFLOW",
		"NUMBER_NEGZERO",
		"NUMBER_UNDERFLOW",
		"BOUND_EXCEEDED",
		"NOT_CANONICAL",
		"CLI_USAGE",
		"INTERNAL_IO",
		"INTERNAL_ERROR",
	}
	for _, c := range classes {
		if strings.Contains(stderr, c) {
			return c
		}
	}
	return "UNKNOWN"
}

func parseKV(args []string) (map[string]string, error) {
	flags := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
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

//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 bundle extraction intentionally opens operator-provided bundle paths.
func extractBundle(bundlePath, outDir string) (*replay.BundleManifest, error) {
	f, err := os.Open(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("open bundle: %w", err)
	}
	defer closeBestEffort(f)

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("open gzip reader: %w", err)
	}
	defer closeBestEffort(gz)

	tr := tar.NewReader(gz)
	for {
		hdr, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return nil, fmt.Errorf("read tar entry: %w", nextErr)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		extractErr := extractTarFile(tr, outDir, hdr)
		if extractErr != nil {
			return nil, extractErr
		}
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest replay.BundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &manifest, nil
}

//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 tar extraction writes controlled archive members under bounded root.
func extractTarFile(tr *tar.Reader, outDir string, hdr *tar.Header) error {
	clean := path.Clean(hdr.Name)
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return fmt.Errorf("unsafe tar path %q", hdr.Name)
	}
	target := filepath.Join(outDir, filepath.FromSlash(clean))
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("mkdir for %s: %w", target, err)
	}

	perm := safeTarMode(hdr.Mode)
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("open output %s: %w", target, err)
	}
	if err := copyTarContent(out, tr, hdr.Size); err != nil {
		closeBestEffort(out)
		return fmt.Errorf("copy tar content %s: %w", target, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close output %s: %w", target, err)
	}
	return nil
}

func safeTarMode(mode int64) os.FileMode {
	if mode < 0 || mode > int64(^uint32(0)) {
		return 0o600
	}
	return os.FileMode(uint32(mode)) & 0o777
}

func copyTarContent(out *os.File, tr *tar.Reader, size int64) error {
	if size < 0 {
		return fmt.Errorf("invalid tar size %d", size)
	}
	if _, err := io.CopyN(out, tr, size); err != nil {
		return fmt.Errorf("copy tar payload: %w", err)
	}
	return nil
}

func verifyExtractedBundle(root string, manifest *replay.BundleManifest) error {
	if err := verifyCoreChecksums(root, manifest); err != nil {
		return err
	}
	if err := verifyVectorChecksums(root, manifest); err != nil {
		return err
	}
	return verifyVectorSetChecksum(manifest)
}

func verifyCoreChecksums(root string, manifest *replay.BundleManifest) error {
	checks := map[string]string{
		manifest.BinaryPath:  manifest.BinarySHA256,
		manifest.WorkerPath:  manifest.WorkerSHA256,
		manifest.MatrixPath:  manifest.MatrixSHA256,
		manifest.ProfilePath: manifest.ProfileSHA256,
	}
	for p, want := range checks {
		if strings.TrimSpace(p) == "" || strings.TrimSpace(want) == "" {
			return fmt.Errorf("bundle manifest missing required digest for %q", p)
		}
		got, err := fileSHA256(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("checksum mismatch for %s", p)
		}
	}
	return nil
}

func verifyVectorChecksums(root string, manifest *replay.BundleManifest) error {
	for _, rel := range manifest.VectorFiles {
		want := manifest.VectorSHA256[rel]
		if want == "" {
			return fmt.Errorf("missing vector checksum for %s", rel)
		}
		got, err := fileSHA256(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("vector checksum mismatch for %s", rel)
		}
	}
	return nil
}

func verifyVectorSetChecksum(manifest *replay.BundleManifest) error {
	items := make([]string, 0, len(manifest.VectorFiles))
	for _, rel := range manifest.VectorFiles {
		items = append(items, rel+":"+manifest.VectorSHA256[rel])
	}
	sort.Strings(items)
	recomputed := sha256.Sum256([]byte(strings.Join(items, "\n")))
	if hex.EncodeToString(recomputed[:]) != manifest.VectorSetSHA256 {
		return fmt.Errorf("vector_set checksum mismatch")
	}
	return nil
}

//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 digest verification reads expected artifact paths from validated manifests.
func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func closeBestEffort(c io.Closer) {
	if err := c.Close(); err != nil {
		_ = err
	}
}

func writef(w io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return fmt.Errorf("write stream: %w", err)
	}
	return nil
}

func writeErrorLine(w io.Writer, err error) {
	if writeErr := writef(w, "error: %v\n", err); writeErr != nil {
		return
	}
}

//nolint:forbidigo // REQ:OFFLINE-EVIDENCE-001 worker evidence intentionally records wall-clock observation timestamps.
func wallClockNowUTC() time.Time {
	return time.Now().UTC()
}
