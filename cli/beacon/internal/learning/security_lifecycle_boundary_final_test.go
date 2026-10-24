package learning

// Final-optimization regression tests.
//
// S4 resolution: the raw Open() store is the trusted-embedding boundary (the
// local operator owns the database), so lifecycle mutations succeed there by
// design. An OpenAuthorized store with an allowlist denies cross-project
// mutations and allows same-project ones. No MCP tool exposes approve/reject/
// supersede, so the raw-store behavior is not an externally reachable bypass.
// These tests pin that boundary so a future change cannot silently remove the
// authorization check from the protected path.

import (
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func finalCandidate(project, id string) asymptoteobserve.LearningCandidateV1 {
	return asymptoteobserve.LearningCandidateV1{
		ID:      id,
		State:   asymptoteobserve.LearningCandidateStateCandidate,
		Kind:    asymptoteobserve.LearningMemoryKindConvention,
		Title:   "t " + id,
		Body:    "b " + id,
		Project: asymptoteobserve.LearningProjectV1{ID: project},
	}
}

func TestFinalLifecycleTrustBoundary(t *testing.T) {
	// Trusted embedding (no authorizer): mutations are allowed by design.
	trusted := Open(filepath.Join(t.TempDir(), "trusted.db"))
	defer trusted.Close()
	if err := trusted.PutCandidate(finalCandidate("p", "c-trusted")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApproveCandidate(trusted, "c-trusted", "x"); err != nil {
		t.Fatalf("trusted embedding must allow approval: %v", err)
	}

	// Authorized store: same-project allowed, foreign project denied.
	authorized := OpenAuthorized(filepath.Join(t.TempDir(), "auth.db"), NewProjectAllowlist("p"))
	defer authorized.Close()

	if err := authorized.PutCandidate(finalCandidate("p", "c-own")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApproveCandidate(authorized, "c-own", "x"); err != nil {
		t.Fatalf("authorized same-project approval must succeed: %v", err)
	}

	if err := authorized.PutCandidate(finalCandidate("q", "c-foreign")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApproveCandidate(authorized, "c-foreign", "x"); err == nil {
		t.Fatal("authorized store must deny cross-project approval")
	}
	if _, err := RejectCandidate(authorized, "c-foreign", "x"); err == nil {
		t.Fatal("authorized store must deny cross-project rejection")
	}
}

func TestFinalAuthorizedSupersessionIsAtomicAndGated(t *testing.T) {
	store := OpenAuthorized(filepath.Join(t.TempDir(), "auth.db"), NewProjectAllowlist("p"))
	defer store.Close()

	if err := store.PutCandidate(finalCandidate("p", "c-old")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApproveCandidate(store, "c-old", "x"); err != nil {
		t.Fatal(err)
	}
	repl := asymptoteobserve.LearningMemoryV1{
		ID: "repl", CandidateID: "c-repl", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "repl", Body: "b", Project: asymptoteobserve.LearningProjectV1{ID: "p"},
	}
	if err := store.PutMemory(repl); err != nil {
		t.Fatal(err)
	}
	if err := store.PutCandidate(finalCandidate("p", "c-new")); err != nil {
		t.Fatal(err)
	}
	_, newMem, err := ApproveCandidate(store, "c-new", "x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SupersedeCandidate(store, "c-new", "repl", "outdated"); err != nil {
		t.Fatalf("authorized same-project supersession must succeed: %v", err)
	}
	got, ok, err := store.GetMemory(newMem.ID)
	if err != nil || !ok {
		t.Fatalf("get superseded memory: ok=%v err=%v", ok, err)
	}
	if got.SupersededBy != "repl" {
		t.Fatalf("superseded_by=%q want %q", got.SupersededBy, "repl")
	}
}
