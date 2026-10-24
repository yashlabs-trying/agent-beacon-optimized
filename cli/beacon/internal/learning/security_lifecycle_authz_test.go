package learning

// P3 tests: store-level lifecycle authorization. A store opened with a project
// authorizer refuses lifecycle mutations and direct writes outside its scope;
// the same operations inside the scope succeed.

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p3Candidate(id, project string) asymptoteobserve.LearningCandidateV1 {
	c := supCandidate(id)
	c.Project = asymptoteobserve.LearningProjectV1{ID: project}
	return c
}

func p3Mem(id, project string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "t", Body: "b", Project: asymptoteobserve.LearningProjectV1{ID: project},
	}
}

func TestP3ForeignApprovalDenied(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	seed := Open(dbPath)
	_ = seed.PutCandidate(p3Candidate("c-b", "project-B"))
	supClose(seed)

	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	defer supClose(s)
	_, _, err := ApproveCandidate(s, "c-b", "x")
	if !errors.Is(err, ErrProjectNotAuthorized) {
		t.Fatalf("foreign approval err = %v, want ErrProjectNotAuthorized", err)
	}
	got, _, _ := Open(dbPath).GetCandidate("c-b")
	if got.State != asymptoteobserve.LearningCandidateStateCandidate {
		t.Fatalf("candidate state changed to %q despite denial", got.State)
	}
}

func TestP3ForeignRejectionDenied(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	seed := Open(dbPath)
	_ = seed.PutCandidate(p3Candidate("c-b", "project-B"))
	supClose(seed)

	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	defer supClose(s)
	if _, err := RejectCandidate(s, "c-b", "x"); !errors.Is(err, ErrProjectNotAuthorized) {
		t.Fatalf("foreign rejection err = %v, want ErrProjectNotAuthorized", err)
	}
}

func TestP3ForeignSupersessionDenied(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	seed := Open(dbPath)
	_ = seed.PutCandidate(p3Candidate("c-b", "project-B"))
	_, m, err := ApproveCandidate(seed, "c-b", "seed")
	if err != nil {
		t.Fatal(err)
	}
	_ = seed.PutMemory(p3Mem("repl", "project-B"))
	supClose(seed)

	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	defer supClose(s)
	if _, err := SupersedeCandidate(s, "c-b", "repl", "x"); !errors.Is(err, ErrProjectNotAuthorized) {
		t.Fatalf("foreign supersession err = %v, want ErrProjectNotAuthorized", err)
	}
	// memory must remain active
	got, _, _ := Open(dbPath).GetMemory(m.ID)
	if got.SupersededBy != "" {
		t.Fatalf("foreign supersession tombstoned the memory")
	}
}

func TestP3ForeignDirectWriteDenied(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	defer supClose(s)
	if err := s.PutMemory(p3Mem("m-b", "project-B")); !errors.Is(err, ErrProjectNotAuthorized) {
		t.Fatalf("foreign PutMemory err = %v, want ErrProjectNotAuthorized", err)
	}
}

func TestP3SameProjectOperationsSucceed(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	defer supClose(s)

	// direct write in scope
	if err := s.PutMemory(p3Mem("m-a", "project-A")); err != nil {
		t.Fatalf("in-scope PutMemory: %v", err)
	}
	// approval in scope
	if err := s.PutCandidate(p3Candidate("c-a", "project-A")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApproveCandidate(s, "c-a", "ok"); err != nil {
		t.Fatalf("in-scope approval: %v", err)
	}
	// rejection in scope
	_ = s.PutCandidate(p3Candidate("c-a2", "project-A"))
	if _, err := RejectCandidate(s, "c-a2", "no"); err != nil {
		t.Fatalf("in-scope rejection: %v", err)
	}
	// supersession in scope
	_ = s.PutCandidate(p3Candidate("c-a3", "project-A"))
	if _, _, err := ApproveCandidate(s, "c-a3", "ok"); err != nil {
		t.Fatal(err)
	}
	_ = s.PutMemory(p3Mem("repl-a", "project-A"))
	if _, err := SupersedeCandidate(s, "c-a3", "repl-a", "x"); err != nil {
		t.Fatalf("in-scope supersession: %v", err)
	}
}

func TestP3RepeatedAuthorizedTransition(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
	defer supClose(s)
	_ = s.PutCandidate(p3Candidate("c-a", "project-A"))
	if _, _, err := ApproveCandidate(s, "c-a", "ok"); err != nil {
		t.Fatal(err)
	}
	// second approval is not an authorization failure; it is a state failure
	_, _, err := ApproveCandidate(s, "c-a", "again")
	if err == nil || errors.Is(err, ErrProjectNotAuthorized) {
		t.Fatalf("repeated approval err = %v, want non-authz state error", err)
	}
}

func TestP3UnauthorizedAfterRestart(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	seed := Open(dbPath)
	_ = seed.PutCandidate(p3Candidate("c-b", "project-B"))
	supClose(seed)

	for i := 0; i < 2; i++ {
		s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
		if _, _, err := ApproveCandidate(s, "c-b", "x"); !errors.Is(err, ErrProjectNotAuthorized) {
			t.Fatalf("restart %d: err = %v, want ErrProjectNotAuthorized", i, err)
		}
		supClose(s)
	}
}

// TestP3S4Matrix runs the S4 lifecycle-mutation matrix against an authorized
// store and counts denials.
func TestP3S4Matrix(t *testing.T) {
	const attempts = 10
	denied := 0
	for i := 0; i < attempts; i++ {
		dbPath := filepath.Join(supTempDir(t), "memory.db")
		cid := fmt.Sprintf("c-b-%d", i)
		seed := Open(dbPath)
		_ = seed.PutCandidate(p3Candidate(cid, "project-B"))
		_, _, _ = ApproveCandidate(seed, cid, "seed")
		_ = seed.PutMemory(p3Mem(fmt.Sprintf("repl-%d", i), "project-B"))
		supClose(seed)

		s := OpenAuthorized(dbPath, NewProjectAllowlist("project-A"))
		if _, err := SupersedeCandidate(s, cid, fmt.Sprintf("repl-%d", i), "x"); errors.Is(err, ErrProjectNotAuthorized) {
			denied++
		}
		supClose(s)
	}
	t.Logf("P3_S4 attempts=%d denied=%d unauthorized_successes=%d", attempts, denied, attempts-denied)
	if denied != attempts {
		t.Fatalf("denied %d/%d", denied, attempts)
	}
}

func TestP3TrustedEmbeddingAllows(t *testing.T) {
	// Documented trust model: a store with no authorizer is the trusted operator
	// embedding (the CLI). It allows any project.
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	s := Open(dbPath)
	defer supClose(s)
	if err := s.PutMemory(p3Mem("m-any", "any-project")); err != nil {
		t.Fatalf("trusted embedding write: %v", err)
	}
}
