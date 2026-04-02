// Command jcs-offline-worker executes replay vectors from an offline bundle.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const maxVectorLineBytes = 4 * 1024 * 1024

const (
	imdsTokenTTL       = "21600"
	imdsRequestTimeout = 2 * time.Second
)

var (
	imdsAddress    = "http://169.254.169.254"
	imdsHTTPClient = http.DefaultClient
	readFileFunc   = os.ReadFile
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

type workerArgs struct {
	bundlePath           string
	evidencePath         string
	attestationPath      string
	challenge            string
	nodeID               string
	mode                 string
	distro               string
	kernelFamily         string
	replayIndex          int
	imageDigest          string
	schemaVersion        string
	infraBindingEvidence bool
	nativeHostEvidence   bool
}

type hostInspection struct {
	Architecture       string `json:"architecture"`
	OSID               string `json:"os_id"`
	OSVersionID        string `json:"os_version_id"`
	Kernel             string `json:"kernel"`
	CPU                string `json:"cpu"`
	InstanceID         string `json:"instance_id"`
	ImageID            string `json:"image_id"`
	AvailabilityZone   string `json:"availability_zone"`
	Region             string `json:"region"`
	IIDDocument        string `json:"iid_document,omitempty"`
	IIDSignature       string `json:"iid_signature,omitempty"`
	IIDPKCS7           string `json:"iid_pkcs7,omitempty"`
	IIDDocumentSHA256  string `json:"iid_document_sha256"`
	IIDSignatureSHA256 string `json:"iid_signature_sha256"`
	IIDPKCS7SHA256     string `json:"iid_pkcs7_sha256"`
	IIDVerified        bool   `json:"iid_verified"`
}

type instanceIdentityDocument struct {
	AvailabilityZone string `json:"availabilityZone"`
	ImageID          string `json:"imageId"`
	InstanceID       string `json:"instanceId"`
	Region           string `json:"region"`
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
	if len(args) > 0 && args[0] == "inspect-host" {
		if err := runInspectHost(stdout); err != nil {
			writeErrorLine(stderr, err)
			return 2
		}
		return 0
	}
	cfg, err := parseWorkerArgs(args)
	if err != nil {
		writeErrorLine(stderr, err)
		return 2
	}
	if err := runWorker(cfg, stdout); err != nil {
		writeErrorLine(stderr, err)
		return 2
	}
	return 0
}

func runWorker(cfg workerArgs, stdout io.Writer) error {
	startedAt := wallClockNowUTC()
	tmpDir, err := os.MkdirTemp("", "jcs-offline-worker-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			_ = removeErr
		}
	}()

	manifest, err := extractBundle(cfg.bundlePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extract bundle: %w", err)
	}
	verifyErr := verifyExtractedBundle(tmpDir, manifest)
	if verifyErr != nil {
		return fmt.Errorf("verify bundle: %w", verifyErr)
	}

	binaryPath := filepath.Join(tmpDir, filepath.FromSlash(manifest.BinaryPath))
	canonicalAcc := &digestAccumulator{}
	verifyAcc := &digestAccumulator{}
	classAcc := &digestAccumulator{}
	exitAcc := &digestAccumulator{}

	caseCount, err := runVectors(binaryPath, tmpDir, manifest, canonicalAcc, verifyAcc, classAcc, exitAcc)
	if err != nil {
		return fmt.Errorf("replay vectors: %w", err)
	}
	independenceErr := checkEnvironmentIndependence(binaryPath)
	if independenceErr != nil {
		return fmt.Errorf("environment-independence check: %w", independenceErr)
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
	if cfg.infraBindingEvidence || cfg.nativeHostEvidence {
		evidence.DiscoveredCPU = discoverCPU()
		evidence.DiscoveredKernel = discoverKernel()
		evidence.ImageDigest = cfg.imageDigest
	}
	if cfg.nativeHostEvidence {
		inspection, err := inspectHost()
		if err != nil {
			return fmt.Errorf("inspect host: %w", err)
		}
		evidence.MeasuredArchitecture = inspection.Architecture
		evidence.MeasuredOSID = inspection.OSID
		evidence.MeasuredOSVersionID = inspection.OSVersionID
		evidence.MeasuredKernel = inspection.Kernel
		evidence.MeasuredCPU = inspection.CPU
		evidence.AWSInstanceID = inspection.InstanceID
		evidence.AWSImageID = inspection.ImageID
		evidence.ImageDigest = cfg.imageDigest
	}

	if err := writeEvidence(cfg.evidencePath, evidence); err != nil {
		return err
	}
	if cfg.attestationPath != "" {
		if err := writeTransportAttestation(cfg, evidence); err != nil {
			return err
		}
	}
	if err := writef(stdout, "ok node=%s replay=%d cases=%d\n", cfg.nodeID, cfg.replayIndex, caseCount); err != nil {
		return err
	}
	return nil
}

func writeTransportAttestation(cfg workerArgs, evidence replay.NodeRunEvidence) error {
	inspection, err := inspectHost()
	if err != nil {
		return fmt.Errorf("inspect host for transport attestation: %w", err)
	}
	evidenceData, err := os.ReadFile(cfg.evidencePath)
	if err != nil {
		return fmt.Errorf("read evidence for transport attestation: %w", err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate transport attestation key: %w", err)
	}
	attestation := replay.TransportAttestation{
		SchemaVersion:      replay.TransportAttestationSchemaVersion,
		Challenge:          cfg.challenge,
		NodeID:             evidence.NodeID,
		ReplayIndex:        evidence.ReplayIndex,
		EvidenceSHA256:     sha256Hex(evidenceData),
		IIDDocument:        inspection.IIDDocument,
		IIDSignature:       inspection.IIDSignature,
		IIDPKCS7:           inspection.IIDPKCS7,
		IIDDocumentSHA256:  inspection.IIDDocumentSHA256,
		IIDSignatureSHA256: inspection.IIDSignatureSHA256,
		IIDPKCS7SHA256:     inspection.IIDPKCS7SHA256,
		PublicKey:          base64.StdEncoding.EncodeToString(publicKey),
	}
	attestation.Signature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(privateKey, []byte(replay.TransportAttestationSigningPayload(&attestation))),
	)
	if err := replay.WriteTransportAttestation(cfg.attestationPath, &attestation); err != nil {
		return err
	}
	return nil
}

func parseWorkerArgs(args []string) (workerArgs, error) {
	flags, err := parseKV(args)
	if err != nil {
		return workerArgs{}, err
	}

	cfg := workerArgs{
		bundlePath:      strings.TrimSpace(flags["--bundle"]),
		evidencePath:    strings.TrimSpace(flags["--evidence"]),
		attestationPath: strings.TrimSpace(flags["--attestation-out"]),
		challenge:       strings.TrimSpace(flags["--challenge"]),
		nodeID:          strings.TrimSpace(flags["--node-id"]),
		mode:            strings.TrimSpace(flags["--mode"]),
		distro:          strings.TrimSpace(flags["--distro"]),
		kernelFamily:    strings.TrimSpace(flags["--kernel-family"]),
		imageDigest:     strings.TrimSpace(flags["--image-digest"]),
		schemaVersion:   resolveWorkerSchemaVersion(flags),
	}
	cfg.infraBindingEvidence, err = parseBoolFlag(flags, "--infra-binding-evidence")
	if err != nil {
		return workerArgs{}, err
	}
	cfg.nativeHostEvidence, err = parseBoolFlag(flags, "--native-host-evidence")
	if err != nil {
		return workerArgs{}, err
	}
	if cfg.nativeHostEvidence {
		cfg.infraBindingEvidence = true
	}
	replayIndexRaw := strings.TrimSpace(flags["--replay-index"])

	if validateErr := validateRequiredWorkerFlags(cfg, replayIndexRaw); validateErr != nil {
		return workerArgs{}, validateErr
	}
	cfg.replayIndex, err = strconv.Atoi(replayIndexRaw)
	if err != nil || cfg.replayIndex < 1 {
		return workerArgs{}, fmt.Errorf("invalid --replay-index %q", replayIndexRaw)
	}
	if cfg.schemaVersion != replay.EvidenceSchemaVersion {
		return workerArgs{}, fmt.Errorf("invalid --schema-version %q", cfg.schemaVersion)
	}
	if (cfg.attestationPath == "") != (cfg.challenge == "") {
		return workerArgs{}, fmt.Errorf("--challenge and --attestation-out must be provided together")
	}
	return cfg, nil
}

func resolveWorkerSchemaVersion(flags map[string]string) string {
	if schema := strings.TrimSpace(flags["--schema-version"]); schema != "" {
		return schema
	}
	if schema, ok := os.LookupEnv("JCS_EVIDENCE_SCHEMA_VERSION"); ok {
		if trimmed := strings.TrimSpace(schema); trimmed != "" {
			return trimmed
		}
	}
	return replay.EvidenceSchemaVersion
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

func parseBoolFlag(flags map[string]string, name string) (bool, error) {
	value := strings.TrimSpace(flags[name])
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q", name, value)
	}
	return parsed, nil
}

func writeEvidence(path string, evidence replay.NodeRunEvidence) error {
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return fmt.Errorf("encode evidence: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := atomicWriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write evidence: %w", err)
	}
	return nil
}

func runInspectHost(stdout io.Writer) error {
	inspection, err := inspectHost()
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(inspection, "", "  ")
	if err != nil {
		return fmt.Errorf("encode host inspection: %w", err)
	}
	return writef(stdout, "%s\n", encoded)
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
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		case tar.TypeSymlink, tar.TypeLink:
			return nil, fmt.Errorf("unsupported tar entry type %q for %s", hdr.Typeflag, hdr.Name)
		default:
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

// discoverCPU reads the CPU model name from /proc/cpuinfo.
// Returns empty string if the file is absent or the field is not present.
func discoverCPU() string {
	data, err := readFileFunc("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	values := parseCPUInfoFields(string(data))
	if model := values["model name"]; model != "" {
		return model
	}
	if hardware := values["Hardware"]; hardware != "" {
		return hardware
	}
	if armSummary := formatARMCPUSummary(values); armSummary != "" {
		return armSummary
	}
	return ""
}

// discoverKernel reads the kernel version from /proc/version.
// Returns empty string if the file is absent or cannot be parsed.
func discoverKernel() string {
	data, err := readFileFunc("/proc/version")
	if err != nil {
		return ""
	}
	// Format: "Linux version 6.1.0-28-amd64 (builder@...) ..."
	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) >= 3 {
		return fields[2]
	}
	return ""
}

func inspectHost() (hostInspection, error) {
	identity, err := fetchInstanceIdentity()
	if err != nil {
		return hostInspection{}, err
	}
	osRelease := readOSRelease()
	return hostInspection{
		Architecture:       discoverArchitecture(),
		OSID:               osRelease["ID"],
		OSVersionID:        osRelease["VERSION_ID"],
		Kernel:             discoverKernel(),
		CPU:                discoverCPU(),
		InstanceID:         identity.InstanceID,
		ImageID:            identity.ImageID,
		AvailabilityZone:   identity.AvailabilityZone,
		Region:             identity.Region,
		IIDDocument:        identity.raw,
		IIDSignature:       identity.signature,
		IIDPKCS7:           identity.pkcs7,
		IIDDocumentSHA256:  sha256Hex([]byte(identity.raw)),
		IIDSignatureSHA256: sha256Hex([]byte(identity.signature)),
		IIDPKCS7SHA256:     sha256Hex([]byte(identity.pkcs7)),
		IIDVerified:        false,
	}, nil
}

type rawInstanceIdentityDocument struct {
	instanceIdentityDocument
	raw       string
	signature string
	pkcs7     string
}

func fetchInstanceIdentity() (rawInstanceIdentityDocument, error) {
	token, err := fetchIMDSToken()
	if err != nil {
		return rawInstanceIdentityDocument{}, err
	}
	documentRaw, err := fetchIMDSPath(token, "/latest/dynamic/instance-identity/document")
	if err != nil {
		return rawInstanceIdentityDocument{}, err
	}
	signatureRaw, err := fetchIMDSPath(token, "/latest/dynamic/instance-identity/signature")
	if err != nil {
		return rawInstanceIdentityDocument{}, err
	}
	pkcs7Raw, err := fetchIMDSPath(token, "/latest/dynamic/instance-identity/pkcs7")
	if err != nil {
		return rawInstanceIdentityDocument{}, err
	}
	var doc instanceIdentityDocument
	if err := json.Unmarshal([]byte(documentRaw), &doc); err != nil {
		return rawInstanceIdentityDocument{}, fmt.Errorf("decode instance identity document: %w", err)
	}
	return rawInstanceIdentityDocument{
		instanceIdentityDocument: doc,
		raw:                      strings.TrimSpace(documentRaw),
		signature:                strings.TrimSpace(signatureRaw),
		pkcs7:                    strings.TrimSpace(pkcs7Raw),
	}, nil
}

func fetchIMDSToken() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), imdsRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, imdsAddress+"/latest/api/token", nil)
	if err != nil {
		return "", fmt.Errorf("build imds token request: %w", err)
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", imdsTokenTTL)
	resp, err := imdsHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request imds token: %w", err)
	}
	defer closeBestEffort(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request imds token: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read imds token: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("empty imds token")
	}
	return token, nil
}

func fetchIMDSPath(token, rel string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), imdsRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imdsAddress+rel, nil)
	if err != nil {
		return "", fmt.Errorf("build imds request %s: %w", rel, err)
	}
	req.Header.Set("X-aws-ec2-metadata-token", token)
	resp, err := imdsHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request imds path %s: %w", rel, err)
	}
	defer closeBestEffort(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request imds path %s: unexpected status %s", rel, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read imds path %s: %w", rel, err)
	}
	return string(data), nil
}

func discoverArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

func readOSRelease() map[string]string {
	data, err := readFileFunc("/etc/os-release")
	if err != nil {
		return map[string]string{}
	}
	fields := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return fields
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		closeBestEffort(tmp)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func parseCPUInfoFields(raw string) map[string]string {
	fields := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if _, exists := fields[key]; !exists {
			fields[key] = value
		}
	}
	return fields
}

func formatARMCPUSummary(fields map[string]string) string {
	parts := []string{"ARM"}
	if arch := fields["CPU architecture"]; arch != "" {
		parts = append(parts, "arch "+arch)
	}
	if impl := fields["CPU implementer"]; impl != "" {
		parts = append(parts, "impl "+impl)
	}
	if part := fields["CPU part"]; part != "" {
		parts = append(parts, "part "+part)
	}
	if len(parts) == 1 {
		return ""
	}
	return strings.Join(parts, " ")
}
