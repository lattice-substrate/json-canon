package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const (
	defaultAWSRegion               = "us-east-1"
	defaultServerContainerImageTag = "debian:13-slim"
	defaultSSHUser                 = "admin"
	defaultSSHPort                 = "22"
	serverRepoURL                  = "https://github.com/lattice-substrate/json-canon"
	serverSSHReadyTimeout          = 3 * time.Minute
	serverSSHConnectTimeout        = 15 * time.Second
	serverProvisionTimeout         = 20 * time.Minute
	serverReleaseGateTimeout       = 10 * time.Minute
	serverRuntimeTimeout           = 12 * time.Hour
)

var fullSHAPattern = regexp.MustCompile("^[0-9a-f]{40}$")

type serverEvidenceOptions struct {
	tag                     string
	awsRegion               string
	sshIngressCIDR          string
	sshKeyPath              string
	sshPublicKeyPath        string
	toolchainLockPath       string
	toolchainRoot           string
	hostArch                string
	outputDir               string
	serverContainerImageTag string
	lockFilePath            string
	infraDir                string
	root                    string
}

type provisionedHost struct {
	Architecture string
	PublicIP     string
	ImageID      string
	InstanceType string
}

type provisionedInfra struct {
	X86   provisionedHost
	Arm64 provisionedHost
}

type discoveredRemoteFacts struct {
	CPU            string
	Kernel         string
	ContainerImage string
}

type serverToolchain struct {
	goBinary          string
	tofuBinary        string
	dockerStaticAMD64 string
	dockerStaticARM64 string
}

type serverSSHRunner struct {
	signer         ssh.Signer
	connectTimeout time.Duration
	hostKeyMu      sync.Mutex
	hostKeys       map[string]string
}

type serverRemoteAdapter struct {
	ssh        *serverSSHRunner
	workerPath string
}

type serverEvidenceRuntime struct {
	opts              serverEvidenceOptions
	toolchain         serverToolchain
	gitCommit         string
	lockSHA           string
	tofuVersion       string
	sshPublicKey      string
	infra             provisionedInfra
	sshRunner         *serverSSHRunner
	x86Target         string
	armTarget         string
	x86Facts          discoveredRemoteFacts
	armFacts          discoveredRemoteFacts
	infraManifestPath string
	x86Artifacts      serverBuildArtifacts
	armArtifacts      serverBuildArtifacts
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
	absKeyPath, err := filepath.Abs(required.sshKeyPath)
	if err != nil {
		return serverEvidenceOptions{}, fmt.Errorf("resolve ssh key path: %w", err)
	}
	sshPubPath, err := resolveSSHPublicKeyPath(absKeyPath)
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
		tag:                     required.tag,
		awsRegion:               defaultString(flags, "--aws-region", defaultAWSRegion),
		sshIngressCIDR:          required.sshIngressCIDR,
		sshKeyPath:              absKeyPath,
		sshPublicKeyPath:        sshPubPath,
		toolchainLockPath:       toolchainLockPath,
		toolchainRoot:           toolchainRoot,
		hostArch:                hostArch,
		outputDir:               outputDir,
		serverContainerImageTag: defaultString(flags, "--server-container-image-tag", defaultServerContainerImageTag),
		lockFilePath:            filepath.Join(root, "infra", ".terraform.lock.hcl"),
		infraDir:                filepath.Join(root, "infra"),
		root:                    root,
	}, nil
}

func runServerEvidence(opts serverEvidenceOptions, stdout io.Writer) (retErr error) {
	runtimeState, err := newServerEvidenceRuntime(opts)
	if err != nil {
		return err
	}
	if err = runtimeState.provision(stdout); err != nil {
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
	tag            string
	sshKeyPath     string
	sshIngressCIDR string
}

func requireServerEvidenceFlags(flags map[string]string) (requiredServerEvidenceFlags, error) {
	required := requiredServerEvidenceFlags{
		tag:            requireFlag(flags, "--tag"),
		sshKeyPath:     requireFlag(flags, "--ssh-key-path"),
		sshIngressCIDR: requireFlag(flags, "--ssh-ingress-cidr"),
	}
	if required.tag == "" || required.sshKeyPath == "" || required.sshIngressCIDR == "" {
		return requiredServerEvidenceFlags{}, fmt.Errorf("server-evidence requires --tag, --ssh-key-path, and --ssh-ingress-cidr")
	}
	return required, nil
}

func resolveSSHPublicKeyPath(absKeyPath string) (string, error) {
	sshPubPath := absKeyPath + ".pub"
	if _, err := os.Stat(sshPubPath); err != nil {
		return "", fmt.Errorf("stat ssh public key %s: %w", sshPubPath, err)
	}
	return sshPubPath, nil
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

func newServerEvidenceRuntime(opts serverEvidenceOptions) (*serverEvidenceRuntime, error) {
	if _, err := os.Stat(opts.lockFilePath); err != nil {
		return nil, fmt.Errorf("stat %s: %w", opts.lockFilePath, err)
	}
	if err := ensureServerOutputDirs(opts.outputDir); err != nil {
		return nil, err
	}
	toolchain, err := resolveServerToolchain()
	if err != nil {
		return nil, err
	}
	gitCommit, err := resolveGitHeadCommit(opts.root)
	if err != nil {
		return nil, err
	}
	lockSHA, err := fileSHA256(opts.lockFilePath)
	if err != nil {
		return nil, fmt.Errorf("sha256 terraform lock: %w", err)
	}
	tofuVersion, err := resolveTofuVersion(toolchain.tofuBinary, opts.infraDir)
	if err != nil {
		return nil, err
	}
	sshPublicKey, err := readTrimmedFile(opts.sshPublicKeyPath)
	if err != nil {
		return nil, err
	}
	return &serverEvidenceRuntime{
		opts:         opts,
		toolchain:    toolchain,
		gitCommit:    gitCommit,
		lockSHA:      lockSHA,
		tofuVersion:  tofuVersion,
		sshPublicKey: sshPublicKey,
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
	workerPath        string
	evidencePath      string
	infraManifestPath string
	sourceGitCommit   string
	sourceGitTag      string
	target            string
	containerImage    string
	sshRunner         *serverSSHRunner
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
	infra, err := provisionServerInfrastructure(r.opts, r.toolchain, r.gitCommit, r.lockSHA, r.sshPublicKey)
	if err != nil {
		return err
	}
	sshRunner, err := newServerSSHRunner(r.opts.sshKeyPath)
	if err != nil {
		return err
	}
	r.infra = infra
	r.sshRunner = sshRunner
	r.x86Target = defaultSSHUser + "@" + infra.X86.PublicIP
	r.armTarget = defaultSSHUser + "@" + infra.Arm64.PublicIP
	if err := writef(stdout, "==> instances ready: x86_64=%s  arm64=%s\n", infra.X86.PublicIP, infra.Arm64.PublicIP); err != nil {
		return err
	}
	if err := writeLine(stdout, "==> waiting for SSH on both instances"); err != nil {
		return err
	}
	if err := r.sshRunner.Wait(context.Background(), r.x86Target, serverSSHReadyTimeout); err != nil {
		return fmt.Errorf("wait for x86_64 ssh: %w", err)
	}
	if err := r.sshRunner.Wait(context.Background(), r.armTarget, serverSSHReadyTimeout); err != nil {
		return fmt.Errorf("wait for arm64 ssh: %w", err)
	}
	return nil
}

func (r *serverEvidenceRuntime) execute(stdout io.Writer) error {
	if err := r.prepareRemoteRuntime(stdout); err != nil {
		return err
	}
	if err := r.discoverRemoteFacts(stdout); err != nil {
		return err
	}
	if err := r.writeInfraManifest(stdout); err != nil {
		return err
	}
	if err := r.buildArtifacts(stdout); err != nil {
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

func (r *serverEvidenceRuntime) prepareRemoteRuntime(stdout io.Writer) error {
	if err := writeLine(stdout, "==> preparing remote container runtime"); err != nil {
		return err
	}
	if err := installRemoteContainerRuntime(context.Background(), r.sshRunner, r.x86Target, r.toolchain.dockerStaticAMD64); err != nil {
		return err
	}
	return installRemoteContainerRuntime(context.Background(), r.sshRunner, r.armTarget, r.toolchain.dockerStaticARM64)
}

func (r *serverEvidenceRuntime) discoverRemoteFacts(stdout io.Writer) error {
	if err := writeLine(stdout, "==> resolving digest-pinned container images on provisioned hosts"); err != nil {
		return err
	}
	x86Facts, err := discoverRemoteFacts(context.Background(), r.sshRunner, r.x86Target, r.opts.serverContainerImageTag)
	if err != nil {
		return err
	}
	armFacts, err := discoverRemoteFacts(context.Background(), r.sshRunner, r.armTarget, r.opts.serverContainerImageTag)
	if err != nil {
		return err
	}
	if err := validateDiscoveredRemoteFacts(x86Facts, armFacts); err != nil {
		return err
	}
	r.x86Facts = x86Facts
	r.armFacts = armFacts
	if err := writeLine(stdout, "==> discovering substrate identity"); err != nil {
		return err
	}
	return writeDiscoveredFactSummary(stdout, x86Facts, armFacts)
}

func validateDiscoveredRemoteFacts(x86Facts, armFacts discoveredRemoteFacts) error {
	for _, item := range []struct {
		label string
		value string
	}{
		{"x86_64 container image digest", x86Facts.ContainerImage},
		{"arm64 container image digest", armFacts.ContainerImage},
		{"x86_64 cpu", x86Facts.CPU},
		{"x86_64 kernel", x86Facts.Kernel},
		{"arm64 cpu", armFacts.CPU},
		{"arm64 kernel", armFacts.Kernel},
	} {
		if strings.TrimSpace(item.value) == "" {
			return fmt.Errorf("failed to discover required remote fact: %s", item.label)
		}
	}
	return nil
}

func writeDiscoveredFactSummary(stdout io.Writer, x86Facts, armFacts discoveredRemoteFacts) error {
	for _, line := range []string{
		"    x86_64 cpu: " + x86Facts.CPU,
		"    x86_64 kernel: " + x86Facts.Kernel,
		"    x86_64 image: " + x86Facts.ContainerImage,
		"    arm64 cpu: " + armFacts.CPU,
		"    arm64 kernel: " + armFacts.Kernel,
		"    arm64 image: " + armFacts.ContainerImage,
	} {
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
	return cmdWriteInfraManifest(map[string]string{
		"--output":                  r.infraManifestPath,
		"--toolchain-lock":          r.opts.toolchainLockPath,
		"--toolchain-root":          r.opts.toolchainRoot,
		"--host-arch":               r.opts.hostArch,
		"--infra-repo-url":          serverRepoURL,
		"--infra-repo-commit":       r.gitCommit,
		"--provider-engine":         "opentofu",
		"--provider-version":        r.tofuVersion,
		"--provider-lock-sha256":    r.lockSHA,
		"--cloud-provider":          "aws",
		"--region":                  r.opts.awsRegion,
		"--x86-instance-type":       r.infra.X86.InstanceType,
		"--x86-image-id":            r.infra.X86.ImageID,
		"--x86-discovered-cpu":      r.x86Facts.CPU,
		"--x86-discovered-kernel":   r.x86Facts.Kernel,
		"--arm64-instance-type":     r.infra.Arm64.InstanceType,
		"--arm64-image-id":          r.infra.Arm64.ImageID,
		"--arm64-discovered-cpu":    r.armFacts.CPU,
		"--arm64-discovered-kernel": r.armFacts.Kernel,
	}, stdout)
}

func (r *serverEvidenceRuntime) buildArtifacts(stdout io.Writer) error {
	if err := writeLine(stdout, "==> building architecture-specific jcs-canon control binaries"); err != nil {
		return err
	}
	x86Artifacts, err := buildServerRunArtifacts(r.opts, r.toolchain, "x86_64")
	if err != nil {
		return err
	}
	armArtifacts, err := buildServerRunArtifacts(r.opts, r.toolchain, "arm64")
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
	return runServerMatrix(context.Background(), cfg, stdout)
}

func (r *serverEvidenceRuntime) runReleaseGates(stdout io.Writer) error {
	for _, arch := range []string{"x86_64", "arm64"} {
		if err := writef(stdout, "==> running release gate: %s\n", arch); err != nil {
			return err
		}
		if err := runServerReleaseGate(r.toolchain.goBinary, r.opts.root, r.releaseGateRunForArch(arch)); err != nil {
			return err
		}
	}
	return nil
}

func (r *serverEvidenceRuntime) destroy() error {
	if r.destroyed {
		return nil
	}
	if err := destroyServerInfrastructure(r.opts, r.toolchain, r.gitCommit, r.lockSHA, r.sshPublicKey); err != nil {
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
		matrixPath:        filepath.Join(r.opts.root, "offline", "matrix.server-"+arch+".yaml"),
		profilePath:       filepath.Join(r.opts.root, "offline", "profiles", "server-linux-"+arch+".yaml"),
		bundlePath:        artifacts.bundlePath,
		controlBinaryPath: artifacts.controlBinaryPath,
		workerPath:        artifacts.workerPath,
		evidencePath:      filepath.Join(r.opts.outputDir, arch, "offline-evidence.json"),
		infraManifestPath: r.infraManifestPath,
		sourceGitCommit:   r.gitCommit,
		sourceGitTag:      r.opts.tag,
		target:            r.targetForArch(arch),
		containerImage:    r.factsForArch(arch).ContainerImage,
		sshRunner:         r.sshRunner,
	}
}

func (r *serverEvidenceRuntime) releaseGateRunForArch(arch string) releaseGateRun {
	artifacts := r.artifactsForArch(arch)
	return releaseGateRun{
		evidencePath:      filepath.Join(r.opts.outputDir, arch, "offline-evidence.json"),
		bundlePath:        artifacts.bundlePath,
		matrixPath:        filepath.Join(r.opts.root, "offline", "matrix.server-"+arch+".yaml"),
		profilePath:       filepath.Join(r.opts.root, "offline", "profiles", "server-linux-"+arch+".yaml"),
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

func (r *serverEvidenceRuntime) factsForArch(arch string) discoveredRemoteFacts {
	if arch == matrixArchitectureARM64 {
		return r.armFacts
	}
	return r.x86Facts
}

func (r *serverEvidenceRuntime) targetForArch(arch string) string {
	if arch == matrixArchitectureARM64 {
		return r.armTarget
	}
	return r.x86Target
}

func buildServerRunArtifacts(opts serverEvidenceOptions, toolchain serverToolchain, matrixArch string) (serverBuildArtifacts, error) {
	archDir := filepath.Join(opts.outputDir, matrixArch)
	controlBinaryPath := filepath.Join(archDir, "jcs-canon")
	workerPath := filepath.Join(archDir, "jcs-offline-worker")
	bundlePath := filepath.Join(archDir, "offline-bundle.tgz")
	if err := buildGoBinary(toolchain.goBinary, opts.root, matrixArch, opts.tag, "./cmd/jcs-canon", controlBinaryPath); err != nil {
		return serverBuildArtifacts{}, err
	}
	if err := buildGoBinary(toolchain.goBinary, opts.root, matrixArch, "v0.0.0-dev", "./cmd/jcs-offline-worker", workerPath); err != nil {
		return serverBuildArtifacts{}, err
	}
	matrixPath := filepath.Join(opts.root, "offline", "matrix.server-"+matrixArch+".yaml")
	profilePath := filepath.Join(opts.root, "offline", "profiles", "server-linux-"+matrixArch+".yaml")
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
	globalEnv := map[string]string{
		"JCS_SERVER_SSH_TARGET":      cfg.target,
		"JCS_SERVER_CONTAINER_IMAGE": cfg.containerImage,
	}
	evidence, err := replay.RunMatrix(runCtx, matrix, profile, newServerAdapterFactory(cfg.sshRunner, cfg.workerPath), replay.RunOptions{
		BundlePath:          cfg.bundlePath,
		BundleSHA256:        bundleSHA,
		ControlBinarySHA256: manifest.BinarySHA256,
		MatrixSHA256:        matrixSHA,
		ProfileSHA256:       profileSHA,
		SourceGitCommit:     cfg.sourceGitCommit,
		SourceGitTag:        cfg.sourceGitTag,
		Orchestrator:        "jcs-offline-replay server-evidence",
		GlobalEnv:           globalEnv,
		InfraManifestSHA256: infraManifestSHA,
		InfraRepoURL:        serverRepoURL,
		InfraRepoCommit:     cfg.sourceGitCommit,
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

func runServerReleaseGate(goBinary, repoRoot string, cfg releaseGateRun) error {
	ctx, cancel := context.WithTimeout(context.Background(), serverReleaseGateTimeout)
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

func provisionServerInfrastructure(opts serverEvidenceOptions, toolchain serverToolchain, gitCommit, lockSHA, sshPublicKey string) (provisionedInfra, error) {
	ctx, cancel := context.WithTimeout(context.Background(), serverProvisionTimeout)
	defer cancel()
	if _, err := runCommandInDir(ctx, opts.infraDir, nil, toolchain.tofuBinary, "init", "-input=false", "-upgrade=false"); err != nil {
		return provisionedInfra{}, err
	}
	args := append([]string{"apply", "-auto-approve", "-input=false"}, tofuVarArgs(gitCommit, lockSHA, opts.awsRegion, opts.sshIngressCIDR, sshPublicKey)...)
	if _, err := runCommandInDir(ctx, opts.infraDir, nil, toolchain.tofuBinary, args...); err != nil {
		return provisionedInfra{}, err
	}
	x86IP, err := tofuOutputRaw(toolchain.tofuBinary, opts.infraDir, "x86_64_public_ip")
	if err != nil {
		return provisionedInfra{}, err
	}
	armIP, err := tofuOutputRaw(toolchain.tofuBinary, opts.infraDir, "arm64_public_ip")
	if err != nil {
		return provisionedInfra{}, err
	}
	x86AMI, err := tofuOutputRaw(toolchain.tofuBinary, opts.infraDir, "x86_64_ami")
	if err != nil {
		return provisionedInfra{}, err
	}
	armAMI, err := tofuOutputRaw(toolchain.tofuBinary, opts.infraDir, "arm64_ami")
	if err != nil {
		return provisionedInfra{}, err
	}
	x86Type, err := tofuOutputRaw(toolchain.tofuBinary, opts.infraDir, "x86_64_instance_type")
	if err != nil {
		return provisionedInfra{}, err
	}
	armType, err := tofuOutputRaw(toolchain.tofuBinary, opts.infraDir, "arm64_instance_type")
	if err != nil {
		return provisionedInfra{}, err
	}
	return provisionedInfra{
		X86: provisionedHost{
			Architecture: "x86_64",
			PublicIP:     x86IP,
			ImageID:      x86AMI,
			InstanceType: x86Type,
		},
		Arm64: provisionedHost{
			Architecture: "arm64",
			PublicIP:     armIP,
			ImageID:      armAMI,
			InstanceType: armType,
		},
	}, nil
}

func destroyServerInfrastructure(opts serverEvidenceOptions, toolchain serverToolchain, gitCommit, lockSHA, sshPublicKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), serverProvisionTimeout)
	defer cancel()
	args := append([]string{"destroy", "-auto-approve", "-input=false"}, tofuVarArgs(gitCommit, lockSHA, opts.awsRegion, opts.sshIngressCIDR, sshPublicKey)...)
	_, err := runCommandInDir(ctx, opts.infraDir, nil, toolchain.tofuBinary, args...)
	return err
}

func tofuVarArgs(gitCommit, lockSHA, awsRegion, sshIngressCIDR, sshPublicKey string) []string {
	return []string{
		"-var", "ssh_public_key=" + sshPublicKey,
		"-var", "infra_repo_url=" + serverRepoURL,
		"-var", "infra_repo_commit=" + gitCommit,
		"-var", "provider_lock_sha256=" + lockSHA,
		"-var", "aws_region=" + awsRegion,
		"-var", "ssh_ingress_cidr=" + sshIngressCIDR,
	}
}

func tofuOutputRaw(tofuBinary, infraDir, name string) (string, error) {
	out, err := runCommandInDir(context.Background(), infraDir, nil, tofuBinary, "output", "-raw", name)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(out)
	if value == "" {
		return "", fmt.Errorf("tofu output %s is empty", name)
	}
	return value, nil
}

func resolveServerToolchain() (serverToolchain, error) {
	info := serverToolchain{
		goBinary:          lookupEnvTrimmed("JCS_TOOL_GO"),
		tofuBinary:        lookupEnvTrimmed("JCS_TOOL_TOFU"),
		dockerStaticAMD64: lookupEnvTrimmed("JCS_TOOL_DOCKER_STATIC_AMD64"),
		dockerStaticARM64: lookupEnvTrimmed("JCS_TOOL_DOCKER_STATIC_ARM64"),
	}
	for _, item := range []struct {
		label string
		path  string
	}{
		{"JCS_TOOL_GO", info.goBinary},
		{"JCS_TOOL_TOFU", info.tofuBinary},
		{"JCS_TOOL_DOCKER_STATIC_AMD64", info.dockerStaticAMD64},
		{"JCS_TOOL_DOCKER_STATIC_ARM64", info.dockerStaticARM64},
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

//nolint:gosec // REQ:OFFLINE-AUTO-001 server evidence reads the operator-selected private key for the billed AWS run.
func newServerSSHRunner(keyPath string) (*serverSSHRunner, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read ssh private key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse ssh private key: %w", err)
	}
	return &serverSSHRunner{
		signer:         signer,
		connectTimeout: serverSSHConnectTimeout,
		hostKeys:       make(map[string]string),
	}, nil
}

func (r *serverSSHRunner) Wait(ctx context.Context, target string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		runCtx, runCancel := context.WithTimeout(waitCtx, r.connectTimeout)
		_, err := r.run(runCtx, target, "true", nil)
		runCancel()
		if err == nil {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("ssh unreachable after %s: %s", timeout, target)
		case <-ticker.C:
		}
	}
}

func installRemoteContainerRuntime(ctx context.Context, runner *serverSSHRunner, target, archivePath string) error {
	if err := runner.uploadFile(ctx, target, archivePath, "/tmp/jcs-docker-static.tgz"); err != nil {
		return err
	}
	script := strings.Join([]string{
		"set -euo pipefail",
		"sudo rm -rf /opt/jcs-docker-static",
		"sudo mkdir -p /opt/jcs-docker-static",
		"sudo tar -xzf /tmp/jcs-docker-static.tgz -C /opt/jcs-docker-static",
		"for bin in /opt/jcs-docker-static/docker/*; do sudo ln -sf \"$bin\" \"/usr/local/bin/$(basename \"$bin\")\"; done",
		"if ! sudo pgrep -x dockerd >/dev/null 2>&1; then sudo nohup /opt/jcs-docker-static/docker/dockerd >/tmp/jcs-dockerd.log 2>&1 & fi",
		"for _ in $(seq 1 60); do if sudo docker version >/dev/null 2>&1; then exit 0; fi; sleep 2; done",
		"echo \"error: docker daemon did not become ready\" >&2",
		"exit 1",
	}, "\n")
	if _, err := runner.run(ctx, target, script, nil); err != nil {
		return fmt.Errorf("install remote container runtime on %s: %w", target, err)
	}
	return nil
}

func discoverRemoteFacts(ctx context.Context, runner *serverSSHRunner, target, imageTag string) (discoveredRemoteFacts, error) {
	imageCmd := fmt.Sprintf("sudo docker pull %s >/dev/null && sudo docker image inspect --format='{{index .RepoDigests 0}}' %s", shellQuote(imageTag), shellQuote(imageTag))
	imageDigest, err := runner.run(ctx, target, imageCmd, nil)
	if err != nil {
		return discoveredRemoteFacts{}, fmt.Errorf("resolve remote image digest on %s: %w", target, err)
	}
	cpuCmd := strings.Join([]string{
		`awk -F: '`,
		`/^model name/ {gsub(/^[ \t]+/, "", $2); print $2; exit}`,
		`/^Hardware/ {hardware=$2; gsub(/^[ \t]+/, "", hardware)}`,
		`/^CPU architecture/ {arch=$2; gsub(/^[ \t]+/, "", arch)}`,
		`/^CPU part/ {part=$2; gsub(/^[ \t]+/, "", part)}`,
		`/^CPU implementer/ {impl=$2; gsub(/^[ \t]+/, "", impl)}`,
		`END {`,
		`  if (hardware != "") { print hardware; exit }`,
		`  if (arch != "" || part != "" || impl != "") {`,
		`    out="ARM"`,
		`    if (arch != "") out=out " arch " arch`,
		`    if (impl != "") out=out " impl " impl`,
		`    if (part != "") out=out " part " part`,
		`    print out`,
		`  }`,
		`}' /proc/cpuinfo`,
	}, "\n")
	cpu, err := runner.run(ctx, target, cpuCmd, nil)
	if err != nil {
		return discoveredRemoteFacts{}, fmt.Errorf("discover remote cpu on %s: %w", target, err)
	}
	kernel, err := runner.run(ctx, target, "uname -r", nil)
	if err != nil {
		return discoveredRemoteFacts{}, fmt.Errorf("discover remote kernel on %s: %w", target, err)
	}
	return discoveredRemoteFacts{
		CPU:            strings.TrimSpace(cpu),
		Kernel:         strings.TrimSpace(kernel),
		ContainerImage: strings.TrimSpace(imageDigest),
	}, nil
}

func newServerAdapterFactory(runner *serverSSHRunner, workerPath string) replay.AdapterFactory {
	return func(node replay.NodeSpec) (replay.NodeAdapter, error) {
		switch node.Mode {
		case replay.NodeModeVM, replay.NodeModeContainer:
			return &serverRemoteAdapter{
				ssh:        runner,
				workerPath: workerPath,
			}, nil
		default:
			return nil, fmt.Errorf("node %s unsupported server mode %q", node.ID, node.Mode)
		}
	}
}

func (a *serverRemoteAdapter) Prepare(_ context.Context, _ replay.NodeSpec, _ string, _ int) error {
	return nil
}

func (a *serverRemoteAdapter) Cleanup(_ context.Context, _ replay.NodeSpec, _ int) error {
	return nil
}

func (a *serverRemoteAdapter) RunReplay(ctx context.Context, node replay.NodeSpec, bundlePath, evidencePath string, replayIndex int) (retErr error) {
	target, schemaVersion, err := resolveReplayInvocation(node)
	if err != nil {
		return err
	}
	remoteTmp := fmt.Sprintf("/tmp/jcs-offline-%s-%03d-%s", node.ID, replayIndex, randomSuffix())
	if prepErr := a.prepareRemoteWorkspace(ctx, target, remoteTmp); prepErr != nil {
		return prepErr
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if cleanupErr := a.cleanupRemoteWorkspace(cleanupCtx, target, remoteTmp); cleanupErr != nil {
			retErr = errors.Join(retErr, cleanupErr)
		}
	}()
	if uploadErr := a.uploadReplayInputs(ctx, target, bundlePath, remoteTmp); uploadErr != nil {
		return uploadErr
	}
	uid, gid, err := a.resolveContainerIdentity(ctx, node, target)
	if err != nil {
		return err
	}
	runCmd, err := buildRemoteReplayCommand(node, replayIndex, remoteTmp, schemaVersion, uid, gid)
	if err != nil {
		return err
	}
	if _, err := a.ssh.run(ctx, target, runCmd, nil); err != nil {
		return err
	}
	if err := a.ssh.downloadFile(ctx, target, remoteTmp+"/evidence.json", evidencePath); err != nil {
		return err
	}
	if err := os.Chmod(evidencePath, filePerm); err != nil {
		return fmt.Errorf("chmod local evidence %s: %w", evidencePath, err)
	}
	return nil
}

func (a *serverRemoteAdapter) cleanupRemoteWorkspace(ctx context.Context, target, remoteTmp string) error {
	_, err := a.ssh.run(ctx, target, "rm -rf "+shellQuote(remoteTmp), nil)
	if err != nil {
		return fmt.Errorf("cleanup remote workspace %s on %s: %w", remoteTmp, target, err)
	}
	return nil
}

func (a *serverRemoteAdapter) prepareRemoteWorkspace(ctx context.Context, target, remoteTmp string) error {
	if _, err := a.ssh.run(ctx, target, "mkdir -p "+shellQuote(remoteTmp), nil); err != nil {
		return fmt.Errorf("prepare remote workspace %s on %s: %w", remoteTmp, target, err)
	}
	return nil
}

func (a *serverRemoteAdapter) uploadReplayInputs(ctx context.Context, target, bundlePath, remoteTmp string) error {
	if err := a.ssh.uploadFile(ctx, target, bundlePath, remoteTmp+"/bundle.tgz"); err != nil {
		return err
	}
	if err := a.ssh.uploadFile(ctx, target, a.workerPath, remoteTmp+"/jcs-offline-worker"); err != nil {
		return err
	}
	return nil
}

func resolveReplayInvocation(node replay.NodeSpec) (string, string, error) {
	target := resolveServerRunnerValue(node.Runner.Env, "JCS_VM_SSH_TARGET", "JCS_VM_SSH_TARGET_ENV")
	if target == "" {
		target = resolveServerRunnerValue(node.Runner.Env, "JCS_SERVER_SSH_TARGET", "")
	}
	if target == "" {
		return "", "", fmt.Errorf("node %s ssh target is empty", node.ID)
	}
	schemaVersion := strings.TrimSpace(node.Runner.Env["JCS_EVIDENCE_SCHEMA_VERSION"])
	if schemaVersion == "" {
		schemaVersion = replay.EvidenceSchemaVersion
	}
	return target, schemaVersion, nil
}

func (a *serverRemoteAdapter) resolveContainerIdentity(ctx context.Context, node replay.NodeSpec, target string) (string, string, error) {
	if node.Mode != replay.NodeModeContainer {
		return "", "", nil
	}
	return a.ssh.remoteUIDGID(ctx, target)
}

func buildRemoteReplayCommand(node replay.NodeSpec, replayIndex int, remoteTmp, schemaVersion, uid, gid string) (string, error) {
	workerBundlePath := remoteTmp + "/bundle.tgz"
	workerEvidencePath := remoteTmp + "/evidence.json"
	workerArgs := []string{
		"--bundle", shellQuote(workerBundlePath),
		"--evidence", shellQuote(workerEvidencePath),
		"--node-id", shellQuote(node.ID),
		"--mode", shellQuote(string(node.Mode)),
		"--distro", shellQuote(node.Distro),
		"--kernel-family", shellQuote(node.KernelFamily),
		"--replay-index", shellQuote(fmt.Sprintf("%d", replayIndex)),
		"--schema-version", shellQuote(schemaVersion),
	}
	if node.Mode == replay.NodeModeVM {
		return strings.Join([]string{
			"chmod +x " + shellQuote(remoteTmp+"/jcs-offline-worker"),
			"LC_ALL=C LANG=C TZ=UTC " + shellQuote(remoteTmp+"/jcs-offline-worker") + " " + strings.Join(workerArgs, " "),
		}, " && "), nil
	}
	image := resolveServerRunnerValue(node.Runner.Env, "JCS_CONTAINER_IMAGE", "JCS_CONTAINER_IMAGE_ENV")
	if image == "" {
		image = resolveServerRunnerValue(node.Runner.Env, "JCS_SERVER_CONTAINER_IMAGE", "")
	}
	if image == "" {
		return "", fmt.Errorf("node %s container image is empty", node.ID)
	}
	if uid == "" || gid == "" {
		return "", fmt.Errorf("node %s remote uid/gid is empty", node.ID)
	}
	containerName := fmt.Sprintf("jcs-replay-%s-%03d", node.ID, replayIndex)
	containerBundlePath := "/work/bundle.tgz"
	containerEvidencePath := "/work/out/evidence.json"
	containerWorkerArgs := append([]string(nil), workerArgs...)
	containerWorkerArgs[1] = shellQuote(containerBundlePath)
	containerWorkerArgs[3] = shellQuote(containerEvidencePath)
	containerWorkerArgs = append(containerWorkerArgs, "--image-digest", shellQuote(image))
	return strings.Join([]string{
		"chmod +x " + shellQuote(remoteTmp+"/jcs-offline-worker"),
		fmt.Sprintf(
			"sudo docker run --rm --name %s --network none --user %s:%s -v %s:/work/bundle.tgz:ro -v %s:/work/jcs-offline-worker:ro -v %s:/work/out -e LC_ALL=C -e LANG=C -e TZ=UTC %s /work/jcs-offline-worker %s",
			shellQuote(containerName),
			shellQuote(uid),
			shellQuote(gid),
			shellQuote(remoteTmp+"/bundle.tgz"),
			shellQuote(remoteTmp+"/jcs-offline-worker"),
			shellQuote(remoteTmp),
			shellQuote(image),
			strings.Join(containerWorkerArgs, " "),
		),
	}, " && "), nil
}

func resolveServerRunnerValue(env map[string]string, key, indirectKey string) string {
	if value := strings.TrimSpace(env[key]); value != "" {
		return value
	}
	if indirectKey == "" {
		return ""
	}
	if name := strings.TrimSpace(env[indirectKey]); name != "" {
		return strings.TrimSpace(env[name])
	}
	return ""
}

func (r *serverSSHRunner) remoteUIDGID(ctx context.Context, target string) (string, string, error) {
	out, err := r.run(ctx, target, "printf '%s:%s' \"$(id -u)\" \"$(id -g)\"", nil)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.TrimSpace(out), ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("parse remote uid/gid %q", strings.TrimSpace(out))
	}
	return parts[0], parts[1], nil
}

//nolint:gosec // REQ:OFFLINE-AUTO-001 server evidence uploads explicit local bundle and worker paths selected by Go-native orchestration.
func (r *serverSSHRunner) uploadFile(ctx context.Context, target, localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read local file %s: %w", localPath, err)
	}
	cmd := "umask 077 && cat > " + shellQuote(remotePath)
	_, err = r.run(ctx, target, cmd, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("upload %s to %s:%s: %w", localPath, target, remotePath, err)
	}
	return nil
}

func (r *serverSSHRunner) downloadFile(ctx context.Context, target, remotePath, localPath string) error {
	data, err := r.run(ctx, target, "cat "+shellQuote(remotePath), nil)
	if err != nil {
		return fmt.Errorf("download %s:%s: %w", target, remotePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), dirPerm); err != nil {
		return fmt.Errorf("create local evidence dir: %w", err)
	}
	if err := os.WriteFile(localPath, []byte(data), filePerm); err != nil {
		return fmt.Errorf("write local file %s: %w", localPath, err)
	}
	return nil
}

func (r *serverSSHRunner) run(ctx context.Context, target, script string, stdin io.Reader) (stdout string, retErr error) {
	user, address, err := parseSSHTarget(target)
	if err != nil {
		return "", err
	}
	client, err := r.dial(ctx, user, address)
	if err != nil {
		return "", err
	}
	defer func() {
		retErr = errors.Join(retErr, closeSSHClient(client, target))
	}()
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh new session %s: %w", target, err)
	}
	defer func() {
		retErr = errors.Join(retErr, closeSSHSession(session, target))
	}()
	session.Stdin = stdin
	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	if err := session.Run("sh -lc " + shellQuote(script)); err != nil {
		msg := strings.TrimSpace(stderrBuf.String())
		if msg == "" {
			msg = strings.TrimSpace(stdoutBuf.String())
		}
		if msg != "" {
			return "", fmt.Errorf("ssh %s failed: %w: %s", target, err, msg)
		}
		return "", fmt.Errorf("ssh %s failed: %w", target, err)
	}
	return stdoutBuf.String(), nil
}

func (r *serverSSHRunner) dial(ctx context.Context, user, address string) (*ssh.Client, error) {
	dialer := &net.Dialer{Timeout: r.connectTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial ssh %s: %w", address, err)
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(r.signer)},
		HostKeyCallback: r.hostKeyCallback(address),
		Timeout:         r.connectTimeout,
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		closeErr := conn.Close()
		if closeErr != nil {
			return nil, errors.Join(fmt.Errorf("ssh handshake %s: %w", address, err), fmt.Errorf("close ssh conn %s: %w", address, closeErr))
		}
		return nil, fmt.Errorf("ssh handshake %s: %w", address, err)
	}
	return ssh.NewClient(clientConn, chans, reqs), nil
}

func (r *serverSSHRunner) hostKeyCallback(address string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)
		r.hostKeyMu.Lock()
		defer r.hostKeyMu.Unlock()
		if existing, ok := r.hostKeys[address]; ok {
			if existing != fingerprint {
				return fmt.Errorf("ssh host key changed for %s: got=%s want=%s", address, fingerprint, existing)
			}
			return nil
		}
		r.hostKeys[address] = fingerprint
		return nil
	}
}

func parseSSHTarget(target string) (string, string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", fmt.Errorf("ssh target is empty")
	}
	parts := strings.SplitN(target, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid ssh target %q", target)
	}
	host := parts[1]
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, defaultSSHPort)
	}
	return parts[0], host, nil
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
	var _ replay.NodeAdapter = (*serverRemoteAdapter)(nil)
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

func closeSSHClient(client *ssh.Client, target string) error {
	if client == nil {
		return nil
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("close ssh client %s: %w", target, err)
	}
	return nil
}

func closeSSHSession(session *ssh.Session, target string) error {
	if session == nil {
		return nil
	}
	if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("close ssh session %s: %w", target, err)
	}
	return nil
}
