package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// InfraManifestSchemaVersion is the stable schema identifier for infrastructure manifests.
const InfraManifestSchemaVersion = "infra-manifest.v1"

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
}

// InfraManifestHost describes one provisioned cloud host.
type InfraManifestHost struct {
	Role             string `json:"role"`
	CloudProvider    string `json:"cloud_provider"`
	Region           string `json:"region"`
	InstanceType     string `json:"instance_type"`
	ImageID          string `json:"image_id"`
	DiscoveredCPU    string `json:"discovered_cpu,omitempty"`
	DiscoveredKernel string `json:"discovered_kernel,omitempty"`
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
	seenRoles := make(map[string]struct{}, len(im.Hosts))
	for i, h := range im.Hosts {
		if err := validateInfraManifestHost(i, h, seenRoles); err != nil {
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
func validateInfraManifestHost(i int, h InfraManifestHost, seenRoles map[string]struct{}) error {
	switch h.Role {
	case "x86_64", "arm64":
	default:
		return fmt.Errorf("infra manifest host[%d] role must be x86_64 or arm64, got %q", i, h.Role)
	}
	if _, ok := seenRoles[h.Role]; ok {
		return fmt.Errorf("infra manifest duplicate host role %q", h.Role)
	}
	seenRoles[h.Role] = struct{}{}
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
	return nil
}
