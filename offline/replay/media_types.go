package replay

import "fmt"

// Governed media types from jcs-spec Chapter 13 registries (JCS-REQ-0218).
// These are the artifact identity strings for cross-repo interchange.
// The canonical source is jcs-spec/registries/media-types.json; these
// compile-time constants are the implementation's black-box recognition
// of that namespace. Cross-repo parity is enforced by jcs-integration-tests.
const (
	MediaTypeEvidenceStatement    = "application/vnd.jcs.evidence.statement.v1+json"
	MediaTypeInfraManifest        = "application/vnd.jcs.infra.manifest.v1+json"
	MediaTypeTransportAttestation = "application/vnd.jcs.transport.attestation.v1+json"
)

// Governed schema IDs from jcs-spec Chapter 13 registries (JCS-REQ-0216).
// These are the canonical schema identifier URIs. The canonical source is
// jcs-spec/registries/schema-registry.json.
const (
	SchemaIDEvidenceStatement    = "https://lattice-substrate.github.io/jcs/schemas/evidence.statement.v1"
	SchemaIDInfraManifest        = "https://lattice-substrate.github.io/jcs/schemas/infra.manifest.v1"
	SchemaIDTransportAttestation = "https://lattice-substrate.github.io/jcs/schemas/transport.attestation.v1"
)

// GovernedArtifactType maps a document-internal schema_version to its
// governing media type and canonical schema ID from jcs-spec Chapter 13
// registries. This three-layer identity is the OCI-style contract boundary
// between repositories.
type GovernedArtifactType struct {
	SchemaVersion string
	MediaType     string
	SchemaID      string
}

// governedArtifactTypes is the compile-time registry of artifact types that
// json-canon produces or consumes as governed cross-repo artifacts.
// Non-governed internal types (toolchain-lock.v1, server-run.v1,
// aws-release-hosts.v1) are intentionally excluded — they validate their
// own schema_version directly and have no entry in jcs-spec's registries.
var governedArtifactTypes = []GovernedArtifactType{
	{EvidenceSchemaVersion, MediaTypeEvidenceStatement, SchemaIDEvidenceStatement},
	{InfraManifestSchemaVersion, MediaTypeInfraManifest, SchemaIDInfraManifest},
	{TransportAttestationSchemaVersion, MediaTypeTransportAttestation, SchemaIDTransportAttestation},
}

// ResolveSchemaVersion returns the governed artifact type for a
// schema_version, or false if the schema_version is not in the governed
// registry.
func ResolveSchemaVersion(schemaVersion string) (GovernedArtifactType, bool) {
	for _, t := range governedArtifactTypes {
		if t.SchemaVersion == schemaVersion {
			return t, true
		}
	}
	return GovernedArtifactType{}, false
}

// GovernedMediaTypes returns the full list of governed artifact types
// recognized by this implementation.
func GovernedMediaTypes() []GovernedArtifactType {
	out := make([]GovernedArtifactType, len(governedArtifactTypes))
	copy(out, governedArtifactTypes)
	return out
}

// requireGovernedSchemaVersion validates that a schema_version is in the
// governed media type registry. Returns an error citing JCS-REQ-0223 if
// the schema_version is unknown. This function is called at every Load and
// Write boundary for governed artifacts — fail-closed, no exceptions.
func requireGovernedSchemaVersion(kind, schemaVersion string) error {
	if _, ok := ResolveSchemaVersion(schemaVersion); !ok {
		return fmt.Errorf("%s: unknown schema_version %q is not in the governed media type registry (JCS-REQ-0223)", kind, schemaVersion)
	}
	return nil
}
