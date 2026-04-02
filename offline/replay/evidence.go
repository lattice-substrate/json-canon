package replay

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// EvidenceSchemaVersion is the stable schema identifier for evidence bundles (v1).
const EvidenceSchemaVersion = "evidence.v1"

// EvidenceSchemaVersionV2 is the schema identifier for evidence bundles with infra-manifest binding.
const EvidenceSchemaVersionV2 = "evidence.v2"

// supportedEvidenceSchemaVersions lists all schema versions accepted by ValidateEvidenceBundle.
var supportedEvidenceSchemaVersions = map[string]bool{
	EvidenceSchemaVersion:   true,
	EvidenceSchemaVersionV2: true,
}

// EvidenceBundle is the machine-consumed replay output artifact.
type EvidenceBundle struct {
	SchemaVersion    string   `json:"schema_version"`
	BundleSHA256     string   `json:"bundle_sha256"`
	ControlBinarySHA string   `json:"control_binary_sha256"`
	MatrixSHA256     string   `json:"matrix_sha256"`
	ProfileSHA256    string   `json:"profile_sha256"`
	SourceGitCommit  string   `json:"source_git_commit"`
	SourceGitTag     string   `json:"source_git_tag"`
	GeneratedAtUTC   string   `json:"generated_at_utc"`
	Orchestrator     string   `json:"orchestrator"`
	ProfileName      string   `json:"profile_name"`
	Architecture     string   `json:"architecture"`
	RequiredSuites   []string `json:"required_suites"`
	HardReleaseGate  bool     `json:"hard_release_gate"`
	// v2 fields: infra-manifest binding (omitempty for v1 compat)
	InfraManifestSHA256 string            `json:"infra_manifest_sha256,omitempty"`
	InfraRepoURL        string            `json:"infra_repo_url,omitempty"`
	InfraRepoCommit     string            `json:"infra_repo_commit,omitempty"`
	NodeReplays         []NodeRunEvidence `json:"node_replays"`
	AggregateCanonical  string            `json:"aggregate_canonical_sha256"`
	AggregateVerify     string            `json:"aggregate_verify_sha256"`
	AggregateClass      string            `json:"aggregate_failure_class_sha256"`
	AggregateExitCode   string            `json:"aggregate_exit_code_sha256"`
}

// NodeRunEvidence is one replay execution on one node.
type NodeRunEvidence struct {
	NodeID             string `json:"node_id"`
	Mode               string `json:"mode"`
	Distro             string `json:"distro"`
	KernelFamily       string `json:"kernel_family"`
	ReplayIndex        int    `json:"replay_index"`
	SessionID          string `json:"session_id"`
	StartedAtUTC       string `json:"started_at_utc"`
	CompletedAtUTC     string `json:"completed_at_utc"`
	CaseCount          int    `json:"case_count"`
	Passed             bool   `json:"passed"`
	CanonicalSHA256    string `json:"canonical_sha256"`
	VerifySHA256       string `json:"verify_sha256"`
	FailureClassSHA256 string `json:"failure_class_sha256"`
	ExitCodeSHA256     string `json:"exit_code_sha256"`
	// v2 fields: discovered substrate identity (omitempty)
	DiscoveredCPU    string `json:"discovered_cpu,omitempty"`
	DiscoveredKernel string `json:"discovered_kernel,omitempty"`
	ImageDigest      string `json:"image_digest,omitempty"`
}

// EvidenceValidationOptions binds evidence metadata to expected immutable inputs.
type EvidenceValidationOptions struct {
	ExpectedBundleSHA256        string
	ExpectedControlBinarySHA256 string
	ExpectedMatrixSHA256        string
	ExpectedProfileSHA256       string
	ExpectedArchitecture        string
	ExpectedSourceGitCommit     string
	ExpectedSourceGitTag        string
	ExpectedInfraManifestSHA256 string
	ExpectedInfraRepoURL        string
	ExpectedInfraRepoCommit     string
}

// WriteEvidence writes a canonical JSON evidence bundle to disk.
func WriteEvidence(path string, e *EvidenceBundle) error {
	if e == nil {
		return fmt.Errorf("evidence bundle is nil")
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write evidence file: %w", err)
	}
	return nil
}

// LoadEvidence loads an evidence bundle from disk.
func LoadEvidence(path string) (*EvidenceBundle, error) {
	var e EvidenceBundle
	if err := decodeStrictJSONFile(path, "evidence", &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// ValidateEvidenceBundle validates replay evidence against matrix/profile policy expectations.
//
//nolint:gocyclo,cyclop,funlen,maintidx,gocognit // REQ:OFFLINE-EVIDENCE-001 evidence gate checks encode many policy invariants with explicit failure attribution.
func ValidateEvidenceBundle(e *EvidenceBundle, m *Matrix, p *Profile, opts EvidenceValidationOptions) error {
	if e == nil {
		return fmt.Errorf("evidence bundle is nil")
	}
	if m == nil || p == nil {
		return fmt.Errorf("matrix and profile are required")
	}
	requiresInfraBinding := profileRequiresInfraBinding(p)
	if !supportedEvidenceSchemaVersions[e.SchemaVersion] {
		return fmt.Errorf("unsupported schema_version %q", e.SchemaVersion)
	}
	isV2 := e.SchemaVersion == EvidenceSchemaVersionV2
	// Reject mixed state: v2-only fields must not appear in a v1 schema bundle.
	if !isV2 && (e.InfraManifestSHA256 != "" || e.InfraRepoURL != "" || e.InfraRepoCommit != "") {
		return fmt.Errorf("evidence schema_version is %q but contains v2 infra fields; use schema_version %q", e.SchemaVersion, EvidenceSchemaVersionV2)
	}
	if e.ProfileName != p.Name {
		return fmt.Errorf("profile mismatch: evidence=%q profile=%q", e.ProfileName, p.Name)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "bundle_sha256", value: e.BundleSHA256},
		{name: "control_binary_sha256", value: e.ControlBinarySHA},
		{name: "matrix_sha256", value: e.MatrixSHA256},
		{name: "profile_sha256", value: e.ProfileSHA256},
		{name: "aggregate_canonical_sha256", value: e.AggregateCanonical},
		{name: "aggregate_verify_sha256", value: e.AggregateVerify},
		{name: "aggregate_failure_class_sha256", value: e.AggregateClass},
		{name: "aggregate_exit_code_sha256", value: e.AggregateExitCode},
	} {
		if err := validateSHA256Token(field.name, field.value); err != nil {
			return err
		}
	}
	if err := validateGitCommitToken("source_git_commit", e.SourceGitCommit); err != nil {
		return err
	}
	if err := validateGitTagToken("source_git_tag", e.SourceGitTag); err != nil {
		return err
	}
	expectedArch := m.Architecture
	if strings.TrimSpace(opts.ExpectedArchitecture) != "" {
		expectedArch = strings.TrimSpace(opts.ExpectedArchitecture)
	}
	if e.Architecture != expectedArch {
		return fmt.Errorf("architecture mismatch: evidence=%q expected=%q", e.Architecture, expectedArch)
	}
	if opts.ExpectedBundleSHA256 != "" && e.BundleSHA256 != opts.ExpectedBundleSHA256 {
		return fmt.Errorf("bundle_sha256 mismatch: evidence=%q expected=%q", e.BundleSHA256, opts.ExpectedBundleSHA256)
	}
	if opts.ExpectedControlBinarySHA256 != "" && e.ControlBinarySHA != opts.ExpectedControlBinarySHA256 {
		return fmt.Errorf("control_binary_sha256 mismatch: evidence=%q expected=%q", e.ControlBinarySHA, opts.ExpectedControlBinarySHA256)
	}
	if opts.ExpectedMatrixSHA256 != "" && e.MatrixSHA256 != opts.ExpectedMatrixSHA256 {
		return fmt.Errorf("matrix_sha256 mismatch: evidence=%q expected=%q", e.MatrixSHA256, opts.ExpectedMatrixSHA256)
	}
	if opts.ExpectedProfileSHA256 != "" && e.ProfileSHA256 != opts.ExpectedProfileSHA256 {
		return fmt.Errorf("profile_sha256 mismatch: evidence=%q expected=%q", e.ProfileSHA256, opts.ExpectedProfileSHA256)
	}
	if expectedCommit := strings.TrimSpace(opts.ExpectedSourceGitCommit); expectedCommit != "" &&
		e.SourceGitCommit != expectedCommit {
		return fmt.Errorf("source_git_commit mismatch: evidence=%q expected=%q", e.SourceGitCommit, expectedCommit)
	}
	if expectedTag := strings.TrimSpace(opts.ExpectedSourceGitTag); expectedTag != "" &&
		e.SourceGitTag != expectedTag {
		return fmt.Errorf("source_git_tag mismatch: evidence=%q expected=%q", e.SourceGitTag, expectedTag)
	}
	if isV2 {
		if err := validateEvidenceV2InfraFields(e, opts); err != nil {
			return err
		}
	}
	if !e.HardReleaseGate {
		return fmt.Errorf("evidence must record hard_release_gate=true")
	}
	if len(e.NodeReplays) == 0 {
		return fmt.Errorf("evidence must include node_replays")
	}

	requiredNodes, err := requiredNodeIDs(m, p)
	if err != nil {
		return err
	}
	matrixByID := make(map[string]NodeSpec, len(m.Nodes))
	for _, node := range m.Nodes {
		matrixByID[node.ID] = node
	}

	byNode := make(map[string][]NodeRunEvidence)
	for _, r := range e.NodeReplays {
		if r.NodeID == "" {
			return fmt.Errorf("node replay has empty node_id")
		}
		node, ok := matrixByID[r.NodeID]
		if !ok {
			return fmt.Errorf("node replay references unknown node_id %q", r.NodeID)
		}
		if r.Mode != string(node.Mode) {
			return fmt.Errorf("node %s mode mismatch: got=%q want=%q", r.NodeID, r.Mode, node.Mode)
		}
		if r.Distro != node.Distro {
			return fmt.Errorf("node %s distro mismatch: got=%q want=%q", r.NodeID, r.Distro, node.Distro)
		}
		if r.KernelFamily != node.KernelFamily {
			return fmt.Errorf("node %s kernel_family mismatch: got=%q want=%q", r.NodeID, r.KernelFamily, node.KernelFamily)
		}
		if r.ReplayIndex < 1 {
			return fmt.Errorf("node %s replay_index must be >=1", r.NodeID)
		}
		if r.CaseCount < 1 {
			return fmt.Errorf("node %s replay %d must have case_count >=1", r.NodeID, r.ReplayIndex)
		}
		if !r.Passed {
			return fmt.Errorf("node %s replay %d is marked failed", r.NodeID, r.ReplayIndex)
		}
		for _, token := range []struct {
			name  string
			value string
		}{
			{"session_id", r.SessionID},
			{"started_at_utc", r.StartedAtUTC},
			{"completed_at_utc", r.CompletedAtUTC},
		} {
			if strings.TrimSpace(token.value) == "" {
				return fmt.Errorf("node %s replay %d missing %s", r.NodeID, r.ReplayIndex, token.name)
			}
		}
		for _, token := range []struct {
			name  string
			value string
		}{
			{"canonical_sha256", r.CanonicalSHA256},
			{"verify_sha256", r.VerifySHA256},
			{"failure_class_sha256", r.FailureClassSHA256},
			{"exit_code_sha256", r.ExitCodeSHA256},
		} {
			if err := validateSHA256Token(token.name, token.value); err != nil {
				return err
			}
		}
		if err := validateNodeRunEvidenceVersionFields(r, node, isV2, requiresInfraBinding); err != nil {
			return err
		}
		byNode[r.NodeID] = append(byNode[r.NodeID], r)
	}

	var baseline *NodeRunEvidence
	for _, id := range requiredNodes {
		runs := byNode[id]
		wantReplays := requiredReplayCount(matrixByID[id], p)
		if len(runs) < wantReplays {
			return fmt.Errorf("node %s has %d replays, want at least %d", id, len(runs), wantReplays)
		}
		seenReplay := make(map[int]struct{}, len(runs))
		for _, run := range runs {
			seenReplay[run.ReplayIndex] = struct{}{}
			if baseline == nil {
				r := run
				baseline = &r
				continue
			}
			if run.CanonicalSHA256 != baseline.CanonicalSHA256 {
				return fmt.Errorf("canonical digest drift at node %s replay %d", run.NodeID, run.ReplayIndex)
			}
			if run.VerifySHA256 != baseline.VerifySHA256 {
				return fmt.Errorf("verify digest drift at node %s replay %d", run.NodeID, run.ReplayIndex)
			}
			if run.FailureClassSHA256 != baseline.FailureClassSHA256 {
				return fmt.Errorf("failure-class digest drift at node %s replay %d", run.NodeID, run.ReplayIndex)
			}
			if run.ExitCodeSHA256 != baseline.ExitCodeSHA256 {
				return fmt.Errorf("exit-code digest drift at node %s replay %d", run.NodeID, run.ReplayIndex)
			}
		}
		for i := 1; i <= wantReplays; i++ {
			if _, ok := seenReplay[i]; !ok {
				return fmt.Errorf("node %s missing replay index %d", id, i)
			}
		}
	}

	if baseline == nil {
		return fmt.Errorf("no baseline replay digest found")
	}
	if e.AggregateCanonical != baseline.CanonicalSHA256 {
		return fmt.Errorf("aggregate canonical digest mismatch")
	}
	if e.AggregateVerify != baseline.VerifySHA256 {
		return fmt.Errorf("aggregate verify digest mismatch")
	}
	if e.AggregateClass != baseline.FailureClassSHA256 {
		return fmt.Errorf("aggregate failure-class digest mismatch")
	}
	if e.AggregateExitCode != baseline.ExitCodeSHA256 {
		return fmt.Errorf("aggregate exit-code digest mismatch")
	}

	suites := append([]string(nil), e.RequiredSuites...)
	sort.Strings(suites)
	wantSuites := append([]string(nil), p.RequiredSuites...)
	sort.Strings(wantSuites)
	if strings.Join(suites, ",") != strings.Join(wantSuites, ",") {
		return fmt.Errorf("required_suites mismatch")
	}

	if requiresInfraBinding && !isV2 {
		return fmt.Errorf("profile requires infra-substrate-binding but evidence schema is %q, want %q", e.SchemaVersion, EvidenceSchemaVersionV2)
	}

	return nil
}

// validateEvidenceV2InfraFields checks the three required infra-manifest binding fields
// in evidence.v2 bundles. Extracted to keep ValidateEvidenceBundle complexity in bounds.
func validateEvidenceV2InfraFields(e *EvidenceBundle, opts EvidenceValidationOptions) error {
	if err := validateSHA256Token("infra_manifest_sha256", e.InfraManifestSHA256); err != nil {
		return err
	}
	if strings.TrimSpace(e.InfraRepoURL) == "" {
		return fmt.Errorf("evidence.v2 requires infra_repo_url")
	}
	if err := validateGitCommitToken("infra_repo_commit", e.InfraRepoCommit); err != nil {
		return err
	}
	if expected := strings.TrimSpace(opts.ExpectedInfraManifestSHA256); expected != "" &&
		e.InfraManifestSHA256 != expected {
		return fmt.Errorf("infra_manifest_sha256 mismatch: evidence=%q expected=%q", e.InfraManifestSHA256, expected)
	}
	if expected := strings.TrimSpace(opts.ExpectedInfraRepoURL); expected != "" &&
		e.InfraRepoURL != expected {
		return fmt.Errorf("infra_repo_url mismatch: evidence=%q expected=%q", e.InfraRepoURL, expected)
	}
	if expected := strings.TrimSpace(opts.ExpectedInfraRepoCommit); expected != "" &&
		e.InfraRepoCommit != expected {
		return fmt.Errorf("infra_repo_commit mismatch: evidence=%q expected=%q", e.InfraRepoCommit, expected)
	}
	return nil
}

func validateNodeRunEvidenceVersionFields(r NodeRunEvidence, node NodeSpec, isV2 bool, requiresInfraBinding bool) error {
	hasV2Fields := nodeRunEvidenceHasV2Fields(r)
	if !isV2 && hasV2Fields {
		return fmt.Errorf("node %s replay %d contains v2-only node fields in schema %q", r.NodeID, r.ReplayIndex, EvidenceSchemaVersion)
	}
	if node.Mode != NodeModeContainer && strings.TrimSpace(r.ImageDigest) != "" {
		return fmt.Errorf("node %s replay %d image_digest is only allowed for container lanes", r.NodeID, r.ReplayIndex)
	}
	if !requiresInfraBinding {
		return nil
	}
	return validateInfraBindingNodeFields(r, node)
}

func nodeRunEvidenceHasV2Fields(r NodeRunEvidence) bool {
	return strings.TrimSpace(r.DiscoveredCPU) != "" ||
		strings.TrimSpace(r.DiscoveredKernel) != "" ||
		strings.TrimSpace(r.ImageDigest) != ""
}

func validateInfraBindingNodeFields(r NodeRunEvidence, node NodeSpec) error {
	if strings.TrimSpace(r.DiscoveredCPU) == "" {
		return fmt.Errorf("node %s replay %d missing discovered_cpu for infra-substrate-binding profile", r.NodeID, r.ReplayIndex)
	}
	if strings.TrimSpace(r.DiscoveredKernel) == "" {
		return fmt.Errorf("node %s replay %d missing discovered_kernel for infra-substrate-binding profile", r.NodeID, r.ReplayIndex)
	}
	if node.Mode == NodeModeContainer && strings.TrimSpace(r.ImageDigest) == "" {
		return fmt.Errorf("node %s replay %d missing image_digest for container lane in infra-substrate-binding profile", r.NodeID, r.ReplayIndex)
	}
	return nil
}

func profileRequiresInfraBinding(profile *Profile) bool {
	if profile == nil {
		return false
	}
	for _, s := range profile.RequiredSuites {
		if s == "infra-substrate-binding" {
			return true
		}
	}
	return false
}

//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 strict JSON decoding reads explicit operator/runtime artifact paths.
func decodeStrictJSONFile(path string, kind string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", kind, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", kind, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode %s: unexpected trailing json content", kind)
		}
		return fmt.Errorf("decode %s: decode trailing json token: %w", kind, err)
	}
	return nil
}

func validateSHA256Token(name, value string) error {
	token := strings.TrimSpace(value)
	if len(token) != 64 {
		return fmt.Errorf("%s must be 64 hex characters", name)
	}
	if _, err := hex.DecodeString(token); err != nil {
		return fmt.Errorf("%s must be valid hex: %w", name, err)
	}
	return nil
}

func validateGitCommitToken(name, value string) error {
	token := strings.TrimSpace(value)
	if len(token) != 40 {
		return fmt.Errorf("%s must be 40 hex characters", name)
	}
	if _, err := hex.DecodeString(token); err != nil {
		return fmt.Errorf("%s must be valid hex: %w", name, err)
	}
	return nil
}

func validateGitTagToken(name, value string) error {
	token := strings.TrimSpace(value)
	if token == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if strings.ContainsAny(token, " \t\r\n") {
		return fmt.Errorf("%s must not contain whitespace", name)
	}
	return nil
}
