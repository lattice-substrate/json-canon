package replay_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func TestGovernedRegistryCompleteness(t *testing.T) {
	types := replay.GovernedMediaTypes()
	if len(types) == 0 {
		t.Fatal("governed artifact type registry is empty")
	}
	for _, at := range types {
		if at.SchemaVersion == "" {
			t.Error("governed artifact type has empty SchemaVersion")
		}
		if at.MediaType == "" {
			t.Errorf("governed artifact type %q has empty MediaType", at.SchemaVersion)
		}
		if at.SchemaID == "" {
			t.Errorf("governed artifact type %q has empty SchemaID", at.SchemaVersion)
		}
		if !strings.HasPrefix(at.MediaType, "application/vnd.jcs.") {
			t.Errorf("governed artifact type %q media type %q does not start with application/vnd.jcs.", at.SchemaVersion, at.MediaType)
		}
		if !strings.HasSuffix(at.MediaType, "+json") {
			t.Errorf("governed artifact type %q media type %q does not end with +json", at.SchemaVersion, at.MediaType)
		}
		if !strings.HasPrefix(at.SchemaID, "https://lattice-substrate.github.io/jcs/schemas/") {
			t.Errorf("governed artifact type %q schema ID %q does not start with canonical schema namespace", at.SchemaVersion, at.SchemaID)
		}
	}
}

func TestGovernedRegistryBijection(t *testing.T) {
	types := replay.GovernedMediaTypes()
	seenSV := make(map[string]bool, len(types))
	seenMT := make(map[string]bool, len(types))
	seenSID := make(map[string]bool, len(types))
	for _, at := range types {
		if seenSV[at.SchemaVersion] {
			t.Errorf("duplicate schema_version in registry: %q", at.SchemaVersion)
		}
		seenSV[at.SchemaVersion] = true
		if seenMT[at.MediaType] {
			t.Errorf("duplicate media type in registry: %q", at.MediaType)
		}
		seenMT[at.MediaType] = true
		if seenSID[at.SchemaID] {
			t.Errorf("duplicate schema ID in registry: %q", at.SchemaID)
		}
		seenSID[at.SchemaID] = true
	}
}

func TestResolveKnownSchemaVersions(t *testing.T) {
	tests := []struct {
		schemaVersion string
		wantMediaType string
		wantSchemaID  string
	}{
		{replay.EvidenceSchemaVersion, replay.MediaTypeEvidenceStatement, replay.SchemaIDEvidenceStatement},
		{replay.InfraManifestSchemaVersion, replay.MediaTypeInfraManifest, replay.SchemaIDInfraManifest},
		{replay.TransportAttestationSchemaVersion, replay.MediaTypeTransportAttestation, replay.SchemaIDTransportAttestation},
	}
	for _, tt := range tests {
		at, ok := replay.ResolveSchemaVersion(tt.schemaVersion)
		if !ok {
			t.Errorf("ResolveSchemaVersion(%q) returned false", tt.schemaVersion)
			continue
		}
		if at.MediaType != tt.wantMediaType {
			t.Errorf("ResolveSchemaVersion(%q).MediaType = %q, want %q", tt.schemaVersion, at.MediaType, tt.wantMediaType)
		}
		if at.SchemaID != tt.wantSchemaID {
			t.Errorf("ResolveSchemaVersion(%q).SchemaID = %q, want %q", tt.schemaVersion, at.SchemaID, tt.wantSchemaID)
		}
	}
}

func TestResolveUnknownSchemaVersionRejected(t *testing.T) {
	unknowns := []string{
		"",
		"unknown.v1",
		"evidence.v99",
		"infra-manifest.v2",
		"transport-attestation.v0",
		" evidence.v1",
		"evidence.v1 ",
	}
	for _, sv := range unknowns {
		if _, ok := replay.ResolveSchemaVersion(sv); ok {
			t.Errorf("ResolveSchemaVersion(%q) returned true for unknown schema_version", sv)
		}
	}
}

func TestGovernedConstantsMatchExpected(t *testing.T) {
	// Exact string parity with jcs-spec/registries/media-types.json and
	// jcs-spec/registries/schema-registry.json. If a constant is changed,
	// this test catches it.
	checks := []struct {
		name     string
		got      string
		expected string
	}{
		{"MediaTypeEvidenceStatement", replay.MediaTypeEvidenceStatement, "application/vnd.jcs.evidence.statement.v1+json"},
		{"MediaTypeInfraManifest", replay.MediaTypeInfraManifest, "application/vnd.jcs.infra.manifest.v1+json"},
		{"MediaTypeTransportAttestation", replay.MediaTypeTransportAttestation, "application/vnd.jcs.transport.attestation.v1+json"},
		{"SchemaIDEvidenceStatement", replay.SchemaIDEvidenceStatement, "https://lattice-substrate.github.io/jcs/schemas/evidence.statement.v1"},
		{"SchemaIDInfraManifest", replay.SchemaIDInfraManifest, "https://lattice-substrate.github.io/jcs/schemas/infra.manifest.v1"},
		{"SchemaIDTransportAttestation", replay.SchemaIDTransportAttestation, "https://lattice-substrate.github.io/jcs/schemas/transport.attestation.v1"},
	}
	for _, c := range checks {
		if c.got != c.expected {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.expected)
		}
	}
}

func TestSchemaVersionConstantsInRegistry(t *testing.T) {
	// Every schema_version constant used by Load/Write/Validate functions
	// must be wired into the governed registry.
	constants := []string{
		replay.EvidenceSchemaVersion,
		replay.InfraManifestSchemaVersion,
		replay.TransportAttestationSchemaVersion,
	}
	for _, sv := range constants {
		if _, ok := replay.ResolveSchemaVersion(sv); !ok {
			t.Errorf("schema_version constant %q is not in the governed registry", sv)
		}
	}
}

func TestGovernedMediaTypesReturnsCopy(t *testing.T) {
	a := replay.GovernedMediaTypes()
	b := replay.GovernedMediaTypes()
	if len(a) != len(b) {
		t.Fatalf("GovernedMediaTypes() returned different lengths: %d vs %d", len(a), len(b))
	}
	// Mutating the returned slice must not affect the registry.
	a[0].SchemaVersion = "mutated"
	c := replay.GovernedMediaTypes()
	if c[0].SchemaVersion == "mutated" {
		t.Error("GovernedMediaTypes() returned a reference to the internal slice, not a copy")
	}
}

func TestLoadEvidenceRejectsUnknownSchemaVersion(t *testing.T) {
	// Fail-closed: LoadEvidence rejects documents with unknown schema_version.
	// This tests requireGovernedSchemaVersion indirectly through the public API.
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	doc := `{"schema_version":"evidence.v99","bundle_sha256":"` + strings.Repeat("a", 64) + `"}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := replay.LoadEvidence(path)
	if err == nil {
		t.Fatal("LoadEvidence accepted unknown schema_version — fail-closed violation")
	}
	if !strings.Contains(err.Error(), "JCS-REQ-0223") {
		t.Errorf("LoadEvidence error does not cite JCS-REQ-0223: %v", err)
	}
	if !strings.Contains(err.Error(), "unknown schema_version") {
		t.Errorf("LoadEvidence error does not contain 'unknown schema_version': %v", err)
	}
}

func TestWriteEvidenceRejectsUnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	e := &replay.EvidenceBundle{SchemaVersion: "evidence.v99"}
	err := replay.WriteEvidence(path, e)
	if err == nil {
		t.Fatal("WriteEvidence accepted unknown schema_version — fail-closed violation")
	}
	if !strings.Contains(err.Error(), "JCS-REQ-0223") {
		t.Errorf("WriteEvidence error does not cite JCS-REQ-0223: %v", err)
	}
}

func TestWriteTransportAttestationRejectsUnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	a := &replay.TransportAttestation{SchemaVersion: "transport-attestation.v99"}
	err := replay.WriteTransportAttestation(path, a)
	if err == nil {
		t.Fatal("WriteTransportAttestation accepted unknown schema_version — fail-closed violation")
	}
	if !strings.Contains(err.Error(), "JCS-REQ-0223") {
		t.Errorf("WriteTransportAttestation error does not cite JCS-REQ-0223: %v", err)
	}
}

func TestResolveSchemaVersionExactMatchOnly(t *testing.T) {
	// Evidence-grade: no trimming, no lossy transformation. The
	// schema_version is compared byte-for-byte. Whitespace-padded
	// values are unknown, not "close enough".
	padded := []string{
		" " + replay.EvidenceSchemaVersion,
		replay.EvidenceSchemaVersion + " ",
		"\t" + replay.EvidenceSchemaVersion,
		replay.EvidenceSchemaVersion + "\n",
	}
	for _, sv := range padded {
		if _, ok := replay.ResolveSchemaVersion(sv); ok {
			t.Errorf("ResolveSchemaVersion(%q) accepted whitespace-padded schema_version", sv)
		}
	}
}
