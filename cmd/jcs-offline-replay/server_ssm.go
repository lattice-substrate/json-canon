package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

type serverSSMAdapter struct {
	aws       serverAWSClients
	bucket    string
	artifacts stagedServerArtifacts
	hosts     map[string]provisionedHost
}

func (r *serverEvidenceRuntime) prepareStaging(stdout io.Writer) error {
	if err := r.setRunRecordStatus(&r.runRecord.StagingStatus, serverRunStatusRunning); err != nil {
		return err
	}
	if err := writeLine(stdout, "==> creating private staging bucket and uploading replay artifacts"); err != nil {
		return err
	}
	bucket, err := createStagingBucketFunc(r.ctx, r.awsClients, r.opts.tag)
	if err != nil {
		markRunRecordStatusBestEffort(r, &r.runRecord.StagingStatus)
		return err
	}
	x86 := stagedServerArtifacts{
		bundleKey: "x86_64/offline-bundle.tgz",
		workerKey: "x86_64/jcs-offline-worker",
	}
	arm := stagedServerArtifacts{
		bundleKey: "arm64/offline-bundle.tgz",
		workerKey: "arm64/jcs-offline-worker",
	}
	for _, item := range []struct {
		key  string
		path string
	}{
		{x86.bundleKey, r.x86Artifacts.bundlePath},
		{x86.workerKey, r.x86Artifacts.workerPath},
		{arm.bundleKey, r.armArtifacts.bundlePath},
		{arm.workerKey, r.armArtifacts.workerPath},
	} {
		if err := uploadStagingFileFunc(r.ctx, r.awsClients, bucket, item.key, item.path); err != nil {
			markRunRecordStatusBestEffort(r, &r.runRecord.StagingStatus)
			return err
		}
	}
	r.staging = serverStaging{
		bucket: bucket,
		x86:    x86,
		arm:    arm,
	}
	r.runRecord.StagingBucket = bucket
	if err := r.persistRunRecord(); err != nil {
		return err
	}
	return r.setRunRecordStatus(&r.runRecord.StagingStatus, serverRunStatusSucceeded)
}

func (r *serverEvidenceRuntime) discoverHostFacts(host provisionedHost) (discoveredRemoteFacts, error) {
	artifacts := r.stagingArtifactsForArch(host.Architecture)
	workerURL, err := presignGetObjectURLFunc(r.ctx, r.awsClients, r.staging.bucket, artifacts.workerKey)
	if err != nil {
		return discoveredRemoteFacts{}, err
	}
	script := strings.Join([]string{
		"set -euo pipefail",
		`tmp="$(mktemp -d)"`,
		`trap 'rm -rf "$tmp"' EXIT`,
		"curl -fsSL " + shellQuote(workerURL) + ` -o "$tmp/jcs-offline-worker"`,
		`chmod +x "$tmp/jcs-offline-worker"`,
		`LC_ALL=C LANG=C TZ=UTC "$tmp/jcs-offline-worker" inspect-host`,
	}, "\n")
	out, err := runSSMShellScriptFunc(r.ctx, r.awsClients, host.InstanceID, "jcs inspect host "+host.HostID, script, 5*time.Minute)
	if err != nil {
		return discoveredRemoteFacts{}, err
	}
	var facts discoveredRemoteFacts
	if err := json.Unmarshal([]byte(out), &facts); err != nil {
		return discoveredRemoteFacts{}, fmt.Errorf("decode host inspection for %s: %w", host.HostID, err)
	}
	if facts.InstanceID != host.InstanceID {
		return discoveredRemoteFacts{}, fmt.Errorf("instance id mismatch for %s: discovered=%s provisioned=%s", host.HostID, facts.InstanceID, host.InstanceID)
	}
	if facts.ImageID != host.ImageID {
		return discoveredRemoteFacts{}, fmt.Errorf("image id mismatch for %s: discovered=%s provisioned=%s", host.HostID, facts.ImageID, host.ImageID)
	}
	if _, err := verifyAWSInstanceIdentityFunc(facts.IIDDocument, facts.IIDSignature, host, r.opts.awsRegion); err != nil {
		return discoveredRemoteFacts{}, err
	}
	facts.IIDVerified = true
	return facts, nil
}

func (r *serverEvidenceRuntime) stagingArtifactsForArch(arch string) stagedServerArtifacts {
	if arch == matrixArchitectureARM64 {
		return r.staging.arm
	}
	return r.staging.x86
}

func newServerSSMAdapterFactory(awsClients serverAWSClients, bucket string, artifacts stagedServerArtifacts, hosts map[string]provisionedHost) replay.AdapterFactory {
	return func(node replay.NodeSpec) (replay.NodeAdapter, error) {
		if node.Mode != replay.NodeModeVM {
			return nil, fmt.Errorf("node %s unsupported server mode %q", node.ID, node.Mode)
		}
		return &serverSSMAdapter{
			aws:       awsClients,
			bucket:    bucket,
			artifacts: artifacts,
			hosts:     hosts,
		}, nil
	}
}

func (a *serverSSMAdapter) Prepare(_ context.Context, _ replay.NodeSpec, _ string, _ int) error {
	return nil
}

func (a *serverSSMAdapter) Cleanup(_ context.Context, _ replay.NodeSpec, _ int) error {
	return nil
}

//nolint:gocyclo,cyclop // REQ:AWS-GATE-001 remote replay flow keeps each transport step explicit for auditability.
func (a *serverSSMAdapter) RunReplay(ctx context.Context, node replay.NodeSpec, _ string, evidencePath string, replayIndex int) error {
	host, ok := a.hosts[node.ID]
	if !ok {
		return fmt.Errorf("node %s missing provisioned host binding", node.ID)
	}
	bundleURL, err := presignGetObjectURLFunc(ctx, a.aws, a.bucket, a.artifacts.bundleKey)
	if err != nil {
		return err
	}
	workerURL, err := presignGetObjectURLFunc(ctx, a.aws, a.bucket, a.artifacts.workerKey)
	if err != nil {
		return err
	}
	evidenceKey := fmt.Sprintf("evidence/%s/%03d/offline-evidence.json", node.ID, replayIndex)
	attestationKey := fmt.Sprintf("evidence/%s/%03d/transport-attestation.v1.json", node.ID, replayIndex)
	evidencePutURL, err := presignPutObjectURLFunc(ctx, a.aws, a.bucket, evidenceKey)
	if err != nil {
		return err
	}
	attestationPutURL, err := presignPutObjectURLFunc(ctx, a.aws, a.bucket, attestationKey)
	if err != nil {
		return err
	}
	challenge, err := newTransportAttestationChallenge()
	if err != nil {
		return err
	}
	runCmd, err := buildRemoteReplaySSMCommand(node, replayIndex, challenge, bundleURL, workerURL, evidencePutURL, attestationPutURL)
	if err != nil {
		return err
	}
	if _, runErr := runSSMShellScriptFunc(ctx, a.aws, host.InstanceID, "jcs replay "+node.ID, runCmd, 30*time.Minute); runErr != nil {
		return runErr
	}
	data, err := downloadStagingObjectFunc(ctx, a.aws, a.bucket, evidenceKey)
	if err != nil {
		return err
	}
	attestationData, err := downloadStagingObjectFunc(ctx, a.aws, a.bucket, attestationKey)
	if err != nil {
		return err
	}
	if verifyErr := verifyTransportAttestation(attestationData, data, challenge, node.ID, replayIndex, host, a.aws.config.Region); verifyErr != nil {
		return verifyErr
	}
	runEvidence, err := replay.LoadNodeRunEvidenceFromBytes(data)
	if err != nil {
		return fmt.Errorf("load verified node evidence: %w", err)
	}
	runEvidence.TransportAttestationSHA256 = sha256HexString(string(attestationData))
	encodedEvidence, err := json.MarshalIndent(runEvidence, "", "  ")
	if err != nil {
		return fmt.Errorf("encode verified node evidence: %w", err)
	}
	encodedEvidence = append(encodedEvidence, '\n')
	attestationPath := strings.TrimSuffix(evidencePath, filepath.Ext(evidencePath)) + ".transport-attestation.v1.json"
	if err := atomicWriteFile(attestationPath, attestationData); err != nil {
		return err
	}
	if err := atomicWriteFile(evidencePath, encodedEvidence); err != nil {
		return err
	}
	return nil
}

func buildRemoteReplaySSMCommand(node replay.NodeSpec, replayIndex int, challenge, bundleURL, workerURL, evidencePutURL, attestationPutURL string) (string, error) {
	if node.Mode != replay.NodeModeVM {
		return "", fmt.Errorf("node %s remote aws release mode must be vm", node.ID)
	}
	workerArgs := []string{
		"--bundle", `"$tmp/bundle.tgz"`,
		"--evidence", `"$tmp/evidence.json"`,
		"--challenge", shellQuote(challenge),
		"--attestation-out", `"$tmp/transport-attestation.json"`,
		"--node-id", shellQuote(node.ID),
		"--mode", shellQuote(string(node.Mode)),
		"--distro", shellQuote(node.Distro),
		"--kernel-family", shellQuote(node.KernelFamily),
		"--replay-index", shellQuote(fmt.Sprintf("%d", replayIndex)),
		"--schema-version", shellQuote(replay.EvidenceSchemaVersion),
		"--infra-binding-evidence", "true",
		"--native-host-evidence", "true",
	}
	return strings.Join([]string{
		"set -euo pipefail",
		`tmp="$(mktemp -d)"`,
		`trap 'rm -rf "$tmp"' EXIT`,
		"curl -fsSL " + shellQuote(bundleURL) + ` -o "$tmp/bundle.tgz"`,
		"curl -fsSL " + shellQuote(workerURL) + ` -o "$tmp/jcs-offline-worker"`,
		`chmod +x "$tmp/jcs-offline-worker"`,
		`LC_ALL=C LANG=C TZ=UTC "$tmp/jcs-offline-worker" ` + strings.Join(workerArgs, " "),
		`curl -fsS -X PUT -T "$tmp/evidence.json" ` + shellQuote(evidencePutURL),
		`curl -fsS -X PUT -T "$tmp/transport-attestation.json" ` + shellQuote(attestationPutURL),
		`printf 'uploaded_evidence=1\n'`,
	}, "\n"), nil
}
