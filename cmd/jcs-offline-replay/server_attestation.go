package main

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"strings"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

type awsInstanceIdentityDocument struct {
	AvailabilityZone string `json:"availabilityZone"`
	ImageID          string `json:"imageId"`
	InstanceID       string `json:"instanceId"`
	Region           string `json:"region"`
}

var awsInstanceIdentityRSACertByRegion = map[string]string{
	"us-east-1": `-----BEGIN CERTIFICATE-----
MIIDITCCAoqgAwIBAgIUE1y2NIKCU+Rg4uu4u32koG9QEYIwDQYJKoZIhvcNAQEL
BQAwXDELMAkGA1UEBhMCVVMxGTAXBgNVBAgTEFdhc2hpbmd0b24gU3RhdGUxEDAO
BgNVBAcTB1NlYXR0bGUxIDAeBgNVBAoTF0FtYXpvbiBXZWIgU2VydmljZXMgTExD
MB4XDTI0MDQyOTE3MzQwMVoXDTI5MDQyODE3MzQwMVowXDELMAkGA1UEBhMCVVMx
GTAXBgNVBAgTEFdhc2hpbmd0b24gU3RhdGUxEDAOBgNVBAcTB1NlYXR0bGUxIDAe
BgNVBAoTF0FtYXpvbiBXZWIgU2VydmljZXMgTExDMIGfMA0GCSqGSIb3DQEBAQUA
A4GNADCBiQKBgQCHvRjf/0kStpJ248khtIaN8qkDN3tkw4VjvA9nvPl2anJO+eIB
UqPfQG09kZlwpWpmyO8bGB2RWqWxCwuB/dcnIob6w420k9WY5C0IIGtDRNauN3ku
vGXkw3HEnF0EjYr0pcyWUvByWY4KswZV42X7Y7XSS13hOIcL6NLA+H94/QIDAQAB
o4HfMIHcMAsGA1UdDwQEAwIHgDAdBgNVHQ4EFgQUJdbMCBXKtvCcWdwUUizvtUF2
UTgwgZkGA1UdIwSBkTCBjoAUJdbMCBXKtvCcWdwUUizvtUF2UTihYKReMFwxCzAJ
BgNVBAYTAlVTMRkwFwYDVQQIExBXYXNoaW5ndG9uIFN0YXRlMRAwDgYDVQQHEwdT
ZWF0dGxlMSAwHgYDVQQKExdBbWF6b24gV2ViIFNlcnZpY2VzIExMQ4IUE1y2NIKC
U+Rg4uu4u32koG9QEYIwEgYDVR0TAQH/BAgwBgEB/wIBADANBgkqhkiG9w0BAQsF
AAOBgQAlxSmwcWnhT4uAeSinJuz+1BTcKhVSWb5jT8pYjQb8ZoZkXXRGb09mvYeU
NeqOBr27rvRAnaQ/9LUQf72+SahDFuS4CMI8nwowytqbmwquqFr4dxA/SDADyRiF
ea1UoMuNHTY49J/1vPomqsVn7mugTp+TbjqCfOJTpu0temHcFA==
-----END CERTIFICATE-----`,
}

var (
	verifyAWSInstanceIdentityFunc    = verifyAWSInstanceIdentity
	newTransportAttestationChallenge = generateTransportAttestationChallenge
)

func strictDecodeJSON(kind string, data []byte, target any) error {
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

//nolint:gocyclo,cyclop // REQ:AWS-GATE-001 attestation verification keeps each failure mode explicit for auditability.
func verifyTransportAttestation(
	attestationData []byte,
	evidenceData []byte,
	expectedChallenge string,
	nodeID string,
	replayIndex int,
	host provisionedHost,
	expectedRegion string,
) error {
	var attestation replay.TransportAttestation
	if err := strictDecodeJSON("transport attestation", attestationData, &attestation); err != nil {
		return err
	}
	if err := replay.ValidateTransportAttestation(&attestation); err != nil {
		return fmt.Errorf("validate transport attestation: %w", err)
	}
	if attestation.Challenge != expectedChallenge {
		return fmt.Errorf("transport attestation challenge mismatch for %s replay %d", nodeID, replayIndex)
	}
	if attestation.NodeID != nodeID {
		return fmt.Errorf("transport attestation node_id mismatch: got=%s want=%s", attestation.NodeID, nodeID)
	}
	if attestation.ReplayIndex != replayIndex {
		return fmt.Errorf("transport attestation replay_index mismatch: got=%d want=%d", attestation.ReplayIndex, replayIndex)
	}
	if got := sha256HexString(string(evidenceData)); got != attestation.EvidenceSHA256 {
		return fmt.Errorf("transport attestation evidence sha256 mismatch for %s replay %d", nodeID, replayIndex)
	}
	if sha256HexString(attestation.IIDDocument) != attestation.IIDDocumentSHA256 {
		return fmt.Errorf("transport attestation iid document digest mismatch for %s replay %d", nodeID, replayIndex)
	}
	if sha256HexString(attestation.IIDSignature) != attestation.IIDSignatureSHA256 {
		return fmt.Errorf("transport attestation iid signature digest mismatch for %s replay %d", nodeID, replayIndex)
	}
	if sha256HexString(attestation.IIDPKCS7) != attestation.IIDPKCS7SHA256 {
		return fmt.Errorf("transport attestation iid pkcs7 digest mismatch for %s replay %d", nodeID, replayIndex)
	}
	publicKeyBytes, err := base64.StdEncoding.DecodeString(attestation.PublicKey)
	if err != nil {
		return fmt.Errorf("decode transport attestation public key: %w", err)
	}
	signatureBytes, err := base64.StdEncoding.DecodeString(attestation.Signature)
	if err != nil {
		return fmt.Errorf("decode transport attestation signature: %w", err)
	}
	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("transport attestation public key must be %d bytes", ed25519.PublicKeySize)
	}
	if !ed25519.Verify(
		ed25519.PublicKey(publicKeyBytes),
		[]byte(replay.TransportAttestationSigningPayload(&attestation)),
		signatureBytes,
	) {
		return fmt.Errorf("transport attestation signature verification failed for %s replay %d", nodeID, replayIndex)
	}
	if _, err := verifyAWSInstanceIdentityFunc(attestation.IIDDocument, attestation.IIDSignature, host, expectedRegion); err != nil {
		return err
	}
	return nil
}

//nolint:gocyclo,cyclop // REQ:AWS-GATE-001 instance identity verification keeps each trust-boundary check explicit.
func verifyAWSInstanceIdentity(rawDocument string, rawSignature string, host provisionedHost, expectedRegion string) (*awsInstanceIdentityDocument, error) {
	var document awsInstanceIdentityDocument
	if err := json.Unmarshal([]byte(rawDocument), &document); err != nil {
		return nil, fmt.Errorf("decode instance identity document for %s: %w", host.HostID, err)
	}
	if strings.TrimSpace(document.InstanceID) != strings.TrimSpace(host.InstanceID) {
		return nil, fmt.Errorf("instance identity document instance_id mismatch for %s: got=%s want=%s", host.HostID, document.InstanceID, host.InstanceID)
	}
	if strings.TrimSpace(document.ImageID) != strings.TrimSpace(host.ImageID) {
		return nil, fmt.Errorf("instance identity document image_id mismatch for %s: got=%s want=%s", host.HostID, document.ImageID, host.ImageID)
	}
	if expected := strings.TrimSpace(expectedRegion); expected != "" && strings.TrimSpace(document.Region) != expected {
		return nil, fmt.Errorf("instance identity document region mismatch for %s: got=%s want=%s", host.HostID, document.Region, expected)
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rawSignature))
	if err != nil {
		return nil, fmt.Errorf("decode instance identity signature for %s: %w", host.HostID, err)
	}
	certPEM, ok := awsInstanceIdentityRSACertByRegion[strings.TrimSpace(document.Region)]
	if !ok {
		return nil, fmt.Errorf("unsupported aws instance identity certificate region %q", document.Region)
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("decode instance identity certificate for region %s", document.Region)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse instance identity certificate for region %s: %w", document.Region, err)
	}
	publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("instance identity certificate for region %s is not rsa", document.Region)
	}
	digest := sha256HexBytes([]byte(rawDocument))
	digestBytes, err := hex.DecodeString(digest)
	if err != nil {
		return nil, fmt.Errorf("decode instance identity document digest: %w", err)
	}
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digestBytes, signature); err != nil {
		return nil, fmt.Errorf("verify instance identity signature for %s: %w", host.HostID, err)
	}
	return &document, nil
}

func sha256HexBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func generateTransportAttestationChallenge() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate transport attestation challenge: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
