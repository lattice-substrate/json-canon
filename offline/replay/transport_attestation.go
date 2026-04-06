package replay

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TransportAttestationSchemaVersion is the schema identifier for replay
// transport-attestation sidecars.
const TransportAttestationSchemaVersion = "transport-attestation.v1"

// TransportAttestation binds one uploaded evidence file to a challenge and the
// worker-observed instance identity material for that replay.
type TransportAttestation struct {
	SchemaVersion      string `json:"schema_version"`
	Challenge          string `json:"challenge"`
	NodeID             string `json:"node_id"`
	ReplayIndex        int    `json:"replay_index"`
	EvidenceSHA256     string `json:"evidence_sha256"`
	IIDDocument        string `json:"iid_document"`
	IIDSignature       string `json:"iid_signature"`
	IIDPKCS7           string `json:"iid_pkcs7"`
	IIDDocumentSHA256  string `json:"iid_document_sha256"`
	IIDSignatureSHA256 string `json:"iid_signature_sha256"`
	IIDPKCS7SHA256     string `json:"iid_pkcs7_sha256"`
	IIDTrustRootSetID  string `json:"iid_trust_root_set_id"`
	PublicKey          string `json:"public_key"`
	Signature          string `json:"signature"`
}

// WriteTransportAttestation writes a transport-attestation artifact to disk.
func WriteTransportAttestation(path string, a *TransportAttestation) error {
	if a == nil {
		return fmt.Errorf("transport attestation is nil")
	}
	if err := requireGovernedSchemaVersion("transport attestation", a.SchemaVersion); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal transport attestation: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write transport attestation: %w", err)
	}
	return nil
}

// LoadTransportAttestation loads and validates a transport-attestation
// artifact from disk.
func LoadTransportAttestation(path string) (*TransportAttestation, error) {
	//nolint:gosec // REQ:OFFLINE-EVIDENCE-001 transport attestation paths are explicit runtime artifacts.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read transport attestation: %w", err)
	}
	var a TransportAttestation
	if err := decodeStrictJSONBytes("transport attestation", data, &a); err != nil {
		return nil, err
	}
	if err := ValidateTransportAttestation(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

// ValidateTransportAttestation enforces the transport-attestation field
// contract before signature verification.
//
//nolint:gocyclo,cyclop // REQ:OFFLINE-EVIDENCE-001 field-by-field validation requires explicit branches for each attestation field.
func ValidateTransportAttestation(a *TransportAttestation) error {
	if a == nil {
		return fmt.Errorf("transport attestation is nil")
	}
	if err := requireGovernedSchemaVersion("transport attestation", a.SchemaVersion); err != nil {
		return err
	}
	if a.SchemaVersion != TransportAttestationSchemaVersion {
		return fmt.Errorf("transport attestation schema_version %q is not the expected version %q", a.SchemaVersion, TransportAttestationSchemaVersion)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"challenge", a.Challenge},
		{"node_id", a.NodeID},
		{"iid_document", a.IIDDocument},
		{"iid_signature", a.IIDSignature},
		{"iid_pkcs7", a.IIDPKCS7},
		{"public_key", a.PublicKey},
		{"signature", a.Signature},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("transport attestation %s is required", field.name)
		}
	}
	if a.ReplayIndex < 1 {
		return fmt.Errorf("transport attestation replay_index must be >=1")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"evidence_sha256", a.EvidenceSHA256},
		{"iid_document_sha256", a.IIDDocumentSHA256},
		{"iid_signature_sha256", a.IIDSignatureSHA256},
		{"iid_pkcs7_sha256", a.IIDPKCS7SHA256},
	} {
		if err := validateSHA256Token(field.name, field.value); err != nil {
			return err
		}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"public_key", a.PublicKey},
		{"signature", a.Signature},
		{"iid_signature", a.IIDSignature},
		{"iid_pkcs7", a.IIDPKCS7},
	} {
		if _, err := base64.StdEncoding.DecodeString(field.value); err != nil {
			return fmt.Errorf("transport attestation %s must be valid base64: %w", field.name, err)
		}
	}
	if a.IIDTrustRootSetID != IIDTrustRootSetIDDefault {
		return fmt.Errorf("transport attestation iid_trust_root_set_id must be %q", IIDTrustRootSetIDDefault)
	}
	return nil
}

// TransportAttestationSigningPayload returns the canonical signed payload for a
// transport attestation.
func TransportAttestationSigningPayload(a *TransportAttestation) string {
	return strings.Join([]string{
		strings.TrimSpace(a.Challenge),
		strings.TrimSpace(a.NodeID),
		fmt.Sprintf("%d", a.ReplayIndex),
		strings.TrimSpace(a.EvidenceSHA256),
		strings.TrimSpace(a.IIDDocumentSHA256),
		strings.TrimSpace(a.IIDSignatureSHA256),
		strings.TrimSpace(a.IIDPKCS7SHA256),
	}, "\n")
}
