package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// EvidenceSchemaVersion is the stable schema identifier for evidence bundles.
const EvidenceSchemaVersion = "evidence.v1"

// ReplayAggregateMethod identifies the governed replay aggregate construction used
// by evidence.v1 aggregate_* fields.
const ReplayAggregateMethod = "replay-aggregate.v1"

// Official ES6 number-corpus proof constants are the governed full-release checksum
// targets already exercised by json-canon's official conformance tests.
const (
	OfficialES6NumberCorpusSuite = "official-es6-number-corpus"
	OfficialES6CorpusFullLines   = 100_000_000
	OfficialES6CorpusFullSHA256  = "0f7dda6b0837dde083c5d6b896f7d62340c8a2415b0c7121d83145e08a755272"
)

// Profile name constants used for evidence profile mapping.
const (
	profileNameBaseConformance              = "base-conformance"
	profileNameOfflineMeasuredEvidence      = "offline-measured-evidence"
	profileNameOfficialCloudMeasuredRelease = "official-cloud-measured-release"
)

// IIDTrustRootSetIDDefault is the governed IID trust-root set identifier for
// AWS instance identity document validation.
const IIDTrustRootSetIDDefault = "aws-iid-trust-roots.v1"

var errNilManifestIndex = errors.New("infra manifest not provided")

// EvidenceBundle is the machine-consumed replay output artifact.
type EvidenceBundle struct {
	SchemaVersion            string   `json:"schema_version"`
	BundleSHA256             string   `json:"bundle_sha256"`
	ControlBinarySHA         string   `json:"control_binary_sha256"`
	MatrixSHA256             string   `json:"matrix_sha256"`
	ProfileSHA256            string   `json:"profile_sha256"`
	VectorSetSHA256          string   `json:"vector_set_sha256"`
	GovernanceUmbrellaCommit string   `json:"governance_umbrella_commit"`
	GovernanceLockSHA256     string   `json:"governance_lock_sha256"`
	SourceGitCommit          string   `json:"source_git_commit"`
	SourceGitTag             string   `json:"source_git_tag"`
	GeneratedAtUTC           string   `json:"generated_at_utc"`
	Orchestrator             string   `json:"orchestrator"`
	ProfileID                string   `json:"profile_id"`
	ProfileName              string   `json:"profile_name"`
	Architecture             string   `json:"architecture"`
	AggregateMethod          string   `json:"aggregate_method"`
	RequiredSuites           []string `json:"required_suites"`
	HardReleaseGate          bool     `json:"hard_release_gate"`
	OfficialES6CorpusLines   int      `json:"official_es6_corpus_lines,omitempty"`
	OfficialES6CorpusSHA256  string   `json:"official_es6_corpus_sha256,omitempty"`
	// Infra-manifest binding for infra-backed/native-host evidence flows.
	InfraManifestSHA256 string            `json:"infra_manifest_sha256,omitempty"`
	InfraRepoURL        string            `json:"infra_repo_url,omitempty"`
	InfraRepoCommit     string            `json:"infra_repo_commit,omitempty"`
	IIDTrustRootSetID   string            `json:"iid_trust_root_set_id,omitempty"`
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
	// Optional substrate identity for infra-backed evidence flows.
	DiscoveredCPU    string `json:"discovered_cpu,omitempty"`
	DiscoveredKernel string `json:"discovered_kernel,omitempty"`
	ImageDigest      string `json:"image_digest,omitempty"`
	// Optional measured host identity for native-host attestation.
	MeasuredArchitecture       string `json:"measured_architecture,omitempty"`
	MeasuredOSID               string `json:"measured_os_id,omitempty"`
	MeasuredOSVersionID        string `json:"measured_os_version_id,omitempty"`
	MeasuredKernel             string `json:"measured_kernel,omitempty"`
	MeasuredCPU                string `json:"measured_cpu,omitempty"`
	AWSInstanceID              string `json:"aws_instance_id,omitempty"`
	AWSImageID                 string `json:"aws_image_id,omitempty"`
	TransportAttestationSHA256 string `json:"transport_attestation_sha256,omitempty"`
}

// EvidenceValidationOptions binds evidence metadata to expected immutable inputs.
type EvidenceValidationOptions struct {
	ExpectedBundleSHA256             string
	ExpectedControlBinarySHA256      string
	ExpectedMatrixSHA256             string
	ExpectedProfileSHA256            string
	ExpectedVectorSetSHA256          string
	ExpectedGovernanceUmbrellaCommit string
	ExpectedGovernanceLockSHA256     string
	ExpectedArchitecture             string
	ExpectedSourceGitCommit          string
	ExpectedSourceGitTag             string
	ExpectedInfraManifestSHA256      string
	ExpectedInfraRepoURL             string
	ExpectedInfraRepoCommit          string
	ExpectedInfraManifest            *InfraManifest
	RequireOfficialES6Proof          bool
	ExpectedOfficialES6CorpusLines   int
	ExpectedOfficialES6CorpusSHA256  string
}

// WriteEvidence writes a canonical JSON evidence bundle to disk.
func WriteEvidence(path string, e *EvidenceBundle) error {
	if e == nil {
		return fmt.Errorf("evidence bundle is nil")
	}
	if err := requireGovernedSchemaVersion("evidence", e.SchemaVersion); err != nil {
		return err
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
	//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 evidence paths are explicit operator/runtime artifacts.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read evidence: %w", err)
	}
	var e EvidenceBundle
	if err := decodeStrictJSONBytes("evidence", data, &e); err != nil {
		return nil, err
	}
	if err := requireGovernedSchemaVersion("evidence", e.SchemaVersion); err != nil {
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
	requiresNativeHostBinding := profileRequiresNativeHostBinding(m, p)
	if err := requireGovernedSchemaVersion("evidence", e.SchemaVersion); err != nil {
		return err
	}
	if e.SchemaVersion != EvidenceSchemaVersion {
		return fmt.Errorf("evidence schema_version %q is not the expected version %q", e.SchemaVersion, EvidenceSchemaVersion)
	}
	expectedProfileName := profileNameForEvidence(p.Name)
	if expectedProfileName == "" {
		return fmt.Errorf("unsupported profile name %q", p.Name)
	}
	if e.ProfileName != expectedProfileName {
		return fmt.Errorf("profile mismatch: evidence=%q profile=%q", e.ProfileName, expectedProfileName)
	}
	if profileIDForName(p.Name) != e.ProfileID {
		return fmt.Errorf("profile_id mismatch: evidence=%q expected=%q", e.ProfileID, profileIDForName(p.Name))
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"generated_at_utc", e.GeneratedAtUTC},
		{"orchestrator", e.Orchestrator},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("evidence %s is required", field.name)
		}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "bundle_sha256", value: e.BundleSHA256},
		{name: "control_binary_sha256", value: e.ControlBinarySHA},
		{name: "matrix_sha256", value: e.MatrixSHA256},
		{name: "profile_sha256", value: e.ProfileSHA256},
		{name: "vector_set_sha256", value: e.VectorSetSHA256},
		{name: "governance_lock_sha256", value: e.GovernanceLockSHA256},
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
	if err := validateGitCommitToken("governance_umbrella_commit", e.GovernanceUmbrellaCommit); err != nil {
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
	if opts.ExpectedVectorSetSHA256 != "" && e.VectorSetSHA256 != opts.ExpectedVectorSetSHA256 {
		return fmt.Errorf("vector_set_sha256 mismatch: evidence=%q expected=%q", e.VectorSetSHA256, opts.ExpectedVectorSetSHA256)
	}
	if expected := strings.TrimSpace(opts.ExpectedGovernanceUmbrellaCommit); expected != "" &&
		e.GovernanceUmbrellaCommit != expected {
		return fmt.Errorf("governance_umbrella_commit mismatch: evidence=%q expected=%q", e.GovernanceUmbrellaCommit, expected)
	}
	if expected := strings.TrimSpace(opts.ExpectedGovernanceLockSHA256); expected != "" &&
		e.GovernanceLockSHA256 != expected {
		return fmt.Errorf("governance_lock_sha256 mismatch: evidence=%q expected=%q", e.GovernanceLockSHA256, expected)
	}
	if expectedCommit := strings.TrimSpace(opts.ExpectedSourceGitCommit); expectedCommit != "" &&
		e.SourceGitCommit != expectedCommit {
		return fmt.Errorf("source_git_commit mismatch: evidence=%q expected=%q", e.SourceGitCommit, expectedCommit)
	}
	if expectedTag := strings.TrimSpace(opts.ExpectedSourceGitTag); expectedTag != "" &&
		e.SourceGitTag != expectedTag {
		return fmt.Errorf("source_git_tag mismatch: evidence=%q expected=%q", e.SourceGitTag, expectedTag)
	}
	if e.AggregateMethod != ReplayAggregateMethod {
		return fmt.Errorf("aggregate_method mismatch: evidence=%q expected=%q", e.AggregateMethod, ReplayAggregateMethod)
	}
	if err := validateEvidenceInfraFields(e, opts, requiresInfraBinding); err != nil {
		return err
	}
	if err := validateOfficialES6Evidence(e, opts); err != nil {
		return err
	}
	if requiresNativeHostBinding || requiresInfraBinding {
		if e.IIDTrustRootSetID != IIDTrustRootSetIDDefault {
			return fmt.Errorf("iid_trust_root_set_id mismatch: evidence=%q expected=%q", e.IIDTrustRootSetID, IIDTrustRootSetIDDefault)
		}
	}
	if err := validateNativeHostManifestExpectation(opts.ExpectedInfraManifest, requiresNativeHostBinding); err != nil {
		return err
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
	manifestIndex, err := buildInfraManifestNodeIndex(opts.ExpectedInfraManifest, requiredNodes)
	if err != nil && !errors.Is(err, errNilManifestIndex) {
		return err
	}
	if errors.Is(err, errNilManifestIndex) {
		manifestIndex = nil
	}
	if err := validateRequiredManifestBindings(manifestIndex, requiredNodes, requiresInfraBinding, requiresNativeHostBinding); err != nil {
		return err
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
		if err := validateNodeRunEvidenceFields(r, node, requiresInfraBinding, requiresNativeHostBinding); err != nil {
			return err
		}
		if err := validateNodeRunEvidenceAgainstManifest(r, manifestIndex); err != nil {
			return err
		}
		byNode[r.NodeID] = append(byNode[r.NodeID], r)
	}

	for _, id := range requiredNodes {
		runs := byNode[id]
		wantReplays := requiredReplayCount(matrixByID[id], p)
		if len(runs) < wantReplays {
			return fmt.Errorf("node %s has %d replays, want at least %d", id, len(runs), wantReplays)
		}
		seenReplay := make(map[int]struct{}, len(runs))
		for _, run := range runs {
			seenReplay[run.ReplayIndex] = struct{}{}
		}
		for i := 1; i <= wantReplays; i++ {
			if _, ok := seenReplay[i]; !ok {
				return fmt.Errorf("node %s missing replay index %d", id, i)
			}
		}
	}

	recomputedCanonical := computeReplayAggregateDigest(e.NodeReplays, func(run NodeRunEvidence) string { return run.CanonicalSHA256 })
	recomputedVerify := computeReplayAggregateDigest(e.NodeReplays, func(run NodeRunEvidence) string { return run.VerifySHA256 })
	recomputedClass := computeReplayAggregateDigest(e.NodeReplays, func(run NodeRunEvidence) string { return run.FailureClassSHA256 })
	recomputedExitCode := computeReplayAggregateDigest(e.NodeReplays, func(run NodeRunEvidence) string { return run.ExitCodeSHA256 })
	if e.AggregateCanonical != recomputedCanonical {
		return fmt.Errorf("aggregate canonical digest mismatch")
	}
	if e.AggregateVerify != recomputedVerify {
		return fmt.Errorf("aggregate verify digest mismatch")
	}
	if e.AggregateClass != recomputedClass {
		return fmt.Errorf("aggregate failure-class digest mismatch")
	}
	if e.AggregateExitCode != recomputedExitCode {
		return fmt.Errorf("aggregate exit-code digest mismatch")
	}

	suites := append([]string(nil), e.RequiredSuites...)
	sort.Strings(suites)
	wantSuites := append([]string(nil), p.RequiredSuites...)
	sort.Strings(wantSuites)
	if strings.Join(suites, ",") != strings.Join(wantSuites, ",") {
		return fmt.Errorf("required_suites mismatch")
	}

	return nil
}

func profileNameForEvidence(name string) string {
	switch {
	case name == profileNameBaseConformance:
		return profileNameBaseConformance
	case name == profileNameOfflineMeasuredEvidence, name == "infra-bound":
		return profileNameOfflineMeasuredEvidence
	case name == "max", strings.HasPrefix(name, "maximal-offline"):
		return profileNameBaseConformance
	case name == profileNameOfficialCloudMeasuredRelease, strings.HasPrefix(name, "aws-native-release-"):
		return profileNameOfficialCloudMeasuredRelease
	default:
		return ""
	}
}

func profileIDForName(name string) string {
	switch profileNameForEvidence(name) {
	case profileNameBaseConformance:
		return "https://lattice-substrate.github.io/jcs/profiles/base-conformance.v1"
	case profileNameOfflineMeasuredEvidence:
		return "https://lattice-substrate.github.io/jcs/profiles/offline-measured-evidence.v1"
	case profileNameOfficialCloudMeasuredRelease:
		return "https://lattice-substrate.github.io/jcs/profiles/official-cloud-measured-release.v1"
	default:
		return ""
	}
}

func computeReplayAggregateDigest(nodeReplays []NodeRunEvidence, selectDigest func(NodeRunEvidence) string) string {
	sorted := append([]NodeRunEvidence(nil), nodeReplays...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].NodeID == sorted[j].NodeID {
			return sorted[i].ReplayIndex < sorted[j].ReplayIndex
		}
		return sorted[i].NodeID < sorted[j].NodeID
	})
	var b strings.Builder
	for _, run := range sorted {
		b.WriteString(run.NodeID)
		b.WriteByte('\x1f')
		b.WriteString(fmt.Sprintf("%03d", run.ReplayIndex))
		b.WriteByte('\x1f')
		b.WriteString(selectDigest(run))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

//nolint:gocyclo,cyclop // REQ:OFFLINE-EVIDENCE-001 infra binding validation keeps each mismatch explicit for auditability.
func validateEvidenceInfraFields(e *EvidenceBundle, opts EvidenceValidationOptions, requiresInfraBinding bool) error {
	hasInfraFields := strings.TrimSpace(e.InfraManifestSHA256) != "" ||
		strings.TrimSpace(e.InfraRepoURL) != "" ||
		strings.TrimSpace(e.InfraRepoCommit) != ""
	hasExpectedManifest := opts.ExpectedInfraManifest != nil
	hasExpectedBinding := strings.TrimSpace(opts.ExpectedInfraManifestSHA256) != "" ||
		strings.TrimSpace(opts.ExpectedInfraRepoURL) != "" ||
		strings.TrimSpace(opts.ExpectedInfraRepoCommit) != "" ||
		hasExpectedManifest
	if !requiresInfraBinding && !hasInfraFields && !hasExpectedBinding {
		return nil
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"infra_manifest_sha256", e.InfraManifestSHA256},
		{"infra_repo_url", e.InfraRepoURL},
		{"infra_repo_commit", e.InfraRepoCommit},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("evidence requires %s", field.name)
		}
	}
	if err := validateSHA256Token("infra_manifest_sha256", e.InfraManifestSHA256); err != nil {
		return err
	}
	if !strings.HasPrefix(strings.TrimSpace(e.InfraRepoURL), "https://") {
		return fmt.Errorf("infra_repo_url must use https")
	}
	if err := validateGitCommitToken("infra_repo_commit", e.InfraRepoCommit); err != nil {
		return err
	}
	if requiresInfraBinding && opts.ExpectedInfraManifest == nil {
		return fmt.Errorf("infra-substrate-binding profile requires expected infra manifest")
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

//nolint:gocyclo,cyclop // REQ:OFFLINE-EVIDENCE-001 node replay validation keeps each field requirement explicit for operator diagnostics.
func validateNodeRunEvidenceFields(
	r NodeRunEvidence,
	node NodeSpec,
	requiresInfraBinding bool,
	requiresNativeHostBinding bool,
) error {
	if node.Mode != NodeModeContainer && strings.TrimSpace(r.ImageDigest) != "" {
		return fmt.Errorf("node %s replay %d image_digest is only allowed for container lanes", r.NodeID, r.ReplayIndex)
	}
	if !requiresInfraBinding {
		return nil
	}
	if strings.TrimSpace(r.DiscoveredCPU) == "" {
		return fmt.Errorf("node %s replay %d missing discovered_cpu for infra-substrate-binding profile", r.NodeID, r.ReplayIndex)
	}
	if strings.TrimSpace(r.DiscoveredKernel) == "" {
		return fmt.Errorf("node %s replay %d missing discovered_kernel for infra-substrate-binding profile", r.NodeID, r.ReplayIndex)
	}
	if requiresNativeHostBinding {
		for _, field := range []struct {
			name  string
			value string
		}{
			{"measured_architecture", r.MeasuredArchitecture},
			{"measured_os_id", r.MeasuredOSID},
			{"measured_os_version_id", r.MeasuredOSVersionID},
			{"measured_kernel", r.MeasuredKernel},
			{"measured_cpu", r.MeasuredCPU},
			{"aws_instance_id", r.AWSInstanceID},
			{"aws_image_id", r.AWSImageID},
			{"transport_attestation_sha256", r.TransportAttestationSHA256},
		} {
			if strings.TrimSpace(field.value) == "" {
				return fmt.Errorf("node %s replay %d missing %s for infra-substrate-binding profile", r.NodeID, r.ReplayIndex, field.name)
			}
		}
		if err := validateSHA256Token("transport_attestation_sha256", r.TransportAttestationSHA256); err != nil {
			return err
		}
	}
	if node.Mode == NodeModeContainer && strings.TrimSpace(r.ImageDigest) == "" {
		return fmt.Errorf("node %s replay %d missing image_digest for container lane in infra-substrate-binding profile", r.NodeID, r.ReplayIndex)
	}
	return nil
}

func validateNativeHostManifestExpectation(manifest *InfraManifest, requiresNativeHostBinding bool) error {
	if !requiresNativeHostBinding {
		return nil
	}
	if manifest == nil {
		return fmt.Errorf("native-host evidence requires expected infra manifest")
	}
	return nil
}

//nolint:gocyclo,cyclop // REQ:OFFLINE-EVIDENCE-001 manifest binding checks stay explicit for per-field audit attribution.
func validateRequiredManifestBindings(
	manifestIndex map[string]InfraManifestHost,
	requiredNodes []string,
	requiresInfraBinding bool,
	requiresNativeHostBinding bool,
) error {
	if !requiresInfraBinding {
		return nil
	}
	if manifestIndex == nil {
		return fmt.Errorf("infra-substrate-binding profile requires manifest host bindings")
	}
	for _, nodeID := range requiredNodes {
		host, ok := manifestIndex[nodeID]
		if !ok {
			return fmt.Errorf("infra manifest missing host binding for required node %s", nodeID)
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{"discovered_cpu", discoveredCPUReference(host)},
			{"discovered_kernel", discoveredKernelReference(host)},
		} {
			if strings.TrimSpace(field.value) == "" {
				return fmt.Errorf("infra manifest host for node %s missing %s", nodeID, field.name)
			}
		}
		if !requiresNativeHostBinding {
			continue
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{"availability_zone", host.AvailabilityZone},
			{"instance_id", host.InstanceID},
			{"os_id", host.OSID},
			{"os_version_id", host.OSVersionID},
			{"cpu", host.CPU},
			{"kernel", host.Kernel},
			{"iid_document_sha256", host.IIDDocumentSHA256},
			{"iid_signature_sha256", host.IIDSignatureSHA256},
			{"iid_pkcs7_sha256", host.IIDPKCS7SHA256},
			{"transport", host.Transport},
			{"subnet_visibility", host.SubnetVisibility},
		} {
			if strings.TrimSpace(field.value) == "" {
				return fmt.Errorf("infra manifest host for node %s missing %s", nodeID, field.name)
			}
		}
		if !host.IIDVerified {
			return fmt.Errorf("infra manifest host for node %s requires iid_verified=true", nodeID)
		}
	}
	return nil
}

func buildInfraManifestNodeIndex(manifest *InfraManifest, requiredNodes []string) (map[string]InfraManifestHost, error) {
	if manifest == nil {
		return nil, errNilManifestIndex
	}
	index := make(map[string]InfraManifestHost, len(manifest.Hosts))
	for _, host := range manifest.Hosts {
		for _, nodeID := range host.NodeIDs {
			if _, ok := index[nodeID]; ok {
				return nil, fmt.Errorf("infra manifest maps node %s more than once", nodeID)
			}
			index[nodeID] = host
		}
	}
	for _, nodeID := range requiredNodes {
		if _, ok := index[nodeID]; !ok {
			return nil, fmt.Errorf("infra manifest missing host binding for required node %s", nodeID)
		}
	}
	return index, nil
}

//nolint:gocyclo,cyclop // REQ:OFFLINE-EVIDENCE-001 manifest/evidence comparisons stay explicit for clear mismatch diagnostics.
func validateNodeRunEvidenceAgainstManifest(r NodeRunEvidence, manifestIndex map[string]InfraManifestHost) error {
	if manifestIndex == nil {
		return nil
	}
	host, ok := manifestIndex[r.NodeID]
	if !ok {
		return fmt.Errorf("node %s replay %d missing infra manifest host binding", r.NodeID, r.ReplayIndex)
	}
	for _, field := range []struct {
		name     string
		evidence string
		manifest string
	}{
		{"discovered_cpu", r.DiscoveredCPU, discoveredCPUReference(host)},
		{"discovered_kernel", r.DiscoveredKernel, discoveredKernelReference(host)},
	} {
		if strings.TrimSpace(field.evidence) == "" {
			continue
		}
		if strings.TrimSpace(field.evidence) != strings.TrimSpace(field.manifest) {
			return fmt.Errorf(
				"node %s replay %d %s mismatch: evidence=%q manifest=%q",
				r.NodeID,
				r.ReplayIndex,
				field.name,
				field.evidence,
				field.manifest,
			)
		}
	}
	if strings.TrimSpace(r.MeasuredArchitecture) == "" &&
		strings.TrimSpace(r.MeasuredOSID) == "" &&
		strings.TrimSpace(r.MeasuredOSVersionID) == "" &&
		strings.TrimSpace(r.MeasuredKernel) == "" &&
		strings.TrimSpace(r.MeasuredCPU) == "" &&
		strings.TrimSpace(r.AWSInstanceID) == "" &&
		strings.TrimSpace(r.AWSImageID) == "" {
		return nil
	}
	for _, field := range []struct {
		name     string
		evidence string
		manifest string
	}{
		{"measured_architecture", r.MeasuredArchitecture, host.Architecture},
		{"measured_os_id", r.MeasuredOSID, host.OSID},
		{"measured_os_version_id", r.MeasuredOSVersionID, host.OSVersionID},
		{"measured_kernel", r.MeasuredKernel, host.Kernel},
		{"measured_cpu", r.MeasuredCPU, host.CPU},
		{"aws_instance_id", r.AWSInstanceID, host.InstanceID},
		{"aws_image_id", r.AWSImageID, host.ImageID},
	} {
		if strings.TrimSpace(field.evidence) != strings.TrimSpace(field.manifest) {
			return fmt.Errorf(
				"node %s replay %d %s mismatch: evidence=%q manifest=%q",
				r.NodeID,
				r.ReplayIndex,
				field.name,
				field.evidence,
				field.manifest,
			)
		}
	}
	return nil
}

func discoveredCPUReference(host InfraManifestHost) string {
	if strings.TrimSpace(host.DiscoveredCPU) != "" {
		return host.DiscoveredCPU
	}
	return host.CPU
}

func discoveredKernelReference(host InfraManifestHost) string {
	if strings.TrimSpace(host.DiscoveredKernel) != "" {
		return host.DiscoveredKernel
	}
	return host.Kernel
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

// ProfileRequiresOfficialES6NumberCorpus reports whether a replay profile binds the
// governed full official ES6 checksum proof into release evidence.
func ProfileRequiresOfficialES6NumberCorpus(profile *Profile) bool {
	if profile == nil {
		return false
	}
	for _, suite := range profile.RequiredSuites {
		if suite == OfficialES6NumberCorpusSuite {
			return true
		}
	}
	return false
}

func profileRequiresNativeHostBinding(matrix *Matrix, profile *Profile) bool {
	if !profileRequiresInfraBinding(profile) {
		return false
	}
	if profile != nil && strings.HasPrefix(strings.TrimSpace(profile.Name), "aws-native-release-") {
		return true
	}
	if matrix == nil {
		return false
	}
	requiredNodes, err := requiredNodeIDs(matrix, profile)
	if err != nil {
		return false
	}
	required := make(map[string]struct{}, len(requiredNodes))
	for _, nodeID := range requiredNodes {
		required[nodeID] = struct{}{}
	}
	for _, node := range matrix.Nodes {
		if _, ok := required[node.ID]; ok && node.Runner.Kind == "vm_ssm" {
			return true
		}
	}
	return false
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

func validateOfficialES6Evidence(e *EvidenceBundle, opts EvidenceValidationOptions) error {
	if e == nil {
		return fmt.Errorf("evidence bundle is nil")
	}
	if e.OfficialES6CorpusLines < 0 {
		return fmt.Errorf("official_es6_corpus_lines must be >= 0")
	}
	if strings.TrimSpace(e.OfficialES6CorpusSHA256) != "" && e.OfficialES6CorpusLines < 1 {
		return fmt.Errorf("official_es6_corpus_lines must be >= 1 when official_es6_corpus_sha256 is set")
	}
	if strings.TrimSpace(e.OfficialES6CorpusSHA256) != "" {
		if err := validateSHA256Token("official_es6_corpus_sha256", e.OfficialES6CorpusSHA256); err != nil {
			return err
		}
	}
	if !opts.RequireOfficialES6Proof {
		return nil
	}
	if e.OfficialES6CorpusLines != opts.ExpectedOfficialES6CorpusLines {
		return fmt.Errorf(
			"official_es6_corpus_lines mismatch: evidence=%d expected=%d",
			e.OfficialES6CorpusLines,
			opts.ExpectedOfficialES6CorpusLines,
		)
	}
	if e.OfficialES6CorpusSHA256 != opts.ExpectedOfficialES6CorpusSHA256 {
		return fmt.Errorf(
			"official_es6_corpus_sha256 mismatch: evidence=%q expected=%q",
			e.OfficialES6CorpusSHA256,
			opts.ExpectedOfficialES6CorpusSHA256,
		)
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
