package learning

// P5 tests: protected Ed25519 approval signing and provenance enforcement.

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p5Authority(t *testing.T) (*SigningAuthority, *TrustRegistry) {
	t.Helper()
	keyPath := filepath.Join(supTempDir(t), "approval.key")
	if err := GenerateSigningKey(keyPath, "alice", "k1"); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	a, err := LoadSigningAuthority(keyPath, "alice", "k1")
	if err != nil {
		t.Fatalf("load key: %v", err)
	}
	reg := NewTrustRegistry()
	a.RegisterPublicKey(reg)
	return a, reg
}

func TestP5ValidSignedApproval(t *testing.T) {
	a, reg := p5Authority(t)
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	s.SetSigningAuthority(a)
	s.SetTrustRegistry(reg)
	defer supClose(s)

	_ = s.PutCandidate(supCandidate("c1"))
	_, m, err := ApproveCandidate(s, "c1", "ok")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got := s.AuthenticateMemory(m); got.Status != ProvenanceVerified {
		t.Fatalf("status = %s, want verified", got.Status)
	}
	// The attestation tag is persisted.
	r := Open(dbPath)
	r.SetTrustRegistry(reg)
	reloaded, ok, _ := r.GetMemory(m.ID)
	if !ok {
		t.Fatal("reloaded memory missing")
	}
	if _, has := AttestationTag(reloaded); !has {
		t.Fatal("stored memory has no attestation tag")
	}
	if r.AuthenticateMemory(reloaded).Status != ProvenanceVerified {
		t.Fatal("reloaded memory did not verify")
	}
}

func TestP5TamperInvalidates(t *testing.T) {
	a, reg := p5Authority(t)
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	s.SetSigningAuthority(a)
	s.SetTrustRegistry(reg)
	defer supClose(s)
	_ = s.PutCandidate(supCandidate("c1"))
	_, m, _ := ApproveCandidate(s, "c1", "ok")

	body := m
	body.Body = "tampered body"
	if got := s.AuthenticateMemory(body); got.Status != ProvenanceInvalid {
		t.Fatalf("tampered body status = %s, want invalid", got.Status)
	}
	proj := m
	proj.Project = asymptoteobserve.LearningProjectV1{ID: "other"}
	if got := s.AuthenticateMemory(proj); got.Status != ProvenanceInvalid {
		t.Fatalf("changed project status = %s, want invalid", got.Status)
	}
}

func TestP5WrongKeyInvalid(t *testing.T) {
	a, _ := p5Authority(t)
	other, otherReg := p5Authority(t)
	_ = other
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	s.SetSigningAuthority(a)
	s.SetTrustRegistry(otherReg) // registry trusts a different key
	defer supClose(s)
	_ = s.PutCandidate(supCandidate("c1"))
	_, m, _ := ApproveCandidate(s, "c1", "ok")
	if got := s.AuthenticateMemory(m); got.Status == ProvenanceVerified {
		t.Fatalf("signed by an untrusted key but verified")
	}
}

func TestP5ReplayOnAnotherMemoryInvalid(t *testing.T) {
	a, reg := p5Authority(t)
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	s.SetSigningAuthority(a)
	s.SetTrustRegistry(reg)
	defer supClose(s)
	_ = s.PutCandidate(supCandidate("c1"))
	_, m, _ := ApproveCandidate(s, "c1", "ok")

	replay := m
	replay.ID = "different-id"
	if got := s.AuthenticateMemory(replay); got.Status != ProvenanceInvalid {
		t.Fatalf("replayed signature status = %s, want invalid", got.Status)
	}
}

func TestP5LegacyUnsignedUnverified(t *testing.T) {
	_, reg := p5Authority(t)
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	s.SetTrustRegistry(reg)
	defer supClose(s)
	_ = s.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "legacy", CandidateID: "c", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "t", Body: "b", Project: asymptoteobserve.LearningProjectV1{ID: "p"}})
	got, _, _ := s.GetMemory("legacy")
	if st := s.AuthenticateMemory(got).Status; st != ProvenanceUnverified {
		t.Fatalf("legacy unsigned status = %s, want unverified", st)
	}
}

func TestP5KeyFileErrors(t *testing.T) {
	dir := supTempDir(t)
	if _, err := LoadSigningAuthority(filepath.Join(dir, "missing.key"), "alice", "k1"); err == nil {
		t.Fatal("missing key must error")
	}
	bad := filepath.Join(dir, "bad.key")
	_ = os.WriteFile(bad, []byte("not-a-key\n"), 0600)
	if _, err := LoadSigningAuthority(bad, "alice", "k1"); err == nil {
		t.Fatal("corrupted key must error")
	}
	if runtime.GOOS != "windows" {
		loose := filepath.Join(dir, "loose.key")
		_ = GenerateSigningKey(loose, "alice", "k1")
		_ = os.Chmod(loose, 0644)
		if _, err := LoadSigningAuthority(loose, "alice", "k1"); err == nil {
			t.Fatal("group/world-readable key must be rejected")
		}
	}
}

func TestP5UnauthorizedCallerCannotGetSignedApproval(t *testing.T) {
	a, reg := p5Authority(t)
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	seed := Open(dbPath)
	_ = seed.PutCandidate(p3Candidate("c-b", "project-B"))
	supClose(seed)

	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	s.SetSigningAuthority(a)
	s.SetTrustRegistry(reg)
	defer supClose(s)
	if _, _, err := ApproveCandidate(s, "c-b", "x"); err == nil {
		t.Fatal("unauthorized approval must be denied")
	}
	// No memory was written, so no signed approval exists.
	db, _ := s.db()
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&n)
	if n != 0 {
		t.Fatalf("unauthorized approval wrote %d memories", n)
	}
}

func TestP5ConcurrentSignedApprovals(t *testing.T) {
	a, reg := p5Authority(t)
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	s.SetSigningAuthority(a)
	s.SetTrustRegistry(reg)
	defer supClose(s)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		id := "c" + string(rune('a'+i))
		_ = s.PutCandidate(supCandidate(id))
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, m, err := ApproveCandidate(s, id, "ok")
			if err == nil && s.AuthenticateMemory(m).Status != ProvenanceVerified {
				t.Errorf("concurrent signed approval did not verify")
			}
		}(id)
	}
	wg.Wait()
}
