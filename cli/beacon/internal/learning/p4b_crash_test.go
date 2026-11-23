package learning

// Batch 4 Phase B: cross-process crash safety and higher writer counts.
// Independent child processes write/approve the same disposable database; the
// parent kills a child at a real write stage and inspects durable state.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p4bWriteChild(t *testing.T, dbPath string, procs, perProc int) {
	t.Helper()
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := 0
	for p := 0; p < procs; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestP4bWriterChild$")
			cmd.Env = append(os.Environ(), "P4B_DB="+dbPath, "P4B_PROC="+strconv.Itoa(p), "P4B_COUNT="+strconv.Itoa(perProc))
			if out, err := cmd.CombinedOutput(); err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				t.Errorf("child %d: %v\n%s", p, err, out)
			}
		}(p)
	}
	wg.Wait()
	if failed > 0 {
		t.Fatalf("%d children failed", failed)
	}
}

func p4bVerifyRows(t *testing.T, dbPath string, want int) {
	t.Helper()
	s := Open(dbPath)
	defer supClose(s)
	db, err := s.db()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("durable rows = %d, want %d (lost %d)", count, want, want-count)
	}
	var integrity string
	_ = db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity)
	if integrity != "ok" {
		t.Fatalf("integrity_check = %q", integrity)
	}
}

func TestP4bWriters16(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	const procs, per = 16, 40
	p4bWriteChild(t, dbPath, procs, per)
	p4bVerifyRows(t, dbPath, procs*per)
}

func TestP4bWriters32(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	const procs, per = 32, 20
	p4bWriteChild(t, dbPath, procs, per)
	p4bVerifyRows(t, dbPath, procs*per)
}

func TestP4bWriterChild(t *testing.T) {
	dbPath := os.Getenv("P4B_DB")
	if dbPath == "" {
		t.Skip("child only")
	}
	proc, _ := strconv.Atoi(os.Getenv("P4B_PROC"))
	count, _ := strconv.Atoi(os.Getenv("P4B_COUNT"))
	s := Open(dbPath)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("p%d-m%d", proc, i)
		if err := s.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
			Title: "t", Body: "cross-process body", Project: asymptoteobserve.LearningProjectV1{ID: "p"},
		}); err != nil {
			t.Fatalf("child %d write %d: %v", proc, i, err)
		}
	}
	supClose(s)
	os.Exit(0)
}

func TestP4bCrashChild(t *testing.T) {
	stage := os.Getenv("BENCH_CRASH_STAGE")
	if stage == "" {
		t.Skip("child only")
	}
	dbPath := os.Getenv("BENCH_CRASH_DB")
	s := Open(dbPath)
	// preseed commits before the hook is armed
	if err := s.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "preseed", CandidateID: "c-preseed", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "t", Body: "preseed", Project: asymptoteobserve.LearningProjectV1{ID: "p"}}); err != nil {
		t.Fatalf("preseed: %v", err)
	}
	faultHook = func(x string) {
		if x == stage {
			os.Exit(2)
		}
	}
	_ = s.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "target", CandidateID: "c-target", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "t", Body: "target", Project: asymptoteobserve.LearningProjectV1{ID: "p"}})
	os.Exit(0)
}

func TestP4bCrashDuringWritePreCommit(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestP4bCrashChild$")
	cmd.Env = append(os.Environ(), "BENCH_CRASH_STAGE=pre_commit", "BENCH_CRASH_DB="+dbPath)
	if err := cmd.Run(); err == nil {
		t.Fatal("child did not crash")
	}
	s := Open(dbPath)
	defer supClose(s)
	db, _ := s.db()
	var integrity string
	_ = db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity)
	if integrity != "ok" {
		t.Fatalf("integrity after crash = %q", integrity)
	}
	if _, ok, _ := s.GetMemory("preseed"); !ok {
		t.Fatal("preseed write was lost")
	}
	if _, ok, _ := s.GetMemory("target"); ok {
		t.Fatal("pre-commit crash left the target committed")
	}
}

func TestP4bCrashAfterCommitDurable(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestP4bCrashChild$")
	cmd.Env = append(os.Environ(), "BENCH_CRASH_STAGE=post_commit", "BENCH_CRASH_DB="+dbPath)
	_ = cmd.Run() // exits 2 after commit
	s := Open(dbPath)
	defer supClose(s)
	if _, ok, _ := s.GetMemory("target"); !ok {
		t.Fatal("post-commit crash lost an acknowledged write")
	}
}

func TestP4bCrashApprovalLeavesNoFalseAudit(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	seed := Open(dbPath)
	_ = seed.PutCandidate(supCandidate("c-crash"))
	supClose(seed)
	cmd := exec.Command(os.Args[0], "-test.run=^TestP4bCrashApprovalChild$")
	cmd.Env = append(os.Environ(), "BENCH_CRASH_DB="+dbPath)
	if err := cmd.Run(); err == nil {
		t.Fatal("approval child did not crash")
	}
	s := Open(dbPath)
	defer supClose(s)
	got, ok, _ := s.GetCandidate("c-crash")
	if !ok || got.State != asymptoteobserve.LearningCandidateStateCandidate {
		t.Fatalf("candidate changed after pre-commit crash: state=%s", got.State)
	}
	entries, _ := s.AuditEntries()
	for _, e := range entries {
		if e.Event == "memory_approved" {
			t.Fatal("false success audit survived a rolled-back approval")
		}
	}
}

func TestP4bCrashApprovalChild(t *testing.T) {
	dbPath := os.Getenv("BENCH_CRASH_DB")
	if dbPath == "" {
		t.Skip("child only")
	}
	s := Open(dbPath)
	faultHook = func(x string) {
		if x == "pre_commit" {
			os.Exit(2)
		}
	}
	_, _, _ = ApproveCandidate(s, "c-crash", "x")
	os.Exit(0)
}
