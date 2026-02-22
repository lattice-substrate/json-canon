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

	"github.com/lattice-substrate/json-canon/offline/replay"
)

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
	flags, err := parseKV(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	bundlePath := strings.TrimSpace(flags["--bundle"])
	evidencePath := strings.TrimSpace(flags["--evidence"])
	nodeID := strings.TrimSpace(flags["--node-id"])
	mode := strings.TrimSpace(flags["--mode"])
	distro := strings.TrimSpace(flags["--distro"])
	kernelFamily := strings.TrimSpace(flags["--kernel-family"])
	replayIndexRaw := strings.TrimSpace(flags["--replay-index"])

	if bundlePath == "" || evidencePath == "" || nodeID == "" || mode == "" || distro == "" || kernelFamily == "" || replayIndexRaw == "" {
		fmt.Fprintln(stderr, "error: required flags: --bundle --evidence --node-id --mode --distro --kernel-family --replay-index")
		return 2
	}
	replayIndex, err := strconv.Atoi(replayIndexRaw)
	if err != nil || replayIndex < 1 {
		fmt.Fprintf(stderr, "error: invalid --replay-index %q\n", replayIndexRaw)
		return 2
	}

	start := time.Now().UTC()
	tmpDir, err := os.MkdirTemp("", "jcs-offline-worker-*")
	if err != nil {
		fmt.Fprintf(stderr, "error: create temp dir: %v\n", err)
		return 2
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manifest, err := extractBundle(bundlePath, tmpDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: extract bundle: %v\n", err)
		return 2
	}
	if err := verifyExtractedBundle(tmpDir, manifest); err != nil {
		fmt.Fprintf(stderr, "error: verify bundle: %v\n", err)
		return 2
	}

	binaryPath := filepath.Join(tmpDir, filepath.FromSlash(manifest.BinaryPath))
	canonicalAcc := &digestAccumulator{}
	verifyAcc := &digestAccumulator{}
	classAcc := &digestAccumulator{}
	exitAcc := &digestAccumulator{}

	caseCount, err := runVectors(binaryPath, tmpDir, manifest, canonicalAcc, verifyAcc, classAcc, exitAcc)
	if err != nil {
		fmt.Fprintf(stderr, "error: replay vectors: %v\n", err)
		return 2
	}
	if err := checkEnvironmentIndependence(binaryPath); err != nil {
		fmt.Fprintf(stderr, "error: environment-independence check: %v\n", err)
		return 2
	}

	evidence := replay.NodeRunEvidence{
		NodeID:             nodeID,
		Mode:               mode,
		Distro:             distro,
		KernelFamily:       kernelFamily,
		ReplayIndex:        replayIndex,
		SessionID:          fmt.Sprintf("%s-%d-%d", nodeID, os.Getpid(), time.Now().UnixNano()),
		StartedAtUTC:       start.Format(time.RFC3339Nano),
		CompletedAtUTC:     time.Now().UTC().Format(time.RFC3339Nano),
		CaseCount:          caseCount,
		Passed:             true,
		CanonicalSHA256:    canonicalAcc.Hex(),
		VerifySHA256:       verifyAcc.Hex(),
		FailureClassSHA256: classAcc.Hex(),
		ExitCodeSHA256:     exitAcc.Hex(),
	}

	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: encode evidence: %v\n", err)
		return 2
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(evidencePath, encoded, 0o600); err != nil {
		fmt.Fprintf(stderr, "error: write evidence: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "ok node=%s replay=%d cases=%d\n", nodeID, replayIndex, caseCount)
	return 0
}

func runVectors(binaryPath, root string, manifest *replay.BundleManifest, canonicalAcc, verifyAcc, classAcc, exitAcc *digestAccumulator) (int, error) {
	vectorFiles := append([]string(nil), manifest.VectorFiles...)
	sort.Strings(vectorFiles)
	caseCount := 0

	for _, rel := range vectorFiles {
		p := filepath.Join(root, filepath.FromSlash(rel))
		fd, err := os.Open(p)
		if err != nil {
			return 0, fmt.Errorf("open vector %s: %w", rel, err)
		}
		sc := bufio.NewScanner(fd)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		lineNo := 0
		for sc.Scan() {
			lineNo++
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			var v vectorCase
			if err := json.Unmarshal([]byte(line), &v); err != nil {
				_ = fd.Close()
				return 0, fmt.Errorf("decode vector %s:%d: %w", rel, lineNo, err)
			}
			args := append([]string(nil), v.Args...)
			if len(args) == 0 {
				if strings.TrimSpace(v.Mode) == "" {
					_ = fd.Close()
					return 0, fmt.Errorf("vector %s:%d id=%s missing mode and args", rel, lineNo, v.ID)
				}
				args = []string{v.Mode, "-"}
			}
			res, err := runCLI(binaryPath, args, []byte(v.Input), nil)
			if err != nil {
				_ = fd.Close()
				return 0, fmt.Errorf("run vector %s:%d id=%s: %w", rel, lineNo, v.ID, err)
			}
			if res.exitCode != v.WantExit {
				_ = fd.Close()
				return 0, fmt.Errorf("vector %s:%d id=%s exit mismatch got=%d want=%d", rel, lineNo, v.ID, res.exitCode, v.WantExit)
			}
			if v.WantStdout != nil && res.stdout != *v.WantStdout {
				_ = fd.Close()
				return 0, fmt.Errorf("vector %s:%d id=%s stdout mismatch", rel, lineNo, v.ID)
			}
			if v.WantStderr != nil && res.stderr != *v.WantStderr {
				_ = fd.Close()
				return 0, fmt.Errorf("vector %s:%d id=%s stderr mismatch", rel, lineNo, v.ID)
			}
			if v.WantStderrContains != nil && !strings.Contains(res.stderr, *v.WantStderrContains) {
				_ = fd.Close()
				return 0, fmt.Errorf("vector %s:%d id=%s stderr missing %q", rel, lineNo, v.ID, *v.WantStderrContains)
			}

			mode := args[0]
			exitStr := strconv.Itoa(res.exitCode)
			exitAcc.Add(v.ID, exitStr)
			if mode == "canonicalize" {
				canonicalAcc.Add(v.ID, res.stdout)
			}
			if mode == "verify" {
				verifyAcc.Add(v.ID, exitStr, res.stdout, res.stderr)
			}
			classToken := "OK"
			if res.exitCode != 0 {
				classToken = extractFailureClass(res.stderr)
			}
			classAcc.Add(v.ID, classToken)
			caseCount++
		}
		if err := sc.Err(); err != nil {
			_ = fd.Close()
			return 0, fmt.Errorf("scan vector %s: %w", rel, err)
		}
		if err := fd.Close(); err != nil {
			return 0, fmt.Errorf("close vector %s: %w", rel, err)
		}
	}
	if caseCount == 0 {
		return 0, fmt.Errorf("no vector cases executed")
	}
	return caseCount, nil
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
			return cliResult{}, err
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

func extractBundle(bundlePath, outDir string) (*replay.BundleManifest, error) {
	f, err := os.Open(bundlePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		clean := path.Clean(hdr.Name)
		if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
			return nil, fmt.Errorf("unsafe tar path %q", hdr.Name)
		}
		target := filepath.Join(outDir, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return nil, err
		}
		if err := out.Close(); err != nil {
			return nil, err
		}
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest replay.BundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func verifyExtractedBundle(root string, manifest *replay.BundleManifest) error {
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

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
