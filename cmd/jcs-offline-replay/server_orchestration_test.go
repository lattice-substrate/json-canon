package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const (
	testObjectURL        = "https://example.com/object"
	testUploadURL        = "https://example.com/upload"
	testUploadedEvidence = "uploaded_evidence=1"
)

func TestResolveServerStateConfig(t *testing.T) {
	t.Run("default local mode", func(t *testing.T) {
		cfg, err := resolveServerStateConfig(map[string]string{}, "us-east-1", "v1.2.3")
		if err != nil {
			t.Fatalf("resolveServerStateConfig: %v", err)
		}
		if cfg.Mode != serverStateModeLocal {
			t.Fatalf("state mode = %q, want %q", cfg.Mode, serverStateModeLocal)
		}
	})

	t.Run("remote mode defaults state key and region", func(t *testing.T) {
		cfg, err := resolveServerStateConfig(map[string]string{
			"--state-mode":       serverStateModeRemote,
			"--state-bucket":     "state-bucket",
			"--state-lock-table": "locks",
		}, "us-east-1", "v1.2.3")
		if err != nil {
			t.Fatalf("resolveServerStateConfig: %v", err)
		}
		if cfg.Region != "us-east-1" {
			t.Fatalf("state region = %q, want us-east-1", cfg.Region)
		}
		if cfg.Key != "server-evidence/v1.2.3/terraform.tfstate" {
			t.Fatalf("state key = %q, want default key", cfg.Key)
		}
	})

	t.Run("remote mode fails closed without backend coordinates", func(t *testing.T) {
		_, err := resolveServerStateConfig(map[string]string{
			"--state-mode": serverStateModeRemote,
		}, "us-east-1", "v1.2.3")
		if err == nil || !strings.Contains(err.Error(), "--state-bucket and --state-lock-table") {
			t.Fatalf("resolveServerStateConfig error = %v, want missing backend coordinate failure", err)
		}
	})
}

func TestNewServerRunRecordUsesStableCleanupPaths(t *testing.T) {
	runtimeOpts := serverEvidenceOptions{
		tag:          "v1.2.3",
		awsRegion:    "us-east-1",
		outputDir:    "/repo/offline/runs/releases/v1.2.3",
		root:         "/repo",
		infraDir:     "/tmp/jcs-offline-source-123/infra",
		lockFilePath: "/tmp/jcs-offline-source-123/infra/.terraform.lock.hcl",
		amiLockPath:  "/tmp/jcs-offline-source-123/infra/aws_release_hosts.lock.json",
		state: serverStateConfig{
			Mode:      serverStateModeRemote,
			Bucket:    "state-bucket",
			Region:    "us-east-1",
			LockTable: "locks",
			Key:       "server-evidence/v1.2.3/terraform.tfstate",
		},
	}
	cleanupOpts := runtimeOpts
	cleanupOpts.infraDir = "/repo/infra"
	cleanupOpts.lockFilePath = "/repo/infra/.terraform.lock.hcl"
	cleanupOpts.amiLockPath = "/repo/infra/aws_release_hosts.lock.json"

	record := newServerRunRecord(
		"/repo/offline/runs/releases/v1.2.3/server-run.v1.json",
		runtimeOpts,
		cleanupOpts,
		strings.Repeat("a", 40),
		"/tmp/jcs-offline-source-123",
		strings.Repeat("b", 64),
	)

	if record.InfraDir != cleanupOpts.infraDir {
		t.Fatalf("infra dir = %q, want %q", record.InfraDir, cleanupOpts.infraDir)
	}
	if record.ProviderLockPath != cleanupOpts.lockFilePath {
		t.Fatalf("provider lock path = %q, want %q", record.ProviderLockPath, cleanupOpts.lockFilePath)
	}
	if record.AMILockPath != cleanupOpts.amiLockPath {
		t.Fatalf("ami lock path = %q, want %q", record.AMILockPath, cleanupOpts.amiLockPath)
	}
	if record.SourceRoot != "/tmp/jcs-offline-source-123" {
		t.Fatalf("source root = %q, want detached source root", record.SourceRoot)
	}
}

func TestRunServerEvidenceCleansUpProvisionFailure(t *testing.T) {
	oldFactory := newServerEvidenceRuntimeFunc
	t.Cleanup(func() {
		newServerEvidenceRuntimeFunc = oldFactory
	})

	provisionErr := errors.New("provision failed")
	destroyed := 0
	cleaned := 0
	newServerEvidenceRuntimeFunc = func(ctx context.Context, _ serverEvidenceOptions) (*serverEvidenceRuntime, error) {
		return &serverEvidenceRuntime{
			ctx:   ctx,
			infra: provisionedInfra{Applied: true, Hosts: map[string]provisionedHost{"aws-x86": {HostID: "aws-x86"}}},
			sourceCleanup: func() error {
				cleaned++
				return nil
			},
			provisionFunc: func(io.Writer) error {
				return provisionErr
			},
			destroyFunc: func() error {
				destroyed++
				return nil
			},
		}, nil
	}

	err := runServerEvidence(serverEvidenceOptions{tag: "v0.0.0-dev"}, io.Discard)
	if !errors.Is(err, provisionErr) {
		t.Fatalf("runServerEvidence error = %v, want %v", err, provisionErr)
	}
	if destroyed != 1 {
		t.Fatalf("destroy count = %d, want 1", destroyed)
	}
	if cleaned != 1 {
		t.Fatalf("source cleanup count = %d, want 1", cleaned)
	}
}

func TestRunServerEvidenceCleansUpAfterApplyWithoutHosts(t *testing.T) {
	oldFactory := newServerEvidenceRuntimeFunc
	t.Cleanup(func() {
		newServerEvidenceRuntimeFunc = oldFactory
	})

	provisionErr := errors.New("tofu output failed after apply")
	destroyed := 0
	runtimeState := &serverEvidenceRuntime{}
	runtimeState.provisionFunc = func(io.Writer) error {
		runtimeState.infra = provisionedInfra{Applied: true}
		return provisionErr
	}
	runtimeState.destroyFunc = func() error {
		destroyed++
		return nil
	}
	newServerEvidenceRuntimeFunc = func(context.Context, serverEvidenceOptions) (*serverEvidenceRuntime, error) {
		return runtimeState, nil
	}

	err := runServerEvidence(serverEvidenceOptions{tag: "v0.0.0-dev"}, io.Discard)
	if !errors.Is(err, provisionErr) {
		t.Fatalf("runServerEvidence error = %v, want %v", err, provisionErr)
	}
	if destroyed != 1 {
		t.Fatalf("destroy count = %d, want 1", destroyed)
	}
}

func TestRunServerEvidenceCleansUpExecuteFailure(t *testing.T) {
	oldFactory := newServerEvidenceRuntimeFunc
	t.Cleanup(func() {
		newServerEvidenceRuntimeFunc = oldFactory
	})

	executeErr := errors.New("release gate failed")
	destroyed := 0
	cleaned := 0
	newServerEvidenceRuntimeFunc = func(ctx context.Context, _ serverEvidenceOptions) (*serverEvidenceRuntime, error) {
		return &serverEvidenceRuntime{
			ctx: ctx,
			sourceCleanup: func() error {
				cleaned++
				return nil
			},
			provisionFunc: func(io.Writer) error {
				return nil
			},
			executeFunc: func(io.Writer) error {
				return executeErr
			},
			destroyFunc: func() error {
				destroyed++
				return nil
			},
		}, nil
	}

	err := runServerEvidence(serverEvidenceOptions{tag: "v0.0.0-dev"}, io.Discard)
	if !errors.Is(err, executeErr) {
		t.Fatalf("runServerEvidence error = %v, want %v", err, executeErr)
	}
	if destroyed != 1 {
		t.Fatalf("destroy count = %d, want 1", destroyed)
	}
	if cleaned != 1 {
		t.Fatalf("source cleanup count = %d, want 1", cleaned)
	}
}

func TestServerEvidenceDestroyUsesCleanupContextAndContinuesAfterBucketError(t *testing.T) {
	oldDelete := deleteStagingBucketFunc
	oldDestroy := destroyServerInfrastructureFunc
	t.Cleanup(func() {
		deleteStagingBucketFunc = oldDelete
		destroyServerInfrastructureFunc = oldDestroy
	})

	parent, cancel := context.WithCancel(context.Background())
	cancel()

	bucketCalled := false
	infraCalled := false
	deleteStagingBucketFunc = func(ctx context.Context, _ serverAWSClients, bucket string) error {
		bucketCalled = true
		if bucket != testRemoteStateBucket {
			t.Fatalf("bucket = %q, want %s", bucket, testRemoteStateBucket)
		}
		if ctx.Err() != nil {
			t.Fatalf("bucket cleanup context unexpectedly canceled: %v", ctx.Err())
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("bucket cleanup context missing deadline")
		}
		return errors.New("bucket cleanup failed")
	}
	destroyServerInfrastructureFunc = func(ctx context.Context, _ serverEvidenceOptions, _ serverToolchain, _, _ string) error {
		infraCalled = true
		if ctx.Err() != nil {
			t.Fatalf("infra cleanup context unexpectedly canceled: %v", ctx.Err())
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("infra cleanup context missing deadline")
		}
		return nil
	}

	runtimeState := &serverEvidenceRuntime{
		ctx:       parent,
		staging:   serverStaging{bucket: "bucket"},
		destroyed: false,
	}
	err := runtimeState.destroy()
	if err == nil || !strings.Contains(err.Error(), "bucket cleanup failed") {
		t.Fatalf("destroy error = %v, want bucket cleanup failure", err)
	}
	if !bucketCalled {
		t.Fatal("bucket cleanup was not called")
	}
	if !infraCalled {
		t.Fatal("infrastructure cleanup was not called after bucket cleanup failure")
	}
	if runtimeState.destroyed {
		t.Fatal("runtime marked destroyed despite cleanup error")
	}
}

func TestServerSSMAdapterRunReplayRejectsEvidenceSHAMismatch(t *testing.T) {
	oldPresignGet := presignGetObjectURLFunc
	oldPresignPut := presignPutObjectURLFunc
	oldRunSSM := runSSMShellScriptFunc
	oldDownload := downloadStagingObjectFunc
	oldChallenge := newTransportAttestationChallenge
	oldVerifyIID := verifyAWSInstanceIdentityFunc
	t.Cleanup(func() {
		presignGetObjectURLFunc = oldPresignGet
		presignPutObjectURLFunc = oldPresignPut
		runSSMShellScriptFunc = oldRunSSM
		downloadStagingObjectFunc = oldDownload
		newTransportAttestationChallenge = oldChallenge
		verifyAWSInstanceIdentityFunc = oldVerifyIID
	})

	presignGetObjectURLFunc = func(context.Context, serverAWSClients, string, string) (string, error) {
		return testObjectURL, nil
	}
	presignPutObjectURLFunc = func(context.Context, serverAWSClients, string, string) (string, error) {
		return testUploadURL, nil
	}
	verifyAWSInstanceIdentityFunc = func(rawDocument string, rawSignature string, host provisionedHost, expectedRegion string) (*awsInstanceIdentityDocument, error) {
		return &awsInstanceIdentityDocument{Region: "us-east-1", InstanceID: host.InstanceID, ImageID: host.ImageID}, nil
	}
	newTransportAttestationChallenge = func() (string, error) { return strings.Repeat("d", 64), nil }
	runSSMShellScriptFunc = func(context.Context, serverAWSClients, string, string, string, time.Duration) (string, error) {
		return testUploadedEvidence, nil
	}
	payload := []byte(`{"node_id":"aws-native-ubuntu","mode":"vm","distro":"ubuntu","kernel_family":"ga","replay_index":1,"session_id":"s","started_at_utc":"2026-01-01T00:00:00Z","completed_at_utc":"2026-01-01T00:00:01Z","case_count":1,"passed":true,"canonical_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","verify_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","failure_class_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","exit_code_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","discovered_cpu":"cpu","discovered_kernel":"kernel","measured_architecture":"x86_64","measured_os_id":"ubuntu","measured_os_version_id":"24.04","measured_kernel":"kernel","measured_cpu":"cpu","aws_instance_id":"i-123","aws_image_id":"ami-123"}`)
	attestation := mustTestTransportAttestationData(t, payload, strings.Repeat("c", 64), "aws-native-ubuntu", 1)
	downloadStagingObjectFunc = func(ctx context.Context, clients serverAWSClients, bucket, key string) ([]byte, error) {
		if strings.HasSuffix(key, "transport-attestation.v1.json") {
			return attestation, nil
		}
		return payload, nil
	}

	adapter := &serverSSMAdapter{
		bucket: "bucket",
		artifacts: stagedServerArtifacts{
			bundleKey: "bundle",
			workerKey: "worker",
		},
		hosts: map[string]provisionedHost{
			"aws-native-ubuntu": {HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123"},
		},
	}
	evidencePath := filepath.Join(t.TempDir(), "offline-evidence.json")
	err := adapter.RunReplay(context.Background(), replay.NodeSpec{
		ID:           "aws-native-ubuntu",
		Mode:         replay.NodeModeVM,
		Distro:       "ubuntu",
		KernelFamily: "ga",
	}, "", evidencePath, 1)
	if err == nil || !strings.Contains(err.Error(), "transport attestation challenge mismatch") {
		t.Fatalf("RunReplay error = %v, want attestation challenge mismatch", err)
	}
	if _, statErr := os.Stat(evidencePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("evidence file unexpectedly written, stat error = %v", statErr)
	}
}

func TestServerSSMAdapterRunReplayWritesVerifiedEvidence(t *testing.T) {
	oldPresignGet := presignGetObjectURLFunc
	oldPresignPut := presignPutObjectURLFunc
	oldRunSSM := runSSMShellScriptFunc
	oldDownload := downloadStagingObjectFunc
	oldChallenge := newTransportAttestationChallenge
	oldVerifyIID := verifyAWSInstanceIdentityFunc
	t.Cleanup(func() {
		presignGetObjectURLFunc = oldPresignGet
		presignPutObjectURLFunc = oldPresignPut
		runSSMShellScriptFunc = oldRunSSM
		downloadStagingObjectFunc = oldDownload
		verifyAWSInstanceIdentityFunc = oldVerifyIID
		newTransportAttestationChallenge = oldChallenge
	})

	payload := []byte(`{"node_id":"aws-native-ubuntu","mode":"vm","distro":"ubuntu","kernel_family":"ga","replay_index":1,"session_id":"s","started_at_utc":"2026-01-01T00:00:00Z","completed_at_utc":"2026-01-01T00:00:01Z","case_count":1,"passed":true,"canonical_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","verify_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","failure_class_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","exit_code_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","discovered_cpu":"cpu","discovered_kernel":"kernel","measured_architecture":"x86_64","measured_os_id":"ubuntu","measured_os_version_id":"24.04","measured_kernel":"kernel","measured_cpu":"cpu","aws_instance_id":"i-123","aws_image_id":"ami-123"}`)
	verifyAWSInstanceIdentityFunc = func(rawDocument string, rawSignature string, host provisionedHost, expectedRegion string) (*awsInstanceIdentityDocument, error) {
		return &awsInstanceIdentityDocument{Region: "us-east-1", InstanceID: host.InstanceID, ImageID: host.ImageID}, nil
	}
	newTransportAttestationChallenge = func() (string, error) { return strings.Repeat("c", 64), nil }
	presignGetObjectURLFunc = func(context.Context, serverAWSClients, string, string) (string, error) {
		return "https://example.com/object", nil
	}
	presignPutObjectURLFunc = func(context.Context, serverAWSClients, string, string) (string, error) {
		return "https://example.com/upload", nil
	}
	runSSMShellScriptFunc = func(context.Context, serverAWSClients, string, string, string, time.Duration) (string, error) {
		return "uploaded_evidence=1", nil
	}
	attestation := mustTestTransportAttestationData(t, payload, strings.Repeat("c", 64), "aws-native-ubuntu", 1)
	downloadStagingObjectFunc = func(ctx context.Context, clients serverAWSClients, bucket, key string) ([]byte, error) {
		if strings.HasSuffix(key, "transport-attestation.v1.json") {
			return attestation, nil
		}
		return payload, nil
	}

	adapter := &serverSSMAdapter{
		bucket: "bucket",
		artifacts: stagedServerArtifacts{
			bundleKey: "bundle",
			workerKey: "worker",
		},
		hosts: map[string]provisionedHost{
			"aws-native-ubuntu": {HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123"},
		},
	}
	evidencePath := filepath.Join(t.TempDir(), "offline-evidence.json")
	err := adapter.RunReplay(context.Background(), replay.NodeSpec{
		ID:           "aws-native-ubuntu",
		Mode:         replay.NodeModeVM,
		Distro:       "ubuntu",
		KernelFamily: "ga",
	}, "", evidencePath, 1)
	if err != nil {
		t.Fatalf("RunReplay: %v", err)
	}
	//nolint:gosec // REQ:AWS-GATE-001 orchestration test reads the temp evidence path it just produced.
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("read evidence file: %v", err)
	}
	if !strings.Contains(string(data), `"transport_attestation_sha256"`) {
		t.Fatalf("verified evidence missing transport attestation digest: %s", string(data))
	}
}

func TestServerSSMAdapterRunReplayPropagatesOversizeDownloadFailure(t *testing.T) {
	oldPresignGet := presignGetObjectURLFunc
	oldPresignPut := presignPutObjectURLFunc
	oldRunSSM := runSSMShellScriptFunc
	oldDownload := downloadStagingObjectFunc
	t.Cleanup(func() {
		presignGetObjectURLFunc = oldPresignGet
		presignPutObjectURLFunc = oldPresignPut
		runSSMShellScriptFunc = oldRunSSM
		downloadStagingObjectFunc = oldDownload
	})

	presignGetObjectURLFunc = func(context.Context, serverAWSClients, string, string) (string, error) {
		return "https://example.com/object", nil
	}
	presignPutObjectURLFunc = func(context.Context, serverAWSClients, string, string) (string, error) {
		return "https://example.com/upload", nil
	}
	runSSMShellScriptFunc = func(context.Context, serverAWSClients, string, string, string, time.Duration) (string, error) {
		return "uploaded_evidence=1", nil
	}
	downloadStagingObjectFunc = func(context.Context, serverAWSClients, string, string) ([]byte, error) {
		return nil, fmt.Errorf("read staging object s3://bucket/evidence exceeds maximum")
	}

	adapter := &serverSSMAdapter{
		bucket: "bucket",
		artifacts: stagedServerArtifacts{
			bundleKey: "bundle",
			workerKey: "worker",
		},
		hosts: map[string]provisionedHost{
			"aws-native-ubuntu": {InstanceID: "i-0123456789abcdef0"},
		},
	}

	err := adapter.RunReplay(context.Background(), replay.NodeSpec{
		ID:           "aws-native-ubuntu",
		Mode:         replay.NodeModeVM,
		Distro:       "ubuntu",
		KernelFamily: "ga",
	}, "", filepath.Join(t.TempDir(), "offline-evidence.json"), 1)
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("RunReplay error = %v, want oversize failure", err)
	}
}

func TestSubprocessHelpersUseProvidedContext(t *testing.T) {
	oldRun := runCommandInDirFunc
	t.Cleanup(func() {
		runCommandInDirFunc = oldRun
	})

	parent, cancel := context.WithCancel(context.Background())
	cancel()

	callCount := 0
	runCommandInDirFunc = func(ctx context.Context, dir string, env map[string]string, name string, args ...string) (string, error) {
		callCount++
		if ctx.Err() == nil {
			t.Fatalf("command %s %v received uncanceled context", name, args)
		}
		switch {
		case len(args) >= 2 && args[0] == "output" && args[1] == "-json":
			return `{"aws-x86":{"host_id":"aws-x86","node_id":"aws-native-ubuntu","architecture":"x86_64"}}`, nil
		case len(args) == 1 && args[0] == "version":
			return "OpenTofu v1.10.6", nil
		case len(args) >= 1 && args[0] == "build":
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("buildGoBinary context missing deadline")
			}
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s %v", name, args)
		}
	}

	hosts, err := tofuOutputHosts(parent, "/tmp/tofu", t.TempDir(), "provisioned_hosts")
	if err != nil {
		t.Fatalf("tofuOutputHosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("tofuOutputHosts host count = %d, want 1", len(hosts))
	}
	version, err := resolveTofuVersion(parent, "/tmp/tofu", t.TempDir())
	if err != nil {
		t.Fatalf("resolveTofuVersion: %v", err)
	}
	if version != "1.10.6" {
		t.Fatalf("resolveTofuVersion = %q, want 1.10.6", version)
	}
	buildOut := filepath.Join(t.TempDir(), "bin", "jcs-canon")
	if err := buildGoBinary(parent, "/tmp/go", t.TempDir(), "x86_64", "v0.0.0-dev", "./cmd/jcs-canon", buildOut); err != nil {
		t.Fatalf("buildGoBinary: %v", err)
	}
	if callCount != 3 {
		t.Fatalf("runCommandInDir call count = %d, want 3", callCount)
	}
}

func TestProvisionRetainsAppliedInfrastructureOnOutputFailure(t *testing.T) {
	oldProvision := provisionServerInfrastructureFunc
	t.Cleanup(func() {
		provisionServerInfrastructureFunc = oldProvision
	})

	recordPath := filepath.Join(t.TempDir(), "server-run.v1.json")
	runtimeState := &serverEvidenceRuntime{
		ctx:           context.Background(),
		runRecordPath: recordPath,
		runRecord: serverRunRecord{
			SchemaVersion:   serverRunRecordSchemaVersion,
			ProvisionStatus: serverRunStatusPending,
		},
		opts:      serverEvidenceOptions{tag: "v0.0.0-dev"},
		gitCommit: strings.Repeat("a", 40),
		lockSHA:   strings.Repeat("b", 64),
	}
	provisionServerInfrastructureFunc = func(context.Context, serverEvidenceOptions, serverToolchain, string, string) (provisionedInfra, error) {
		return provisionedInfra{Applied: true}, errors.New("tofu output failed")
	}

	err := runtimeState.provision(io.Discard)
	if err == nil || !strings.Contains(err.Error(), "tofu output failed") {
		t.Fatalf("provision error = %v, want tofu output failure", err)
	}
	if !runtimeState.infra.Applied {
		t.Fatal("runtime did not retain applied infrastructure state")
	}
	if runtimeState.runRecord.ProvisionStatus != serverRunStatusFailed {
		t.Fatalf("provision status = %q, want %q", runtimeState.runRecord.ProvisionStatus, serverRunStatusFailed)
	}
}

func TestPrepareStagingUploadsArchitectureArtifacts(t *testing.T) {
	oldCreate := createStagingBucketFunc
	oldUpload := uploadStagingFileFunc
	t.Cleanup(func() {
		createStagingBucketFunc = oldCreate
		uploadStagingFileFunc = oldUpload
	})

	createStagingBucketFunc = func(context.Context, serverAWSClients, string) (string, error) {
		return testRemoteStateBucket, nil
	}
	uploads := make(map[string]string)
	uploadStagingFileFunc = func(_ context.Context, _ serverAWSClients, bucket, key, path string) error {
		if bucket != testRemoteStateBucket {
			t.Fatalf("upload bucket = %q, want %s", bucket, testRemoteStateBucket)
		}
		uploads[key] = path
		return nil
	}

	tempDir := t.TempDir()
	recordPath := filepath.Join(tempDir, "server-run.v1.json")
	runtimeState := &serverEvidenceRuntime{
		ctx:           context.Background(),
		runRecordPath: recordPath,
		runRecord: serverRunRecord{
			SchemaVersion: serverRunRecordSchemaVersion,
			StagingStatus: serverRunStatusPending,
		},
		opts: serverEvidenceOptions{tag: "v0.0.0-dev"},
		x86Artifacts: serverBuildArtifacts{
			bundlePath: filepath.Join(tempDir, "x86-bundle.tgz"),
			workerPath: filepath.Join(tempDir, "x86-worker"),
		},
		armArtifacts: serverBuildArtifacts{
			bundlePath: filepath.Join(tempDir, "arm-bundle.tgz"),
			workerPath: filepath.Join(tempDir, "arm-worker"),
		},
	}

	if err := runtimeState.prepareStaging(io.Discard); err != nil {
		t.Fatalf("prepareStaging: %v", err)
	}
	if runtimeState.staging.bucket != "bucket" {
		t.Fatalf("staging bucket = %q, want bucket", runtimeState.staging.bucket)
	}
	if runtimeState.runRecord.StagingStatus != serverRunStatusSucceeded {
		t.Fatalf("staging status = %q, want %q", runtimeState.runRecord.StagingStatus, serverRunStatusSucceeded)
	}
	if runtimeState.runRecord.StagingBucket != "bucket" {
		t.Fatalf("staging bucket in run record = %q, want bucket", runtimeState.runRecord.StagingBucket)
	}
	record, err := loadServerRunRecord(recordPath)
	if err != nil {
		t.Fatalf("loadServerRunRecord: %v", err)
	}
	if record.StagingStatus != serverRunStatusSucceeded {
		t.Fatalf("persisted staging status = %q, want %q", record.StagingStatus, serverRunStatusSucceeded)
	}
	for key, wantPath := range map[string]string{
		"x86_64/offline-bundle.tgz": runtimeState.x86Artifacts.bundlePath,
		"x86_64/jcs-offline-worker": runtimeState.x86Artifacts.workerPath,
		"arm64/offline-bundle.tgz":  runtimeState.armArtifacts.bundlePath,
		"arm64/jcs-offline-worker":  runtimeState.armArtifacts.workerPath,
	} {
		if uploads[key] != wantPath {
			t.Fatalf("upload %s path = %q, want %q", key, uploads[key], wantPath)
		}
	}
}

func TestDiscoverHostFactsReadsAndValidatesNativeIdentity(t *testing.T) {
	oldPresignGet := presignGetObjectURLFunc
	oldRunSSM := runSSMShellScriptFunc
	oldVerifyIID := verifyAWSInstanceIdentityFunc
	t.Cleanup(func() {
		presignGetObjectURLFunc = oldPresignGet
		runSSMShellScriptFunc = oldRunSSM
		verifyAWSInstanceIdentityFunc = oldVerifyIID
	})

	presignGetObjectURLFunc = func(context.Context, serverAWSClients, string, string) (string, error) {
		return "https://example.com/worker", nil
	}
	verifyAWSInstanceIdentityFunc = func(rawDocument string, rawSignature string, host provisionedHost, expectedRegion string) (*awsInstanceIdentityDocument, error) {
		return &awsInstanceIdentityDocument{Region: "us-east-1", InstanceID: host.InstanceID, ImageID: host.ImageID}, nil
	}
	runSSMShellScriptFunc = func(context.Context, serverAWSClients, string, string, string, time.Duration) (string, error) {
		return `{"architecture":"x86_64","os_id":"ubuntu","os_version_id":"24.04","cpu":"Intel","kernel":"6.8.0","instance_id":"i-0123456789abcdef0","image_id":"ami-0abc1234","availability_zone":"us-east-1a","region":"us-east-1","iid_document":"{\"availabilityZone\":\"us-east-1a\",\"imageId\":\"ami-0abc1234\",\"instanceId\":\"i-0123456789abcdef0\",\"region\":\"us-east-1\"}","iid_signature":"c2lnbmF0dXJl","iid_pkcs7":"cGtjczc=","iid_document_sha256":"` +
			strings.Repeat("1", 64) + `","iid_signature_sha256":"` + strings.Repeat("2", 64) + `","iid_pkcs7_sha256":"` + strings.Repeat("3", 64) + `","iid_verified":false}`, nil
	}

	runtimeState := &serverEvidenceRuntime{
		ctx:     context.Background(),
		staging: serverStaging{bucket: "bucket", x86: stagedServerArtifacts{workerKey: "x86_64/jcs-offline-worker"}},
	}
	facts, err := runtimeState.discoverHostFacts(provisionedHost{
		HostID:       "aws-native-ubuntu",
		Architecture: "x86_64",
		InstanceID:   "i-0123456789abcdef0",
		ImageID:      "ami-0abc1234",
	})
	if err != nil {
		t.Fatalf("discoverHostFacts: %v", err)
	}
	if facts.OSID != "ubuntu" {
		t.Fatalf("os_id = %q, want ubuntu", facts.OSID)
	}
	if facts.Region != "us-east-1" {
		t.Fatalf("region = %q, want us-east-1", facts.Region)
	}
}

func TestRunServerCleanupIdempotentAfterSuccessfulDestroy(t *testing.T) {
	oldDelete := deleteStagingBucketFunc
	oldDestroy := destroyServerInfrastructureFunc
	t.Cleanup(func() {
		deleteStagingBucketFunc = oldDelete
		destroyServerInfrastructureFunc = oldDestroy
	})

	deleteCalled := false
	destroyCalled := false
	deleteStagingBucketFunc = func(context.Context, serverAWSClients, string) error {
		deleteCalled = true
		return nil
	}
	destroyServerInfrastructureFunc = func(context.Context, serverEvidenceOptions, serverToolchain, string, string) error {
		destroyCalled = true
		return nil
	}

	tempDir := t.TempDir()
	infraManifestPath := filepath.Join(tempDir, "infra-manifest.v1.json")
	x86EvidencePath := filepath.Join(tempDir, "x86_64", "offline-evidence.json")
	armEvidencePath := filepath.Join(tempDir, "arm64", "offline-evidence.json")
	for path, data := range map[string]string{
		infraManifestPath: `{"schema_version":"infra-manifest.v1"}`,
		x86EvidencePath:   `{"schema_version":"evidence.v1"}`,
		armEvidencePath:   `{"schema_version":"evidence.v1"}`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(data), filePerm); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	recordPath := filepath.Join(tempDir, "server-run.v1.json")
	record := &serverRunRecord{
		SchemaVersion:     serverRunRecordSchemaVersion,
		RunRecordPath:     recordPath,
		OutputDir:         tempDir,
		Tag:               "v1.2.3",
		SourceGitCommit:   strings.Repeat("a", 40),
		SourceGitTag:      "v1.2.3",
		InfraManifestPath: infraManifestPath,
		X86EvidencePath:   x86EvidencePath,
		ArmEvidencePath:   armEvidencePath,
		DestroyStatus:     serverRunStatusSucceeded,
		StateMode:         serverStateModeRemote,
	}
	if err := writeServerRunRecord(recordPath, record); err != nil {
		t.Fatalf("writeServerRunRecord: %v", err)
	}

	var stdout strings.Builder
	if err := runServerCleanup(record, &stdout); err != nil {
		t.Fatalf("runServerCleanup: %v", err)
	}
	if deleteCalled {
		t.Fatal("delete staging bucket unexpectedly called for idempotent cleanup")
	}
	if destroyCalled {
		t.Fatal("destroy infrastructure unexpectedly called for idempotent cleanup")
	}
	if !strings.Contains(stdout.String(), "cleanup already complete") {
		t.Fatalf("stdout = %q, want already complete message", stdout.String())
	}
	for _, path := range []string{
		filepath.Join(tempDir, "audit", "server-evidence-summary.json"),
		filepath.Join(tempDir, "audit", "server-evidence-summary.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}

func TestRunServerCleanupSucceedsWithoutEvidenceArtifacts(t *testing.T) {
	oldDelete := deleteStagingBucketFunc
	oldDestroy := destroyServerInfrastructureFunc
	oldAWSClients := newServerAWSClientsFunc
	t.Cleanup(func() {
		deleteStagingBucketFunc = oldDelete
		destroyServerInfrastructureFunc = oldDestroy
		newServerAWSClientsFunc = oldAWSClients
	})

	deleteStagingBucketFunc = func(context.Context, serverAWSClients, string) error {
		return nil
	}
	destroyServerInfrastructureFunc = func(context.Context, serverEvidenceOptions, serverToolchain, string, string) error {
		return nil
	}
	newServerAWSClientsFunc = func(context.Context, string) (serverAWSClients, error) {
		return serverAWSClients{}, nil
	}

	tempDir := t.TempDir()
	recordPath := filepath.Join(tempDir, "server-run.v1.json")
	record := &serverRunRecord{
		SchemaVersion:      serverRunRecordSchemaVersion,
		RunRecordPath:      recordPath,
		OutputDir:          tempDir,
		RepoRoot:           tempDir,
		Tag:                "v1.2.3",
		SourceGitCommit:    strings.Repeat("a", 40),
		SourceGitTag:       "v1.2.3",
		AWSRegion:          "us-east-1",
		StateMode:          serverStateModeRemote,
		InfraDir:           filepath.Join(tempDir, "infra"),
		ProviderLockPath:   filepath.Join(tempDir, "infra", ".terraform.lock.hcl"),
		ProviderLockSHA256: strings.Repeat("b", 64),
		AMILockPath:        filepath.Join(tempDir, "infra", "aws_release_hosts.lock.json"),
		StagingBucket:      "bucket",
		InfraManifestPath:  filepath.Join(tempDir, "infra-manifest.v1.json"),
		X86EvidencePath:    filepath.Join(tempDir, "x86_64", "offline-evidence.json"),
		ArmEvidencePath:    filepath.Join(tempDir, "arm64", "offline-evidence.json"),
		DestroyStatus:      serverRunStatusPending,
		RunStatus:          serverRunStatusRunning,
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "infra"), dirPerm); err != nil {
		t.Fatalf("mkdir infra: %v", err)
	}
	if err := os.WriteFile(record.ProviderLockPath, []byte("lock"), filePerm); err != nil {
		t.Fatalf("write provider lock: %v", err)
	}
	if err := os.WriteFile(record.AMILockPath, []byte("ami-lock"), filePerm); err != nil {
		t.Fatalf("write ami lock: %v", err)
	}
	for _, envVar := range []string{"JCS_TOOL_GO", "JCS_TOOL_TOFU"} {
		toolPath := filepath.Join(tempDir, strings.ToLower(envVar))
		if err := os.WriteFile(toolPath, []byte("tool"), filePerm); err != nil {
			t.Fatalf("write %s: %v", envVar, err)
		}
		t.Setenv(envVar, toolPath)
	}

	var stdout strings.Builder
	if err := runServerCleanup(record, &stdout); err != nil {
		t.Fatalf("runServerCleanup: %v", err)
	}
	if record.DestroyStatus != serverRunStatusSucceeded {
		t.Fatalf("destroy status = %q, want %q", record.DestroyStatus, serverRunStatusSucceeded)
	}
	for _, path := range []string{
		filepath.Join(tempDir, "audit", "server-evidence-summary.json"),
		filepath.Join(tempDir, "audit", "server-evidence-summary.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}
