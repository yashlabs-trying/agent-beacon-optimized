package learning

// P0 regression tests: supersession of an approved candidate (RG-1), plus
// duplicate/after-rejection/concurrent/interrupted supersession.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func supTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "beacon-sup-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func supCandidate(id string) asymptoteobserve.LearningCandidateV1 {
	return asymptoteobserve.LearningCandidateV1{
		ID:      id,
		State:   asymptoteobserve.LearningCandidateStateCandidate,
		Kind:    asymptoteobserve.LearningMemoryKindConvention,
		Title:   "sup candidate " + id,
		Body:    "sup body for " + id,
		Project: asymptoteobserve.LearningProjectV1{ID: "proj"},
	}
}

func supClose(*Store) {}

func supSeedApproved(t *testing.T, dbPath, candidateID string) (asymptoteobserve.LearningCandidateV1, asymptoteobserve.LearningMemoryV1) {
	t.Helper()
	store := Open(dbPath)
	defer supClose(store)
	if err := store.PutCandidate(supCandidate(candidateID)); err != nil {
		t.Fatal(err)
	}
	c, m, err := ApproveCandidate(store, candidateID, "seed")
	if err != nil {
		t.Fatalf("seed approve: %v", err)
	}
	return c, m
}

func supReplacement(t *testing.T, dbPath, id string) asymptoteobserve.LearningMemoryV1 {
	t.Helper()
	store := Open(dbPath)
	defer supClose(store)
	repl := asymptoteobserve.LearningMemoryV1{ID: id, CandidateID: "c-" + id,
		Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "repl", Body: "b",
		Project: asymptoteobserve.LearningProjectV1{ID: "proj"}}
	if err := store.PutMemory(repl); err != nil {
		t.Fatal(err)
	}
	return repl
}

// TestSupersedeApprovedCandidateWritesTombstone is the RG-1 regression: an
// approved candidate must be supersedable and its memory tombstoned.
func TestSupersedeApprovedCandidateWritesTombstone(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	_, m := supSeedApproved(t, dbPath, "c1")
	repl := supReplacement(t, dbPath, "repl")
	store := Open(dbPath)
	defer supClose(store)

	c, err := SupersedeCandidate(store, "c1", repl.ID, "replaced")
	if err != nil {
		t.Fatalf("supersede approved candidate: %v", err)
	}
	if c.State != asymptoteobserve.LearningCandidateStateSuperseded || c.SupersededBy != repl.ID {
		t.Fatalf("candidate state=%q supersededBy=%q", c.State, c.SupersededBy)
	}
	got, ok, err := store.GetMemory(m.ID)
	if err != nil || !ok {
		t.Fatalf("get memory: ok=%v err=%v", ok, err)
	}
	if got.SupersededBy != repl.ID {
		t.Fatalf("memory tombstone superseded_by=%q want %q", got.SupersededBy, repl.ID)
	}
}

// TestSupersedeCandidateOnlyStillWorks covers the original (unapproved) path.
func TestSupersedeCandidateOnlyStillWorks(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	store := Open(dbPath)
	defer supClose(store)
	if err := store.PutCandidate(supCandidate("c-only")); err != nil {
		t.Fatal(err)
	}
	repl := supReplacement(t, dbPath, "repl2")
	c, err := SupersedeCandidate(store, "c-only", repl.ID, "x")
	if err != nil || c.State != asymptoteobserve.LearningCandidateStateSuperseded {
		t.Fatalf("supersede candidate-only: err=%v state=%q", err, c.State)
	}
}

func TestSupersedeDuplicateFails(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	supSeedApproved(t, dbPath, "c1")
	repl := supReplacement(t, dbPath, "repl")
	store := Open(dbPath)
	defer supClose(store)
	if _, err := SupersedeCandidate(store, "c1", repl.ID, "first"); err != nil {
		t.Fatalf("first supersede: %v", err)
	}
	if _, err := SupersedeCandidate(store, "c1", repl.ID, "second"); err == nil {
		t.Fatal("duplicate supersession must fail")
	}
	// no duplicate memory rows
	db, _ := store.db()
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM memories WHERE candidate_id = 'c1'`).Scan(&n)
	if n != 1 {
		t.Fatalf("memory rows for candidate = %d, want 1", n)
	}
}

func TestSupersedeAfterRejectionFails(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	store := Open(dbPath)
	defer supClose(store)
	_ = store.PutCandidate(supCandidate("c1"))
	if _, err := RejectCandidate(store, "c1", "no"); err != nil {
		t.Fatal(err)
	}
	repl := supReplacement(t, dbPath, "repl")
	if _, err := SupersedeCandidate(store, "c1", repl.ID, "x"); err == nil {
		t.Fatal("superseding a rejected candidate must fail")
	}
}

func TestConcurrentSupersessionSingleWinner(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	supSeedApproved(t, dbPath, "c1")
	repl := supReplacement(t, dbPath, "repl")
	store := Open(dbPath)
	defer supClose(store)

	var success int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := SupersedeCandidate(store, "c1", repl.ID, "race"); err == nil {
				atomic.AddInt32(&success, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if success != 1 {
		t.Fatalf("supersede successes = %d, want exactly 1", success)
	}
}

func TestConcurrentApprovalAndSupersessionConsistent(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	store := Open(dbPath)
	defer supClose(store)
	_ = store.PutCandidate(supCandidate("c1"))
	repl := supReplacement(t, dbPath, "repl")

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			<-start
			if k%2 == 0 {
				_, _, _ = ApproveCandidate(store, "c1", "a")
			} else {
				_, _ = SupersedeCandidate(store, "c1", repl.ID, "s")
			}
		}(i)
	}
	close(start)
	wg.Wait()

	c, ok, err := store.GetCandidate("c1")
	if err != nil || !ok {
		t.Fatalf("get candidate: ok=%v err=%v", ok, err)
	}
	// Final state must be self-consistent: either approved (memory exists, not
	// tombstoned) or superseded (memory, if any, tombstoned).
	db, _ := store.db()
	var rows int
	_ = db.QueryRow(`SELECT COUNT(*) FROM memories WHERE candidate_id = 'c1'`).Scan(&rows)
	switch c.State {
	case asymptoteobserve.LearningCandidateStateApproved:
		if rows != 1 {
			t.Fatalf("approved but memory rows=%d", rows)
		}
		got, _, _ := store.GetMemory(c.MemoryID)
		if got.SupersededBy != "" {
			t.Fatalf("approved memory is tombstoned")
		}
	case asymptoteobserve.LearningCandidateStateSuperseded:
		// any memory for the candidate must be tombstoned
		var unTombstoned int
		_ = db.QueryRow(`SELECT COUNT(*) FROM memories WHERE candidate_id = 'c1' AND (superseded_by IS NULL OR superseded_by = '')`).Scan(&unTombstoned)
		if unTombstoned != 0 {
			t.Fatalf("superseded but %d active memory rows", unTombstoned)
		}
	default:
		t.Fatalf("unexpected final state %q", c.State)
	}
}

// TestSupersessionCrashAtomicity kills a supersession at pre_commit and checks
// the pair (candidate state, memory tombstone) is never half-applied.
func TestSupersessionCrashAtomicity(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	_, m := supSeedApproved(t, dbPath, "c1")
	repl := supReplacement(t, dbPath, "repl")

	cmd := exec.Command(os.Args[0], "-test.run=^TestSupersessionCrashHelper$")
	cmd.Env = append(os.Environ(), "BENCH_SUP_DB="+dbPath, "BENCH_SUP_REPL="+repl.ID, "BENCH_SUP_STAGE=pre_commit")
	if err := cmd.Run(); err == nil {
		t.Fatal("helper did not crash")
	}

	store := Open(dbPath)
	defer supClose(store)
	c, ok, err := store.GetCandidate("c1")
	if err != nil || !ok {
		t.Fatalf("get candidate: ok=%v err=%v", ok, err)
	}
	got, _, _ := store.GetMemory(m.ID)
	// Invariant: candidate superseded iff memory tombstoned.
	candSup := c.State == asymptoteobserve.LearningCandidateStateSuperseded
	memSup := got.SupersededBy != ""
	if candSup != memSup {
		t.Fatalf("inconsistent after crash: candidate_superseded=%v memory_tombstoned=%v", candSup, memSup)
	}
}

func TestSupersessionCrashHelper(t *testing.T) {
	dbPath := os.Getenv("BENCH_SUP_DB")
	if dbPath == "" {
		t.Skip("child only")
	}
	stage := os.Getenv("BENCH_SUP_STAGE")
	faultHook = func(s string) {
		if s == stage {
			os.Exit(2)
		}
	}
	store := Open(dbPath)
	_, _ = SupersedeCandidate(store, "c1", os.Getenv("BENCH_SUP_REPL"), "crash")
	os.Exit(0)
}

// TestSupersessionRecoveryPersists reopens the store after a successful
// supersession and confirms the tombstone survived.
func TestSupersessionRecoveryPersists(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	_, m := supSeedApproved(t, dbPath, "c1")
	repl := supReplacement(t, dbPath, "repl")
	store := Open(dbPath)
	if _, err := SupersedeCandidate(store, "c1", repl.ID, "x"); err != nil {
		t.Fatal(err)
	}
	supClose(store)

	reopened := Open(dbPath)
	defer supClose(reopened)
	got, ok, err := reopened.GetMemory(m.ID)
	if err != nil || !ok || got.SupersededBy != repl.ID {
		t.Fatalf("tombstone did not persist: ok=%v err=%v supersededBy=%q", ok, err, got.SupersededBy)
	}
}
