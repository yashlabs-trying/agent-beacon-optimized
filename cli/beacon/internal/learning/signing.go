package learning

// Protected approval signing. The signing authority holds an Ed25519 private key
// supplied by the operator (never derived from a request) and is used to attest
// approved memory content. This is the missing production half of the
// provenance mechanism in provenance.go: SignAttestation/Authenticate already
// exist; this file makes a real key usable and wires signing into approval.
//
// Key material on disk is always protected: on Windows through DPAPI
// (current-user scope), on POSIX through owner-only file permissions. The
// OS-specific encode/decode/permission checks live in keyfile_windows.go and
// keyfile_posix.go. There is no plaintext fallback.

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// SigningAuthority is a loaded approval signing key.
type SigningAuthority struct {
	approver string
	keyID    string
	priv     ed25519.PrivateKey
}

// Approver and KeyID identify the signing authority in an attestation.
func (a *SigningAuthority) Approver() string { return a.approver }
func (a *SigningAuthority) KeyID() string    { return a.keyID }

// PublicKey returns the verification key to provision into a TrustRegistry.
func (a *SigningAuthority) PublicKey() ed25519.PublicKey {
	return a.priv.Public().(ed25519.PublicKey)
}

// Sign returns the reserved attestation tag for a memory under this authority.
func (a *SigningAuthority) Sign(m asymptoteobserve.LearningMemoryV1) string {
	return SignAttestation(m, a.approver, a.keyID, a.priv)
}

// GenerateSigningKey writes a fresh Ed25519 private key to path, protected by
// the platform mechanism. It refuses to overwrite an existing file and never
// writes plaintext key material.
func GenerateSigningKey(path, approver, keyID string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("signing key already exists: %s", path)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	blob, err := encodeKeyBlob(priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// LoadSigningAuthority reads and unprotects a private key file. A decryption
// failure is an error: the key is never regenerated or replaced, and there is no
// plaintext fallback.
func LoadSigningAuthority(path, approver, keyID string) (*SigningAuthority, error) {
	if strings.TrimSpace(approver) == "" || strings.TrimSpace(keyID) == "" {
		return nil, fmt.Errorf("signing authority requires an approver and key id")
	}
	if err := checkKeyFilePerms(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := decodeKeyBlob(data)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("signing key: expected %d bytes, got %d", ed25519.PrivateKeySize, len(raw))
	}
	return &SigningAuthority{approver: approver, keyID: keyID, priv: ed25519.PrivateKey(raw)}, nil
}

// RegisterPublicKey provisions this authority's public key into a registry.
func (a *SigningAuthority) RegisterPublicKey(reg *TrustRegistry) {
	reg.Add(a.approver, a.keyID, a.PublicKey())
}
