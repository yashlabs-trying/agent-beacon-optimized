package learning

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// TestPutMemoryCrashHelper runs only in the child process. os.Exit bypasses all
// defers and closes the SQLite process while its write is at the named phase.
func TestPutMemoryCrashHelper(t *testing.T) {
	if os.Getenv("BEACON_CRASH_HELPER") != "1" {
		return
	}
	store := Open(os.Getenv("BEACON_CRASH_DB"))
	store.writeCheckpoint = func(phase string) {
		if phase == os.Getenv("BEACON_CRASH_PHASE") {
			os.Exit(77)
		}
	}
	if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "fault-memory", CandidateID: "fault-candidate",
		Kind:  asymptoteobserve.LearningMemoryKindConvention,
		Title: "fault token", Body: "faulttoken survives only a committed write",
		Project: asymptoteobserve.LearningProjectV1{ID: "phase1-project"},
	}); err != nil {
		t.Fatal(err)
	}
	t.Fatal("the requested crash checkpoint was not reached")
}

func TestPutMemoryProcessCrashRecovery(t *testing.T) {
	for _, phase := range []string{
		"after-begin", "after-authoritative", "after-index",
		"after-generation", "before-commit", "after-commit",
	} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "memory.db")
			store := Open(path)
			if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
				ID: "baseline", CandidateID: "baseline-candidate",
				Kind:  asymptoteobserve.LearningMemoryKindConvention,
				Title: "baseline", Body: "baseline memory for index readiness",
				Project: asymptoteobserve.LearningProjectV1{ID: "phase1-project"},
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := store.ListMemories(Query{ProjectID: "phase1-project", Q: "baseline", Limit: 5}); err != nil {
				t.Fatal(err)
			}
			before := crashGeneration(t, path)

			cmd := exec.Command(os.Args[0], "-test.run=^TestPutMemoryCrashHelper$")
			cmd.Env = append(os.Environ(), "BEACON_CRASH_HELPER=1", "BEACON_CRASH_DB="+path, "BEACON_CRASH_PHASE="+phase)
			output, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 77 {
				t.Fatalf("child at %s: exit=%v output=%s", phase, err, output)
			}

			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var integrity string
			if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
				t.Fatalf("integrity after %s: %q, %v", phase, integrity, err)
			}
			var rowCount, indexCount int
			if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE id = 'fault-memory'`).Scan(&rowCount); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memory_id = 'fault-memory'`).Scan(&indexCount); err != nil {
				t.Fatal(err)
			}
			want := 0
			if phase == "after-commit" {
				want = 1
			}
			if rowCount != want || indexCount != want {
				t.Fatalf("after %s: authoritative=%d index=%d, want %d", phase, rowCount, indexCount, want)
			}
			generation := crashGeneration(t, path)
			if generation != before+int64(want) {
				t.Fatalf("after %s: generation=%d, want %d", phase, generation, before+int64(want))
			}
			found, err := Open(path).ListMemories(Query{ProjectID: "phase1-project", Q: "faulttoken", Limit: 5})
			if err != nil || len(found) != want {
				t.Fatalf("search after %s: count=%d, want %d, error=%v", phase, len(found), want, err)
			}
		})
	}
}

func crashGeneration(t *testing.T, path string) int64 {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw string
	if err := db.QueryRow(`SELECT value FROM memory_index_meta WHERE key = 'generation'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	generation, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
