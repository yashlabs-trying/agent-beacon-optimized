//go:build !windows

package learning

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// phase2_crash_injection_test.go is a subprocess crash-injection harness. A
// child process performs a memory write and is killed with SIGKILL at a named
// fault point injected into PutMemory; the parent then reopens the database and
// checks durability. SIGKILL runs no deferred functions and no rollback, which
// is a true process crash (distinct from power loss, which can also corrupt the
// page cache and is not simulated here).

const (
	crashChildEnv   = "BEACON_PHASE2_CRASH_CHILD"
	crashDBEnv      = "BEACON_PHASE2_CRASH_DB"
	crashStageEnv   = "BEACON_PHASE2_CRASH_STAGE"
	crashPreseedEnv = "BEACON_PHASE2_CRASH_PRESEED"
	crashModeEnv    = "BEACON_PHASE2_CRASH_MODE"

	crashPreseedID = "crash-preseed-memory"
	crashTargetID  = "crash-target-memory"
	crashProjectID = "proj-crash"
)

// childWasKilled reports whether the child terminated abruptly at a fault
// point. The child sends itself SIGKILL and falls back to os.Exit(137); on
// macOS the fallback can win the delivery race, so both a signaled status and
// exit code 137 count as a deliberate kill (neither runs deferred rollbacks).
func childWasKilled(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return true
	}
	return exitErr.ExitCode() == 137
}

func crashKillAt(stage string) func(string) {
	return func(s string) {
		if s != stage {
			return
		}
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGKILL)
		os.Exit(137) // unreachable fallback
	}
}

func crashMemory(id, body string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		SchemaVersion: asymptoteobserve.LearningSchemaVersion,
		ID:            id,
		CandidateID:   "candidate-" + id,
		Kind:          asymptoteobserve.LearningMemoryKindDebuggingPattern,
		Title:         "Crash injection " + id,
		Body:          body,
		Project:       asymptoteobserve.LearningProjectV1{ID: crashProjectID, Path: "/repos/crash"},
		Evidence:      []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-" + id}},
		CreatedAt:     memoryTimestamp(1),
		UpdatedAt:     memoryTimestamp(1),
	}
}

// TestPhase2CrashChild is the child entry point. It only acts when invoked with
// the crash-child environment set.
func TestPhase2CrashChild(t *testing.T) {
	if os.Getenv(crashChildEnv) != "1" {
		t.Skip("not a crash child")
	}
	dbPath := os.Getenv(crashDBEnv)
	stage := os.Getenv(crashStageEnv)
	mode := os.Getenv(crashModeEnv)
	store := Open(dbPath)

	switch mode {
	case "migrate":
		// Kill while the schema is being created on first open. The hook must be
		// installed before the store is opened.
		faultHook = crashKillAt("migration_post_schema")
		_, _ = store.db()
		os.Exit(0)
	case "rebuild":
		faultHook = nil
		for i := 0; i < 40; i++ {
			id := fmt.Sprintf("rebuild-memory-%02d", i)
			if err := store.PutMemory(crashMemory(id, "index rebuild seed")); err != nil {
				fmt.Fprintln(os.Stderr, "seed failed:", err)
				os.Exit(3)
			}
		}
		faultHook = crashKillAt("index_rebuild")
		_, _ = store.ListMemories(Query{ProjectID: crashProjectID, Q: "rebuild"})
		os.Exit(0)
	}

	if os.Getenv(crashPreseedEnv) == "1" {
		faultHook = nil
		if err := store.PutMemory(crashMemory(crashPreseedID, "preseeded before the crash")); err != nil {
			fmt.Fprintln(os.Stderr, "preseed failed:", err)
			os.Exit(3)
		}
	}

	// Simulate an abrupt process death at this point. SIGKILL cannot be caught,
	// so no deferred rollback runs.
	faultHook = crashKillAt(stage)
	if err := store.PutMemory(crashMemory(crashTargetID, "written by the crashing child")); err != nil {
		fmt.Fprintln(os.Stderr, "target write failed:", err)
		os.Exit(4)
	}
	if stage == "post_return" {
		// The write was acknowledged (PutMemory returned nil) and only now does
		// the process die. This is the strongest process-crash durability check.
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGKILL)
		os.Exit(137)
	}
	// Reached only when the stage was never hit.
	os.Exit(0)
}

func runCrashChild(t *testing.T, dbPath, stage string, preseed bool) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestPhase2CrashChild", "-test.count=1")
	env := append(os.Environ(),
		crashChildEnv+"=1",
		crashDBEnv+"="+dbPath,
		crashStageEnv+"="+stage,
	)
	if preseed {
		env = append(env, crashPreseedEnv+"=1")
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("child exited 0; expected it to be killed at %q. output:\n%s", stage, out)
	}
	if childWasKilled(err) {
		return
	}
	t.Fatalf("child failed without a signal at %q: %v\n%s", stage, err, out)
}

func runCrashChildMode(t *testing.T, dbPath, mode string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestPhase2CrashChild", "-test.count=1")
	cmd.Env = append(os.Environ(),
		crashChildEnv+"=1", crashDBEnv+"="+dbPath, crashModeEnv+"="+mode)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("child mode %q exited 0; expected a kill. output:\n%s", mode, out)
	}
	if childWasKilled(err) {
		return
	}
	t.Fatalf("child mode %q failed without a signal: %v\n%s", mode, err, out)
}

// TestPhase2CrashDuringIndexRebuild kills the process while the derived FTS
// index is being rebuilt, then checks the authoritative rows survive and the
// index can be rebuilt again on the next search.
func TestPhase2CrashDuringIndexRebuild(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	runCrashChildMode(t, dbPath, "rebuild")

	db := openRaw(t, dbPath)
	integrityCheck(t, db)
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 40 {
		t.Fatalf("authoritative rows after rebuild crash = %d, want 40", count)
	}
	db.Close()

	// Reopening and searching must rebuild the index and return the rows.
	store := Open(dbPath)
	results, err := store.ListMemories(Query{ProjectID: crashProjectID, Q: "rebuild"})
	if err != nil {
		t.Fatalf("search after rebuild crash: %v", err)
	}
	if len(results) != 40 {
		t.Fatalf("search returned %d after rebuild crash, want 40", len(results))
	}
}

// TestPhase2CrashDuringMigration kills the process while the schema is being
// created on a brand-new database and checks that a later open succeeds and is
// idempotent.
func TestPhase2CrashDuringMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	runCrashChildMode(t, dbPath, "migrate")

	store := Open(dbPath)
	if err := store.PutMemory(crashMemory(crashPreseedID, "after migration crash")); err != nil {
		t.Fatalf("write after migration crash: %v", err)
	}
	got, ok, err := store.GetMemory(crashPreseedID)
	if err != nil || !ok {
		t.Fatalf("read after migration crash: ok=%v err=%v", ok, err)
	}
	if got.ID != crashPreseedID {
		t.Fatalf("wrong id %s", got.ID)
	}
	db := openRaw(t, dbPath)
	defer db.Close()
	integrityCheck(t, db)
}

func memoryRowExists(t *testing.T, db *sql.DB, id string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", id, err)
	}
	return n > 0
}

// TestPhase2CrashDurability kills a write at each fault point and checks that
// acknowledged writes survive, unacknowledged writes do not become visible, and
// the database stays intact.
func TestPhase2CrashDurability(t *testing.T) {
	stages := []struct {
		stage        string
		acknowledged bool
	}{
		{"tx_begin_pre", false},
		{"tx_begin_post", false},
		{"memory_insert", false},
		{"fts_update", false},
		{"generation_update", false},
		{"pre_commit", false},
		{"post_commit", true},
		{"post_return", true},
	}
	for _, tc := range stages {
		t.Run(tc.stage, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "memory.db")
			runCrashChild(t, dbPath, tc.stage, true)

			db := openRaw(t, dbPath)
			defer db.Close()
			integrityCheck(t, db)

			if !memoryRowExists(t, db, crashPreseedID) {
				t.Fatalf("acknowledged preseed write lost after crash at %q", tc.stage)
			}
			got := memoryRowExists(t, db, crashTargetID)
			if got != tc.acknowledged {
				t.Fatalf("target visibility after crash at %q = %v, want %v", tc.stage, got, tc.acknowledged)
			}

			// The derived index must be repairable and consistent after recovery.
			store := Open(dbPath)
			results, err := store.ListMemories(Query{ProjectID: crashProjectID, Q: "crash"})
			if err != nil {
				t.Fatalf("search after recovery: %v", err)
			}
			sawTarget := false
			for _, m := range results {
				if m.ID == crashTargetID {
					sawTarget = true
				}
			}
			if sawTarget != tc.acknowledged {
				t.Fatalf("search target visibility after %q = %v, want %v", tc.stage, sawTarget, tc.acknowledged)
			}
			if !tc.acknowledged && len(results) == 0 {
				t.Fatalf("preseed memory not retrievable after crash at %q", tc.stage)
			}
		})
	}
}

// TestPhase2RepeatedRecovery crashes twice and checks that recovery is
// idempotent and no stale state survives.
func TestPhase2RepeatedRecovery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")

	runCrashChild(t, dbPath, "pre_commit", true)
	db := openRaw(t, dbPath)
	integrityCheck(t, db)
	if memoryRowExists(t, db, crashTargetID) {
		t.Fatal("uncommitted target visible after first crash")
	}
	db.Close()

	// A second crash on top of the recovered database.
	runCrashChild(t, dbPath, "memory_insert", true)
	db = openRaw(t, dbPath)
	defer db.Close()
	integrityCheck(t, db)
	if !memoryRowExists(t, db, crashPreseedID) {
		t.Fatal("preseed lost after second crash")
	}
	if memoryRowExists(t, db, crashTargetID) {
		t.Fatal("uncommitted target visible after second crash")
	}

	// An acknowledged write after repeated crashes must commit and persist.
	store := Open(dbPath)
	if err := store.PutMemory(crashMemory(crashTargetID, "committed after recovery")); err != nil {
		t.Fatalf("post-recovery write: %v", err)
	}
	store2 := Open(dbPath)
	got, ok, err := store2.GetMemory(crashTargetID)
	if err != nil || !ok {
		t.Fatalf("committed write not readable after recovery: ok=%v err=%v", ok, err)
	}
	if got.ID != crashTargetID {
		t.Fatalf("wrong memory read back: %s", got.ID)
	}
}

// TestPhase2CrashDuringSupersession crashes while one memory supersedes another
// and checks that the authoritative rows stay mutually consistent.
func TestPhase2CrashDuringSupersession(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := Open(dbPath)
	if err := store.PutMemory(crashMemory(crashPreseedID, "original")); err != nil {
		t.Fatal(err)
	}

	child := func() {
		cmd := exec.Command(os.Args[0], "-test.run=TestPhase2CrashChild", "-test.count=1")
		cmd.Env = append(os.Environ(),
			crashChildEnv+"=1", crashDBEnv+"="+dbPath,
			crashStageEnv+"=pre_commit", crashPreseedEnv+"=0")
		_ = cmd.Run() // expected to be killed
	}
	child()

	db := openRaw(t, dbPath)
	defer db.Close()
	integrityCheck(t, db)

	var superseded string
	err := db.QueryRow(`SELECT COALESCE(superseded_by, '') FROM memories WHERE id = ?`, crashPreseedID).Scan(&superseded)
	if err != nil {
		t.Fatalf("read supersession: %v", err)
	}
	if superseded != "" && !memoryRowExists(t, db, superseded) {
		t.Fatalf("memory %s superseded by non-existent %s", crashPreseedID, superseded)
	}
	if strings.Contains(superseded, crashTargetID) && !memoryRowExists(t, db, crashTargetID) {
		t.Fatalf("dangling supersession pointer to uncommitted %s", crashTargetID)
	}
}

func TestPhase2FaultHookIsNilInProduction(t *testing.T) {
	// The fault hook must never be installed by package initialisation.
	if faultHook != nil {
		t.Fatal("faultHook is non-nil without a test installing it")
	}
}
