package learning

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// TestSearchCrossStoreVisibility checks that a write through one Store is
// visible to another Store on the same database on its next query, and that the
// second store's cache does not serve the pre-write result.
func TestSearchCrossStoreVisibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	a := Open(path)
	b := Open(path)
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}

	if err := a.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "m1", CandidateID: "c1", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "first", Body: "the needle", Project: project,
	}); err != nil {
		t.Fatal(err)
	}
	first, err := b.ListMemories(Query{ProjectID: "proj-a", Q: "needle"})
	if err != nil || len(first) != 1 {
		t.Fatalf("store b first read: %v (%d)", err, len(first))
	}

	// Write through a; b's cached entry must not be served afterwards.
	if err := a.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "m2", CandidateID: "c2", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "second", Body: "another needle", Project: project,
	}); err != nil {
		t.Fatal(err)
	}
	second, err := b.ListMemories(Query{ProjectID: "proj-a", Q: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 {
		t.Fatalf("store b did not see the cross-store write: got %v", ids(second))
	}
}

// TestSearchRecoversAfterIndexDrop drops the derived index table, checks that a
// write still succeeds (the authoritative store must not depend on derived
// state), that the next query returns the correct answer, and that the index is
// rebuilt and marked ready again.
func TestSearchRecoversAfterIndexDrop(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	seedMemories(t, store, diverseDataset(300))
	query := Query{ProjectID: "proj-a", Q: "unique-needle", Limit: 50}

	before, err := store.ListMemories(query)
	if err != nil || len(before) == 0 {
		t.Fatalf("baseline query: %v (%d)", err, len(before))
	}

	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE memories_fts`); err != nil {
		t.Fatal(err)
	}

	// A write must succeed even though the derived index is gone.
	if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "post-drop", CandidateID: "post-drop", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "post drop", Body: "a fresh unique-needle note", Project: project,
	}); err != nil {
		t.Fatalf("write failed with the index gone: %v", err)
	}

	// The result must be correct and must include the write.
	after, err := store.ListMemories(query)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range after {
		if m.ID == "post-drop" {
			found = true
		}
	}
	if !found {
		t.Fatalf("post-drop write missing from results: %v", ids(after))
	}
	if len(after) != len(before)+1 {
		t.Fatalf("post-drop result count = %d, want %d", len(after), len(before)+1)
	}

	// The index must be rebuilt and marked ready.
	db, err = store.db()
	if err != nil {
		t.Fatal(err)
	}
	var ready string
	if err := db.QueryRow(`SELECT value FROM memory_index_meta WHERE key = 'index_ready'`).Scan(&ready); err != nil {
		t.Fatal(err)
	}
	if ready != "1" {
		t.Fatalf("index did not recover: index_ready=%q", ready)
	}
	var ftsRows, memRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories_fts`).Scan(&ftsRows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&memRows); err != nil {
		t.Fatal(err)
	}
	if ftsRows != memRows {
		t.Fatalf("rebuilt index has %d rows, want %d", ftsRows, memRows)
	}
}

// TestSearchConcurrentSupersedeAndRead exercises superseding (the store's
// deletion mechanism) concurrently with reads and confirms superseded memories
// stop being returned and no read observes a torn state.
func TestSearchConcurrentSupersedeAndRead(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	const count = 200
	for i := 0; i < count; i++ {
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("m-%03d", i), CandidateID: fmt.Sprintf("c-%03d", i),
			Kind:  asymptoteobserve.LearningMemoryKindConvention,
			Title: "note", Body: "the needle body", Project: project,
		}); err != nil {
			t.Fatal(err)
		}
	}

	done := make(chan struct{})
	var readers sync.WaitGroup
	for r := 0; r < 6; r++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "needle", Limit: 500})
				if err != nil {
					t.Errorf("read: %v", err)
					return
				}
				for _, m := range got {
					if m.SupersededBy != "" {
						t.Errorf("superseded memory %s returned", m.ID)
						return
					}
				}
			}
		}()
	}

	var writers sync.WaitGroup
	for w := 0; w < 3; w++ {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := w; i < count; i += 3 {
				if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
					ID: fmt.Sprintf("m-%03d", i), CandidateID: fmt.Sprintf("c-%03d", i),
					Kind:  asymptoteobserve.LearningMemoryKindConvention,
					Title: "note", Body: "the needle body", Project: project,
					SupersededBy: "m-999",
				}); err != nil {
					t.Errorf("supersede: %v", err)
					return
				}
			}
		}(w)
	}
	writers.Wait()
	close(done)
	readers.Wait()

	remaining, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "needle", Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected all memories superseded, %d remain", len(remaining))
	}
}
