//go:build windows

package learning

// Native Windows DPAPI key-protection tests. These run only on Windows and
// exercise CryptProtectData/CryptUnprotectData through the signing API.

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func TestWindowsKeyIsDPAPIProtected(t *testing.T) {
	keyPath := filepath.Join(supTempDir(t), "approval.key")
	if err := GenerateSigningKey(keyPath, "alice", "k1"); err != nil {
		t.Fatalf("generate: %v", err)
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	// The on-disk blob must not be plaintext base64 of a 64-byte key.
	if raw, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data))); derr == nil && len(raw) == 64 {
		t.Fatal("key file looks like plaintext base64, not DPAPI ciphertext")
	}
	a, err := LoadSigningAuthority(keyPath, "alice", "k1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	reg := NewTrustRegistry()
	a.RegisterPublicKey(reg)
	m := asymptoteobserve.LearningMemoryV1{ID: "m", CandidateID: "c",
		Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "t", Body: "b",
		Project: asymptoteobserve.LearningProjectV1{ID: "p"}}
	m.Tags = []string{a.Sign(m)}
	if Authenticate(m, reg).Status != ProvenanceVerified {
		t.Fatal("DPAPI-protected key did not produce a verifiable signature")
	}
}

func TestWindowsTamperedKeyRejected(t *testing.T) {
	keyPath := filepath.Join(supTempDir(t), "approval.key")
	if err := GenerateSigningKey(keyPath, "alice", "k1"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(keyPath)
	data[len(data)/2] ^= 0xFF
	if err := os.WriteFile(keyPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSigningAuthority(keyPath, "alice", "k1"); err == nil {
		t.Fatal("tampered DPAPI blob must fail to decrypt")
	}
}

func TestWindowsRestartPersistence(t *testing.T) {
	keyPath := filepath.Join(supTempDir(t), "approval.key")
	if err := GenerateSigningKey(keyPath, "alice", "k1"); err != nil {
		t.Fatal(err)
	}
	// Simulate a restart: load again in a fresh call and verify the same key.
	a1, err := LoadSigningAuthority(keyPath, "alice", "k1")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := LoadSigningAuthority(keyPath, "alice", "k1")
	if err != nil {
		t.Fatal(err)
	}
	if string(a1.PublicKey()) != string(a2.PublicKey()) {
		t.Fatal("public key changed across loads")
	}
}

func TestWindowsNoPlaintextFallbackOnCorrupt(t *testing.T) {
	keyPath := filepath.Join(supTempDir(t), "approval.key")
	if err := os.WriteFile(keyPath, []byte("not a dpapi blob\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSigningAuthority(keyPath, "alice", "k1"); err == nil {
		t.Fatal("corrupt key must error, never fall back")
	}
}
