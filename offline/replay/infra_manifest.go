package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// InfraManifestSchemaVersion is the schema identifier for infrastructure manifests.
	InfraManifestSchemaVersion = "infra-manifest.v1"
)

// InfraManifest records the IaC-provisioned substrate identity for a conformance run.
// It captures what was requested (AMI, instance type, IaC commit) and optionally
// what was discovered (CPU model, kernel version) on each provisioned host.
type InfraManifest struct {
	SchemaVersion      string              `json:"schema_version"`
	GeneratedAtUTC     string              `json:"generated_at_utc"`
	InfraRepoURL       string              `json:"infra_repo_url"`
	InfraRepoCommit    string              `json:"infra_repo_commit"`
	ProviderEngine     string              `json:"provider_engine"`
	ProviderVersion    string              `json:"provider_version"`
	ProviderLockSHA256 string              `json:"provider_lock_sha256"`
	Hosts              []InfraManifestHost `json:"hosts"`
	Tools              []InfraManifestTool `json:"tools"`
}

// InfraManifestHost describes one provisioned cloud host.
type InfraManifestHost struct {
	Architecture       string   `json:"architecture"`
	NodeIDs            []string `json:"node_ids"`
	Role               string   `json:"role"`
	CloudProvider      string   `json:"cloud_provider"`
	Region             string   `json:"region"`
	AvailabilityZone   string   `json:"availability_zone,omitempty"`
	InstanceType       string   `json:"instance_type"`
	InstanceID         string   `json:"instance_id,omitempty"`
	ImageID            string   `json:"image_id"`
	OSID               string   `json:"os_id,omitempty"`
	OSVersionID        string   `json:"os_version_id,omitempty"`
	CPU                string   `json:"cpu,omitempty"`
	Kernel             string   `json:"kernel,omitempty"`
	IIDDocumentSHA256  string   `json:"iid_document_sha256,omitempty"`
	IIDSignatureSHA256 string   `json:"iid_signature_sha256,omitempty"`
	Transport          string   `json:"transport,omitempty"`
	SubnetVisibility   string   `json:"subnet_visibility,omitempty"`
	DiscoveredCPU      string   `json:"discovered_cpu,omitempty"`
	DiscoveredKernel   string   `json:"discovered_kernel,omitempty"`
}

// InfraManifestTool records one pinned tool artifact used in the evidenced release flow.
type InfraManifestTool struct {
	ID                     string `json:"id"`
	Scope                  string `json:"scope"`
	Purpose                string `json:"purpose"`
	Name                   string `json:"name"`
	Version                string `json:"version"`
	OS                     string `json:"os"`
	Arch                   string `json:"arch"`
	Format                 string `json:"format"`
	SourceURL              string `json:"source_url"`
	SHA256                 string `json:"sha256"`
	ArtifactRelativePath   string `json:"artifact_relative_path"`
	ExecutableRelativePath string `json:"executable_relative_path,omitempty"`
}

// LoadInfraManifest reads, decodes, and validates an infrastructure manifest document.
//
//nolint:gosec // REQ:OFFLINE-INFRA-001 infra manifest path is explicit operator input for release-gate validation.
func LoadInfraManifest(path string) (*InfraManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read infra manifest: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var im InfraManifest
	if err := dec.Decode(&im); err != nil {
		return nil, fmt.Errorf("decode infra manifest json: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode infra manifest json: unexpected trailing json content")
		}
		return nil, fmt.Errorf("decode infra manifest json: decode trailing json token: %w", err)
	}
	if err := ValidateInfraManifest(&im); err != nil {
		return nil, err
	}
	return &im, nil
}

// ValidateInfraManifest validates infrastructure manifest semantics.
func ValidateInfraManifest(im *InfraManifest) error {
	if im == nil {
		return fmt.Errorf("infra manifest is nil")
	}
	if err := validateInfraManifestScalars(im); err != nil {
		return err
	}
	if len(im.Hosts) == 0 {
		return fmt.Errorf("infra manifest must include at least one host")
	}
	if len(im.Tools) == 0 {
		return fmt.Errorf("infra manifest must include at least one pinned tool artifact")
	}
	seenRoles := make(map[string]struct{}, len(im.Hosts))
	seenNodeIDs := make(map[string]string, len(im.Hosts))
	for i, h := range im.Hosts {
		if err := validateInfraManifestHost(i, h, seenRoles, seenNodeIDs); err != nil {
			return err
		}
	}
	seenTools := make(map[string]struct{}, len(im.Tools))
	for i, tool := range im.Tools {
		if err := validateInfraManifestTool(i, tool, seenTools); err != nil {
			return err
		}
	}
	return nil
}

// validateInfraManifestScalars checks all required scalar fields in an InfraManifest.
// Extracted from ValidateInfraManifest to keep cyclomatic complexity within lint bounds.
func validateInfraManifestScalars(im *InfraManifest) error {
	if im.SchemaVersion != InfraManifestSchemaVersion {
		return fmt.Errorf("unsupported infra manifest schema_version %q", im.SchemaVersion)
	}
	if strings.TrimSpace(im.GeneratedAtUTC) == "" {
		return fmt.Errorf("infra manifest generated_at_utc is required")
	}
	if strings.TrimSpace(im.InfraRepoURL) == "" {
		return fmt.Errorf("infra manifest infra_repo_url is required")
	}
	if !strings.HasPrefix(strings.TrimSpace(im.InfraRepoURL), "https://") {
		return fmt.Errorf("infra manifest infra_repo_url must use https")
	}
	if err := validateGitCommitToken("infra_repo_commit", im.InfraRepoCommit); err != nil {
		return fmt.Errorf("infra manifest %w", err)
	}
	if strings.TrimSpace(im.ProviderEngine) == "" {
		return fmt.Errorf("infra manifest provider_engine is required")
	}
	if strings.TrimSpace(im.ProviderVersion) == "" {
		return fmt.Errorf("infra manifest provider_version is required")
	}
	if err := validateSHA256Token("provider_lock_sha256", im.ProviderLockSHA256); err != nil {
		return fmt.Errorf("infra manifest %w", err)
	}
	return nil
}

// validateInfraManifestHost checks one host entry and records its role in seenRoles.
// Extracted from ValidateInfraManifest to keep cyclomatic complexity within lint bounds.
func validateInfraManifestHost(
	i int,
	h InfraManifestHost,
	seenRoles map[string]struct{},
	seenNodeIDs map[string]string,
) error {
	if err := validateInfraManifestHostIdentity(i, h, seenRoles); err != nil {
		return err
	}
	if err := validateInfraManifestHostNodeIDs(i, h.NodeIDs, seenNodeIDs, h.Role); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{
		{"cloud_provider", h.CloudProvider},
		{"region", h.Region},
		{"instance_type", h.InstanceType},
		{"image_id", h.ImageID},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("infra manifest host[%d] %s is required", i, field.name)
		}
	}
	for _, field := range []struct{ name, value string }{
		{"iid_document_sha256", h.IIDDocumentSHA256},
		{"iid_signature_sha256", h.IIDSignatureSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			continue
		}
		if err := validateSHA256Token(field.name, field.value); err != nil {
			return fmt.Errorf("infra manifest host[%d] %w", i, err)
		}
	}
	if transport := strings.TrimSpace(h.Transport); transport != "" {
		switch transport {
		case "ssh", "ssm":
		default:
			return fmt.Errorf("infra manifest host[%d] transport must be ssh or ssm, got %q", i, h.Transport)
		}
	}
	if visibility := strings.TrimSpace(h.SubnetVisibility); visibility != "" {
		switch visibility {
		case "public", "private":
		default:
			return fmt.Errorf("infra manifest host[%d] subnet_visibility must be public or private, got %q", i, h.SubnetVisibility)
		}
	}
	return nil
}

func validateInfraManifestHostIdentity(i int, h InfraManifestHost, seenRoles map[string]struct{}) error {
	if strings.TrimSpace(h.Role) == "" {
		return fmt.Errorf("infra manifest host[%d] role is required", i)
	}
	if _, ok := seenRoles[h.Role]; ok {
		return fmt.Errorf("infra manifest duplicate host role %q", h.Role)
	}
	seenRoles[h.Role] = struct{}{}
	switch h.Architecture {
	case architectureX8664, architectureARM64:
		return nil
	default:
		return fmt.Errorf("infra manifest host[%d] architecture must be %s or %s, got %q", i, architectureX8664, architectureARM64, h.Architecture)
	}
}

func validateInfraManifestHostNodeIDs(i int, nodeIDs []string, seenNodeIDs map[string]string, role string) error {
	if len(nodeIDs) == 0 {
		return fmt.Errorf("infra manifest host[%d] node_ids is required", i)
	}
	seenHostNodeIDs := make(map[string]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			return fmt.Errorf("infra manifest host[%d] node_ids must not contain empty values", i)
		}
		if _, ok := seenHostNodeIDs[nodeID]; ok {
			return fmt.Errorf("infra manifest host[%d] duplicate node_id %q", i, nodeID)
		}
		if existingRole, ok := seenNodeIDs[nodeID]; ok {
			return fmt.Errorf("infra manifest node_id %q appears in multiple hosts: %s and %s", nodeID, existingRole, role)
		}
		seenHostNodeIDs[nodeID] = struct{}{}
		seenNodeIDs[nodeID] = role
	}
	return nil
}

func validateInfraManifestTool(i int, tool InfraManifestTool, seenTools map[string]struct{}) error {
	if err := validateInfraManifestToolFields(i, tool); err != nil {
		return err
	}
	if err := validateInfraManifestToolIdentity(i, tool); err != nil {
		return err
	}
	return validateInfraManifestToolPaths(i, tool, seenTools)
}

func validateInfraManifestToolFields(i int, tool InfraManifestTool) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"id", tool.ID},
		{"scope", tool.Scope},
		{"purpose", tool.Purpose},
		{"name", tool.Name},
		{"version", tool.Version},
		{"os", tool.OS},
		{"arch", tool.Arch},
		{"format", tool.Format},
		{"source_url", tool.SourceURL},
		{"artifact_relative_path", tool.ArtifactRelativePath},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("infra manifest tool[%d] %s is required", i, field.name)
		}
	}
	return nil
}

func validateInfraManifestToolIdentity(i int, tool InfraManifestTool) error {
	switch tool.Scope {
	case ToolchainScopeHost, ToolchainScopeRemote:
	default:
		return fmt.Errorf("infra manifest tool[%d] scope must be host or remote, got %q", i, tool.Scope)
	}
	switch tool.Arch {
	case architectureAMD64, architectureARM64:
	default:
		return fmt.Errorf("infra manifest tool[%d] arch must be %s or %s, got %q", i, architectureAMD64, architectureARM64, tool.Arch)
	}
	switch tool.Format {
	case toolchainFormatRaw, toolchainFormatTGZ, toolchainFormatZIP:
	default:
		return fmt.Errorf("infra manifest tool[%d] unsupported format %q", i, tool.Format)
	}
	if !strings.HasPrefix(tool.SourceURL, "https://") {
		return fmt.Errorf("infra manifest tool[%d] source_url must use https", i)
	}
	if err := validateSHA256Token("sha256", tool.SHA256); err != nil {
		return fmt.Errorf("infra manifest tool[%d] %w", i, err)
	}
	return nil
}

func validateInfraManifestToolPaths(i int, tool InfraManifestTool, seenTools map[string]struct{}) error {
	if _, ok := seenTools[tool.ID]; ok {
		return fmt.Errorf("infra manifest duplicate tool id %q", tool.ID)
	}
	seenTools[tool.ID] = struct{}{}
	if err := validateManifestRelativePath("artifact_relative_path", tool.ArtifactRelativePath, true); err != nil {
		return fmt.Errorf("infra manifest tool[%d] %w", i, err)
	}
	if tool.Scope == ToolchainScopeHost && strings.TrimSpace(tool.ExecutableRelativePath) == "" {
		return fmt.Errorf("infra manifest tool[%d] executable_relative_path is required for host tools", i)
	}
	if err := validateManifestRelativePath("executable_relative_path", tool.ExecutableRelativePath, tool.Scope == ToolchainScopeHost); err != nil {
		return fmt.Errorf("infra manifest tool[%d] %w", i, err)
	}
	return nil
}

func validateManifestRelativePath(name, path string, required bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("%s must be relative", name)
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return fmt.Errorf("%s must not contain '..'", name)
		}
	}
	return nil
}
