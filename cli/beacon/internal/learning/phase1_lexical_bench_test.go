package learning

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// The same file is run unmodified in the detached original Beacon checkout.
// It measures identical queries and data on the same host, excluding setup.
func BenchmarkPhase1Lexical(b *testing.B) {
	for _, n := range []int{100, 1000, 5000, 10000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			store := Open(filepath.Join(b.TempDir(), "memory.db"))
			for i := 0; i < n; i++ {
				body := fmt.Sprintf("Build workflow memory number %d describes module configuration and test setup.", i)
				if i%100 == 0 {
					body += " rareneedle recovery step."
				}
				if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
					ID: fmt.Sprintf("bench-%05d", i), CandidateID: fmt.Sprintf("candidate-%05d", i),
					Kind:  asymptoteobserve.LearningMemoryKindConvention,
					Title: "benchmark memory", Body: body,
					Project: asymptoteobserve.LearningProjectV1{ID: "bench-project"},
				}); err != nil {
					b.Fatal(err)
				}
			}
			for _, query := range []struct {
				name, text string
				wantEmpty  bool
			}{
				{"rare", "rareneedle", false},
				{"no-match", "absenttoken", true},
			} {
				b.Run(query.name, func(b *testing.B) {
					q := Query{ProjectID: "bench-project", Q: query.text, Limit: 5}
					got, err := store.ListMemories(q)
					if err != nil || (len(got) == 0) != query.wantEmpty {
						b.Fatalf("preflight query %q: count=%d error=%v", query.text, len(got), err)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := store.ListMemories(q); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
