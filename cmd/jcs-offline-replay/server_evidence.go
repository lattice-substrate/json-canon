package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const (
	defaultAWSRegion         = "us-east-1"
	serverRepoURL            = "https://github.com/lattice-substrate/json-canon"
	serverProvisionTimeout   = 20 * time.Minute
	serverReleaseGateTimeout = 10 * time.Minute
	serverRuntimeTimeout     = 12 * time.Hour
)

var fullSHAPattern = regexp.MustCompile("^[0-9a-f]{40}$")

type serverEvidenceOptions struct {
	tag               string
	awsRegion         string
	amiLockPath       string
	toolchainLockPath string
	toolchainRoot     string
	hostArch          string
	outputDir         string
	lockFilePath      string
	infraDir          string
	root              string
}

type provisionedHost struct {
	HostID           string `json:"host_id"`
	NodeID           string `json:"node_id"`
	Architecture     string `json:"architecture"`
	PrivateIP        string `json:"private_ip"`
	ImageID          string `json:"image_id"`
	InstanceID       string `json:"instance_id"`
	AvailabilityZone string `json:"availability_zone"`
	InstanceType     string `json:"instance_type"`
	Distro           string `json:"distro"`
	KernelFamily     string `json:"kernel_family"`
}

type provisionedInfra struct {
	Hosts map[string]provisionedHost
}

type discoveredRemoteFacts struct {
	Architecture       string `json:"architecture"`
	OSID               string `json:"os_id"`
	OSVersionID        string `json:"os_version_id"`
	CPU                string `json:"cpu"`
	Kernel             string `json:"kernel"`
	InstanceID         string `json:"instance_id"`
	ImageID            string `json:"image_id"`
	AvailabilityZone   string `json:"availability_zone"`
	Region             string `json:"region"`
	IIDDocumentSHA256  string `json:"iid_document_sha256"`
	IIDSignatureSHA256 string `json:"iid_signature_sha256"`
}

type serverToolchain struct {
	goBinary   string
	tofuBinary string
}

type serverEvidenceRuntime struct {
	ctx               context.Context
	opts              serverEvidenceOptions
	toolchain         serverToolchain
	gitCommit         string
	lockSHA           string
	tofuVersion       string
	awsClients        serverAWSClients
	infra             provisionedInfra
	staging           serverStaging
	hostFacts         map[string]discoveredRemoteFacts
	infraManifestPath string
	x86Artifacts      serverBuildArtifacts
	armArtifacts      serverBuildArtifacts
	sourceRoot        string
	sourceCleanup     func() error
	destroyed         bool
}

func cmdInitInfraLock(flags map[string]string, stdout io.Writer) error {
	infraDir := strings.TrimSpace(flags["--infra-dir"])
	if infraDir == "" {
		infraDir = filepath.Join(resolveRepoRoot(), "infra")
	}
	toolchain, err := resolveServerToolchain()
	if err != nil {
		return err
	}
	if _, err := runCommandInDir(context.Background(), infraDir, nil, toolchain.tofuBinary, "init", "-input=false", "-upgrade=false"); err != nil {
		return err
	}
	return writef(stdout, "infra lock ready: %s\n", filepath.Join(infraDir, ".terraform.lock.hcl"))
}

func cmdServerEvidence(flags map[string]string, stdout io.Writer) error {
	opts, err := parseServerEvidenceOptions(flags)
	if err != nil {
		return err
	}
	return runServerEvidence(opts, stdout)
}

func parseServerEvidenceOptions(flags map[string]string) (serverEvidenceOptions, error) {
	root := resolveRepoRoot()
	required, err := requireServerEvidenceFlags(flags)
	if err != nil {
		return serverEvidenceOptions{}, err
	}
	hostArch, err := resolveToolchainHostArch(flags)
	if err != nil {
		return serverEvidenceOptions{}, err
	}
	outputDir := resolveServerEvidencePath(root, flags["--output-dir"], filepath.Join("offline", "runs", "releases", required.tag))
	toolchainRoot := resolveServerEvidencePath(root, flags["--toolchain-root"], filepath.Join(outputDir, "toolchain"))
	toolchainLockPath := resolveServerEvidencePath(root, flags["--toolchain-lock"], filepath.Join("offline", "toolchain.lock.tsv"))
	return serverEvidenceOptions{
		tag:               required.tag,
		awsRegion:         defaultString(flags, "--aws-region", defaultAWSRegion),
		amiLockPath:       resolveServerEvidencePath(root, flags["--ami-lock"], filepath.Join("infra", "aws_release_hosts.lock.json")),
		toolchainLockPath: toolchainLockPath,
		toolchainRoot:     toolchainRoot,
		hostArch:          hostArch,
		outputDir:         outputDir,
		lockFilePath:      filepath.Join(root, "infra", ".terraform.lock.hcl"),
		infraDir:          filepath.Join(root, "infra"),
		root:              root,
	}, nil
}

func runServerEvidence(opts serverEvidenceOptions, stdout io.Writer) (retErr error) {
	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(signalCtx, serverRuntimeTimeout)
	defer cancel()

	runtimeState, err := newServerEvidenceRuntime(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if runtimeState.sourceCleanup != nil {
			retErr = errors.Join(retErr, runtimeState.sourceCleanup())
		}
	}()
	if err = runtimeState.provision(stdout); err != nil {
		if len(runtimeState.infra.Hosts) != 0 {
			err = errors.Join(err, runtimeState.destroy())
		}
		return err
	}
	defer func() {
		if runtimeState.destroyed {
			return
		}
		retErr = errors.Join(retErr, runtimeState.destroy())
	}()
	return runtimeState.execute(stdout)
}

type requiredServerEvidenceFlags struct {
	tag string
}

func requireServerEvidenceFlags(flags map[string]string) (requiredServerEvidenceFlags, error) {
	required := requiredServerEvidenceFlags{
		tag: requireFlag(flags, "--tag"),
	}
	if required.tag == "" {
		return requiredServerEvidenceFlags{}, fmt.Errorf("server-evidence requires --tag")
	}
	return required, nil
}

func resolveServerEvidencePath(root, rawPath, fallback string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		path = fallback
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func newServerEvidenceRuntime(ctx context.Context, opts serverEvidenceOptions) (*serverEvidenceRuntime, error) {
	if _, err := os.Stat(opts.lockFilePath); err != nil {
		return nil, fmt.Errorf("stat %s: %w", opts.lockFilePath, err)
	}
	if err := validateCleanGitWorktree(ctx, opts.root); err != nil {
		return nil, err
	}
	gitCommit, err := resolveGitHeadCommit(opts.root)
	if err != nil {
		return nil, err
	}
	sourceRoot, cleanupSourceRoot, err := prepareDetachedSourceTree(ctx, opts.root, gitCommit)
	if err != nil {
		return nil, err
	}
	if err := ensureServerOutputDirs(opts.outputDir); err != nil {
		_ = cleanupSourceRoot()
		return nil, err
	}
	toolchain, err := resolveServerToolchain()
	if err != nil {
		_ = cleanupSourceRoot()
		return nil, err
	}
	opts.toolchainLockPath = rebaseDetachedRepoPath(opts.root, sourceRoot, opts.toolchainLockPath)
	opts.amiLockPath = rebaseDetachedRepoPath(opts.root, sourceRoot, opts.amiLockPath)
	opts.infraDir = filepath.Join(sourceRoot, "infra")
	opts.lockFilePath = filepath.Join(opts.infraDir, ".terraform.lock.hcl")
	lockSHA, err := fileSHA256(opts.lockFilePath)
	if err != nil {
		_ = cleanupSourceRoot()
		return nil, fmt.Errorf("sha256 terraform lock: %w", err)
	}
	tofuVersion, err := resolveTofuVersion(toolchain.tofuBinary, opts.infraDir)
	if err != nil {
		_ = cleanupSourceRoot()
		return nil, err
	}
	awsClients, err := newServerAWSClients(ctx, opts.awsRegion)
	if err != nil {
		_ = cleanupSourceRoot()
		return nil, err
	}
	return &serverEvidenceRuntime{
		ctx:           ctx,
		opts:          opts,
		toolchain:     toolchain,
		gitCommit:     gitCommit,
		lockSHA:       lockSHA,
		tofuVersion:   tofuVersion,
		awsClients:    awsClients,
		hostFacts:     make(map[string]discoveredRemoteFacts),
		sourceRoot:    sourceRoot,
		sourceCleanup: cleanupSourceRoot,
	}, nil
}

func ensureServerOutputDirs(outputDir string) error {
	for _, arch := range []string{"x86_64", "arm64"} {
		if err := os.MkdirAll(filepath.Join(outputDir, arch), dirPerm); err != nil {
			return fmt.Errorf("create %s output dir: %w", arch, err)
		}
	}
	return nil
}

type serverBuildArtifacts struct {
	controlBinaryPath string
	workerPath        string
	bundlePath        string
}

type serverMatrixRun struct {
	matrixPath        string
	profilePath       string
	bundlePath        string
	controlBinaryPath string
	evidencePath      string
	infraManifestPath string
	sourceGitCommit   string
	sourceGitTag      string
	awsClients        serverAWSClients
	stagingBucket     string
	stagedArtifacts   stagedServerArtifacts
	hosts             map[string]provisionedHost
}

type releaseGateRun struct {
	evidencePath      string
	bundlePath        string
	matrixPath        string
	profilePath       string
	controlBinaryPath string
	expectedCommit    string
	expectedTag       string
	infraManifestPath string
}

func (r *serverEvidenceRuntime) provision(stdout io.Writer) error {
	if err := writef(stdout, "==> provisioning infrastructure (tag=%s, commit=%s)\n", r.opts.tag, r.gitCommit[:12]); err != nil {
		return err
	}
	infra, err := provisionServerInfrastructure(r.ctx, r.opts, r.toolchain, r.gitCommit, r.lockSHA)
	if err != nil {
		return err
	}
	r.infra = infra
	if err := writef(stdout, "==> instances ready: %d official AWS hosts\n", len(infra.Hosts)); err != nil {
		return err
	}
	if err := writeLine(stdout, "==> waiting for SSM on all provisioned hosts"); err != nil {
		return err
	}
	return waitForSSMManagedInstances(r.ctx, r.awsClients, infra.Hosts, serverSSMReadyTimeout)
}

func (r *serverEvidenceRuntime) execute(stdout io.Writer) error {
	if err := r.buildArtifacts(stdout); err != nil {
		return err
	}
	if err := r.prepareStaging(stdout); err != nil {
		return err
	}
	if err := r.discoverRemoteFacts(stdout); err != nil {
		return err
	}
	if err := r.writeInfraManifest(stdout); err != nil {
		return err
	}
	if err := r.runReplays(stdout); err != nil {
		return err
	}
	if err := r.runReleaseGates(stdout); err != nil {
		return err
	}
	if err := r.destroy(); err != nil {
		return err
	}
	return r.writeSuccess(stdout)
}

func (r *serverEvidenceRuntime) discoverRemoteFacts(stdout io.Writer) error {
	if err := writeLine(stdout, "==> discovering native substrate identity on provisioned hosts"); err != nil {
		return err
	}
	for _, hostID := range sortedProvisionedHostIDs(r.infra.Hosts) {
		host := r.infra.Hosts[hostID]
		facts, err := r.discoverHostFacts(host)
		if err != nil {
			return err
		}
		if err := validateDiscoveredRemoteFacts(hostID, facts); err != nil {
			return err
		}
		r.hostFacts[hostID] = facts
	}
	return writeDiscoveredFactSummary(stdout, r.infra.Hosts, r.hostFacts)
}

func validateDiscoveredRemoteFacts(hostID string, facts discoveredRemoteFacts) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"architecture", facts.Architecture},
		{"os_id", facts.OSID},
		{"os_version_id", facts.OSVersionID},
		{"cpu", facts.CPU},
		{"kernel", facts.Kernel},
		{"instance_id", facts.InstanceID},
		{"image_id", facts.ImageID},
		{"availability_zone", facts.AvailabilityZone},
		{"region", facts.Region},
		{"iid_document_sha256", facts.IIDDocumentSHA256},
		{"iid_signature_sha256", facts.IIDSignatureSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("failed to discover required remote fact: %s %s", hostID, field.name)
		}
	}
	return nil
}

func writeDiscoveredFactSummary(stdout io.Writer, hosts map[string]provisionedHost, facts map[string]discoveredRemoteFacts) error {
	for _, hostID := range sortedProvisionedHostIDs(hosts) {
		hostFacts := facts[hostID]
		line := fmt.Sprintf(
			"    %s (%s): os=%s/%s cpu=%s kernel=%s instance=%s image=%s",
			hostID,
			hosts[hostID].Architecture,
			hostFacts.OSID,
			hostFacts.OSVersionID,
			hostFacts.CPU,
			hostFacts.Kernel,
			hostFacts.InstanceID,
			hostFacts.ImageID,
		)
		if err := writeLine(stdout, line); err != nil {
			return err
		}
	}
	return nil
}

func (r *serverEvidenceRuntime) writeInfraManifest(stdout io.Writer) error {
	r.infraManifestPath = filepath.Join(r.opts.outputDir, "infra-manifest.v1.json")
	if err := writeLine(stdout, "==> writing infra manifest"); err != nil {
		return err
	}
	tools, err := collectToolchainEvidence(map[string]string{
		"--toolchain-lock": r.opts.toolchainLockPath,
		"--toolchain-root": r.opts.toolchainRoot,
		"--host-arch":      r.opts.hostArch,
		"--purposes":       "build,provision",
	}, r.infraManifestPath)
	if err != nil {
		return err
	}
	return writeInfraManifestDocument(r.infraManifestPath, &replay.InfraManifest{
		SchemaVersion:      replay.InfraManifestSchemaVersion,
		GeneratedAtUTC:     manifestNowUTC().Format(time.RFC3339Nano),
		InfraRepoURL:       serverRepoURL,
		InfraRepoCommit:    r.gitCommit,
		ProviderEngine:     "opentofu",
		ProviderVersion:    r.tofuVersion,
		ProviderLockSHA256: r.lockSHA,
		Hosts:              buildProvisionedInfraManifestHosts(r.infra.Hosts, r.hostFacts, r.opts.awsRegion),
		Tools:              tools,
	})
}

func (r *serverEvidenceRuntime) buildArtifacts(stdout io.Writer) error {
	if err := writeLine(stdout, "==> building architecture-specific jcs-canon control binaries"); err != nil {
		return err
	}
	x86Artifacts, err := buildServerRunArtifacts(r.opts, r.sourceRoot, r.toolchain, "x86_64")
	if err != nil {
		return err
	}
	armArtifacts, err := buildServerRunArtifacts(r.opts, r.sourceRoot, r.toolchain, "arm64")
	if err != nil {
		return err
	}
	r.x86Artifacts = x86Artifacts
	r.armArtifacts = armArtifacts
	return nil
}

func (r *serverEvidenceRuntime) runReplays(stdout io.Writer) error {
	if err := writeLine(stdout, "==> running x86_64 replay"); err != nil {
		return err
	}
	if err := r.runReplayForArch(stdout, "x86_64"); err != nil {
		return err
	}
	if err := writeLine(stdout, "==> running arm64 replay"); err != nil {
		return err
	}
	return r.runReplayForArch(stdout, "arm64")
}

func (r *serverEvidenceRuntime) runReplayForArch(stdout io.Writer, arch string) error {
	cfg := r.serverMatrixRunForArch(arch)
	return runServerMatrix(r.ctx, cfg, stdout)
}

func (r *serverEvidenceRuntime) runReleaseGates(stdout io.Writer) error {
	for _, arch := range []string{"x86_64", "arm64"} {
		if err := writef(stdout, "==> running release gate: %s\n", arch); err != nil {
			return err
		}
		if err := runServerReleaseGate(r.ctx, r.toolchain.goBinary, r.opts.root, r.releaseGateRunForArch(arch)); err != nil {
			return err
		}
	}
	return nil
}

func (r *serverEvidenceRuntime) destroy() error {
	if r.destroyed {
		return nil
	}
	if err := deleteStagingBucket(context.WithoutCancel(r.ctx), r.awsClients, r.staging.bucket); err != nil {
		return err
	}
	if err := destroyServerInfrastructure(r.ctx, r.opts, r.toolchain, r.gitCommit, r.lockSHA); err != nil {
		return err
	}
	r.destroyed = true
	return nil
}

func (r *serverEvidenceRuntime) writeSuccess(stdout io.Writer) error {
	if err := writeLine(stdout, ""); err != nil {
		return err
	}
	if err := writef(stdout, "==> SUCCESS: server evidence written to %s\n", r.opts.outputDir); err != nil {
		return err
	}
	if err := writef(stdout, "    x86_64 evidence: %s\n", filepath.Join(r.opts.outputDir, "x86_64", "offline-evidence.json")); err != nil {
		return err
	}
	if err := writef(stdout, "    arm64 evidence:  %s\n", filepath.Join(r.opts.outputDir, "arm64", "offline-evidence.json")); err != nil {
		return err
	}
	return writef(stdout, "    infra manifest:  %s\n", r.infraManifestPath)
}

func (r *serverEvidenceRuntime) serverMatrixRunForArch(arch string) serverMatrixRun {
	artifacts := r.artifactsForArch(arch)
	return serverMatrixRun{
		matrixPath:        filepath.Join(r.sourceRoot, "offline", "matrix.server-"+arch+".yaml"),
		profilePath:       filepath.Join(r.sourceRoot, "offline", "profiles", "server-linux-"+arch+".yaml"),
		bundlePath:        artifacts.bundlePath,
		controlBinaryPath: artifacts.controlBinaryPath,
		evidencePath:      filepath.Join(r.opts.outputDir, arch, "offline-evidence.json"),
		infraManifestPath: r.infraManifestPath,
		sourceGitCommit:   r.gitCommit,
		sourceGitTag:      r.opts.tag,
		awsClients:        r.awsClients,
		stagingBucket:     r.staging.bucket,
		stagedArtifacts:   r.stagingArtifactsForArch(arch),
		hosts:             hostsForArchitecture(r.infra.Hosts, arch),
	}
}

func (r *serverEvidenceRuntime) releaseGateRunForArch(arch string) releaseGateRun {
	artifacts := r.artifactsForArch(arch)
	return releaseGateRun{
		evidencePath:      filepath.Join(r.opts.outputDir, arch, "offline-evidence.json"),
		bundlePath:        artifacts.bundlePath,
		matrixPath:        filepath.Join(r.sourceRoot, "offline", "matrix.server-"+arch+".yaml"),
		profilePath:       filepath.Join(r.sourceRoot, "offline", "profiles", "server-linux-"+arch+".yaml"),
		controlBinaryPath: artifacts.controlBinaryPath,
		expectedCommit:    r.gitCommit,
		expectedTag:       r.opts.tag,
		infraManifestPath: r.infraManifestPath,
	}
}

func (r *serverEvidenceRuntime) artifactsForArch(arch string) serverBuildArtifacts {
	if arch == matrixArchitectureARM64 {
		return r.armArtifacts
	}
	return r.x86Artifacts
}

func buildServerRunArtifacts(opts serverEvidenceOptions, sourceRoot string, toolchain serverToolchain, matrixArch string) (serverBuildArtifacts, error) {
	archDir := filepath.Join(opts.outputDir, matrixArch)
	controlBinaryPath := filepath.Join(archDir, "jcs-canon")
	workerPath := filepath.Join(archDir, "jcs-offline-worker")
	bundlePath := filepath.Join(archDir, "offline-bundle.tgz")
	if err := buildGoBinary(toolchain.goBinary, sourceRoot, matrixArch, opts.tag, "./cmd/jcs-canon", controlBinaryPath); err != nil {
		return serverBuildArtifacts{}, err
	}
	if err := buildGoBinary(toolchain.goBinary, sourceRoot, matrixArch, "v0.0.0-dev", "./cmd/jcs-offline-worker", workerPath); err != nil {
		return serverBuildArtifacts{}, err
	}
	matrixPath := filepath.Join(sourceRoot, "offline", "matrix.server-"+matrixArch+".yaml")
	profilePath := filepath.Join(sourceRoot, "offline", "profiles", "server-linux-"+matrixArch+".yaml")
	matrix, err := replay.LoadMatrix(matrixPath)
	if err != nil {
		return serverBuildArtifacts{}, fmt.Errorf("load matrix: %w", err)
	}
	if _, err := replay.LoadProfile(profilePath); err != nil {
		return serverBuildArtifacts{}, fmt.Errorf("load profile: %w", err)
	}
	if err := validateExecutableArchitecture(controlBinaryPath, matrix.Architecture, "control binary"); err != nil {
		return serverBuildArtifacts{}, err
	}
	if err := validateExecutableArchitecture(workerPath, matrix.Architecture, "worker binary"); err != nil {
		return serverBuildArtifacts{}, err
	}
	if _, err := replay.CreateBundle(replay.BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  controlBinaryPath,
		WorkerPath:  workerPath,
		MatrixPath:  matrixPath,
		ProfilePath: profilePath,
		VectorsGlob: "conformance/vectors/*.jsonl",
		Version:     "bundle.v1",
	}); err != nil {
		return serverBuildArtifacts{}, fmt.Errorf("create bundle: %w", err)
	}
	return serverBuildArtifacts{
		controlBinaryPath: controlBinaryPath,
		workerPath:        workerPath,
		bundlePath:        bundlePath,
	}, nil
}

func runServerMatrix(ctx context.Context, cfg serverMatrixRun, stdout io.Writer) error {
	matrix, profile, manifest, bundleSHA, matrixSHA, profileSHA, err := loadRunInputs(cfg.matrixPath, cfg.profilePath, cfg.bundlePath)
	if err != nil {
		return fmt.Errorf("load run inputs: %w", err)
	}
	infraManifest, err := replay.LoadInfraManifest(cfg.infraManifestPath)
	if err != nil {
		return fmt.Errorf("load infra manifest: %w", err)
	}
	infraManifestSHA, err := fileSHA256(cfg.infraManifestPath)
	if err != nil {
		return fmt.Errorf("sha256 infra manifest: %w", err)
	}
	controlBinarySHA, err := fileSHA256(cfg.controlBinaryPath)
	if err != nil {
		return fmt.Errorf("sha256 control binary: %w", err)
	}
	runCtx, cancel := context.WithTimeout(ctx, serverRuntimeTimeout)
	defer cancel()
	evidence, err := replay.RunMatrix(runCtx, matrix, profile, newServerSSMAdapterFactory(cfg.awsClients, cfg.stagingBucket, cfg.stagedArtifacts, cfg.hosts), replay.RunOptions{
		BundlePath:            cfg.bundlePath,
		BundleSHA256:          bundleSHA,
		ControlBinarySHA256:   manifest.BinarySHA256,
		MatrixSHA256:          matrixSHA,
		ProfileSHA256:         profileSHA,
		SourceGitCommit:       cfg.sourceGitCommit,
		SourceGitTag:          cfg.sourceGitTag,
		Orchestrator:          "jcs-offline-replay server-evidence",
		EvidenceSchemaVersion: replay.EvidenceSchemaVersion,
		InfraManifestSHA256:   infraManifestSHA,
		InfraRepoURL:          serverRepoURL,
		InfraRepoCommit:       cfg.sourceGitCommit,
		InfraManifest:         infraManifest,
	})
	if err != nil {
		return fmt.Errorf("run replay matrix: %w", err)
	}
	if evidence.ControlBinarySHA != controlBinarySHA {
		return fmt.Errorf("evidence control binary sha mismatch: got=%s want=%s", evidence.ControlBinarySHA, controlBinarySHA)
	}
	if err := replay.WriteEvidence(cfg.evidencePath, evidence); err != nil {
		return fmt.Errorf("write evidence: %w", err)
	}
	return writeRunSummary(stdout, cfg.evidencePath, evidence)
}

func runServerReleaseGate(parent context.Context, goBinary, repoRoot string, cfg releaseGateRun) error {
	ctx, cancel := context.WithTimeout(parent, serverReleaseGateTimeout)
	defer cancel()
	env := map[string]string{
		"JCS_OFFLINE_EVIDENCE":            cfg.evidencePath,
		"JCS_OFFLINE_BUNDLE":              cfg.bundlePath,
		"JCS_OFFLINE_MATRIX":              cfg.matrixPath,
		"JCS_OFFLINE_PROFILE":             cfg.profilePath,
		"JCS_OFFLINE_CONTROL_BINARY":      cfg.controlBinaryPath,
		"JCS_OFFLINE_EXPECTED_GIT_COMMIT": cfg.expectedCommit,
		"JCS_OFFLINE_EXPECTED_GIT_TAG":    cfg.expectedTag,
		"JCS_OFFLINE_INFRA_MANIFEST":      cfg.infraManifestPath,
	}
	_, err := runCommandInDir(ctx, repoRoot, env, goBinary, "test", "-mod=readonly", "./offline/conformance", "-run", "TestOfflineReplayEvidenceReleaseGate", "-count=1", "-v")
	return err
}

func provisionServerInfrastructure(ctx context.Context, opts serverEvidenceOptions, toolchain serverToolchain, gitCommit, lockSHA string) (provisionedInfra, error) {
	ctx, cancel := context.WithTimeout(ctx, serverProvisionTimeout)
	defer cancel()
	if _, err := runCommandInDir(ctx, opts.infraDir, nil, toolchain.tofuBinary, "init", "-input=false", "-upgrade=false"); err != nil {
		return provisionedInfra{}, err
	}
	args := append([]string{"apply", "-auto-approve", "-input=false"}, tofuVarArgs(gitCommit, lockSHA, opts.awsRegion, opts.amiLockPath)...)
	if _, err := runCommandInDir(ctx, opts.infraDir, nil, toolchain.tofuBinary, args...); err != nil {
		return provisionedInfra{}, err
	}
	hosts, err := tofuOutputHosts(toolchain.tofuBinary, opts.infraDir, "provisioned_hosts")
	if err != nil {
		return provisionedInfra{}, err
	}
	return provisionedInfra{Hosts: hosts}, nil
}

func destroyServerInfrastructure(ctx context.Context, opts serverEvidenceOptions, toolchain serverToolchain, gitCommit, lockSHA string) error {
	ctx, cancel := context.WithTimeout(ctx, serverProvisionTimeout)
	defer cancel()
	args := append([]string{"destroy", "-auto-approve", "-input=false"}, tofuVarArgs(gitCommit, lockSHA, opts.awsRegion, opts.amiLockPath)...)
	_, err := runCommandInDir(ctx, opts.infraDir, nil, toolchain.tofuBinary, args...)
	return err
}

func tofuVarArgs(gitCommit, lockSHA, awsRegion, amiLockPath string) []string {
	return []string{
		"-var", "infra_repo_url=" + serverRepoURL,
		"-var", "infra_repo_commit=" + gitCommit,
		"-var", "provider_lock_sha256=" + lockSHA,
		"-var", "aws_region=" + awsRegion,
		"-var", "aws_release_host_lock_path=" + amiLockPath,
	}
}

func tofuOutputHosts(tofuBinary, infraDir, name string) (map[string]provisionedHost, error) {
	out, err := runCommandInDir(context.Background(), infraDir, nil, tofuBinary, "output", "-json", name)
	if err != nil {
		return nil, err
	}
	var hosts map[string]provisionedHost
	if err := json.Unmarshal([]byte(out), &hosts); err != nil {
		return nil, fmt.Errorf("decode tofu output %s: %w", name, err)
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("tofu output %s is empty", name)
	}
	for _, hostID := range sortedProvisionedHostIDs(hosts) {
		host := hosts[hostID]
		if strings.TrimSpace(host.HostID) == "" {
			host.HostID = hostID
		}
		if host.HostID != hostID {
			return nil, fmt.Errorf("tofu output %s host id mismatch: map key=%s payload=%s", name, hostID, host.HostID)
		}
		hosts[hostID] = host
	}
	return hosts, nil
}

func sortedProvisionedHostIDs(hosts map[string]provisionedHost) []string {
	ids := make([]string, 0, len(hosts))
	for hostID := range hosts {
		ids = append(ids, hostID)
	}
	sort.Strings(ids)
	return ids
}

func hostsForArchitecture(hosts map[string]provisionedHost, arch string) map[string]provisionedHost {
	filtered := make(map[string]provisionedHost)
	for _, hostID := range sortedProvisionedHostIDs(hosts) {
		host := hosts[hostID]
		if host.Architecture != arch {
			continue
		}
		filtered[host.NodeID] = host
	}
	return filtered
}

func buildProvisionedInfraManifestHosts(hosts map[string]provisionedHost, facts map[string]discoveredRemoteFacts, region string) []replay.InfraManifestHost {
	manifestHosts := make([]replay.InfraManifestHost, 0, len(hosts))
	for _, hostID := range sortedProvisionedHostIDs(hosts) {
		host := hosts[hostID]
		hostFacts := facts[hostID]
		manifestHosts = append(manifestHosts, replay.InfraManifestHost{
			Role:               host.HostID,
			Architecture:       host.Architecture,
			NodeIDs:            []string{host.NodeID},
			CloudProvider:      "aws",
			Region:             firstNonEmpty(hostFacts.Region, region),
			AvailabilityZone:   firstNonEmpty(hostFacts.AvailabilityZone, host.AvailabilityZone),
			InstanceType:       host.InstanceType,
			InstanceID:         firstNonEmpty(hostFacts.InstanceID, host.InstanceID),
			ImageID:            firstNonEmpty(hostFacts.ImageID, host.ImageID),
			OSID:               hostFacts.OSID,
			OSVersionID:        hostFacts.OSVersionID,
			CPU:                hostFacts.CPU,
			Kernel:             hostFacts.Kernel,
			IIDDocumentSHA256:  hostFacts.IIDDocumentSHA256,
			IIDSignatureSHA256: hostFacts.IIDSignatureSHA256,
			Transport:          "ssm",
			SubnetVisibility:   "private",
			DiscoveredCPU:      hostFacts.CPU,
			DiscoveredKernel:   hostFacts.Kernel,
		})
	}
	return manifestHosts
}

func resolveServerToolchain() (serverToolchain, error) {
	info := serverToolchain{
		goBinary:   lookupEnvTrimmed("JCS_TOOL_GO"),
		tofuBinary: lookupEnvTrimmed("JCS_TOOL_TOFU"),
	}
	for _, item := range []struct {
		label string
		path  string
	}{
		{"JCS_TOOL_GO", info.goBinary},
		{"JCS_TOOL_TOFU", info.tofuBinary},
	} {
		if strings.TrimSpace(item.path) == "" {
			return serverToolchain{}, fmt.Errorf("%s is required; run scripts/bootstrap-pinned-toolchain.sh first", item.label)
		}
		if _, err := os.Stat(item.path); err != nil {
			return serverToolchain{}, fmt.Errorf("stat %s %s: %w", item.label, item.path, err)
		}
	}
	return info, nil
}

func resolveTofuVersion(tofuBinary, infraDir string) (string, error) {
	out, err := runCommandInDir(context.Background(), infraDir, nil, tofuBinary, "version")
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`v([0-9]+\.[0-9]+\.[0-9]+)`)
	match := re.FindStringSubmatch(out)
	if len(match) != 2 {
		return "", fmt.Errorf("parse opentofu version from %q", strings.TrimSpace(out))
	}
	return match[1], nil
}

func buildGoBinary(goBinary, repoRoot, matrixArch, version, pkgPath, outPath string) error {
	goArch, err := goArchForMatrixArch(matrixArch)
	if err != nil {
		return err
	}
	if mkErr := os.MkdirAll(filepath.Dir(outPath), dirPerm); mkErr != nil {
		return fmt.Errorf("create output dir for %s: %w", outPath, mkErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_, err = runCommandInDir(ctx, repoRoot, map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        "linux",
		"GOARCH":      goArch,
	}, goBinary, "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid= -X main.version="+version, "-o", outPath, pkgPath)
	if err != nil {
		return fmt.Errorf("build %s: %w", pkgPath, err)
	}
	return nil
}

func validateCleanGitWorktree(ctx context.Context, root string) error {
	out, err := runCommandInDir(ctx, root, nil, "git", "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return fmt.Errorf("check git worktree cleanliness: %w", err)
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("server-evidence requires a clean git worktree")
	}
	return nil
}

func prepareDetachedSourceTree(ctx context.Context, root, commit string) (string, func() error, error) {
	sourceRoot, err := os.MkdirTemp("", "jcs-offline-source-*")
	if err != nil {
		return "", nil, fmt.Errorf("create detached source root: %w", err)
	}
	cleanup := func() error {
		_, removeErr := runCommandInDir(context.Background(), root, nil, "git", "worktree", "remove", "--force", sourceRoot)
		fileErr := os.RemoveAll(sourceRoot)
		return errors.Join(removeErr, fileErr)
	}
	if _, err := runCommandInDir(ctx, root, nil, "git", "worktree", "add", "--detach", sourceRoot, commit); err != nil {
		_ = os.RemoveAll(sourceRoot)
		return "", nil, fmt.Errorf("create detached source worktree: %w", err)
	}
	return sourceRoot, cleanup, nil
}

func rebaseDetachedRepoPath(root, detachedRoot, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return path
	}
	return filepath.Join(detachedRoot, rel)
}

func resolveGitHeadCommit(root string) (string, error) {
	gitDir, err := resolveGitDir(root)
	if err != nil {
		return "", err
	}
	headPath := filepath.Join(gitDir, "HEAD")
	head, err := readTrimmedFile(headPath)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(head, "ref: ") {
		return resolveDetachedHeadCommit(head)
	}
	ref := strings.TrimSpace(strings.TrimPrefix(head, "ref: "))
	return resolveGitRefCommit(gitDir, ref)
}

func resolveGitDir(root string) (string, error) {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Stat(dotGit)
	if err == nil && info.IsDir() {
		return dotGit, nil
	}
	if err == nil && !info.IsDir() {
		raw, readErr := readTrimmedFile(dotGit)
		if readErr != nil {
			return "", readErr
		}
		const prefix = "gitdir: "
		if !strings.HasPrefix(raw, prefix) {
			return "", fmt.Errorf("resolve gitdir from %s", dotGit)
		}
		gitDir := strings.TrimSpace(strings.TrimPrefix(raw, prefix))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(root, gitDir)
		}
		return gitDir, nil
	}
	return "", fmt.Errorf("unable to resolve source commit without .git metadata")
}

func validFullSHA(raw string) bool {
	return fullSHAPattern.MatchString(strings.TrimSpace(raw))
}

//nolint:gosec // REQ:OFFLINE-AUTO-001 server evidence reads explicit git metadata and operator public-key inputs from validated paths.
func readTrimmedFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func runCommandInDir(ctx context.Context, dir string, env map[string]string, name string, args ...string) (string, error) {
	// #nosec G204 -- server evidence orchestration executes pinned binaries with explicit arguments.
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if len(env) != 0 {
		merged := cmd.Environ()
		for key, value := range env {
			merged = append(merged, key+"="+value)
		}
		cmd.Env = merged
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg != "" {
			return "", fmt.Errorf("run %s %s failed: %w: %s", name, strings.Join(args, " "), err, msg)
		}
		return "", fmt.Errorf("run %s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return out.String(), nil
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
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

func sha256HexString(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func randomSuffix() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "fallback"
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var out strings.Builder
	for _, b := range buf {
		out.WriteByte(alphabet[int(b)%len(alphabet)])
	}
	return out.String()
}

func init() {
	var _ replay.NodeAdapter = (*serverSSMAdapter)(nil)
}

func resolveDetachedHeadCommit(head string) (string, error) {
	if validFullSHA(head) {
		return head, nil
	}
	return "", fmt.Errorf("invalid detached HEAD sha %q", head)
}

func resolveGitRefCommit(gitDir, ref string) (string, error) {
	commit, err := resolveLooseGitRef(gitDir, ref)
	if err == nil {
		return commit, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	commit, err = resolvePackedGitRef(gitDir, ref)
	if err != nil {
		return "", err
	}
	return commit, nil
}

func resolveLooseGitRef(gitDir, ref string) (string, error) {
	refPath := filepath.Join(gitDir, filepath.FromSlash(ref))
	commit, err := readTrimmedFile(refPath)
	if err != nil {
		return "", err
	}
	if !validFullSHA(commit) {
		return "", fmt.Errorf("invalid ref sha %q", commit)
	}
	return commit, nil
}

//nolint:gosec // REQ:OFFLINE-AUTO-001 git packed-refs is repository-controlled metadata resolved under the source checkout.
func resolvePackedGitRef(gitDir, ref string) (string, error) {
	packedRefsPath := filepath.Join(gitDir, "packed-refs")
	data, err := os.ReadFile(packedRefsPath)
	if err != nil {
		return "", fmt.Errorf("read packed refs: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == ref && validFullSHA(fields[0]) {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("resolve git head commit from %s", ref)
}

func closeBestEffort(c io.Closer) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		_ = err
	}
}
