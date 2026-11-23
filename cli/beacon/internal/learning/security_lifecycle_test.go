package learning

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// Finding B: approval and supersession must be atomic. These tests prove the
// transition commits all-or-nothing (crash injection) and that concurrent
// reviewers cannot both transition the same candidate.

func seedReviewCandidate(t *testing.T, dbPath, id string) asymptoteobserve.LearningCandidateV1 {
	t.Helper()
	store := Open(dbPath)
	defer store.Close()
	candidate := asymptoteobserve.LearningCandidateV1{
		ID:      id,
		State:   asymptoteobserve.LearningCandidateStateCandidate,
		Kind:    asymptoteobserve.LearningMemoryKindConvention,
		Title:   "Crash candidate",
		Body:    "Body for the crash candidate.",
		Project: asymptoteobserve.LearningProjectV1{ID: "proj"},
	}
	if err := store.PutCandidate(candidate); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	return candidate
}

func candidateStateAndMemoryRows(t *testing.T, dbPath, candidateID string) (string, int) {
	t.Helper()
	db := openRaw(t, dbPath)
	defer db.Close()
	var state string
	if err := db.QueryRow(`SELECT state FROM candidates WHERE id = ?`, candidateID).Scan(&state); err != nil {
		t.Fatalf("read candidate state: %v", err)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE candidate_id = ?`, candidateID).Scan(&rows); err != nil {
		t.Fatalf("count memories: %v", err)
	}
	return state, rows
}

// TestApprovalCrashAtomicity kills the approval process at each write stage and
// checks that the pair (candidate state, memory row) is never half-applied. Before
// commit, neither is visible; after commit, both are.
func TestApprovalCrashAtomicity(t *testing.T) {
	dbPath := filepath.Join(scratchDirTB(t), "memory.db")
	candidate := seedReviewCandidate(t, dbPath, "candidate-crash")

	for _, stage := range []string{"tx_begin_pre", "tx_begin_post", "memory_insert", "fts_update", "generation_update", "pre_commit"} {
		runApprovalCrashHelper(t, dbPath, candidate.ID, stage)
		state, rows := candidateStateAndMemoryRows(t, dbPath, candidate.ID)
		if state != asymptoteobserve.LearningCandidateStateCandidate {
			t.Fatalf("stage %s: candidate state = %q, want candidate (pre-commit crash must not transition)", stage, state)
		}
		if rows != 0 {
			t.Fatalf("stage %s: %d memory rows visible after pre-commit crash, want 0", stage, rows)
		}
	}

	runApprovalCrashHelper(t, dbPath, candidate.ID, "post_commit")
	state, rows := candidateStateAndMemoryRows(t, dbPath, candidate.ID)
	if state != asymptoteobserve.LearningCandidateStateApproved {
		t.Fatalf("post-commit: candidate state = %q, want approved", state)
	}
	if rows != 1 {
		t.Fatalf("post-commit: memory rows = %d, want 1", rows)
	}
}

func runApprovalCrashHelper(t *testing.T, dbPath, candidateID, stage string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestApprovalCrashHelper$")
	cmd.Env = append(os.Environ(),
		"BEACON_CRASH_STAGE="+stage,
		"BEACON_CRASH_DB="+dbPath,
		"BEACON_CRASH_CANDIDATE="+candidateID,
	)
	err := cmd.Run()
	if stage == "post_commit" {
		// The crash point is after commit, so the transition is already durable.
		// Whether the helper exits 0 or 2, the parent verifies the committed state.
		_ = err
		return
	}
	if err == nil {
		t.Fatalf("stage %s: helper did not crash as expected", stage)
	}
}

// TestApprovalCrashHelper is the subprocess body. It only acts when driven by the
// parent test through the environment.
func TestApprovalCrashHelper(t *testing.T) {
	stage := os.Getenv("BEACON_CRASH_STAGE")
	dbPath := os.Getenv("BEACON_CRASH_DB")
	candidateID := os.Getenv("BEACON_CRASH_CANDIDATE")
	if stage == "" || dbPath == "" {
		t.Skip("crash helper runs only as a subprocess")
	}
	faultHook = func(s string) {
		if s == stage {
			os.Exit(2)
		}
	}
	store := Open(dbPath)
	if _, _, err := ApproveCandidate(store, candidateID, "crash-helper"); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestConcurrentApprovalSingleWinner(t *testing.T) {
	dbPath := filepath.Join(scratchDirTB(t), "memory.db")
	candidate := seedReviewCandidate(t, dbPath, "candidate-race")
	store := Open(dbPath)
	defer store.Close()

	var successes int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := ApproveCandidate(store, candidate.ID, "race"); err == nil {
				atomic.AddInt32(&successes, 1)
			}
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("approval successes = %d, want exactly 1", successes)
	}
	state, rows := candidateStateAndMemoryRows(t, dbPath, candidate.ID)
	if state != asymptoteobserve.LearningCandidateStateApproved || rows != 1 {
		t.Fatalf("final state = %q, rows = %d; want approved and 1", state, rows)
	}
}

func TestRejectedCandidateCannotBeApproved(t *testing.T) {
	dbPath := filepath.Join(scratchDirTB(t), "memory.db")
	candidate := seedReviewCandidate(t, dbPath, "candidate-reject")
	store := Open(dbPath)
	defer store.Close()

	if _, err := RejectCandidate(store, candidate.ID, "no"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, _, err := ApproveCandidate(store, candidate.ID, "late"); err == nil {
		t.Fatal("approving a rejected candidate must fail")
	}
	state, rows := candidateStateAndMemoryRows(t, dbPath, candidate.ID)
	if state != asymptoteobserve.LearningCandidateStateRejected || rows != 0 {
		t.Fatalf("state = %q, rows = %d; want rejected and 0", state, rows)
	}
}
