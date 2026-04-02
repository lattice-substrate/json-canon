package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	if err := writeLine(stdout, "==> creating private staging bucket and uploading replay artifacts"); err != nil {
		return err
	}
	bucket, err := createStagingBucket(r.ctx, r.awsClients, r.opts.tag)
	if err != nil {
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
		if err := uploadStagingFile(r.ctx, r.awsClients, bucket, item.key, item.path); err != nil {
			return err
		}
	}
	r.staging = serverStaging{
		bucket: bucket,
		x86:    x86,
		arm:    arm,
	}
	return nil
}

func (r *serverEvidenceRuntime) discoverHostFacts(host provisionedHost) (discoveredRemoteFacts, error) {
	artifacts := r.stagingArtifactsForArch(host.Architecture)
	workerURL, err := presignGetObjectURL(r.ctx, r.awsClients, r.staging.bucket, artifacts.workerKey)
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
	out, err := runSSMShellScript(r.ctx, r.awsClients, host.InstanceID, "jcs inspect host "+host.HostID, script, 5*time.Minute)
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

func (a *serverSSMAdapter) RunReplay(ctx context.Context, node replay.NodeSpec, _ string, evidencePath string, replayIndex int) error {
	host, ok := a.hosts[node.ID]
	if !ok {
		return fmt.Errorf("node %s missing provisioned host binding", node.ID)
	}
	bundleURL, err := presignGetObjectURL(ctx, a.aws, a.bucket, a.artifacts.bundleKey)
	if err != nil {
		return err
	}
	workerURL, err := presignGetObjectURL(ctx, a.aws, a.bucket, a.artifacts.workerKey)
	if err != nil {
		return err
	}
	evidenceKey := fmt.Sprintf("evidence/%s/%03d/offline-evidence.json", node.ID, replayIndex)
	evidencePutURL, err := presignPutObjectURL(ctx, a.aws, a.bucket, evidenceKey)
	if err != nil {
		return err
	}
	runCmd, err := buildRemoteReplaySSMCommand(node, replayIndex, bundleURL, workerURL, evidencePutURL)
	if err != nil {
		return err
	}
	stdout, err := runSSMShellScript(ctx, a.aws, host.InstanceID, "jcs replay "+node.ID, runCmd, 30*time.Minute)
	if err != nil {
		return err
	}
	remoteSHA := parseEvidenceSHA256(stdout)
	if remoteSHA == "" {
		return fmt.Errorf("ssm replay for %s did not emit evidence sha256", node.ID)
	}
	data, err := downloadStagingObject(ctx, a.aws, a.bucket, evidenceKey)
	if err != nil {
		return err
	}
	if sha256HexString(string(data)) != remoteSHA {
		return fmt.Errorf("downloaded evidence sha256 mismatch for %s replay %d", node.ID, replayIndex)
	}
	if err := atomicWriteFile(evidencePath, data, filePerm); err != nil {
		return err
	}
	return nil
}

func buildRemoteReplaySSMCommand(node replay.NodeSpec, replayIndex int, bundleURL, workerURL, evidencePutURL string) (string, error) {
	if node.Mode != replay.NodeModeVM {
		return "", fmt.Errorf("node %s remote aws release mode must be vm", node.ID)
	}
	workerArgs := []string{
		"--bundle", `"$tmp/bundle.tgz"`,
		"--evidence", `"$tmp/evidence.json"`,
		"--node-id", shellQuote(node.ID),
		"--mode", shellQuote(string(node.Mode)),
		"--distro", shellQuote(node.Distro),
		"--kernel-family", shellQuote(node.KernelFamily),
		"--replay-index", shellQuote(fmt.Sprintf("%d", replayIndex)),
		"--schema-version", shellQuote(replay.EvidenceSchemaVersionV3),
	}
	return strings.Join([]string{
		"set -euo pipefail",
		`tmp="$(mktemp -d)"`,
		`trap 'rm -rf "$tmp"' EXIT`,
		"curl -fsSL " + shellQuote(bundleURL) + ` -o "$tmp/bundle.tgz"`,
		"curl -fsSL " + shellQuote(workerURL) + ` -o "$tmp/jcs-offline-worker"`,
		`chmod +x "$tmp/jcs-offline-worker"`,
		`LC_ALL=C LANG=C TZ=UTC "$tmp/jcs-offline-worker" ` + strings.Join(workerArgs, " "),
		`sha="$(sha256sum "$tmp/evidence.json" | awk '{print $1}')"`,
		`curl -fsS -X PUT -T "$tmp/evidence.json" ` + shellQuote(evidencePutURL),
		`printf 'evidence_sha256=%s\n' "$sha"`,
	}, "\n"), nil
}

func parseEvidenceSHA256(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "evidence_sha256=") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "evidence_sha256="))
	}
	return ""
}
