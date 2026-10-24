package learning

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// These tests hold the adaptive search path to the original scan it replaced.
// listMemoriesScan is the reference: it is the exact loop Store.ListMemories ran
// before the index existed. Every case compares ListMemories against it.

func memoryTimestamp(i int) string {
	return time.Unix(1700000000+int64(i), 0).UTC().Format(time.RFC3339Nano)
}

// seedMemories inserts memories directly, bypassing PutMemory so large fixtures
// are cheap. It must run before any search, while index_ready is still 0, so the
// first search rebuilds the index from these rows.
func seedMemories(tb testing.TB, store *Store, memories []asymptoteobserve.LearningMemoryV1) {
	tb.Helper()
	db, err := store.db()
	if err != nil {
		tb.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		tb.Fatal(err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO memories (id, candidate_id, project_id, kind, title, created_at, updated_at, superseded_by, memory_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tb.Fatal(err)
	}
	defer stmt.Close()
	for _, m := range memories {
		data, err := json.Marshal(m)
		if err != nil {
			tb.Fatal(err)
		}
		if _, err := stmt.Exec(m.ID, m.CandidateID, m.Project.ID, m.Kind, m.Title, m.CreatedAt, m.UpdatedAt, m.SupersededBy, string(data)); err != nil {
			tb.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		tb.Fatal(err)
	}
}

// diverseDataset builds deterministic memories that exercise the interesting
// corners of the matcher: a common word, rare tokens, multi-word phrases,
// Unicode, punctuation, multiple projects and kinds, and superseded rows.
func diverseDataset(n int) []asymptoteobserve.LearningMemoryV1 {
	projects := []string{"proj-a", "proj-a", "proj-a", "proj-b"}
	kinds := []string{
		asymptoteobserve.LearningMemoryKindConvention,
		asymptoteobserve.LearningMemoryKindDebuggingPattern,
	}
	out := make([]asymptoteobserve.LearningMemoryV1, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("memory-%05d", i)
		title := fmt.Sprintf("Note %d about the workflow", i)
		body := "The agent should follow the ordinary the steps for the task."
		switch {
		case i == 1:
			body = "The zzzoldneedle recovery note for a long-past incident."
		case i%500 == 0:
			body = "Remember the rare unique-needle recovery step for the outage."
		case i%37 == 0:
			body = "Run the package smoke test again after a transient fetch failure."
		case i%53 == 0:
			body = "Prefer café-style Unicode labels here."
		case i%71 == 0:
			body = "Use the foo-bar flag together with foo.baz."
		}
		project := projects[i%len(projects)]
		m := asymptoteobserve.LearningMemoryV1{
			ID:          id,
			CandidateID: "candidate-" + id,
			Kind:        kinds[i%len(kinds)],
			Title:       title,
			Body:        body,
			Project:     asymptoteobserve.LearningProjectV1{ID: project, Path: "/repos/" + project},
			CreatedAt:   memoryTimestamp(i),
			UpdatedAt:   memoryTimestamp(i),
		}
		if i%97 == 0 && i > 0 {
			m.SupersededBy = fmt.Sprintf("memory-%05d", i-1)
		}
		out = append(out, m)
	}
	return out
}

func resultsEquivalent(got, want []asymptoteobserve.LearningMemoryV1) bool {
	if len(got) != len(want) {
		return false
	}
	gotCount := map[string]int{}
	wantCount := map[string]int{}
	for i := range got {
		gotCount[got[i].ID]++
		wantCount[want[i].ID]++
		if got[i].UpdatedAt != want[i].UpdatedAt {
			return false
		}
	}
	if len(gotCount) != len(wantCount) {
		return false
	}
	for id, count := range gotCount {
		if wantCount[id] != count {
			return false
		}
	}
	return true
}

func oracleScan(t *testing.T, store *Store, query Query) []asymptoteobserve.LearningMemoryV1 {
	t.Helper()
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := store.scopeQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.listMemoriesScan(db, scoped)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSearchMatchesScanAcrossCases(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	seedMemories(t, store, diverseDataset(1200))

	cases := []Query{
		{ProjectID: "proj-a", Q: "unique-needle"},
		{ProjectID: "proj-a", Q: "UNIQUE-NEEDLE"},
		{ProjectID: "proj-a", Q: "package smoke"},
		{ProjectID: "proj-a", Q: "package   smoke"},
		{ProjectID: "proj-a", Q: "café"},
		{ProjectID: "proj-a", Q: "cafe"},
		{ProjectID: "proj-a", Q: "foo-bar"},
		{ProjectID: "proj-a", Q: "foo.bar"},
		{ProjectID: "proj-a", Q: "oo-ba"},
		{ProjectID: "proj-a", Q: "the", Limit: 10},
		{ProjectID: "proj-a", Q: "the", Limit: 500},
		{ProjectID: "proj-a", Q: "workflow"},
		{ProjectID: "proj-a", Q: "no-such-token-anywhere"},
		{ProjectID: "proj-a", Q: "ne"},
		{ProjectID: "proj-a", Q: "n"},
		{ProjectID: "proj-a", Q: "zz"},
		{ProjectID: "proj-a", Q: "the task"},
		{ProjectID: "proj-a", Q: "task the"},
		{ProjectID: "proj-a", Q: "ordinary", Kind: asymptoteobserve.LearningMemoryKindConvention},
		{ProjectID: "proj-b", Q: "workflow"},
		{ProjectID: "proj-a", Q: "workflow", Limit: 5, Page: 2},
		{ProjectID: "proj-a", Q: "the", Limit: 20, Page: 3},
		{ProjectID: "proj-a", Q: "recovery"},
	}
	for _, query := range cases {
		t.Run(fmt.Sprintf("q=%q/kind=%q/limit=%d/page=%d/proj=%s", query.Q, query.Kind, query.Limit, query.Page, query.ProjectID), func(t *testing.T) {
			want := oracleScan(t, store, query)
			got, err := store.ListMemories(query)
			if err != nil {
				t.Fatalf("ListMemories: %v", err)
			}
			if !resultsEquivalent(got, want) {
				t.Fatalf("adaptive result differs from scan\n got=%d %v\nwant=%d %v", len(got), ids(got), len(want), ids(want))
			}
		})
	}
}

func ids(list []asymptoteobserve.LearningMemoryV1) []string {
	out := make([]string, len(list))
	for i, m := range list {
		out[i] = m.ID
	}
	return out
}

func TestSearchIndexFallsBackWhenIndexTableDropped(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	seedMemories(t, store, diverseDataset(300))

	first, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "unique-needle"})
	if err != nil || len(first) == 0 {
		t.Fatalf("baseline query: %v (%d)", err, len(first))
	}

	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE memories_fts`); err != nil {
		t.Fatal(err)
	}
	resetSearchCacheForTest()

	got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "unique-needle"})
	if err != nil {
		t.Fatalf("after drop: %v", err)
	}
	if !resultsEquivalent(got, first) {
		t.Fatalf("dropped index changed results: got %v want %v", ids(got), ids(first))
	}
}

func TestSearchCacheInvalidatedByWrite(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "memory-1", CandidateID: "candidate-1", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "First", Body: "the needle is here", Project: project,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "needle"})
	if err != nil || len(got) != 1 {
		t.Fatalf("first query: %v (%d)", err, len(got))
	}
	// A second, newer memory must appear even though the first query is cached.
	if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "memory-2", CandidateID: "candidate-2", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "Second", Body: "another needle", Project: project,
	}); err != nil {
		t.Fatal(err)
	}
	got, err = store.ListMemories(Query{ProjectID: "proj-a", Q: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("write did not invalidate cache: got %v", ids(got))
	}
}

func TestSearchCacheInvalidatedBySupersede(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	original := asymptoteobserve.LearningMemoryV1{
		ID: "memory-1", CandidateID: "candidate-1", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "First", Body: "the needle is here", Project: project,
	}
	if err := store.PutMemory(original); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.ListMemories(Query{ProjectID: "proj-a", Q: "needle"}); len(got) != 1 {
		t.Fatalf("pre-supersede query: %d", len(got))
	}
	original.SupersededBy = "memory-9"
	if err := store.PutMemory(original); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("superseded memory still returned: %v", ids(got))
	}
}

func TestPlannerSelectivity(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	// Every memory contains the common word, so a query for it overflows the
	// candidate cap and must be handed to the scan path. One memory also carries
	// a rare token that the index can answer.
	memories := make([]asymptoteobserve.LearningMemoryV1, 0, searchCandidateCap+50)
	for i := 0; i < searchCandidateCap+50; i++ {
		body := "the ordinary workflow"
		if i == 7 {
			body = "the zzzneedle recovery"
		}
		memories = append(memories, asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("memory-%05d", i), CandidateID: fmt.Sprintf("candidate-%05d", i),
			Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "Common", Body: body,
			Project:   asymptoteobserve.LearningProjectV1{ID: "proj-a"},
			CreatedAt: memoryTimestamp(i), UpdatedAt: memoryTimestamp(i),
		})
	}
	seedMemories(t, store, memories)

	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	if !store.ensureIndexReady(db) {
		t.Fatal("index did not become ready")
	}
	common, err := store.fetchIndexedCandidates(db, Query{ProjectID: "proj-a", Q: "ordinary"}, []string{"ordinary"})
	if err != nil {
		t.Fatal(err)
	}
	if common != nil {
		t.Fatalf("common term should overflow to the scan path, got %d candidates", len(common))
	}
	rare, err := store.fetchIndexedCandidates(db, Query{ProjectID: "proj-a", Q: "zzzneedle"}, []string{"zzzneedle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rare) != 1 {
		t.Fatalf("rare term should be answered by the index, got %d candidates", len(rare))
	}
}

func TestSearchIndexRebuildIsIdempotent(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	seedMemories(t, store, diverseDataset(200))
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if !store.ensureIndexReady(db) {
			t.Fatalf("rebuild %d failed", i)
		}
	}
	got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "unique-needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected a match after repeated rebuilds")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories_fts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if count != total {
		t.Fatalf("index rows = %d, want one per memory (%d)", count, total)
	}
}

func TestSearchHandlesQuotesInQuery(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID: "memory-1", CandidateID: "candidate-1", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: `Handling "quoted" input`, Body: "the needle", Project: project,
	}); err != nil {
		t.Fatal(err)
	}
	// An unbalanced quote must not be interpreted as FTS syntax; it should simply
	// be treated as a substring and match nothing here.
	got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: `"quoted"`})
	if err != nil {
		t.Fatalf("ListMemories with quotes: %v", err)
	}
	want := oracleScan(t, store, Query{ProjectID: "proj-a", Q: `"quoted"`})
	if !resultsEquivalent(got, want) {
		t.Fatalf("quote query mismatch: got %v want %v", ids(got), ids(want))
	}
}

// TestSearchConcurrentReadersAndWriters exercises the shared store under
// concurrent search and mutation. Run with -race, it checks that the index,
// generation counter and cache never expose a torn or cross-project result, and
// that acknowledged writes are all visible afterwards.
func TestSearchConcurrentReadersAndWriters(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	seedMemories(t, store, diverseDataset(500))

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
				got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "unique-needle", Limit: 10})
				if err != nil {
					t.Errorf("concurrent read: %v", err)
					return
				}
				for _, m := range got {
					if m.Project.ID != "proj-a" {
						t.Errorf("cross-project leak: got project %q", m.Project.ID)
						return
					}
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
				id := fmt.Sprintf("concurrent-%d-%d", w, j)
				if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
					ID:          id,
					CandidateID: "candidate-" + id,
					Kind:        asymptoteobserve.LearningMemoryKindConvention,
					Title:       "Concurrent",
					Body:        "a fresh unique-needle note",
					Project:     asymptoteobserve.LearningProjectV1{ID: "proj-a"},
				}); err != nil {
					t.Errorf("concurrent write: %v", err)
					return
				}
			}
		}(w)
	}
	writers.Wait()
	close(done)
	readers.Wait()

	got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "unique-needle", Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 100 {
		t.Fatalf("acknowledged writes missing after concurrency: got %d, want at least 100", len(got))
	}
}
