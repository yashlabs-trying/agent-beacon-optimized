package learning

// Final-optimization regression test for the search selectivity path.
//
// queryIsSelective was changed from one SQL round trip per query term to a
// single batched trigram lookup. The verdict must be identical, so the adaptive
// indexed path (searchMemories) must return exactly the same rows as the
// authoritative scan (listMemoriesScan) for every query shape, including long
// multi-term queries.

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func finalIDs(ms []asymptoteobserve.LearningMemoryV1) string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.ID
	}
	return strings.Join(out, ",")
}

func TestFinalSearchIndexedMatchesScan(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "m.db"))
	defer store.Close()
	for i := 0; i < 400; i++ {
		body := "ordinary workflow text about modules and retries"
		if i%17 == 0 {
			body = "the rare unique-needle recovery step"
		}
		if i%5 == 0 {
			body = "cache invalidation strategy for the cache layer"
		}
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("m-%04d", i), CandidateID: fmt.Sprintf("c-%04d", i),
			Kind: asymptoteobserve.LearningMemoryKindConvention, Title: "t",
			Body: body, Project: asymptoteobserve.LearningProjectV1{ID: "p"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	if !store.ensureIndexReady(db) {
		t.Fatal("derived index did not become ready")
	}

	queries := []string{
		"workflow",
		"workflow modules retries",
		"cache invalidation",
		"unique-needle",
		strings.Repeat("workflow modules ", 20), // 40 terms: the regression target
		"zzzunmatchablezzz",
	}
	for _, q := range queries {
		query := Query{ProjectID: "p", Q: q, Limit: 10}
		indexed, err := store.searchMemories(db, query)
		if err != nil {
			t.Fatalf("searchMemories(%q): %v", q, err)
		}
		scan, err := store.listMemoriesScan(db, query)
		if err != nil {
			t.Fatalf("listMemoriesScan(%q): %v", q, err)
		}
		if finalIDs(indexed) != finalIDs(scan) {
			t.Fatalf("query %q: indexed=%q scan=%q", q, finalIDs(indexed), finalIDs(scan))
		}
	}
}
