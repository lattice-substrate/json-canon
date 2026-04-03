package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func TestCmdWriteInfraManifest(t *testing.T) {
	fixtures := buildToolchainFixtures(t)
	server := newToolchainFixtureServer(t, fixtures)
	defer server.Close()

	oldClient := toolchainHTTPClient
	toolchainHTTPClient = server.Client()
	t.Cleanup(func() {
		toolchainHTTPClient = oldClient
	})

	root := t.TempDir()
	lockPath := filepath.Join(root, "toolchain.lock.tsv")
	toolchainRoot := filepath.Join(root, "toolchain")
	envPath := filepath.Join(root, "toolchain.env")
	if err := os.WriteFile(lockPath, []byte(toolchainLockFixture(server.URL, fixtures)), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	runToolchainSync(t, lockPath, toolchainRoot, envPath)

	outputPath := filepath.Join(root, "infra-manifest.v1.json")
	var stdout bytes.Buffer
	err := cmdWriteInfraManifest(map[string]string{
		"--output":                  outputPath,
		"--toolchain-lock":          lockPath,
		"--toolchain-root":          toolchainRoot,
		"--host-arch":               "amd64",
		"--infra-repo-url":          "https://github.com/lattice-substrate/json-canon",
		"--infra-repo-commit":       strings.Repeat("a", 40),
		"--provider-engine":         "opentofu",
		"--provider-version":        "1.10.6",
		"--provider-lock-sha256":    strings.Repeat("b", 64),
		"--cloud-provider":          "aws",
		"--region":                  "us-east-1",
		"--x86-instance-type":       "c6i.large",
		"--x86-image-id":            "ami-x86",
		"--x86-discovered-cpu":      "Intel Xeon",
		"--x86-discovered-kernel":   "6.8.0-x86",
		"--arm64-instance-type":     "c7g.large",
		"--arm64-image-id":          "ami-arm",
		"--arm64-discovered-cpu":    "Graviton3",
		"--arm64-discovered-kernel": "6.8.0-arm",
	}, &stdout)
	if err != nil {
		t.Fatalf("cmdWriteInfraManifest: %v", err)
	}
	if !strings.Contains(stdout.String(), "infra-manifest: "+outputPath) {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	manifest, err := replay.LoadInfraManifest(outputPath)
	if err != nil {
		t.Fatalf("LoadInfraManifest: %v", err)
	}
	if len(manifest.Hosts) != 2 {
		t.Fatalf("hosts len = %d, want 2", len(manifest.Hosts))
	}
	if len(manifest.Tools) != 3 {
		t.Fatalf("tools len = %d, want 3", len(manifest.Tools))
	}
}

func TestLoadAWSReleaseHostCatalogAndRefreshLock(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "aws_release_hosts.json")
	outputPath := filepath.Join(root, "aws_release_hosts.lock.json")
	if err := os.WriteFile(inputPath, []byte(`{
  "schema_version": "aws-release-hosts.v1",
  "hosts": [
    {
      "host_id": "host-b",
      "node_id": "node-b",
      "architecture": "arm64",
      "distro": "al2023",
      "kernel_family": "cloud-g3",
      "instance_type": "c7g.large",
      "ami_source": "ssm",
      "ami_ssm_parameter": "/aws/service/example/arm64"
    },
    {
      "host_id": "host-a",
      "node_id": "node-a",
      "architecture": "x86_64",
      "distro": "debian-13",
      "kernel_family": "cloud-amd",
      "instance_type": "c6i.large",
      "ami_owner": "123456789012",
      "ami_name": "debian-*"
    }
  ]
}`), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	oldNewClients := newServerAWSClientsFunc
	oldResolveLock := resolveAWSReleaseHostLockFunc
	oldResolveAMIID := resolveAMIIDForHostFunc
	t.Cleanup(func() {
		newServerAWSClientsFunc = oldNewClients
		resolveAWSReleaseHostLockFunc = oldResolveLock
		resolveAMIIDForHostFunc = oldResolveAMIID
	})

	newServerAWSClientsFunc = func(context.Context, string) (serverAWSClients, error) {
		return serverAWSClients{}, nil
	}
	resolveAMIIDForHostFunc = func(_ context.Context, _ serverAWSClients, host awsReleaseHostSelector) (string, error) {
		return "ami-for-" + host.HostID, nil
	}
	resolveAWSReleaseHostLockFunc = resolveAWSReleaseHostLock

	var stdout bytes.Buffer
	if err := cmdRefreshAWSAMILock(map[string]string{
		"--input":      inputPath,
		"--output":     outputPath,
		"--aws-region": "us-east-1",
	}, &stdout); err != nil {
		t.Fatalf("cmdRefreshAWSAMILock: %v", err)
	}
	if !strings.Contains(stdout.String(), outputPath) {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	var lock awsReleaseHostLock
	//nolint:gosec // REQ:AWS-AMI-001 test reads the temp output path it just wrote.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(lock.Hosts) != 2 {
		t.Fatalf("lock hosts len = %d, want 2", len(lock.Hosts))
	}
	if lock.Hosts[0].HostID != "host-a" || lock.Hosts[1].HostID != "host-b" {
		t.Fatalf("hosts not sorted by host_id: %#v", lock.Hosts)
	}
	if lock.Hosts[0].AMIID != "ami-for-host-a" || lock.Hosts[1].AMIID != "ami-for-host-b" {
		t.Fatalf("unexpected ami ids: %#v", lock.Hosts)
	}
}

//nolint:gocognit // REQ:AWS-GATE-001 test keeps the full mocked execute lifecycle explicit for audit coverage.
func TestServerEvidenceExecuteEndToEnd(t *testing.T) {
	oldBuildArtifacts := buildServerRunArtifactsFunc
	oldCreateBucket := createStagingBucketFunc
	oldUpload := uploadStagingFileFunc
	oldPresignGet := presignGetObjectURLFunc
	oldRunSSM := runSSMShellScriptFunc
	oldCollectTools := collectToolchainEvidenceFunc
	oldWriteManifest := writeInfraManifestDocumentFunc
	oldRunMatrix := runServerMatrixFunc
	oldRunReleaseGate := runServerReleaseGateFunc
	oldCompareCrossArch := compareCrossArchEvidenceFunc
	oldDeleteBucket := deleteStagingBucketFunc
	oldDestroyInfra := destroyServerInfrastructureFunc
	oldVerifyIID := verifyAWSInstanceIdentityFunc
	t.Cleanup(func() {
		buildServerRunArtifactsFunc = oldBuildArtifacts
		createStagingBucketFunc = oldCreateBucket
		uploadStagingFileFunc = oldUpload
		presignGetObjectURLFunc = oldPresignGet
		runSSMShellScriptFunc = oldRunSSM
		collectToolchainEvidenceFunc = oldCollectTools
		writeInfraManifestDocumentFunc = oldWriteManifest
		runServerMatrixFunc = oldRunMatrix
		runServerReleaseGateFunc = oldRunReleaseGate
		compareCrossArchEvidenceFunc = oldCompareCrossArch
		deleteStagingBucketFunc = oldDeleteBucket
		destroyServerInfrastructureFunc = oldDestroyInfra
		verifyAWSInstanceIdentityFunc = oldVerifyIID
	})

	root := t.TempDir()
	outputDir := filepath.Join(root, "offline", "runs", "releases", "v1.2.3")
	for _, arch := range []string{"x86_64", "arm64"} {
		if err := os.MkdirAll(filepath.Join(outputDir, arch), 0o750); err != nil {
			t.Fatalf("mkdir arch dir: %v", err)
		}
	}
	opts := serverEvidenceOptions{
		tag:               "v1.2.3",
		awsRegion:         "us-east-1",
		outputDir:         outputDir,
		root:              root,
		toolchainLockPath: filepath.Join(root, "offline", "toolchain.lock.tsv"),
		toolchainRoot:     filepath.Join(root, "toolchain"),
		hostArch:          "amd64",
	}
	runRecordPath := filepath.Join(outputDir, "server-run.v1.json")
	runRecord := newServerRunRecord(runRecordPath, opts, opts, strings.Repeat("a", 40), root, strings.Repeat("b", 64))

	runtimeState := &serverEvidenceRuntime{
		ctx:         context.Background(),
		opts:        opts,
		toolchain:   serverToolchain{goBinary: "/bin/true", tofuBinary: "/bin/true"},
		gitCommit:   strings.Repeat("a", 40),
		lockSHA:     strings.Repeat("b", 64),
		tofuVersion: "1.10.6",
		awsClients:  serverAWSClients{},
		infra: provisionedInfra{
			Hosts: map[string]provisionedHost{
				"host-x86": {
					HostID:           "host-x86",
					NodeID:           "aws-native-x86",
					Architecture:     "x86_64",
					ImageID:          "ami-x86",
					InstanceID:       "i-x86",
					AvailabilityZone: "us-east-1a",
					InstanceType:     "c6i.large",
				},
				"host-arm": {
					HostID:           "host-arm",
					NodeID:           "aws-native-arm",
					Architecture:     "arm64",
					ImageID:          "ami-arm",
					InstanceID:       "i-arm",
					AvailabilityZone: "us-east-1b",
					InstanceType:     "c7g.large",
				},
			},
		},
		hostFacts:     make(map[string]discoveredRemoteFacts),
		runRecordPath: runRecordPath,
		runRecord:     runRecord,
	}

	if err := writeServerRunRecord(runRecordPath, &runtimeState.runRecord); err != nil {
		t.Fatalf("writeServerRunRecord: %v", err)
	}

	buildServerRunArtifactsFunc = func(_ context.Context, opts serverEvidenceOptions, _ string, _ serverToolchain, matrixArch string) (serverBuildArtifacts, error) {
		archDir := filepath.Join(opts.outputDir, matrixArch)
		control := filepath.Join(archDir, "jcs-canon")
		worker := filepath.Join(archDir, "jcs-offline-worker")
		bundle := filepath.Join(archDir, "offline-bundle.tgz")
		for _, path := range []string{control, worker, bundle} {
			if err := os.WriteFile(path, []byte(matrixArch), 0o600); err != nil {
				return serverBuildArtifacts{}, fmt.Errorf("write build artifact %s: %w", path, err)
			}
		}
		return serverBuildArtifacts{
			controlBinaryPath: control,
			workerPath:        worker,
			bundlePath:        bundle,
		}, nil
	}

	createStagingBucketFunc = func(context.Context, serverAWSClients, string) (string, error) {
		return "bucket-test", nil
	}
	uploaded := make([]string, 0, 4)
	uploadStagingFileFunc = func(_ context.Context, _ serverAWSClients, bucket, key, path string) error {
		if bucket != "bucket-test" {
			t.Fatalf("bucket = %q, want bucket-test", bucket)
		}
		uploaded = append(uploaded, key+":"+filepath.Base(path))
		return nil
	}
	presignGetObjectURLFunc = func(_ context.Context, _ serverAWSClients, bucket, key string) (string, error) {
		return "https://example.test/" + bucket + "/" + key, nil
	}
	verifyAWSInstanceIdentityFunc = func(rawDocument string, rawSignature string, host provisionedHost, expectedRegion string) (*awsInstanceIdentityDocument, error) {
		return &awsInstanceIdentityDocument{Region: "us-east-1", InstanceID: host.InstanceID, ImageID: host.ImageID}, nil
	}
	runSSMShellScriptFunc = func(_ context.Context, _ serverAWSClients, instanceID, comment, script string, _ time.Duration) (string, error) {
		if !strings.Contains(script, "inspect-host") {
			return "", errors.New("unexpected script: " + comment)
		}
		facts := discoveredRemoteFacts{
			Architecture:       "x86_64",
			OSID:               "debian",
			OSVersionID:        "13",
			CPU:                "Example CPU",
			Kernel:             "6.8.0-test",
			InstanceID:         instanceID,
			AvailabilityZone:   "us-east-1a",
			Region:             "us-east-1",
			IIDDocument:        `{"availabilityZone":"us-east-1a","imageId":"ami-x86","instanceId":"` + instanceID + `","region":"us-east-1"}`,
			IIDSignature:       "c2lnbmF0dXJl",
			IIDPKCS7:           "cGtjczc=",
			IIDDocumentSHA256:  strings.Repeat("1", 64),
			IIDSignatureSHA256: strings.Repeat("2", 64),
			IIDPKCS7SHA256:     strings.Repeat("5", 64),
			IIDVerified:        true,
		}
		if instanceID == "i-arm" {
			facts.Architecture = "arm64"
			facts.OSID = "amzn"
			facts.OSVersionID = "2023"
			facts.CPU = "Graviton3"
			facts.Kernel = "6.8.0-arm"
			facts.ImageID = "ami-arm"
			facts.AvailabilityZone = "us-east-1b"
			facts.IIDDocument = `{"availabilityZone":"us-east-1b","imageId":"ami-arm","instanceId":"` + instanceID + `","region":"us-east-1"}`
			facts.IIDDocumentSHA256 = strings.Repeat("3", 64)
			facts.IIDSignatureSHA256 = strings.Repeat("4", 64)
			facts.IIDPKCS7SHA256 = strings.Repeat("6", 64)
		} else {
			facts.ImageID = "ami-x86"
		}
		data, err := json.Marshal(facts)
		if err != nil {
			return "", fmt.Errorf("marshal discovered remote facts: %w", err)
		}
		return string(data), nil
	}
	collectToolchainEvidenceFunc = func(map[string]string, string) ([]replay.InfraManifestTool, error) {
		return []replay.InfraManifestTool{
			{
				ID:                     "go-linux-amd64",
				Scope:                  "host",
				Purpose:                "build",
				Name:                   "go",
				Version:                "1.24.13",
				OS:                     "linux",
				Arch:                   "amd64",
				Format:                 "tar.gz",
				SourceURL:              "https://example.test/go.tar.gz",
				SHA256:                 strings.Repeat("a", 64),
				ArtifactRelativePath:   "downloads/go.tar.gz",
				ExecutableRelativePath: "go/bin/go",
			},
		}, nil
	}
	writeInfraManifestDocumentFunc = writeInfraManifestDocument
	matrixRuns := make([]string, 0, 2)
	runServerMatrixFunc = func(_ context.Context, cfg serverMatrixRun, _ io.Writer) error {
		matrixRuns = append(matrixRuns, filepath.Base(cfg.matrixPath))
		if cfg.infraManifestPath == "" {
			t.Fatal("infra manifest path missing from matrix cfg")
		}
		return nil
	}
	releaseGates := make([]string, 0, 2)
	runServerReleaseGateFunc = func(_ context.Context, _ string, _ string, cfg releaseGateRun) error {
		releaseGates = append(releaseGates, filepath.Base(cfg.matrixPath))
		if cfg.infraManifestPath == "" {
			t.Fatal("infra manifest path missing from gate cfg")
		}
		return nil
	}
	crossArchCompareCalled := false
	compareCrossArchEvidenceFunc = func(x86EvidencePath, armEvidencePath, jsonPath, mdPath, repoRoot string) (*crossArchReport, error) {
		crossArchCompareCalled = true
		if repoRoot != root {
			t.Fatalf("repoRoot = %q, want %q", repoRoot, root)
		}
		if !strings.HasSuffix(x86EvidencePath, filepath.Join("x86_64", "offline-evidence.json")) {
			t.Fatalf("unexpected x86 evidence path: %q", x86EvidencePath)
		}
		if !strings.HasSuffix(armEvidencePath, filepath.Join("arm64", "offline-evidence.json")) {
			t.Fatalf("unexpected arm evidence path: %q", armEvidencePath)
		}
		if err := os.WriteFile(jsonPath, []byte("{}\n"), 0o600); err != nil {
			return nil, fmt.Errorf("write cross-arch json: %w", err)
		}
		if err := os.WriteFile(mdPath, []byte("# compare\n"), 0o600); err != nil {
			return nil, fmt.Errorf("write cross-arch markdown: %w", err)
		}
		return &crossArchReport{Result: resultPass}, nil
	}
	deleteStagingBucketFunc = func(context.Context, serverAWSClients, string) error { return nil }
	destroyServerInfrastructureFunc = func(context.Context, serverEvidenceOptions, serverToolchain, string, string) error {
		return nil
	}

	var stdout bytes.Buffer
	if err := runtimeState.execute(&stdout); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(uploaded) != 4 {
		t.Fatalf("uploaded len = %d, want 4", len(uploaded))
	}
	sort.Strings(matrixRuns)
	sort.Strings(releaseGates)
	if strings.Join(matrixRuns, ",") != "matrix.server-arm64.yaml,matrix.server-x86_64.yaml" {
		t.Fatalf("unexpected matrix runs: %#v", matrixRuns)
	}
	if strings.Join(releaseGates, ",") != "matrix.server-arm64.yaml,matrix.server-x86_64.yaml" {
		t.Fatalf("unexpected release gates: %#v", releaseGates)
	}
	if !crossArchCompareCalled {
		t.Fatal("cross-arch compare not called")
	}
	if runtimeState.runRecord.CrossArchStatus != serverRunStatusSucceeded {
		t.Fatalf("cross-arch status = %q", runtimeState.runRecord.CrossArchStatus)
	}
	if runtimeState.runRecord.DestroyStatus != serverRunStatusSucceeded {
		t.Fatalf("destroy status = %q", runtimeState.runRecord.DestroyStatus)
	}
	if runtimeState.runRecord.InfraManifestStatus != serverRunStatusSucceeded {
		t.Fatalf("infra manifest status = %q", runtimeState.runRecord.InfraManifestStatus)
	}
	if _, err := replay.LoadInfraManifest(filepath.Join(outputDir, "infra-manifest.v1.json")); err != nil {
		t.Fatalf("load generated infra manifest: %v", err)
	}
	if !strings.Contains(stdout.String(), "SUCCESS: server evidence written") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cross-arch:") {
		t.Fatalf("missing cross-arch summary in stdout: %q", stdout.String())
	}
}

func TestRunServerMatrixAndReleaseGate(t *testing.T) {
	oldLoadInfraManifest := loadInfraManifestFunc
	oldRunReplayMatrix := runReplayMatrixFunc
	oldWriteEvidence := writeEvidenceBundleFunc
	oldRunCommand := runCommandInDirFunc
	t.Cleanup(func() {
		loadInfraManifestFunc = oldLoadInfraManifest
		runReplayMatrixFunc = oldRunReplayMatrix
		writeEvidenceBundleFunc = oldWriteEvidence
		runCommandInDirFunc = oldRunCommand
	})

	root := t.TempDir()
	controlPath := filepath.Join(root, "jcs-canon")
	workerPath := filepath.Join(root, "jcs-offline-worker")
	matrixPath := filepath.Join(root, "matrix.json")
	profilePath := filepath.Join(root, "profile.json")
	vectorPath := filepath.Join(root, "vectors.jsonl")
	evidencePath := filepath.Join(root, "offline-evidence.json")
	infraManifestPath := filepath.Join(root, "infra-manifest.v1.json")
	for _, item := range []struct {
		path string
		data string
		perm os.FileMode
	}{
		{controlPath, "#!/bin/sh\n", 0o700},
		{workerPath, "#!/bin/sh\n", 0o700},
		{matrixPath, `{"version":"v1","architecture":"x86_64","nodes":[{"id":"aws-native-x86","mode":"vm","distro":"debian-13","kernel_family":"cloud-amd","replays":1,"runner":{"kind":"vm_ssm","replay":["true"]}}]}` + "\n", 0o600},
		{profilePath, `{"name":"aws-native-release-linux-x86_64","required_suites":["infra-substrate-binding"],"min_cold_replays":1,"hard_release_gate":true,"evidence_required":true,"version":"v1"}` + "\n", 0o600},
		{vectorPath, `{"id":"case-1","mode":"canonicalize","input":"{}","want_stdout":"{}","want_exit":0}` + "\n", 0o600},
	} {
		if err := os.WriteFile(item.path, []byte(item.data), item.perm); err != nil {
			t.Fatalf("write %s: %v", item.path, err)
		}
	}

	manifest := &replay.InfraManifest{
		SchemaVersion:      replay.InfraManifestSchemaVersion,
		GeneratedAtUTC:     "2026-01-01T00:00:00Z",
		InfraRepoURL:       serverRepoURL,
		InfraRepoCommit:    strings.Repeat("a", 40),
		ProviderEngine:     "opentofu",
		ProviderVersion:    "1.10.6",
		ProviderLockSHA256: strings.Repeat("b", 64),
		Hosts: []replay.InfraManifestHost{{
			Role:               "host-x86",
			Architecture:       "x86_64",
			NodeIDs:            []string{"aws-native-x86"},
			CloudProvider:      "aws",
			Region:             "us-east-1",
			InstanceType:       "c6i.large",
			ImageID:            "ami-x86",
			InstanceID:         "i-x86",
			Transport:          "ssm",
			SubnetVisibility:   "private",
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
	if err := writeInfraManifestDocument(infraManifestPath, manifest); err != nil {
		t.Fatalf("writeInfraManifestDocument: %v", err)
	}

	bundlePath := filepath.Join(root, "offline-bundle.tgz")
	if _, err := replay.CreateBundle(replay.BundleOptions{
		OutputPath:  bundlePath,
		BinaryPath:  controlPath,
		WorkerPath:  workerPath,
		MatrixPath:  matrixPath,
		ProfilePath: profilePath,
		VectorsGlob: vectorPath,
		Version:     "bundle.v1",
	}); err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}

	runReplayMatrixFunc = func(_ context.Context, matrix *replay.Matrix, profile *replay.Profile, _ replay.AdapterFactory, opts replay.RunOptions) (*replay.EvidenceBundle, error) {
		if matrix.Architecture != "x86_64" || profile.Name != "aws-native-release-linux-x86_64" {
			t.Fatalf("unexpected matrix/profile: %#v %#v", matrix, profile)
		}
		if opts.InfraManifestSHA256 == "" || opts.InfraManifest == nil {
			t.Fatalf("missing infra binding in run opts: %#v", opts)
		}
		return &replay.EvidenceBundle{
			SchemaVersion:       replay.EvidenceSchemaVersion,
			BundleSHA256:        opts.BundleSHA256,
			ControlBinarySHA:    opts.ControlBinarySHA256,
			MatrixSHA256:        opts.MatrixSHA256,
			ProfileSHA256:       opts.ProfileSHA256,
			SourceGitCommit:     opts.SourceGitCommit,
			SourceGitTag:        opts.SourceGitTag,
			GeneratedAtUTC:      "2026-01-01T00:00:00Z",
			Orchestrator:        "jcs-offline-replay server-evidence",
			ProfileName:         profile.Name,
			Architecture:        matrix.Architecture,
			RequiredSuites:      append([]string(nil), profile.RequiredSuites...),
			HardReleaseGate:     profile.HardReleaseGate,
			InfraManifestSHA256: opts.InfraManifestSHA256,
			InfraRepoURL:        opts.InfraRepoURL,
			InfraRepoCommit:     opts.InfraRepoCommit,
			NodeReplays: []replay.NodeRunEvidence{{
				NodeID:                     "aws-native-x86",
				Mode:                       "vm",
				Distro:                     "debian",
				KernelFamily:               "ga",
				ReplayIndex:                1,
				SessionID:                  "session-1",
				StartedAtUTC:               "2026-01-01T00:00:00Z",
				CompletedAtUTC:             "2026-01-01T00:00:01Z",
				CaseCount:                  1,
				Passed:                     true,
				CanonicalSHA256:            strings.Repeat("1", 64),
				VerifySHA256:               strings.Repeat("1", 64),
				FailureClassSHA256:         strings.Repeat("1", 64),
				ExitCodeSHA256:             strings.Repeat("1", 64),
				DiscoveredCPU:              "Example CPU",
				DiscoveredKernel:           "6.8.0-test",
				MeasuredArchitecture:       "x86_64",
				MeasuredOSID:               "debian",
				MeasuredOSVersionID:        "13",
				MeasuredKernel:             "6.8.0-test",
				MeasuredCPU:                "Example CPU",
				AWSInstanceID:              "i-x86",
				AWSImageID:                 "ami-x86",
				TransportAttestationSHA256: strings.Repeat("2", 64),
			}},
			AggregateCanonical: strings.Repeat("1", 64),
			AggregateVerify:    strings.Repeat("1", 64),
			AggregateClass:     strings.Repeat("1", 64),
			AggregateExitCode:  strings.Repeat("1", 64),
		}, nil
	}
	loadInfraManifestFunc = replay.LoadInfraManifest
	writeEvidenceBundleFunc = replay.WriteEvidence

	cfg := serverMatrixRun{
		matrixPath:        matrixPath,
		profilePath:       profilePath,
		bundlePath:        bundlePath,
		controlBinaryPath: controlPath,
		evidencePath:      evidencePath,
		infraManifestPath: infraManifestPath,
		sourceGitCommit:   strings.Repeat("a", 40),
		sourceGitTag:      "v1.2.3",
		hosts: map[string]provisionedHost{
			"aws-native-x86": {NodeID: "aws-native-x86", Architecture: "x86_64", InstanceID: "i-x86", ImageID: "ami-x86"},
		},
	}
	var stdout bytes.Buffer
	if err := runServerMatrix(context.Background(), cfg, &stdout); err != nil {
		t.Fatalf("runServerMatrix: %v", err)
	}
	if _, err := replay.LoadEvidence(evidencePath); err != nil {
		t.Fatalf("LoadEvidence: %v", err)
	}
	if !strings.Contains(stdout.String(), "aggregate_canonical_sha256") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	var capturedEnv map[string]string
	runCommandInDirFunc = func(_ context.Context, dir string, env map[string]string, name string, args ...string) (string, error) {
		if dir != root || name != "go" {
			t.Fatalf("unexpected command target: dir=%q name=%q args=%v", dir, name, args)
		}
		capturedEnv = env
		return "", nil
	}
	if err := runServerReleaseGate(context.Background(), "go", root, releaseGateRun{
		evidencePath:      evidencePath,
		bundlePath:        bundlePath,
		matrixPath:        matrixPath,
		profilePath:       profilePath,
		controlBinaryPath: controlPath,
		expectedCommit:    strings.Repeat("a", 40),
		expectedTag:       "v1.2.3",
		infraManifestPath: infraManifestPath,
	}); err != nil {
		t.Fatalf("runServerReleaseGate: %v", err)
	}
	if capturedEnv["JCS_OFFLINE_INFRA_MANIFEST"] != infraManifestPath {
		t.Fatalf("missing infra manifest env: %#v", capturedEnv)
	}
}

func TestInfrastructureHelpersAndGitGuards(t *testing.T) {
	oldRunCommand := runCommandInDirFunc
	t.Cleanup(func() {
		runCommandInDirFunc = oldRunCommand
	})

	var calls [][]string
	runCommandInDirFunc = func(_ context.Context, dir string, _ map[string]string, name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name, dir}, args...))
		switch {
		case len(args) >= 1 && args[0] == "status":
			return "", nil
		case len(args) >= 1 && args[0] == "output":
			return `{"host-a":{"host_id":"host-a","node_id":"node-a","architecture":"x86_64","instance_id":"i-123","image_id":"ami-123"}}`, nil
		default:
			return "", nil
		}
	}

	opts := serverEvidenceOptions{
		infraDir:    "/repo/infra",
		awsRegion:   "us-east-1",
		amiLockPath: "/repo/infra/aws_release_hosts.lock.json",
		state: serverStateConfig{
			Mode:      serverStateModeRemote,
			Bucket:    "bucket",
			Region:    "us-east-1",
			LockTable: "locks",
			Key:       "server-evidence/v1.2.3/terraform.tfstate",
		},
	}
	toolchain := serverToolchain{tofuBinary: "/bin/tofu"}
	if err := initServerInfrastructure(context.Background(), opts, toolchain); err != nil {
		t.Fatalf("initServerInfrastructure: %v", err)
	}
	infra, err := provisionServerInfrastructure(context.Background(), opts, toolchain, strings.Repeat("a", 40), strings.Repeat("b", 64))
	if err != nil {
		t.Fatalf("provisionServerInfrastructure: %v", err)
	}
	if !infra.Applied || len(infra.Hosts) != 1 {
		t.Fatalf("unexpected infra: %#v", infra)
	}
	if err := destroyServerInfrastructure(context.Background(), opts, toolchain, strings.Repeat("a", 40), strings.Repeat("b", 64)); err != nil {
		t.Fatalf("destroyServerInfrastructure: %v", err)
	}
	if len(calls) < 4 {
		t.Fatalf("expected multiple tofu calls, got %d", len(calls))
	}
	if args := tofuVarArgs("commit", "locksha", "us-east-1", "/repo/infra/aws_release_hosts.lock.json"); len(args) != 10 {
		t.Fatalf("unexpected tofuVarArgs len: %d", len(args))
	}
	if err := validateCleanGitWorktree(context.Background(), "/repo"); err != nil {
		t.Fatalf("validateCleanGitWorktree: %v", err)
	}

	runCommandInDirFunc = func(_ context.Context, _ string, _ map[string]string, _ string, args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "status" {
			return " M README.md\n", nil
		}
		return "", nil
	}
	if err := validateCleanGitWorktree(context.Background(), "/repo"); err == nil {
		t.Fatal("expected dirty worktree error")
	}
}

func TestGitReferenceAndAdapterHelpers(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatalf("mkdir git dir: %v", err)
	}
	refsHeads := filepath.Join(gitDir, "refs", "heads")
	if err := os.MkdirAll(refsHeads, 0o750); err != nil {
		t.Fatalf("mkdir refs heads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(refsHeads, "main"), []byte(strings.Repeat("c", 40)+"\n"), 0o600); err != nil {
		t.Fatalf("write loose ref: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "packed-refs"), []byte(strings.Repeat("a", 40)+" refs/tags/v1.2.3\n"), 0o600); err != nil {
		t.Fatalf("write packed-refs: %v", err)
	}
	if got, err := resolveGitRefCommit(gitDir, "refs/heads/main"); err != nil || got != strings.Repeat("c", 40) {
		t.Fatalf("resolveGitRefCommit loose got=%q err=%v", got, err)
	}
	if got, err := resolvePackedGitRef(gitDir, "refs/tags/v1.2.3"); err != nil || got != strings.Repeat("a", 40) {
		t.Fatalf("resolvePackedGitRef got=%q err=%v", got, err)
	}
	if got, err := resolveGitRefCommit(gitDir, "refs/tags/v1.2.3"); err != nil || got != strings.Repeat("a", 40) {
		t.Fatalf("resolveGitRefCommit packed got=%q err=%v", got, err)
	}
	if _, err := resolveDetachedHeadCommit("not-a-sha"); err == nil {
		t.Fatal("expected detached head validation error")
	}
	if got, err := resolveDetachedHeadCommit(strings.Repeat("b", 40)); err != nil || got != strings.Repeat("b", 40) {
		t.Fatalf("resolveDetachedHeadCommit got=%q err=%v", got, err)
	}
	if got := rebaseDetachedRepoPath("/repo", "/tmp/source", "/repo/infra/file"); got != filepath.Join("/tmp/source", "infra", "file") {
		t.Fatalf("unexpected rebased path: %q", got)
	}
	if len(randomSuffix()) != 6 {
		t.Fatalf("randomSuffix length mismatch")
	}

	factory := newServerSSMAdapterFactory(serverAWSClients{}, "bucket", stagedServerArtifacts{}, map[string]provisionedHost{})
	adapter, err := factory(replay.NodeSpec{ID: "node-a", Mode: replay.NodeModeVM})
	if err != nil {
		t.Fatalf("newServerSSMAdapterFactory: %v", err)
	}
	if err := adapter.Prepare(context.Background(), replay.NodeSpec{}, "", 1); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if err := adapter.Cleanup(context.Background(), replay.NodeSpec{}, 1); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := factory(replay.NodeSpec{ID: "bad-node", Mode: replay.NodeModeContainer}); err == nil {
		t.Fatal("expected non-vm rejection")
	}
}
