package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const (
	testRemoteStateBucket = "bucket"
	testTaggedRelease     = "v9.9.9"
)

func TestNewServerEvidenceRuntimeSuccess(t *testing.T) {
	oldNewClients := newServerAWSClientsFunc
	oldResolveIdentity := resolveServerAWSIdentityFunc
	t.Cleanup(func() {
		newServerAWSClientsFunc = oldNewClients
		resolveServerAWSIdentityFunc = oldResolveIdentity
	})

	repoRoot, gitCommit := initServerEvidenceTestRepo(t)
	outputDir := filepath.Join(repoRoot, "offline", "runs", "releases", "v9.9.9")
	toolchainDir := t.TempDir()
	goBinary := writeFakeExecutable(t, filepath.Join(toolchainDir, "go"), "#!/bin/sh\nexit 0\n")
	tofuBinary := writeFakeExecutable(t, filepath.Join(toolchainDir, "tofu"), "#!/bin/sh\nif [ \"$1\" = \"version\" ]; then\n  printf 'OpenTofu v1.10.6\\n'\n  exit 0\nfi\nexit 1\n")
	t.Setenv("JCS_TOOL_GO", goBinary)
	t.Setenv("JCS_TOOL_TOFU", tofuBinary)
	t.Setenv("GITHUB_RUN_ID", "12345")

	newServerAWSClientsFunc = func(context.Context, string) (serverAWSClients, error) {
		return serverAWSClients{}, nil
	}
	resolveServerAWSIdentityFunc = func(context.Context, serverAWSClients) (serverAWSIdentity, error) {
		return serverAWSIdentity{
			AccountID: "123456789012",
			ARN:       "arn:aws:iam::123456789012:role/test-role",
		}, nil
	}

	rt, err := newServerEvidenceRuntime(context.Background(), serverEvidenceOptions{
		tag:               "v9.9.9",
		awsRegion:         "us-east-1",
		amiLockPath:       filepath.Join(repoRoot, "infra", "aws_release_hosts.lock.json"),
		toolchainLockPath: filepath.Join(repoRoot, "offline", "toolchain.lock.tsv"),
		toolchainRoot:     filepath.Join(repoRoot, "toolchain"),
		hostArch:          "amd64",
		outputDir:         outputDir,
		lockFilePath:      filepath.Join(repoRoot, "infra", ".terraform.lock.hcl"),
		infraDir:          filepath.Join(repoRoot, "infra"),
		root:              repoRoot,
	})
	if err != nil {
		t.Fatalf("newServerEvidenceRuntime: %v", err)
	}
	t.Cleanup(func() {
		if cleanupErr := rt.sourceCleanup(); cleanupErr != nil {
			t.Fatalf("sourceCleanup: %v", cleanupErr)
		}
	})

	if rt.gitCommit != gitCommit {
		t.Fatalf("gitCommit=%q want %q", rt.gitCommit, gitCommit)
	}
	if rt.tofuVersion != "1.10.6" {
		t.Fatalf("tofuVersion=%q want 1.10.6", rt.tofuVersion)
	}
	if rt.runRecord.AWSAccountID != "123456789012" {
		t.Fatalf("AWSAccountID=%q", rt.runRecord.AWSAccountID)
	}
	if rt.runRecord.WorkflowRunURL != serverRepoURL+"/actions/runs/12345" {
		t.Fatalf("WorkflowRunURL=%q", rt.runRecord.WorkflowRunURL)
	}
	if rt.sourceRoot == repoRoot {
		t.Fatal("expected detached source root")
	}
	for _, arch := range []string{"x86_64", "arm64"} {
		if _, statErr := os.Stat(filepath.Join(outputDir, arch)); statErr != nil {
			t.Fatalf("missing output dir for %s: %v", arch, statErr)
		}
	}

	record, err := loadServerRunRecord(rt.runRecordPath)
	if err != nil {
		t.Fatalf("loadServerRunRecord: %v", err)
	}
	if record.SourceGitCommit != gitCommit {
		t.Fatalf("run record source commit=%q want %q", record.SourceGitCommit, gitCommit)
	}
	if record.ProviderLockSHA256 == "" {
		t.Fatal("expected provider lock sha in run record")
	}
}

func TestBuildServerRunArtifactsCurrentArchitecture(t *testing.T) {
	matrixArch := currentMatrixArchitecture(t)
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("LookPath(go): %v", err)
	}
	repoRoot := resolveRepoRoot()
	outputDir := t.TempDir()

	withWorkingDirectory(t, repoRoot, func() {
		artifacts, buildErr := buildServerRunArtifacts(context.Background(), serverEvidenceOptions{
			tag:       "v0.0.1-test",
			outputDir: outputDir,
		}, repoRoot, serverToolchain{goBinary: goBinary}, matrixArch)
		if buildErr != nil {
			t.Fatalf("buildServerRunArtifacts: %v", buildErr)
		}
		for _, path := range []string{artifacts.controlBinaryPath, artifacts.workerPath, artifacts.bundlePath} {
			info, statErr := os.Stat(path)
			if statErr != nil {
				t.Fatalf("stat %s: %v", path, statErr)
			}
			if info.Size() == 0 {
				t.Fatalf("artifact %s is empty", path)
			}
		}
	})
}

func TestRunRecordCompletionAndCommandHelpers(t *testing.T) {
	root := t.TempDir()
	recordPath := filepath.Join(root, "server-run.v1.json")
	rt := &serverEvidenceRuntime{
		runRecordPath: recordPath,
		runRecord:     newServerRunRecord(recordPath, serverEvidenceOptions{tag: "v1.2.3", outputDir: root}, serverEvidenceOptions{root: root}, strings.Repeat("a", 40), filepath.Join(root, "source"), strings.Repeat("b", 64)),
	}
	if err := writeServerRunRecord(recordPath, &rt.runRecord); err != nil {
		t.Fatalf("writeServerRunRecord: %v", err)
	}

	failErr := errors.New("boom")
	if err := rt.failRunRecord(failErr); err != nil {
		t.Fatalf("failRunRecord: %v", err)
	}
	if rt.runRecord.RunStatus != serverRunStatusFailed || rt.runRecord.LastError != "boom" {
		t.Fatalf("unexpected failed run record: %#v", rt.runRecord)
	}
	if err := rt.completeRunRecordSuccess(); err != nil {
		t.Fatalf("completeRunRecordSuccess: %v", err)
	}
	if rt.runRecord.RunStatus != serverRunStatusSucceeded || rt.runRecord.LastError != "" || rt.runRecord.CompletedAtUTC == "" {
		t.Fatalf("unexpected completed run record: %#v", rt.runRecord)
	}

	out, err := runCommandInDir(context.Background(), root, map[string]string{"JCS_TEST_ENV": "set"}, "bash", "-lc", "printf '%s' \"$JCS_TEST_ENV\"")
	if err != nil {
		t.Fatalf("runCommandInDir success: %v", err)
	}
	if out != "set" {
		t.Fatalf("runCommandInDir output=%q want set", out)
	}
	if _, runErr := runCommandInDir(context.Background(), root, nil, "bash", "-lc", "echo failure && exit 3"); runErr == nil || !strings.Contains(runErr.Error(), "failure") {
		t.Fatalf("expected command failure with captured output, got %v", runErr)
	}

	trimmedPath := filepath.Join(root, "trimmed.txt")
	mustWriteFile(t, trimmedPath, []byte("  value \n"), 0o600)
	trimmed, err := readTrimmedFile(trimmedPath)
	if err != nil {
		t.Fatalf("readTrimmedFile: %v", err)
	}
	if trimmed != "value" {
		t.Fatalf("trimmed=%q want value", trimmed)
	}
	if got := sha256HexString("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha256HexString mismatch: %q", got)
	}
	if got := firstNonEmpty("", "  ", "winner", "fallback"); got != "winner" {
		t.Fatalf("firstNonEmpty=%q want winner", got)
	}
	if !validFullSHA(strings.Repeat("f", 40)) {
		t.Fatal("expected valid full sha")
	}

	stderr := &bytes.Buffer{}
	if code := writeErrorLine(stderr, errors.New("broken")); code != 2 {
		t.Fatalf("writeErrorLine code=%d want 2", code)
	}
	if !strings.Contains(stderr.String(), "error: broken") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}

	c := &testCloser{err: errors.New("ignore")}
	closeBestEffort(c)
	if !c.closed {
		t.Fatal("closeBestEffort did not close")
	}
}

func TestCmdPreflightAndResolveExpectedInfraBinding(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	mustMkdirAll(t, fakeBin)
	writeFakeCommand(t, filepath.Join(fakeBin, "go"), "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n")
	writeFakeCommand(t, filepath.Join(fakeBin, "tar"), "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n")
	writeFakeCommand(t, filepath.Join(fakeBin, "podman"), "#!/usr/bin/env bash\nset -euo pipefail\nif [ \"${1:-}\" = \"info\" ]; then exit 0; fi\nif [ \"${1:-}\" = \"image\" ] && [ \"${2:-}\" = \"inspect\" ]; then exit 0; fi\nexit 0\n")
	pathValue, _ := os.LookupEnv("PATH")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+pathValue)
	matrixPath := filepath.Join(root, "matrix.yaml")
	mustWriteFile(t, matrixPath, []byte(`{"version":"v1","architecture":"x86_64","nodes":[{"id":"container-only","mode":"container","distro":"debian-13","kernel_family":"host","replays":1,"runner":{"kind":"container_command","replay":["true","debian:13"]}}]}`+"\n"), 0o600)

	var out bytes.Buffer
	if err := cmdPreflight(map[string]string{
		"--matrix":    matrixPath,
		"--no-strict": boolTrue,
	}, &out); err != nil {
		t.Fatalf("cmdPreflight non-strict: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "[WARN] no vm nodes found in matrix") {
		t.Fatalf("expected warning output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "[preflight] RESULT=PASS") {
		t.Fatalf("expected pass output, got %q", out.String())
	}

	var strictOut bytes.Buffer
	if err := cmdPreflight(map[string]string{
		"--matrix": matrixPath,
		"--strict": boolTrue,
	}, &strictOut); err == nil {
		t.Fatal("expected strict preflight failure")
	}

	manifestPath := filepath.Join(root, "infra-manifest.v1.json")
	manifest := &replay.InfraManifest{
		SchemaVersion:      replay.InfraManifestSchemaVersion,
		GeneratedAtUTC:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		InfraRepoURL:       serverRepoURL,
		InfraRepoCommit:    strings.Repeat("a", 40),
		ProviderEngine:     "opentofu",
		ProviderVersion:    "1.10.6",
		ProviderLockSHA256: strings.Repeat("b", 64),
		Hosts: []replay.InfraManifestHost{{
			Role:             "x86_64",
			Architecture:     "x86_64",
			NodeIDs:          []string{"x86_64"},
			CloudProvider:    "aws",
			Region:           "us-east-1",
			InstanceType:     "c6i.large",
			ImageID:          "ami-x86",
			SubnetVisibility: "private",
			Transport:        "ssm",
		}},
		Tools: []replay.InfraManifestTool{{
			ID:                     "go-linux-amd64",
			Scope:                  "host",
			Purpose:                "build",
			Name:                   "go",
			Version:                "1.24.13",
			OS:                     "linux",
			Arch:                   "amd64",
			Format:                 "tar.gz",
			SourceURL:              "https://example.test/go.tar.gz",
			SHA256:                 strings.Repeat("c", 64),
			ArtifactRelativePath:   "downloads/go.tar.gz",
			ExecutableRelativePath: "go/bin/go",
		}},
	}
	if err := writeInfraManifestDocument(manifestPath, manifest); err != nil {
		t.Fatalf("writeInfraManifestDocument: %v", err)
	}
	profile := &replay.Profile{
		Name:           "aws-native-release-linux-x86_64",
		RequiredSuites: []string{"infra-substrate-binding"},
	}
	evidence := &replay.EvidenceBundle{
		InfraRepoURL:    serverRepoURL,
		InfraRepoCommit: strings.Repeat("a", 40),
	}
	manifestSHA, repoURL, repoCommit, loadedManifest, err := resolveExpectedInfraBinding(map[string]string{
		"--infra-manifest": manifestPath,
	}, evidence, profile)
	if err != nil {
		t.Fatalf("resolveExpectedInfraBinding: %v", err)
	}
	if manifestSHA == "" || repoURL != serverRepoURL || repoCommit != strings.Repeat("a", 40) || loadedManifest == nil {
		t.Fatalf("unexpected infra binding resolution: sha=%q repo=%q commit=%q manifest=%#v", manifestSHA, repoURL, repoCommit, loadedManifest)
	}
	if _, _, _, _, err := resolveExpectedInfraBinding(map[string]string{}, evidence, profile); err == nil {
		t.Fatal("expected required infra manifest error")
	}
}

func TestRunOfflineReleaseGateWithGeneratedArtifacts(t *testing.T) {
	matrixArch := currentMatrixArchitecture(t)
	repoRoot := resolveRepoRoot()
	root := t.TempDir()

	controlPath := filepath.Join(root, "bin", "jcs-canon")
	workerPath := filepath.Join(root, "bin", "jcs-offline-worker")
	mustMkdirAll(t, filepath.Dir(controlPath))
	currentExecutable, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 test reads the current executable path returned by the Go runtime.
	controlData, err := os.ReadFile(currentExecutable)
	if err != nil {
		t.Fatalf("read executable: %v", err)
	}
	mustWriteFile(t, controlPath, controlData, 0o700)
	mustWriteFile(t, workerPath, controlData, 0o700)

	matrixPath := filepath.Join(root, "matrix.yaml")
	profilePath := filepath.Join(root, "profile.yaml")
	vectorPath := filepath.Join(root, "vectors.jsonl")
	manifestPath := filepath.Join(root, "infra-manifest.v1.json")
	evidencePath := filepath.Join(root, "offline-evidence.json")
	bundlePath := filepath.Join(root, "offline-bundle.tgz")

	mustWriteFile(t, matrixPath, []byte(`{"version":"v1","architecture":"`+matrixArch+`","nodes":[{"id":"aws-node","mode":"vm","distro":"debian-13","kernel_family":"cloud-amd","replays":1,"runner":{"kind":"vm_ssm","replay":["true"]}}]}`+"\n"), 0o600)
	mustWriteFile(t, profilePath, []byte(`{"name":"aws-native-release-linux-`+matrixArch+`","required_suites":["infra-substrate-binding"],"min_cold_replays":1,"hard_release_gate":true,"evidence_required":true,"version":"v1"}`+"\n"), 0o600)
	mustWriteFile(t, vectorPath, []byte(`{"id":"case-1","mode":"canonicalize","input":"{}","want_stdout":"{}","want_exit":0}`+"\n"), 0o600)

	manifest := &replay.InfraManifest{
		SchemaVersion:      replay.InfraManifestSchemaVersion,
		GeneratedAtUTC:     "2026-01-01T00:00:00Z",
		InfraRepoURL:       serverRepoURL,
		InfraRepoCommit:    strings.Repeat("a", 40),
		ProviderEngine:     "opentofu",
		ProviderVersion:    "1.10.6",
		ProviderLockSHA256: strings.Repeat("b", 64),
		Hosts: []replay.InfraManifestHost{{
			Role:               "aws-node",
			Architecture:       matrixArch,
			NodeIDs:            []string{"aws-node"},
			CloudProvider:      "aws",
			Region:             "us-east-1",
			AvailabilityZone:   "us-east-1a",
			InstanceType:       "c6i.large",
			ImageID:            "ami-test",
			InstanceID:         "i-test",
			OSID:               "debian",
			OSVersionID:        "13",
			CPU:                "Example CPU",
			Kernel:             "6.8.0-test",
			Transport:          "ssm",
			SubnetVisibility:   "private",
			DiscoveredCPU:      "Example CPU",
			DiscoveredKernel:   "6.8.0-test",
			IIDDocumentSHA256:  strings.Repeat("1", 64),
			IIDSignatureSHA256: strings.Repeat("2", 64),
			IIDPKCS7SHA256:     strings.Repeat("3", 64),
			IIDVerified:        true,
		}},
		Tools: []replay.InfraManifestTool{{
			ID:                     "go-linux-amd64",
			Scope:                  "host",
			Purpose:                "build",
			Name:                   "go",
			Version:                "1.24.13",
			OS:                     "linux",
			Arch:                   "amd64",
			Format:                 "tar.gz",
			SourceURL:              "https://example.test/go.tar.gz",
			SHA256:                 strings.Repeat("c", 64),
			ArtifactRelativePath:   "downloads/go.tar.gz",
			ExecutableRelativePath: "go/bin/go",
		}},
	}
	if writeErr := writeInfraManifestDocument(manifestPath, manifest); writeErr != nil {
		t.Fatalf("writeInfraManifestDocument: %v", writeErr)
	}

	if _, bundleErr := replay.CreateBundle(replay.BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  controlPath,
		WorkerPath:  workerPath,
		MatrixPath:  matrixPath,
		ProfilePath: profilePath,
		VectorsGlob: vectorPath,
		Version:     "bundle.v1",
	}); bundleErr != nil {
		t.Fatalf("CreateBundle: %v", bundleErr)
	}

	bundleSHA, err := fileSHA256(bundlePath)
	if err != nil {
		t.Fatalf("fileSHA256(bundle): %v", err)
	}
	controlSHA, err := fileSHA256(controlPath)
	if err != nil {
		t.Fatalf("fileSHA256(control): %v", err)
	}
	matrixSHA, err := fileSHA256(matrixPath)
	if err != nil {
		t.Fatalf("fileSHA256(matrix): %v", err)
	}
	profileSHA, err := fileSHA256(profilePath)
	if err != nil {
		t.Fatalf("fileSHA256(profile): %v", err)
	}
	manifestSHA, err := fileSHA256(manifestPath)
	if err != nil {
		t.Fatalf("fileSHA256(manifest): %v", err)
	}
	aggregateDigest := governedReplayAggregateDigest("aws-node", 1, strings.Repeat("f", 64))

	if err := replay.WriteEvidence(evidencePath, &replay.EvidenceBundle{
		SchemaVersion:            replay.EvidenceSchemaVersion,
		BundleSHA256:             bundleSHA,
		ControlBinarySHA:         controlSHA,
		MatrixSHA256:             matrixSHA,
		ProfileSHA256:            profileSHA,
		VectorSetSHA256:          strings.Repeat("e", 64),
		GovernanceUmbrellaCommit: strings.Repeat("b", 40),
		GovernanceLockSHA256:     strings.Repeat("c", 64),
		SourceGitCommit:          strings.Repeat("a", 40),
		SourceGitTag:             "v1.2.3-test",
		GeneratedAtUTC:           "2026-01-01T00:00:00Z",
		Orchestrator:             "jcs-offline-replay server-evidence",
		ProfileID:                "https://lattice-substrate.github.io/jcs/profiles/official-cloud-measured-release.v1",
		ProfileName:              "official-cloud-measured-release",
		Architecture:             matrixArch,
		AggregateMethod:          replay.ReplayAggregateMethod,
		RequiredSuites:           []string{"infra-substrate-binding"},
		HardReleaseGate:          true,
		InfraManifestSHA256:      manifestSHA,
		InfraRepoURL:             serverRepoURL,
		InfraRepoCommit:          strings.Repeat("a", 40),
		IIDTrustRootSetID:        "aws-iid-trust-roots.v1",
		AggregateCanonical:       aggregateDigest,
		AggregateVerify:          aggregateDigest,
		AggregateClass:           aggregateDigest,
		AggregateExitCode:        aggregateDigest,
		NodeReplays: []replay.NodeRunEvidence{{
			NodeID:                     "aws-node",
			Mode:                       "vm",
			Distro:                     "debian-13",
			KernelFamily:               "cloud-amd",
			ReplayIndex:                1,
			SessionID:                  "session-1",
			StartedAtUTC:               "2026-01-01T00:00:00Z",
			CompletedAtUTC:             "2026-01-01T00:00:10Z",
			CaseCount:                  1,
			Passed:                     true,
			CanonicalSHA256:            strings.Repeat("f", 64),
			VerifySHA256:               strings.Repeat("f", 64),
			FailureClassSHA256:         strings.Repeat("f", 64),
			ExitCodeSHA256:             strings.Repeat("f", 64),
			DiscoveredCPU:              "Example CPU",
			DiscoveredKernel:           "6.8.0-test",
			MeasuredArchitecture:       matrixArch,
			MeasuredOSID:               "debian",
			MeasuredOSVersionID:        "13",
			MeasuredKernel:             "6.8.0-test",
			MeasuredCPU:                "Example CPU",
			AWSInstanceID:              "i-test",
			AWSImageID:                 "ami-test",
			TransportAttestationSHA256: strings.Repeat("4", 64),
		}},
	}); err != nil {
		t.Fatalf("WriteEvidence: %v", err)
	}

	withWorkingDirectory(t, repoRoot, func() {
		var stdout bytes.Buffer
		if err := runOfflineReleaseGate(matrixPath, profilePath, evidencePath, strings.Repeat("a", 40), "v1.2.3-test", manifestPath, filepath.Join(root, "release-gate.log"), &stdout); err != nil {
			t.Fatalf("runOfflineReleaseGate: %v\n%s", err, stdout.String())
		}
		if !strings.Contains(stdout.String(), "[run] release gate test") {
			t.Fatalf("unexpected stdout: %q", stdout.String())
		}
	})
}

func governedReplayAggregateDigest(nodeID string, replayIndex int, digest string) string {
	sum := sha256.Sum256([]byte(nodeID + "\x1f" + fmt.Sprintf("%03d", replayIndex) + "\x1f" + digest + "\n"))
	return hex.EncodeToString(sum[:])
}

func TestServerEvidenceCommandWrappers(t *testing.T) {
	outputDir := t.TempDir()
	toolchainRoot := t.TempDir()
	toolchainLock := filepath.Join(t.TempDir(), "toolchain.lock.tsv")
	mustWriteFile(t, toolchainLock, []byte("tool\tversion\n"), 0o600)

	var stdout bytes.Buffer
	if err := cmdServerEvidence(map[string]string{}, &stdout); err == nil {
		t.Fatal("expected missing tag error")
	}

	parsedOpts, err := parseServerEvidenceOptions(map[string]string{
		"--tag":            "v4.5.6",
		"--output-dir":     outputDir,
		"--toolchain-root": toolchainRoot,
		"--toolchain-lock": toolchainLock,
		"--host-arch":      "amd64",
	})
	if err != nil {
		t.Fatalf("parseServerEvidenceOptions: %v", err)
	}
	if parsedOpts.tag != "v4.5.6" || parsedOpts.outputDir != outputDir || parsedOpts.toolchainRoot != toolchainRoot || parsedOpts.toolchainLockPath != toolchainLock {
		t.Fatalf("unexpected parsed options: %#v", parsedOpts)
	}

	remoteOpts, err := parseServerEvidenceOptions(map[string]string{
		"--tag":              "v7.8.9",
		"--state-mode":       serverStateModeRemote,
		"--state-bucket":     testRemoteStateBucket,
		"--state-lock-table": "locks",
		"--state-region":     "us-west-2",
		"--state-key":        "custom.tfstate",
		"--host-arch":        "amd64",
	})
	if err != nil {
		t.Fatalf("parseServerEvidenceOptions remote: %v", err)
	}
	if remoteOpts.state.Mode != serverStateModeRemote || remoteOpts.state.Bucket != testRemoteStateBucket || remoteOpts.state.Region != "us-west-2" || remoteOpts.state.Key != "custom.tfstate" {
		t.Fatalf("unexpected remote state opts: %#v", remoteOpts.state)
	}
	if _, err := requireServerEvidenceFlags(map[string]string{}); err == nil {
		t.Fatal("expected missing tag error")
	}
}

func TestCmdInitInfraLockAndServerCleanup(t *testing.T) {
	oldRunCommand := runCommandInDirFunc
	oldNewClients := newServerAWSClientsFunc
	oldDeleteBucket := deleteStagingBucketFunc
	oldDestroyInfra := destroyServerInfrastructureFunc
	t.Cleanup(func() {
		runCommandInDirFunc = oldRunCommand
		newServerAWSClientsFunc = oldNewClients
		deleteStagingBucketFunc = oldDeleteBucket
		destroyServerInfrastructureFunc = oldDestroyInfra
	})

	toolchainDir := t.TempDir()
	goBinary := writeFakeExecutable(t, filepath.Join(toolchainDir, "go"), "#!/bin/sh\nexit 0\n")
	tofuBinary := writeFakeExecutable(t, filepath.Join(toolchainDir, "tofu"), "#!/bin/sh\nexit 0\n")
	t.Setenv("JCS_TOOL_GO", goBinary)
	t.Setenv("JCS_TOOL_TOFU", tofuBinary)

	var commandDir, commandName string
	var commandArgs []string
	runCommandInDirFunc = func(_ context.Context, dir string, _ map[string]string, name string, args ...string) (string, error) {
		commandDir = dir
		commandName = name
		commandArgs = append([]string(nil), args...)
		return "", nil
	}

	var initOut bytes.Buffer
	if err := cmdInitInfraLock(map[string]string{"--infra-dir": "/tmp/infra"}, &initOut); err != nil {
		t.Fatalf("cmdInitInfraLock: %v", err)
	}
	if commandDir != "/tmp/infra" || commandName != tofuBinary {
		t.Fatalf("unexpected init target dir=%q name=%q", commandDir, commandName)
	}
	if strings.Join(commandArgs, " ") != "init -input=false -upgrade=false -backend=false" {
		t.Fatalf("unexpected init args: %v", commandArgs)
	}
	if !strings.Contains(initOut.String(), "infra lock ready: /tmp/infra/.terraform.lock.hcl") {
		t.Fatalf("unexpected init stdout: %q", initOut.String())
	}

	runCommandInDirFunc = oldRunCommand
	newServerAWSClientsFunc = func(context.Context, string) (serverAWSClients, error) {
		return serverAWSClients{}, nil
	}
	var deletedBucket, destroyedInfra bool
	deleteStagingBucketFunc = func(context.Context, serverAWSClients, string) error {
		deletedBucket = true
		return nil
	}
	destroyServerInfrastructureFunc = func(context.Context, serverEvidenceOptions, serverToolchain, string, string) error {
		destroyedInfra = true
		return nil
	}

	outputDir := t.TempDir()
	runRecordPath := filepath.Join(outputDir, "server-run.v1.json")
	matrixPath := filepath.Join(outputDir, "matrix.yaml")
	mustWriteFile(t, matrixPath, []byte(`{"version":"v1","architecture":"x86_64","nodes":[{"id":"container-only","mode":"container","distro":"debian-13","kernel_family":"host","replays":1,"runner":{"kind":"container_command","replay":["true","debian:13"]}}]}`+"\n"), 0o600)
	record := newServerRunRecord(runRecordPath, serverEvidenceOptions{
		tag:       "v1.2.3",
		outputDir: outputDir,
		awsRegion: "us-east-1",
	}, serverEvidenceOptions{
		root:         outputDir,
		infraDir:     filepath.Join(outputDir, "infra"),
		lockFilePath: filepath.Join(outputDir, "infra", ".terraform.lock.hcl"),
		amiLockPath:  filepath.Join(outputDir, "infra", "aws_release_hosts.lock.json"),
	}, strings.Repeat("a", 40), filepath.Join(outputDir, "source"), strings.Repeat("b", 64))
	record.RunRecordPath = runRecordPath
	record.StagingBucket = testRemoteStateBucket
	record.RunStatus = serverRunStatusRunning
	if err := writeServerRunRecord(runRecordPath, &record); err != nil {
		t.Fatalf("writeServerRunRecord: %v", err)
	}

	var cleanupOut bytes.Buffer
	if err := cmdServerCleanup(map[string]string{"--run-record": runRecordPath}, &cleanupOut); err != nil {
		t.Fatalf("cmdServerCleanup: %v\n%s", err, cleanupOut.String())
	}
	if !deletedBucket || !destroyedInfra {
		t.Fatalf("expected cleanup operations, deletedBucket=%t destroyedInfra=%t", deletedBucket, destroyedInfra)
	}
	updated, err := loadServerRunRecord(runRecordPath)
	if err != nil {
		t.Fatalf("loadServerRunRecord: %v", err)
	}
	if updated.DestroyStatus != serverRunStatusSucceeded || updated.RunStatus != serverRunStatusFailed {
		t.Fatalf("unexpected cleanup record: %#v", updated)
	}
	for _, path := range []string{
		filepath.Join(outputDir, "audit", "server-evidence-summary.json"),
		filepath.Join(outputDir, "audit", "server-evidence-summary.md"),
	} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("expected cleanup audit artifact %s: %v", path, statErr)
		}
	}
}

func TestRunServerCleanupBranchesAndResolveSourceIdentity(t *testing.T) {
	oldNewClients := newServerAWSClientsFunc
	oldDeleteBucket := deleteStagingBucketFunc
	oldDestroyInfra := destroyServerInfrastructureFunc
	t.Cleanup(func() {
		newServerAWSClientsFunc = oldNewClients
		deleteStagingBucketFunc = oldDeleteBucket
		destroyServerInfrastructureFunc = oldDestroyInfra
	})

	toolchainDir := t.TempDir()
	goBinary := writeFakeExecutable(t, filepath.Join(toolchainDir, "go"), "#!/bin/sh\nexit 0\n")
	tofuBinary := writeFakeExecutable(t, filepath.Join(toolchainDir, "tofu"), "#!/bin/sh\nexit 0\n")
	t.Setenv("JCS_TOOL_GO", goBinary)
	t.Setenv("JCS_TOOL_TOFU", tofuBinary)
	newServerAWSClientsFunc = func(context.Context, string) (serverAWSClients, error) {
		return serverAWSClients{}, nil
	}

	completedDir := t.TempDir()
	completedRecord := newServerRunRecord(filepath.Join(completedDir, "server-run.v1.json"), serverEvidenceOptions{
		tag:       "v1.0.0",
		outputDir: completedDir,
		awsRegion: "us-east-1",
	}, serverEvidenceOptions{
		root:         completedDir,
		infraDir:     filepath.Join(completedDir, "infra"),
		lockFilePath: filepath.Join(completedDir, "infra", ".terraform.lock.hcl"),
		amiLockPath:  filepath.Join(completedDir, "infra", "aws_release_hosts.lock.json"),
	}, strings.Repeat("a", 40), filepath.Join(completedDir, "source"), strings.Repeat("b", 64))
	completedRecord.RunRecordPath = filepath.Join(completedDir, "server-run.v1.json")
	completedRecord.DestroyStatus = serverRunStatusSucceeded
	if err := writeServerRunRecord(completedRecord.RunRecordPath, &completedRecord); err != nil {
		t.Fatalf("writeServerRunRecord(completed): %v", err)
	}
	var completedOut bytes.Buffer
	if err := runServerCleanup(&completedRecord, &completedOut); err != nil {
		t.Fatalf("runServerCleanup completed: %v", err)
	}
	if !strings.Contains(completedOut.String(), "cleanup already complete") {
		t.Fatalf("unexpected completed cleanup output: %q", completedOut.String())
	}

	failingDir := t.TempDir()
	failingRecord := newServerRunRecord(filepath.Join(failingDir, "server-run.v1.json"), serverEvidenceOptions{
		tag:       "v2.0.0",
		outputDir: failingDir,
		awsRegion: "us-east-1",
	}, serverEvidenceOptions{
		root:         failingDir,
		infraDir:     filepath.Join(failingDir, "infra"),
		lockFilePath: filepath.Join(failingDir, "infra", ".terraform.lock.hcl"),
		amiLockPath:  filepath.Join(failingDir, "infra", "aws_release_hosts.lock.json"),
	}, strings.Repeat("c", 40), filepath.Join(failingDir, "source"), strings.Repeat("d", 64))
	failingRecord.RunRecordPath = filepath.Join(failingDir, "server-run.v1.json")
	failingRecord.StagingBucket = testRemoteStateBucket
	if err := writeServerRunRecord(failingRecord.RunRecordPath, &failingRecord); err != nil {
		t.Fatalf("writeServerRunRecord(failing): %v", err)
	}
	deleteStagingBucketFunc = func(context.Context, serverAWSClients, string) error {
		return errors.New("bucket delete failed")
	}
	destroyServerInfrastructureFunc = func(context.Context, serverEvidenceOptions, serverToolchain, string, string) error {
		return errors.New("infra destroy failed")
	}
	var failingOut bytes.Buffer
	err := runServerCleanup(&failingRecord, &failingOut)
	if err == nil {
		t.Fatal("expected cleanup failure")
	}
	if !strings.Contains(err.Error(), "bucket delete failed") || !strings.Contains(err.Error(), "infra destroy failed") {
		t.Fatalf("unexpected cleanup error: %v", err)
	}
	stored, err := loadServerRunRecord(failingRecord.RunRecordPath)
	if err != nil {
		t.Fatalf("loadServerRunRecord(failing): %v", err)
	}
	if stored.DestroyStatus != serverRunStatusFailed || stored.LastError == "" || stored.CompletedAtUTC == "" {
		t.Fatalf("unexpected stored failing cleanup record: %#v", stored)
	}

	t.Setenv("JCS_OFFLINE_SOURCE_GIT_COMMIT", strings.Repeat("e", 40))
	t.Setenv("JCS_OFFLINE_SOURCE_GIT_TAG", "v3.4.5")
	commit, tag, err := resolveSourceIdentity(map[string]string{})
	if err != nil {
		t.Fatalf("resolveSourceIdentity(env): %v", err)
	}
	if commit != strings.Repeat("e", 40) || tag != "v3.4.5" {
		t.Fatalf("unexpected env source identity: commit=%q tag=%q", commit, tag)
	}
	if len(utcStamp()) != len("20060102T150405Z") {
		t.Fatalf("unexpected utcStamp format: %q", utcStamp())
	}
}

//nolint:gocognit // REQ:OFFLINE-AUTO-001 test intentionally exercises multiple CLI dispatch branches in one place.
func TestResolveSourceIdentityGitFallbacksAndDispatchSubcommand(t *testing.T) {
	repoRoot, commit := initServerEvidenceTestRepo(t)
	runGitCommand(t, repoRoot, "tag", testTaggedRelease)

	withWorkingDirectory(t, repoRoot, func() {
		t.Setenv("JCS_OFFLINE_SOURCE_GIT_COMMIT", "")
		t.Setenv("JCS_OFFLINE_SOURCE_GIT_TAG", "")

		gotCommit, gotTag, err := resolveSourceIdentity(map[string]string{})
		if err != nil {
			t.Fatalf("resolveSourceIdentity(git): %v", err)
		}
		if gotCommit != commit || gotTag != testTaggedRelease {
			t.Fatalf("unexpected git source identity: commit=%q tag=%q", gotCommit, gotTag)
		}

		offlineCommit, offlineTag, err := resolveOfflineSourceIdentity()
		if err != nil {
			t.Fatalf("resolveOfflineSourceIdentity(git): %v", err)
		}
		if offlineCommit != commit || offlineTag != testTaggedRelease {
			t.Fatalf("unexpected offline source identity: commit=%q tag=%q", offlineCommit, offlineTag)
		}
	})

	oldRunCommand := runCommandInDirFunc
	oldNewClients := newServerAWSClientsFunc
	oldDeleteBucket := deleteStagingBucketFunc
	oldDestroyInfra := destroyServerInfrastructureFunc
	t.Cleanup(func() {
		runCommandInDirFunc = oldRunCommand
		newServerAWSClientsFunc = oldNewClients
		deleteStagingBucketFunc = oldDeleteBucket
		destroyServerInfrastructureFunc = oldDestroyInfra
	})

	toolchainDir := t.TempDir()
	goBinary := writeFakeExecutable(t, filepath.Join(toolchainDir, "go"), "#!/bin/sh\nexit 0\n")
	tofuBinary := writeFakeExecutable(t, filepath.Join(toolchainDir, "tofu"), "#!/bin/sh\nexit 0\n")
	t.Setenv("JCS_TOOL_GO", goBinary)
	t.Setenv("JCS_TOOL_TOFU", tofuBinary)
	runCommandInDirFunc = func(_ context.Context, _ string, _ map[string]string, _ string, _ ...string) (string, error) {
		return "", nil
	}
	newServerAWSClientsFunc = func(context.Context, string) (serverAWSClients, error) {
		return serverAWSClients{}, nil
	}
	deleteStagingBucketFunc = func(context.Context, serverAWSClients, string) error { return nil }
	destroyServerInfrastructureFunc = func(context.Context, serverEvidenceOptions, serverToolchain, string, string) error {
		return nil
	}

	outputDir := t.TempDir()
	runRecordPath := filepath.Join(outputDir, "server-run.v1.json")
	matrixPath := filepath.Join(outputDir, "matrix.yaml")
	mustWriteFile(t, matrixPath, []byte(`{"version":"v1","architecture":"x86_64","nodes":[{"id":"container-only","mode":"container","distro":"debian-13","kernel_family":"host","replays":1,"runner":{"kind":"container_command","replay":["true","debian:13"]}}]}`+"\n"), 0o600)
	record := newServerRunRecord(runRecordPath, serverEvidenceOptions{
		tag:       "v1.2.3",
		outputDir: outputDir,
		awsRegion: "us-east-1",
	}, serverEvidenceOptions{
		root:         outputDir,
		infraDir:     filepath.Join(outputDir, "infra"),
		lockFilePath: filepath.Join(outputDir, "infra", ".terraform.lock.hcl"),
		amiLockPath:  filepath.Join(outputDir, "infra", "aws_release_hosts.lock.json"),
	}, strings.Repeat("a", 40), filepath.Join(outputDir, "source"), strings.Repeat("b", 64))
	record.RunRecordPath = runRecordPath
	if err := writeServerRunRecord(runRecordPath, &record); err != nil {
		t.Fatalf("writeServerRunRecord: %v", err)
	}

	tests := []struct {
		name    string
		sub     string
		flags   map[string]string
		wantErr string
	}{
		{
			name:  "init infra lock",
			sub:   "init-infra-lock",
			flags: map[string]string{"--infra-dir": filepath.Join(outputDir, "infra")},
		},
		{
			name:    "server evidence missing tag",
			sub:     "server-evidence",
			flags:   map[string]string{},
			wantErr: "server-evidence requires --tag",
		},
		{
			name:  "server cleanup",
			sub:   "server-cleanup",
			flags: map[string]string{"--run-record": runRecordPath},
		},
		{
			name:    "report missing evidence",
			sub:     "report",
			flags:   map[string]string{},
			wantErr: "report requires --evidence",
		},
		{
			name:    "prepare missing flags",
			sub:     "prepare",
			flags:   map[string]string{},
			wantErr: "prepare requires --matrix, --profile, --binary, --bundle",
		},
		{
			name:    "run missing flags",
			sub:     "run",
			flags:   map[string]string{},
			wantErr: "run requires --matrix, --profile, --bundle, --evidence",
		},
		{
			name:    "preflight",
			sub:     "preflight",
			flags:   map[string]string{"--matrix": matrixPath, "--no-strict": boolTrue},
			wantErr: "preflight failed",
		},
		{
			name:    "audit summary missing args",
			sub:     "audit-summary",
			flags:   map[string]string{},
			wantErr: "audit-summary requires --matrix, --profile, --evidence",
		},
		{
			name:    "verify evidence missing flags",
			sub:     "verify-evidence",
			flags:   map[string]string{},
			wantErr: "verify-evidence requires --matrix, --profile, --evidence",
		},
		{
			name:  "inspect matrix",
			sub:   "inspect-matrix",
			flags: map[string]string{"--matrix": matrixPath},
		},
		{
			name:    "refresh aws ami lock missing input",
			sub:     "refresh-aws-ami-lock",
			flags:   map[string]string{"--input": filepath.Join(outputDir, "missing.json")},
			wantErr: "read aws release host catalog",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code, err := dispatchSubcommand(tc.sub, tc.flags, &stdout, &stderr)
			if tc.wantErr == "" {
				if err != nil || code != 0 {
					t.Fatalf("dispatchSubcommand(%s) code=%d err=%v stdout=%q stderr=%q", tc.sub, code, err, stdout.String(), stderr.String())
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("dispatchSubcommand(%s) err=%v want %q", tc.sub, err, tc.wantErr)
			}
		})
	}
}

type gateCall struct {
	logPath string
	env     map[string]string
	args    []string
}

func TestOfficialGateWrappers(t *testing.T) {
	oldRunGo := runGoCommandLoggedFunc
	t.Cleanup(func() {
		runGoCommandLoggedFunc = oldRunGo
	})

	var calls []gateCall
	runGoCommandLoggedFunc = func(logPath string, _ io.Writer, env map[string]string, args ...string) error {
		copiedEnv := map[string]string(nil)
		if env != nil {
			copiedEnv = make(map[string]string, len(env))
			for k, v := range env {
				copiedEnv[k] = v
			}
		}
		calls = append(calls, gateCall{
			logPath: logPath,
			env:     copiedEnv,
			args:    append([]string(nil), args...),
		})
		return nil
	}

	var stdout bytes.Buffer
	if err := runOfficialVectorGates("/tmp/out", &stdout); err != nil {
		t.Fatalf("runOfficialVectorGates: %v", err)
	}
	if err := runOfficialES6100MGate("/tmp/out", &stdout); err != nil {
		t.Fatalf("runOfficialES6100MGate: %v", err)
	}
	if err := buildCanonicalizer("/tmp/out/bin/jcs-canon", currentMatrixArchitecture(t), "v1.2.3", "/tmp/out/logs/build-jcs-canon.log", &stdout); err != nil {
		t.Fatalf("buildCanonicalizer: %v", err)
	}
	if err := buildController("/tmp/out/bin/jcs-offline-replay", "/tmp/out/logs/build-jcs-offline-replay.log", &stdout); err != nil {
		t.Fatalf("buildController: %v", err)
	}
	if len(calls) != 5 {
		t.Fatalf("calls len=%d want 5", len(calls))
	}
	assertOfficialBuildCall(t, calls[0])
	assertOfficialVectorCall(t, calls[1])
	assertOfficialES6Call(t, calls[2])
	assertCanonicalizerBuildCall(t, calls[3])
	assertControllerBuildCall(t, calls[4])
}

func assertOfficialBuildCall(t *testing.T, c gateCall) {
	t.Helper()
	if c.logPath != filepath.Join("/tmp/out", "logs", "official-build.log") {
		t.Fatalf("unexpected official build log path: %q", c.logPath)
	}
	if c.env["CGO_ENABLED"] != "0" {
		t.Fatalf("unexpected official build env: %#v", c.env)
	}
	if !strings.Contains(strings.Join(c.args, " "), "./cmd/jcs-canon") {
		t.Fatalf("unexpected official build args: %v", c.args)
	}
}

func assertOfficialVectorCall(t *testing.T, c gateCall) {
	t.Helper()
	if c.logPath != filepath.Join("/tmp/out", "logs", "official-vectors.log") {
		t.Fatalf("unexpected vector log path: %q", c.logPath)
	}
	if len(c.env) != 0 {
		t.Fatalf("unexpected official vector env: %#v", c.env)
	}
	joined := strings.Join(c.args, " ")
	if !strings.Contains(joined, "./cmd/jcs-conformance") || !strings.Contains(joined, "--family official") {
		t.Fatalf("unexpected official vector repo args: %v", c.args)
	}
	if !strings.Contains(joined, "requirements-registry.json") {
		t.Fatalf("unexpected official vector args: %v", c.args)
	}
}

func assertOfficialES6Call(t *testing.T, c gateCall) {
	t.Helper()
	if c.env["JCS_OFFICIAL_ES6_ENABLE_100M"] != "1" {
		t.Fatalf("missing ES6 env toggle: %#v", c.env)
	}
	joined := strings.Join(c.args, " ")
	if !strings.Contains(joined, "./conformance") || !strings.Contains(joined, "TestOfficialES6CorpusChecksums100M") {
		t.Fatalf("unexpected official ES6 repo args: %v", c.args)
	}
}

func assertCanonicalizerBuildCall(t *testing.T, c gateCall) {
	t.Helper()
	if c.env["CGO_ENABLED"] != "0" || c.env["GOOS"] != "linux" || c.env["GOARCH"] == "" {
		t.Fatalf("unexpected canonicalizer env: %#v", c.env)
	}
	if !strings.Contains(strings.Join(c.args, " "), "./cmd/jcs-canon") {
		t.Fatalf("unexpected canonicalizer args: %v", c.args)
	}
}

func assertControllerBuildCall(t *testing.T, c gateCall) {
	t.Helper()
	if c.env["CGO_ENABLED"] != "0" {
		t.Fatalf("unexpected controller env: %#v", c.env)
	}
	if !strings.Contains(strings.Join(c.args, " "), "./cmd/jcs-offline-replay") {
		t.Fatalf("unexpected controller args: %v", c.args)
	}
}

func TestSourceIdentityOverrides(t *testing.T) {
	commit, tag, err := resolveSourceIdentity(map[string]string{
		"--source-git-commit": strings.Repeat("a", 40),
		"--source-git-tag":    "v1.2.3",
	})
	if err != nil {
		t.Fatalf("resolveSourceIdentity(flags): %v", err)
	}
	if commit != strings.Repeat("a", 40) || tag != "v1.2.3" {
		t.Fatalf("unexpected explicit source identity: commit=%q tag=%q", commit, tag)
	}

	t.Setenv("JCS_OFFLINE_SOURCE_GIT_COMMIT", strings.Repeat("b", 40))
	t.Setenv("JCS_OFFLINE_SOURCE_GIT_TAG", "v9.9.9")
	offlineCommit, offlineTag, err := resolveOfflineSourceIdentity()
	if err != nil {
		t.Fatalf("resolveOfflineSourceIdentity(env): %v", err)
	}
	if offlineCommit != strings.Repeat("b", 40) || offlineTag != "v9.9.9" {
		t.Fatalf("unexpected env offline source identity: commit=%q tag=%q", offlineCommit, offlineTag)
	}

	t.Setenv("JCS_OFFLINE_SOURCE_GIT_COMMIT", strings.Repeat("c", 40))
	t.Setenv("JCS_OFFLINE_SOURCE_GIT_TAG", "")
	offlineCommit, offlineTag, err = resolveOfflineSourceIdentity()
	if err != nil {
		t.Fatalf("resolveOfflineSourceIdentity(untagged): %v", err)
	}
	if offlineCommit != strings.Repeat("c", 40) || offlineTag != "untagged" {
		t.Fatalf("unexpected untagged offline source identity: commit=%q tag=%q", offlineCommit, offlineTag)
	}
}

type testCloser struct {
	closed bool
	err    error
}

func (c *testCloser) Close() error {
	c.closed = true
	return c.err
}

func initServerEvidenceTestRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "infra"))
	mustMkdirAll(t, filepath.Join(root, "offline"))
	mustWriteFile(t, filepath.Join(root, "infra", ".terraform.lock.hcl"), []byte("# terraform lock\n"), 0o600)
	mustWriteFile(t, filepath.Join(root, "offline", "toolchain.lock.tsv"), []byte("tool\tversion\n"), 0o600)
	mustWriteFile(t, filepath.Join(root, "README.md"), []byte("fixture\n"), 0o600)

	runGitCommand(t, root, "init")
	runGitCommand(t, root, "config", "user.name", "Codex Test")
	runGitCommand(t, root, "config", "user.email", "codex@example.test")
	runGitCommand(t, root, "add", ".")
	runGitCommand(t, root, "commit", "-m", "initial")
	commit := strings.TrimSpace(runGitCommand(t, root, "rev-parse", "HEAD"))
	return root, commit
}

func writeFakeExecutable(t *testing.T, path, script string) string {
	t.Helper()
	mustWriteFile(t, path, []byte(script), 0o700)
	return path
}

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("run git %v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func currentMatrixArchitecture(t *testing.T) string {
	t.Helper()
	switch runtime.GOARCH {
	case "amd64":
		return matrixArchitectureX8664
	case matrixArchitectureARM64:
		return matrixArchitectureARM64
	default:
		t.Skipf("unsupported GOARCH %q for architecture-sensitive test", runtime.GOARCH)
		return ""
	}
}
