package learning

// P4 tests: cross-process write reliability. Real independent child processes
// write the same disposable database; the parent verifies durability and
// read-after-write visibility. Also checks bounded BUSY retry stats.

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

func TestP4CrossProcessWriters(t *testing.T) {
	for _, procs := range []int{2, 4, 8} {
		procs := procs
		t.Run(fmt.Sprintf("writers=%d", procs), func(t *testing.T) {
			dbPath := filepath.Join(supTempDir(t), "memory.db")
			const perProc = 50
			var wg sync.WaitGroup
			var mu sync.Mutex
			failures := 0
			for p := 0; p < procs; p++ {
				wg.Add(1)
				go func(p int) {
					defer wg.Done()
					cmd := exec.Command(os.Args[0], "-test.run=^TestP4WriterChild$")
					cmd.Env = append(os.Environ(),
						"P4_DB="+dbPath, "P4_PROC="+strconv.Itoa(p), "P4_COUNT="+strconv.Itoa(perProc))
					if out, err := cmd.CombinedOutput(); err != nil {
						mu.Lock()
						failures++
						mu.Unlock()
						t.Errorf("child %d failed: %v\n%s", p, err, out)
					}
				}(p)
			}
			wg.Wait()
			if failures > 0 {
				t.Fatalf("%d children failed", failures)
			}
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
			want := procs * perProc
			if count != want {
				t.Fatalf("durable rows = %d, want %d (lost %d)", count, want, want-count)
			}
			var integrity string
			_ = db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity)
			if integrity != "ok" {
				t.Fatalf("integrity_check = %q", integrity)
			}
		})
	}
}

func TestP4WriterChild(t *testing.T) {
	dbPath := os.Getenv("P4_DB")
	if dbPath == "" {
		t.Skip("child only")
	}
	proc, _ := strconv.Atoi(os.Getenv("P4_PROC"))
	count, _ := strconv.Atoi(os.Getenv("P4_COUNT"))
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

// TestP4ReadAfterWriteAcrossProcesses: one process writes, another observes it.
func TestP4ReadAfterWriteAcrossProcesses(t *testing.T) {
	dbPath := filepath.Join(supTempDir(t), "memory.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestP4WriterChild$")
	cmd.Env = append(os.Environ(), "P4_DB="+dbPath, "P4_PROC=0", "P4_COUNT=20")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("writer child: %v\n%s", err, out)
	}
	s := Open(dbPath)
	defer supClose(s)
	got, ok, err := s.GetMemory("p0-m7")
	if err != nil || !ok || got.Project.ID != "p" {
		t.Fatalf("read-after-write across processes: ok=%v err=%v", ok, err)
	}
}

// TestP4BoundedRetryStats confirms the retry counters are wired and bounded.
func TestP4BoundedRetryStats(t *testing.T) {
	s := Open(filepath.Join(supTempDir(t), "memory.db"))
	defer supClose(s)
	if err := s.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "m", CandidateID: "c", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "t", Body: "b", Project: asymptoteobserve.LearningProjectV1{ID: "p"},
	}); err != nil {
		t.Fatal(err)
	}
	retries, exhausted := s.BusyRetryStats()
	if retries != 0 || exhausted != 0 {
		t.Fatalf("unexpected retry stats retries=%d exhausted=%d", retries, exhausted)
	}
}
