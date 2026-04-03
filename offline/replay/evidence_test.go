package replay_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const (
	testCPUIntel  = "Intel"
	testCPUArm    = "Neoverse"
	testKernel610 = "6.1.0"
)

func TestValidateEvidenceBundleParity(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err != nil {
		t.Fatalf("validate evidence: %v", err)
	}
}

func TestValidateEvidenceBundleDetectsDrift(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	e.NodeReplays[3] = mkRun("v1", "vm", "ubuntu", "ga", 2, strings.Repeat("b", 64))
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err == nil {
		t.Fatal("expected drift validation error")
	}
}

func TestValidateEvidenceBundleRejectsTamperedMetadata(t *testing.T) {
	m, p, base, opts := validEvidenceFixture()
	tests := []struct {
		name   string
		tamper func(*replay.EvidenceBundle)
		want   string
	}{
		{name: "bundle_sha256", tamper: func(e *replay.EvidenceBundle) { e.BundleSHA256 = strings.Repeat("b", 64) }, want: "bundle_sha256 mismatch"},
		{name: "control_binary_sha256", tamper: func(e *replay.EvidenceBundle) { e.ControlBinarySHA = strings.Repeat("b", 64) }, want: "control_binary_sha256 mismatch"},
		{name: "matrix_sha256", tamper: func(e *replay.EvidenceBundle) { e.MatrixSHA256 = strings.Repeat("b", 64) }, want: "matrix_sha256 mismatch"},
		{name: "profile_sha256", tamper: func(e *replay.EvidenceBundle) { e.ProfileSHA256 = strings.Repeat("b", 64) }, want: "profile_sha256 mismatch"},
		{name: "vector_set_sha256", tamper: func(e *replay.EvidenceBundle) { e.VectorSetSHA256 = strings.Repeat("b", 64) }, want: "vector_set_sha256 mismatch"},
		{name: "architecture", tamper: func(e *replay.EvidenceBundle) { e.Architecture = "arm64" }, want: "architecture mismatch"},
		{name: "source_git_commit", tamper: func(e *replay.EvidenceBundle) { e.SourceGitCommit = strings.Repeat("b", 40) }, want: "source_git_commit mismatch"},
		{name: "source_git_tag", tamper: func(e *replay.EvidenceBundle) { e.SourceGitTag = "v0.0.0-wrong" }, want: "source_git_tag mismatch"},
		{name: "aggregate_method", tamper: func(e *replay.EvidenceBundle) { e.AggregateMethod = "legacy" }, want: "aggregate_method mismatch"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := cloneEvidence(base)
			tc.tamper(e)
			err := replay.ValidateEvidenceBundle(e, m, p, opts)
			if err == nil {
				t.Fatalf("expected %s validation error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateEvidenceBundleRejectsMalformedNodeDigestTokens(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	e.NodeReplays[0].CanonicalSHA256 = "abc"

	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected malformed canonical digest validation error")
	}
	if !strings.Contains(err.Error(), "canonical_sha256 must be 64 hex characters") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleRejectsMalformedAggregateDigestTokens(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	e.AggregateCanonical = "abc"

	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected malformed aggregate digest validation error")
	}
	if !strings.Contains(err.Error(), "aggregate_canonical_sha256 must be 64 hex characters") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleInfraBoundV1Parity(t *testing.T) {
	m, p, e, opts := validInfraBoundEvidenceFixture()
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err != nil {
		t.Fatalf("validate infra-bound v1 evidence: %v", err)
	}
}

func TestValidateEvidenceBundleNativeHostV1Parity(t *testing.T) {
	m, p, e, opts := validNativeHostEvidenceFixture()
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err != nil {
		t.Fatalf("validate native-host v1 evidence: %v", err)
	}
}

func TestValidateEvidenceBundleInfraBindingRejectsPartialTopLevelBinding(t *testing.T) {
	m, p, e, opts := validInfraBoundEvidenceFixture()
	e.InfraRepoCommit = ""
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected infra binding validation error")
	}
	if !strings.Contains(err.Error(), "infra_repo_commit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleInfraBindingRequiresManifest(t *testing.T) {
	m, p, e, opts := validInfraBoundEvidenceFixture()
	opts.ExpectedInfraManifest = nil
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected missing infra manifest error")
	}
	if !strings.Contains(err.Error(), "requires expected infra manifest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleInfraBindingRequiresDiscoveredFields(t *testing.T) {
	m, p, e, opts := validInfraBoundEvidenceFixture()
	e.NodeReplays[0].DiscoveredCPU = ""
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected discovered field validation error")
	}
	if !strings.Contains(err.Error(), "discovered_cpu") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleRejectsImageDigestOnVMLane(t *testing.T) {
	m, p, e, opts := validInfraBoundEvidenceFixture()
	e.NodeReplays[2].ImageDigest = "debian@sha256:" + strings.Repeat("2", 64)
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected vm image_digest validation error")
	}
	if !strings.Contains(err.Error(), "image_digest is only allowed for container lanes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleNativeHostRequiresMeasuredFields(t *testing.T) {
	m, p, e, opts := validNativeHostEvidenceFixture()
	e.NodeReplays[0].MeasuredCPU = ""
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected measured field validation error")
	}
	if !strings.Contains(err.Error(), "measured_cpu") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleNativeHostManifestMismatch(t *testing.T) {
	m, p, e, opts := validNativeHostEvidenceFixture()
	e.NodeReplays[0].AWSInstanceID = "i-wrong"
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected manifest-bound mismatch")
	}
	if !strings.Contains(err.Error(), "aws_instance_id mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadEvidenceRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	dir := t.TempDir()
	unknownPath := filepath.Join(dir, "unknown.json")
	unknownDoc := `{"schema_version":"evidence.v1","bundle_sha256":"` + strings.Repeat("a", 64) + `","control_binary_sha256":"` + strings.Repeat("a", 64) + `","matrix_sha256":"` + strings.Repeat("a", 64) + `","profile_sha256":"` + strings.Repeat("a", 64) + `","vector_set_sha256":"` + strings.Repeat("a", 64) + `","source_git_commit":"` + strings.Repeat("b", 40) + `","source_git_tag":"v0.0.0","generated_at_utc":"2026-01-01T00:00:00Z","orchestrator":"jcs-offline-replay","profile_id":"https://lattice-substrate.github.io/jcs/profiles/offline-measured-evidence.v1","profile_name":"offline-measured-evidence","architecture":"x86_64","aggregate_method":"replay-aggregate.v1","required_suites":["canonical-byte-stability"],"hard_release_gate":true,"node_replays":[{"node_id":"c1","mode":"container","distro":"debian","kernel_family":"host","replay_index":1,"session_id":"s","started_at_utc":"2026-01-01T00:00:00Z","completed_at_utc":"2026-01-01T00:00:01Z","case_count":1,"passed":true,"canonical_sha256":"` + strings.Repeat("a", 64) + `","verify_sha256":"` + strings.Repeat("a", 64) + `","failure_class_sha256":"` + strings.Repeat("a", 64) + `","exit_code_sha256":"` + strings.Repeat("a", 64) + `"}],"aggregate_canonical_sha256":"` + strings.Repeat("a", 64) + `","aggregate_verify_sha256":"` + strings.Repeat("a", 64) + `","aggregate_failure_class_sha256":"` + strings.Repeat("a", 64) + `","aggregate_exit_code_sha256":"` + strings.Repeat("a", 64) + `","unknown":true}`
	if err := os.WriteFile(unknownPath, []byte(unknownDoc), 0o600); err != nil {
		t.Fatalf("write unknown fixture: %v", err)
	}
	if _, err := replay.LoadEvidence(unknownPath); err == nil {
		t.Fatal("expected unknown field error")
	}

	trailingPath := filepath.Join(dir, "trailing.json")
	if err := os.WriteFile(trailingPath, []byte(unknownDoc[:len(unknownDoc)-17]+`}`+`{"trailing":true}`), 0o600); err != nil {
		t.Fatalf("write trailing fixture: %v", err)
	}
	if _, err := replay.LoadEvidence(trailingPath); err == nil {
		t.Fatal("expected trailing json error")
	}
}

func validEvidenceFixture() (*replay.Matrix, *replay.Profile, *replay.EvidenceBundle, replay.EvidenceValidationOptions) {
	m := &replay.Matrix{
		Version:      "v1",
		Architecture: "x86_64",
		Nodes: []replay.NodeSpec{
			{ID: "c1", Mode: replay.NodeModeContainer, Distro: "debian", KernelFamily: "host", Replays: 2, Runner: replay.RunnerConfig{Kind: "container_command", Replay: []string{"true"}}},
			{ID: "v1", Mode: replay.NodeModeVM, Distro: "ubuntu", KernelFamily: "ga", Replays: 2, Runner: replay.RunnerConfig{Kind: "libvirt_command", Replay: []string{"true"}}},
		},
	}
	p := &replay.Profile{
		Version:          "v1",
		Name:             "max",
		RequiredSuites:   []string{"canonical-byte-stability"},
		MinColdReplays:   2,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}
	digest := strings.Repeat("a", 64)
	sourceCommit := strings.Repeat("f", 40)
	sourceTag := "v0.0.0-dev"
	e := &replay.EvidenceBundle{
		SchemaVersion:      replay.EvidenceSchemaVersion,
		BundleSHA256:       digest,
		ControlBinarySHA:   digest,
		MatrixSHA256:       digest,
		ProfileSHA256:      digest,
		VectorSetSHA256:    digest,
		SourceGitCommit:    sourceCommit,
		SourceGitTag:       sourceTag,
		GeneratedAtUTC:     "2026-01-01T00:00:00Z",
		Orchestrator:       "jcs-offline-replay",
		ProfileID:          "https://lattice-substrate.github.io/jcs/profiles/base-conformance.v1",
		ProfileName:        "base-conformance",
		Architecture:       "x86_64",
		AggregateMethod:    replay.ReplayAggregateMethod,
		HardReleaseGate:    true,
		RequiredSuites:     []string{"canonical-byte-stability"},
		NodeReplays: []replay.NodeRunEvidence{
			mkRun("c1", "container", "debian", "host", 1, digest),
			mkRun("c1", "container", "debian", "host", 2, digest),
			mkRun("v1", "vm", "ubuntu", "ga", 1, digest),
			mkRun("v1", "vm", "ubuntu", "ga", 2, digest),
		},
	}
	e.AggregateCanonical = testReplayAggregateDigest(e.NodeReplays, func(run replay.NodeRunEvidence) string { return run.CanonicalSHA256 })
	e.AggregateVerify = testReplayAggregateDigest(e.NodeReplays, func(run replay.NodeRunEvidence) string { return run.VerifySHA256 })
	e.AggregateClass = testReplayAggregateDigest(e.NodeReplays, func(run replay.NodeRunEvidence) string { return run.FailureClassSHA256 })
	e.AggregateExitCode = testReplayAggregateDigest(e.NodeReplays, func(run replay.NodeRunEvidence) string { return run.ExitCodeSHA256 })
	opts := replay.EvidenceValidationOptions{
		ExpectedBundleSHA256:        digest,
		ExpectedControlBinarySHA256: digest,
		ExpectedMatrixSHA256:        digest,
		ExpectedProfileSHA256:       digest,
		ExpectedVectorSetSHA256:     digest,
		ExpectedArchitecture:        "x86_64",
		ExpectedSourceGitCommit:     sourceCommit,
		ExpectedSourceGitTag:        sourceTag,
	}
	return m, p, e, opts
}

func validInfraBoundEvidenceFixture() (*replay.Matrix, *replay.Profile, *replay.EvidenceBundle, replay.EvidenceValidationOptions) {
	m, _, e, opts := validEvidenceFixture()
	p := &replay.Profile{
		Version:          "v1",
		Name:             "infra-bound",
		RequiredSuites:   []string{"canonical-byte-stability", "infra-substrate-binding"},
		MinColdReplays:   2,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}
	e.ProfileID = "https://lattice-substrate.github.io/jcs/profiles/offline-measured-evidence.v1"
	e.ProfileName = "offline-measured-evidence"
	e.RequiredSuites = append([]string(nil), p.RequiredSuites...)
	e.InfraManifestSHA256 = strings.Repeat("c", 64)
	e.InfraRepoURL = "https://github.com/example/json-canon-conformance-infra"
	e.InfraRepoCommit = strings.Repeat("d", 40)
	e.IIDTrustRootSetID = "aws-iid-trust-roots.v1"
	e.NodeReplays[0].DiscoveredCPU = testCPUIntel
	e.NodeReplays[0].DiscoveredKernel = testKernel610
	e.NodeReplays[0].ImageDigest = "debian@sha256:" + strings.Repeat("1", 64)
	e.NodeReplays[1].DiscoveredCPU = testCPUIntel
	e.NodeReplays[1].DiscoveredKernel = testKernel610
	e.NodeReplays[1].ImageDigest = "debian@sha256:" + strings.Repeat("1", 64)
	e.NodeReplays[2].DiscoveredCPU = testCPUArm
	e.NodeReplays[2].DiscoveredKernel = testKernel610
	e.NodeReplays[3].DiscoveredCPU = testCPUArm
	e.NodeReplays[3].DiscoveredKernel = testKernel610
	opts.ExpectedInfraManifestSHA256 = e.InfraManifestSHA256
	opts.ExpectedInfraRepoURL = e.InfraRepoURL
	opts.ExpectedInfraRepoCommit = e.InfraRepoCommit
	opts.ExpectedInfraManifest = infraBoundManifestFixture(e.InfraRepoCommit)
	return m, p, e, opts
}

func validNativeHostEvidenceFixture() (*replay.Matrix, *replay.Profile, *replay.EvidenceBundle, replay.EvidenceValidationOptions) {
	digest := strings.Repeat("a", 64)
	sourceCommit := strings.Repeat("f", 40)
	sourceTag := "v0.0.0-dev"
	matrix := &replay.Matrix{
		Version:      "v1",
		Architecture: "x86_64",
		Nodes: []replay.NodeSpec{
			{ID: "aws-native-ubuntu", Mode: replay.NodeModeVM, Distro: "ubuntu", KernelFamily: "ga", Replays: 2, Runner: replay.RunnerConfig{Kind: "vm_ssm", Replay: []string{"true"}}},
		},
	}
	profile := &replay.Profile{
		Version:          "v1",
		Name:             "aws-native-release-linux-x86_64",
		RequiredSuites:   []string{"canonical-byte-stability", "infra-substrate-binding"},
		MinColdReplays:   2,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}
	manifest := nativeHostManifestFixture(strings.Repeat("d", 40))
	e := &replay.EvidenceBundle{
		SchemaVersion:       replay.EvidenceSchemaVersion,
		BundleSHA256:        digest,
		ControlBinarySHA:    digest,
		MatrixSHA256:        digest,
		ProfileSHA256:       digest,
		VectorSetSHA256:     digest,
		SourceGitCommit:     sourceCommit,
		SourceGitTag:        sourceTag,
		GeneratedAtUTC:      "2026-01-01T00:00:00Z",
		Orchestrator:        "jcs-offline-replay server-evidence",
		ProfileID:           "https://lattice-substrate.github.io/jcs/profiles/official-cloud-measured-release.v1",
		ProfileName:         "official-cloud-measured-release",
		Architecture:        "x86_64",
		AggregateMethod:     replay.ReplayAggregateMethod,
		RequiredSuites:      append([]string(nil), profile.RequiredSuites...),
		HardReleaseGate:     true,
		InfraManifestSHA256: strings.Repeat("c", 64),
		InfraRepoURL:        manifest.InfraRepoURL,
		InfraRepoCommit:     manifest.InfraRepoCommit,
		IIDTrustRootSetID:   "aws-iid-trust-roots.v1",
		NodeReplays: []replay.NodeRunEvidence{
			nativeRun("aws-native-ubuntu", 1, digest),
			nativeRun("aws-native-ubuntu", 2, digest),
		},
	}
	e.AggregateCanonical = testReplayAggregateDigest(e.NodeReplays, func(run replay.NodeRunEvidence) string { return run.CanonicalSHA256 })
	e.AggregateVerify = testReplayAggregateDigest(e.NodeReplays, func(run replay.NodeRunEvidence) string { return run.VerifySHA256 })
	e.AggregateClass = testReplayAggregateDigest(e.NodeReplays, func(run replay.NodeRunEvidence) string { return run.FailureClassSHA256 })
	e.AggregateExitCode = testReplayAggregateDigest(e.NodeReplays, func(run replay.NodeRunEvidence) string { return run.ExitCodeSHA256 })
	opts := replay.EvidenceValidationOptions{
		ExpectedBundleSHA256:        digest,
		ExpectedControlBinarySHA256: digest,
		ExpectedMatrixSHA256:        digest,
		ExpectedProfileSHA256:       digest,
		ExpectedVectorSetSHA256:     digest,
		ExpectedArchitecture:        "x86_64",
		ExpectedSourceGitCommit:     sourceCommit,
		ExpectedSourceGitTag:        sourceTag,
		ExpectedInfraManifestSHA256: e.InfraManifestSHA256,
		ExpectedInfraRepoURL:        manifest.InfraRepoURL,
		ExpectedInfraRepoCommit:     manifest.InfraRepoCommit,
		ExpectedInfraManifest:       manifest,
	}
	return matrix, profile, e, opts
}

func infraBoundManifestFixture(commit string) *replay.InfraManifest {
	return &replay.InfraManifest{
		SchemaVersion:      replay.InfraManifestSchemaVersion,
		GeneratedAtUTC:     "2026-01-01T00:00:00Z",
		InfraRepoURL:       "https://github.com/example/json-canon-conformance-infra",
		InfraRepoCommit:    commit,
		ProviderEngine:     "opentofu",
		ProviderVersion:    "1.8.0",
		ProviderLockSHA256: strings.Repeat("b", 64),
		Hosts: []replay.InfraManifestHost{
			{
				Architecture:     "x86_64",
				NodeIDs:          []string{"c1"},
				Role:             "container",
				CloudProvider:    "aws",
				Region:           "us-east-1",
				InstanceType:     "c6i.large",
				ImageID:          "debian@sha256:" + strings.Repeat("1", 64),
				DiscoveredCPU:    testCPUIntel,
				DiscoveredKernel: testKernel610,
			},
			{
				Architecture:     "x86_64",
				NodeIDs:          []string{"v1"},
				Role:             "vm",
				CloudProvider:    "aws",
				Region:           "us-east-1",
				InstanceType:     "c6i.large",
				ImageID:          "ami-0abc1234",
				DiscoveredCPU:    testCPUArm,
				DiscoveredKernel: testKernel610,
			},
		},
		Tools: manifestToolsFixture(),
	}
}

func nativeHostManifestFixture(commit string) *replay.InfraManifest {
	return &replay.InfraManifest{
		SchemaVersion:      replay.InfraManifestSchemaVersion,
		GeneratedAtUTC:     "2026-01-01T00:00:00Z",
		InfraRepoURL:       "https://github.com/example/json-canon-conformance-infra",
		InfraRepoCommit:    commit,
		ProviderEngine:     "opentofu",
		ProviderVersion:    "1.8.0",
		ProviderLockSHA256: strings.Repeat("b", 64),
		Hosts: []replay.InfraManifestHost{
			{
				Architecture:       "x86_64",
				NodeIDs:            []string{"aws-native-ubuntu"},
				Role:               "x86_64",
				CloudProvider:      "aws",
				Region:             "us-east-1",
				AvailabilityZone:   "us-east-1a",
				InstanceType:       "c6i.large",
				InstanceID:         "i-0123456789abcdef0",
				ImageID:            "ami-0abc1234",
				OSID:               "ubuntu",
				OSVersionID:        "22.04",
				CPU:                testCPUArm,
				Kernel:             testKernel610,
				IIDDocumentSHA256:  strings.Repeat("1", 64),
				IIDSignatureSHA256: strings.Repeat("2", 64),
				IIDPKCS7SHA256:     strings.Repeat("3", 64),
				IIDVerified:        true,
				Transport:          "ssm",
				SubnetVisibility:   "private",
				DiscoveredCPU:      testCPUArm,
				DiscoveredKernel:   testKernel610,
			},
		},
		Tools: manifestToolsFixture(),
	}
}

func manifestToolsFixture() []replay.InfraManifestTool {
	return []replay.InfraManifestTool{
		{
			ID:                     "go-linux-amd64",
			Scope:                  replay.ToolchainScopeHost,
			Purpose:                "build",
			Name:                   "go",
			Version:                "1.24.13",
			OS:                     "linux",
			Arch:                   "amd64",
			Format:                 "tar.gz",
			SourceURL:              "https://go.dev/dl/go1.24.13.linux-amd64.tar.gz",
			SHA256:                 strings.Repeat("c", 64),
			ArtifactRelativePath:   "toolchain/downloads/go-linux-amd64/go1.24.13.linux-amd64.tar.gz",
			ExecutableRelativePath: "toolchain/.extracted/go-linux-amd64/go/bin/go",
		},
	}
}

func cloneEvidence(in *replay.EvidenceBundle) *replay.EvidenceBundle {
	out := *in
	out.RequiredSuites = append([]string(nil), in.RequiredSuites...)
	out.NodeReplays = append([]replay.NodeRunEvidence(nil), in.NodeReplays...)
	return &out
}

func mkRun(nodeID, mode, distro, kernel string, replayIndex int, digest string) replay.NodeRunEvidence {
	return replay.NodeRunEvidence{
		NodeID:             nodeID,
		Mode:               mode,
		Distro:             distro,
		KernelFamily:       kernel,
		ReplayIndex:        replayIndex,
		SessionID:          "sess",
		StartedAtUTC:       "2026-01-01T00:00:00Z",
		CompletedAtUTC:     "2026-01-01T00:00:01Z",
		CaseCount:          10,
		Passed:             true,
		CanonicalSHA256:    digest,
		VerifySHA256:       digest,
		FailureClassSHA256: digest,
		ExitCodeSHA256:     digest,
	}
}

func nativeRun(nodeID string, replayIndex int, digest string) replay.NodeRunEvidence {
	run := mkRun(nodeID, "vm", "ubuntu", "ga", replayIndex, digest)
	run.DiscoveredCPU = testCPUArm
	run.DiscoveredKernel = testKernel610
	run.MeasuredArchitecture = "x86_64"
	run.MeasuredOSID = "ubuntu"
	run.MeasuredOSVersionID = "22.04"
	run.MeasuredKernel = testKernel610
	run.MeasuredCPU = testCPUArm
	run.AWSInstanceID = "i-0123456789abcdef0"
	run.AWSImageID = "ami-0abc1234"
	run.TransportAttestationSHA256 = strings.Repeat("9", 64)
	return run
}

func testReplayAggregateDigest(nodeReplays []replay.NodeRunEvidence, selectDigest func(replay.NodeRunEvidence) string) string {
	sorted := append([]replay.NodeRunEvidence(nil), nodeReplays...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].NodeID == sorted[j].NodeID {
			return sorted[i].ReplayIndex < sorted[j].ReplayIndex
		}
		return sorted[i].NodeID < sorted[j].NodeID
	})
	if len(sorted) >= 0 {
		var governed strings.Builder
		for _, run := range sorted {
			governed.WriteString(run.NodeID)
			governed.WriteByte('\x1f')
			governed.WriteString(fmt.Sprintf("%03d", run.ReplayIndex))
			governed.WriteByte('\x1f')
			governed.WriteString(selectDigest(run))
			governed.WriteByte('\n')
		}
		sum := sha256.Sum256([]byte(governed.String()))
		return hex.EncodeToString(sum[:])
	}
	var b strings.Builder
	for _, run := range sorted {
		b.WriteString(run.NodeID)
		b.WriteByte('\x1f')
		if run.ReplayIndex < 10 {
			b.WriteString("00")
		} else if run.ReplayIndex < 100 {
			b.WriteByte('0')
		}
		b.WriteString(strings.TrimSpace(strings.TrimLeft(strings.ReplaceAll(strings.Repeat("0", 0), " ", ""), "")))
		b.WriteString(strings.TrimLeft(strings.Repeat("0", 0), "0"))
		b.WriteString(strings.TrimSpace(strings.TrimPrefix(strings.Repeat("0", 0), "")))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteString(strings.TrimPrefix(strings.TrimSuffix("", ""), ""))
		b.WriteString(strings.TrimSpace(""))
		b.WriteByte('\x1f')
		b.WriteString(selectDigest(run))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
