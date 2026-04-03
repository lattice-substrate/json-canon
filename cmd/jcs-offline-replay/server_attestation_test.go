package main

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

var testCertificateNotBefore = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
var testCertificateNotAfter = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

func mustTestTransportAttestationData(t *testing.T, evidence []byte, challenge, nodeID string, replayIndex int) []byte {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	attestation := replay.TransportAttestation{
		SchemaVersion:  replay.TransportAttestationSchemaVersion,
		Challenge:      challenge,
		NodeID:         nodeID,
		ReplayIndex:    replayIndex,
		EvidenceSHA256: sha256HexString(string(evidence)),
		IIDDocument:    `{"availabilityZone":"us-east-1a","imageId":"ami-123","instanceId":"i-123","region":"us-east-1"}`,
		IIDSignature:   base64.StdEncoding.EncodeToString([]byte("signature")),
		IIDPKCS7:       base64.StdEncoding.EncodeToString([]byte("pkcs7")),
		PublicKey:      base64.StdEncoding.EncodeToString(publicKey),
	}
	attestation.IIDDocumentSHA256 = sha256HexString(attestation.IIDDocument)
	attestation.IIDSignatureSHA256 = sha256HexString(attestation.IIDSignature)
	attestation.IIDPKCS7SHA256 = sha256HexString(attestation.IIDPKCS7)
	attestation.Signature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(privateKey, []byte(replay.TransportAttestationSigningPayload(&attestation))),
	)
	data, err := json.Marshal(attestation)
	if err != nil {
		t.Fatalf("marshal attestation: %v", err)
	}
	return data
}

func TestVerifyTransportAttestation(t *testing.T) {
	oldVerify := verifyAWSInstanceIdentityFunc
	t.Cleanup(func() {
		verifyAWSInstanceIdentityFunc = oldVerify
	})

	verifyAWSInstanceIdentityFunc = func(rawDocument string, rawSignature string, host provisionedHost, expectedRegion string) (*awsInstanceIdentityDocument, error) {
		return &awsInstanceIdentityDocument{Region: "us-east-1", InstanceID: host.InstanceID, ImageID: host.ImageID}, nil
	}

	evidence := []byte(`{"node_id":"aws-native-ubuntu","mode":"vm","distro":"ubuntu","kernel_family":"ga","replay_index":1,"session_id":"s","started_at_utc":"2026-01-01T00:00:00Z","completed_at_utc":"2026-01-01T00:00:01Z","case_count":1,"passed":true,"canonical_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","verify_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","failure_class_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","exit_code_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","discovered_cpu":"cpu","discovered_kernel":"kernel","measured_architecture":"x86_64","measured_os_id":"ubuntu","measured_os_version_id":"24.04","measured_kernel":"kernel","measured_cpu":"cpu","aws_instance_id":"i-123","aws_image_id":"ami-123"}`)
	attestation := mustTestTransportAttestationData(t, evidence, strings.Repeat("c", 64), "aws-native-ubuntu", 1)

	if err := verifyTransportAttestation(attestation, evidence, strings.Repeat("c", 64), "aws-native-ubuntu", 1, provisionedHost{
		HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123",
	}, "us-east-1"); err != nil {
		t.Fatalf("verifyTransportAttestation: %v", err)
	}

	if err := verifyTransportAttestation(attestation, evidence, strings.Repeat("d", 64), "aws-native-ubuntu", 1, provisionedHost{
		HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123",
	}, "us-east-1"); err == nil || !strings.Contains(err.Error(), "challenge mismatch") {
		t.Fatalf("verifyTransportAttestation mismatch error = %v", err)
	}

	var parsed replay.TransportAttestation
	if err := json.Unmarshal(attestation, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	parsed.EvidenceSHA256 = strings.Repeat("e", 64)
	tampered, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := verifyTransportAttestation(tampered, evidence, strings.Repeat("c", 64), "aws-native-ubuntu", 1, provisionedHost{
		HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123",
	}, "us-east-1"); err == nil || !strings.Contains(err.Error(), "evidence sha256 mismatch") {
		t.Fatalf("verifyTransportAttestation tamper error = %v", err)
	}

	if err := strictDecodeJSON("transport attestation", []byte(`{"ok":true}{"trailing":true}`), &parsed); err == nil {
		t.Fatal("expected strictDecodeJSON trailing error")
	}
}

func TestVerifyAWSInstanceIdentity(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-ec2-instance-identity",
		},
		NotBefore:             testCertificateNotBefore,
		NotAfter:              testCertificateNotAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	oldCerts := awsInstanceIdentityRSACertByRegion
	t.Cleanup(func() {
		awsInstanceIdentityRSACertByRegion = oldCerts
	})
	awsInstanceIdentityRSACertByRegion = map[string]string{
		"us-east-1": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
	}

	document := `{"availabilityZone":"us-east-1a","imageId":"ami-123","instanceId":"i-123","region":"us-east-1"}`
	digest := sha256.Sum256([]byte(document))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15: %v", err)
	}
	rawSignature := base64.StdEncoding.EncodeToString(signature)
	if _, verifyErr := verifyAWSInstanceIdentity(document, rawSignature, provisionedHost{
		HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123",
	}, "us-east-1"); verifyErr != nil {
		t.Fatalf("verifyAWSInstanceIdentity: %v", verifyErr)
	}
	if _, verifyErr := verifyAWSInstanceIdentity(document, rawSignature, provisionedHost{
		HostID: "aws-native-ubuntu", InstanceID: "i-wrong", ImageID: "ami-123",
	}, "us-east-1"); verifyErr == nil || !strings.Contains(verifyErr.Error(), "instance_id mismatch") {
		t.Fatalf("verifyAWSInstanceIdentity mismatch error = %v", verifyErr)
	}
	if _, verifyErr := verifyAWSInstanceIdentity(document, "!!!", provisionedHost{
		HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123",
	}, "us-east-1"); verifyErr == nil || !strings.Contains(verifyErr.Error(), "decode instance identity signature") {
		t.Fatalf("verifyAWSInstanceIdentity base64 error = %v", verifyErr)
	}
	if _, verifyErr := verifyAWSInstanceIdentity(document, rawSignature, provisionedHost{
		HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123",
	}, "us-west-2"); verifyErr == nil || !strings.Contains(verifyErr.Error(), "region mismatch") {
		t.Fatalf("verifyAWSInstanceIdentity region error = %v", verifyErr)
	}
	unsupportedRegionDocument := `{"availabilityZone":"eu-west-1a","imageId":"ami-123","instanceId":"i-123","region":"eu-west-1"}`
	unsupportedDigest := sha256.Sum256([]byte(unsupportedRegionDocument))
	unsupportedSignature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, unsupportedDigest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15 unsupported region: %v", err)
	}
	if _, verifyErr := verifyAWSInstanceIdentity(unsupportedRegionDocument, base64.StdEncoding.EncodeToString(unsupportedSignature), provisionedHost{
		HostID: "aws-native-ubuntu", InstanceID: "i-123", ImageID: "ami-123",
	}, "eu-west-1"); verifyErr == nil || !strings.Contains(verifyErr.Error(), "unsupported aws instance identity certificate region") {
		t.Fatalf("verifyAWSInstanceIdentity unsupported-region error = %v", verifyErr)
	}
}
