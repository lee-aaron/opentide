// Package security provides cryptographic utilities for skill signing,
// manifest verification, and audit hashing.
package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/opentide/opentide/pkg/skillspec"
)

// KeyPair holds an Ed25519 signing key pair.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateKeyPair creates a new Ed25519 key pair for skill signing.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}
	return &KeyPair{PublicKey: pub, PrivateKey: priv}, nil
}

// PublicKeyHex returns the hex-encoded public key.
func (kp *KeyPair) PublicKeyHex() string {
	return hex.EncodeToString(kp.PublicKey)
}

// PrivateKeyHex returns the hex-encoded private key (seed + public).
func (kp *KeyPair) PrivateKeyHex() string {
	return hex.EncodeToString(kp.PrivateKey)
}

// LoadPublicKey decodes a hex-encoded Ed25519 public key.
func LoadPublicKey(hexKey string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex public key: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length: got %d, want %d", len(b), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}

// LoadPrivateKey decodes a hex-encoded Ed25519 private key.
func LoadPrivateKey(hexKey string) (ed25519.PrivateKey, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex private key: %w", err)
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: got %d, want %d", len(b), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(b), nil
}

// SignManifest signs a skill manifest and returns a SignedManifest.
func SignManifest(manifest *skillspec.Manifest, privateKey ed25519.PrivateKey) (*skillspec.SignedManifest, error) {
	data, err := manifest.MarshalYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	sig := ed25519.Sign(privateKey, data)
	pubKey := privateKey.Public().(ed25519.PublicKey)

	return &skillspec.SignedManifest{
		Manifest: *manifest,
		Signature: skillspec.Signature{
			PublicKey: hex.EncodeToString(pubKey),
			Signature: hex.EncodeToString(sig),
			SignedAt:  time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

// VerifyManifest checks that a signed manifest's signature is valid.
func VerifyManifest(signed *skillspec.SignedManifest) error {
	pubKey, err := LoadPublicKey(signed.Signature.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key in signature: %w", err)
	}

	sigBytes, err := hex.DecodeString(signed.Signature.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}

	data, err := signed.Manifest.MarshalYAML()
	if err != nil {
		return fmt.Errorf("failed to marshal manifest for verification: %w", err)
	}

	if !ed25519.Verify(pubKey, data, sigBytes) {
		return fmt.Errorf("signature verification failed: manifest may have been tampered with")
	}

	return nil
}

// VerifyManifestWithKey checks a signed manifest against a specific trusted public key.
// This is used when the registry has a known set of trusted author keys.
func VerifyManifestWithKey(signed *skillspec.SignedManifest, trustedKey ed25519.PublicKey) error {
	// First check that the embedded key matches the trusted key
	embeddedKey, err := LoadPublicKey(signed.Signature.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key in signature: %w", err)
	}

	if !embeddedKey.Equal(trustedKey) {
		return fmt.Errorf("signature key does not match trusted key")
	}

	return VerifyManifest(signed)
}
