package learning

// Phase 3C lifecycle and pooling regression tests. They verify that a Store owns
// one reusable *sql.DB pool, that Close is safe and idempotent, that independent
// Stores do not share state, that concurrent initialization is race-free, and
// that a failed initialization does not leak a pool.

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func p3cMemory(id, project string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: id, Body: "pool lifecycle marker " + id,
		Project:  asymptoteobserve.LearningProjectV1{ID: project},
		Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr-" + id}},
	}
}

// TestStoreReusesOnePool asserts every db() call returns the same pool.
func TestStoreReusesOnePool(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	first, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		again, err := store.db()
		if err != nil {
			t.Fatalf("db() #%d: %v", i, err)
		}
		if again != first {
			t.Fatalf("db() #%d returned a different pool", i)
		}
	}
	if err := store.PutMemory(p3cMemory("m1", "p")); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListMemories(Query{ProjectID: "p", Q: "lifecycle"})
	if err != nil || len(got) != 1 {
		t.Fatalf("read after reuse: %v (%d)", err, len(got))
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestStoreCloseIsIdempotentAndBlocksUse verifies Close releases the pool and
// that later operations fail with ErrStoreClosed.
func TestStoreCloseIsIdempotentAndBlocksUse(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	if err := store.PutMemory(p3cMemory("m1", "p")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close must be a no-op, got %v", err)
	}
	if _, err := store.db(); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("db() after Close = %v, want ErrStoreClosed", err)
	}
	if err := store.PutMemory(p3cMemory("m2", "p")); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("PutMemory after Close = %v, want ErrStoreClosed", err)
	}
	if _, err := store.ListMemories(Query{ProjectID: "p", Q: "x"}); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("ListMemories after Close = %v, want ErrStoreClosed", err)
	}
}

// TestMultipleStoresIndependent verifies two Stores do not share state and that
// closing one leaves the other usable.
func TestMultipleStoresIndependent(t *testing.T) {
	a := Open(filepath.Join(t.TempDir(), "a.db"))
	b := Open(filepath.Join(t.TempDir(), "b.db"))
	if err := a.PutMemory(p3cMemory("a1", "proj-a")); err != nil {
		t.Fatal(err)
	}
	if err := b.PutMemory(p3cMemory("b1", "proj-b")); err != nil {
		t.Fatal(err)
	}
	if da, err := a.db(); err != nil {
		t.Fatal(err)
	} else if db, err := b.db(); err != nil {
		t.Fatal(err)
	} else if da == db {
		t.Fatal("independent Stores returned the same pool")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := b.ListMemories(Query{ProjectID: "proj-b", Q: "lifecycle"})
	if err != nil || len(got) != 1 {
		t.Fatalf("store b unusable after closing a: %v (%d)", err, len(got))
	}
	if _, err := a.ListMemories(Query{ProjectID: "proj-a", Q: "lifecycle"}); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("store a usable after Close: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestConcurrentStoreInitRaceFree drives many goroutines through lazy pool
// creation at once. Run under -race it checks initialization is race-free and
// every caller receives the same pool.
func TestConcurrentStoreInitRaceFree(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	const n = 32
	dbs := make([]*sql.DB, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			dbs[i], errs[i] = store.db()
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("db() #%d: %v", i, err)
		}
	}
	for i := 1; i < n; i++ {
		if dbs[i] != dbs[0] {
			t.Fatalf("concurrent init returned different pools (%d vs 0)", i)
		}
	}
	// The schema must be usable after the concurrent init.
	if err := store.PutMemory(p3cMemory("m1", "p")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestConcurrentStoresSamePathInitRaceFree verifies multiple Store instances on
// the same path can initialize concurrently without racing or exposing a
// partially initialized schema.
func TestConcurrentStoresSamePathInitRaceFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	const n = 12
	stores := make([]*Store, n)
	for i := range stores {
		stores[i] = Open(path)
	}
	var wg sync.WaitGroup
	for i := range stores {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := stores[i].db(); err != nil {
				t.Errorf("store %d init: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	// Every Store sees a working schema; a write through one is visible in another.
	if err := stores[0].PutMemory(p3cMemory("shared", "p")); err != nil {
		t.Fatal(err)
	}
	got, err := stores[n-1].ListMemories(Query{ProjectID: "p", Q: "lifecycle"})
	if err != nil || len(got) != 1 {
		t.Fatalf("cross-store read: %v (%d)", err, len(got))
	}
	for _, s := range stores {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestFailedInitializationDoesNotLeakPool points a Store at a path that cannot
// be opened as a database and checks that no pool is retained and that repeated
// attempts fail the same way.
func TestFailedInitializationDoesNotLeakPool(t *testing.T) {
	// A directory path cannot be opened as a SQLite database file.
	dir := t.TempDir()
	store := Open(dir)
	if _, err := store.db(); err == nil {
		t.Fatal("expected initialization to fail on a directory path")
	}
	store.mu.Lock()
	leaked := store.dbPool
	store.mu.Unlock()
	if leaked != nil {
		t.Fatal("failed initialization retained a pool")
	}
	if _, err := store.db(); err == nil {
		t.Fatal("second attempt unexpectedly succeeded")
	}
	// The Store is still usable for a valid path after a failed init elsewhere.
	ok := Open(filepath.Join(dir, "ok.db"))
	if err := ok.PutMemory(p3cMemory("m1", "p")); err != nil {
		t.Fatalf("valid store failed: %v", err)
	}
	if err := ok.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestConcurrentReadsWritesThenClose runs concurrent readers and writers on one
// Store, then closes it and verifies no error and no goroutine is left blocked.
func TestConcurrentReadsWritesThenClose(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	for i := 0; i < 50; i++ {
		if err := store.PutMemory(p3cMemory(fmt.Sprintf("seed-%02d", i), "p")); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				if _, err := store.ListMemories(Query{ProjectID: "p", Q: "lifecycle", Limit: 10}); err != nil {
					t.Errorf("read: %v", err)
					return
				}
			}
		}()
	}
	var writers sync.WaitGroup
	for w := 0; w < 4; w++ {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for j := 0; j < 25; j++ {
				if err := store.PutMemory(p3cMemory(fmt.Sprintf("w-%d-%d", w, j), "p")); err != nil {
					t.Errorf("write: %v", err)
					return
				}
			}
		}(w)
	}
	writers.Wait()
	close(done)
	readers.Wait()
	if err := store.Close(); err != nil {
		t.Fatalf("Close after concurrency: %v", err)
	}
}
