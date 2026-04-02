package replay

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	// ToolchainLockSchemaVersion is the pinned-tool lock format identifier.
	ToolchainLockSchemaVersion = "toolchain-lock.v1"

	// ToolchainScopeHost marks artifacts executed on the orchestration host.
	ToolchainScopeHost = "host"
	// ToolchainScopeRemote marks artifacts installed onto replay targets.
	ToolchainScopeRemote = "remote"

	toolchainFormatRaw    = "raw"
	toolchainFormatTGZ    = "tar.gz"
	toolchainFormatZIP    = "zip"
	toolchainHeaderLine   = "id\tscope\tpurpose\tname\tversion\tos\tarch\tformat\tsource_url\tsha256\texecutable_path"
	toolchainSchemaPrefix = "# schema_version="
)

// ToolchainLock records pinned release-tool artifacts that are downloaded and verified
// before a release or server-backed evidence run.
type ToolchainLock struct {
	SchemaVersion string
	Artifacts     []ToolchainArtifact
}

// ToolchainArtifact describes one pinned binary artifact.
type ToolchainArtifact struct {
	ID             string
	Scope          string
	Purpose        string
	Name           string
	Version        string
	OS             string
	Arch           string
	Format         string
	SourceURL      string
	SHA256         string
	ExecutablePath string
}

// LoadToolchainLock reads, parses, and validates the pinned toolchain lock file.
//
//nolint:gosec // REQ:OFFLINE-TOOLCHAIN-001 toolchain lock path is explicit operator-controlled input.
func LoadToolchainLock(path string) (*ToolchainLock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read toolchain lock: %w", err)
	}
	lock, err := parseToolchainLock(string(data))
	if err != nil {
		return nil, err
	}
	if err := ValidateToolchainLock(lock); err != nil {
		return nil, err
	}
	return lock, nil
}

func parseToolchainLock(raw string) (*ToolchainLock, error) {
	lock := &ToolchainLock{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	lineNo := 0
	seenHeader := false
	for scanner.Scan() {
		lineNo++
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if strings.HasPrefix(trimmed, toolchainSchemaPrefix) {
				lock.SchemaVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, toolchainSchemaPrefix))
			}
			continue
		}
		if !seenHeader {
			if line != toolchainHeaderLine {
				return nil, fmt.Errorf("parse toolchain lock: invalid header line %d", lineNo)
			}
			seenHeader = true
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 11 {
			return nil, fmt.Errorf("parse toolchain lock: line %d has %d fields, want 11", lineNo, len(fields))
		}
		lock.Artifacts = append(lock.Artifacts, ToolchainArtifact{
			ID:             strings.TrimSpace(fields[0]),
			Scope:          strings.TrimSpace(fields[1]),
			Purpose:        strings.TrimSpace(fields[2]),
			Name:           strings.TrimSpace(fields[3]),
			Version:        strings.TrimSpace(fields[4]),
			OS:             strings.TrimSpace(fields[5]),
			Arch:           strings.TrimSpace(fields[6]),
			Format:         strings.TrimSpace(fields[7]),
			SourceURL:      strings.TrimSpace(fields[8]),
			SHA256:         strings.TrimSpace(fields[9]),
			ExecutablePath: strings.TrimSpace(fields[10]),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan toolchain lock: %w", err)
	}
	if !seenHeader {
		return nil, fmt.Errorf("parse toolchain lock: missing header line")
	}
	return lock, nil
}

// ValidateToolchainLock validates schema and artifact invariants.
func ValidateToolchainLock(lock *ToolchainLock) error {
	if lock == nil {
		return fmt.Errorf("toolchain lock is nil")
	}
	if lock.SchemaVersion != ToolchainLockSchemaVersion {
		return fmt.Errorf("unsupported toolchain lock schema_version %q", lock.SchemaVersion)
	}
	if len(lock.Artifacts) == 0 {
		return fmt.Errorf("toolchain lock must include at least one artifact")
	}
	seenIDs := make(map[string]struct{}, len(lock.Artifacts))
	for i, artifact := range lock.Artifacts {
		if err := validateToolchainArtifact(i, artifact); err != nil {
			return err
		}
		if _, ok := seenIDs[artifact.ID]; ok {
			return fmt.Errorf("toolchain lock duplicate artifact id %q", artifact.ID)
		}
		seenIDs[artifact.ID] = struct{}{}
	}
	return nil
}

func validateToolchainArtifact(i int, artifact ToolchainArtifact) error {
	if err := validateToolchainArtifactFields(i, artifact); err != nil {
		return err
	}
	if err := validateToolchainArtifactIdentity(i, artifact); err != nil {
		return err
	}
	return validateToolchainArtifactPaths(i, artifact)
}

func validateToolchainArtifactFields(i int, artifact ToolchainArtifact) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"id", artifact.ID},
		{"purpose", artifact.Purpose},
		{"name", artifact.Name},
		{"version", artifact.Version},
		{"os", artifact.OS},
		{"arch", artifact.Arch},
		{"source_url", artifact.SourceURL},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("toolchain lock artifact[%d] %s is required", i, field.name)
		}
	}
	return nil
}

func validateToolchainArtifactIdentity(i int, artifact ToolchainArtifact) error {
	switch artifact.Scope {
	case ToolchainScopeHost, ToolchainScopeRemote:
	default:
		return fmt.Errorf("toolchain lock artifact[%d] scope must be host or remote, got %q", i, artifact.Scope)
	}
	switch artifact.Format {
	case toolchainFormatRaw, toolchainFormatTGZ, toolchainFormatZIP:
	default:
		return fmt.Errorf("toolchain lock artifact[%d] unsupported format %q", i, artifact.Format)
	}
	switch artifact.Arch {
	case architectureAMD64, architectureARM64:
	default:
		return fmt.Errorf("toolchain lock artifact[%d] arch must be %s or %s, got %q", i, architectureAMD64, architectureARM64, artifact.Arch)
	}
	if !strings.HasPrefix(artifact.SourceURL, "https://") {
		return fmt.Errorf("toolchain lock artifact[%d] source_url must use https", i)
	}
	if err := validateSHA256Token("sha256", artifact.SHA256); err != nil {
		return fmt.Errorf("toolchain lock artifact[%d] %w", i, err)
	}
	return nil
}

func validateToolchainArtifactPaths(i int, artifact ToolchainArtifact) error {
	if artifact.Scope == ToolchainScopeHost && strings.TrimSpace(artifact.ExecutablePath) == "" {
		return fmt.Errorf("toolchain lock artifact[%d] executable_path is required for host tools", i)
	}
	if err := validateManifestRelativePath("toolchain executable_path", artifact.ExecutablePath, artifact.Scope == ToolchainScopeHost); err != nil {
		return fmt.Errorf("toolchain lock artifact[%d] %w", i, err)
	}
	return nil
}

// SelectToolchainArtifacts returns the host-arch-specific host tools plus all remote tools.
func SelectToolchainArtifacts(lock *ToolchainLock, hostArch string) ([]ToolchainArtifact, error) {
	return SelectToolchainArtifactsForPurposes(lock, hostArch, nil)
}

// SelectToolchainArtifactsForPurposes returns the host-arch-specific host tools plus any
// remote tools whose purposes are explicitly allowed. When purposes is empty, all purposes
// are selected.
func SelectToolchainArtifactsForPurposes(lock *ToolchainLock, hostArch string, purposes []string) ([]ToolchainArtifact, error) {
	if err := ValidateToolchainLock(lock); err != nil {
		return nil, err
	}
	hostArch = NormalizeToolchainArch(hostArch)
	switch hostArch {
	case "amd64", "arm64":
	default:
		return nil, fmt.Errorf("unsupported host toolchain arch %q", hostArch)
	}
	allowedPurposes := make(map[string]struct{}, len(purposes))
	for _, purpose := range purposes {
		purpose = strings.TrimSpace(purpose)
		if purpose == "" {
			continue
		}
		allowedPurposes[purpose] = struct{}{}
	}
	selectAllPurposes := len(allowedPurposes) == 0
	selected := make([]ToolchainArtifact, 0, len(lock.Artifacts))
	for _, artifact := range lock.Artifacts {
		if !toolchainPurposeAllowed(artifact, allowedPurposes, selectAllPurposes) {
			continue
		}
		if artifact.Scope == ToolchainScopeRemote || artifact.Arch == hostArch {
			selected = append(selected, artifact)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].ID < selected[j].ID
	})
	return selected, nil
}

func toolchainPurposeAllowed(artifact ToolchainArtifact, allowedPurposes map[string]struct{}, selectAllPurposes bool) bool {
	if selectAllPurposes {
		return true
	}
	_, ok := allowedPurposes[artifact.Purpose]
	return ok
}

// NormalizeToolchainArch maps common runtime arch tokens to the lock-file arch vocabulary.
func NormalizeToolchainArch(raw string) string {
	switch strings.TrimSpace(raw) {
	case architectureX8664, architectureAMD64:
		return architectureAMD64
	case "aarch64", architectureARM64:
		return architectureARM64
	default:
		return strings.TrimSpace(raw)
	}
}
