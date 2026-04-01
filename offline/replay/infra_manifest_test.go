package replay_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

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
				Role:          "x86_64",
				CloudProvider: "aws",
				Region:        "us-east-1",
				InstanceType:  "c6i.large",
				ImageID:       "ami-0abc1234",
			},
			{
				Role:          "arm64",
				CloudProvider: "aws",
				Region:        "us-east-1",
				InstanceType:  "c7g.large",
				ImageID:       "ami-0def5678",
			},
		},
	}
}

func TestValidateInfraManifest(t *testing.T) {
	im := validInfraManifestFixture()
	if err := replay.ValidateInfraManifest(im); err != nil {
		t.Fatalf("valid manifest failed: %v", err)
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
				im.SchemaVersion = "infra-manifest.v2"
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
			name: "invalid role",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts[0].Role = "mips64"
			},
			want: "role must be x86_64 or arm64",
		},
		{
			name: "duplicate role",
			mutate: func(im *replay.InfraManifest) {
				im.Hosts[1].Role = "x86_64"
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
	if loaded.Hosts[0].Role != "x86_64" {
		t.Fatalf("expected host[0].role=x86_64, got %q", loaded.Hosts[0].Role)
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
  "hosts": [{"role":"x86_64","cloud_provider":"aws","region":"us-east-1","instance_type":"c6i.large","image_id":"ami-0abc"}],
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
