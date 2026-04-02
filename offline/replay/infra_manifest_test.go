package replay_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const testArchX8664 = "x86_64"

func validInfraManifestFixture() *replay.InfraManifest {
	return &replay.InfraManifest{
		SchemaVersion:      replay.InfraManifestSchemaVersion,
		GeneratedAtUTC:     "2026-01-01T00:00:00Z",
		InfraRepoURL:       "https://github.com/example/json-canon-conformance-infra",
		InfraRepoCommit:    strings.Repeat("a", 40),
		ProviderEngine:     "opentofu",
		ProviderVersion:    "1.8.0",
		ProviderLockSHA256: strings.Repeat("b", 64),
		Hosts: []replay.InfraManifestHost{
			{
				Architecture:  testArchX8664,
				NodeIDs:       []string{"aws-native-debian13-amd-x86_64"},
				Role:          testArchX8664,
				CloudProvider: "aws",
				Region:        "us-east-1",
				InstanceType:  "c6i.large",
				ImageID:       "ami-0abc1234",
			},
			{
				Architecture:  "arm64",
				NodeIDs:       []string{"aws-native-debian13-g2-arm64"},
				Role:          "arm64",
				CloudProvider: "aws",
				Region:        "us-east-1",
				InstanceType:  "c7g.large",
				ImageID:       "ami-0def5678",
			},
		},
		Tools: []replay.InfraManifestTool{
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
			{
				ID:                     "tofu-linux-amd64",
				Scope:                  replay.ToolchainScopeHost,
				Purpose:                "provision",
				Name:                   "opentofu",
				Version:                "1.10.6",
				OS:                     "linux",
				Arch:                   "amd64",
				Format:                 "zip",
				SourceURL:              "https://github.com/opentofu/opentofu/releases/download/v1.10.6/tofu_1.10.6_linux_amd64.zip",
				SHA256:                 strings.Repeat("d", 64),
				ArtifactRelativePath:   "toolchain/downloads/tofu-linux-amd64/tofu_1.10.6_linux_amd64.zip",
				ExecutableRelativePath: "toolchain/.extracted/tofu-linux-amd64/tofu",
			},
		},
	}
}

func validInfraManifestV2Fixture() *replay.InfraManifest {
	im := validInfraManifestFixture()
	im.SchemaVersion = replay.InfraManifestSchemaVersionV2
	im.Hosts[0].AvailabilityZone = "us-east-1a"
	im.Hosts[0].InstanceID = "i-0123456789abcdef0"
	im.Hosts[0].OSID = "debian"
	im.Hosts[0].OSVersionID = "13"
	im.Hosts[0].CPU = "Intel(R) Xeon(R)"
	im.Hosts[0].Kernel = "6.1.0-28-cloud-amd64"
	im.Hosts[0].IIDDocumentSHA256 = strings.Repeat("1", 64)
	im.Hosts[0].IIDSignatureSHA256 = strings.Repeat("2", 64)
	im.Hosts[0].Transport = "ssh"
	im.Hosts[0].SubnetVisibility = "public"
	im.Hosts[1].AvailabilityZone = "us-east-1b"
	im.Hosts[1].InstanceID = "i-abcdef01234567890"
	im.Hosts[1].OSID = "debian"
	im.Hosts[1].OSVersionID = "13"
	im.Hosts[1].CPU = "Neoverse"
	im.Hosts[1].Kernel = "6.1.0-28-cloud-arm64"
	im.Hosts[1].IIDDocumentSHA256 = strings.Repeat("3", 64)
	im.Hosts[1].IIDSignatureSHA256 = strings.Repeat("4", 64)
	im.Hosts[1].Transport = "ssh"
	im.Hosts[1].SubnetVisibility = "public"
	return im
}

func TestValidateInfraManifest(t *testing.T) {
	im := validInfraManifestFixture()
	if err := replay.ValidateInfraManifest(im); err != nil {
		t.Fatalf("valid manifest failed: %v", err)
	}
}

func TestValidateInfraManifestV2(t *testing.T) {
	im := validInfraManifestV2Fixture()
	if err := replay.ValidateInfraManifest(im); err != nil {
		t.Fatalf("valid v2 manifest failed: %v", err)
	}
}

func TestValidateInfraManifestOptionalDiscoveredFields(t *testing.T) {
	im := validInfraManifestFixture()
	im.Hosts[0].DiscoveredCPU = "Intel(R) Xeon(R) Platinum 8375C"
	im.Hosts[0].DiscoveredKernel = "6.1.0-28-cloud-amd64"
	im.Hosts[1].DiscoveredCPU = "Neoverse-N1 (AWS Graviton3)"
	im.Hosts[1].DiscoveredKernel = "6.1.0-28-cloud-arm64"
	if err := replay.ValidateInfraManifest(im); err != nil {
		t.Fatalf("manifest with discovered fields failed: %v", err)
	}
}

func TestValidateInfraManifestRejectsInvalid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*replay.InfraManifest)
		want   string
	}{
		{
			name:   "nil",
			mutate: nil,
			want:   "is nil",
		},
		{
			name: "wrong schema version",
			mutate: func(im *replay.InfraManifest) {
				im.SchemaVersion = "infra-manifest.v99"
			},
			want: "unsupported infra manifest schema_version",
		},
		{
			name: "missing generated_at_utc",
			mutate: func(im *replay.InfraManifest) {
				im.GeneratedAtUTC = ""
			},
			want: "generated_at_utc is required",
		},
		{
			name: "missing infra_repo_url",
			mutate: func(im *replay.InfraManifest) {
				im.InfraRepoURL = ""
			},
			want: "infra_repo_url is required",
		},
		{
			name: "non-https infra_repo_url",
			mutate: func(im *replay.InfraManifest) {
				im.InfraRepoURL = "http://example.com/repo"
			},
			want: "infra_repo_url must use https",
		},
		{
			name: "bad infra_repo_commit",
			mutate: func(im *replay.InfraManifest) {
				im.InfraRepoCommit = "notahex"
			},
			want: "infra_repo_commit",
		},
		{
			name: "missing provider_engine",
			mutate: func(im *replay.InfraManifest) {
				im.ProviderEngine = ""
			},
			want: "provider_engine is required",
		},
		{
			name: "missing provider_version",
			mutate: func(im *replay.InfraManifest) {
				im.ProviderVersion = ""
			},
			want: "provider_version is required",
		},
		{
			name: "bad provider_lock_sha256",
			mutate: func(im *replay.InfraManifest) {
				im.ProviderLockSHA256 = "short"
			},
			want: "provider_lock_sha256",
		},
		{
			name: "no hosts",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts = nil
			},
			want: "at least one host",
		},
		{
			name: "no tools",
			mutate: func(im *replay.InfraManifest) {
				im.Tools = nil
			},
			want: "at least one pinned tool artifact",
		},
		{
			name: "invalid architecture",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts[0].Architecture = "mips64"
			},
			want: "architecture must be x86_64 or arm64",
		},
		{
			name: "duplicate role",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts[1].Role = testArchX8664
			},
			want: "duplicate host role",
		},
		{
			name: "missing cloud_provider",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts[0].CloudProvider = ""
			},
			want: "cloud_provider is required",
		},
		{
			name: "missing region",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts[0].Region = ""
			},
			want: "region is required",
		},
		{
			name: "missing instance_type",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts[0].InstanceType = ""
			},
			want: "instance_type is required",
		},
		{
			name: "missing image_id",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts[0].ImageID = ""
			},
			want: "image_id is required",
		},
		{
			name: "host tool missing executable path",
			mutate: func(im *replay.InfraManifest) {
				im.Tools[0].ExecutableRelativePath = ""
			},
			want: "executable_relative_path is required",
		},
		{
			name: "tool artifact path must be relative",
			mutate: func(im *replay.InfraManifest) {
				im.Tools[0].ArtifactRelativePath = "/tmp/go.tar.gz"
			},
			want: "artifact_relative_path must be relative",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mutate == nil {
				err := replay.ValidateInfraManifest(nil)
				if err == nil {
					t.Fatal("expected error for nil manifest")
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("expected %q, got %v", tc.want, err)
				}
				return
			}
			im := validInfraManifestFixture()
			tc.mutate(im)
			err := replay.ValidateInfraManifest(im)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateInfraManifestV2RejectsMissingAttestationFields(t *testing.T) {
	im := validInfraManifestV2Fixture()
	im.Hosts[0].IIDDocumentSHA256 = ""
	err := replay.ValidateInfraManifest(im)
	if err == nil {
		t.Fatal("expected v2 attestation validation error")
	}
	if !strings.Contains(err.Error(), "iid_document_sha256") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInfraManifest(t *testing.T) {
	im := validInfraManifestFixture()
	data, err := json.MarshalIndent(im, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "infra-manifest.json")
	if writeErr := os.WriteFile(path, append(data, '\n'), 0o600); writeErr != nil {
		t.Fatalf("write manifest: %v", writeErr)
	}
	loaded, err := replay.LoadInfraManifest(path)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if loaded.SchemaVersion != replay.InfraManifestSchemaVersion {
		t.Fatalf("schema_version mismatch: %q", loaded.SchemaVersion)
	}
	if len(loaded.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(loaded.Hosts))
	}
	if len(loaded.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(loaded.Tools))
	}
	if loaded.Hosts[0].Role != testArchX8664 {
		t.Fatalf("expected host[0].role=x86_64, got %q", loaded.Hosts[0].Role)
	}
	if loaded.Hosts[0].Architecture != testArchX8664 {
		t.Fatalf("expected host[0].architecture=x86_64, got %q", loaded.Hosts[0].Architecture)
	}
	if loaded.Hosts[1].Role != "arm64" {
		t.Fatalf("expected host[1].role=arm64, got %q", loaded.Hosts[1].Role)
	}
}

func TestLoadInfraManifestRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "infra-manifest.json")
	raw := `{
  "schema_version": "infra-manifest.v1",
  "generated_at_utc": "2026-01-01T00:00:00Z",
  "infra_repo_url": "https://example.com",
  "infra_repo_commit": "` + strings.Repeat("a", 40) + `",
  "provider_engine": "opentofu",
  "provider_version": "1.8.0",
  "provider_lock_sha256": "` + strings.Repeat("b", 64) + `",
  "hosts": [{"architecture":"x86_64","node_ids":["aws-native-debian13-amd-x86_64"],"role":"x86_64","cloud_provider":"aws","region":"us-east-1","instance_type":"c6i.large","image_id":"ami-0abc"}],
  "tools": [{"id":"go-linux-amd64","scope":"host","purpose":"build","name":"go","version":"1.24.13","os":"linux","arch":"amd64","format":"tar.gz","source_url":"https://go.dev/dl/go1.24.13.linux-amd64.tar.gz","sha256":"` + strings.Repeat("c", 64) + `","artifact_relative_path":"toolchain/downloads/go/go.tar.gz","executable_relative_path":"toolchain/.extracted/go/bin/go"}],
  "unknown_field": "should_fail"
}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	_, err := replay.LoadInfraManifest(path)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}
