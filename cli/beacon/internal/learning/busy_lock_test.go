package learning

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func TestPutMemoryWaitsForBoundedWriteContention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	store := Open(path)
	if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "first", CandidateID: "first-c", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "first", Body: "initial write", Project: asymptoteobserve.LearningProjectV1{ID: "p"},
	}); err != nil {
		t.Fatal(err)
	}
	blocker, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	tx, err := blocker.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE memory_index_meta SET value = CAST(value AS INTEGER) + 1 WHERE key = 'generation'`); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		close(started)
		finished <- store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: "second", CandidateID: "second-c", Kind: asymptoteobserve.LearningMemoryKindConvention,
			Title: "second", Body: "acknowledged after bounded contention",
			Project: asymptoteobserve.LearningProjectV1{ID: "p"},
		})
	}()
	<-started
	// This exceeds the former five-second SQLite busy timeout even when one
	// schema check and the transaction each consume a separate wait window.
	time.Sleep(11 * time.Second)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("write after lock release: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("write did not complete after the lock was released")
	}
	if _, ok, err := store.GetMemory("second"); err != nil || !ok {
		t.Fatalf("acknowledged write not durable: ok=%v error=%v", ok, err)
	}
}
