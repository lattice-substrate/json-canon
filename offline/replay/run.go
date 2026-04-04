package replay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// NodeAdapter executes replay operations for one node lane.
type NodeAdapter interface {
	Prepare(ctx context.Context, node NodeSpec, bundlePath string, replayIndex int) error
	RunReplay(ctx context.Context, node NodeSpec, bundlePath string, evidencePath string, replayIndex int) error
	Cleanup(ctx context.Context, node NodeSpec, replayIndex int) error
}

// AdapterFactory selects the correct adapter for each node mode.
type AdapterFactory func(node NodeSpec) (NodeAdapter, error)

// RunOptions configures matrix orchestration.
type RunOptions struct {
	BundlePath            string
	BundleSHA256          string
	ControlBinarySHA256   string
	MatrixSHA256          string
	ProfileSHA256         string
	VectorSetSHA256       string
	GovernanceUmbrellaCommit string
	GovernanceLockSHA256 string
	SourceGitCommit       string
	SourceGitTag          string
	Orchestrator          string
	EvidenceSchemaVersion string
	GlobalEnv             map[string]string
	Now                   func() time.Time
	// Optional infra-manifest binding recorded in evidence.v1 when the profile requires it.
	InfraManifestSHA256 string
	InfraRepoURL        string
	InfraRepoCommit     string
	InfraManifest       *InfraManifest
	AttestationOutputRoot string
}

// RunMatrix orchestrates replay execution across required nodes and replays.
//
//nolint:gocyclo,cyclop,funlen,gocognit // REQ:OFFLINE-EVIDENCE-001 orchestration keeps checks explicit for reproducible replay diagnostics.
func RunMatrix(ctx context.Context, matrix *Matrix, profile *Profile, factory AdapterFactory, opts RunOptions) (*EvidenceBundle, error) {
	if matrix == nil || profile == nil {
		return nil, fmt.Errorf("matrix and profile are required")
	}
	if err := ValidateMatrix(matrix); err != nil {
		return nil, err
	}
	if err := ValidateProfile(profile); err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, fmt.Errorf("adapter factory is required")
	}
	now := opts.Now
	if now == nil {
		now = wallClockNow
	}
	if opts.Orchestrator == "" {
		opts.Orchestrator = "jcs-offline-replay"
	}
	sourceCommit := strings.TrimSpace(opts.SourceGitCommit)
	if sourceCommit == "" {
		// Deterministic placeholder for non-release runs that do not supply source identity.
		sourceCommit = "0000000000000000000000000000000000000000"
	}
	sourceTag := strings.TrimSpace(opts.SourceGitTag)
	if sourceTag == "" {
		sourceTag = "untagged"
	}
	governanceCommit := strings.TrimSpace(opts.GovernanceUmbrellaCommit)
	if governanceCommit == "" {
		governanceCommit = "0000000000000000000000000000000000000000"
	}
	governanceLockSHA := strings.TrimSpace(opts.GovernanceLockSHA256)
	if governanceLockSHA == "" {
		sum := sha256.Sum256(nil)
		governanceLockSHA = hex.EncodeToString(sum[:])
	}

	requiredNodes, err := requiredNodeIDs(matrix, profile)
	if err != nil {
		return nil, err
	}
	nodeIndex := make(map[string]NodeSpec, len(matrix.Nodes))
	for _, n := range matrix.Nodes {
		nodeIndex[n.ID] = n
	}

	schemaVersion := EvidenceSchemaVersion
	if requested := strings.TrimSpace(opts.EvidenceSchemaVersion); requested != "" && requested != EvidenceSchemaVersion {
		return nil, fmt.Errorf("unsupported evidence schema_version %q", requested)
	}
	if opts.GlobalEnv == nil {
		opts.GlobalEnv = make(map[string]string, 1)
	}
	opts.GlobalEnv["JCS_EVIDENCE_SCHEMA_VERSION"] = schemaVersion
	bundle := &EvidenceBundle{
		SchemaVersion:       schemaVersion,
		BundleSHA256:        opts.BundleSHA256,
		ControlBinarySHA:    opts.ControlBinarySHA256,
		MatrixSHA256:        opts.MatrixSHA256,
		ProfileSHA256:       opts.ProfileSHA256,
		VectorSetSHA256:     opts.VectorSetSHA256,
		GovernanceUmbrellaCommit: governanceCommit,
		GovernanceLockSHA256: governanceLockSHA,
		SourceGitCommit:     sourceCommit,
		SourceGitTag:        sourceTag,
		GeneratedAtUTC:      now().UTC().Format(time.RFC3339Nano),
		Orchestrator:        opts.Orchestrator,
		ProfileID:           profileIDForName(profile.Name),
		ProfileName:         profileNameForEvidence(profile.Name),
		Architecture:        matrix.Architecture,
		AggregateMethod:     ReplayAggregateMethod,
		RequiredSuites:      append([]string(nil), profile.RequiredSuites...),
		HardReleaseGate:     profile.HardReleaseGate,
		InfraManifestSHA256: strings.TrimSpace(opts.InfraManifestSHA256),
		InfraRepoURL:        strings.TrimSpace(opts.InfraRepoURL),
		InfraRepoCommit:     strings.TrimSpace(opts.InfraRepoCommit),
	}
	if bundle.InfraManifestSHA256 != "" || bundle.InfraRepoURL != "" || bundle.InfraRepoCommit != "" {
		bundle.IIDTrustRootSetID = "aws-iid-trust-roots.v1"
	}

	tmpRoot, err := os.MkdirTemp("", "jcs-offline-replay-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpRoot); removeErr != nil {
			_ = removeErr
		}
	}()

	for _, nodeID := range requiredNodes {
		node := nodeIndex[nodeID]
		if len(opts.GlobalEnv) != 0 {
			merged := make(map[string]string, len(node.Runner.Env)+len(opts.GlobalEnv))
			for k, v := range node.Runner.Env {
				merged[k] = v
			}
			for k, v := range opts.GlobalEnv {
				merged[k] = v
			}
			node.Runner.Env = merged
		}
		adapter, err := factory(node)
		if err != nil {
			return nil, fmt.Errorf("node %s adapter: %w", node.ID, err)
		}
		for replayIdx := 1; replayIdx <= requiredReplayCount(node, profile); replayIdx++ {
			if err := adapter.Prepare(ctx, node, opts.BundlePath, replayIdx); err != nil {
				return nil, fmt.Errorf("node %s replay %d prepare: %w", node.ID, replayIdx, err)
			}

			evidencePath := filepath.Join(tmpRoot, fmt.Sprintf("%s-replay-%03d.json", node.ID, replayIdx))
			runErr := adapter.RunReplay(ctx, node, opts.BundlePath, evidencePath, replayIdx)
			cleanupErr := adapter.Cleanup(ctx, node, replayIdx)
			if runErr != nil {
				return nil, fmt.Errorf("node %s replay %d run: %w", node.ID, replayIdx, runErr)
			}
			if cleanupErr != nil {
				return nil, fmt.Errorf("node %s replay %d cleanup: %w", node.ID, replayIdx, cleanupErr)
			}

			runEvidence, err := LoadNodeRunEvidence(evidencePath)
			if err != nil {
				return nil, fmt.Errorf("node %s replay %d load evidence: %w", node.ID, replayIdx, err)
			}
			if err := copyAttestationSidecar(evidencePath, opts.AttestationOutputRoot, node.ID, replayIdx); err != nil {
				return nil, fmt.Errorf("node %s replay %d persist attestation: %w", node.ID, replayIdx, err)
			}
			bundle.NodeReplays = append(bundle.NodeReplays, *runEvidence)
		}
	}
	if len(bundle.NodeReplays) == 0 {
		return nil, fmt.Errorf("matrix execution produced no replay evidence")
	}

	sort.Slice(bundle.NodeReplays, func(i, j int) bool {
		if bundle.NodeReplays[i].NodeID == bundle.NodeReplays[j].NodeID {
			return bundle.NodeReplays[i].ReplayIndex < bundle.NodeReplays[j].ReplayIndex
		}
		return bundle.NodeReplays[i].NodeID < bundle.NodeReplays[j].NodeID
	})

	bundle.AggregateCanonical = computeReplayAggregateDigest(bundle.NodeReplays, func(run NodeRunEvidence) string {
		return run.CanonicalSHA256
	})
	bundle.AggregateVerify = computeReplayAggregateDigest(bundle.NodeReplays, func(run NodeRunEvidence) string {
		return run.VerifySHA256
	})
	bundle.AggregateClass = computeReplayAggregateDigest(bundle.NodeReplays, func(run NodeRunEvidence) string {
		return run.FailureClassSHA256
	})
	bundle.AggregateExitCode = computeReplayAggregateDigest(bundle.NodeReplays, func(run NodeRunEvidence) string {
		return run.ExitCodeSHA256
	})

	if err := ValidateEvidenceBundle(bundle, matrix, profile, EvidenceValidationOptions{
		ExpectedBundleSHA256:        opts.BundleSHA256,
		ExpectedControlBinarySHA256: opts.ControlBinarySHA256,
		ExpectedMatrixSHA256:        opts.MatrixSHA256,
		ExpectedProfileSHA256:       opts.ProfileSHA256,
		ExpectedVectorSetSHA256:     opts.VectorSetSHA256,
		ExpectedGovernanceUmbrellaCommit: governanceCommit,
		ExpectedGovernanceLockSHA256: governanceLockSHA,
		ExpectedArchitecture:        matrix.Architecture,
		ExpectedSourceGitCommit:     sourceCommit,
		ExpectedSourceGitTag:        sourceTag,
		ExpectedInfraManifestSHA256: strings.TrimSpace(opts.InfraManifestSHA256),
		ExpectedInfraRepoURL:        strings.TrimSpace(opts.InfraRepoURL),
		ExpectedInfraRepoCommit:     strings.TrimSpace(opts.InfraRepoCommit),
		ExpectedInfraManifest:       opts.InfraManifest,
	}); err != nil {
		return nil, err
	}
	return bundle, nil
}

// LoadNodeRunEvidence loads one node replay evidence artifact from disk.
func LoadNodeRunEvidence(path string) (*NodeRunEvidence, error) {
	var run NodeRunEvidence
	if err := decodeStrictJSONFile(path, "node evidence", &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// LoadNodeRunEvidenceFromBytes decodes one node replay evidence artifact from
// in-memory JSON bytes.
func LoadNodeRunEvidenceFromBytes(data []byte) (*NodeRunEvidence, error) {
	var run NodeRunEvidence
	if err := decodeStrictJSONBytes("node evidence", data, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func copyAttestationSidecar(evidencePath, outputRoot, nodeID string, replayIndex int) error {
	if strings.TrimSpace(outputRoot) == "" {
		return nil
	}
	sourcePath := strings.TrimSuffix(evidencePath, filepath.Ext(evidencePath)) + ".transport-attestation.v1.json"
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	destPath := filepath.Join(outputRoot, nodeID, fmt.Sprintf("%03d", replayIndex), "transport-attestation.v1.json")
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0o600)
}

//nolint:forbidigo // REQ:OFFLINE-EVIDENCE-001 default runtime clock for evidence generation when no injected clock is provided.
func wallClockNow() time.Time {
	return time.Now()
}
