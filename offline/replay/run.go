package replay

import (
	"context"
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
	BundlePath          string
	BundleSHA256        string
	ControlBinarySHA256 string
	MatrixSHA256        string
	ProfileSHA256       string
	SourceGitCommit     string
	SourceGitTag        string
	Orchestrator        string
	GlobalEnv           map[string]string
	Now                 func() time.Time
	// v2 fields: when InfraManifestSHA256 is non-empty, evidence.v2 is emitted.
	InfraManifestSHA256 string
	InfraRepoURL        string
	InfraRepoCommit     string
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

	requiredNodes, err := requiredNodeIDs(matrix, profile)
	if err != nil {
		return nil, err
	}
	nodeIndex := make(map[string]NodeSpec, len(matrix.Nodes))
	for _, n := range matrix.Nodes {
		nodeIndex[n.ID] = n
	}

	schemaVersion := EvidenceSchemaVersion
	if strings.TrimSpace(opts.InfraManifestSHA256) != "" {
		schemaVersion = EvidenceSchemaVersionV2
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
		SourceGitCommit:     sourceCommit,
		SourceGitTag:        sourceTag,
		GeneratedAtUTC:      now().UTC().Format(time.RFC3339Nano),
		Orchestrator:        opts.Orchestrator,
		ProfileName:         profile.Name,
		Architecture:        matrix.Architecture,
		RequiredSuites:      append([]string(nil), profile.RequiredSuites...),
		HardReleaseGate:     profile.HardReleaseGate,
		InfraManifestSHA256: strings.TrimSpace(opts.InfraManifestSHA256),
		InfraRepoURL:        strings.TrimSpace(opts.InfraRepoURL),
		InfraRepoCommit:     strings.TrimSpace(opts.InfraRepoCommit),
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

	base := bundle.NodeReplays[0]
	bundle.AggregateCanonical = base.CanonicalSHA256
	bundle.AggregateVerify = base.VerifySHA256
	bundle.AggregateClass = base.FailureClassSHA256
	bundle.AggregateExitCode = base.ExitCodeSHA256

	if err := ValidateEvidenceBundle(bundle, matrix, profile, EvidenceValidationOptions{
		ExpectedBundleSHA256:        opts.BundleSHA256,
		ExpectedControlBinarySHA256: opts.ControlBinarySHA256,
		ExpectedMatrixSHA256:        opts.MatrixSHA256,
		ExpectedProfileSHA256:       opts.ProfileSHA256,
		ExpectedArchitecture:        matrix.Architecture,
		ExpectedSourceGitCommit:     sourceCommit,
		ExpectedSourceGitTag:        sourceTag,
		ExpectedInfraManifestSHA256: strings.TrimSpace(opts.InfraManifestSHA256),
		ExpectedInfraRepoURL:        strings.TrimSpace(opts.InfraRepoURL),
		ExpectedInfraRepoCommit:     strings.TrimSpace(opts.InfraRepoCommit),
	}); err != nil {
		return nil, err
	}
	return bundle, nil
}

// LoadNodeRunEvidence loads one node replay evidence artifact from disk.
//
//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 node evidence path is explicit operator/runtime input.
func LoadNodeRunEvidence(path string) (*NodeRunEvidence, error) {
	var run NodeRunEvidence
	if err := decodeStrictJSONFile(path, "node evidence", &run); err != nil {
		return nil, err
	}
	return &run, nil
}

//nolint:forbidigo // REQ:OFFLINE-EVIDENCE-001 default runtime clock for evidence generation when no injected clock is provided.
func wallClockNow() time.Time {
	return time.Now()
}
