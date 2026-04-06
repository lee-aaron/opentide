package security

import (
	"crypto/ed25519"
	"testing"

	"github.com/opentide/opentide/pkg/skillspec"
)

func testManifest() *skillspec.Manifest {
	return &skillspec.Manifest{
		Name:        "test-skill",
		Version:     "0.1.0",
		Description: "A test skill for signing",
		Author:      "test-author",
		Security: skillspec.Security{
			Egress:     []string{"api.example.com:443"},
			Filesystem: "read-only",
		},
		Triggers: skillspec.Triggers{
			ToolName: "test_tool",
		},
		Runtime: skillspec.Runtime{
			Image: "test:latest",
		},
	}
}

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("public key length = %d, want %d", len(kp.PublicKey), ed25519.PublicKeySize)
	}
	if len(kp.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("private key length = %d, want %d", len(kp.PrivateKey), ed25519.PrivateKeySize)
	}

	// Hex encoding round-trip
	pubHex := kp.PublicKeyHex()
	pub, err := LoadPublicKey(pubHex)
	if err != nil {
		t.Fatalf("LoadPublicKey failed: %v", err)
	}
	if !pub.Equal(kp.PublicKey) {
		t.Error("public key round-trip failed")
	}

	privHex := kp.PrivateKeyHex()
	priv, err := LoadPrivateKey(privHex)
	if err != nil {
		t.Fatalf("LoadPrivateKey failed: %v", err)
	}
	if !priv.Equal(kp.PrivateKey) {
		t.Error("private key round-trip failed")
	}
}

func TestSignAndVerifyManifest(t *testing.T) {
	kp, _ := GenerateKeyPair()
	m := testManifest()

	signed, err := SignManifest(m, kp.PrivateKey)
	if err != nil {
		t.Fatalf("SignManifest failed: %v", err)
	}

	if signed.Signature.PublicKey == "" {
		t.Error("signature missing public key")
	}
	if signed.Signature.Signature == "" {
		t.Error("signature missing signature")
	}
	if signed.Signature.SignedAt == "" {
		t.Error("signature missing timestamp")
	}

	// Verify should succeed
	if err := VerifyManifest(signed); err != nil {
		t.Fatalf("VerifyManifest failed: %v", err)
	}
}

func TestVerifyManifestTamperedContent(t *testing.T) {
	kp, _ := GenerateKeyPair()
	m := testManifest()

	signed, _ := SignManifest(m, kp.PrivateKey)

	// Tamper with the manifest after signing
	signed.Manifest.Name = "evil-skill"

	err := VerifyManifest(signed)
	if err == nil {
		t.Fatal("expected verification failure for tampered manifest")
	}
}

func TestVerifyManifestWrongKey(t *testing.T) {
	kp1, _ := GenerateKeyPair()
	kp2, _ := GenerateKeyPair()
	m := testManifest()

	signed, _ := SignManifest(m, kp1.PrivateKey)

	// Verify with a different trusted key should fail
	err := VerifyManifestWithKey(signed, kp2.PublicKey)
	if err == nil {
		t.Fatal("expected verification failure with wrong trusted key")
	}
}

func TestVerifyManifestWithCorrectKey(t *testing.T) {
	kp, _ := GenerateKeyPair()
	m := testManifest()

	signed, _ := SignManifest(m, kp.PrivateKey)

	if err := VerifyManifestWithKey(signed, kp.PublicKey); err != nil {
		t.Fatalf("VerifyManifestWithKey failed: %v", err)
	}
}

func TestLoadPublicKeyInvalid(t *testing.T) {
	_, err := LoadPublicKey("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}

	_, err = LoadPublicKey("abcd")
	if err == nil {
		t.Fatal("expected error for wrong length")
	}
}

func TestLoadPrivateKeyInvalid(t *testing.T) {
	_, err := LoadPrivateKey("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}

	_, err = LoadPrivateKey("abcd")
	if err == nil {
		t.Fatal("expected error for wrong length")
	}
}
