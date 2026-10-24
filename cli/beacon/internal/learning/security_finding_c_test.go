package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// Finding C reproduction: the frozen original ListMemories, when given any text
// filter, issued `SELECT memory_json FROM memories WHERE <scope> ORDER BY
// updated_at DESC` with no LIMIT, JSON-decoded every scoped row, and filtered in
// Go. A no-match query therefore scanned and decoded the entire project. V7
// keeps that algorithm verbatim as listMemoriesScan (the correctness reference)
// and adds an FTS5-backed path, searchMemories. These benchmarks measure both on
// the same data so the cost difference is reproducible rather than asserted.

func scratchDirTB(tb testing.TB) string {
	tb.Helper()
	dir, err := os.MkdirTemp("", "beacon-finding-c-")
	if err != nil {
		tb.Fatalf("mkdtemp: %v", err)
	}
	tb.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func seedFindingC(tb testing.TB, n int) *Store {
	tb.Helper()
	store := Open(filepath.Join(scratchDirTB(tb), "memory.db"))
	project := asymptoteobserve.LearningProjectV1{ID: "proj"}
	for i := 0; i < n; i++ {
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID:          fmt.Sprintf("memory-%d", i),
			CandidateID: fmt.Sprintf("candidate-%d", i),
			Kind:        asymptoteobserve.LearningMemoryKindConvention,
			Title:       fmt.Sprintf("Convention %d about module %d", i, i%37),
			Body:        "The module uses the standard local workflow and records results.",
			Project:     project,
		}); err != nil {
			tb.Fatalf("seed %d: %v", i, err)
		}
	}
	return store
}

func BenchmarkFindingCSearchNoMatch(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		store := seedFindingC(b, n)
		db, err := store.db()
		if err != nil {
			b.Fatal(err)
		}
		query := Query{ProjectID: "proj", Q: "zzzunmatchablezzz", Limit: 5}

		b.Run(fmt.Sprintf("original-scan/N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				got, err := store.listMemoriesScan(db, query)
				if err != nil {
					b.Fatal(err)
				}
				if len(got) != 0 {
					b.Fatalf("no-match query returned %d rows", len(got))
				}
			}
		})
		b.Run(fmt.Sprintf("v7-indexed/N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				got, err := store.searchMemories(db, query)
				if err != nil {
					b.Fatal(err)
				}
				if len(got) != 0 {
					b.Fatalf("no-match query returned %d rows", len(got))
				}
			}
		})
		if err := store.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
